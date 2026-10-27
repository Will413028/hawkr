//go:build !integration

package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 決策樹（§7.2.2）——純函式，逐節點
//
// 🔴 這棵樹的每一個分支都是稅務後果，而 fail-closed 的方向是**折讓**：
// 折讓在任何情況下都合法，作廢不是。所以下面的表把「往作廢走」當成需要五個條件
// 同時成立的窄路徑，其餘一律折讓。
// ---------------------------------------------------------------------------

// refundTestNow 是台北時間 2026-08-20 12:00（期別 11504 ＝ 7–8 月，尚未終結）。
var refundTestNow = time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)

// voidableDoc 造一張「五個條件全部成立」的發票 —— 決策樹唯一會走到 VOID 的形狀。
//
// 每一支測試從它出發、**只改一格**，那一格就是該支測試的變數。這樣寫的理由是：
// 造一個「剛好符合」的 fixture 很容易在無意間同時滿足兩個條件，於是測試以為自己
// 驗到了 A，實際上是 B 讓它通過的。
func voidableDoc() domain.Document {
	no := "AB12345678"
	period := "11504"
	var upload int16 = 99
	issueDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	return domain.Document{
		ID:                   9001,
		Status:               codemeta.InvoiceDocumentStatusIssued.Value,
		DocumentKind:         codemeta.InvoiceDocumentKindB2c.Value,
		FulfillmentMode:      codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
		TotalAmount:          1000,
		TaxAmount:            0,
		AllowanceTotalAmount: 0,
		ProviderInvoiceNo:    &no,
		TaxPeriodCode:        &period,
		ProviderUploadStatus: &upload,
		InvoiceDateTaipei:    &issueDate,
	}
}

func voidableFacts() refundFacts {
	return refundFacts{
		Doc:                    voidableDoc(),
		RefundAmount:           1000,
		CumulativeRefundAmount: 1000,
		Now:                    refundTestNow,
	}
}

