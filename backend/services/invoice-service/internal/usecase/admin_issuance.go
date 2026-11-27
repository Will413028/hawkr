package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

const (
	manualIssuanceMaxLines   = 50
	manualIssuanceNameMax    = 60
	manualLineDescriptionMax = 200

	issuanceSourceOrderSnapshot = "ORDER_SNAPSHOT"
	issuanceSourceManualLines   = "MANUAL_LINES"

	sourceTypeManual  = "MANUAL"
	triggerTypeManual = "ADM_MANUAL"

	issuanceReasonAdminManual = "ADMIN_MANUAL_ISSUANCE"
	issuanceAuditAction       = "INVOICE_MANUAL_ISSUANCE"
)

var (
	ErrIssuanceConfirmRequired     = errors.New("invoice: this action requires confirmed=true")
	ErrIssuanceReasonRequired      = errors.New("invoice: reason is required")
	ErrIssuanceDocumentKindInvalid = errors.New("invoice: document_kind must be B2C, B2B or RECEIPT")
	ErrIssuanceSourceKindInvalid   = errors.New("invoice: source.kind must be ORDER_SNAPSHOT or MANUAL_LINES")
	ErrIssuanceLinesRequired       = errors.New("invoice: source.lines must contain 1 to 50 rows")
	ErrIssuanceLineInvalid         = errors.New("invoice: line amount must satisfy net_amount = quantity × unit_price")
	ErrIssuanceLineSignInvalid     = errors.New("invoice: DISCOUNT lines must be negative and all other lines non-negative")
	ErrIssuanceLineTypeInvalid     = errors.New("invoice: line_type must be PRODUCT, SHIPPING or DISCOUNT")
	// ErrIssuanceLineQuantityInvalid ＝ 數量不是正整數（chk_invline_qty，#3089 B 組第 6 項）。
	//
	// 🔴 在此之前這條 CHECK **全鏈路零業務守衛**（usecase／grpc／gateway／domain 四層皆無），
	// 實打 quantity="0" 或 "-1" 都是 **HTTP 500**。而 quantity="0" 正是真實誤操作的形狀
	// ——ADM 在數量格打 0。
	//
	// ⚠️ 它必須擋在 chk_invline_amount 與 chk_invline_sign **之前**：
	// quantity="-1" unitPrice="-100" netAmount="100" 會讓那兩道都通過
	// （乘法對得上、非折扣行金額為正），只有本道擋得住。
	ErrIssuanceLineQuantityInvalid = errors.New("invoice: quantity must be a positive integer")
	ErrIssuanceLineTaxInvalid      = errors.New("invoice: tax_category must be TAXABLE, ZERO_RATED or TAX_EXEMPT")
	ErrIssuanceRecipientTaxID      = errors.New("invoice: B2B recipient_tax_id must be 8 digits")
	ErrIssuanceRecipientNameLong   = errors.New("invoice: recipient name exceeds 60 characters")
	ErrIssuanceTotalNotPositive    = errors.New("invoice: total amount must be greater than zero")
	// ErrIssuanceHeaderNegative ＝ 表頭某個稅別的加總為負（chk_invdoc_amounts_nonneg）。
	//
	// 🔴 與 ErrIssuanceTotalNotPositive 分開是因為**總額為正時它仍可能成立**：
	// 折扣行與商品行稅別不同時，一個稅別為負、另一個為正，總額看起來完全正常。
	// 每一行單獨看也都合法（qty／amount／sign 三道全過）——只有表頭看得出來。
	ErrIssuanceHeaderNegative = errors.New("invoice: 折扣行的稅別必須與被折抵的商品行一致，否則單一稅別的銷售額會是負數")

	// ErrIssuanceOrderSnapshotUnavailable 是 ORDER_SNAPSHOT 分支的能力缺口。
	//
	// 🔴 它是**故障不是業務結果**（走 codes.Unimplemented，不是 envelope 的業務碼）：
	// ORD 的 GetOrderInvoiceSnapshotInternal 回傳裡沒有 `store_id` 也沒有 `buyer_id`
	// ⇒ 本服務無法驗「這張訂單的賣家 ＝ 請求裡的 issuer.storeId」（矩陣最後一列的 9346），
	// 也無法取得 I-47 歸屬驗證所需的 orders.buyer_id。
	// 拿請求裡 ADM 手填的 buyerId 代替是**明確禁止**的（I-47：buyer_id 只能取自訂單），
	// 那會讓 A11 變成一個可以把任何人的常用發票資料綁到任何訂單上的 IDOR 入口。
	// 缺口與所需的 proto 欄位登記在交付回報。
	ErrIssuanceOrderSnapshotUnavailable = errors.New(
		"invoice: ORDER_SNAPSHOT issuance needs order_no → (store_id, buyer_id) resolution, which OrderInvoiceSnapshotData does not carry")
)

