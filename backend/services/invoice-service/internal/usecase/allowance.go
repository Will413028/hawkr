package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 折讓執行軸（§7.3.7 M7 的 G08–G15）
//
// 🔴 **這一整條路之前不存在**（#3073 的稽查結論）。當時的狀態是：AMEGO client 的
// IssueAllowance／QueryAllowance 寫好了、19 支 SQL 寫好了、sqlc 生成物有了、
// §5A.9 的金額換算（domain/allowance_lines.go）也寫好了 —— 但**零呼叫端**，
// 沒有任何東西把它們串起來。invoice_adjustments 建得出來，之後沒有人推它。
//
// 後果：買家退款後拿不到折讓單（法定憑證），而那些列在 DB 裡無聲累積。
//
// 本檔補的是「串起來」那一段，形狀逐字沿用 Voiding.SubmitVoid：同一支 worker、
// 同一組候選、同一份租約策略，只有送出的東西與落點不同。
// ---------------------------------------------------------------------------

// Allowance 是折讓軸。
//
// 它與 Voiding 分開而不是加一個 if：兩者送的 payload、落的狀態、要維護的反正規化欄位
// 完全不同（折讓要累加 allowance_total_amount 與逐行 allowanced_amount，作廢不用）。
type Allowance struct {
	tx          database.TxManager
	docs        domain.DocumentRepository
	ops         domain.OperationRepository
	adjustments domain.AdjustmentRepository
	accounts    domain.ProviderAccountRepository
	issuers     domain.IssuerRepository
	audit       domain.AuditRepository
	outbox      domain.OutboxRepository
	provider    ProviderClient

	now func() time.Time
	cfg IssuanceConfig
	// b2cSplitTax 對應 INVOICE_ALLOWANCE_B2C_SPLIT_TAX（§5A.9.4 未決 U5）。
	b2cSplitTax bool
	log         *zap.Logger
}

// NewAllowance 組出折讓軸。cfg 與 Issuance／Voiding 共用同一份 —— 三者搶的是
// **同一張表的同一組候選**，租約長度與退避必須一致。
func NewAllowance(
	tx database.TxManager,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	adjustments domain.AdjustmentRepository,
	accounts domain.ProviderAccountRepository,
	issuers domain.IssuerRepository,
	audit domain.AuditRepository,
	outbox domain.OutboxRepository,
	provider ProviderClient,
	cfg IssuanceConfig,
	b2cSplitTax bool,
	log *zap.Logger,
) *Allowance {
	return &Allowance{
		tx: tx, docs: docs, ops: ops, adjustments: adjustments,
		accounts: accounts, issuers: issuers, audit: audit, outbox: outbox, provider: provider,
		now:         func() time.Time { return time.Now().UTC() },
		cfg:         cfg,
		b2cSplitTax: b2cSplitTax,
		log:         orNop(log),
	}
}

// WithClock 讓測試注入時鐘。
func (u *Allowance) WithClock(now func() time.Time) *Allowance {
	if now != nil {
		u.now = now
	}
	return u
}

