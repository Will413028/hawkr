package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 發票 PDF 授權下載（58 §6.1.4 的三條二進位子路由 ＋ 端點 10／70 的中介資料）
//
// 🔴 **本檔的存在理由是授權，不是取檔。** 取檔本身只有兩行（讀 pdf_object_key、
// 向 media 換 bytes）；難的是「誰能拿哪一份」。這條路徑在 2026-09-02 之前是完全
// 壞掉的（三條路徑分別回 501／503／404，#3008），而「壞掉」讓越權問題看起來不存在。
// 修好取檔卻沒同時補歸屬判定，會從「都拿不到」變成「誰都拿得到」——而發票 PDF 上
// 印著完整的買受人姓名、統編與金額。
//
// 🔴 **越權與不存在必須走同一個出口**（§6.1.4「這條通道上必做的四件事」第 1 條）：
// 兩者可分辨時，這條路由就變成一台可枚舉的 oracle——攻擊者拿一串 document_id 打過去，
// 403 與 404 的差別就是「這張發票存在」。所以本檔**沒有任何一條路徑回 403**。
// ---------------------------------------------------------------------------

// FileActorKind 是三個合法讀者面。值域刻意封閉：新增第四個讀者要回來改這裡，
// 而不是在某個 handler 裡多寫一個 if。
type FileActorKind int

const (
	// FileActorBuyer：述詞 ＝ 這份文件的 recipient_ref_id 是這位買家。
	FileActorBuyer FileActorKind = iota
	// FileActorSeller：述詞 ＝ 這份文件的 issuer 解析出來的 store_id 是這位賣家。
	FileActorSeller
	// FileActorAdmin：ADM 沒有 ownership 概念（§6.1.4 的路由表逐字），
	// 授權在 gateway 的 admin group ⇒ 這裡只驗文件存在。
	FileActorAdmin
)

// FileRequest 是一次取檔請求。
type FileRequest struct {
	Actor      FileActorKind
	DocumentID int64
	// BuyerID／StoreID 依 Actor 二選一。**缺值即 fail-closed**：
	// 反代路徑上這兩個值來自 gateway 注入的 header，注入失敗時 header 會是空的，
	// 而空值配上「沒有述詞就不篩」的實作等於全部放行（§6.1.4 逐字要求 fail-closed）。
	BuyerID int64
	StoreID int64
	// Style 是 PDF 版面。空字串 ＝ 用文件自己算出來的預設。
	Style string
}

// FileResult 是取檔結果的中介資料（端點 10／70 回這個，不回 bytes）。
type FileResult struct {
	DocumentID int64
	ObjectKey  string
	FileName   string
	Style      string
}

// SecureAssetReader 是把機敏檔讀回位元組的出向面（實作在 client/media）。
//
// 介面宣告在消費端：本層只需要「給我 key 與 refID，還我 bytes」，
// 不需要知道 media 的 ref_type 值域，也不該知道它是 gRPC 還是 HTTP。
type SecureAssetReader interface {
	ReadSecure(ctx context.Context, objectKey, refID string) (data []byte, contentType, filename string, err error)
}

// DocumentFile 是 PDF 授權下載 usecase。
//
// 它刻意**沒有** TxManager 與 ProviderClient：下載不寫資料、也不打 AMEGO
// （AMEGO 的 file_url 只在 INV.pdf 抓檔那條路徑上出現，且絕不外流——§6.1.4 鐵則 1）。
type DocumentFile struct {
	documents domain.DocumentRepository
	media     SecureAssetReader
	now       func() time.Time
	log       *zap.Logger
}

// NewDocumentFile 組出 PDF 授權下載 usecase。
func NewDocumentFile(
	documents domain.DocumentRepository,
	media SecureAssetReader,
	log *zap.Logger,
) *DocumentFile {
	return &DocumentFile{documents: documents, media: media, now: time.Now, log: log}
}

// allowedStyles 是 §6.1.4 鐵則 3 的白名單。
//
// 🔴 AMEGO 對 download_style **無值域驗證**（傳 9 仍回 code:0 並產出 URL）
// ⇒ 我方必須自驗。這裡是「往下傳之前」那一道。
var allowedStyles = map[string]struct{}{
	"0": {}, "1": {}, "2": {}, "3": {}, "5": {},
}