// ManualIssuanceConfig 是端點 57 需要的環境值。
//
// PlatformTaxID ＝ AMEGO_PLATFORM_INVOICE。它是 issuer_tax_id_snapshot 的唯一來源
// （invoice_issuers 沒有統編欄，而 PLATFORM 沒有 seller_tax_profiles 那一列）。
type ManualIssuanceConfig struct {
	PlatformTaxID string
}

// AdminIssuance 是 A11 的寫入 usecase。
type AdminIssuance struct {
	tx       database.TxManager
	issuers  domain.IssuerRepository
	profiles domain.SellerTaxProfileRepository
	docs     domain.DocumentRepository
	ops      domain.OperationRepository
	audit    domain.AuditRepository
	cfg      ManualIssuanceConfig
	issueCfg IssuanceConfig

	newID func() int64
	now   func() time.Time
	log   *zap.Logger
}

// NewAdminIssuance 組出 A11 的寫入 usecase。
func NewAdminIssuance(
	tx database.TxManager,
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	audit domain.AuditRepository,
	cfg ManualIssuanceConfig,
	issueCfg IssuanceConfig,
	newID func() int64,
	log *zap.Logger,
) *AdminIssuance {
	return &AdminIssuance{
		tx: tx, issuers: issuers, profiles: profiles, docs: docs, ops: ops, audit: audit,
		cfg: cfg, issueCfg: issueCfg, newID: newID,
		now: func() time.Time { return time.Now().UTC() },
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *AdminIssuance) WithClock(now func() time.Time) *AdminIssuance {
	if now != nil {
		u.now = now
	}
	return u
}

// ManualIssuanceIssuer／Recipient／Source 是端點 57 的三段輸入（proto 同名 message）。
type ManualIssuanceIssuer struct {
	Kind    string // SELLER / PLATFORM
	StoreID int64  // Kind=SELLER 時必填
}

type ManualIssuanceRecipient struct {
	Kind         string // BUYER / SELLER / EXTERNAL
	BuyerID      int64
	StoreID      int64
	Name         string
	TaxID        string
	Email        string
	CarrierType  string
	CarrierValue string
}

type ManualIssuanceLine struct {
	LineType    string
	Name        string
	Quantity    int32
	UnitPrice   int64
	NetAmount   int64
	TaxCategory string // InvoiceTaxCategory 的 **Code**（TAXABLE/ZERO_RATED/TAX_EXEMPT）
}

type ManualIssuanceSource struct {
	Kind    string
	OrderNo string
	Lines   []ManualIssuanceLine
}

// ManualIssuanceInput 是端點 57 的輸入。
type ManualIssuanceInput struct {
	ActorAccountID int64
	Issuer         ManualIssuanceIssuer
	Recipient      ManualIssuanceRecipient
	Source         ManualIssuanceSource
	DocumentKind   string
	Reason         string
	Confirmed      bool
	IdempotencyKey string
}

// ManualIssuanceResult 是端點 57 的輸出（proto CreateManualIssuanceForAdminData）。
//
// Duplicate=true ⇒ **HTTP 200 ＋ 9347 ＋ data.documentId**（§6.1.15）：同一份請求內容
// 已經建過文件，把既有那一份指回去，不是再開一張。
type ManualIssuanceResult struct {
	JobID      int64
	DocumentID int64
	Status     string
	AuditLogID int64
	Duplicate  bool
}

type issuanceCombo struct {
	issuer    string
	recipient string
	source    string
}

var allowedIssuanceCombos = []issuanceCombo{
	{codemeta.InvoiceIssuerTypeSeller.Value, codemeta.InvoiceRecipientTypeBuyer.Value, issuanceSourceOrderSnapshot},
	{codemeta.InvoiceIssuerTypeSeller.Value, codemeta.InvoiceRecipientTypeExternal.Value, issuanceSourceManualLines},
	{codemeta.InvoiceIssuerTypePlatform.Value, codemeta.InvoiceRecipientTypeSeller.Value, issuanceSourceManualLines},
	{codemeta.InvoiceIssuerTypePlatform.Value, codemeta.InvoiceRecipientTypeExternal.Value, issuanceSourceManualLines},
}

// Create 手動開立一張發票／收據（端點 57）。
func (u *AdminIssuance) Create(ctx context.Context, in ManualIssuanceInput) (ManualIssuanceResult, error) {
	if !in.Confirmed {
		return ManualIssuanceResult{}, ErrIssuanceConfirmRequired
	}
	if isBlank(in.Reason) {
		return ManualIssuanceResult{}, ErrIssuanceReasonRequired
	}
	if utf8.RuneCountInString(in.Reason) > adminReasonMaxRunes {
		return ManualIssuanceResult{}, ErrJobReasonTooLong
	}
	if _, ok := codemeta.InvoiceDocumentKindFromValue(in.DocumentKind); !ok {
		return ManualIssuanceResult{}, ErrIssuanceDocumentKindInvalid
	}
	if in.Source.Kind != issuanceSourceOrderSnapshot && in.Source.Kind != issuanceSourceManualLines {
		return ManualIssuanceResult{}, ErrIssuanceSourceKindInvalid
	}
	if !isAllowedIssuanceCombo(in) {
		return ManualIssuanceResult{}, jobBiz(bizcode.InvoiceIssuanceTargetInvalid)
	}
	if in.Source.Kind == issuanceSourceOrderSnapshot {
		return ManualIssuanceResult{}, ErrIssuanceOrderSnapshotUnavailable
	}

	lines, err := buildManualLines(in.Source.Lines)
	if err != nil {
		return ManualIssuanceResult{}, err
	}

	var out ManualIssuanceResult
	err = u.tx.WithTx(ctx, func(ctx context.Context) error {
		now := u.now()

		issuer, issuerName, issuerTaxID, err := u.resolveIssuer(ctx, in.Issuer)
		if err != nil {
			return err
		}
		recipientName, recipientTaxID, recipientRefID, err := u.resolveRecipient(ctx, in)
		if err != nil {
			return err
		}

		header := domain.BuildHeader(lines, recipientTaxID)
		if header.TotalAmount <= 0 {
			return ErrIssuanceTotalNotPositive
		}
		if header.SalesAmount < 0 || header.FreeTaxSalesAmount < 0 ||
			header.ZeroTaxSalesAmount < 0 || header.TaxAmount < 0 {
			return ErrIssuanceHeaderNegative
		}

		requestNo := manualProviderRequestNo(issuer, in)
		if existing, err := u.docs.GetByProviderRequestNo(ctx, requestNo); err == nil {
			out = ManualIssuanceResult{
				DocumentID: existing.ID, Status: existing.Status, Duplicate: true,
			}
			return nil
		} else if !isBizCode(err, bizcode.InvoiceDocumentNotFound) {
			return err
		}

		doc, err := u.buildDocument(ctx, in, issuer, issuerName, issuerTaxID,
			recipientName, recipientTaxID, recipientRefID, header, requestNo, now)
		if err != nil {
			return err
		}

		created, inserted, err := u.docs.Create(ctx, doc)
		if err != nil {
			return err
		}
		if !inserted {
			existing, err := u.docs.GetByProviderRequestNo(ctx, requestNo)
			if err != nil {
				return errManualIssuanceIndexDrift
			}
			out = ManualIssuanceResult{
				DocumentID: existing.ID, Status: existing.Status, Duplicate: true,
			}
			return nil
		}

		if err := u.docs.InsertLines(ctx, created.ID, toNewLines(lines), now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectDocument, created.ID, "", created.Status,
			issuanceReasonAdminManual, in.Reason, in.ActorAccountID, now); err != nil {
			return err
		}

		opID, opStatus, err := u.createOperation(ctx, created, now, in)
		if err != nil {
			return err
		}

		auditID, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
			ActorAccountID: in.ActorAccountID,
			Action:         issuanceAuditAction,
			ResourceType:   docResourceType,
			ResourceID:     created.ID,
			AfterState: jobAuditState(created.Status, map[string]string{
				"documentKind":  created.DocumentKind,
				"issuerKind":    in.Issuer.Kind,
				"recipientKind": in.Recipient.Kind,
				"totalAmount":   strconv.FormatInt(created.TotalAmount, 10),
				"jobId":         strconv.FormatInt(opID, 10),
			}),
			ReasonCode:   issuanceReasonAdminManual,
			ReasonDetail: in.Reason,
			RequestID:    nilIfEmpty(in.IdempotencyKey),
			Now:          now,
		})
		if err != nil {
			return err
		}

		u.logger().Info("invoice.admin.issuance.created",
			zap.Int64("document_id", created.ID), zap.Int64("job_id", opID),
			zap.String("issuer_kind", in.Issuer.Kind), zap.String("recipient_kind", in.Recipient.Kind),
			zap.String("document_kind", created.DocumentKind), zap.Int64("actor_account_id", in.ActorAccountID))
		out = ManualIssuanceResult{
			JobID: opID, DocumentID: created.ID, Status: opStatus, AuditLogID: auditID,
		}
		return nil
	})
	if err != nil {
		return ManualIssuanceResult{}, err
	}
	return out, nil
}