// ClaimForAllowanceAt 是 worker 的租約閘門（形狀與 ClaimForVoidAt 相同）。
//
// 🔴 `ops.Claim` 的簽章是 `(ctx, id, now, leaseUntil)`，兩個 time.Time **不可對調**：
// `now` 落在 `started_at`／`updated_at` 與候選述詞的 `next_attempt_at <= claimable_before`，
// `leaseUntil` 落在 `next_attempt_at`（＝租約本身）。對調的後果是
// `next_attempt_at` 被寫成**當下** ⇒ 租約長度變 0 ⇒ 這一列在副作用還在飛的時候就已經
// 回到候選集合，而 `MarkInvoiceAdjustmentSending` 的 allowed-from 刻意含 `SENDING` 自己
// ⇒ adjustment 那一層攔不住第二個認領者 ⇒ 同一筆折讓打第二次 AMEGO。
// 兩個參數同型別，編譯器不會出聲；dev 上的指紋是 `started_at − updated_at ＝ +ClaimLease`
// （2026-09-05 實查 ALLOWANCE 5 列全中，而 VOID 15 列全無）。
func (u *Allowance) ClaimForAllowanceAt(
	ctx context.Context, operationID int64, now, leaseUntil time.Time,
) (bool, error) {
	claimed := false
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, operationID)
		if err != nil {
			return err
		}
		ok, err := u.ops.Claim(ctx, operationID, now, leaseUntil)
		if err != nil || !ok {
			return err
		}

		// 🔴 G08：adjustment 必須跟著進 SENDING，**否則整條軸必然失敗**。
		// MarkCompleted 的 allowed-from 是 (SENDING, WAITING_CONFIRMATION,
		// FAILED_UNKNOWN)，不含 PENDING ⇒ 少了這一句，每一筆折讓都會在
		// settleAllowanceCompleted 撞上 rows=0 而回
		// 「could not enter COMPLETED」——而那時 AMEGO 已經開好折讓單了，
		// 錢退了、憑證開了，我方的帳卻是失敗的。
		//
		// 這一句原本漏掉，是在自查「誰把 adjustment 從 PENDING 推走」時抓到的；
		// 作廢軸的同一格漏了三天沒人發現（見 Voiding.advanceAdjustment）。
		adj, err := u.adjustmentOf(ctx, op)
		if err != nil {
			return err
		}
		ok, err = u.adjustments.MarkSending(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			// 與作廢軸相反：這裡**要**當錯誤。折讓的 adjustment 是主體
			// （單號、三個金額欄、累計落帳都掛在它身上），推不動代表狀態機破了，
			// 而放行會讓 AMEGO 收到一張我方帳上不存在的折讓單。
			return fmt.Errorf("invoice: adjustment %d could not enter SENDING (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusSending.Value, "", "", now); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// SubmitAllowance 送一張折讓單（G08 → G09／G10／G11／G12）。
//
// 契約與 SubmitVoid 逐字相同：回 error 只代表「這一輪連狀態都沒寫成」（DB 斷線之類），
// 所有業務落點都回 nil —— 狀態已經寫定。兩者共用同一個 worker，契約不一致會讓
// worker 的 attempt 簿記打在錯的前提上。
func (u *Allowance) SubmitAllowance(ctx context.Context, operationID int64) error {
	op, err := u.ops.GetByID(ctx, operationID)
	if err != nil {
		return err
	}
	if op.Status != codemeta.InvoiceOperationStatusSending.Value {
		// 沒有租約就送 ＝ 可能有第二個持有者同時在打 AMEGO ⇒ 重複折讓，
		// 而折讓單是法定憑證、開錯不可逆。
		return fmt.Errorf("invoice: operation %d is %s, expected SENDING (claim first)", op.ID, op.Status)
	}

	adj, err := u.adjustmentOf(ctx, op)
	if err != nil {
		return err
	}
	// resume checkpoint：已經 COMPLETED 就不再打 AMEGO（同開票路徑的
	// 「已有發票號碼就不再送」）。擋的是本地已知結果之後的任何重試。
	if adj.Status == codemeta.InvoiceAdjustmentStatusCompleted.Value {
		return u.finishOperation(ctx, op, u.now())
	}

	doc, err := u.docs.GetByID(ctx, adj.DocumentID)
	if err != nil {
		return err
	}
	if doc.ProviderInvoiceNo == nil || *doc.ProviderInvoiceNo == "" {
		// 原發票沒有號碼 ⇒ 折讓無所依附。這不是重試得回來的。
		return u.settleAllowancePermanent(ctx, adj, op,
			// #3082：這一格是**我方判定**不是 provider 拒絕，也不是超額。
			codemeta.InvoiceManualReasonAllowanceNoSourceInvoice.Value, "", "",
			"原發票沒有 provider_invoice_no，折讓無所依附", u.now())
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	lines, err := u.adjustments.ListLines(ctx, adj.ID)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		// G01 應該已經把明細寫好了。走到這裡代表建列那一段漏了明細，
		// 而送一張沒有 ProductItem 的折讓單會被 AMEGO 拒絕。
		return u.settleAllowancePermanent(ctx, adj, op,
			// #3082：明細行數不合法，值域內既有的碼就說得準，不必借「超額」。
			codemeta.InvoiceManualReasonLineCountInvalid.Value, "", "",
			"折讓明細為空", u.now())
	}

	req := allowanceRequestOf(adj, doc, lines)

	var sendErr error
	// 🔴 AppKey 明文的活動範圍只到這個 callback：不回傳、不 log、不進任何 struct 欄位。
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		_, sendErr = u.provider.IssueAllowance(auditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey}, req)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if sendErr == nil {
		return u.settleAllowanceCompleted(ctx, adj, doc, op, now)
	}
	return u.settleAllowanceFailure(ctx, adj, doc, op, sendErr, now)
}

// allowanceRequestOf 把落庫的明細組成 AMEGO g0401。
//
// 🔴 金額欄位一律取 **未稅**（明細的 UnitPriceExTax／NetAmountExTax）：AMEGO 折讓的
// UnitPrice／Amount 固定未稅，與開立端 DetailVat=1 的含稅語意相反（§5A.9.3）。
// 拿含稅值填會讓折讓金額整批多出 5%，而那是稅務錯誤。
func allowanceRequestOf(
	adj domain.Adjustment, doc domain.Document, lines []domain.AdjustmentLine,
) amego.AllowanceRequest {
	items := make([]amego.AllowanceProductItem, 0, len(lines))
	for _, l := range lines {
		date := ""
		if l.OriginalInvoiceDate != nil {
			date = l.OriginalInvoiceDate.Format(amegoDateLayout)
		}
		items = append(items, amego.AllowanceProductItem{
			OriginalInvoiceNumber: l.OriginalInvoiceNo,
			OriginalInvoiceDate:   date,
			OriginalDescription:   l.OriginalDescription,
			Quantity:              strconv.FormatInt(int64(l.Quantity), 10),
			UnitPrice:             strconv.FormatInt(l.UnitPriceExTax, 10),
			Amount:                strconv.FormatInt(l.NetAmountExTax, 10),
			Tax:                   strconv.FormatInt(l.TaxAmount, 10),
			TaxType:               strconv.FormatInt(int64(l.TaxCategory), 10),
		})
	}
	allowanceDate := ""
	if adj.AllowanceDateTaipei != nil {
		allowanceDate = adj.AllowanceDateTaipei.Format(amegoDateLayout)
	}
	return amego.AllowanceRequest{
		AllowanceNumber: derefString(adj.AllowanceNumber),
		AllowanceDate:   allowanceDate,
		// AllowanceType 2 ＝ 買受人開立（我方代開）。1 是營業人開立，需要買受人簽署。
		AllowanceType:   allowanceTypeBuyerIssued,
		BuyerIdentifier: doc.RecipientTaxID,
		BuyerName:       doc.RecipientName,
		ProductItem:     items,
		TaxAmount:       strconv.FormatInt(derefInt64(adj.TaxAmount), 10),
		TotalAmount:     strconv.FormatInt(derefInt64(adj.NetAmountExTax), 10),
	}
}

const (
	amegoDateLayout          = "20060102"
	allowanceTypeBuyerIssued = "2"
)

// settleAllowanceCompleted 是 G09：折讓生效。
//
// 一個交易寫四樣東西：adjustment → COMPLETED、原發票的 allowance_total_amount 累加、
// 逐行 allowanced_amount 累加、outbox（P4）。拆開任何一樣的後果都是「金額對不上」——
// 而下一次折讓的上限判斷用的正是這兩個累計值。
func (u *Allowance) settleAllowanceCompleted(
	ctx context.Context, adj domain.Adjustment, doc domain.Document, op domain.Operation, now time.Time,
) error {
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := u.allowanceIssuedPayload(issuer, doc, adj, now)
	if err != nil {
		return err
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		// outbox 排最前面，理由與 settleIssued 逐字相同：把它放最後的話，
		// 「事件寫入有沒有跟隨業務交易」只能靠「讓交易在它之後失敗」來驗，
		// 而它後面沒有任何一句 SQL ⇒ 那個變異是無法轉紅的 no-op。
		if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateAdjustment,
			AggregateID:   adj.ID,
			EventType:     string(pkgmq.EventTypeInvoiceAllowanceIssued),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    pkgmq.RoutingKeyInvoiceAllowanceIssued,
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); err != nil {
			return err
		}

		ok, err := u.adjustments.MarkCompleted(ctx, domain.MarkAdjustmentCompleted{ID: adj.ID, Now: now})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter COMPLETED (rows=0, status=%s)",
				adj.ID, adj.Status)
		}

		// I4：反正規化累計與 adjustment 同交易維護。
		//
		// 🔴 **rows=0 必須整筆 rollback，不可當成功。** 那一句是單句 CAS，述詞是
		// `allowance_total_amount + delta <= total_amount`；不成立時它回 rows=0
		// 而**不是** error（那是刻意的：23514 會讓整個交易 aborted，25P02 之後每一句
		// 都被拒）。契約在三處逐字寫著「呼叫端必須整筆 rollback」——
		// gen/invoice_documents.sql.go、document_repo.go 的 AddAllowanceTotal、
		// domain/repository.go 的介面註解。
		//
		// 這裡曾經寫 `if _, err := ...`，**把那個 bool 丟掉**：超額時 adjustment 會落
		// COMPLETED、折讓事件照發、而 allowance_total_amount 停在舊值 ⇒ 下一筆退款的
		// 上限判斷用的是錯的基數，零錯誤零告警（2026-09-05 第 6 輪稽核）。
		// 同一個迴圈下方 12 行的 AddLineAllowancedAmount 一直都有檢查 ok
		// ——**相鄰兩個決策方向相反本身就是訊號**。
		ok, err = u.docs.AddAllowanceTotal(ctx, doc.ID, derefInt64(adj.GrossAmount), now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf(
				"invoice: document %d allowance cap exceeded (adjustment %d, delta %d, already %d, total %d)",
				doc.ID, adj.ID, derefInt64(adj.GrossAmount), doc.AllowanceTotalAmount, doc.TotalAmount)
		}

		lines, err := u.adjustments.ListLines(ctx, adj.ID)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if l.SourceLineID == nil {
				continue
			}
			// 逐行累加的是**含稅**折讓額（allowanceable_base 是含稅基數），
			// 而明細存的是未稅 ⇒ 這裡要加回稅額。少加稅額的後果是下一次折讓
			// 的剩餘額被高估，於是累計超過發票金額、AMEGO 4040173 才發現。
			ok, err := u.docs.AddLineAllowancedAmount(ctx, *l.SourceLineID,
				l.NetAmountExTax+l.TaxAmount, now)
			if err != nil {
				return err
			}
			if !ok {
				// chk_invline_allowanced_cap 擋下來了 ⇒ 這一行的累計會超過可折讓基數。
				// 整筆 rollback：折讓單還沒被 AMEGO 接受成事實之前，這是唯一安全的處置。
				return fmt.Errorf("invoice: line %d allowanced cap exceeded (adjustment %d)", *l.SourceLineID, adj.ID)
			}
		}

		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusCompleted.Value, "", "", now); err != nil {
			return err
		}
		return u.finishOperationTx(ctx, op, now)
	})
}

