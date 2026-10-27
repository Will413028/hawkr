//go:build !integration

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
)

// ---------------------------------------------------------------------------
// fakes
//
// fakeInboxRepo 只實作 INV.dl 真的會用到的那幾支；其餘由嵌入的 nil 介面承接，**呼叫到
// 就 panic**。這是刻意的：deadlineInboxRepo 若哪天走了設計外的路徑（例如自己 MarkDeferred
// 把 DEFERRED 列翻掉），要在測試裡當場炸掉而不是靜默通過。
// ---------------------------------------------------------------------------

type dlClaim struct {
	id              int64
	leaseUntil      time.Time
	claimableBefore time.Time
	at              time.Time
}

type dlAttempt struct {
	id            int64
	attemptCount  int32
	nextAttemptAt time.Time
	at            time.Time
}

type dlFail struct {
	id            int64
	nextAttemptAt time.Time
	at            time.Time
}

type fakeInboxRepo struct {
	domain.InboxRepository

	rows                  []domain.InboxEvent
	fetchedType           []string
	fetchedActionableType []string
	claims                []dlClaim
	claimOK               bool
	markDoneOK            bool
	markDoneIDs           []int64
	attempts              []dlAttempt
	fails                 []dlFail
}

func (f *fakeInboxRepo) Insert(_ context.Context, in domain.NewInboxEvent) (int64, bool, error) {
	id := int64(len(f.rows) + 1)
	f.rows = append(f.rows, domain.InboxEvent{
		ID: id, EventID: in.EventID, EventType: in.EventType, Status: in.Status,
		AggregateRef: in.AggregateRef, Payload: in.Payload,
		NextAttemptAt: in.NextAttemptAt, OccurredAt: in.OccurredAt,
	})
	return id, true, nil
}

func (f *fakeInboxRepo) FetchDueByType(_ context.Context, eventType string, _ time.Time, _ int32) ([]domain.InboxEvent, error) {
	f.fetchedType = append(f.fetchedType, eventType)
	return f.rowsOf(eventType, true), nil
}

// FetchDueActionableByType 是**排除 DEFERRED** 的變體（#3073）。
//
// fake 必須真的把那一格差異做出來：實作成「呼叫 FetchDueByType」的話，
// 「退款走了哪一支」這件事在 fake 裡就消失了，而那正是本檔要驗的東西。
func (f *fakeInboxRepo) FetchDueActionableByType(_ context.Context, eventType string, _ time.Time, _ int32) ([]domain.InboxEvent, error) {
	f.fetchedActionableType = append(f.fetchedActionableType, eventType)
	return f.rowsOf(eventType, false), nil
}

