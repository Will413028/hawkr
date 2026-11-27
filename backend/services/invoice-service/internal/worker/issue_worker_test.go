package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

type fakeOps struct {
	domain.OperationRepository

	due            []domain.Operation
	byID           map[int64]domain.Operation
	releasedHolds  int
	promotedTrig   int
	markedDone     []int64
	markedFailed   []int64
	updatedAttempt map[int64]int32
	fetchErr       error
}

func newFakeOps(ops ...domain.Operation) *fakeOps {
	f := &fakeOps{byID: map[int64]domain.Operation{}, updatedAttempt: map[int64]int32{}}
	for _, op := range ops {
		f.due = append(f.due, op)
		f.byID[op.ID] = op
	}
	return f
}

func (f *fakeOps) ReleaseExpiredHolds(context.Context, time.Time) (int64, error) {
	f.releasedHolds++
	return 0, nil
}

func (f *fakeOps) PromoteDueTriggers(context.Context, time.Time) (int64, error) {
	f.promotedTrig++
	return 0, nil
}

func (f *fakeOps) FetchDue(context.Context, time.Time, int32) ([]domain.Operation, error) {
	return f.due, f.fetchErr
}

func (f *fakeOps) GetByID(_ context.Context, id int64) (domain.Operation, error) {
	op, ok := f.byID[id]
	if !ok {
		return domain.Operation{}, errors.New("not found")
	}
	return op, nil
}

func (f *fakeOps) MarkDone(_ context.Context, id int64, _ time.Time) (bool, error) {
	f.markedDone = append(f.markedDone, id)
	return false, nil
}

func (f *fakeOps) MarkFailed(_ context.Context, id int64, _ string, _ time.Time) (bool, error) {
	f.markedFailed = append(f.markedFailed, id)
	return true, nil
}

func (f *fakeOps) UpdateAttempt(_ context.Context, id int64, n int32, _ time.Time, _ *string, _ time.Time) error {
	f.updatedAttempt[id] = n
	return nil
}

type fakeDocsRepo struct {
	domain.DocumentRepository
	byID map[int64]domain.Document
}

func (f *fakeDocsRepo) GetByID(_ context.Context, id int64) (domain.Document, error) {
	d, ok := f.byID[id]
	if !ok {
		return domain.Document{}, errors.New("not found")
	}
	return d, nil
}

type claimRecord struct {
	id         int64
	now        time.Time
	leaseUntil time.Time
	at         time.Time
}

type fakeSubmitter struct {
	claims     []claimRecord
	submitted  []int64
	submitErr  error
	beforeExit func()
	strict     bool
	t          *testing.T
}

func strictSubmitter(t *testing.T) *fakeSubmitter {
	t.Helper()
	return &fakeSubmitter{strict: true, t: t}
}

func (f *fakeSubmitter) ClaimForSubmitAt(_ context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	if f.strict && f.t != nil {
		f.t.Errorf("非開票單卻走了 ClaimForSubmitAt(id=%d) —— 分派把它落到 default 分支了", id)
	}
	f.claims = append(f.claims, claimRecord{id: id, now: now, leaseUntil: leaseUntil, at: time.Now()})
	return true, nil
}

func (f *fakeSubmitter) Submit(_ context.Context, id int64) error {
	f.submitted = append(f.submitted, id)
	if f.beforeExit != nil {
		f.beforeExit()
	}
	return f.submitErr
}

func testIssueCfg(lease time.Duration) outbox.Config {
	return outbox.Config{
		MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute,
		FetchBatch: 50, ClaimLease: lease,
	}
}

func sendingOp(id int64, docID int64) domain.Operation {
	return domain.Operation{
		ID: id, DocumentID: docID,
		OperationType: codemeta.InvoiceOperationTypeIssue.Value,
		Status:        codemeta.InvoiceOperationStatusSending.Value,
		MaxAttempts:   13,
	}
}