// Resolve 做完整條授權判定並回中介資料。**不讀 bytes。**
//
// 端點 10／70（JSON）與三條 /file/content（bytes）共用它，理由是那兩條路徑對
// 「誰能拿」的答案必須逐字相同：分兩份實作時，其中一份長胖就會出現
// 「列表說可以下載、下載說沒有這份」或者更糟的反向。
func (u *DocumentFile) Resolve(ctx context.Context, req FileRequest) (FileResult, error) {
	doc, err := u.authorize(ctx, req)
	if err != nil {
		return FileResult{}, err
	}

	// 🔴 fileAvailable=false 的文件在這裡就要停，且回**業務碼**而不是 bytes
	// （§6.1.4 第 4 條）。9312 是正常業務分支不是錯誤：載具發票中獎前我方拿不到 PDF。
	if doc.PdfObjectKey == nil || *doc.PdfObjectKey == "" {
		return FileResult{}, fileUnavailableError(doc, u.reasonFor(doc))
	}

	style := req.Style
	if style == "" {
		style = strconv.Itoa(downloadStyleFor(doc))
	}
	if _, ok := allowedStyles[style]; !ok {
		return FileResult{}, ErrInvalidFileStyle
	}

	return FileResult{
		DocumentID: doc.ID,
		ObjectKey:  *doc.PdfObjectKey,
		FileName:   fileNameFor(doc),
		Style:      style,
	}, nil
}

// Read 做完 Resolve 之後把 bytes 取回來。三條 /file/content 用它。
func (u *DocumentFile) Read(ctx context.Context, req FileRequest) (FileResult, []byte, string, error) {
	res, err := u.Resolve(ctx, req)
	if err != nil {
		return FileResult{}, nil, "", err
	}
	data, contentType, name, err := u.media.ReadSecure(ctx, res.ObjectKey, strconv.FormatInt(res.DocumentID, 10))
	if err != nil {
		// M15：**只在 media 明確說「這份資產不存在」時**遞增，不是任何讀取失敗都算。
		// 它守的是 §8.1.5(e) 的 asset_reference 有沒有真的建立——沒建的話 GC 會在
		// 7 天後刪掉實體，而 AMEGO 的 invoice_file 只查得到 180 天內。把 media 當機
		// 一起算進去，這條線索就會被雜訊蓋掉；只算不存在，它每一次遞增都是真的缺件。
		if errors.Is(err, media.ErrAssetNotFound) {
			observeDocumentPDFMissing()
		}
		// media 取不到**不能**折成 404：那會把「我方系統故障」講成「你這張發票不存在」，
		// 而買家的下一步完全不同（前者是稍後再試，後者是去申訴）。
		u.log.Error("invoice.document_file.media_read_failed",
			zap.Int64("document_id", res.DocumentID),
			zap.String("object_key", res.ObjectKey),
			zap.Error(err))
		return FileResult{}, nil, "", fmt.Errorf("invoice: read secure asset: %w", err)
	}
	if contentType == "" {
		contentType = pdfContentType
	}
	if name != "" {
		res.FileName = name
	}
	return res, data, contentType, nil
}

// authorize 依 actor 走各自的述詞，**述詞寫在 SQL 裡**（GetForBuyerByID／GetForSeller）。
//
// 🔴 三條分支的失敗一律折成同一個 ErrNotFound：見本檔頂端的紅線。
func (u *DocumentFile) authorize(ctx context.Context, req FileRequest) (domain.Document, error) {
	if req.DocumentID == 0 {
		return domain.Document{}, errDocumentNotFound()
	}
	switch req.Actor {
	case FileActorBuyer:
		if req.BuyerID == 0 {
			// fail-closed：gateway 的 InjectBuyerIDHeader 失敗時 header 是空的。
			// 「沒有述詞就不篩」在這裡等於把每一張發票的 PDF 開放給任何登入者。
			return domain.Document{}, errDocumentNotFound()
		}
		doc, err := u.documents.GetForBuyerByID(ctx, req.DocumentID, req.BuyerID)
		if err != nil {
			return domain.Document{}, mapToNotFound(err)
		}
		return doc, nil
	case FileActorSeller:
		if req.StoreID == 0 {
			return domain.Document{}, errDocumentNotFound()
		}
		item, err := u.documents.GetForSeller(ctx, req.DocumentID, req.StoreID)
		if err != nil {
			return domain.Document{}, mapToNotFound(err)
		}
		return item.Document, nil
	case FileActorAdmin:
		doc, err := u.documents.GetByID(ctx, req.DocumentID)
		if err != nil {
			return domain.Document{}, mapToNotFound(err)
		}
		return doc, nil
	default:
		// 值域長胖時 fail-closed。放行的後果是新讀者面預設無述詞。
		return domain.Document{}, errDocumentNotFound()
	}
}