// rowsOf 依 event_type 過濾，withDeferred=false 時另外排除 DEFERRED。
//
// 舊版回 `f.rows` 全部、不看型別——那在「只撈一次」的年代看不出問題，
// 但撈兩次之後每一列都會出現兩遍，型別分派就驗不到任何東西了。
func (f *fakeInboxRepo) rowsOf(eventType string, withDeferred bool) []domain.InboxEvent {
	var out []domain.InboxEvent
	for _, r := range f.rows {
		if r.EventType != "" && r.EventType != eventType {
			continue
		}
		if !withDeferred && r.Status == "DEFERRED" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (f *fakeInboxRepo) Claim(_ context.Context, id int64, leaseUntil, claimableBefore time.Time) (bool, error) {
	f.claims = append(f.claims, dlClaim{id: id, leaseUntil: leaseUntil, claimableBefore: claimableBefore, at: time.Now()})
	return f.claimOK, nil
}

func (f *fakeInboxRepo) MarkProcessed(_ context.Context, id int64, _ time.Time) (bool, error) {
	f.markDoneIDs = append(f.markDoneIDs, id)
	return f.markDoneOK, nil
}

func (f *fakeInboxRepo) MarkFailed(_ context.Context, id int64, _ string, nextAttemptAt time.Time) (bool, error) {
	f.fails = append(f.fails, dlFail{id: id, nextAttemptAt: nextAttemptAt, at: time.Now()})
	return true, nil
}

func (f *fakeInboxRepo) UpdateAttempt(_ context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, _ *string) (bool, error) {
	f.attempts = append(f.attempts, dlAttempt{id: id, attemptCount: attemptCount, nextAttemptAt: nextAttemptAt, at: time.Now()})
	return true, nil
}

type fakeReprocessor struct {
	calls   []int64
	firstAt time.Time
	err     error
	delay   time.Duration
}

func (f *fakeReprocessor) ReprocessFromInbox(_ context.Context, ev domain.InboxEvent) error {
	if len(f.calls) == 0 {
		f.firstAt = time.Now()
	}
	f.calls = append(f.calls, ev.ID)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.err
}

func dlRow(id int64, attempts int32) domain.InboxEvent {
	ref := fmt.Sprintf("O%09d", id)
	return domain.InboxEvent{
		ID:           id,
		EventType:    string(pkgmq.EventTypeOrderPaid),
		AggregateRef: &ref,
		AttemptCount: attempts,
		Status:       "RECEIVED",
	}
}

// newDLWorker 一律走 deadlineCfg()（＝生產參數），不在測試裡另組一份 outbox.Config。
func newDLWorker(repo domain.InboxRepository, uc InboxOrderPaidReprocessor) *DeadlineWorker {
	return newDLWorkerWithRefund(repo, uc, strictRefundReprocessor{})
}

// newDLWorkerWithRefund 讓退款那一軸也可以指定替身（#3073）。
func newDLWorkerWithRefund(repo domain.InboxRepository, uc InboxOrderPaidReprocessor, rf InboxRefundReprocessor) *DeadlineWorker {
	// K05 的 sweeper 傳 nil：本檔驗的是 inbox 兜底那一半，而 RunOnce 對未 wire 的
	// sweeper 安靜跳過（見該處註解）。K05 自己的行為由 usecase 側的測試守。
	return NewDeadlineWorker(repo, uc, rf, deadlineCfg(), dlParkInterval, nil, testObsC, nil)
}

// newDLWorkerWithSweeper 掛上申訴掃描那一半，用來驗 RunOnce 的**編排**。
func newDLWorkerWithSweeper(repo domain.InboxRepository, sw OverdueComplaintSweeper) *DeadlineWorker {
	return NewDeadlineWorker(repo, &fakeReprocessor{}, strictRefundReprocessor{},
		deadlineCfg(), dlParkInterval, sw, testObsC, nil)
}

// newDLWorkerWithBoth 同時掛申訴那三支與 §5.2.1 的逾期分類，用來驗四者的編排。
func newDLWorkerWithBoth(repo domain.InboxRepository, sw OverdueComplaintSweeper, st StatutoryDeadlineSweeper) *DeadlineWorker {
	return newDLWorkerWithSweeper(repo, sw).WithStatutorySweeper(st)
}

// dlStatutory 是 §5.2.1 逾期分類的替身。
type dlStatutory struct {
	n     int
	err   error
	calls int
}

func (s *dlStatutory) SweepOverdueStatutoryDeadlines(context.Context, time.Time) (int, error) {
	s.calls++
	return s.n, s.err
}

// 🔴 申訴那三支的失敗**不得**讓 §5.2.1 的逾期分類被跳過。
//
// 兩者是完全不同的軸（申訴 vs 發票文件），而逾期分類守的是法定上傳期限——
// 那是會罰錢的東西。讓申訴掃描的錯誤阻斷它，等於把一個故障放大成兩個。
func TestDeadlineWorker_ComplaintFailureDoesNotSkipStatutory(t *testing.T) {
	sw := &dlSweeper{sweepErr: errors.New("boom"), escErr: errors.New("boom"), revErr: errors.New("boom")}
	st := &dlStatutory{n: 2}
	w := newDLWorkerWithBoth(&fakeInboxRepo{claimOK: true, markDoneOK: true}, sw, st)

	if err := w.RunOnce(context.Background()); err == nil {
		t.Fatal("申訴掃描失敗必須往上回報")
	}
	if st.calls != 1 {
		t.Fatalf("逾期分類應照樣跑 1 次，實得 %d", st.calls)
	}
}

// 反向：逾期分類失敗也要回報，而且不影響申訴那三支已經跑完的事實。
func TestDeadlineWorker_StatutoryFailureIsReported(t *testing.T) {
	sw := &dlSweeper{sweepN: 1, escN: 1, revN: 1}
	st := &dlStatutory{err: errors.New("boom")}
	w := newDLWorkerWithBoth(&fakeInboxRepo{claimOK: true, markDoneOK: true}, sw, st)

	if err := w.RunOnce(context.Background()); err == nil {
		t.Fatal("逾期分類失敗必須往上回報")
	}
	if sw.sweepCalls != 1 || sw.escCalls != 1 || sw.revCalls != 1 || st.calls != 1 {
		t.Fatalf("四支都該跑到，實得 K04=%d K05=%d K06=%d 逾期=%d",
			sw.sweepCalls, sw.escCalls, sw.revCalls, st.calls)
	}
}

// 未掛時安靜跳過（nil 即跳過，與 overdue 同慣例）——但 DI 那側已改成必要依賴，
// 所以這一格守的是「本地／測試組裝路徑不會 panic」，不是生產路徑。
func TestDeadlineWorker_NilStatutoryIsSkipped(t *testing.T) {
	sw := &dlSweeper{sweepN: 1, escN: 1, revN: 1}
	w := newDLWorkerWithSweeper(&fakeInboxRepo{claimOK: true, markDoneOK: true}, sw)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("未掛逾期分類時應照常跑完：%v", err)
	}
}

// dlSweeper 是 K04／K05 的替身。**兩支各自記呼叫次數**——只記一個總數的替身對
// 「K04 失敗時 K05 有沒有跑到」零鑑別力，而那正是本節要守的東西。
type dlSweeper struct {
	sweepN, escN, revN             int
	sweepErr, escErr, revErr       error
	sweepCalls, escCalls, revCalls int
}

func (s *dlSweeper) ResolveExpiredBuyerReviews(context.Context, time.Time) (int, error) {
	s.revCalls++
	return s.revN, s.revErr
}

func (s *dlSweeper) SweepOverdueComplaints(context.Context, time.Time) (int, error) {
	s.sweepCalls++
	return s.sweepN, s.sweepErr
}

func (s *dlSweeper) EscalateOverdueComplaints(context.Context, time.Time) (int, error) {
	s.escCalls++
	return s.escN, s.escErr
}

// K04 失敗**不得**吃掉 K05。
//
// 🔴 這是 2026-09-08 修掉的真實缺陷：原本的 RunOnce 在「inbox 成功、K04 失敗」時直接
// `return err`，於是接在它後面的任何一段都被靜默跳過。而 K04 最可能失敗的時候
// （DB 不穩、通知 outbox 寫不進去）正是申訴最需要被升級的時候——兩者的候選集合
// 完全無關，讓其中一個的錯誤阻斷另一個等於把故障放大。
func TestDeadlineWorker_SweepFailureDoesNotSkipEscalate(t *testing.T) {
	sw := &dlSweeper{sweepErr: errors.New("boom"), escN: 2}
	w := newDLWorkerWithSweeper(&fakeInboxRepo{claimOK: true, markDoneOK: true}, sw)

	err := w.RunOnce(context.Background())
	if err == nil {
		t.Fatal("K04 失敗必須往上回報，不得靜默吞掉")
	}
	if sw.sweepCalls != 1 {
		t.Fatalf("K04 應被呼叫 1 次，實得 %d", sw.sweepCalls)
	}
	if sw.escCalls != 1 {
		t.Fatalf("K05 應照樣被呼叫 1 次（K04 的錯誤不得阻斷它），實得 %d", sw.escCalls)
	}
	if sw.revCalls != 1 {
		t.Fatalf("K06 也應照樣被呼叫 1 次，實得 %d", sw.revCalls)
	}
}

// 反向：K05 失敗也不得讓 K04 的成功被當成失敗以外的東西——兩支都要跑到，錯誤要回報。
func TestDeadlineWorker_EscalateFailureIsReported(t *testing.T) {
	sw := &dlSweeper{sweepN: 1, escErr: errors.New("boom")}
	w := newDLWorkerWithSweeper(&fakeInboxRepo{claimOK: true, markDoneOK: true}, sw)

	if err := w.RunOnce(context.Background()); err == nil {
		t.Fatal("K05 失敗必須往上回報")
	}
	if sw.sweepCalls != 1 || sw.escCalls != 1 || sw.revCalls != 1 {
		t.Fatalf("三支都該跑到，實得 K04=%d K05=%d K06=%d", sw.sweepCalls, sw.escCalls, sw.revCalls)
	}
}

// 兩支都成功時不得回錯誤（避免上面兩支測試被一個「永遠回 error」的實作矇混過去）。
func TestDeadlineWorker_BothSweepsSucceed(t *testing.T) {
	sw := &dlSweeper{sweepN: 1, escN: 1}
	w := newDLWorkerWithSweeper(&fakeInboxRepo{claimOK: true, markDoneOK: true}, sw)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("兩支都成功時不該回錯誤：%v", err)
	}
	if sw.sweepCalls != 1 || sw.escCalls != 1 || sw.revCalls != 1 {
		t.Fatalf("三支都該跑到，實得 K04=%d K05=%d K06=%d", sw.sweepCalls, sw.escCalls, sw.revCalls)
	}
}