func (u *AdminIssuance) resolveIssuer(ctx context.Context, in ManualIssuanceIssuer) (domain.Issuer, string, string, error) {
	switch in.Kind {
	case codemeta.InvoiceIssuerTypeSeller.Value:
		issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, in.StoreID)
		if err != nil {
			return domain.Issuer{}, "", "", err
		}
		profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return domain.Issuer{}, "", "", err
		}
		return issuer, issuerNameSnapshot(issuer, profile), derefString(profile.TaxID), nil

	case codemeta.InvoiceIssuerTypePlatform.Value:
		if u.cfg.PlatformTaxID == "" {
			return domain.Issuer{}, "", "", pkgerrors.NewBusiness(
				bizcode.InvoiceIssuanceTargetInvalid,
				"平台 issuer 未啟用（AMEGO_PLATFORM_INVOICE 未設定），無法以平台名義開立")
		}
		issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypePlatform.Value, 0)
		if err != nil {
			return domain.Issuer{}, "", "", err
		}
		return issuer, issuer.DisplayName, u.cfg.PlatformTaxID, nil
	}
	return domain.Issuer{}, "", "", jobBiz(bizcode.InvoiceIssuanceTargetInvalid)
}

func (u *AdminIssuance) resolveRecipient(ctx context.Context, in ManualIssuanceInput) (string, string, *int64, error) {
	name := in.Recipient.Name
	taxID := in.Recipient.TaxID
	var refID *int64

	switch in.Recipient.Kind {
	case codemeta.InvoiceRecipientTypeSeller.Value:
		issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, in.Recipient.StoreID)
		if err != nil {
			return "", "", nil, err
		}
		profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
		if err != nil {
			return "", "", nil, err
		}
		name = firstNonEmpty(name, derefString(profile.LegalName))
		taxID = firstNonEmpty(taxID, derefString(profile.TaxID))
		storeID := in.Recipient.StoreID
		refID = &storeID

	case codemeta.InvoiceRecipientTypeBuyer.Value:
		buyerID := in.Recipient.BuyerID
		refID = &buyerID

	case codemeta.InvoiceRecipientTypeExternal.Value:
	}

	if utf8.RuneCountInString(name) > manualIssuanceNameMax {
		return "", "", nil, ErrIssuanceRecipientNameLong
	}
	if in.DocumentKind == codemeta.InvoiceDocumentKindB2b.Value {
		if !isEightDigits(taxID) {
			return "", "", nil, ErrIssuanceRecipientTaxID
		}
	} else {
		taxID = domain.B2CBuyerIdentifier
	}
	return recipientName(in.DocumentKind, name), taxID, refID, nil
}

