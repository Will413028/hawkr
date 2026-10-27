package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// isBizCode 回報 err 是不是指定業務碼的 DomainError。
//
// 用途只有一個：把 repository 的「查無此列」從錯誤降級成決策樹的一個輸入值。
// 🔴 只認明確列出的碼——用 errors.Is(err, pgx.ErrNoRows) 之類的泛化判斷會把
// 「連線斷了」也吞成「這位賣家沒送審」，而那兩件事的正確處置相反（重試 vs 不重試）。
func isBizCode(err error, code int32) bool {
	var de *pkgerrors.DomainError
	return errors.As(err, &de) && int32(de.Code) == code
}

// taxResolverFor 由賣家稅務資格組出課稅別 resolver（Phase 1：整張訂單同一個課稅別）。
//
// 🔴 default_tax_category 轉不出 CodeMeta 條目時**不得 fallback 成 1（應稅）**：
// 那會把一張免稅發票靜默開成應稅（§C.10 ③）。零值（賣家尚未核定）也走應稅——那是
// DB DEFAULT 的語意，不是猜測。
func taxResolverFor(profile domain.SellerTaxProfile) (domain.TaxCategoryResolver, error) {
	cat := domain.TaxCategoryTaxable
	if profile.DefaultTaxCategory != 0 {
		if _, ok := domain.TaxCategoryFromDB(profile.DefaultTaxCategory); !ok {
			return nil, &domain.InvariantError{
				Code:   domain.InvariantCode(codemeta.InvoiceManualReasonMixedTaxCategory.Value),
				Detail: "seller_tax_profiles.default_tax_category has no CodeMeta entry: " + strconv.Itoa(int(profile.DefaultTaxCategory)),
			}
		}
		cat = domain.TaxCategory(profile.DefaultTaxCategory)
	}
	return domain.SellerDefaultTaxResolver{Default: cat}, nil
}

// creationStatus 把決策樹的葉節點折成 M1 **建立邊**允許的兩個值。
//
// AWAITING_MANUAL 不是建立邊（M1 只有 `[*]` → PENDING 與 `[*]` → NOT_APPLICABLE），
// 所以那一格先建成 PENDING，再由 persist 在同一交易內走 PENDING → AWAITING_MANUAL。
func creationStatus(d domain.FulfillmentDecision) string {
	if d.DocumentStatus == codemeta.InvoiceDocumentStatusNotApplicable.Value {
		return codemeta.InvoiceDocumentStatusNotApplicable.Value
	}
	return codemeta.InvoiceDocumentStatusPending.Value
}

// headerTaxType 把表頭稅別折進 chk_invdoc_tax_type 的值域 (1,2,3,9)。
//
// 無輸出行（零元訂單）時 BuildHeader 回 0，而 0 不在值域內 ⇒ 取 1（應稅）。
// 這不是「猜稅別」：那一列的 total_amount 恆為 0、永遠不會被送去開票，
// tax_type 只是為了滿足 NOT NULL ＋ CHECK 而必須有一個值。
func headerTaxType(h domain.Header) domain.TaxCategory {
	if h.TaxType == 0 {
		return domain.TaxCategoryTaxable
	}
	return h.TaxType
}

// recipientName 依 §5.1.5 的取值表決定 AMEGO BuyerName（I-45，不可由實作者自行決定）。
//
// 🔴 永不寫入自然人真實姓名：B2C 恆為常數「消費者」。這也是 recipient_name 得以是明文、
// 不進遮罩清單的唯一理由。
func recipientName(documentKind, buyerTitle string) string {
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value && buyerTitle != "" {
		return truncateRunes(buyerTitle, 60) // varchar(60)
	}
	return "消費者"
}

// recipientTaxID 依 document_kind 決定 recipient_tax_id。
//
// chk_invdoc_b2c 要求 B2C 恆為 '0000000000'，chk_invdoc_b2b 要求 B2B 為 8 碼統編
// （**不可左補零成 10 碼**，會被 3040122 拒）。
func recipientTaxID(documentKind, buyerTaxID string) string {
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value && domain.IsBusinessTaxID(buyerTaxID) {
		return buyerTaxID
	}
	return domain.B2CBuyerIdentifier
}