// strictRefundReprocessor 被呼叫到就回 error：預設情境下不該有退款列走進來。
type strictRefundReprocessor struct{}

func (strictRefundReprocessor) ReprocessFromInbox(_ context.Context, ev domain.InboxEvent) error {
	return fmt.Errorf("非退款事件卻走了退款重跑入口（inbox_id=%d, event_type=%s）—— 分派寫反了",
		ev.ID, ev.EventType)
}

// recordingRefundReprocessor 記下走進來的 inbox id。
type recordingRefundReprocessor struct {
	seen []int64
	err  error
}

func (r *recordingRefundReprocessor) ReprocessFromInbox(_ context.Context, ev domain.InboxEvent) error {
	r.seen = append(r.seen, ev.ID)
	return r.err
}

// ---------------------------------------------------------------------------
// 搶佔規範（docs/開發規範/多副本排程搶佔規範.md）
// ---------------------------------------------------------------------------

// TestDeadlineWorker_ClaimHappensBeforeSideEffect 守規則 ①：搶佔在副作用之前。
//
// 反過來的話兩個副本會同時跑 §7.1.2 步驟 5–9（兩支 peer gRPC ＋ Prepare），
// 而 Prepare 的冪等只擋得住「重複建 document」，擋不住重複打 order/payment。
func TestDeadlineWorker_ClaimHappensBeforeSideEffect(t *testing.T) {
	repo := &fakeInboxRepo{rows: []domain.InboxEvent{dlRow(1, 0)}, claimOK: true, markDoneOK: true}
	uc := &fakeReprocessor{}
	if err := newDLWorker(repo, uc).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.claims) != 1 || len(uc.calls) != 1 {
		t.Fatalf("claims=%d reprocess=%d，want 1/1", len(repo.claims), len(uc.calls))
	}
	if !repo.claims[0].at.Before(uc.firstAt) && !repo.claims[0].at.Equal(uc.firstAt) {
		t.Fatal("副作用發生在 claim 之前")
	}
	if repo.claims[0].id != uc.calls[0] {
		t.Fatal("claim 的列與重跑的列不同")
	}
}

// TestDeadlineWorker_LostClaimSkipsSideEffect 守「rows=0 不得靜默當成成功」。
//
// claim 回 0 ＝ 這一輪被別的副本搶走。此時本副本**什麼都不能做**：既不能重跑
// （重複副作用），也不能記一次 attempt 失敗（那會替別人的成功列燒退避）。
func TestDeadlineWorker_LostClaimSkipsSideEffect(t *testing.T) {
	repo := &fakeInboxRepo{rows: []domain.InboxEvent{dlRow(1, 0)}, claimOK: false, markDoneOK: true}
	uc := &fakeReprocessor{}
	if err := newDLWorker(repo, uc).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(uc.calls) != 0 {
		t.Fatalf("claim 失敗仍跑了 %d 次副作用", len(uc.calls))
	}
	if len(repo.markDoneIDs)+len(repo.attempts)+len(repo.fails) != 0 {
		t.Fatalf("claim 失敗不得動任何狀態：markDone=%d attempt=%d fail=%d",
			len(repo.markDoneIDs), len(repo.attempts), len(repo.fails))
	}
}

