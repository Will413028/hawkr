//go:build integration

// INV.dl 的**並發與時序**攻擊面：真 PostgreSQL、真 pkg/outbox.Worker、真 OrderPaidConsumer、
// 真 usecase.Issuance。本檔的預設立場是「這個修復在多副本下會壞」，每一支都在試著弄壞它。
//
// 為什麼不能用 fake 驗：deadline_worker_test.go 那組 fake 的 Claim 直接回一個布林
// （`claimOK`），**搶佔的正確性完全沒有被測到**——真正在守的是 Postgres 對單句 UPDATE
// 的列鎖，那是 fake 依定義模擬不了的東西。同理「租約到期但持有者還在跑」需要兩個
// 真的併發持有者對同一列寫入，fake 的 slice append 不會產生任何競爭。
package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	invoicemq "github.com/Will413028/hawkr/invoice-service/internal/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
)

// raceCfg 是把 deadlineCfg() 的租約換成 lease、其餘照抄生產值。
//
// 只有 ClaimLease 可調：其餘（MaxAttempts／backoff／FetchBatch）都是被攻擊的對象，
// 在測試裡另給一組會讓這裡驗到的是一個不存在的設定。
func raceCfg(lease time.Duration) outbox.Config {
	c := deadlineCfg()
	c.ClaimLease = lease
	return c
}

// countingReprocessor 記錄「副作用真的跑了幾次、對哪幾列」，並可在中途卡住。
//
// gate 是 nil 時不阻塞；非 nil 時每一次呼叫都等它被關閉——用來把一個持有者釘在
// 「已 claim、副作用進行中」的狀態，這正是租約要保護的窗口。
type countingReprocessor struct {
	mu      sync.Mutex
	calls   []int64
	entered chan struct{}
	gate    chan struct{}
	err     error
}

func (r *countingReprocessor) ReprocessFromInbox(_ context.Context, ev domain.InboxEvent) error {
	r.mu.Lock()
	r.calls = append(r.calls, ev.ID)
	n := len(r.calls)
	r.mu.Unlock()
	if r.entered != nil && n == 1 {
		close(r.entered)
	}
	if r.gate != nil {
		<-r.gate
	}
	return r.err
}

func (r *countingReprocessor) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// barrierInboxRepo 把「兩個 pod 都撈完了、都還沒 claim」這個狀態變成**確定的**。
//
// 🔴 沒有它的話，兩個 goroutine 各跑一次 RunOnce 幾乎測不到東西：先起跑的那個通常在
// 另一個發出 FetchDueByType 之前就已經 claim ＋ 副作用 ＋ MarkDone 跑完，於是第二個
// 撈到空集合，測試綠燈但**搶佔一行都沒被執行到**。實測（2026-08-31）把 ClaimInboxEvent
// 的租約述詞整個拿掉之後，沒有 barrier 的版本照樣全綠——那正是「綠燈是假的」。
//
// 生產上這個狀態是常態而非巧合：兩個 pod 的 ticker 相位獨立，5 分鐘一次的掃描很容易
// 在同一秒內各撈一次同一批候選。
type barrierInboxRepo struct {
	domain.InboxRepository
	wg *sync.WaitGroup
}

func (r barrierInboxRepo) FetchDueByType(ctx context.Context, eventType string, now time.Time, limit int32) ([]domain.InboxEvent, error) {
	rows, err := r.InboxRepository.FetchDueByType(ctx, eventType, now, limit)
	// 先撈完再對齊：對齊點必須在 claim **之前**、fetch **之後**。
	r.wg.Done()
	r.wg.Wait()
	return rows, err
}

