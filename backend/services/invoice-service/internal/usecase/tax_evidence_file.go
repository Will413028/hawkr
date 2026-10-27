package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// TaxEvidenceReader 是把稅籍佐證檔讀回位元組的出向面（實作在 client/media）。
//
// refID 是 seller_tax_profiles.id：它與 objectKey、refType 合起來是 media 的授權三元組，
// refType 由實作端固定，本層不需要知道。
type TaxEvidenceReader interface {
	ReadTaxEvidence(ctx context.Context, objectKey string, taxProfileID int64) (data []byte, contentType, filename string, err error)
}

// TaxEvidenceFile 是後台稅籍審核頁的佐證檔下載 usecase（plan 8f-1）。
//
// 授權述詞由 invoice 持有：該 store 的稅務資格存在，且在 document_id 的位置上有一份佐證。
// 取代已刪除的通用 admin 機敏下載（任何管理員以連號 asset ID 讀任何機敏檔、不驗歸屬）。
//
// 🔴 查無 store、查無稅務資格、位置不存在一律回同一個 9301，與 DocumentFile 同一個出口，
// 不讓呼叫端分辨「這個 store 有資格但沒有這份」與「沒有這個 store」。
type TaxEvidenceFile struct {
	issuers  domain.IssuerRepository
	profiles domain.SellerTaxProfileRepository
	audit    domain.AuditRepository
	media    TaxEvidenceReader
	now      func() time.Time
	log      *zap.Logger
}

// auditResourceTaxEvidence 是稅籍佐證讀取的稽核資源類型（resource_id ＝ 稅務資格 id）。
// invoice_admin_audit_logs.resource_type 是 varchar(32)、沒有 CHECK 限制值域，不需要 migration。
const auditResourceTaxEvidence = "INVOICE_TAX_EVIDENCE"

// TaxEvidenceContent 是下載結果。
type TaxEvidenceContent struct {
	Data        []byte
	ContentType string
	FileName    string
}

// NewTaxEvidenceFile 組出稅籍佐證檔下載 usecase。
func NewTaxEvidenceFile(
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	audit domain.AuditRepository,
	media TaxEvidenceReader,
	log *zap.Logger,
) *TaxEvidenceFile {
	return &TaxEvidenceFile{issuers: issuers, profiles: profiles, audit: audit, media: media, now: time.Now, log: log}
}

// Read 讀 storeID 的稅務資格上第 documentID 份佐證，actorAccountID 是讀取的管理員。
//
// 🔴 **先記存取紀錄、再向 media 讀 bytes，fail-closed**：稽核寫不進去就不送內容
// （同 recordPIIReveal 的裁定；證件影像是 P 類資料，「誰看了什麼」必須留痕）。
// actor 缺漏（header 沒注入）一律當作查無，不匿名放行。
//
// documentID 的定義與 GetTaxReview 的 TaxDocumentInfo.document_id 相同：
// evidence_object_keys 的 1 起算位置，順序來自 domain.SellerTaxProfile.EvidenceKeys。
func (u *TaxEvidenceFile) Read(ctx context.Context, actorAccountID, storeID, documentID int64) (TaxEvidenceContent, error) {
	if actorAccountID <= 0 || storeID <= 0 || documentID <= 0 {
		return TaxEvidenceContent{}, errDocumentNotFound()
	}

	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err != nil {
		return TaxEvidenceContent{}, taxEvidenceLookupError(err)
	}
	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil {
		return TaxEvidenceContent{}, taxEvidenceLookupError(err)
	}

	keys := profile.EvidenceKeys()
	if documentID > int64(len(keys)) {
		return TaxEvidenceContent{}, errDocumentNotFound()
	}
	objectKey := keys[documentID-1]
	if objectKey == "" {
		return TaxEvidenceContent{}, errDocumentNotFound()
	}

	if err := u.recordAccess(ctx, actorAccountID, profile.ID, documentID, objectKey); err != nil {
		return TaxEvidenceContent{}, err
	}

	data, contentType, fileName, err := u.media.ReadTaxEvidence(ctx, objectKey, profile.ID)
	if err != nil {
		if errors.Is(err, media.ErrAssetNotFound) {
			return TaxEvidenceContent{}, errDocumentNotFound()
		}
		u.log.Error("invoice.tax_evidence.media_read_failed",
			zap.Int64("store_id", storeID),
			zap.Int64("document_id", documentID),
			zap.Error(err))
		return TaxEvidenceContent{}, fmt.Errorf("invoice: read tax evidence: %w", err)
	}
	if len(data) == 0 {
		return TaxEvidenceContent{}, errDocumentNotFound()
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName == "" {
		fileName = "tax-evidence-" + strconv.FormatInt(documentID, 10)
	}
	return TaxEvidenceContent{Data: data, ContentType: contentType, FileName: fileName}, nil
}

// taxEvidenceLookupError 把兩段查無（issuer、profile）折成 9301，其餘錯誤原樣往上：
// 連線斷了不能被講成「文件不存在」（同 sellerIssuer 的規則）。
func taxEvidenceLookupError(err error) error {
	if isBizCode(err, bizcode.InvoiceProviderAccountNotFound) || isBizCode(err, bizcode.SellerTaxProfileNotFound) {
		return errDocumentNotFound()
	}
	return err
}

// recordAccess 留一列 INVOICE_PII_REVEAL 稽核（action 與 M13 告警共用）：誰、讀了哪個稅務資格的第幾份、哪個 objectKey。
// request_id 傳 nil：uq_invadmaudit_client_request 會把同一 ADM 的重複讀取摺成一列（見 recordPIIReveal）。
func (u *TaxEvidenceFile) recordAccess(ctx context.Context, actor, profileID, position int64, objectKey string) error {
	corr := interceptor.CorrelationIDFromContext(ctx)
	var corrPtr *string
	if corr != "" {
		corrPtr = &corr
	}
	if _, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
		ActorAccountID: actor,
		Action:         auditActionPIIReveal,
		ResourceType:   auditResourceTaxEvidence,
		ResourceID:     profileID,
		ReasonCode:     auditReasonRead,
		ReasonDetail: fmt.Sprintf("ADM 讀取稅籍佐證檔（plan 8f-1）：position=%d object_key=%s",
			position, objectKey),
		CorrelationID: corrPtr,
		Now:           u.now(),
	}); err != nil {
		u.log.Error("invoice.tax_evidence.audit.failed",
			zap.Int64("profile_id", profileID), zap.Int64("position", position), zap.Error(err))
		return fmt.Errorf("invoice: record tax evidence access: %w", err)
	}
	observeAdminPIIReveal()
	return nil
}