// settleAllowanceFailure 依 AMEGO 的錯誤分類決定落點（G10／G11／G12）。
func (u *Allowance) settleAllowanceFailure(
	ctx context.Context, adj domain.Adjustment, doc domain.Document,
	op domain.Operation, sendErr error, now time.Time,
) error {
	var apiErr *amego.APIError
	if !errors.As(sendErr, &apiErr) {
		// 非 provider 錯誤（ctx 取消、transport 尚未分類）⇒ 結果不明。折讓可能已經
		// 送達 ⇒ **不得重送**（單號會重複），走 WAITING_CONFIRMATION 由查詢收斂。
		return u.settleAllowanceUnknown(ctx, adj, op, now)
	}
	switch apiErr.Class {
	case amego.ClassRetryable:
		return u.settleAllowanceRetry(ctx, adj, op, apiErr, now)
	case amego.ClassReconcile:
		// 含逾時與「已存在」等值成功。兩者處置相同：去問 AMEGO。
		// 盲目重送才是會產生第二張折讓單的那條路。
		return u.settleAllowanceUnknown(ctx, adj, op, now)
	case amego.ClassManual, amego.ClassPermanent:
		// 🔴 **不猜分類**（#3082，DEC-20260904-120527 選 A）。
		//
		// 在此之前這裡硬編碼 ALLOWANCE_EXCEEDS_INVOICE（累計折讓超過原發票金額），
		// 不論真實原因是什麼。dev 實測：AMEGO 回 4040156「原發票不存在」的一筆，
		// 在 ADM 的異常工作台被分類成「超額」⇒ 把診斷方向帶去查金額。
		//
		// PROVIDER_REJECTED_PERMANENT 是中性碼：它說的是「服務商明確拒絕」，
		// 真實原因在 provider_code／provider_message（本次一併真的寫入）
		// 與 reason_detail。**猜一個具體分類比不分類更糟**。
		return u.settleAllowancePermanent(ctx, adj, op,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			apiErr.Code, apiErr.Msg, apiErr.Error(), now)
	default:
		// 分類值域長胖時預設走對帳：多打一次唯讀查詢，代價最小且會自己收斂。
		return u.settleAllowanceUnknown(ctx, adj, op, now)
	}
}