func TestPlanRefundAdjustment(t *testing.T) {
	notReported := func() {
		// 前提自檢：測試日期必須真的落在 NOT_REPORTED，否則整張表的 VOID 期望值
		// 全部靠一個錯的前提。這一格失敗代表要改的是 refundTestNow 不是被測程式。
		got, err := domain.ReportedStatus("11504", refundTestNow)
		if err != nil || got != domain.ReportedStatusNotReported {
			t.Fatalf("前提不成立：ReportedStatus(11504, %v) = %q, %v，want NOT_REPORTED",
				refundTestNow, got, err)
		}
	}
	notReported()

	// SKIP 的分支驗 `Disposition`（CodeMeta 值，可 GROUP BY），
	// 其餘分支驗 `Reason`（那幾條沒有 disposition，Reason 就是它們的識別）。
	for _, c := range []struct {
		name        string
		mutate      func(*refundFacts)
		want        refundPlanKind
		reason      string
		disposition string
	}{
		{name: "五格全中 ⇒ 作廢", mutate: func(*refundFacts) {}, want: refundPlanVoid, reason: "FULL_REFUND_VOIDABLE"},

		{name: "非 AMEGO_AUTO ⇒ 不調整", mutate: func(f *refundFacts) {
			f.Doc.FulfillmentMode = codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value
		}, want: refundPlanSkip, disposition: codemeta.InvoiceInboxDispositionNotAmegoAuto.Value},

		{name: "收據 ⇒ 不調整", mutate: func(f *refundFacts) {
			f.Doc.DocumentKind = codemeta.InvoiceDocumentKindReceipt.Value
		}, want: refundPlanSkip, disposition: codemeta.InvoiceInboxDispositionNotAmegoAuto.Value},

		{name: "correction 佔用 ⇒ 退避重試", mutate: func(f *refundFacts) {
			f.CorrectionInFlight = true
		}, want: refundPlanRetryLater, reason: "CORRECTION_IN_FLIGHT"},

		{name: "開票結果未確認 ⇒ 退避重試", mutate: func(f *refundFacts) {
			f.IssueResultUnknown = true
		}, want: refundPlanRetryLater, reason: "ISSUE_RESULT_UNKNOWN"},

		{name: "還沒開出來 ⇒ 退避重試", mutate: func(f *refundFacts) {
			f.Doc.Status = codemeta.InvoiceDocumentStatusIssuing.Value
		}, want: refundPlanRetryLater, reason: "DOCUMENT_NOT_ISSUED_YET"},

		{name: "已作廢 ⇒ 沒有可調整的發票", mutate: func(f *refundFacts) {
			f.Doc.Status = codemeta.InvoiceDocumentStatusVoided.Value
		}, want: refundPlanSkip, disposition: codemeta.InvoiceInboxDispositionNoAdjustableDocument.Value},

		// B 節點必須在 C 之前：AMEGO 3050141「已存在折讓單則不得作廢」。
		// 順序寫反的話「先部分退 200、再退剩下的 800」會走到 VOID 而被 AMEGO 拒絕。
		{name: "已有折讓 ⇒ 補折讓（不得作廢）", mutate: func(f *refundFacts) {
			f.Doc.AllowanceTotalAmount = 200
			f.RefundAmount = 800
			f.CumulativeRefundAmount = 1000
		}, want: refundPlanAllowance, reason: "HAS_EXISTING_ALLOWANCE"},

		{name: "部分退款 ⇒ 折讓", mutate: func(f *refundFacts) {
			f.RefundAmount = 300
			f.CumulativeRefundAmount = 300
		}, want: refundPlanAllowance, reason: "PARTIAL_REFUND"},

		{name: "沒有期別碼 ⇒ 折讓（fail-closed）", mutate: func(f *refundFacts) {
			f.Doc.TaxPeriodCode = nil
		}, want: refundPlanAllowance, reason: "NO_TAX_PERIOD_CODE"},

		// 期已結、申報期限未到 ⇒ UNKNOWN ⇒ 折讓。這一格是 fail-closed 的核心：
		// 「不確定有沒有申報」時作廢是稅務違規，折讓不是。
		{name: "申報狀態 UNKNOWN ⇒ 折讓", mutate: func(f *refundFacts) {
			f.Now = time.Date(2026, 9, 5, 4, 0, 0, 0, time.UTC)
		}, want: refundPlanAllowance, reason: "REPORTED_STATUS_UNKNOWN"},

		{name: "上傳未完成 ⇒ 折讓", mutate: func(f *refundFacts) {
			var s int16 = 1
			f.Doc.ProviderUploadStatus = &s
		}, want: refundPlanAllowance, reason: "UPLOAD_NOT_COMPLETED"},

		{name: "沒有上傳狀態 ⇒ 折讓", mutate: func(f *refundFacts) {
			f.Doc.ProviderUploadStatus = nil
		}, want: refundPlanAllowance, reason: "UPLOAD_NOT_COMPLETED"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := voidableFacts()
			c.mutate(&f)
			got := planRefundAdjustment(f)
			if got.Kind != c.want {
				t.Errorf("Kind = %d，want %d（reason=%s）", got.Kind, c.want, got.Reason)
			}
			if c.reason != "" && got.Reason != c.reason {
				t.Errorf("Reason = %q，want %q", got.Reason, c.reason)
			}
			if c.disposition != "" && got.Disposition != c.disposition {
				t.Errorf("Disposition = %q，want %q —— 它要能 GROUP BY，"+
					"所以必須是 CodeMeta 的值而不是拼出來的字串", got.Disposition, c.disposition)
			}
			if c.disposition == "" && got.Disposition != "" {
				t.Errorf("非 SKIP 的分支帶了 Disposition=%q", got.Disposition)
			}
		})
	}
}

