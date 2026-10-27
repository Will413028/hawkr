package infrastructure

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkghttputil "github.com/Will413028/hawkr/go-pkg/httputil"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// 反代路徑上的營運鍵 header。**名稱必須與 gateway 的 middleware 逐字相同**
// （gateway/internal/middleware/domain_id_header.go 的 BuyerIDHeader／StoreIDHeader）。
// 拼錯不會編譯錯、不會有錯誤訊息——只會讓每一次請求都走 fail-closed 的 404，
// 症狀是「登入了卻說發票不存在」。
const (
	buyerIDHeader = "x-buyer-id"
	storeIDHeader = "x-store-id"
	// accountIDHeader 由 proxy.CreateProxy 注入（gateway/internal/proxy/reverse_proxy.go）。
	accountIDHeader = "x-account-id"
)

// notFoundBizCode 是 9301。越權與不存在共用它——見 usecase/document_file.go 檔頭。
const notFoundBizCode = bizcode.InvoiceDocumentNotFound

// fileReadHeaderTimeout 擋 Slowloris；bytes 面沒有請求 body，10 秒綽綽有餘。
const fileReadHeaderTimeout = 10 * time.Second

// FileServerParams contains the fx dependencies for the PDF download channel.
type FileServerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Config    *config.Config
	Files     *usecase.DocumentFile
	// AdminJobs 提供 A4 解鎖證據的下載（#3108 第 4 件）。
	AdminJobs *usecase.AdminJob
	// TaxEvidence 提供後台稅籍審核頁的佐證檔下載（plan 8f-1）。
	TaxEvidence *usecase.TaxEvidenceFile
	Logger      *zap.Logger
}

// NewFileHTTPServer 建立 10042 的 PDF 授權下載通道（58 號設計書 §4.3.1 / §6.1.4）。
//
// 為什麼是**獨立的 http.Server** 而不是掛在 10040：
//   - 10040 的共用 router 由 pkg/server 提供，並被 TestNewRouter_ExposesOnlyOperationalRoutes
//     釘死在 /health、/metrics、/debug/pprof 三條；把 bytes 路由掛上去會讓那支測試轉紅。
//   - NetworkPolicy 對這兩個 port 的來源集合不同：10040 需要 node/pod CIDR 的 ipBlock
//     （kubelet probe 與中央 Prometheus），10042 只放行 gateway 一個 podSelector。
//     併在同一個 port 上就沒辦法只放行 gateway。
//   - gRPC 面不承載 bytes，所以也不能併進 10041。
//
// Gateway 側以 proxy.CreateProxy(ServiceInvoice, …) 反代到這裡，且**已在反代之前**
// 注入 x-buyer-id／x-store-id（proxy 本身只帶 x-account-id／x-account-type）。
// 到這裡的路徑已被 gateway 去掉 /api/v1 前綴。
func NewFileHTTPServer(p FileServerParams) *http.Server {
	mux := http.NewServeMux()

	// 三條二進位子路由（§6.1.4 的路由表）。**三個面各自一條**而不是一條共用：
	// 每一面的授權述詞不同，而「用同一條路由再靠參數分流」正是 Tier A 最容易破的形狀。
	mux.HandleFunc("GET /app/orders/{order_no}/invoice_documents/{document_id}/file/content",
		serveFile(p, usecase.FileActorBuyer))
	mux.HandleFunc("GET /app/seller/invoices/documents/{document_id}/file/content",
		serveFile(p, usecase.FileActorSeller))
	mux.HandleFunc("GET /admin/invoices/documents/{document_id}/file/content",
		serveFile(p, usecase.FileActorAdmin))

	// A4 解鎖證據（#3108 第 4 件）。**獨立一條**而不是掛在上面那條：
	// 它的資源是 invoice_manual_unlocks 不是 invoice_documents，授權述詞也不同
	// （unlock 必須屬於該 job）。用同一條再靠參數分流正是 Tier A 最容易破的形狀。
	mux.HandleFunc("GET /admin/invoices/jobs/{job_id}/manual_unlocks/{unlock_id}/evidence/content",
		serveUnlockEvidence(p))

	// 後台稅籍審核頁的佐證檔（plan 8f-1）。授權述詞是「該 store 的稅務資格在這個位置上
	// 有一份佐證」，由 invoice 持有；取代已刪除的通用 admin 機敏下載（連號 asset ID、不驗歸屬）。
	mux.HandleFunc("GET /admin/invoices/tax_reviews/{store_id}/documents/{document_id}/content",
		serveTaxEvidence(p))

	// 其餘路徑一律 501：回 404 會與「Gateway 反代設定漏了」的症狀混淆，
	// 而那兩者的排查方向完全不同（§12.1.2 F 的 _FILE_PORT 與 §12.2.4 (d) NetworkPolicy）。
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invoice file channel: route not implemented", http.StatusNotImplemented)
	})

	srv := &http.Server{
		Addr:              ":" + p.Config.Server.FilePort,
		Handler:           mux,
		ReadHeaderTimeout: fileReadHeaderTimeout,
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			p.Logger.Info("Starting invoice file HTTP server",
				zap.String("port", p.Config.Server.FilePort))
			go func() {
				if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
					p.Logger.Error("invoice file HTTP server error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.Logger.Info("Stopping invoice file HTTP server")
			return srv.Shutdown(ctx)
		},
	})

	return srv
}