// settleAllowanceRetry 是 G11：可重試的失敗回到 PENDING，退避表達在 operation 上。
//
// 🔴 **折讓單號不換**（§7.2.2 細節 5）：號碼在送出前就落庫了，重送用同一個號碼才
// 不會產生第二張折讓單。這也是為什麼 Reschedule 只改狀態、不碰 allowance_number。
func (u *Allowance) settleAllowanceRetry(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, apiErr *amego.APIError, now time.Time,
) error {
	newAttempt := op.AttemptCount + 1
	exhausted := newAttempt >= op.MaxAttempts
	delay := backoffFor(int(newAttempt), u.cfg.BaseBackoff, u.cfg.MaxBackoff)
	lastErr := apiErr.Error()

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := u.ops.UpdateAttempt(ctx, op.ID, newAttempt, now.Add(delay), &lastErr, now); err != nil {
			return err
		}
		ok, err := u.adjustments.Reschedule(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not go back to PENDING (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusPending.Value, "", lastErr, "", now); err != nil {
			return err
		}
		if !exhausted {
			return u.rescheduleOperationTx(ctx, op, newAttempt, now.Add(delay), lastErr, now)
		}
		// 打滿重試次數 ⇒ operation 落 FAILED_RETRY_EXHAUSTED，adjustment 留在 PENDING。
		// 那不是矛盾：adjustment 的下一步是 ADM 在 A06 按安全重試（G16），
		// 而它的 allowed-from 就是 operation 的失敗態。
		return u.exhaustOperationTx(ctx, op, lastErr, now)
	})
}

// settleAllowancePermanent 是 G10：確定送不出去。
//
// blocked_reason_code 必填（chk_invadj_blocked_reason）。落點是 ADM 佇列而不是靜默丟棄
// —— 一筆退款已經退了錢卻開不出折讓單，那是要有人處理的稅務缺口。
func (u *Allowance) settleAllowancePermanent(
	ctx context.Context, adj domain.Adjustment, op domain.Operation,
	reasonCode, providerCode, providerMessage, detail string, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.adjustments.MarkFailedPermanent(ctx, adj.ID, reasonCode, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter FAILED_PERMANENT (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusFailedPermanent.Value, reasonCode, detail, "", now); err != nil {
			return err
		}
		return u.failOperationTx(ctx, op, reasonCode, providerCode, providerMessage, detail, now)
	})
}

// settleAllowanceUnknown 是 G12：結果不明。
//
// 🔴 **不得重送**：我方自產的折讓單號已經送出去了，重送會讓 AMEGO 端出現第二張
// 同號憑證（或撞唯一鍵）。唯一安全的下一步是查詢（G13／G14 由 recheck 軸收斂）。
func (u *Allowance) settleAllowanceUnknown(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, now time.Time,
) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.adjustments.MarkWaitingConfirmation(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: adjustment %d could not enter WAITING_CONFIRMATION (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		if err := appendStatusHistory(ctx, u.audit, subjectAdjustment, adj.ID,
			adj.Status, codemeta.InvoiceAdjustmentStatusWaitingConfirmation.Value, "", "", now); err != nil {
			return err
		}
		return u.waitOperationTx(ctx, op, now)
	})
}

// allowanceIssuedPayload 組 P4（`invoice.allowance.issued`，58 §6.3.9 定版 13 列第 5 列）。
type allowanceIssuedEvent struct {
	NotificationRefID      string            `json:"notification_ref_id"`
	InvoiceDocumentID      string            `json:"invoice_document_id"`
	AdjustmentID           string            `json:"adjustment_id"`
	OrderNo                string            `json:"order_no"`
	BuyerID                eventwire.BuyerID `json:"buyer_id"`
	StoreID                eventwire.StoreID `json:"store_id"`
	AllowanceNumber        string            `json:"allowance_number"`
	AllowanceAmount        string            `json:"allowance_amount"`
	CumulativeAllowanceAmt string            `json:"cumulative_allowance_amount"`
	InvoiceNo              string            `json:"invoice_no"`
	OccurredAt             string            `json:"occurred_at"`
}