// TestIssueWorker_LeaseBaselineIsPerRow 是**變異③的靶心**。
//
// 《多副本排程搶佔規範》：租約基準必須在 claim 當下逐筆取。在批次迴圈外算一次的話，
// 不變式會從「**單筆**最壞耗時 < 租約」退化成「**批次總**耗時 < 租約」——而後者隨
// batch-size 一起破：FetchBatch=50、單筆最壞 60.4s，批次總耗時可以是 50 分鐘，
// 而租約只有 120s ⇒ 批次後段的列在持有者還在打 AMEGO 時就被另一個副本合法搶走
// ⇒ **重複開票**。編譯、測試、/metrics 全部正常。
//
// 做法：讓第一筆的 Submit 花掉 40ms，然後斷言第二筆的 leaseUntil 至少晚 30ms。
// 基準若被提到迴圈外，兩筆的 leaseUntil 會完全相同 ⇒ 差值 0 ⇒ 轉紅。
func TestIssueWorker_LeaseBaselineIsPerRow(t *testing.T) {
	const processDelay = 40 * time.Millisecond

	ops := newFakeOps(sendingOp(1, 101), sendingOp(2, 102))
	docs := &fakeDocsRepo{byID: map[int64]domain.Document{}}
	sub := &fakeSubmitter{beforeExit: func() { time.Sleep(processDelay) }}

	w := NewIssueWorker(ops, docs, sub, strictVoider(t), strictAllowancer(t), testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(sub.claims) != 2 {
		t.Fatalf("claim 次數 = %d，want 2", len(sub.claims))
	}
	gap := sub.claims[1].leaseUntil.Sub(sub.claims[0].leaseUntil)
	if gap < processDelay*3/4 {
		t.Fatalf("兩筆的租約基準只差 %v（第一筆的處理花了 %v）—— 基準被提到批次迴圈外算了一次，"+
			"不變式已退化成「批次總耗時 < 租約」", gap, processDelay)
	}
	for i, c := range sub.claims {
		d := c.leaseUntil.Sub(c.now)
		if d < 120*time.Second {
			t.Errorf("第 %d 筆的租約長度 = %v，不得小於 120s", i+1, d)
		}
		if d > 120*time.Second+time.Second {
			t.Errorf("第 %d 筆的租約長度 = %v，遠超過 120s（基準算錯了）", i+1, d)
		}
	}
}

// TestIssueWorker_ClaimHappensBeforeSideEffect 守「claim 在副作用之前」。
// 反過來的話，兩個副本會同時打 AMEGO ⇒ 重複開票。
func TestIssueWorker_ClaimHappensBeforeSideEffect(t *testing.T) {
	ops := newFakeOps(sendingOp(1, 101))
	sub := &fakeSubmitter{}
	w := NewIssueWorker(ops, &fakeDocsRepo{byID: map[int64]domain.Document{}}, sub, strictVoider(t), strictAllowancer(t), testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(sub.claims) != 1 || len(sub.submitted) != 1 {
		t.Fatalf("claims=%d submits=%d", len(sub.claims), len(sub.submitted))
	}
	if !sub.claims[0].at.Before(time.Now()) {
		t.Fatal("時間軸壞了")
	}
	if sub.claims[0].id != sub.submitted[0] {
		t.Fatal("claim 的列與送出的列不同")
	}
}

// TestIssueWorker_RunsPrerequisiteUpdates 守 ⑪⑫ 兩支前置步驟真的被呼叫。
//
// 少了 PromoteDueTriggers，J05（AWAITING_TRIGGER → READY）**沒有任何元件會執行**
// ——不是編譯錯、不是測試紅，是那批列永遠不動。
func TestIssueWorker_RunsPrerequisiteUpdates(t *testing.T) {
	ops := newFakeOps()
	w := NewIssueWorker(ops, &fakeDocsRepo{}, &fakeSubmitter{}, strictVoider(t), strictAllowancer(t), testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if ops.releasedHolds != 1 {
		t.Errorf("ReleaseExpiredHolds 被呼叫 %d 次，want 1", ops.releasedHolds)
	}
	if ops.promotedTrig != 1 {
		t.Errorf("PromoteDueTriggers 被呼叫 %d 次，want 1（少了它 J05 沒有任何元件會執行）", ops.promotedTrig)
	}
}

// TestIssueWorker_SubmitErrorBumpsAttempt 守 transient 失敗（DB 斷線之類）帶著退避
// 回到候選集合，而不是被當成完成。
func TestIssueWorker_SubmitErrorBumpsAttempt(t *testing.T) {
	ops := newFakeOps(sendingOp(1, 101))
	sub := &fakeSubmitter{submitErr: errors.New("db is down")}
	w := NewIssueWorker(ops, &fakeDocsRepo{}, sub, strictVoider(t), strictAllowancer(t), testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if ops.updatedAttempt[1] != 1 {
		t.Fatalf("attempt_count = %d，want 1", ops.updatedAttempt[1])
	}
	if len(ops.markedDone) != 0 {
		t.Fatal("Submit 失敗時不得呼叫 MarkDone")
	}
}

// TestIssueResultOf 釘住 M2 終態 → M1 result 的映射。
//
// SELLER_ACTION_REQUIRED 刻意**不是**終態：賣家去財政部配完字軌之後它會回到迴圈，
// 算進去會讓同一個 operation 被計兩次（而 M1 的語意是「一輩子一次」）。
func TestIssueResultOf(t *testing.T) {
	terminal := map[string]string{
		codemeta.InvoiceOperationStatusSucceeded.Value:            "succeeded",
		codemeta.InvoiceOperationStatusFailedPermanent.Value:      "permanent_failed",
		codemeta.InvoiceOperationStatusFailedRetryExhausted.Value: "retry_exhausted",
		codemeta.InvoiceOperationStatusFailedUnknown.Value:        "unknown_terminal",
		codemeta.InvoiceOperationStatusCancelledByAdmin.Value:     "cancelled",
		codemeta.InvoiceOperationStatusCancelledBeforeIssue.Value: "cancelled",
		codemeta.InvoiceOperationStatusNotApplicable.Value:        "not_applicable",
		codemeta.InvoiceOperationStatusResolvedExternally.Value:   "resolved_externally",
	}
	for status, want := range terminal {
		got, ok := issueResultOf(status)
		if !ok || got != want {
			t.Errorf("issueResultOf(%s) = (%q, %v), want (%q, true)", status, got, ok, want)
		}
	}
	for _, status := range []string{
		codemeta.InvoiceOperationStatusSending.Value,
		codemeta.InvoiceOperationStatusRetryScheduled.Value,
		codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
		codemeta.InvoiceOperationStatusSellerActionRequired.Value,
	} {
		if _, ok := issueResultOf(status); ok {
			t.Errorf("issueResultOf(%s) 不該是終態", status)
		}
	}
}

type fakeVoider struct {
	claims    []claimRecord
	submitted []int64
	claimOK   bool
	claimErr  error
	submitErr error
	strict    bool
	t         *testing.T
}

func (f *fakeVoider) ClaimForVoidAt(_ context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	if f.strict && f.t != nil {
		f.t.Errorf("非作廢單卻走了 ClaimForVoidAt(id=%d) —— 分派寫反了", id)
	}
	f.claims = append(f.claims, claimRecord{id: id, now: now, leaseUntil: leaseUntil})
	return f.claimOK, f.claimErr
}

func (f *fakeVoider) SubmitVoid(_ context.Context, id int64) error {
	if f.strict && f.t != nil {
		f.t.Errorf("非作廢單卻走了 SubmitVoid(id=%d) —— 分派寫反了", id)
	}
	f.submitted = append(f.submitted, id)
	return f.submitErr
}

func strictVoider(t *testing.T) *fakeVoider {
	t.Helper()
	return &fakeVoider{strict: true, t: t}
}

type fakeAllowancer struct {
	claims    []claimRecord
	submitted []int64
	claimOK   bool
	claimErr  error
	submitErr error
	strict    bool
	t         *testing.T
}

func (f *fakeAllowancer) ClaimForAllowanceAt(_ context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	if f.strict && f.t != nil {
		f.t.Errorf("非折讓單卻走了 ClaimForAllowanceAt(id=%d) —— 分派寫反了", id)
	}
	f.claims = append(f.claims, claimRecord{id: id, now: now, leaseUntil: leaseUntil})
	return f.claimOK, f.claimErr
}

func (f *fakeAllowancer) SubmitAllowance(_ context.Context, id int64) error {
	if f.strict && f.t != nil {
		f.t.Errorf("非折讓單卻走了 SubmitAllowance(id=%d) —— 分派寫反了", id)
	}
	f.submitted = append(f.submitted, id)
	return f.submitErr
}

func strictAllowancer(t *testing.T) *fakeAllowancer {
	t.Helper()
	return &fakeAllowancer{strict: true, t: t}
}

func allowanceOp(id, docID int64) domain.Operation {
	op := sendingOp(id, docID)
	op.OperationType = codemeta.InvoiceOperationTypeAllowance.Value
	return op
}

func voidOp(id, docID int64) domain.Operation {
	op := sendingOp(id, docID)
	op.OperationType = codemeta.InvoiceOperationTypeVoid.Value
	return op
}

// TestIssueWorker_DispatchesByOperationType 一次驗三軸的**正向**分派。
//
// 為什麼三個一起驗而不是各寫一支：分派是一個 switch，錯法幾乎都是「某一支被別人
// 吃掉」——三筆同時進去才看得出「誰吃了誰」。三個 fake 各自只接受自己那一筆。
func TestIssueWorker_DispatchesByOperationType(t *testing.T) {
	ops := newFakeOps(sendingOp(1, 101), voidOp(2, 102), allowanceOp(3, 103))
	docs := &fakeDocsRepo{byID: map[int64]domain.Document{}}
	iss := &fakeSubmitter{}
	vd := &fakeVoider{claimOK: true}
	al := &fakeAllowancer{claimOK: true}

	w := NewIssueWorker(ops, docs, iss, vd, al, testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	for _, c := range []struct {
		name      string
		claims    []claimRecord
		submitted []int64
		wantID    int64
	}{
		{"開票軸", iss.claims, iss.submitted, 1},
		{"作廢軸", vd.claims, vd.submitted, 2},
		{"折讓軸", al.claims, al.submitted, 3},
	} {
		if len(c.claims) != 1 || c.claims[0].id != c.wantID {
			t.Errorf("%s 的 claim = %+v，want 只有 operation %d", c.name, c.claims, c.wantID)
		}
		if len(c.submitted) != 1 || c.submitted[0] != c.wantID {
			t.Errorf("%s 的 submit = %v，want [%d]", c.name, c.submitted, c.wantID)
		}
	}
}

// TestIssueWorker_AllowanceNeverGoesToIssueAxis 是**負向控制**。
//
// 上面那支驗「折讓走折讓軸」；這支驗它**不會**走另外兩軸——後者才是分派這件事的
// 實質保證（`default:` 分支吃掉折讓單是最可能的錯法，而它不會有任何錯誤訊息：
// 折讓單被拿去打 f0401 開票，AMEGO 那邊會開出一張莫名其妙的新發票）。
func TestIssueWorker_AllowanceNeverGoesToIssueAxis(t *testing.T) {
	ops := newFakeOps(allowanceOp(7, 107))
	al := &fakeAllowancer{claimOK: true}

	w := NewIssueWorker(ops, &fakeDocsRepo{byID: map[int64]domain.Document{}},
		strictSubmitter(t), strictVoider(t), al, testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(al.submitted) != 1 {
		t.Fatalf("折讓軸沒收到那一筆（submitted=%v）—— 分派把它送去別處了", al.submitted)
	}
}

// TestIssueWorker_UnroutableOperationTypeFailsClosed 守分派的白名單（2026-09-03）。
//
// 🔴 原本 Claim 用 `default:` fallthrough，把**任何**不是 VOID／ALLOWANCE 的型別
// 送去開票路徑。chk_invop_type 的值域有六個，另外兩個（QUERY／VOID_ALLOWANCE）
// 今天沒有生產者 ⇒ 這個洞完全靜默；哪天有人補上生產者，那一類會被拿去開票，
// 症狀是 `document ... could not enter ISSUING` —— 正是 voidSubmitter／
// allowanceSubmitter 檔頭警告的同一個形狀。
func TestIssueWorker_UnroutableOperationTypeFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		opTyp string
	}{
		{"作廢折讓（g0501，整條軸今天不存在）", codemeta.InvoiceOperationTypeVoidAllowance.Value},
		{"查詢對帳", codemeta.InvoiceOperationTypeQuery.Value},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := sendingOp(9, 109)
			op.OperationType = tc.opTyp
			ops := newFakeOps(op)
			iss := &fakeSubmitter{}
			w := NewIssueWorker(ops, &fakeDocsRepo{byID: map[int64]domain.Document{}},
				iss, strictVoider(t), strictAllowancer(t),
				testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
			_ = w.RunOnce(context.Background())
			if len(iss.claims) != 0 || len(iss.submitted) != 0 {
				t.Fatalf("%s 被開票軸接走了（claims=%v submits=%v）——"+
					"它會把 document 推去 ISSUING，而那條邊在 M1 不存在",
					tc.opTyp, iss.claims, iss.submitted)
			}
		})
	}
}

// 鑑別力的另一半：白名單不得把**真的**該走開票路徑的兩種型別也擋掉。
func TestIssueWorker_IssueAndReissueStillRouteToIssuePath(t *testing.T) {
	for _, opTyp := range []string{
		codemeta.InvoiceOperationTypeIssue.Value,
		codemeta.InvoiceOperationTypeReissue.Value,
	} {
		op := sendingOp(11, 111)
		op.OperationType = opTyp
		ops := newFakeOps(op)
		iss := &fakeSubmitter{}
		w := NewIssueWorker(ops, &fakeDocsRepo{byID: map[int64]domain.Document{}},
			iss, strictVoider(t), strictAllowancer(t),
			testIssueCfg(120*time.Second), testMetrics, testObsA, nil)
		if err := w.RunOnce(context.Background()); err != nil {
			t.Fatalf("%s RunOnce: %v", opTyp, err)
		}
		if len(iss.submitted) != 1 {
			t.Errorf("%s 沒有走開票路徑（submits=%v）", opTyp, iss.submitted)
		}
	}
}