// normalizeBuyerIdentity 把三選一（統編／載具／捐贈碼）正規化成 chk_invdoc_exclusive 接受的形狀。
//
// 🔴 這裡**不做判斷只做落庫正規化**：輸入互斥與否已由 A15 在 Validate 內判過並落成
// BUYER_INFO_CONFLICT。兩者分開的理由是「我幫你挑一個」與「你給的資料互相矛盾」是兩件事，
// 合併之後互斥就再也不會被回報，而錯的憑證 AMEGO 照樣會收（三種組合實測皆 code:0）。
func normalizeBuyerIdentity(documentKind string, draft *domain.IssueDraft, donationCode string) (carrierType, carrierID1, carrierID2, donation string) {
	if draft == nil {
		return "", "", "", ""
	}
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value {
		return "", "", "", ""
	}
	if draft.CarrierType != domain.AmegoCarrierNone && draft.CarrierValue != "" {
		// 🔴 **顯碼與隱碼填同一個值**（2026-09-05 第 6 輪稽核）。
		//
		// AMEGO 的 f0401 要求 CarrierId1（顯碼）與 CarrierId2（隱碼）相符，
		// 三種載具類型皆然——《AMEGO 電子發票 API 總覽》§2.18.2 的實測矩陣逐字列著
		// 「顯碼 ≠ 隱碼／**只給顯碼、隱碼留空** → 3040131 載具顯碼、隱碼不相符」，
		// 而 §3.8 另記 amego 會員載具走 3040133、自訂 6 碼載具走 3040135，都是同一件事。
		//
		// 在此之前這裡回的是 `carrierID2 = ""` ⇒ **每一張帶載具的 B2C 統一發票都被
		// AMEGO 退回**，落 ISSUE_FAILED 等人工。dev 實測（2026-09-05）：
		// 帶載具的 B2C 憑證 ISSUED = 0、ISSUE_FAILED = 2，錯誤碼都是 3040131；
		// 唯二 ISSUED 的帶載具憑證是平台代製收據（RECEIPT），根本不經 AMEGO 開統一發票
		// ——所以「有成功的載具憑證」這個表象是假的，它們走的是另一條路。
		//
		// ⚠️ 我方只存一個載具值（buyer_invoice_preferences 與 invoice_documents 都是
		// 一個值 ＋ 一個 hash），這是對的：手機條碼／自然人憑證的顯隱碼**本來就是同一個
		// 字串**，存兩份等於製造兩個可能不一致的真相。要複製的位置是**送出的那一刻**。
		return draft.CarrierType, draft.CarrierValue, draft.CarrierValue, ""
	}
	return "", "", "", donationCode
}

// issuerNameSnapshot 取開票當下的開票人抬頭快照。
//
// 沒有 seller_tax_profile_version 欄（I-62）：seller_tax_profiles 一 issuer 一列、就地更新、
// 無歷史表 ⇒ 版本號 dereference 不了，「開立當下的稅籍資格」只能由**已複製的值**承擔。
func issuerNameSnapshot(issuer domain.Issuer, profile domain.SellerTaxProfile) string {
	if profile.LegalName != nil && *profile.LegalName != "" {
		return truncateRunes(*profile.LegalName, 120) // varchar(120)
	}
	return truncateRunes(issuer.DisplayName, 120)
}

// providerAccountVersion 回開票當下的 provider 帳戶版本快照（對外 wire 名是 cutoverVersion）。
// 帳戶不存在時回 nil —— 那一列本來就不會走 AMEGO。
func providerAccountVersion(a domain.ProviderAccount) *int64 {
	if a.ID == 0 {
		return nil
	}
	v := a.Version
	return &v
}