// TestDeadlineWorker_LeaseBaselineIsPerRow 守 §三 第 6 點：租約基準逐筆在 claim 當下取。
//
// 提到批次迴圈外算一次的話，不變式會從「**單筆**最壞耗時 < 租約」退化成「**批次總**耗時
// < 租約」，隨 dlFetchBatch(20) 一起破。做法與 TestIssueWorker_LeaseBaselineIsPerRow 同：
// 讓第一筆花掉 40ms，再斷言第二筆的 leaseUntil 明顯晚於第一筆（共用基準時兩者完全相等）。
func TestDeadlineWorker_LeaseBaselineIsPerRow(t *testing.T) {
	const processDelay = 40 * time.Millisecond

	repo := &fakeInboxRepo{rows: []domain.InboxEvent{dlRow(1, 0), dlRow(2, 0)}, claimOK: true, markDoneOK: true}
	uc := &fakeReprocessor{delay: processDelay}
	if err := newDLWorker(repo, uc).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.claims) != 2 {
		t.Fatalf("claim 次數 = %d，want 2", len(repo.claims))
	}
	if gap := repo.claims[1].leaseUntil.Sub(repo.claims[0].leaseUntil); gap < processDelay*3/4 {
		t.Fatalf("兩筆的租約基準只差 %v（第一筆處理花了 %v）—— 基準被提到批次迴圈外算了一次", gap, processDelay)
	}
	// 租約長度本身仍是設定值（差距來自基準推移，不是租約變長）。
	for i, c := range repo.claims {
		if d := c.leaseUntil.Sub(c.at); d > dlClaimLease || d < dlClaimLease-time.Second {
			t.Errorf("第 %d 筆的租約長度 = %v，want ≈ %v", i+1, d, dlClaimLease)
		}
	}
	// CAS 上界（claimable_before）刻意**共用 tick 起點**：它讓候選集合更小，是保守方向。
	// 若哪天改成逐筆的 now，一列在本 tick 稍早被別的副本 claim 走、租約落在兩者之間時，
	// 本副本會再 claim 成功一次 ⇒ 搶佔失效。
	if !repo.claims[0].claimableBefore.Equal(repo.claims[1].claimableBefore) {
		t.Errorf("claimable_before 逐筆變動（%v vs %v）：CAS 上界必須共用 tick 起點",
			repo.claims[0].claimableBefore, repo.claims[1].claimableBefore)
	}
}

// TestDeadlineWorker_LeaseCoversWorstCaseRowTime 守 §三 第 5 點的**下界**。
//
// 🔴 這一支存在的理由寫在 di.go 的 dlFetchBatch 註解裡：那組數字是**算出來的**，
// 而它依賴 peer client 的 per-call deadline。日後有人把某支 peer 的 DefaultDeadline
// 調大，租約下界就可能不成立（持有者還在打 gRPC、租約已到期、另一個副本合法搶走
// 同一筆），而在補這支之前**沒有任何東西會轉紅**。
//
// 🔴 為什麼它必須成立，而不是「加個 fencing token 就好」：
// MarkDone／MarkFailed／UpdateAttempt 的 SQL 述詞只有 `id + status`，**沒有租約項**
// ⇒ 一個租約已到期、卻還在跑的殭屍持有者，它的寫入照樣會落在已被別人接手的那一列上。
// 換句話說**整條路徑的正確性靠的是一個數字關係**（租約 > 單筆最壞耗時），不是任何機制。
// 這一支就是那個數字關係的機械保證；它的鑑別力由
// TestIntegration_DLRace_ExpiredLeaseLetsASecondPodInWhileTheFirstStillRuns 證明
// （把租約縮到 80ms 讓關係不成立，量出重複處理）。
//
// 🔴 2026-08-31 從「手列 order + payment」改成「AST 掃出 internal/client 底下**全部**
// peer client 的 DefaultDeadline 並加總」。原本的寫法有一個安靜的缺口：它枚舉的是
// **今天**這條路徑會打的兩支 gRPC，所以「日後有人在 ReprocessFromInbox 加了第三個
// 外部呼叫」會讓 worst 變大而公式渾然不知 —— 前提破了，測試照樣綠。
// 加總全部 peer 會**高估** INV.dl 的真實最壞耗時（它只打 order 與 payment），
// 而高估是下界檢查的安全方向：寧可要求租約更長，也不要在前提破掉時保持綠燈。
func TestDeadlineWorker_LeaseCoversWorstCaseRowTime(t *testing.T) {
	// 單筆最壞耗時的骨幹＝全部 peer gRPC 的 deadline 加總；其餘（claim／Prepare 的
	// 本地交易／標記）都是本地 DB，用 1s 一起概括。
	const localOverhead = time.Second
	deadlines := peerClientDeadlines(t)
	// 鑑別力自檢：掃不到任何常數時「加總 = 0」看起來跟「全部很小」一模一樣。
	for _, must := range []string{"order", "payment"} {
		if _, ok := deadlines[must]; !ok {
			t.Fatalf("internal/client/%s 掃不到 DefaultDeadline（實際掃到 %v）—— "+
				"這不是「deadline 很短」，是這支檢查瞎了", must, deadlines)
		}
	}
	// 🔴 兩個數字，不是一個 —— 兩條斷言要的方向相反：
	//
	//   worstAllPeers  安全性下界用。加總**全部** peer，刻意高估：高估只會要求租約更長，
	//                  而「租約不夠長」是重複處理（正確性問題）。這一條必須 fail-closed。
	//   worstOnPath    容量上界用。只算這條路徑**實際**會打的兩支。這裡高估會造成假紅，
	//                  而假紅的下場是被放寬或刪掉 —— 那比沒有檢查更糟。
	//
	// ⚠️ worstOnPath 仍是手列的（AST 判不出「哪些 client 出現在這條呼叫鏈上」）。
	// 它的失效後果是**容量**不是正確性：新增第三個外部呼叫而沒更新它，一輪掃描會超出
	// tick 預算，訊號是 M25／告警 ⑲（invoice_worker_tick_duration_seconds）。
	// 正確性那一半已經由 worstAllPeers 接住。
	worstAllPeers := localOverhead
	for _, d := range deadlines {
		worstAllPeers += d
	}
	worstOnPath := deadlines["order"] + deadlines["payment"] + localOverhead
	t.Logf("peer client deadlines = %v；安全性下界用 %v、容量上界用 %v；租約 = %v",
		deadlines, worstAllPeers, worstOnPath, dlClaimLease)

	if dlClaimLease <= worstAllPeers {
		t.Fatalf("dlClaimLease=%v 不大於單筆最壞耗時上界 %v（＝全部 peer client 的 "+
			"DefaultDeadline 加總 ＋ %v 本地開銷）—— 租約在持有者仍在工作時到期，搶佔等於沒有"+
			"（§三 第 5 點）。\n"+
			"🔴 這條特別要緊，因為 MarkDone／MarkFailed／UpdateAttempt 的述詞只有 id + status、"+
			"**沒有 fencing token** ⇒ 殭屍持有者的寫入照樣落地。整條路徑的正確性靠的就是這個"+
			"數字關係，不是任何機制。",
			dlClaimLease, worstAllPeers, localOverhead)
	}
	// 上界：候選述詞（status IN(RECEIVED,DEFERRED,FAILED) AND next_attempt_at <= now）
	// 沒有第二個時間上界 ⇒ 候選窗口 = ∞ ⇒ §三 第 1–3 步無條件成立。真正的上界是
	// 「持有者崩潰後多久這一列回到候選集合」，取一個 tick 的量級即可。
	if dlClaimLease > 4*dlScanInterval {
		t.Errorf("dlClaimLease=%v 遠大於 tick %v —— 持有者崩潰後這一列要等太久才回到候選集合",
			dlClaimLease, dlScanInterval)
	}
	// §9.4.4 給 INV.dl 的 tick 預算是 240s；批次大小乘上單筆最壞耗時不得超過它，
	// 否則一輪掃描會吃掉其他 INV.dl 工作（K04–K06 申訴 SLA、I8 掃描）的時間。
	const tickBudget = 240 * time.Second
	if got := time.Duration(dlFetchBatch) * worstOnPath; got > tickBudget {
		t.Errorf("dlFetchBatch(%d) × 本路徑單筆最壞耗時(%v) = %v，超過 tick 預算 %v",
			dlFetchBatch, worstOnPath, got, tickBudget)
	}
}