// TestPlanRefundAdjustment_VoidLockIsShadowedByReportedStatus 記錄一個**真實發現**：
// 在現行 ReportedStatus 算式下，D 節點嚴格強於 E 節點 ⇒ E 今天走不到 false。
//
// 推導：NOT_REPORTED 的條件是 `now < 期末`（PeriodEndTaipei），而關帳鎖定
// （ProviderCloseDeadlineTaipei）在**次期**首月 5 日 23:59:59
// ⇒ `now < 期末 < 關帳日`，E 的述詞恆真。
//
// 為什麼要寫這一支而不是刪掉 E：一支「用某個日期驗 E 擋下來」的測試在今天是**假的**
// ——擋下它的其實是 D，而測試會綠。與其留一支驗錯節點的測試，不如把「它今天走不到」
// 這件事本身釘住：ReportedStatus 的邊界日後若調整，本測試會轉紅，那時再回去看 E。
func TestPlanRefundAdjustment_VoidLockIsShadowedByReportedStatus(t *testing.T) {
	const period = "11504"
	end, err := domain.PeriodEndTaipei(period)
	if err != nil {
		t.Fatalf("PeriodEndTaipei: %v", err)
	}
	lock, err := domain.ProviderCloseDeadlineTaipei(period)
	if err != nil {
		t.Fatalf("ProviderCloseDeadlineTaipei: %v", err)
	}
	if !end.Before(lock) {
		t.Fatalf("期末 %v 不早於關帳日 %v —— 前提變了，E 節點可能已經有效，回去看 planRefundAdjustment 的 ⚠️", end, lock)
	}

	// 關帳日前一刻：D 已經是 UNKNOWN（期已結、申報期限未到）⇒ 走折讓，而**不是**因為 E。
	f := voidableFacts()
	f.Now = lock.Add(-time.Second)
	got := planRefundAdjustment(f)
	if got.Reason != "REPORTED_STATUS_UNKNOWN" {
		t.Errorf("關帳日前一刻的落點 = %q，want REPORTED_STATUS_UNKNOWN "+
			"（若這裡變成 PAST_VOID_LOCK 或 FULL_REFUND_VOIDABLE，代表 D／E 的關係變了）", got.Reason)
	}
}

// TestPlanRefundAdjustment_CumulativeTakesMax 守「左手邊取 max」。
//
// 上游的 cumulative_refund_amount 可能落後於我方帳上的事實（aftersales 每開一張新售後單
// 都拿 orders.total_actual_amount 當可退上限、不扣既退）。取小的那個會讓「其實已經退光」
// 被判成部分退款 ⇒ 永遠走不到 VOID 分支。
func TestPlanRefundAdjustment_CumulativeTakesMax(t *testing.T) {
	f := voidableFacts()
	f.Doc.AllowanceTotalAmount = 0
	f.RefundAmount = 1000
	f.CumulativeRefundAmount = 400 // 上游落後

	if got := planRefundAdjustment(f); got.Kind != refundPlanVoid {
		t.Fatalf("Kind = %d（%s），want VOID —— 左手邊沒有取 max(cumulative, 已折讓+本次)",
			got.Kind, got.Reason)
	}
}

// ---------------------------------------------------------------------------
// Apply —— 建列（G01／G02／G03）
// ---------------------------------------------------------------------------

type refundAdjs struct {
	byRefund   map[string]domain.Adjustment
	inserted   []domain.NewAdjustment
	lines      map[int64][]domain.NewAdjustmentLine
	notInsert  bool
	nextNo     string
	insertErr  error
	getByRefEr error
}

func newRefundAdjs() *refundAdjs {
	return &refundAdjs{
		byRefund: map[string]domain.Adjustment{},
		lines:    map[int64][]domain.NewAdjustmentLine{},
		nextNo:   "A000000000001",
	}
}

func (f *refundAdjs) GetByRefund(_ context.Context, _ int64, refundNo string) (domain.Adjustment, bool, error) {
	if f.getByRefEr != nil {
		return domain.Adjustment{}, false, f.getByRefEr
	}
	a, ok := f.byRefund[refundNo]
	return a, ok, nil
}

func (f *refundAdjs) Insert(_ context.Context, in domain.NewAdjustment) (domain.Adjustment, bool, error) {
	if f.insertErr != nil {
		return domain.Adjustment{}, false, f.insertErr
	}
	f.inserted = append(f.inserted, in)
	if f.notInsert {
		return domain.Adjustment{}, false, nil
	}
	return domain.Adjustment{ID: in.ID, DocumentID: in.DocumentID, Status: in.Status}, true, nil
}

func (f *refundAdjs) InsertLines(_ context.Context, adjID int64, lines []domain.NewAdjustmentLine, _ time.Time) error {
	f.lines[adjID] = lines
	return nil
}

func (f *refundAdjs) NextAllowanceNumber(context.Context) (string, error) { return f.nextNo, nil }

type refundDocs struct {
	lines []domain.DocumentLine
}

func (f *refundDocs) ListLines(context.Context, int64) ([]domain.DocumentLine, error) {
	return f.lines, nil
}

type promotion struct {
	id int64
	to string
}