// toNewLines 把 domain 的明細行換成 repository 的寫入參數。
//
// 🔴 全部行都落庫（含 gross == 0 的行），只有送 AMEGO 時才過濾 —— 對帳要看得到那些行，
// 而 line_seq 與 ProductItem 陣列索引因此必然錯開，兩者不得互相推導。
func toNewLines(lines []domain.Line) []domain.NewDocumentLine {
	out := make([]domain.NewDocumentLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, domain.NewDocumentLine{
			LineSeq:           l.LineSeq,
			LineType:          l.LineType,
			OrderItemID:       l.OrderItemID,
			SkuID:             l.SkuID,
			Description:       l.Description,
			Quantity:          l.Quantity,
			UnitPrice:         l.UnitPrice,
			NetAmount:         l.NetAmount,
			TaxCategory:       int16(l.TaxCategory),
			TaxRateBps:        domain.TaxRateBps,
			AllocatedDiscount: l.AllocatedDiscount,
			AllowanceableBase: l.AllowanceableBase,
		})
	}
	return out
}

// sourceSnapshot 是 invoice_documents.source_snapshot 的形狀：**開票判定依據**的白名單快照。
//
// 🔴 禁存收件人姓名／電話／地址／買家 email／載具值（§5.1.5 COMMENT）。
// 這裡列的每一欄都是「為什麼判成這個履約模式」的輸入，不是訂單內容的鏡射。
type sourceSnapshot struct {
	OrderNo                   string `json:"order_no"`
	SnapshotVersion           int32  `json:"snapshot_version"`
	SellerReviewStatus        string `json:"seller_review_status"`
	ProviderStatus            string `json:"provider_status"`
	ProviderAccountVersion    *int64 `json:"provider_account_version"`
	AutomationCutoverAt       string `json:"automation_cutover_at,omitempty"`
	DefaultTaxCategory        int16  `json:"default_tax_category"`
	FulfillmentMode           string `json:"fulfillment_mode"`
	DocumentKind              string `json:"document_kind"`
	HasBuyerTaxID             bool   `json:"has_buyer_tax_id"`
	HasCarrier                bool   `json:"has_carrier"`
	HasDonation               bool   `json:"has_donation"`
	HasPlatformFundedDiscount bool   `json:"has_platform_funded_discount"`
	IsConsolidatedOrder       bool   `json:"is_consolidated_order"`
	TotalOriginalAmount       int64  `json:"total_original_amount"`
	ShippingFee               int64  `json:"shipping_fee"`
	DiscountAmount            int64  `json:"discount_amount"`
	TotalActualAmount         int64  `json:"total_actual_amount"`
}

// SnapshotAmounts 是 A06 第 3 層的金額拆解（#3031）。
//
// 🔴 來源是 **order_snapshot_json**，不是 invoice_documents 的金額欄：
// 那五欄（sales／freeTax／zeroTax／tax／total）是 **AMEGO 的分解**（依稅別），
// 而 A06 要的是**結帳的分解**（商品／運費／折扣），兩者是同一筆錢的兩種切法。
// 拿前者去湊後者會得出看似合理、實際對不上結帳頁的數字。
type SnapshotAmounts struct {
	ProductAmount  int64
	ShippingAmount int64
	DiscountAmount int64
	InvoiceAmount  int64
	TaxCategory    int16
}

// ParseSnapshotAmounts 從 order_snapshot_json 取出金額拆解。
//
// ok=false ⇒ 快照不存在或解不開 ⇒ 呼叫端不要填那個欄位（回 nil）。
// **不要回一組零值**：A06 會把它畫成「商品 0 元、運費 0 元」，而那是一個看起來
// 完全正常的錯誤畫面。
func ParseSnapshotAmounts(raw json.RawMessage) (SnapshotAmounts, bool) {
	if len(raw) == 0 {
		return SnapshotAmounts{}, false
	}
	var snap sourceSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return SnapshotAmounts{}, false
	}
	// 快照存在但四個金額全 0 ⇒ 視為沒有可用的拆解（早期快照沒有這幾欄）。
	// 判準用 TotalActualAmount：它是「發票總額的唯一權威」（domain.OrderSnapshot 逐字），
	// 一張真實發票不可能總額為 0。
	if snap.TotalActualAmount == 0 {
		return SnapshotAmounts{}, false
	}
	return SnapshotAmounts{
		ProductAmount:  snap.TotalOriginalAmount,
		ShippingAmount: snap.ShippingFee,
		DiscountAmount: snap.DiscountAmount,
		InvoiceAmount:  snap.TotalActualAmount,
		TaxCategory:    snap.DefaultTaxCategory,
	}, true
}