// TestDeadlineWorker_TerminalFailureParksInsteadOfHotLooping 守 dlParkInterval。
//
// 🔴 **invoice_inbox_events 沒有真正的終態**：候選述詞含 'FAILED'。MarkFailed 若把
// Worker 傳進來的 now 直接寫回 next_attempt_at，這一列下一個 tick（5 分鐘）就會被重撈、
// 重打兩支 peer gRPC ⇒ 一條沒有退避的熱迴圈，而且完全不會報錯。
func TestDeadlineWorker_TerminalFailureParksInsteadOfHotLooping(t *testing.T) {
	repo := &fakeInboxRepo{rows: []domain.InboxEvent{dlRow(1, dlMaxAttempts-1)}, claimOK: true, markDoneOK: true}
	uc := &fakeReprocessor{err: errors.New("seller tax profile missing")}
	if err := newDLWorker(repo, uc).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.fails) != 1 {
		t.Fatalf("MarkFailed 被呼叫 %d 次，want 1（attempt_count 已達上限）", len(repo.fails))
	}
	if got := repo.fails[0].nextAttemptAt.Sub(repo.fails[0].at); got < dlParkInterval-time.Second {
		t.Fatalf("MarkFailed 寫的 next_attempt_at 只推了 %v，want ≥ %v —— 這一列會在下一個 tick "+
			"就被重撈，變成沒有退避的熱迴圈", got, dlParkInterval)
	}
	if len(repo.attempts) != 0 {
		t.Errorf("打滿次數時不得再走 UpdateAttempt（實際 %d 次）", len(repo.attempts))
	}
}

// TestDeadlineWorker_RetryableFailureKeepsStatusAndBacksOff 守 UpdateAttempt 與 MarkFailed
// 的分工：未達上限時只記次數與退避，**不動 status**。
//
// 把 status 寫進去 ⇒ 第一次失敗就變成 FAILED，ADM 異常工作台會看到一批其實還在自動
// 重試的「失敗」。
func TestDeadlineWorker_RetryableFailureKeepsStatusAndBacksOff(t *testing.T) {
	repo := &fakeInboxRepo{rows: []domain.InboxEvent{dlRow(1, 0)}, claimOK: true, markDoneOK: true}
	uc := &fakeReprocessor{err: errors.New("order-service unavailable")}
	if err := newDLWorker(repo, uc).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.attempts) != 1 {
		t.Fatalf("UpdateAttempt 被呼叫 %d 次，want 1", len(repo.attempts))
	}
	// 絕對值，不是增量。
	if repo.attempts[0].attemptCount != 1 {
		t.Errorf("attempt_count = %d，want 1（絕對值）", repo.attempts[0].attemptCount)
	}
	if got, want := repo.attempts[0].nextAttemptAt.Sub(repo.attempts[0].at), 2*dlBaseBackoff; got < want-time.Second || got > want+time.Second {
		t.Errorf("退避 = %v，want ≈ %v（2^1 × BaseBackoff）", got, want)
	}
	if len(repo.fails) != 0 || len(repo.markDoneIDs) != 0 {
		t.Errorf("未達上限時不得 MarkFailed／MarkProcessed：fail=%d done=%d", len(repo.fails), len(repo.markDoneIDs))
	}
}