// reasonFor 與 Query.FileAvailability 的判準保持一致（載具未中獎／逾查詢期限／其餘）。
//
// ⚠️ 這裡**不能**直接呼叫 Query.FileAvailability：Query 的建構子有 15 個依賴，
// 為了一個純函式把它整包拉進來，會讓下載路徑跟著 ADM 稽核、商業日鏡像一起起停。
// 兩份判準要一致靠的是 documentFileUnavailableReason 這支共用純函式。
func (u *DocumentFile) reasonFor(doc domain.Document) string {
	return documentFileUnavailableReason(doc, u.now())
}

// fileNameFor 是下載時的檔名：以發票號為主，退回 document id。
func fileNameFor(doc domain.Document) string {
	if doc.ProviderInvoiceNo != nil && *doc.ProviderInvoiceNo != "" {
		return "invoice-" + *doc.ProviderInvoiceNo + ".pdf"
	}
	if doc.ReceiptNo != nil && *doc.ReceiptNo != "" {
		return "receipt-" + *doc.ReceiptNo + ".pdf"
	}
	return "invoice-" + strconv.FormatInt(doc.ID, 10) + ".pdf"
}

// fileUnavailableError 把三個不可用理由折成對應的業務碼。
//
// 9312／9313 是**正常業務分支**（§6.1.4 鐵則 4），不是系統錯誤；
// 其餘（尚未抓檔、不適用）走 9301 —— 對使用者的意思同樣是「現在沒有這份檔案」。
func fileUnavailableError(doc domain.Document, reason string) error {
	switch reason {
	case FileUnavailableCarrierNotWon:
		return pkgerrors.NewBusiness(bizcode.InvoicePdfUnavailableCarrier,
			"載具發票於中獎前不提供發票檔案下載")
	case FileUnavailableExpired:
		return pkgerrors.NewBusiness(bizcode.InvoicePdfExpired,
			"已超過發票檔案的查詢期限")
	default:
		_ = doc
		return errDocumentNotFound()
	}
}

func errDocumentNotFound() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceDocumentNotFound, "文件不存在")
}

// ErrInvalidFileStyle ＝ style 不在 §6.1.4 鐵則 3 的白名單內。
//
// 做成 sentinel 而不是業務碼：它是**參數格式**錯誤（400），而業務碼的出口是
// 200 ＋ envelope。兩個邊界（gRPC 與 /file/content）各自把它折成自己的 400。
var ErrInvalidFileStyle = errors.New("invoice: download style is outside the whitelist {0,1,2,3,5}")

// mapToNotFound 保證「查無列」與「述詞不符」走同一個出口。
//
// repository 的三支授權讀取（GetForBuyerByID／GetForSeller／GetByID）已經把述詞寫在
// SQL 裡，兩種情況本來就同樣回 9301 ⇒ 這裡多數時候是直通。留著它是為了讓
// 「不可分辨」這件事在**本層**也看得見：日後有人換成先查再比對，這一行會提醒他
// 兩條路徑必須匯流。
//
// 非 9301 的錯誤（連線失敗等）原樣往上拋：把它們也折成 404 會讓一次 DB 斷線被
// 講成「你的發票不見了」，而那兩者的下一步完全不同。
func mapToNotFound(err error) error {
	if isBizCode(err, bizcode.InvoiceDocumentNotFound) {
		return errDocumentNotFound()
	}
	return err
}