// seedDueInbox 造一列「已到期、等著被兜底掃描撿走」的 ORDER_PAID 事件，payload 就是
// consumer 真的會落庫的那一份，好讓 ReprocessFromInbox 走得完整條路。
func seedDueInbox(t *testing.T, env *e2eEnv, eventID string, nextAttemptAt time.Time) int64 {
	t.Helper()
	inbox := postgres.NewInboxRepository(env.pool)
	id, inserted, err := inbox.Insert(context.Background(), domain.NewInboxEvent{
		EventID: eventID,
		// 🔴 event_type 的值必須來自 pkgmq 常數，不是 routing key 的字面值：
		// 兩者長得像但不相等（"ORDER_PAID" vs "order.paid"），寫錯的話
		// FetchDueByType 永遠撈不到，於是每一支攻擊都會「通過」而什麼都沒測到。
		EventType:        string(pkgmq.EventTypeOrderPaid),
		RoutingKey:       "order.paid",
		Exchange:         "hawkr.order",
		BusinessDedupKey: "ORDER_PAID:" + env.orderNo,
		AggregateRef:     &env.orderNo,
		Payload:          e2ePaidBody(t, eventID, env),
		Status:           "RECEIVED",
		NextAttemptAt:    nextAttemptAt,
		OccurredAt:       time.Now().UTC(),
		Now:              time.Now().UTC(),
	})
	if err != nil || !inserted {
		t.Fatalf("seed inbox: id=%d inserted=%v err=%v", id, inserted, err)
	}
	return id
}

func containsDLRow(evs []domain.InboxEvent, id int64) bool {
	for _, e := range evs {
		if e.ID == id {
			return true
		}
	}
	return false
}

func inboxRowState(t *testing.T, env *e2eEnv, id int64) (status string, attempt int32, nextAt time.Time) {
	t.Helper()
	if err := env.pool.QueryRow(context.Background(),
		`SELECT status, attempt_count, next_attempt_at FROM invoice_inbox_events WHERE id=$1`,
		id).Scan(&status, &attempt, &nextAt); err != nil {
		t.Fatalf("read inbox row %d: %v", id, err)
	}
	return
}

// ---------------------------------------------------------------------------
// 攻擊 1：CAS 到底原不原子
// ---------------------------------------------------------------------------