func (u *AdminIssuance) buildDocument(
	ctx context.Context, in ManualIssuanceInput,
	issuer domain.Issuer, issuerName, issuerTaxID string,
	recipientName, recipientTaxID string, recipientRefID *int64,
	header domain.Header, requestNo string, now time.Time,
) (domain.NewDocument, error) {
	isReceipt := in.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value

	mode := codemeta.InvoiceFulfillmentModeAmegoAuto.Value
	provider := providerAmego
	receiptNo := ""
	if isReceipt {
		mode = codemeta.InvoiceFulfillmentModePlatformReceipt.Value
		provider = ""
		no, err := u.docs.NextReceiptNo(ctx)
		if err != nil {
			return domain.NewDocument{}, err
		}
		receiptNo = no
	}

	invoiceDate := domain.InvoiceDateTaipei(now)
	var deadline *time.Time
	if d, ok := domain.StatutoryDeadline(invoiceDate, in.DocumentKind); ok {
		deadline = &d
	}

	snapshot, err := json.Marshal(map[string]string{
		"issuerKind":    in.Issuer.Kind,
		"recipientKind": in.Recipient.Kind,
		"sourceKind":    in.Source.Kind,
		"trigger":       triggerTypeManual,
		"reason":        truncateRunes(in.Reason, adminReasonMaxRunes),
	})
	if err != nil {
		return domain.NewDocument{}, err
	}

	return domain.NewDocument{
		ID:                  u.newID(),
		IssuerID:            issuer.ID,
		IssuerNameSnapshot:  issuerName,
		IssuerTaxIDSnapshot: issuerTaxID,
		SourceType:          sourceTypeManual,
		TriggerType:         triggerTypeManual,
		RecipientType:       in.Recipient.Kind,
		RecipientRefID:      recipientRefID,
		RecipientName:       recipientName,
		RecipientTaxID:      recipientTaxID,
		RecipientEmail:      in.Recipient.Email,
		DocumentKind:        in.DocumentKind,
		FulfillmentMode:     mode,
		Status:              codemeta.InvoiceDocumentStatusPending.Value,
		SnapshotVersion:     1,
		IssueAttemptSeq:     1,

		SalesAmount:        header.SalesAmount,
		FreeTaxSalesAmount: header.FreeTaxSalesAmount,
		ZeroTaxSalesAmount: header.ZeroTaxSalesAmount,
		TaxAmount:          header.TaxAmount,
		TotalAmount:        header.TotalAmount,
		TaxType:            int16(headerTaxType(header)),
		TaxRateBps:         domain.TaxRateBps,

		Provider:                    provider,
		ProviderRequestNo:           requestNo,
		ReceiptNo:                   receiptNo,
		ReceiptAuthorizationVersion: receiptAuthNoneV1,

		InvoiceDateTaipei:   &invoiceDate,
		TaxPeriodCode:       taxPeriodFor(in.DocumentKind, invoiceDate),
		IssueNotBefore:      now,
		StatutoryDeadlineAt: deadline,
		SourceSnapshot:      snapshot,
		Now:                 now,
	}, nil
}