func (u *Allowance) allowanceIssuedPayload(
	issuer domain.Issuer, doc domain.Document, adj domain.Adjustment, now time.Time,
) (json.RawMessage, error) {
	gross := derefInt64(adj.GrossAmount)
	return json.Marshal(allowanceIssuedEvent{
		// 冪等鍵 `INV:{allowance_document_id}`（CodeMeta INVOICE_ALLOWANCE_ISSUED 逐字）。
		// 用 adjustment id 而不是 document id：同一張發票可以分多次折讓，
		// 共用 document id 會讓第二張折讓單的通知被 notification 端去重掉。
		NotificationRefID:      "INV:" + strconv.FormatInt(adj.ID, 10),
		InvoiceDocumentID:      strconv.FormatInt(doc.ID, 10),
		AdjustmentID:           strconv.FormatInt(adj.ID, 10),
		OrderNo:                derefString(doc.SourceRef),
		BuyerID:                eventwire.BuyerIDOfDocument(doc),
		StoreID:                eventwire.StoreIDOf(issuer),
		AllowanceNumber:        derefString(adj.AllowanceNumber),
		AllowanceAmount:        strconv.FormatInt(gross, 10),
		CumulativeAllowanceAmt: strconv.FormatInt(doc.AllowanceTotalAmount+gross, 10),
		InvoiceNo:              derefString(doc.ProviderInvoiceNo),
		OccurredAt:             timeutil.FormatISO8601(now),
	})
}

// adjustmentOf 由 operation 找回它服務的那一筆 adjustment。
//
// 🔴 走 `op.AdjustmentID` 這個外鍵，**不要**用「document ＋ 未結案述詞」去撈。
// 後者原本是這裡的寫法，而它有兩個問題：
//
//	① 未結案述詞排除 COMPLETED ⇒ `SubmitAllowance` 開頭那個
//	   「已經 COMPLETED 就不再打 AMEGO」的 resume checkpoint **永遠走不到**
//	   （撈不到那一列，先回 error 了）。重投時的行為因此從「安靜收尾」變成「報錯」。
//	② 它依賴「同一張發票同時只有一筆非終態 ALLOWANCE」這個假設。那個假設今天由
//	   uq_invadj_allowance_refund 成立，但它是 partial unique（帶 refund_no 條件）
//	   ——BUYER_CORRECTION／ADM_MANUAL 來源的折讓不在它的述詞內。
//
// invoice_operations.adjustment_id 是 chk 要求三種調整型別必填的欄位，直接用它沒有歧義。
func (u *Allowance) adjustmentOf(ctx context.Context, op domain.Operation) (domain.Adjustment, error) {
	if op.AdjustmentID == nil {
		return domain.Adjustment{}, fmt.Errorf(
			"invoice: operation %d has no adjustment_id (type=%s)", op.ID, op.OperationType)
	}
	return u.adjustments.GetByID(ctx, *op.AdjustmentID)
}

// ---------------------------------------------------------------------------
// operation 側的連動
//
// adjustment 與 operation 是**兩個狀態機**（M7 與 M2），一次動作要同時推。
// 分開推的後果是「折讓完成了但 operation 還在 SENDING」——那一列會被下一輪
// worker 重新撈起來再送一次，而折讓單是法定憑證。
// ---------------------------------------------------------------------------

// finishOperation 是 resume checkpoint 用的：adjustment 已 COMPLETED，只補 operation。
func (u *Allowance) finishOperation(ctx context.Context, op domain.Operation, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		return u.finishOperationTx(ctx, op, now)
	})
}

func (u *Allowance) finishOperationTx(ctx context.Context, op domain.Operation, now time.Time) error {
	// 🔴 這條路有**兩個**進入點，狀態不同：G09（settleAllowanceCompleted 由送出路徑
	// 呼叫，status=SENDING）與 G13（ConfirmAllowance 由 INV.rc 呼叫，claim 已把
	// status 蓋成 PROVIDER_STATUS_RECHECK）。原本只呼叫 MarkDone（守 SENDING）
	// ⇒ G13 那條必然 rows=0，而 rows 被丟給 `_`、軌跡照寫 ⇒ 折讓確認成功之後
	// operation 永遠留在 INV.rc 的候選集合裡。作廢軸的同一格已由 dev 實測坐實。
	return finishOperationSucceeded(ctx, u.ops, u.audit, op, now)
}

func (u *Allowance) rescheduleOperationTx(
	ctx context.Context, op domain.Operation, attempt int32, nextAt time.Time, lastErr string, now time.Time,
) error {
	_ = attempt // UpdateAttempt 已在呼叫端寫過，這裡只補歷程
	_ = nextAt
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusRetryScheduled.Value, "", lastErr, "", now)
}

func (u *Allowance) exhaustOperationTx(
	ctx context.Context, op domain.Operation, lastErr string, now time.Time,
) error {
	if _, err := u.ops.MarkRetryExhausted(ctx, op.ID, lastErr, now); err != nil {
		return err
	}
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusFailedRetryExhausted.Value, "", lastErr, "", now)
}