type refundOps struct {
	onDoc    []domain.Operation
	created  []domain.NewOperation
	promoted []promotion

	// byKey 是 fake 的 uq_invop_operation_key。**必須真的維護**：
	// createOperation 在 Create 回 inserted=false 之後靠 GetByKey 分辨兩種 rows=0，
	// 而只記 calls 的替身對「有沒有回讀」與「回讀結果怎麼用」零鑑別力。
	byKey map[string]domain.Operation

	// gateHeld 模擬撞 uq_invop_open_adjustment_per_document（I5 序列閘門被別的調整
	// 佔用）：Create 回 inserted=false 且 **key 不進 byKey** ⇒ GetByKey 必然查不到。
	// 這正是真實 DB 的行為——那一列根本沒插進去，所以那把 key 不存在。
	gateHeld bool
}

// errFakeOperationKeyNotFound 是 fake 的 pgx.ErrNoRows（真實 repo 會翻成
// bizcode.InvoiceJobNotFound）。
var errFakeOperationKeyNotFound = errors.New("fake: operation_key not found")

func (f *refundOps) GetByKey(_ context.Context, key string) (domain.Operation, error) {
	op, ok := f.byKey[key]
	if !ok {
		return domain.Operation{}, errFakeOperationKeyNotFound
	}
	return op, nil
}

func (f *refundOps) ListByDocument(context.Context, int64) ([]domain.Operation, error) {
	return f.onDoc, nil
}

func (f *refundOps) Create(_ context.Context, in domain.NewOperation) (domain.Operation, bool, error) {
	f.created = append(f.created, in)
	if f.byKey == nil {
		f.byKey = map[string]domain.Operation{}
	}
	if f.gateHeld {
		// ON CONFLICT DO NOTHING 命中 partial 索引：**沒有任何列被插入**，
		// 所以這把 key 之後也查不到。
		return domain.Operation{}, false, nil
	}
	if existing, dup := f.byKey[in.OperationKey]; dup {
		// 同一把 key 重送：也是 rows=0，但那一列**存在**。
		return existing, false, nil
	}
	// 🔴 回 CREATED 而不是 READY —— 那是 SQL 的 DEFAULT。
	// 回 READY 的 fake 會讓「有沒有 promote」這件事看不出來，而 CREATED 的 operation
	// 不在 INV.op 的候選述詞裡（READY／RETRY_SCHEDULED／SENDING）⇒ 沒有任何執行者。
	op := domain.Operation{ID: in.ID, Status: codemeta.InvoiceOperationStatusCreated.Value}
	f.byKey[in.OperationKey] = op
	return op, true, nil
}

func (f *refundOps) Promote(_ context.Context, id int64, nextStatus string, _, _ time.Time) (bool, error) {
	f.promoted = append(f.promoted, promotion{id: id, to: nextStatus})
	return true, nil
}

// refundHarness 把四個替身與 usecase 綁在一起。
type refundHarness struct {
	uc   *RefundAdjustment
	adjs *refundAdjs
	docs *refundDocs
	ops  *refundOps
}

func newRefundHarness(t *testing.T) *refundHarness {
	t.Helper()
	adjs, docs, ops := newRefundAdjs(), &refundDocs{lines: refundDocLines()}, &refundOps{}
	var seq int64 = 5000
	uc := NewRefundAdjustment(directTx{}, adjs, docs, ops, &fakeAudit{},
		func() int64 { seq++; return seq },
		IssuanceConfig{MaxAttempts: 13},
		true, nil, func() time.Time { return refundTestNow })
	return &refundHarness{uc: uc, adjs: adjs, docs: docs, ops: ops}
}

// refundDocLines 造兩行商品 ＋ 一行運費，含稅基數合計 1000。
func refundDocLines() []domain.DocumentLine {
	return []domain.DocumentLine{
		{ID: 8001, LineSeq: 1, LineType: "PRODUCT", Description: "有機高麗菜",
			Quantity: 2, UnitPrice: 300, NetAmount: 600, AllowanceableBase: 600, TaxCategory: 1},
		{ID: 8002, LineSeq: 2, LineType: "PRODUCT", Description: "紅蘿蔔",
			Quantity: 1, UnitPrice: 300, NetAmount: 300, AllowanceableBase: 300, TaxCategory: 1},
		{ID: 8003, LineSeq: 3, LineType: "SHIPPING", Description: "運費",
			Quantity: 1, UnitPrice: 100, NetAmount: 100, AllowanceableBase: 100, TaxCategory: 1},
	}
}

// issuedDoc 是一張已開立、可折讓的 B2C 發票（總額 1000）。
func issuedDoc() domain.Document {
	d := voidableDoc()
	// 讓它落在「部分退款」分支：本節驗的是建列，不是決策樹。
	return d
}