func (u *AdminIssuance) createOperation(
	ctx context.Context, doc domain.Document, now time.Time, in ManualIssuanceInput,
) (int64, string, error) {
	opID, status, _, err := createAndPromoteIssueOperation(ctx, u.ops, issueOperationSpec{
		OperationID:         u.newID(),
		DocumentID:          doc.ID,
		IssueAttemptSeq:     1,
		ProviderRequestNo:   derefString(doc.ProviderRequestNo),
		RequestSnapshotHash: "manual:" + strconv.FormatInt(doc.ID, 10),
		MaxAttempts:         u.issueCfg.MaxAttempts,
		NextAttemptAt:       now,
		StatutoryDeadlineAt: doc.StatutoryDeadlineAt,
		Now:                 now,
	}, func(ctx context.Context, operationID int64, from, to string) error {
		return u.appendHistory(ctx, subjectOperation, operationID, from, to,
			issuanceReasonAdminManual, in.Reason, in.ActorAccountID, now)
	})
	if err != nil {
		return 0, "", err
	}
	return opID, status, nil
}

func (u *AdminIssuance) appendHistory(
	ctx context.Context, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail string, actorAccountID int64, now time.Time,
) error {
	return u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		FromStatus:     nilIfEmpty(from),
		ToStatus:       to,
		ReasonCode:     nilIfEmpty(reasonCode),
		ReasonDetail:   nilIfEmpty(reasonDetail),
		ActorType:      jobActorAdmin,
		ActorAccountID: &actorAccountID,
		Now:            now,
	})
}