func (u *Allowance) failOperationTx(
	ctx context.Context, op domain.Operation, reasonCode, providerCode, providerMessage, detail string, now time.Time,
) error {
	// 🔴 走 MarkFailedWithProviderResult 而不是 MarkFailed（#3082）：
	// provider_code／provider_message 兩個專用欄位在此之前**從未被任何路徑寫入**，
	// 而 ADM 的服務商往來就靠它們分辨同一個數字碼的不同語意。
	if _, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
		ID:              op.ID,
		ProviderCode:    providerCode,
		ProviderMessage: providerMessage,
		LastError:       detail,
		Now:             now,
	}); err != nil {
		return err
	}
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusFailedPermanent.Value, reasonCode, detail, "", now)
}

func (u *Allowance) waitOperationTx(ctx context.Context, op domain.Operation, now time.Time) error {
	ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
		ID:            op.ID,
		NextAttemptAt: now.Add(u.cfg.RecheckBase),
		LastError:     "allowance result unknown",
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not enter WAITING_CONFIRMATION (rows=0, snapshot status=%s)",
			op.ID, op.Status)
	}
	return appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
		op.Status, codemeta.InvoiceOperationStatusWaitingConfirmation.Value, "", "", now)
}

// adjustmentAlreadySettled 回報 M3 上這個狀態「已經不需要（也不可能）再被推去
// FAILED_UNKNOWN」。
//
// 四個值分兩類，但對呼叫端的意義相同——**adjustment 這一半沒有事情要做**：
//   - FAILED_UNKNOWN：已經在目標狀態（G15 冪等重入，J20 週期重查的必然結果）
//   - COMPLETED／CANCELLED：M3 終態
//   - FAILED_PERMANENT：只有 ADM 的 G16 重試推得動它，自動軸不該覆寫
//
// ⚠️ 這裡**不是**「adjustment 的終態集合」，不要拿去當那個用：FAILED_UNKNOWN 有兩條
// 出邊（G17／G18），FAILED_PERMANENT 有一條（G16）。本函式回答的是一個更窄的問題。
func adjustmentAlreadySettled(status string) bool {
	switch status {
	case codemeta.InvoiceAdjustmentStatusFailedUnknown.Value,
		codemeta.InvoiceAdjustmentStatusCompleted.Value,
		codemeta.InvoiceAdjustmentStatusCancelled.Value,
		codemeta.InvoiceAdjustmentStatusFailedPermanent.Value:
		return true
	default:
		return false
	}
}

// outboxAggregateAdjustment 是 invoice_outbox_events.aggregate_type 的折讓軸。
const outboxAggregateAdjustment = "ADJUSTMENT"

// ---------------------------------------------------------------------------
// 確認軸：G13 / G14 / G15
// ---------------------------------------------------------------------------

// ConfirmAllowance 是折讓的「結果未明」收斂路徑，由 INV.rc 分派進來。
//
// 🔴 折讓單**不能**走 recheck 預設的 probeOperation：那條打的是 invoice_query
// type=order，會查到那張仍然存在的**原發票**並判成「開立成功」⇒ 把折讓 operation
// 標成完成，而折讓到底開沒開立沒有人知道。折讓只有一種問法——拿我方自產的
// allowance_number 打 allowance_query（AMEGO 開立折讓不回傳任何識別碼，
// 所以那個號碼是「送出成功但回應遺失」時唯一的救回路徑，見 InsertInvoiceAdjustment）。
// 同一形狀的坑在作廢軸已經踩過一次，見 RecheckVoidConfirmer 的說明。
func (u *Allowance) ConfirmAllowance(ctx context.Context, op domain.Operation) error {
	adj, err := u.adjustmentOf(ctx, op)
	if err != nil {
		return err
	}
	if adj.Status == codemeta.InvoiceAdjustmentStatusCompleted.Value {
		// 已經確認過（重投）。operation 補收尾即可。
		return u.finishOperation(ctx, op, u.now())
	}
	if adj.AllowanceNumber == nil || *adj.AllowanceNumber == "" {
		// 沒有單號 ⇒ 無從查起，而且這代表建列那一段沒把單號寫進去（chk 應該擋住了）。
		// 重試不會長出單號來 ⇒ 走人工。
		return u.settleAllowancePermanent(ctx, adj, op,
			// #3082：折讓單自己缺單號，與原發票沒號碼、與超額都是不同的事。
			codemeta.InvoiceManualReasonAllowanceNumberMissing.Value, "", "",
			"折讓單缺 allowance_number，無法查詢確認", u.now())
	}

	doc, err := u.docs.GetByID(ctx, adj.DocumentID)
	if err != nil {
		return err
	}
	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	var detail *amego.AllowanceDetail
	var queryErr error
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		detail, queryErr = u.provider.QueryAllowance(auditCtx,
			amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey},
			*adj.AllowanceNumber)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	if queryErr != nil || detail == nil || detail.AllowanceNumber == "" {
		// 查不到 ⇒ 仍不確定（G14）。**不得**當成「沒開成功」而重送：折讓單號已經
		// 送出去了，重送會讓 AMEGO 端出現第二張同號憑證。
		u.log.Warn("invoice.allowance.confirm_query_failed",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID), zap.Error(queryErr))
		// 🔴 **ADM 取消在途時的收束路徑是 M2 的 J24／J26，不是 G14／G15。**
		//
		// 判別式逐字沿用 §7.5.4 第 2 條（也是 decideRecheck 用的那一個）：
		// cancel_requested_at IS NOT NULL AND cancel_confirmed_at IS NULL。
		//
		// 少了這一格，ADM 對一筆折讓 job 按下取消（端點 53：operation → CANCEL_REQUESTED、
		// adjustment → CANCELLED，同一交易）之後，INV.rc 會把它 claim 進
		// PROVIDER_STATUS_RECHECK 一路走到 G15，而 G15 的第一句
		// `MarkFailedUnknown` 的 allowed-from 只有 WAITING_CONFIRMATION
		// ⇒ rows=0 ⇒ 整個交易 rollback ⇒ operation 永遠不終結、每 ~2 分鐘真的打一次
		// AMEGO。dev 實測見 `evidence/A6/ALW-03/g15-experiment.txt`（9 筆外呼、
		// invoice_status_history 零列）。
		//
		// ⚠️ 折讓軸**不能**直接呼叫 decideRecheck：那支的 J23／J25 兩條分支對折讓是錯的
		// （J23 的「查到已開立」在上面走 settleAllowanceCompleted；J25 的「安全補送」
		// 對折讓不成立——單號已經送出去了，補送會讓 AMEGO 端出現第二張同號憑證）。
		// 共用的只有判別式與落點，不是整支決策函式。
		if op.CancelRequestedAt != nil && op.CancelConfirmedAt == nil {
			return u.settleAllowanceAdminCancel(ctx, adj, op, allowanceQueryNotFound(queryErr), now)
		}
		return u.scheduleAllowanceRecheck(ctx, adj, op, now)
	}

	// 查得到 ⇒ 折讓確實在 AMEGO 端成立（G13）。即使它之後被作廢（CancelDate ≠ 0），
	// 「曾經開立成功」仍然是事實，累計與明細照樣要落帳——作廢折讓是另一條邊（G19）。
	return u.settleAllowanceCompleted(ctx, adj, doc, op, now)
}

