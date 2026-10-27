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

// 本檔是 A11 手動開票（§6.1.15 端點 57）——A2 通用 issuer→recipient 模型在 Phase 1 的
// 唯一可操作入口。
//
// 🔴 **這是一個不可逆的對外動作**：開出去的發票收不回來（只能作廢重開，而作廢本身也會
// 進國稅局）。全檔因此採 deny-by-default：
//
//	① 合法組合矩陣**正面列舉**（§6.1.15 的四列 ✅），不在表上的一律 9346。
//	   反過來寫（列出禁止組合）會讓日後新增的 issuer/recipient 值自動變成允許。
//	② 任何一個欄位取不到就**回錯誤**，絕不用零值往下走。設計書刻意否決了「回零值繼續」：
//	   那會開出一張沒有品項、金額 0 的發票——而它一樣是一張真的發票。
//	③ `confirmed=true` 是必填的二次確認；缺了是 400，不是「預設為 true」。
//	④ 冪等鍵落進 `provider_request_no`（partial UNIQUE），重複請求回 **9347 ＋ 既有
//	   documentId**，不是再開一張。
//	⑤ 明細行的三個 CHECK（net = qty × price、DISCOUNT 為負、其餘非負）在本層先擋：
//	   讓 DB 擋會得到一句 23514，ADM 看到的是 500 而不是「第 3 行金額對不起來」。
//
// 建立不發任何事件（§6.3 沒有 invoice.document.created）；文件落 PENDING ＋ operation
// 落 READY 之後交給 INV.op，端點 57 自己**不打 AMEGO**（§6.4 的 deadline 表逐字）。