// TestDeadlineWorker_MarkDoneRowsZeroIsNotAFailure 守「目標狀態已達成不是錯誤」。
//
// MarkProcessed 的 allowed-from 是 RECEIVED/DEFERRED/FAILED，rows=0 代表別的路徑
// （consumer 自己的 markInboxProcessed）先標過了。把它當錯誤 ⇒ Worker 走 bumpAttempt，
// 把一列**已經成功**的事件重排進候選集合，下一輪再跑一次兩支 peer gRPC。
func TestDeadlineWorker_MarkDoneRowsZeroIsNotAFailure(t *testing.T) {
	repo := &fakeInboxRepo{rows: []domain.InboxEvent{dlRow(1, 0)}, claimOK: true, markDoneOK: false}
	uc := &fakeReprocessor{}
	if err := newDLWorker(repo, uc).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.attempts) != 0 || len(repo.fails) != 0 {
		t.Fatalf("MarkProcessed 回 rows=0 被當成失敗：attempt=%d fail=%d", len(repo.attempts), len(repo.fails))
	}
}

// TestDeadlineWorker_ScansBothInboxTypes 是 TestDeadlineWorker_OnlyScansOrderPaid
// 的繼任者（#3073）。舊版釘的是「本波只掃 ORDER_PAID」這個**決定**，而 §7.2.2 落地之後
// 那個決定變成錯的——退款事件的 RECEIVED／FAILED 列會永遠沒有執行者。
//
// 🔴 兩種型別各走**不同的 SQL**，這正是本測試的重點：
//
//	ORDER_PAID              → FetchDueByType           （含 DEFERRED）
//	ORDER_REFUND_FINALIZED  → FetchDueActionableByType （排除 DEFERRED）
//
// 退款那一邊若誤用 FetchDueByType，亂序落 DEFERRED 的列會被撈進來，失敗時翻成 FAILED
// ⇒ ListDeferredByAggregateRef 的反向認領述詞永久不成立，該筆退款再也不會被認領，
// 而且完全無聲。
func TestDeadlineWorker_ScansBothInboxTypes(t *testing.T) {
	repo := &fakeInboxRepo{claimOK: true, markDoneOK: true}
	if err := newDLWorker(repo, &fakeReprocessor{}).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if want := []string{string(pkgmq.EventTypeOrderPaid)}; len(repo.fetchedType) != 1 || repo.fetchedType[0] != want[0] {
		t.Fatalf("走 FetchDueByType 的 event_type = %v，want %v", repo.fetchedType, want)
	}
	want := []string{string(pkgmq.EventTypeOrderRefundFinalized)}
	if len(repo.fetchedActionableType) != 1 || repo.fetchedActionableType[0] != want[0] {
		t.Fatalf("走 FetchDueActionableByType 的 event_type = %v，want %v —— "+
			"退款必須走排除 DEFERRED 的那一支", repo.fetchedActionableType, want)
	}
}

// TestDeadlineWorker_RefundScanNeverTouchesDeferred 是上一支的**鑑別力**來源。
//
// 上一支只驗「呼叫了哪一支方法」；這一支驗那個差異真的有效果——一列 DEFERRED 的退款
// 事件放在 repo 裡，掃描不得碰它。
//
// 為什麼這件事值得一支獨立的測試：翻錯一列 DEFERRED 是**永久且無聲**的
// （反向認領的述詞是 `status='DEFERRED'`，翻成 FAILED 之後它就再也不成立），
// 而症狀是「某一筆退款沒有折讓單」——沒有錯誤、沒有告警、要對帳才看得出來。
func TestDeadlineWorker_RefundScanNeverTouchesDeferred(t *testing.T) {
	repo := &fakeInboxRepo{claimOK: true, markDoneOK: true}
	repo.rows = []domain.InboxEvent{
		{ID: 1, EventType: string(pkgmq.EventTypeOrderRefundFinalized), Status: "DEFERRED"},
		{ID: 2, EventType: string(pkgmq.EventTypeOrderRefundFinalized), Status: "RECEIVED"},
	}
	rf := &recordingRefundReprocessor{}
	if err := newDLWorkerWithRefund(repo, &fakeReprocessor{}, rf).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(rf.seen) != 1 || rf.seen[0] != 2 {
		t.Fatalf("退款重跑走到的 inbox id = %v，want [2]（1 是 DEFERRED，歸反向認領）", rf.seen)
	}
}

// TestDeadlineWorker_DispatchesByEventType 守 process 的分派。
//
// 走錯的後果不是錯誤而是**靜默成功**：兩個 payload struct 的欄名沒有交集 ⇒
// unmarshal 全部拿到零值 ⇒ 一筆金額 0 的退款會安靜地跑完。
func TestDeadlineWorker_DispatchesByEventType(t *testing.T) {
	repo := &fakeInboxRepo{claimOK: true, markDoneOK: true}
	repo.rows = []domain.InboxEvent{
		{ID: 10, EventType: string(pkgmq.EventTypeOrderPaid), Status: "RECEIVED"},
		{ID: 11, EventType: string(pkgmq.EventTypeOrderRefundFinalized), Status: "RECEIVED"},
	}
	paid := &fakeReprocessor{}
	rf := &recordingRefundReprocessor{}
	if err := newDLWorkerWithRefund(repo, paid, rf).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(paid.calls) != 1 || paid.calls[0] != 10 {
		t.Fatalf("開票軸走到的 inbox id = %v，want [10]", paid.calls)
	}
	if len(rf.seen) != 1 || rf.seen[0] != 11 {
		t.Fatalf("退款軸走到的 inbox id = %v，want [11]", rf.seen)
	}
}