func marshalSourceSnapshot(
	in OrderPaidInput,
	decision domain.FulfillmentDecision,
	profile domain.SellerTaxProfile,
	account domain.ProviderAccount,
) (json.RawMessage, error) {
	snap := sourceSnapshot{
		OrderNo:                   in.OrderNo,
		SnapshotVersion:           in.SnapshotVersion,
		SellerReviewStatus:        profile.ReviewStatus,
		ProviderStatus:            account.Status,
		ProviderAccountVersion:    providerAccountVersion(account),
		DefaultTaxCategory:        profile.DefaultTaxCategory,
		FulfillmentMode:           decision.FulfillmentMode,
		DocumentKind:              decision.DocumentKind,
		HasBuyerTaxID:             domain.IsBusinessTaxID(in.BuyerTaxID),
		HasCarrier:                in.CarrierID1 != "",
		HasDonation:               in.DonationCode != "",
		HasPlatformFundedDiscount: in.HasPlatformFundedDiscount,
		IsConsolidatedOrder:       in.IsConsolidatedOrder,
		TotalOriginalAmount:       in.Snapshot.TotalOriginalAmount,
		ShippingFee:               in.Snapshot.ShippingFee,
		DiscountAmount:            in.Snapshot.DiscountAmount,
		TotalActualAmount:         in.Snapshot.TotalActualAmount,
	}
	if account.AutomationCutoverAt != nil {
		snap.AutomationCutoverAt = account.AutomationCutoverAt.UTC().Format(time.RFC3339)
	}
	return json.Marshal(snap)
}

// appendHistory 寫一列 invoice_status_history（append-only）。
//
// from 為空字串代表建立邊（`[*]` → X）。ReasonCode 空字串寫成 NULL。
// finishOperationSucceeded 把 operation 推到 SUCCEEDED，**同時容納 INV.op 與 INV.rc 兩條 lane**。
//
// 🔴 為什麼不能只呼叫 MarkDone：它守 `status = 'SENDING'`，那是 **INV.op lane** 的狀態。
// 從 INV.rc 進來時（ConfirmVoid／ConfirmAllowance），ClaimInvoiceRecheck 已經把 status
// 蓋成 PROVIDER_STATUS_RECHECK ⇒ MarkDone 必然 rows=0。而呼叫端把 rows 丟給 `_`，
// 於是**完全無聲**：document 收尾了、adjustment 收尾了、invoice.voided 也發了，
// 唯獨 operation 永遠留在 INV.rc 的候選集合裡，每 60 秒被 claim 一次，
// 且 unknown_since 從不推進 ⇒ scheduleVoidRecheck 的 24h 放棄門檻永遠到不了。
// db/queries/invoice_operations.sql 的 ⑧ 註解逐字寫著「不得沿用 ③ MarkInvoiceOperationDone」，
// 本函式是那句話的實作面。
//
// 🔴 兩條 lane 都不成立時**回 error 而不是寫軌跡**：原本的寫法在 rows=0 之後照樣
// appendStatusHistory(SUCCEEDED)，於是 invoice_status_history 上留下一條
// 「WAITING_CONFIRMATION → SUCCEEDED」而實際 status 是 PROVIDER_STATUS_RECHECK
// ——下一個人看到的軌跡是假的。
//
// from_status 取**真正推動它的那條 lane** 的狀態，不取 op.Status：後者是 claim 之前
// 讀到的快照，claim 已經把它改掉了。
func finishOperationSucceeded(
	ctx context.Context,
	ops domain.OperationRepository,
	audit domain.AuditRepository,
	op domain.Operation,
	now time.Time,
) error {
	completed := now
	ok, err := ops.MarkRecheckResolved(ctx, domain.MarkRecheckResult{
		ID:            op.ID,
		NextStatus:    codemeta.InvoiceOperationStatusSucceeded.Value,
		NextAttemptAt: now,
		CompletedAt:   &completed,
		Now:           now,
	})
	if err != nil {
		return err
	}
	from := codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	if !ok {
		from = codemeta.InvoiceOperationStatusSending.Value
		ok, err = ops.MarkDone(ctx, op.ID, now)
		if err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not enter SUCCEEDED (rows=0 on both lanes, snapshot status=%s)",
			op.ID, op.Status)
	}
	return appendStatusHistory(ctx, audit, subjectOperation, op.ID, from,
		codemeta.InvoiceOperationStatusSucceeded.Value, "", "", now)
}