// serveFile 是三條子路由共用的 handler 主體，actor 由掛載點決定而不是由請求決定。
//
// 🔴 **actor 是閉包參數不是 query／header**：讓請求自己說「我是 admin」等於沒有授權。
func serveFile(p FileServerParams, actor usecase.FileActorKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		documentID, err := strconv.ParseInt(r.PathValue("document_id"), 10, 64)
		if err != nil {
			// 格式錯誤同樣走 not-found 出口：回 400 會告訴攻擊者「這個 id 格式是對的」。
			writeNotFound(w)
			return
		}

		req := usecase.FileRequest{
			Actor:      actor,
			DocumentID: documentID,
			Style:      r.URL.Query().Get("style"),
		}
		// 🔴 fail-closed：缺 header 就當作沒有述詞 ⇒ 404（§6.1.4 逐字要求）。
		// usecase 側對 0 值也會 fail-closed，這裡不解析成 0 只是為了少一層依賴。
		switch actor {
		case usecase.FileActorBuyer:
			req.BuyerID = headerID(r, buyerIDHeader)
		case usecase.FileActorSeller:
			req.StoreID = headerID(r, storeIDHeader)
		case usecase.FileActorAdmin:
			// ADM 沒有 ownership 概念，授權在 gateway 的 admin group。
		}

		res, data, contentType, err := p.Files.Read(r.Context(), req)
		if err != nil {
			writeFileError(w, p.Logger, documentID, err)
			return
		}

		// 🔴 兩個 header 都必須帶，而且彼此無關（§6.1.4「必做的四件事」第 2 條）：
		//   - Cache-Control: private, no-store —— 少了它，中間層會快取住別人的發票
		//   - Content-Disposition —— `?disposition=inline` 是頁內 PDF 檢視器的前提；
		//     <iframe src> 指向回 attachment 的 URL 時瀏覽器會觸發下載而不是顯示，
		//     而且**不會有任何錯誤訊息**（前端做了也看不到東西）。
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Disposition", contentDisposition(r, res.FileName))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(data); err != nil {
			p.Logger.Warn("invoice.file.write_failed",
				zap.Int64("document_id", documentID), zap.Error(err))
		}
	}
}

// contentDisposition 依 ?disposition= 決定；預設 attachment（§6.1.4）。
//
// 🔴 **這裡的檔名不再「由我方產生」**。這段註解原本寫著「檔名由我方產生
// （invoice-<發票號>.pdf）……明天有人把買家姓名放進檔名」——那個明天在 backend#3149
// 就到了：A4 解鎖證據走 ServerReadSecureAsset，media 現在回的是**使用者上傳時的原始
// 檔名**（`AssetFilename.resolveDisplayName`），而 ADM 操作者上傳的取消證據幾乎必然是
// 中文檔名。
//
// 於是輸出必須是 RFC 6266 的雙參數：`filename` 給不認得的舊 client（非 ASCII 換底線），
// `filename*` 才是真正的檔名。只給 `filename=` 的話 CJK 會被瀏覽器亂碼或截斷——而這條
// 路徑存在的理由（§J28）正是「讓覆核者看得到證據」。
func contentDisposition(r *http.Request, fileName string) string {
	kind := "attachment"
	if r.URL.Query().Get("disposition") == "inline" {
		kind = "inline"
	}
	// 編碼那一半交給 pkg/httputil（csvutil 也用同一支，避免兩份實作漂移）；
	// 這裡只負責「剝掉會破壞 header 的字元」與本服務的預設名。
	return pkghttputil.ContentDisposition(kind, sanitizeFileName(fileName), "invoice.pdf")
}

// sanitizeFileName 去掉會破壞 header 的字元。
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("\r", "", "\n", "", `"`, "", "\\", "")
	out := replacer.Replace(name)
	if out == "" {
		return "invoice.pdf"
	}
	return out
}