// allowanceQueryNotFound 回報這次 allowance_query 的失敗是不是 AMEGO 的 code 71
// （「查無資料」）。
//
// 🔴 只有 71 才能當成「確定沒開」：其餘失敗（逾時、51、憑證解不開、JSON 壞掉）都是
// 「仍不確定」，把它們一起判成沒開會讓 J24 在折讓其實已經成立時把 operation 收掉。
func allowanceQueryNotFound(err error) bool {
	var apiErr *amego.APIError
	if errors.As(err, &apiErr) {
		return apiErr.NotFound
	}
	return false
}

// settleAllowanceAdminCancel 是「ADM 取消在途」時的收束（M2 的 J24／J26）。
//
//	J24  PROVIDER_STATUS_RECHECK → CANCELLED_BY_ADMIN   code=71 ⇒ 確定沒開，取消成立
//	J26  PROVIDER_STATUS_RECHECK → CANCEL_BLOCKED_UNKNOWN 仍不確定 ⇒ 等 ADM 雙人確認（J28）
//
// 🔴 **adjustment 那一半刻意不動**：走到這裡代表端點 53 已經在同一交易內把它推到
// CANCELLED（終態），而 M3 上沒有「CANCELLED → 任何狀態」的邊。硬推的話 rows=0，
// 而把那個 rows=0 當錯誤正是 P0-2 的成因本身。
//
// 🔴 J24 **必須帶 cancel_evidence_kind = PROVIDER_QUERY_71**：J24 沒有第二個人
// （cancel_confirmed_by 恆為 NULL），那個值是 chk_invop_admin_cancel_needs_confirm
// 放行它的唯一依據，不帶就是 23514。cancel_confirmed_at 同樣要寫，否則
// 「ADM 取消在途」判別式永遠成立，這筆在資料上會一直看起來像還沒處理完。
func (u *Allowance) settleAllowanceAdminCancel(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, notIssued bool, now time.Time,
) error {
	if adj.Status != codemeta.InvoiceAdjustmentStatusCancelled.Value {
		// 端點 53 對「已在 SENDING」的 adjustment 刻意不動（租約持有中可能正在打
		// AMEGO）⇒ 這個組合是合法的。記一行讓對帳看得到：operation 要終結，
		// 而 adjustment 那一半留給 A06。
		u.log.Warn("invoice.allowance.admin_cancel_adjustment_not_settled",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID),
			zap.String("adjustment_status", adj.Status))
	}

	if notIssued {
		ev := codemeta.InvoiceCancelEvidenceKindProviderQuery71.Value
		completed := now
		confirmed := now
		next := codemeta.InvoiceOperationStatusCancelledByAdmin.Value
		ok, err := u.ops.MarkRecheckResolved(ctx, domain.MarkRecheckResult{
			ID:                 op.ID,
			NextStatus:         next,
			NextAttemptAt:      now,
			CompletedAt:        &completed,
			CancelEvidenceKind: &ev,
			CancelConfirmedAt:  &confirmed,
			Now:                now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf(
				"invoice: operation %d could not enter CANCELLED_BY_ADMIN (rows=0, snapshot status=%s)",
				op.ID, op.Status)
		}
		u.log.Info("invoice.allowance.admin_cancel_confirmed",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID))
		return appendStatusHistory(ctx, u.audit, subjectOperation, op.ID,
			op.Status, next, recheckReasonCode, "", now)
	}

	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	next := codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value
	lastErr := "allowance cancellation blocked: provider result still unknown"
	ok, err := u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
		ID:            op.ID,
		NextStatus:    next,
		NextAttemptAt: now.Add(recheckBackoff(elapsed)),
		LastError:     &lastErr,
		Now:           now,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf(
			"invoice: operation %d could not enter CANCEL_BLOCKED_UNKNOWN (rows=0, snapshot status=%s)",
			op.ID, op.Status)
	}
	return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
		op.Status, next, recheckReasonCode, lastErr, "", now)
}