func refundInput(amount int64) RefundAdjustmentInput {
	return RefundAdjustmentInput{
		OrderNo: "O000000123", RefundRef: "RF-0001",
		RefundAmount: amount, CumulativeRefundAmount: amount,
		OrderTotalActualAmount: 1000, OccurredAt: refundTestNow,
	}
}

// TestApply_PartialRefundCreatesAllowanceWithLinesAndOperation 是折讓軸的**入口**測試。
//
// 它一次驗四件事，因為少任何一件這條軸都不會動：adjustment 建了、明細建了、
// operation 建了、而且 operation_key 的前綴是 allowance:。
func TestApply_PartialRefundCreatesAllowanceWithLinesAndOperation(t *testing.T) {
	h := newRefundHarness(t)
	if _, err := h.uc.Apply(context.Background(), refundInput(300), issuedDoc()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.adjs.inserted) != 1 {
		t.Fatalf("建了 %d 筆 adjustment，want 1", len(h.adjs.inserted))
	}
	got := h.adjs.inserted[0]
	if got.AdjustmentType != codemeta.InvoiceAdjustmentTypeAllowance.Value {
		t.Errorf("type = %q，want ALLOWANCE", got.AdjustmentType)
	}
	if got.Status != codemeta.InvoiceAdjustmentStatusPending.Value {
		t.Errorf("status = %q，want PENDING", got.Status)
	}
	if got.RefundNo == nil || *got.RefundNo != "RF-0001" {
		t.Errorf("refund_no = %v，want RF-0001 —— 它是 uq_invadj_allowance_refund 的冪等鍵",
			got.RefundNo)
	}
	// chk_invadj_allowance_shape：ALLOWANCE 必須帶齊三個金額欄 ＋ 單號 ＋ 日期。
	for name, v := range map[string]any{
		"allowance_number": got.AllowanceNumber, "allowance_date_taipei": got.AllowanceDateTaipei,
		"net_amount_ex_tax": got.NetAmountExTax, "tax_amount": got.TaxAmount, "gross_amount": got.GrossAmount,
	} {
		if isNilPtr(v) {
			t.Errorf("%s 是 nil —— chk_invadj_allowance_shape 會當場 23514", name)
		}
	}
	if *got.GrossAmount != 300 {
		t.Errorf("gross = %d，want 300（＝退款額）", *got.GrossAmount)
	}
	if *got.NetAmountExTax+*got.TaxAmount != *got.GrossAmount {
		t.Errorf("chk_invadj_allowance_sum 不成立：%d + %d != %d",
			*got.NetAmountExTax, *got.TaxAmount, *got.GrossAmount)
	}

	lines := h.adjs.lines[got.ID]
	if len(lines) == 0 {
		t.Fatal("沒有建折讓明細 —— 送一張沒有 ProductItem 的折讓單會被 AMEGO 拒絕")
	}
	var sum int64
	for _, l := range lines {
		sum += l.NetAmountExTax + l.TaxAmount
		if l.SourceLineID == nil {
			t.Errorf("line_seq=%d 沒有 source_line_id —— 逐行累計 allowanced_amount 會落空",
				l.LineSeq)
		}
	}
	if sum != 300 {
		t.Errorf("明細含稅合計 = %d，want 300", sum)
	}

	if len(h.ops.created) != 1 {
		t.Fatalf("建了 %d 筆 operation，want 1 —— 沒有它折讓永遠不會被送出", len(h.ops.created))
	}
	op := h.ops.created[0]
	if op.OperationType != codemeta.InvoiceOperationTypeAllowance.Value {
		t.Errorf("operation_type = %q，want ALLOWANCE", op.OperationType)
	}
	wantKey, _ := domain.OperationKeyFor(codemeta.InvoiceOperationTypeAllowance.Value, 0, got.ID, 0, 0)
	if op.OperationKey != wantKey {
		t.Errorf("operation_key = %q，want %q", op.OperationKey, wantKey)
	}
	if op.AdjustmentID != got.ID {
		t.Errorf("operation.adjustment_id = %d，want %d", op.AdjustmentID, got.ID)
	}

	// 🔴 建完必須 promote 到 READY。
	//
	// `Create` 落在 `CREATED`（SQL 的 DEFAULT），而 INV.op 的候選述詞是
	// `status IN ('READY','RETRY_SCHEDULED','SENDING')` ⇒ 停在 CREATED 的 operation
	// **沒有任何執行者**，而且完全無聲：adjustment 是 PENDING、operation 是 CREATED，
	// 兩者看起來都「剛建好、等著跑」。
	//
	// 這一格是 dev E2E 抓到的——第一次跑建出了正確的 adjustment ＋ 明細 ＋ operation，
	// 然後**什麼都沒發生**。單元測試當時全綠，因為 fake 的 Create 回的是 READY。
	if len(h.ops.promoted) != 1 {
		t.Fatalf("沒有 promote（promoted=%v）—— operation 會停在 CREATED，"+
			"而 INV.op 的候選述詞不含它，沒有任何執行者", h.ops.promoted)
	}
	if h.ops.promoted[0].to != codemeta.InvoiceOperationStatusReady.Value {
		t.Errorf("promote 到 %q，want READY", h.ops.promoted[0].to)
	}
	if h.ops.promoted[0].id != op.ID {
		t.Errorf("promote 的是 operation %d，want %d", h.ops.promoted[0].id, op.ID)
	}
}

