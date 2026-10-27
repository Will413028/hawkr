package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeOutboxRepo struct {
	domain.OutboxRepository

	pending   []domain.OutboxEvent
	claimed   map[int64]bool
	done      map[int64]bool
	parked    map[int64]string
	parkUntil map[int64]time.Time
	attempts  map[int64]int32
	claimLog  []time.Time

	// parkNotPending 模擬「那一列已被維運 SQL 隔離成 'failed'」⇒ Park 打在空集合上。
	parkNotPending bool
}

func newFakeOutboxRepo(evs ...domain.OutboxEvent) *fakeOutboxRepo {
	return &fakeOutboxRepo{
		pending: evs, claimed: map[int64]bool{}, done: map[int64]bool{},
		parked: map[int64]string{}, parkUntil: map[int64]time.Time{},
		attempts: map[int64]int32{},
	}
}

func (f *fakeOutboxRepo) FetchPending(context.Context, time.Time, int32) ([]domain.OutboxEvent, error) {
	// 🔴 只排除 done：**停泊過的列仍然是候選**。這正是 Park 與舊 MarkFailed 的差別，
	// 而舊 fake 把 failed 排除掉，等於在測試裡複製了那個永久旗標的行為。
	out := make([]domain.OutboxEvent, 0, len(f.pending))
	for _, e := range f.pending {
		if !f.done[e.ID] {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeOutboxRepo) Claim(_ context.Context, id int64, leaseUntil, _ time.Time) (bool, error) {
	f.claimed[id] = true
	f.claimLog = append(f.claimLog, leaseUntil)
	return true, nil
}

func (f *fakeOutboxRepo) MarkDone(_ context.Context, id int64, _ time.Time) error {
	f.done[id] = true
	return nil
}

// Park 記下停泊：**status 不變**（所以 FetchPending 下一輪仍看得到它），只記錄
// 停泊到期時間與錯誤。fake 這樣寫是刻意的——真表的 Park 也不動 status，而
// 「停泊之後那一列還在不在 pending」正是本輪要守的不變式。
func (f *fakeOutboxRepo) Park(_ context.Context, id int64, parkUntil time.Time, lastError string) (bool, error) {
	f.parked[id] = lastError
	f.parkUntil[id] = parkUntil
	if f.parkNotPending {
		return false, nil
	}
	return true, nil
}

func (f *fakeOutboxRepo) UpdateAttempt(_ context.Context, id int64, n int32, _ time.Time, _ *string) error {
	f.attempts[id] = n
	return nil
}

type fakePublisher struct {
	calls []publishCall
	err   error
}

type publishCall struct {
	exchange   string
	routingKey string
	body       []byte
}

func (p *fakePublisher) PublishRaw(_ context.Context, exchange, routingKey string, body []byte) error {
	if p.err != nil {
		return p.err
	}
	p.calls = append(p.calls, publishCall{exchange, routingKey, append([]byte(nil), body...)})
	return nil
}

type fakeSellerResolver struct {
	accountID int64
	err       error
}

func (f fakeSellerResolver) ResolveSellerAccountID(context.Context, int64) (int64, error) {
	return f.accountID, f.err
}

func testOutboxEvent(id int64, payload map[string]any) domain.OutboxEvent {
	b, _ := json.Marshal(payload)
	return domain.OutboxEvent{
		ID: id, AggregateType: "DOCUMENT", AggregateID: 42,
		EventType: "INVOICE_ISSUED", Exchange: "hawkr.invoice",
		RoutingKey: "invoice.issued", Payload: b, Status: "pending",
	}
}

func testOutboxCfg() outbox.Config {
	return outbox.Config{
		MaxAttempts: 8, BaseBackoff: time.Second, MaxBackoff: time.Minute,
		FetchBatch: 100, ClaimLease: 30 * time.Second,
	}
}

// ---------------------------------------------------------------------------
// 順序不變式：publish 成功之前不得標記完成
// ---------------------------------------------------------------------------

// TestOutboxWorker_PublishFailureLeavesRowPending 是**變異④的靶心**。
//
// 把 process 改成「先 MarkDone 再 publish」之後，publish 失敗那一列仍會變成 done
// ⇒ 買家該收到的發票通知從此不存在，而且沒有任何錯誤、沒有任何告警。
// 這一支用「publish 失敗時該列必須還在（未 done）」把那個形狀釘死。
func TestOutboxWorker_PublishFailureLeavesRowPending(t *testing.T) {
	repo := newFakeOutboxRepo(testOutboxEvent(1, map[string]any{"store_id": "2685914682"}))
	pub := &fakePublisher{err: errors.New("broker unreachable")}
	w := NewOutboxWorker(repo, pub, fakeSellerResolver{accountID: 99}, testOutboxCfg(), obParkInterval, testObsB, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if repo.done[1] {
		t.Fatal("publish 失敗時該列被標記完成了 —— 那則事件從此蒸發，沒有任何訊號")
	}
	if repo.attempts[1] != 1 {
		t.Fatalf("attempt_count = %d，want 1（失敗要記帳並退避）", repo.attempts[1])
	}
	if len(pub.calls) != 0 {
		t.Fatalf("publisher 不該有成功紀錄，got %d", len(pub.calls))
	}
}

// TestOutboxWorker_PublishSuccessMarksDone 是上一支的陽性對照：兩支缺一，
// 「永遠不 MarkDone」也會讓上一支通過。
func TestOutboxWorker_PublishSuccessMarksDone(t *testing.T) {
	repo := newFakeOutboxRepo(testOutboxEvent(1, map[string]any{"store_id": "2685914682"}))
	pub := &fakePublisher{}
	w := NewOutboxWorker(repo, pub, fakeSellerResolver{accountID: 99}, testOutboxCfg(), obParkInterval, testObsB, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !repo.claimed[1] {
		t.Fatal("該列沒有被 claim")
	}
	if len(pub.calls) != 1 {
		t.Fatalf("publish 次數 = %d，want 1", len(pub.calls))
	}
	if !repo.done[1] {
		t.Fatal("publish 成功後該列必須被標記完成")
	}
	if got := pub.calls[0].exchange; got != "hawkr.invoice" {
		t.Errorf("exchange = %q", got)
	}
	if got := pub.calls[0].routingKey; got != "invoice.issued" {
		t.Errorf("routing key = %q", got)
	}
}

// ---------------------------------------------------------------------------
// 派送時解析 notify_seller_account_id（§6.3.3.2）
// ---------------------------------------------------------------------------

func TestOutboxWorker_ResolvesSellerAccountAtDispatch(t *testing.T) {
	repo := newFakeOutboxRepo(testOutboxEvent(1, map[string]any{
		"store_id": "2685914682", "order_no": "O1",
	}))
	pub := &fakePublisher{}
	w := NewOutboxWorker(repo, pub, fakeSellerResolver{accountID: 777}, testOutboxCfg(), obParkInterval, testObsB, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	got := decodePublished(t, pub)
	// 🔴 字串不是數字：int64 進 generic-map consumer 會被解成 float64 而截斷。
	if got["notify_seller_account_id"] != "777" {
		t.Fatalf("notify_seller_account_id = %#v，want 字串 \"777\"", got["notify_seller_account_id"])
	}
}

// TestOutboxWorker_UnresolvedSellerStillPublishes 守「解析失敗帶 0、不阻斷派送」。
//
// 賣家向那一則通知會被下游 skip，但買家向靠 buyer_id 路由、完全不受影響。
// 為了賣家那一則把整批事件卡住，是把小缺口換成大缺口。
func TestOutboxWorker_UnresolvedSellerStillPublishes(t *testing.T) {
	repo := newFakeOutboxRepo(testOutboxEvent(1, map[string]any{
		"store_id": "2685914682", "buyer_id": "2623900520",
	}))
	pub := &fakePublisher{}
	w := NewOutboxWorker(repo, pub, UnresolvedSellerAccountResolver{}, testOutboxCfg(), obParkInterval, testObsB, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("解析失敗不得阻斷派送：publish 次數 = %d", len(pub.calls))
	}
	got := decodePublished(t, pub)
	if got["notify_seller_account_id"] != "0" {
		t.Fatalf("notify_seller_account_id = %#v，want \"0\"", got["notify_seller_account_id"])
	}
	if got["buyer_id"] != "2623900520" {
		t.Fatalf("買家路由欄位被動到了：%#v", got["buyer_id"])
	}
	if !repo.done[1] {
		t.Fatal("派送成功仍要標記完成")
	}
}

// TestOutboxWorker_DoesNotOverwriteExistingNotifyField 守冪等：重送同一列不得把
// 已經解析好的值換掉。
func TestOutboxWorker_DoesNotOverwriteExistingNotifyField(t *testing.T) {
	repo := newFakeOutboxRepo(testOutboxEvent(1, map[string]any{
		"store_id": "1", "notify_seller_account_id": "555",
	}))
	pub := &fakePublisher{}
	w := NewOutboxWorker(repo, pub, fakeSellerResolver{accountID: 999}, testOutboxCfg(), obParkInterval, testObsB, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if got := decodePublished(t, pub)["notify_seller_account_id"]; got != "555" {
		t.Fatalf("既有值被覆寫成 %#v", got)
	}
}

// 規則本體已搬到 internal/notifyrouting（葉 package，讓 usecase 側的契約測試打得到
// 同一份）。ParseStringID 的行為測試隨之搬到 internal/notifyrouting/notifyrouting_test.go。

func decodePublished(t *testing.T, pub *fakePublisher) map[string]any {
	t.Helper()
	if len(pub.calls) == 0 {
		t.Fatal("沒有任何 publish")
	}
	var out map[string]any
	if err := json.Unmarshal(pub.calls[len(pub.calls)-1].body, &out); err != nil {
		t.Fatalf("decode published body: %v", err)
	}
	return out
}

// ---------------------------------------------------------------------------
// 停泊而非終結（甲）
// ---------------------------------------------------------------------------

// TestOutboxWorker_RetryExhaustionParksInsteadOfFlaggingTerminal 是「永久旗標 ＋
// 告警方向相反」那個缺陷的行為層證據。
//
// 🔴 舊行為：耗盡重試 ⇒ `SET status='failed', completed_at=now()`。而
//   - FetchPending／Claim／idx_invoutbox_pending 的述詞全是 status='pending'，
//     且本表沒有任何時間欄能讓 'failed' 回到候選集合 ⇒ 永久旗標
//   - CountPending／OldestPendingCreatedAt 的述詞也是 'pending' ⇒ 那一列一落到
//     'failed' 就退出 outbox_oldest_pending_age_seconds ⇒ 告警 InvoiceOutboxLag
//     在事件**真正卡死的那一刻**由 firing 轉 resolved
//
// 這一支釘住三件事，缺一都會讓上面那個形狀復活：
//
//	① 走的是 Park 不是任何終態寫入
//	② 停泊之後那一列**仍然是候選**（fake 的 FetchPending 只排除 done）
//	③ 停泊到期時間明顯晚於正常退避梯子的總長 ⇒ 不是沒有退避的熱迴圈
func TestOutboxWorker_RetryExhaustionParksInsteadOfFlaggingTerminal(t *testing.T) {
	ev := testOutboxEvent(1, map[string]any{"store_id": "2685914682"})
	cfg := testOutboxCfg()
	ev.AttemptCount = int32(cfg.MaxAttempts - 1) // 下一次失敗就是「耗盡」
	repo := newFakeOutboxRepo(ev)
	pub := &fakePublisher{err: errors.New("broker unreachable")}
	w := NewOutboxWorker(repo, pub, fakeSellerResolver{accountID: 99}, cfg, obParkInterval, testObsB, nil)

	before := time.Now()
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// ① 停泊發生了，而且帶著那句錯誤（維運唯一需要的線索）。
	if repo.parked[1] == "" {
		t.Fatal("耗盡重試沒有走 Park —— 若改回 status='failed'，那一列會永久退出候選集合，" +
			"而 InvoiceOutboxLag 會在它卡死那一刻熄滅")
	}
	// ② 仍在候選集合：這是「告警持續亮著」與「修好之後自己排空」共同的前提。
	rows, err := repo.FetchPending(context.Background(), before, 100)
	if err != nil {
		t.Fatalf("FetchPending: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("停泊之後候選集合有 %d 列，want 1 —— 那一列離開候選集合就等於永久旗標", len(rows))
	}
	if repo.done[1] {
		t.Fatal("停泊不是完成")
	}
	// ③ 停泊間隔要大於整條退避梯子，否則只是把梯子接長。
	ladder := time.Duration(0)
	d := cfg.BaseBackoff
	for i := 1; i < cfg.MaxAttempts; i++ {
		if d *= 2; d > cfg.MaxBackoff {
			d = cfg.MaxBackoff
		}
		ladder += d
	}
	gotPark := repo.parkUntil[1].Sub(before)
	if gotPark <= ladder {
		t.Fatalf("停泊 %v 不大於整條退避梯子 %v —— 看不出「這一列已經不正常」", gotPark, ladder)
	}
	if gotPark < obParkInterval || gotPark > obParkInterval+time.Minute {
		t.Fatalf("停泊到期 = now+%v，want ≈ now+%v", gotPark, obParkInterval)
	}
}

// TestOutboxWorker_ParkOnAnAlreadyQuarantinedRowIsNotAnError 守 rows=0 的處置。
//
// 'failed' 已不再由程式寫入，唯一來源是維運 SQL 的隔離。打在那樣一列上的 Park 會
// rows=0；那不是錯誤（目標狀態已達成），但必須留痕，否則它從此無聲。
func TestOutboxWorker_ParkOnAnAlreadyQuarantinedRowIsNotAnError(t *testing.T) {
	ev := testOutboxEvent(1, map[string]any{"store_id": "2685914682"})
	cfg := testOutboxCfg()
	ev.AttemptCount = int32(cfg.MaxAttempts - 1)
	repo := newFakeOutboxRepo(ev)
	repo.parkNotPending = true
	pub := &fakePublisher{err: errors.New("broker unreachable")}
	w := NewOutboxWorker(repo, pub, fakeSellerResolver{accountID: 99}, cfg, obParkInterval, testObsB, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce 不該因為 rows=0 而回錯：%v", err)
	}
}