const (
	// manualIssuanceMaxLines 是 MANUAL_LINES 的行數上限（§6.1 proto 逐字「1–50 行」）。
	manualIssuanceMaxLines = 50
	// manualIssuanceNameMax 對齊 invoice_documents.recipient_name varchar(60)。
	manualIssuanceNameMax = 60
	// manualLineDescriptionMax 對齊 invoice_document_lines.description 的長度。
	manualLineDescriptionMax = 200

	// 端點 57 的 source.kind 值域（兩值不多不少）。
	issuanceSourceOrderSnapshot = "ORDER_SNAPSHOT"
	issuanceSourceManualLines   = "MANUAL_LINES"

	// invoice_documents.source_type／trigger_type 的手動軸。
	sourceTypeManual  = "MANUAL"
	triggerTypeManual = "ADM_MANUAL"

	// invoice_status_history.reason_code ／ invoice_admin_audit_logs.action。
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

// issuanceCombo 是合法組合矩陣的一列（§6.1.15 的 ✅ 四列，正面列舉）。
type issuanceCombo struct {
	issuer    string
	recipient string
	source    string
}

// allowedIssuanceCombos 逐列抄自 §6.1.15 的矩陣。
//
// 🔴 **正面列舉不得改成「列出禁止組合」**：後者會讓日後新增的 issuer／recipient 值
// 自動落進允許集合，而那是一個沒有人會察覺的放寬（§C 的 I-28 同一條規則）。
var allowedIssuanceCombos = []issuanceCombo{
	// 補開一筆漏開的訂單發票
	{codemeta.InvoiceIssuerTypeSeller.Value, codemeta.InvoiceRecipientTypeBuyer.Value, issuanceSourceOrderSnapshot},
	// 賣家對非會員買家開票（線下交易）
	{codemeta.InvoiceIssuerTypeSeller.Value, codemeta.InvoiceRecipientTypeExternal.Value, issuanceSourceManualLines},
	// 平台服務費
	{codemeta.InvoiceIssuerTypePlatform.Value, codemeta.InvoiceRecipientTypeSeller.Value, issuanceSourceManualLines},
	// 平台對外部對象開票
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
		// 矩陣允許它，但本服務今天湊不齊必要事實 ⇒ 回錯誤而不是用零值往下走。
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
			// 零元或負數總額的發票不是「一張金額 0 的發票」，是一張**不該存在**的發票。
			// 自動線把零元訂單判成 NOT_APPLICABLE，手動線沒有那條分支 ⇒ 在這裡擋。
			return ErrIssuanceTotalNotPositive
		}
		// 🔴 **總額為正不代表每一欄都非負**（chk_invdoc_amounts_nonneg）。
		//
		// DISCOUNT 行的 tax_category 與商品行不同時，某一個稅別的加總會是負的
		// 而總額仍為正：例如 TAX_EXEMPT 商品 1000 ＋ TAXABLE 折扣 -500
		// ⇒ sales_amount=-500、free_tax=1000、total=500 > 0
		// ⇒ 上面那道放行，落庫當場 23514 ⇒ ADM 看到 HTTP 500。
		// 每一行單獨看都合法（qty／amount／sign 三道全過），只有表頭看得出來。
		if header.SalesAmount < 0 || header.FreeTaxSalesAmount < 0 ||
			header.ZeroTaxSalesAmount < 0 || header.TaxAmount < 0 {
			return ErrIssuanceHeaderNegative
		}

		requestNo := manualProviderRequestNo(issuer, in)
		// 冪等：provider_request_no 上有 partial UNIQUE ⇒ 同樣內容的第二次請求會被
		// ON CONFLICT DO NOTHING 擋下，回既有那一份（9347）而不是再開一張。
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
			// 守衛與 INSERT 之間的窄窗口（另一個分頁同時送出）。唯一索引擋下它 ⇒ 走與
			// 上面同一個出口；查不到就是述詞與索引已經漂移，回錯讓整筆 rollback。
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
			// before_state 是 nil：這一列本來不存在，沒有「之前的狀態」可寫。
			AfterState: jobAuditState(created.Status, map[string]string{
				// 🔴 只放路由與狀態欄。抬頭／統編／email 一律不進——那是一張保存 5 年的表
				// （§9.5），把買受人身分複製進去等於憑空造一個五年期的 PII 倉庫。
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

		// 🔴 手動開票是不可逆的對外動作 ⇒ 留一行 INFO。稽核列答的是「事後查得到」，
		// log 答的是「當下看得到」——兩者不互相取代。
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

// resolveIssuer 解出開票主體，並回「開票當下的抬頭與統編快照」。
//
// 🔴 兩者都必須取得到，取不到就回錯誤：issuer_name_snapshot 是印在發票上的銷售人名稱，
// 用佔位字串頂替等於開出一張抬頭是「store-123」的發票。
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
		// 🔴 平台統編未設定 ＝ 平台 issuer 未啟用，**不得開票**（#3230）。
		//
		// 這一格是 §4.5.8 第三層：AMEGO_PLATFORM_INVOICE 自 #3230 起是選填（空值 ＝ 尚未
		// 開通，是合法的部署狀態），fail-closed 因此從「啟動時拒絕啟動」移到「開票當下拒絕
		// 開票」。少了這一格，空統編會一路走到 issuer_tax_id_snapshot，開出一張銷售人統編
		// 是空字串的發票——而發票開出去不可回收。
		//
		// ⚠️ **必須在 GetByRef 之前**：prod 與 dev 的 invoice_issuers 都還沒有 PLATFORM 那
		// 一列（由維運預先建好），GetByRef 會先回 not-found。放在它之後這一格永遠執行不到，
		// 變異測試也會假綠（拿掉它照樣被擋，但擋的是另一個原因）。
		//
		// 碼用 9346（開票人與受票人的組合不合法）而不新增專用碼：語意涵蓋「以平台為開票人
		// 但平台未啟用」，而新增碼要跑 generate.sh --all 並同步四份前端拷貝，對一條至今從未
		// 被執行過的預防性路徑不成比例。message 自訂，讓 ADM 看得出真正原因。
		if u.cfg.PlatformTaxID == "" {
			return domain.Issuer{}, "", "", pkgerrors.NewBusiness(
				bizcode.InvoiceIssuanceTargetInvalid,
				"平台 issuer 未啟用（AMEGO_PLATFORM_INVOICE 未設定），無法以平台名義開立")
		}
		// issuer_ref_id 恆為 0（CodeMeta 逐字）。這一列由維運預先建好——它的 display_name
		// 是平台的**法定名稱**，本服務沒有第二個來源，自己造一個等於在發票上冒名。
		issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypePlatform.Value, 0)
		if err != nil {
			return domain.Issuer{}, "", "", err
		}
		return issuer, issuer.DisplayName, u.cfg.PlatformTaxID, nil
	}
	return domain.Issuer{}, "", "", jobBiz(bizcode.InvoiceIssuanceTargetInvalid)
}