// TestApply_FullRefundCreatesVoidWithoutLines 驗 VOID 分支的形狀。
//
// chk_invadj_allowance_shape 是**雙向**的：VOID 帶了任何一個折讓欄就是 23514。
func TestApply_FullRefundCreatesVoidWithoutLines(t *testing.T) {
	h := newRefundHarness(t)
	if _, err := h.uc.Apply(context.Background(), refundInput(1000), issuedDoc()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(h.adjs.inserted) != 1 {
		t.Fatalf("建了 %d 筆，want 1", len(h.adjs.inserted))
	}
	got := h.adjs.inserted[0]
	if got.AdjustmentType != codemeta.InvoiceAdjustmentTypeVoid.Value {
		t.Fatalf("type = %q，want VOID（全額退款＋五格全中）", got.AdjustmentType)
	}
	for name, v := range map[string]any{
		"allowance_number": got.AllowanceNumber, "net_amount_ex_tax": got.NetAmountExTax,
		"tax_amount": got.TaxAmount, "gross_amount": got.GrossAmount,
	} {
		if !isNilPtr(v) {
			t.Errorf("VOID 帶了 %s —— chk_invadj_allowance_shape 是雙向的，會 23514", name)
		}
	}
	if len(h.adjs.lines) != 0 {
		t.Errorf("VOID 建了折讓明細 %v —— 作廢沒有明細", h.adjs.lines)
	}
	if len(h.ops.created) != 1 ||
		h.ops.created[0].OperationType != codemeta.InvoiceOperationTypeVoid.Value {
		t.Errorf("operation = %+v，want 一筆 VOID", h.ops.created)
	}
}

// TestApply_IsIdempotentByRefundRef 守冪等。
//
// 🔴 這是本 usecase 最重要的一條不變式：aftersales 的 RefundSyncWorker 每小時重發同一筆
// 退款、每次換新 event_id。冪等沒接好 ⇒ **每小時一張新折讓單**，而折讓是法定憑證。
func TestApply_IsIdempotentByRefundRef(t *testing.T) {
	h := newRefundHarness(t)
	h.adjs.byRefund["RF-0001"] = domain.Adjustment{ID: 4242, Status: "COMPLETED"}

	if _, err := h.uc.Apply(context.Background(), refundInput(300), issuedDoc()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(h.adjs.inserted) != 0 {
		t.Fatalf("已經開過折讓還是建了 %d 筆 —— 每小時一張新折讓單", len(h.adjs.inserted))
	}
	if len(h.ops.created) != 0 {
		t.Fatalf("已經開過折讓還是建了 %d 筆 operation", len(h.ops.created))
	}
}

// TestApply_InsertConflictDoesNotCreateOperation 守第二道冪等：唯一鍵。
//
// GetByRefund 沒攔到（兩個副本同時進來）時，Insert 走 ON CONFLICT DO NOTHING ⇒
// inserted=false。此時**不得**建 operation：那會讓同一筆 adjustment 有兩個執行者，
// 而它們會各打一次 AMEGO g0401。
func TestApply_InsertConflictDoesNotCreateOperation(t *testing.T) {
	h := newRefundHarness(t)
	h.adjs.notInsert = true

	if _, err := h.uc.Apply(context.Background(), refundInput(300), issuedDoc()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(h.ops.created) != 0 {
		t.Fatalf("Insert 衝突（rows=0）卻建了 %d 筆 operation —— 同一筆折讓會被送兩次",
			len(h.ops.created))
	}
}

// TestApply_NotAdjustableYetReturnsErrorNotSkip 守「現在做不了」與「刻意不做」的分野。
//
// 🔴 三種落點各有各的意思，混掉任一組都會讓事後查帳看到錯的東西：
//
//	error       現在還做不了 ⇒ inbox FAILED ＋ 退避重試（INV.dl 會再來）
//	RefundSkip  刻意不做     ⇒ inbox SKIPPED ＋ disposition（不會再有人碰它）
//	nil         做完了       ⇒ inbox PROCESSED
//
// 回 nil 的後果最嚴重：那筆退款會被標成「處理完成」，而折讓根本沒建，
// 也不會有任何東西再回頭看它。
func TestApply_NotAdjustableYetReturnsErrorNotSkip(t *testing.T) {
	for _, c := range []struct {
		name   string
		mutate func(*refundHarness, *domain.Document)
	}{
		{"開票結果未確認", func(h *refundHarness, d *domain.Document) {
			d.Status = codemeta.InvoiceDocumentStatusIssuing.Value
			h.ops.onDoc = []domain.Operation{{
				OperationType: codemeta.InvoiceOperationTypeIssue.Value,
				Status:        codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
			}}
		}},

		{"correction 佔用", func(h *refundHarness, _ *domain.Document) {
			h.ops.onDoc = []domain.Operation{{
				OperationType: codemeta.InvoiceOperationTypeReissue.Value,
				Status:        codemeta.InvoiceOperationStatusReady.Value,
			}}
		}},

		{"發票還沒開出來", func(_ *refundHarness, d *domain.Document) {
			d.Status = codemeta.InvoiceDocumentStatusPending.Value
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newRefundHarness(t)
			doc := issuedDoc()
			c.mutate(h, &doc)

			skip, err := h.uc.Apply(context.Background(), refundInput(300), doc)
			if err == nil {
				t.Fatalf("回了 (%+v, nil) —— 呼叫端會標 PROCESSED 或 SKIPPED，"+
					"而這筆退款其實還沒被處理、也不該被放棄", skip)
			}
			if skip != nil {
				t.Errorf("同時回了 skip=%+v 與 error —— 兩者互斥", skip)
			}
			if len(h.adjs.inserted) != 0 || len(h.ops.created) != 0 {
				t.Errorf("還做不了卻建了東西：adj=%d op=%d", len(h.adjs.inserted), len(h.ops.created))
			}
		})
	}
}

// TestApply_EmptyRefundRefIsRejected 守第二道 fail-loud。
//
// 空字串當 refund_no 會讓 uq_invadj_allowance_refund 的 partial 述詞
// （`refund_no IS NOT NULL`）不成立 ⇒ 冪等保護整個失效。
func TestApply_EmptyRefundRefIsRejected(t *testing.T) {
	h := newRefundHarness(t)
	in := refundInput(300)
	in.RefundRef = ""
	if _, err := h.uc.Apply(context.Background(), in, issuedDoc()); err == nil {
		t.Fatal("空 refund_ref 沒有被擋下 —— 冪等鍵失效，每次重投開一張新折讓單")
	}
	if len(h.adjs.inserted) != 0 {
		t.Fatalf("空 refund_ref 卻建了 %d 筆", len(h.adjs.inserted))
	}
}

// isNilPtr 判斷一個介面裡裝的指標是不是 nil。
func isNilPtr(v any) bool {
	switch p := v.(type) {
	case *string:
		return p == nil
	case *int64:
		return p == nil
	case *time.Time:
		return p == nil
	default:
		return v == nil
	}
}

// TestApply_SourceLineIDMapsBySeqNotByIndex 是 adjustmentLinesOf 那條紅字註解的靶心。
//
// BuildAllowanceDraft 會**跳過** a_i == 0 的行（`if a == 0 { continue }`），所以草稿的
// 切片索引與原明細的索引從第一個被跳過的行開始就錯開。用位置對的話，折讓額會累加到
// **別的品項**上 ⇒ 下一次折讓的剩餘基數算錯 ⇒ 累計超過發票金額，而 AMEGO 4040173
// 要到很後面才發現。
//
// 🔴 造這個情境需要「某一行已經折讓完」：第一行 allowanced_amount == allowanceable_base
// ⇒ 剩餘為 0 ⇒ 分攤到它的金額是 0 ⇒ 被跳過。這正是多次部分退款的真實形狀。
func TestApply_SourceLineIDMapsBySeqNotByIndex(t *testing.T) {
	h := newRefundHarness(t)
	// 第一行（id=8001）已折讓完 ⇒ 剩餘 0 ⇒ 草稿會跳過它。
	h.docs.lines[0].AllowancedAmount = h.docs.lines[0].AllowanceableBase

	doc := issuedDoc()
	doc.AllowanceTotalAmount = 600 // 與上面那一行的已折讓額一致
	if _, err := h.uc.Apply(context.Background(), refundInput(200), doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	adjID := h.adjs.inserted[0].ID
	lines := h.adjs.lines[adjID]
	if len(lines) != 2 {
		t.Fatalf("折讓明細 %d 行，want 2（第一行剩餘為 0 應被跳過）", len(lines))
	}
	// 剩下的兩行必須指向 8002 與 8003。用位置對的話會拿到 8001 與 8002。
	want := []int64{8002, 8003}
	for i, l := range lines {
		if l.SourceLineID == nil {
			t.Fatalf("第 %d 行沒有 source_line_id", i+1)
		}
		if *l.SourceLineID != want[i] {
			t.Errorf("第 %d 行 source_line_id = %d，want %d —— "+
				"用切片位置對而不是 line_seq 查表，折讓額會累加到別的品項上",
				i+1, *l.SourceLineID, want[i])
		}
	}
}

// TestApply_SkipReturnsDispositionNotNil 守「不需要調整 ≠ 處理完成」。
//
// 🔴 回 `(nil, nil)` 會讓呼叫端標 PROCESSED，而那個狀態說「業務動作做完了」——
// 這裡什麼都沒做。兩者合併之後，事後查「為什麼這筆退款沒有折讓單」只能靠猜，
// 而 invoice_inbox_events 存在的理由正是回答這個問題。
func TestApply_SkipReturnsDispositionNotNil(t *testing.T) {
	for _, c := range []struct {
		name   string
		mutate func(*domain.Document)
		want   string
	}{
		{"賣家自行開立", func(d *domain.Document) {
			d.FulfillmentMode = codemeta.InvoiceFulfillmentModeSellerSelfIssue.Value
		}, codemeta.InvoiceInboxDispositionNotAmegoAuto.Value},

		{"平台收據", func(d *domain.Document) {
			d.DocumentKind = codemeta.InvoiceDocumentKindReceipt.Value
		}, codemeta.InvoiceInboxDispositionNotAmegoAuto.Value},

		{"發票已作廢", func(d *domain.Document) {
			d.Status = codemeta.InvoiceDocumentStatusVoided.Value
		}, codemeta.InvoiceInboxDispositionNoAdjustableDocument.Value},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newRefundHarness(t)
			doc := issuedDoc()
			c.mutate(&doc)

			skip, err := h.uc.Apply(context.Background(), refundInput(300), doc)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if skip == nil {
				t.Fatal("回了 nil —— 呼叫端會標 PROCESSED，而這裡什麼都沒做")
			}
			if skip.Disposition != c.want {
				t.Errorf("disposition = %q，want %q", skip.Disposition, c.want)
			}
			if skip.Reason == "" {
				t.Error("reason 是空的 —— disposition 能 GROUP BY，但看的人需要知道是哪一格")
			}
			if len(h.adjs.inserted) != 0 || len(h.ops.created) != 0 {
				t.Errorf("SKIP 卻建了東西：adj=%d op=%d", len(h.adjs.inserted), len(h.ops.created))
			}
		})
	}
}

// TestApply_SuccessReturnsNilSkip 是上一支的反面。
//
// 兩支合起來才是完整的保證：「該 SKIP 的有 SKIP」＋「不該 SKIP 的沒有 SKIP」。
// 少了這一支，一個「一律回 RefundSkip」的實作會讓上一支全綠——而那會讓每一筆
// 真的建了折讓的退款都被標成 SKIPPED。
func TestApply_SuccessReturnsNilSkip(t *testing.T) {
	h := newRefundHarness(t)
	skip, err := h.uc.Apply(context.Background(), refundInput(300), issuedDoc())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if skip != nil {
		t.Fatalf("建了折讓卻回 %+v —— 呼叫端會標 SKIPPED，那筆折讓在帳上就變成「刻意不處理」", skip)
	}
}