// TestDeadlineScanCannotPreemptARunningHandler 守 internal/mq 的 handlerLease。
//
// consumer 的 Insert 把 next_attempt_at 寫成 `now + handlerLease`。若寫成 `now`，那一列
// 在 handler 還在跑（兩支 peer gRPC）的時候就已經到期 ⇒ INV.dl 的下一輪可以合法搶走一列
// **正在被處理**的事件。
//
// 🔴 這一支住在 internal/worker 而不是 internal/mq，是因為 internal/mq **不能** import
// internal/client（client/order 與 client/payment 都 import internal/mq，反向即 cycle），
// 而這條不變式的右邊正是那兩個 DefaultDeadline。放在這裡才能拿真的常數比，不抄數字。
func TestDeadlineScanCannotPreemptARunningHandler(t *testing.T) {
	const localOverhead = time.Second
	// 與 TestDeadlineWorker_LeaseCoversWorstCaseRowTime 同一個理由用同一支 helper：
	// 手列「今天會打哪兩支 gRPC」的公式，對「日後多了第三支」是全盲的。
	deadlines := peerClientDeadlines(t)
	for _, must := range []string{"order", "payment"} {
		if _, ok := deadlines[must]; !ok {
			t.Fatalf("internal/client/%s 掃不到 DefaultDeadline（實際掃到 %v）—— 檢查瞎了", must, deadlines)
		}
	}
	worst := localOverhead
	for _, d := range deadlines {
		worst += d
	}

	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	inbox := &fakeInboxRepo{}
	// peer client 一律回 ErrPeerClientUnavailable ⇒ handler 在步驟 5 就失敗、那一列留在
	// RECEIVED，正是「兜底掃描要接手」的形狀。issuance／applier／docs 走不到，傳 nil。
	c := invoicemq.NewOrderPaidConsumer(inbox, nil,
		invoicemq.UnavailableOrderFactsClient{}, invoicemq.UnavailablePaymentFactsClient{},
		nil, nil, nil).WithClock(func() time.Time { return now })

	body, err := json.Marshal(map[string]any{
		"event_id": "e-lease", "order_no": "O000000009", "occurred_at": "2026-08-30T11:59:00Z",
		"buyer_id": 2623900520, "store_id": 2685914682,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := c.Handle(context.Background(), "order.paid", body); err == nil {
		t.Fatal("peer client 不可用時必須 fail-loud")
	}
	if len(inbox.rows) != 1 {
		t.Fatalf("inbox 列數 = %d，want 1", len(inbox.rows))
	}
	reserved := inbox.rows[0].NextAttemptAt.Sub(now)
	if reserved <= worst {
		t.Fatalf("handler 落庫時只預留 %v 的租約，不大於單筆最壞耗時 %v —— "+
			"INV.dl 可以搶走一列正在被處理的事件", reserved, worst)
	}
}

// peerClientDeadlines 掃出 internal/client/*/ 底下**每一支** peer client 的 per-call
// deadline，並在掃不出來時 t.Fatalf。
//
// 為什麼是掃原始碼而不是 import 那幾個常數：import 的那份清單是**手寫**的，
// 而本檢查要守的正是「清單有沒有漏掉新加的那一支」。手寫清單守不了自己。
//
// 🔴 **fail-closed（2026-08-31）**：舊版只認 `const DefaultDeadline = <n> * time.<Unit>`
// 這一種形狀，認不出來就**靜默略過那個目錄**。而 internal/client/amego 從第一天起就是
// 另一種形狀（`func DefaultDeadlines() Deadlines { ... }`，Issue 30s／Query 10s／
// List 20s／Time 5s）⇒ 它從來沒有被加總過，而呼叫端的鑑別力自檢只點名 order 與 payment
// ⇒ 沒有任何東西會轉紅。「換寫法會讓那一支從加總中消失」在舊版註解裡是**已知風險**，
// 而它當時就已經發生了。
//
// 現在認兩種形狀，第三種一律報錯而不是略過：
//
//	const / var DefaultDeadline  = 3 * time.Second           → 3s
//	func DefaultDeadlines() Deadlines { return Deadlines{    → 取**最大值** 30s
//	    Issue: 30 * time.Second, Query: 10 * time.Second, …}}
//
// 取最大值而不是加總：這個函式回的語意是「這支 client **一次呼叫**的時間預算」，
// 與單一 DefaultDeadline 對齊。跨 client 的保守方向由呼叫端負責（它加總全部 peer）。
//
// 新增一支 peer client 而沒有可辨識的 deadline 宣告時，這裡會直接 fail 並告訴對方
// 要補什麼 —— 那是刻意的：一支沒有 per-call deadline 的 gRPC client 讓「單筆最壞耗時」
// 失去上界，而整條 INV.dl 路徑的正確性靠的就是「租約 > 單筆最壞耗時」這個數字關係。
func peerClientDeadlines(t *testing.T) map[string]time.Duration {
	t.Helper()
	out := map[string]time.Duration{}
	root := "../client"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, e.Name(), "*.go"))
		if err != nil {
			t.Fatalf("glob %s: %v", e.Name(), err)
		}
		best, found := time.Duration(0), false
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", f, err)
			}
			for _, d := range deadlinesDeclaredIn(t, f, parsed) {
				found = true
				if d > best {
					best = d
				}
			}
		}
		if !found {
			t.Fatalf("internal/client/%s 掃不出任何 per-call deadline。\n"+
				"🔴 這是 fail-closed，不是誤報：本檢查用「全部 peer client 的 deadline 加總」"+
				"當單筆最壞耗時的上界，而漏掉一支會讓那個上界偏小 ⇒ 租約看起來夠長、"+
				"實際上持有者還在打 gRPC 時就到期，另一個副本合法搶走同一筆。\n"+
				"認得的兩種宣告形狀：\n"+
				"  const/var DefaultDeadline = <n> * time.<Unit>\n"+
				"  func DefaultDeadlines() <T> { return <T>{ F: <n> * time.<Unit>, ... } }\n"+
				"換第三種寫法就要同時教會 deadlinesDeclaredIn 認它 —— "+
				"這一段刻意不接受「靜默略過」。", e.Name())
		}
		out[e.Name()] = best
	}
	if len(out) == 0 {
		t.Fatalf("%s 底下一個 peer client 目錄都沒掃到 —— 檢查本身失效了", root)
	}
	return out
}