// TestIntegration_DLRace_ClaimIsAtomicUnderRealConcurrency 攻擊「兩個 pod 同時掃到同一列」。
//
// 既有的 TestIntegration_Inbox_ClaimIsALeaseNotAPermanentFlag 是**循序**打兩次 claim：
// 第二次一定看得到第一次的結果，所以它驗的是述詞寫對了，不是併發安全。真正的問題是
// 16 個連線同時送出同一句 UPDATE 時會不會有兩個都回 rows=1（若成立，兩個副本會各自跑
// 一次 §7.1.2 步驟 5–9，也就是對同一張訂單開兩次票）。
func TestIntegration_DLRace_ClaimIsAtomicUnderRealConcurrency(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	now := time.Now().UTC()
	id := seedDueInbox(t, env, "evt-atomic", now.Add(-time.Minute))

	const racers = 16
	var wins atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := repo.Claim(ctx, id, now.Add(dlClaimLease), now)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if ok {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := wins.Load(); got != 1 {
		t.Fatalf("%d 個併發 claim 有 %d 個成功，want 1 —— CAS 不原子，"+
			"同一列會被多個副本同時處理", racers, got)
	}
}

// ---------------------------------------------------------------------------
// 攻擊 2：兩個 pod 各跑一次 RunOnce，副作用是真的 Prepare
// ---------------------------------------------------------------------------

// TestIntegration_DLRace_TwoPodsProduceOneInvoice 攻擊整支 DeadlineWorker，不只 SQL。
//
// 兩個 DeadlineWorker ＝ 兩個 pod，共用同一個 DB，同時 RunOnce。斷言三件事：
// 副作用只跑一次、只產生一張 invoice_documents、那一列被推進 PROCESSED。
//
// 🔴 第二項不能省：就算兩個副本都跑了副作用，uq_invdoc_order_identity 也會讓第二張
// document 插不進去（ON CONFLICT DO NOTHING）⇒ 只驗文件數會把「搶佔失效」誤判成通過。
// 第一項（真的只跑一次）才是搶佔本身的證據。
func TestIntegration_DLRace_TwoPodsProduceOneInvoice(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	id := seedDueInbox(t, env, "evt-twopods", time.Now().UTC().Add(-time.Minute))

	// 兩個 pod 共用同一個 reprocessor 才數得出「總共跑了幾次」；業務面它們是同一支
	// OrderPaidConsumer.ReprocessFromInbox 的兩個副本，行為相同。
	rp := &countingReprocessor{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	braked := barrierInboxRepo{InboxRepository: repo, wg: &barrier}
	// K05 的 sweeper 傳 nil：本檔驗的是兩個 pod 搶同一列 inbox 的租約行為，
	// 與申訴逾期無關（RunOnce 對未 wire 的 sweeper 安靜跳過）。
	podA := NewDeadlineWorker(braked, rp, strictRefundReprocessor{}, deadlineCfg(), dlParkInterval, nil, testObsC, nil)
	podB := NewDeadlineWorker(braked, rp, strictRefundReprocessor{}, deadlineCfg(), dlParkInterval, nil, testObsC, nil)

	var wg sync.WaitGroup
	for _, w := range []*DeadlineWorker{podA, podB} {
		wg.Add(1)
		go func(w *DeadlineWorker) {
			defer wg.Done()
			if err := w.RunOnce(ctx); err != nil {
				t.Errorf("RunOnce: %v", err)
			}
		}(w)
	}
	wg.Wait()

	if got := rp.count(); got != 1 {
		t.Fatalf("兩個 pod 同時掃描，副作用跑了 %d 次，want 1 —— 同一張訂單被開了 %d 次票", got, got)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_documents WHERE source_ref=$1`, env.orderNo); n != 0 {
		t.Fatalf("countingReprocessor 不該產生 document，卻有 %d 列（測試自身失準）", n)
	}
	if status, _, _ := inboxRowState(t, env, id); status != "PROCESSED" {
		t.Fatalf("成功處理後 inbox 狀態 = %s，want PROCESSED", status)
	}
}

// TestIntegration_DLRace_TwoPodsRealIssuanceOneDocument 同上，但副作用換成**真的**
// OrderPaidConsumer.ReprocessFromInbox（真 Prepare、真交易、真唯一索引）。
//
// 這一支回答的是「就算搶佔哪天壞掉，資料面兜不兜得住」——它與上一支是兩個獨立的問題，
// 上一支綠不代表這一支綠（反之亦然）。
func TestIntegration_DLRace_TwoPodsRealIssuanceOneDocument(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	seedDueInbox(t, env, "evt-realissue", time.Now().UTC().Add(-time.Minute))

	var barrier sync.WaitGroup
	barrier.Add(2)
	braked := barrierInboxRepo{InboxRepository: repo, wg: &barrier}
	podA := NewDeadlineWorker(braked, env.paid, strictRefundReprocessor{}, deadlineCfg(), dlParkInterval, nil, testObsC, nil)
	podB := NewDeadlineWorker(braked, env.paid, strictRefundReprocessor{}, deadlineCfg(), dlParkInterval, nil, testObsC, nil)

	var wg sync.WaitGroup
	for _, w := range []*DeadlineWorker{podA, podB} {
		wg.Add(1)
		go func(w *DeadlineWorker) {
			defer wg.Done()
			if err := w.RunOnce(ctx); err != nil {
				t.Errorf("RunOnce: %v", err)
			}
		}(w)
	}
	wg.Wait()

	if n := countRows(t, env, `SELECT count(*) FROM invoice_documents WHERE source_ref=$1`, env.orderNo); n != 1 {
		t.Fatalf("invoice_documents = %d 列，want 1 —— 一張訂單開出多張發票", n)
	}
	if n := countRows(t, env,
		`SELECT count(*) FROM invoice_operations o JOIN invoice_documents d ON d.id=o.document_id
		  WHERE d.source_ref=$1`, env.orderNo); n != 1 {
		t.Fatalf("invoice_operations = %d 列，want 1 —— INV.op 會送出多次 AMEGO 開票", n)
	}
	if got := env.orderFax.calls.Load(); got != 1 {
		t.Fatalf("ORD GetOrderInvoiceSnapshot 被呼叫 %d 次，want 1", got)
	}
}

// ---------------------------------------------------------------------------
// 攻擊 3：租約到期，但前一個持有者還在跑
// ---------------------------------------------------------------------------

// TestIntegration_DLRace_ExpiredLeaseLetsASecondPodInWhileTheFirstStillRuns 是本檔最重要的一支。
//
// 形狀：pod A claim 之後**卡在副作用裡**，租約到期，pod B 合法搶走同一列並跑第二次副作用。
// 這不是 bug，是 at-least-once 的定義；真正的問題在於**保證由什麼提供**：
//
//	保證 ＝ 「租約長度 > 單筆最壞耗時」這個數字關係，而不是任何機制。
//
// pkg/outbox.Worker 的 MarkDone／MarkFailed／UpdateAttempt **都沒有 fencing token**
// （SQL 述詞只有 id ＋ status，沒有租約項），所以一旦那個數字關係不成立，殭屍持有者
// 的寫入照樣落地。本支把租約縮到 80ms 讓那個關係不成立，量出後果。
//
// 🔴 它同時是 TestDeadlineWorker_LeaseCoversWorstCaseRowTime 的**鑑別力證明**：
// 那支測的是常數之間的算術，這支證明算術一旦不成立，真的會有兩次副作用。
func TestIntegration_DLRace_ExpiredLeaseLetsASecondPodInWhileTheFirstStillRuns(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	id := seedDueInbox(t, env, "evt-expired", time.Now().UTC().Add(-time.Minute))

	const lease = 80 * time.Millisecond
	rpA := &countingReprocessor{entered: make(chan struct{}), gate: make(chan struct{})}
	rpB := &countingReprocessor{}
	podA := NewDeadlineWorker(repo, rpA, strictRefundReprocessor{}, raceCfg(lease), dlParkInterval, nil, testObsC, nil)
	podB := NewDeadlineWorker(repo, rpB, strictRefundReprocessor{}, raceCfg(dlClaimLease), dlParkInterval, nil, testObsC, nil)

	doneA := make(chan struct{})
	go func() {
		defer close(doneA)
		if err := podA.RunOnce(ctx); err != nil {
			t.Errorf("pod A RunOnce: %v", err)
		}
	}()

	select {
	case <-rpA.entered: // A 已 claim 且正在副作用中
	case <-time.After(30 * time.Second):
		// 沒有這個 case 的話，種子沒被撈到就會靜靜地卡到 go test 的 10 分鐘 panic，
		// 而 panic 的訊息完全不會指向真正的原因（event_type 寫錯）。
		t.Fatal("pod A 從未進入副作用 —— 種子沒有進入 INV.dl 的候選集合，本支的前提不成立")
	}
	time.Sleep(lease + 40*time.Millisecond)

	if err := podB.RunOnce(ctx); err != nil {
		t.Fatalf("pod B RunOnce: %v", err)
	}

	gotB := rpB.count()
	close(rpA.gate)
	<-doneA

	if gotB != 1 {
		t.Fatalf("租約（%v）已到期而 pod B 沒有搶到（副作用 %d 次）—— "+
			"本支的前提沒有成立，後面的斷言沒有意義", lease, gotB)
	}

	// 這才是本支要留下的證據：租約到期後，同一列被跑了兩次副作用，
	// 而 A 直到最後都不知道自己已經不是持有者。
	total := rpA.count() + gotB
	if total != 2 {
		t.Fatalf("副作用總次數 = %d，want 2（A 一次、B 一次）", total)
	}
	status, _, _ := inboxRowState(t, env, id)
	t.Logf("租約 %v < 單筆耗時 ⇒ 同一列跑了 %d 次副作用；殭屍持有者 A 的 MarkDone "+
		"仍然落地（列狀態=%s，rows=0 被當成成功）", lease, total, status)
}

// TestIntegration_DLRace_ZombieHolderCanResetALiveHoldersLease 把上一支的機制講死。
//
// 純 SQL、無 goroutine、完全確定性：證明 UpdateAttempt 沒有租約 fence，
// 於是一個租約已到期的殭屍持有者可以把**現任持有者**的租約改掉，讓那一列在現任
// 還在工作時重新回到候選集合（＝第三個副本可以再搶一次）。
//
// 🔴 這不是「租約沒用」——租約在數字關係成立時完全有效（見上一支的前提檢查）。
// 這是「租約失效之後沒有第二道防線」：規範 §三 的下界不是建議，是這裡唯一的保證。
func TestIntegration_DLRace_ZombieHolderCanResetALiveHoldersLease(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	now := time.Now().UTC()
	id := seedDueInbox(t, env, "evt-zombie", now.Add(-time.Minute))

	// A 取得一個「即將到期」的租約（模擬 A 很慢、租約已經走完）。
	if ok, err := repo.Claim(ctx, id, now.Add(-time.Second), now); err != nil || !ok {
		t.Fatalf("A claim: ok=%v err=%v", ok, err)
	}
	// B 合法搶走（A 的租約已過期）。
	if ok, err := repo.Claim(ctx, id, now.Add(dlClaimLease), now); err != nil || !ok {
		t.Fatalf("B claim 應成功（A 租約已到期）：ok=%v err=%v", ok, err)
	}
	if _, _, nextAt := inboxRowState(t, env, id); !nextAt.After(now.Add(dlClaimLease - time.Minute)) {
		t.Fatalf("B 的租約沒有寫進去（next_attempt_at=%v）", nextAt)
	}

	// 殭屍 A 這時才失敗返回，寫它自己的重試簿記。它的租約早就沒了。
	ok, err := repo.UpdateAttempt(ctx, id, 1, now.Add(2*time.Second), nil)
	if err != nil {
		t.Fatalf("zombie UpdateAttempt: %v", err)
	}
	if !ok {
		t.Skip("UpdateAttempt 已帶租約 fence（rows=0）—— 本攻擊不成立，這是好事")
	}

	// 後果：B 仍在工作，但那一列已經因為 A 的寫入回到候選集合。
	due, err := repo.FetchDueByType(ctx, string(pkgmq.EventTypeOrderPaid), now.Add(3*time.Second), 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	found := false
	for _, e := range due {
		if e.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("殭屍寫入之後該列沒有回到候選集合 —— 攻擊不成立")
	}
	t.Logf("殭屍持有者（租約已到期）的 UpdateAttempt 覆蓋了現任持有者 B 的租約：" +
		"該列在 B 仍在工作時回到候選集合，第三個副本可再搶一次。" +
		"⇒ MarkDone/MarkFailed/UpdateAttempt 都沒有 fencing token，" +
		"唯一的保證是「租約 > 單筆最壞耗時」這個數字關係。")
}

// TestIntegration_DLRace_ZombieCannotResurrectACompletedRow 是上一支的反面，也是
// 「這個設計到底守住了什麼」的答案。
//
// 殭屍持有者的寫入雖然沒有租約 fence，但三支 Mark* 都帶 `status IN (RECEIVED,DEFERRED,FAILED)`。
// 於是最嚴重的那個後果——把一列**已經開好票**的事件打回候選集合、讓它被重開一次——
// 是被擋住的。這一項若失守，後果比上一支嚴重一個量級。
func TestIntegration_DLRace_ZombieCannotResurrectACompletedRow(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	now := time.Now().UTC()
	id := seedDueInbox(t, env, "evt-resurrect", now.Add(-time.Minute))

	if ok, err := repo.Claim(ctx, id, now.Add(-time.Second), now); err != nil || !ok {
		t.Fatalf("A claim: ok=%v err=%v", ok, err)
	}
	// B 搶走並成功完成。
	if ok, err := repo.Claim(ctx, id, now.Add(dlClaimLease), now); err != nil || !ok {
		t.Fatalf("B claim: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.MarkProcessed(ctx, id, now); err != nil || !ok {
		t.Fatalf("B MarkProcessed: ok=%v err=%v", ok, err)
	}

	// 殭屍 A 現在做它會做的三件事，全部必須是 no-op。
	for _, tc := range []struct {
		name string
		run  func() (bool, error)
	}{
		{"UpdateAttempt", func() (bool, error) { return repo.UpdateAttempt(ctx, id, 1, now.Add(time.Second), nil) }},
		{"MarkFailed", func() (bool, error) { return repo.MarkFailed(ctx, id, "zombie", now.Add(time.Second)) }},
		{"MarkDeferred", func() (bool, error) { return repo.MarkDeferred(ctx, id, now.Add(time.Second)) }},
	} {
		ok, err := tc.run()
		if err != nil {
			t.Fatalf("zombie %s: %v", tc.name, err)
		}
		if ok {
			t.Fatalf("殭屍持有者的 %s 打在一列已 PROCESSED 的事件上並生效 —— "+
				"一張已經開好的發票會被重開", tc.name)
		}
	}
	if status, _, _ := inboxRowState(t, env, id); status != "PROCESSED" {
		t.Fatalf("狀態被殭屍改成 %s，want PROCESSED", status)
	}
}

// ---------------------------------------------------------------------------
// 攻擊 4：崩潰在副作用中途
// ---------------------------------------------------------------------------

// TestIntegration_DLRace_CrashMidSideEffectIsRecoveredAndIdempotent 攻擊「持有者在
// 副作用做到一半就消失」。
//
// 模擬崩潰＝claim 之後**什麼都不做**（沒有 MarkDone、沒有 UpdateAttempt——進程沒了，
// 不會有任何 defer 跑）。此時 document 已經寫進去、inbox 列還停在 RECEIVED。
//
// 兩件事要同時成立才算過：
//
//	① 租約到期後那一列自己回到候選集合（否則就是永久旗標，那張訂單的發票永遠開不出來）
//	② 重跑一次**不會**開出第二張發票（Prepare 的冪等由 uq_invdoc_order_identity 兜著）
//
// 🔴 租約到期一律用**真的時間**（短租約 ＋ Sleep），不得用「傳一個未來的 now 給
// FetchDueByType」來模擬：RunOnce 內部用的是 time.Now()，兩者不同步 ⇒ 假的到期
// 會讓 pod B 撈不到那一列，而測試會把「租約還沒到期」誤讀成「接手失敗」。
// （本支第一版就是這樣寫的，紅在最後一行斷言上，而真正的原因在二十行以前。）
func TestIntegration_DLRace_CrashMidSideEffectIsRecoveredAndIdempotent(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	now := time.Now().UTC()
	id := seedDueInbox(t, env, "evt-crash", now.Add(-time.Minute))

	// pod A：claim（短租約，好讓它在測試時間內真的到期）→ 副作用成功 → 崩潰
	// （MarkDone 從未執行——進程沒了，不會有任何 defer 跑）。
	const crashLease = 150 * time.Millisecond
	if ok, err := repo.Claim(ctx, id, now.Add(crashLease), now); err != nil || !ok {
		t.Fatalf("A claim: ok=%v err=%v", ok, err)
	}
	ev, err := repo.FetchDueByType(ctx, string(pkgmq.EventTypeOrderPaid), now, 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	if containsDLRow(ev, id) {
		t.Fatal("claim 之後該列仍在候選集合裡 —— 這不是搶佔")
	}

	// A 的副作用跑完了（document 已落庫），然後進程消失。
	if err := env.paid.ReprocessFromInbox(ctx, domain.InboxEvent{
		ID: id, EventType: string(pkgmq.EventTypeOrderPaid),
		AggregateRef: &env.orderNo, Payload: e2ePaidBody(t, "evt-crash", env),
		Status: "RECEIVED", OccurredAt: now,
	}); err != nil {
		t.Fatalf("A 的副作用: %v", err)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_documents WHERE source_ref=$1`, env.orderNo); n != 1 {
		t.Fatalf("第一次副作用後 document = %d 列，want 1", n)
	}
	if status, _, _ := inboxRowState(t, env, id); status != "RECEIVED" {
		t.Fatalf("模擬崩潰後狀態 = %s，want RECEIVED（沒有人標記過它）", status)
	}

	// ① 租約真的到期之後，那一列自己回到候選集合。
	time.Sleep(crashLease + 100*time.Millisecond)
	back, err := repo.FetchDueByType(ctx, string(pkgmq.EventTypeOrderPaid), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	if !containsDLRow(back, id) {
		t.Fatal("租約到期後該列沒有回到候選集合 —— claim 是永久旗標，" +
			"持有者崩潰即該訂單永遠開不出發票")
	}

	// ② pod B 接手，跑完整條 RunOnce（真時間、生產參數）。
	podB := NewDeadlineWorker(repo, env.paid, strictRefundReprocessor{}, deadlineCfg(), dlParkInterval, nil, testObsC, nil)
	if err := podB.RunOnce(ctx); err != nil {
		t.Fatalf("pod B RunOnce: %v", err)
	}
	if n := countRows(t, env, `SELECT count(*) FROM invoice_documents WHERE source_ref=$1`, env.orderNo); n != 1 {
		t.Fatalf("崩潰後重跑產生了 %d 張 document，want 1 —— 一張訂單開出兩張發票", n)
	}
	if n := countRows(t, env,
		`SELECT count(*) FROM invoice_operations o JOIN invoice_documents d ON d.id=o.document_id
		  WHERE d.source_ref=$1`, env.orderNo); n != 1 {
		t.Fatalf("崩潰後重跑產生了 %d 個 operation，want 1 —— AMEGO 會被送兩次開票", n)
	}
	if status, _, _ := inboxRowState(t, env, id); status != "PROCESSED" {
		t.Fatalf("接手成功後狀態 = %s，want PROCESSED", status)
	}
}

// ---------------------------------------------------------------------------
// 攻擊 5：consumer 的 handlerLease vs worker 的 ClaimLease
// ---------------------------------------------------------------------------

// TestIntegration_DLRace_RunningHandlerIsNotPreemptedByARealScan 用**真的**
// DeadlineWorker 打真的 DB，驗 handlerLease 擋得住兜底掃描。
//
// 既有的 TestDeadlineScanCannotPreemptARunningHandler 比的是兩個 Go 常數的算術
// （60s > 7s），它證明不了「掃描真的撈不到這一列」——那要 SQL 的候選述詞、
// consumer 寫進 next_attempt_at 的那個值、與 worker 的 fetch 三者對得上才成立。
func TestIntegration_DLRace_RunningHandlerIsNotPreemptedByARealScan(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	// consumer 的 peer client 不可用 ⇒ handler 在步驟 5 失敗、那一列留在 RECEIVED，
	// 正是「兜底掃描要接手、但現在還不可以」的形狀。
	failing := invoicemq.NewOrderPaidConsumer(repo, env.uc,
		invoicemq.UnavailableOrderFactsClient{}, invoicemq.UnavailablePaymentFactsClient{},
		invoicemq.UnimplementedAdjustmentApplier{}, nil, nil)
	if err := failing.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-handlerlease", env)); err == nil {
		t.Fatal("peer client 不可用時 handler 必須 fail-loud")
	}

	var id int64
	var nextAt time.Time
	if err := env.pool.QueryRow(ctx,
		`SELECT id, next_attempt_at FROM invoice_inbox_events WHERE business_dedup_key=$1`,
		"ORDER_PAID:"+env.orderNo).Scan(&id, &nextAt); err != nil {
		t.Fatalf("read inbox row: %v", err)
	}

	// 立刻跑一輪兜底掃描：它必須撈不到這一列。
	rp := &countingReprocessor{}
	dl := NewDeadlineWorker(repo, rp, strictRefundReprocessor{}, deadlineCfg(), dlParkInterval, nil, testObsC, nil)
	if err := dl.RunOnce(ctx); err != nil {
		t.Fatalf("INV.dl RunOnce: %v", err)
	}
	if got := rp.count(); got != 0 {
		t.Fatalf("handler 剛落庫的那一列就被兜底掃描搶走（副作用 %d 次）—— "+
			"handlerLease 沒有生效，兩條路徑會同時跑 §7.1.2 步驟 5–9", got)
	}
	if reserved := time.Until(nextAt); reserved <= 0 {
		t.Fatalf("落庫時預留的租約已經是過去（next_attempt_at=%v）", nextAt)
	}

	// 租約到期之後，同一支掃描必須撈得到——否則就變成永遠沒人接手。
	due, err := repo.FetchDueByType(ctx, string(pkgmq.EventTypeOrderPaid), nextAt.Add(time.Second), 10)
	if err != nil {
		t.Fatalf("FetchDueByType: %v", err)
	}
	found := false
	for _, e := range due {
		if e.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("handlerLease 到期後該列仍不在候選集合 —— 失敗的 order.paid 永遠沒有人接手")
	}
}

// TestIntegration_DLRace_ConsumerCanStealTheRowFromAClaimedWorker 記錄一個**真實的
// 不對稱**：worker 會 claim，consumer 不會看 claim。
//
// consumer 的 markInboxProcessed 走 MarkInboxEventProcessed，述詞只有 id ＋ status，
// 沒有租約項。所以一個跑超過 handlerLease（60s）的 handler，會在 worker 已經合法
// claim 之後把同一列標成 PROCESSED，而 worker 這時正在跑第二份副作用。
//
// 這與 TestIntegration_DLRace_ZombieHolderCanResetALiveHoldersLease 是同一個根：
// 兩條路徑對「誰持有這一列」沒有共同的判準，只有一個數字關係（handler 耗時 < 60s）。
func TestIntegration_DLRace_ConsumerCanStealTheRowFromAClaimedWorker(t *testing.T) {
	ctx := context.Background()
	env := setupE2E(t)
	repo := postgres.NewInboxRepository(env.pool)

	now := time.Now().UTC()
	id := seedDueInbox(t, env, "evt-steal", now.Add(-time.Minute))

	// worker 合法 claim（租約 5 分鐘，還沒到期）。
	if ok, err := repo.Claim(ctx, id, now.Add(dlClaimLease), now); err != nil || !ok {
		t.Fatalf("worker claim: ok=%v err=%v", ok, err)
	}

	// 慢 handler 這時才跑完，標記自己那一列。租約在它眼裡不存在。
	ok, err := repo.MarkProcessed(ctx, id, now)
	if err != nil {
		t.Fatalf("consumer MarkProcessed: %v", err)
	}
	if !ok {
		t.Skip("MarkProcessed 已帶租約 fence（rows=0）—— 本不對稱不存在，這是好事")
	}
	t.Log("consumer 的 MarkProcessed 沒有租約述詞：它可以把一列**已被 worker claim**的事件" +
		"標成 PROCESSED，而 worker 正在跑第二份副作用。" +
		"擋住這件事的唯一機制是 handlerLease(60s) > handler 最壞耗時，不是任何 CAS。")
}

// strictRefundReprocessor 被呼叫到就回 error：本檔只造 ORDER_PAID 的列，
// 走到退款軸代表 process 的分派寫反了（#3073）。
//
// 與 deadline_worker_test.go 的同名型別是**兩份**：那一份帶 `//go:build !integration`，
// 在本 build 下不存在。為了一個五行的替身另開一個無 tag 的檔案不划算——
// 重點是兩份的行為必須一致，所以連訊息也照抄。
type strictRefundReprocessor struct{}

func (strictRefundReprocessor) ReprocessFromInbox(_ context.Context, ev domain.InboxEvent) error {
	return fmt.Errorf("非退款事件卻走了退款重跑入口（inbox_id=%d, event_type=%s）—— 分派寫反了",
		ev.ID, ev.EventType)
}