// resolveRecipient 解出買受人抬頭、統編與業務鍵。
//
// recipient_type='SELLER'（方向二）的抬頭與統編來自 **seller_tax_profiles**（§5.1.5
// 取值表：該表明文擴充為「賣家的稅籍資料」，開票人與受票人兩個方向共用）；ADM 可覆寫，
// 覆寫值落本文件的快照欄、**不回寫 seller_tax_profiles**。
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
		// recipient_ref_id 恆為 NULL（CodeMeta 逐字）：抬頭／統編只存在於本文件的快照欄。
	}

	if utf8.RuneCountInString(name) > manualIssuanceNameMax {
		return "", "", nil, ErrIssuanceRecipientNameLong
	}
	// B2C／RECEIPT 一律 10 個 0（chk_invdoc_b2c／chk_invdoc_taxid_shape）；
	// B2B 必為 8 碼且不得左補零成 10 碼（會被 AMEGO 3040122 拒）。
	if in.DocumentKind == codemeta.InvoiceDocumentKindB2b.Value {
		if !isEightDigits(taxID) {
			return "", "", nil, ErrIssuanceRecipientTaxID
		}
	} else {
		taxID = domain.B2CBuyerIdentifier
	}
	return recipientName(in.DocumentKind, name), taxID, refID, nil
}

// buildDocument 組出 invoice_documents 的一列。
//
// 每一個「與 status 無關但與 document_kind 綁定」的欄位都在這裡決定，理由是那幾條
// CHECK 是**雙向等式**：chk_invdoc_receipt（RECEIPT ⟺ receipt_no 非空）、
// chk_invdoc_kind_mode（RECEIPT ⟺ PLATFORM_RECEIPT）、chk_invdoc_deadline_presence
// （B2C/B2B ⟺ statutory_deadline_at 非空）。少寫任一邊都是當場 23514。
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
		// chk_invdoc_source_ref 是雙向等式：MANUAL ⇒ source_ref 必須是 NULL。
		SourceType:      sourceTypeManual,
		TriggerType:     triggerTypeManual,
		RecipientType:   in.Recipient.Kind,
		RecipientRefID:  recipientRefID,
		RecipientName:   recipientName,
		RecipientTaxID:  recipientTaxID,
		RecipientEmail:  in.Recipient.Email,
		DocumentKind:    in.DocumentKind,
		FulfillmentMode: mode,
		Status:          codemeta.InvoiceDocumentStatusPending.Value,
		SnapshotVersion: 1,
		IssueAttemptSeq: 1,

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

		InvoiceDateTaipei: &invoiceDate,
		// 收據沒有稅期別（#3026）——ADM 手動建單同樣可以建 RECEIPT，判準與 issuance.go 共用。
		TaxPeriodCode:       taxPeriodFor(in.DocumentKind, invoiceDate),
		IssueNotBefore:      now,
		StatutoryDeadlineAt: deadline,
		SourceSnapshot:      snapshot,
		Now:                 now,
	}, nil
}