// deadlinesDeclaredIn 回一個檔案裡所有可辨識的 per-call deadline。
//
// 🔴 「認得出名字但解不開值」一律 t.Fatalf，不回空：那正是舊版靜默略過 amego 的形狀。
func deadlinesDeclaredIn(t *testing.T, path string, f *ast.File) []time.Duration {
	t.Helper()
	var out []time.Duration
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.GenDecl:
			// 形狀一：const / var DefaultDeadline = <n> * time.<Unit>
			for _, spec := range decl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "DefaultDeadline" {
					continue
				}
				if len(vs.Values) != 1 {
					t.Fatalf("%s 的 DefaultDeadline 沒有單一初值運算式 —— 解不開就不得略過", path)
				}
				d, ok := durationLiteral(vs.Values[0])
				if !ok {
					t.Fatalf("%s 的 DefaultDeadline 不是 `<n> * time.<Unit>`（實際 %T）。\n"+
						"認不出來就是漏掉一支 peer 的 deadline，而漏掉的方向是「上界偏小」"+
						"⇒ 租約檢查會假綠。", path, vs.Values[0])
				}
				out = append(out, d)
			}
		case *ast.FuncDecl:
			// 形狀二：func DefaultDeadlines() T { return T{F: <n> * time.<Unit>, ...} }
			if decl.Recv != nil || decl.Name.Name != "DefaultDeadlines" || decl.Body == nil {
				continue
			}
			out = append(out, maxReturnedDeadline(t, path, decl))
		}
	}
	return out
}

// maxReturnedDeadline 解 `func DefaultDeadlines() T { return T{F: <n> * time.<Unit>, …} }`
// 並回其中最大的一個。
//
// 🔴 **每一個欄位都必須解得開**，解不開就 t.Fatalf。只取「解得開的那些」的最大值是一個
// 很隱蔽的洞：把最大的那一欄（amego 的 Issue 30s）改成 `time.Duration(30) * time.Second`
// 之後，其餘三欄照樣解得開 ⇒ 這支回 20s ⇒ 上界少算 10s ⇒ 租約檢查靜默假綠。
// 那正是本輪要修的那個形狀（少算 amego 30s），只是換了一層。
func maxReturnedDeadline(t *testing.T, path string, fn *ast.FuncDecl) time.Duration {
	t.Helper()
	best, found := time.Duration(0), false
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok {
			continue
		}
		for _, res := range ret.Results {
			cl, ok := res.(*ast.CompositeLit)
			if !ok {
				t.Fatalf("%s 的 DefaultDeadlines 回的不是複合字面量（實際 %T）—— "+
					"認不出來就不得當成 0，那會讓單筆最壞耗時的上界少算一整支 peer。", path, res)
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					t.Fatalf("%s 的 DefaultDeadlines 用了位置參數複合字面量 —— "+
						"欄位對應關係無從驗證。", path)
				}
				d, ok := durationLiteral(kv.Value)
				if !ok {
					t.Fatalf("%s 的 DefaultDeadlines 欄位 %s 不是 `<n> * time.<Unit>`。\n"+
						"🔴 這裡刻意 fail 而不是「跳過這一欄取其餘的最大值」："+
						"跳過最大的那一欄會讓上界少算，而少算的方向是租約檢查假綠。\n"+
						"換寫法就要同時教會 durationLiteral 認它。", path, exprName(kv.Key))
				}
				found = true
				if d > best {
					best = d
				}
			}
		}
	}
	if !found {
		t.Fatalf("%s 的 DefaultDeadlines 解不出任何欄位 —— 它是一支 peer client 對外的"+
			"時間預算，解不開就不得當成 0。", path)
	}
	return best
}

func exprName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return "<expr>"
}

// durationLiteral 解 `<int literal> * time.<Unit>`。
func durationLiteral(e ast.Expr) (time.Duration, bool) {
	units := map[string]time.Duration{
		"Nanosecond": time.Nanosecond, "Microsecond": time.Microsecond,
		"Millisecond": time.Millisecond, "Second": time.Second,
		"Minute": time.Minute, "Hour": time.Hour,
	}
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.MUL {
		return 0, false
	}
	lit, ok := bin.X.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	sel, ok := bin.Y.(*ast.SelectorExpr)
	if !ok {
		return 0, false
	}
	unit, ok := units[sel.Sel.Name]
	if !ok {
		return 0, false
	}
	n64, err := strconv.ParseInt(lit.Value, 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(n64) * unit, true
}