// scheduleAllowanceRecheck 是 G14（繼續等）與 G15（超過 24h 放棄）的分岔。
//
// 退避階梯與放棄窗口一律沿用開票／作廢軸的 recheckBackoff／recheckGiveUpAfter：
// 各寫一份必然漂，而漂掉的那一份沒有任何東西會發現。
func (u *Allowance) scheduleAllowanceRecheck(
	ctx context.Context, adj domain.Adjustment, op domain.Operation, now time.Time,
) error {
	elapsed := time.Duration(0)
	if op.UnknownSince != nil {
		elapsed = now.Sub(*op.UnknownSince)
	}
	if elapsed <= recheckGiveUpAfter {
		// 🔴 rows 不得丟給 `_`：本句的 allowed-from 是
		// ('SENDING','PROVIDER_STATUS_RECHECK')，rows=0 代表這一列既不在送出中、
		// 也不在重查中 —— 那時「繼續等」根本沒寫進去，而退避與 unknown_since
		// 都靠這一句。無聲吞掉的後果是那一列被 60s 租約推著無限重查
		// （dev 第 3 輪實測）。
		ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
			ID:            op.ID,
			NextAttemptAt: now.Add(recheckBackoff(elapsed)),
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf(
				"invoice: operation %d could not re-enter WAITING_CONFIRMATION (rows=0, snapshot status=%s)",
				op.ID, op.Status)
		}
		return nil
	}

	// G15：放棄自動收斂。adjustment 與 operation 一起進 FAILED_UNKNOWN，
	// 兩者同一交易——只落一邊的話，ADM 看到的兩個狀態會互相矛盾。
	lastErr := "invoice: allowance confirmation still unknown after give-up window"
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.adjustments.MarkFailedUnknown(ctx, adj.ID, now)
		if err != nil {
			return err
		}
		switch {
		case ok:
			if err := appendStatusHistoryDetail(ctx, u.audit, subjectAdjustment, adj.ID,
				adj.Status, codemeta.InvoiceAdjustmentStatusFailedUnknown.Value, "", lastErr, "", now); err != nil {
				return err
			}
		case adjustmentAlreadySettled(adj.Status):
			// 🔴 rows=0 而 adjustment **已經在終態或已在目標狀態** ⇒ 這一半沒有事情要做，
			// 不是錯誤。最常見的來源是 J20：G15 放棄過一次之後 operation 是 FAILED_UNKNOWN，
			// 週期重查把它推回 PROVIDER_STATUS_RECHECK 再走一次 G15，而 adjustment
			// 早已是 FAILED_UNKNOWN（`MarkInvoiceAdjustmentFailedUnknown` 的 allowed-from
			// 只有 WAITING_CONFIRMATION）。
			//
			// 🔴 把它當錯誤的代價已經在 dev 上量到：整段包在同一個 `WithTx` 裡
			// ⇒ 一個 rows=0 讓**整筆 rollback** ⇒ operation 停在 PROVIDER_STATUS_RECHECK、
			// `next_attempt_at` 只被 claim 的租約推兩分鐘 ⇒ 每 ~2 分鐘真的打一次 AMEGO 的
			// 永久熱迴圈，而 ADM 的重試／取消兩顆按鈕都是灰的、
			// `invoice_adjustment_stuck_rows` 也看不到它（adjustment 已是終態）。
			//
			// 不變式是「**operation 必須終結**」：adjustment 這一半推不動時要放行、
			// 讓下面那句把 operation 落到 FAILED_UNKNOWN，而不是拖著它一起 rollback。
			u.log.Info("invoice.allowance.give_up_adjustment_already_settled",
				zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID),
				zap.String("adjustment_status", adj.Status))
		default:
			// 非終態、也不是目標狀態（PENDING／SENDING／BLOCKED_*）⇒ 真的是狀態漂移，
			// fail-closed 維持不變。
			return fmt.Errorf("invoice: adjustment %d could not enter FAILED_UNKNOWN (rows=0, status=%s)",
				adj.ID, adj.Status)
		}
		ok, err = u.ops.MarkRecheckUnresolved(ctx, domain.MarkRecheckResult{
			ID:            op.ID,
			NextStatus:    codemeta.InvoiceOperationStatusFailedUnknown.Value,
			NextAttemptAt: now.Add(recheckBackoff(elapsed)),
			LastError:     &lastErr,
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_UNKNOWN (rows=0)", op.ID)
		}
		u.log.Error("invoice.allowance.confirm_gave_up",
			zap.Int64("operation_id", op.ID), zap.Int64("adjustment_id", adj.ID),
			zap.Duration("elapsed", elapsed))
		return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			op.Status, codemeta.InvoiceOperationStatusFailedUnknown.Value, "", lastErr, "", now)
	})
}