func appendStatusHistory(ctx context.Context, audit domain.AuditRepository, subjectType string, subjectID int64, from, to, reasonCode, correlationID string, now time.Time) error {
	var fromPtr, reasonPtr, corrPtr *string
	if from != "" {
		fromPtr = &from
	}
	if reasonCode != "" {
		reasonPtr = &reasonCode
	}
	if correlationID != "" {
		corrPtr = &correlationID
	}
	return audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:   subjectType,
		SubjectID:     subjectID,
		FromStatus:    fromPtr,
		ToStatus:      to,
		ReasonCode:    reasonPtr,
		ActorType:     actorTypeSystem,
		CorrelationID: corrPtr,
		Now:           now,
	})
}

// appendStatusHistoryActor 與 appendStatusHistoryDetail 相同，但**由呼叫端指定 actor**。
//
// 🔴 為什麼非有不可：另外兩支把 `ActorType` 硬寫成 `SYSTEM`，而 `chk_invhist_actor`
// 的值域有五個（SYSTEM／ADMIN／SELLER／BUYER／PROVIDER）。申訴軸（COMPLAINT）的三條
// 路徑分別是**買家**、**賣家**、**ADM** 的動作，用硬寫 SYSTEM 的那兩支等於把
// 「誰做的」整欄抹掉——A10 稽核與 AuditTrailDrawer 讀的就是這一欄。
// dev 實測（2026-09-05，complaint 7501677659433205760）：賣家按下「已重新寄送」
// 之後的 `OPEN → SELLER_RESPONDED` 那一列 `actor_type = SYSTEM`、`actor_account_id = NULL`。
//
// ⚠️ `actorAccountID` 傳的是**該 actor 的營運鍵**，與同一份 code 裡 `complaint_events`
// 的慣例逐字相同（買家 `buyer_id`／賣家 `store_id`／ADM `account_id`，見
// `complaint.go` 的 AppendEvent 與 `receipt_authorization.go` 的 `&in.StoreID`）。
// 欄名叫 actor_account_id 而值不一定是 account_id，這個既有的語意債登記在 #3036，
// **本函式沿用現況而不是在這裡開第二套**——兩套並存比一套錯的更難查。
func appendStatusHistoryActor(
	ctx context.Context, audit domain.AuditRepository, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail, actorType string, actorAccountID *int64, now time.Time,
) error {
	var fromPtr, reasonPtr, detailPtr *string
	if from != "" {
		fromPtr = &from
	}
	if reasonCode != "" {
		reasonPtr = &reasonCode
	}
	if reasonDetail != "" {
		detailPtr = &reasonDetail
	}
	return audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		FromStatus:     fromPtr,
		ToStatus:       to,
		ReasonCode:     reasonPtr,
		ReasonDetail:   detailPtr,
		ActorType:      actorType,
		ActorAccountID: actorAccountID,
		Now:            now,
	})
}