// createOperation 建 ISSUE operation 並在**同一事務內**推進 J02（CREATED → READY）。
//
// 少了 Promote，那一列會永遠停在 CREATED——而 CREATED 不在任何 worker 的候選述詞裡
// ⇒ 這張發票永遠不會被送出，且沒有任何告警。
func (u *AdminIssuance) createOperation(
	ctx context.Context, doc domain.Document, now time.Time, in ManualIssuanceInput,
) (int64, string, error) {
	// 建列與 J02 的推進共用 createAndPromoteIssueOperation（三個呼叫端一份實作，
	// 見 issue_operation.go 的檔頭）。
	opID, status, _, err := createAndPromoteIssueOperation(ctx, u.ops, issueOperationSpec{
		OperationID:       u.newID(),
		DocumentID:        doc.ID,
		IssueAttemptSeq:   1,
		ProviderRequestNo: derefString(doc.ProviderRequestNo),
		// request_snapshot_hash 在手動線沒有 IssueDraft 可雜湊（明細由 ADM 手填）：
		// 落文件 id 當識別。它只被 J18 的「ADM 修正資料後重試」讀，不進任何述詞。
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

// appendHistory 記一次狀態轉移（actor 固定 ADMIN）。
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

// ---------------------------------------------------------------------------
// 純函式
// ---------------------------------------------------------------------------

func isAllowedIssuanceCombo(in ManualIssuanceInput) bool {
	for _, c := range allowedIssuanceCombos {
		if c.issuer == in.Issuer.Kind && c.recipient == in.Recipient.Kind && c.source == in.Source.Kind {
			return true
		}
	}
	return false
}

// buildManualLines 把 ADM 手填的明細折成 domain.Line，並把四個 CHECK 先在本層擋掉。
//
// 🔴 **不補算、不校正**：net_amount 與 quantity × unit_price 對不起來時回錯誤，
// 不是「幫他算一個」。ADM 打錯一個數字而系統默默改成另一個數字，開出去的是一張
// 金額不是他要的、而且他以為沒問題的發票。
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
		// chk_invline_qty —— **必須排在下面兩道之前**（見該錯誤的註解）。
		if l.Quantity <= 0 {
			return nil, ErrIssuanceLineQuantityInvalid
		}
		// chk_invline_amount
		if l.NetAmount != int64(l.Quantity)*l.UnitPrice {
			return nil, ErrIssuanceLineInvalid
		}
		// chk_invline_sign
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
		// chk_invline_allowance_shape：DISCOUNT 行三個 allowance 欄恆 0；其餘 base = net。
		// 手動線沒有訂單折扣要分攤 ⇒ AllocatedDiscount 恆 0。
		if !isDiscount {
			line.AllowanceableBase = line.NetAmount
		}
		out = append(out, line)
	}
	return out, nil
}

// manualProviderRequestNo 組出手動線的業務冪等鍵，落 invoice_documents.provider_request_no
// （varchar(40)，partial UNIQUE）。
//
// 🔴 鍵**只由請求內容決定**、不含 client 的 Idempotency-Key：含了的話，同一份內容配兩把
// 不同的鍵就會開出兩張發票——而端點 57 是不可逆的。代價是「刻意重複開立」必須讓內容有差
// （例如備註不同），那是正確的一側。
func manualProviderRequestNo(issuer domain.Issuer, in ManualIssuanceInput) string {
	canonical, _ := json.Marshal(struct {
		DocumentKind string                  `json:"documentKind"`
		Recipient    ManualIssuanceRecipient `json:"recipient"`
		Source       ManualIssuanceSource    `json:"source"`
	}{in.DocumentKind, in.Recipient, in.Source})
	sum := sha256.Sum256(canonical)
	// "MAN-" + 最長 19 位 issuer id + "-" + 16 hex ＝ 最長 40，剛好落在 varchar(40) 內。
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

// errManualIssuanceIndexDrift 是「唯一索引擋下了，卻用同一把鍵查不到那一列」。
// 故障不是業務結果 ⇒ 刻意不是 DomainError，走 codes.Internal。
var errManualIssuanceIndexDrift = errors.New(
	"invoice: manual issuance insert was rejected by a unique index but no document was found for the same request_no")

func (u *AdminIssuance) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}