func isAllowedIssuanceCombo(in ManualIssuanceInput) bool {
	for _, c := range allowedIssuanceCombos {
		if c.issuer == in.Issuer.Kind && c.recipient == in.Recipient.Kind && c.source == in.Source.Kind {
			return true
		}
	}
	return false
}

func buildManualLines(in []ManualIssuanceLine) ([]domain.Line, error) {
	if len(in) == 0 || len(in) > manualIssuanceMaxLines {
		return nil, ErrIssuanceLinesRequired
	}
	out := make([]domain.Line, 0, len(in))
	for i, l := range in {
		if _, ok := codemeta.InvoiceLineTypeFromValue(l.LineType); !ok {
			return nil, ErrIssuanceLineTypeInvalid
		}
		cat, ok := codemeta.InvoiceTaxCategoryFromCode(l.TaxCategory)
		if !ok {
			return nil, ErrIssuanceLineTaxInvalid
		}
		catValue, err := strconv.Atoi(cat.Value)
		if err != nil {
			return nil, ErrIssuanceLineTaxInvalid
		}
		if l.Quantity <= 0 {
			return nil, ErrIssuanceLineQuantityInvalid
		}
		if l.NetAmount != int64(l.Quantity)*l.UnitPrice {
			return nil, ErrIssuanceLineInvalid
		}
		isDiscount := l.LineType == codemeta.InvoiceLineTypeDiscount.Value
		if isDiscount != (l.NetAmount < 0) {
			return nil, ErrIssuanceLineSignInvalid
		}
		line := domain.Line{
			LineSeq:     int32(i + 1),
			LineType:    l.LineType,
			Description: truncateRunes(l.Name, manualLineDescriptionMax),
			Quantity:    l.Quantity,
			UnitPrice:   l.UnitPrice,
			NetAmount:   l.NetAmount,
			TaxCategory: domain.TaxCategory(catValue), //nolint:gosec // 值域由 CodeMeta 守
		}
		if !isDiscount {
			line.AllowanceableBase = line.NetAmount
		}
		out = append(out, line)
	}
	return out, nil
}

func manualProviderRequestNo(issuer domain.Issuer, in ManualIssuanceInput) string {
	canonical, _ := json.Marshal(struct {
		DocumentKind string                  `json:"documentKind"`
		Recipient    ManualIssuanceRecipient `json:"recipient"`
		Source       ManualIssuanceSource    `json:"source"`
	}{in.DocumentKind, in.Recipient, in.Source})
	sum := sha256.Sum256(canonical)
	return "MAN-" + strconv.FormatInt(issuer.ID, 10) + "-" + hex.EncodeToString(sum[:])[:16]
}

func isEightDigits(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var errManualIssuanceIndexDrift = errors.New(
	"invoice: manual issuance insert was rejected by a unique index but no document was found for the same request_no")

func (u *AdminIssuance) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}