// appendStatusHistoryDetail 與 appendStatusHistory 相同，但把**人看的說明**寫進
// reason_detail（text）而不是 correlation_id。
//
// 🔴 為什麼需要它：appendStatusHistory 只有 reasonCode 與 correlationID 兩個字串入口，
// 於是想記錄失敗原因的呼叫端只能把訊息塞進 correlationID —— 而
// invoice_status_history.correlation_id 是 **varchar(64)**。
// dev 實測（第 3 輪 INV58R3-ALW06，AMEGO 回「查無此發票」）：折讓失敗處置本身以
// `SQLSTATE 22001 value too long for type character varying(64)` 整個 rollback，
// 真正的 provider 錯誤訊息被那句 22001 取代 ⇒ ADM 看到的是資料庫錯誤而不是業務原因。
// 也就是說**失敗處置在訊息夠長時自己會失敗**，而 correlation_id 這個追蹤欄位同時被污染。
//
// reason_detail 是 text（無長度上限），而 reason_code 是 varchar(48) ——
// 碼放 code、話放 detail、追蹤 id 放 correlation_id，三者不得互相借位。
func appendStatusHistoryDetail(
	ctx context.Context, audit domain.AuditRepository, subjectType string, subjectID int64,
	from, to, reasonCode, reasonDetail, correlationID string, now time.Time,
) error {
	var fromPtr, reasonPtr, detailPtr, corrPtr *string
	if from != "" {
		fromPtr = &from
	}
	if reasonCode != "" {
		reasonPtr = &reasonCode
	}
	if reasonDetail != "" {
		detailPtr = &reasonDetail
	}
	if correlationID != "" {
		corrPtr = &correlationID
	}
	return audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
		SubjectType:   subjectType,
		SubjectID:     subjectID,
		FromStatus:    fromPtr,
		ToStatus:      to,
		ReasonCode:    reasonPtr,
		ReasonDetail:  detailPtr,
		ActorType:     actorTypeSystem,
		CorrelationID: corrPtr,
		Now:           now,
	})
}

// appendHistory 是 Issuance 的便利包裝，行為與 appendStatusHistory 完全相同。
// 兩者並存的理由：作廢軸（Voiding）與開票軸（Issuance）是不同的 struct 但共用同一份
// 稽核語意，把它抽成函式才不會出現第二份「怎麼寫歷程」的實作。
func (u *Issuance) appendHistory(ctx context.Context, subjectType string, subjectID int64, from, to, reasonCode, correlationID string, now time.Time) error {
	return appendStatusHistory(ctx, u.audit, subjectType, subjectID, from, to, reasonCode, correlationID, now)
}

func (u *Issuance) logf(ctx context.Context) *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}

// derefInt64 是 *int64 → int64（nil ⇒ 0）。
//
// 從 admin_manual_unlock_test.go 搬過來的：折讓軸（#3073）需要它，而同一個 package
// 內留兩份同義 helper 會在下一次有人加第三份時變成三份。
func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// truncateRunes 以 rune 截斷。禁止 s[:n]：以 byte 切會把中文切成半個字產生無效 UTF-8。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ---------------------------------------------------------------------------
// 出向事件的 store_id / buyer_id
// ---------------------------------------------------------------------------
//
// 🔴 這裡**沒有**產生點，是刻意的。兩個軸的值只能由 internal/eventwire 的
// StoreIDOf / BuyerIDOfDocument / BuyerIDOfComplaint 產生，而那三支住在另一個
// package 才使「本 package 內組不出一個錯的值」成為**編譯期**事實。
//
// 這裡曾經有一支 `storeIDWire(issuer domain.Issuer) string`。它是對的，但回 string
// ⇒ `ev.StoreID = 任何字串` 照樣編得過，保證只能靠 AST 檢查維持，而 AST 檢查漏了
// 「事後賦值」這一種寫法整整一輪。型別擋得住的東西不要交給檢查去擋。

// taxPeriodFor 回這份文件的財政部期別碼；**收據回空字串**（#3026）。
//
// 期別碼的語意是「這張發票落在哪一個申報期」。收據不申報、不上傳財政部平台、
// 也沒有作廢期限（設計書 §7.2：收據只重開／註記）⇒ 它沒有期別可言。
//
// 抽成函式而不是在呼叫點寫 if：`admin_issuance.go` 與 `correction.go` 也各有一個
// 建 document 的路徑，三處要用同一份判準。三份字面值 if 就是「平行清單只更新一部分」
// 的標準形狀。
func taxPeriodFor(documentKind string, invoiceDate time.Time) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return ""
	}
	return domain.TaxPeriodCode(invoiceDate)
}