// headerID 解析營運鍵 header。解析不出來回 0，而 0 在 usecase 側是 fail-closed。
func headerID(r *http.Request, name string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(name)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// writeFileError 把 usecase 的錯誤折成這條通道的回應。
//
// 🔴 業務碼（9312／9313）走 **409 ＋ envelope JSON**，不回 bytes（§6.1.4 第 4 條）：
// 回 0 bytes 的「PDF」會讓使用者拿到一個打不開的檔案，而那看起來像我方壞了。
// 9301 走 404，且與越權**逐字相同**。
func writeFileError(w http.ResponseWriter, log *zap.Logger, documentID int64, err error) {
	if errors.Is(err, usecase.ErrInvalidFileStyle) {
		writeEnvelope(w, http.StatusBadRequest, 400, "style 不在允許值域內（0/1/2/3/5）")
		return
	}
	var domainErr *pkgerrors.DomainError
	if errors.As(err, &domainErr) {
		if int32(domainErr.Code) == notFoundBizCode {
			writeNotFound(w)
			return
		}
		writeEnvelope(w, http.StatusConflict, int32(domainErr.Code), domainErr.Message)
		return
	}
	// 非業務錯誤（media 取不到、DB 斷線）⇒ 503。**不折成 404**：
	// 把系統故障講成「你的發票不存在」會把使用者推去申訴一件沒發生的事。
	log.Error("invoice.file.serve_failed", zap.Int64("document_id", documentID), zap.Error(err))
	writeEnvelope(w, http.StatusServiceUnavailable, 503, "invoice file temporarily unavailable")
}

func writeNotFound(w http.ResponseWriter) {
	writeEnvelope(w, http.StatusNotFound, notFoundBizCode, "文件不存在")
}

// writeEnvelope 手寫 envelope 而不是 import gateway 的 response package：
// 這是服務側的 HTTP 面，不該相依 gateway。形狀仍遵守 §2 的 {code,message,data}。
func writeEnvelope(w http.ResponseWriter, httpStatus int, code int32, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(httpStatus)
	_, _ = w.Write([]byte(`{"code":` + strconv.FormatInt(int64(code), 10) +
		`,"message":` + strconv.Quote(message) + `,"data":null}`))
}

// serveUnlockEvidence 送出 A4 解鎖的取消證據檔（#3108 第 4 件）。
//
// 🔴 覆核者看得到檔案本身，雙人確認才不是儀式——J28 的註解逐字：
// 「沒有它，第二個人是在對一份看不見證據的請求按同意」。
//
// header 與 serveFile 逐字相同（Cache-Control 少了它中間層會快取住別人的證據；
// Content-Disposition 讓 ?disposition=inline 的頁內預覽成立）。
func serveUnlockEvidence(p FileServerParams) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID, err := strconv.ParseInt(r.PathValue("job_id"), 10, 64)
		if err != nil {
			writeNotFound(w)
			return
		}
		unlockID, err := strconv.ParseInt(r.PathValue("unlock_id"), 10, 64)
		if err != nil {
			// 格式錯誤同樣走 not-found 出口：回 400 會告訴攻擊者「這個 id 格式是對的」。
			writeNotFound(w)
			return
		}
		if p.AdminJobs == nil {
			writeNotFound(w)
			return
		}

		file, err := p.AdminJobs.ReadUnlockEvidence(r.Context(), jobID, unlockID)
		if err != nil {
			writeFileError(w, p.Logger, unlockID, err)
			return
		}

		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
		w.Header().Set("Content-Disposition", contentDisposition(r, file.FileName))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(file.Data); err != nil {
			p.Logger.Warn("invoice.unlock_evidence.write_failed",
				zap.Int64("unlock_id", unlockID), zap.Error(err))
		}
	}
}

// serveTaxEvidence 送出稅籍審核頁的佐證檔（plan 8f-1）。
//
// header 與 serveUnlockEvidence 逐字相同。id 格式錯誤、位置超出範圍、別的 store 的資產
// 全部走同一個 not-found 出口，不告訴呼叫端哪一段不成立。
func serveTaxEvidence(p FileServerParams) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeID, err := strconv.ParseInt(r.PathValue("store_id"), 10, 64)
		if err != nil {
			writeNotFound(w)
			return
		}
		documentID, err := strconv.ParseInt(r.PathValue("document_id"), 10, 64)
		if err != nil {
			writeNotFound(w)
			return
		}
		if p.TaxEvidence == nil {
			writeNotFound(w)
			return
		}

		// actor 是 gateway 的 proxy 注入的 x-account-id（管理員）；缺漏時 usecase fail-closed 回查無。
		file, err := p.TaxEvidence.Read(r.Context(), headerID(r, accountIDHeader), storeID, documentID)
		if err != nil {
			writeFileError(w, p.Logger, documentID, err)
			return
		}

		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
		w.Header().Set("Content-Disposition", contentDisposition(r, file.FileName))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(file.Data); err != nil {
			p.Logger.Warn("invoice.tax_evidence.write_failed",
				zap.Int64("store_id", storeID),
				zap.Int64("document_id", documentID), zap.Error(err))
		}
	}
}
