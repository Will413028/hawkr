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

const (
	buyerIDHeader = "x-buyer-id"
	storeIDHeader = "x-store-id"
	accountIDHeader = "x-account-id"
)

const notFoundBizCode = bizcode.InvoiceDocumentNotFound

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

	mux.HandleFunc("GET /app/orders/{order_no}/invoice_documents/{document_id}/file/content",
		serveFile(p, usecase.FileActorBuyer))
	mux.HandleFunc("GET /app/seller/invoices/documents/{document_id}/file/content",
		serveFile(p, usecase.FileActorSeller))
	mux.HandleFunc("GET /admin/invoices/documents/{document_id}/file/content",
		serveFile(p, usecase.FileActorAdmin))

	mux.HandleFunc("GET /admin/invoices/jobs/{job_id}/manual_unlocks/{unlock_id}/evidence/content",
		serveUnlockEvidence(p))

	mux.HandleFunc("GET /admin/invoices/tax_reviews/{store_id}/documents/{document_id}/content",
		serveTaxEvidence(p))

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

func serveFile(p FileServerParams, actor usecase.FileActorKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		documentID, err := strconv.ParseInt(r.PathValue("document_id"), 10, 64)
		if err != nil {
			writeNotFound(w)
			return
		}

		req := usecase.FileRequest{
			Actor:      actor,
			DocumentID: documentID,
			Style:      r.URL.Query().Get("style"),
		}
		switch actor {
		case usecase.FileActorBuyer:
			req.BuyerID = headerID(r, buyerIDHeader)
		case usecase.FileActorSeller:
			req.StoreID = headerID(r, storeIDHeader)
		case usecase.FileActorAdmin:
		}

		res, data, contentType, err := p.Files.Read(r.Context(), req)
		if err != nil {
			writeFileError(w, p.Logger, documentID, err)
			return
		}

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

func contentDisposition(r *http.Request, fileName string) string {
	kind := "attachment"
	if r.URL.Query().Get("disposition") == "inline" {
		kind = "inline"
	}
	return pkghttputil.ContentDisposition(kind, sanitizeFileName(fileName), "invoice.pdf")
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("\r", "", "\n", "", `"`, "", "\\", "")
	out := replacer.Replace(name)
	if out == "" {
		return "invoice.pdf"
	}
	return out
}

func headerID(r *http.Request, name string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(name)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

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
	log.Error("invoice.file.serve_failed", zap.Int64("document_id", documentID), zap.Error(err))
	writeEnvelope(w, http.StatusServiceUnavailable, 503, "invoice file temporarily unavailable")
}

func writeNotFound(w http.ResponseWriter) {
	writeEnvelope(w, http.StatusNotFound, notFoundBizCode, "文件不存在")
}

func writeEnvelope(w http.ResponseWriter, httpStatus int, code int32, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(httpStatus)
	_, _ = w.Write([]byte(`{"code":` + strconv.FormatInt(int64(code), 10) +
		`,"message":` + strconv.Quote(message) + `,"data":null}`))
}

func serveUnlockEvidence(p FileServerParams) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID, err := strconv.ParseInt(r.PathValue("job_id"), 10, 64)
		if err != nil {
			writeNotFound(w)
			return
		}
		unlockID, err := strconv.ParseInt(r.PathValue("unlock_id"), 10, 64)
		if err != nil {
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
