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
		if errors.Is(err, media.ErrAssetNotFound) {
			observeDocumentPDFMissing()
		}
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

func (u *DocumentFile) authorize(ctx context.Context, req FileRequest) (domain.Document, error) {
	if req.DocumentID == 0 {
		return domain.Document{}, errDocumentNotFound()
	}
	switch req.Actor {
	case FileActorBuyer:
		if req.BuyerID == 0 {
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
		return domain.Document{}, errDocumentNotFound()
	}
}

func (u *DocumentFile) reasonFor(doc domain.Document) string {
	return documentFileUnavailableReason(doc, u.now())
}

func fileNameFor(doc domain.Document) string {
	if doc.ProviderInvoiceNo != nil && *doc.ProviderInvoiceNo != "" {
		return "invoice-" + *doc.ProviderInvoiceNo + ".pdf"
	}
	if doc.ReceiptNo != nil && *doc.ReceiptNo != "" {
		return "receipt-" + *doc.ReceiptNo + ".pdf"
	}
	return "invoice-" + strconv.FormatInt(doc.ID, 10) + ".pdf"
}

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

func mapToNotFound(err error) error {
	if isBizCode(err, bizcode.InvoiceDocumentNotFound) {
		return errDocumentNotFound()
	}
	return err
}
