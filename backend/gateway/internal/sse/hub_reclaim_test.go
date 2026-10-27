package sse

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// issue #2040：殭屍連線佔住 per-user slot 使 cap=3 實際等於 1。
//
// 這些測試全部用可注入的假時鐘。時間維度是本案的核心——「連線幾歲」決定了它
// 會不會被回收——所以測試必須真的推進時間，而不是只驗資料結構的形狀；同時也
// 不能真的等 8 分鐘。
// ---------------------------------------------------------------------------

// fakeClock 是可手動推進的時鐘。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	// 用固定基準時間，讓測試不受真實時間影響。
	return &fakeClock{t: time.Date(2026, 7, 31, 3, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newClockedHub 建一個時鐘被接管的 Hub。
func newClockedHub(cfg HubConfig) (*Hub, *fakeClock) {
	clk := newFakeClock()
	h := NewHub(cfg, nil)
	h.now = clk.Now
	return h, clk
}

// userConn 建一條屬於某 accountId 的連線。
func userConn(t *testing.T, uid int64) *Connection {
	t.Helper()
	c := newConn(t, "notification:user:1", "")
	c.AccountID = uid
	return c
}

func isClosed(c *Connection) bool {
	select {
	case <-c.Closed():
		return true
	default:
		return false
	}
}

// TestHub_ZombieConnections_DoNotStarveUserCap 是這張票的直接重現。
//
// 情境（照 log 還原出來的真實序列）：賣家訂單頁需要 2 條 SSE。使用者離開頁面
// 後，中間層仍替那 2 條收下 heartbeat，origin 完全感知不到 → 它們變成殭屍、
// 續佔位子。使用者重新載入頁面，新頁面同樣要 2 條。cap=3 的情況下：
//
//	修復前：第 1 條佔到最後一個空位，第 2 條吃 429 → 賣家靜默失去即時同步。
//	修復後：第 2 條把最舊的殭屍位子收回來 → 兩條都通。
func TestHub_ZombieConnections_DoNotStarveUserCap(t *testing.T) {
	const uid = int64(2685914682)
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 3
	hub, clk := newClockedHub(cfg)

	// t=0：舊頁面的兩條連線。
	zombie1 := userConn(t, uid)
	if _, _, err := hub.Subscribe(zombie1); err != nil {
		t.Fatalf("zombie1 subscribe: %v", err)
	}
	clk.Advance(time.Second)
	zombie2 := userConn(t, uid)
	if _, _, err := hub.Subscribe(zombie2); err != nil {
		t.Fatalf("zombie2 subscribe: %v", err)
	}

	// 使用者離開頁面。關鍵：**沒有任何 Unsubscribe**——origin 收不到任何訊號，
	// 這正是殭屍連線的定義。時間往前走 100 秒（票內實測的殭屍壽命是 600 秒，
	// 100 秒只是「使用者重新載入時它們還活著」的任一時點）。
	clk.Advance(100 * time.Second)

	// 重新載入後的新頁面：同樣要兩條。
	fresh1 := userConn(t, uid)
	if _, _, err := hub.Subscribe(fresh1); err != nil {
		t.Fatalf("fresh1 subscribe: %v", err)
	}
	fresh2 := userConn(t, uid)
	if _, _, err := hub.Subscribe(fresh2); err != nil {
		t.Fatalf("fresh2 subscribe 應該要成功（修復前這裡是 ErrUserCapReached，"+
			"賣家訂單頁就是在這裡靜默失去即時同步）: %v", err)
	}

	// 讓位的必須是最舊的那條，而不是任何一條新的。
	if !isClosed(zombie1) {
		t.Error("最舊的殭屍 zombie1 應該被回收並關閉")
	}
	if isClosed(fresh1) || isClosed(fresh2) {
		t.Error("新連線不該被回收")
	}

	// cap 仍然名副其實：同時存活的位子數不得超過上限。
	if got := hub.CapGroupSize(uid, ""); got != cfg.MaxConnPerUser {
		t.Errorf("per-user 佔位數 = %d，want %d（cap 必須仍然是硬上限）", got, cfg.MaxConnPerUser)
	}
	if got := hub.ActiveConnections(); got != int64(cfg.MaxConnPerUser) {
		t.Errorf("ActiveConnections = %d，want %d", got, cfg.MaxConnPerUser)
	}
}

// TestHub_MaxLifetime_ReclaimsZombiesAndFreesSlot 驗證「殭屍會被回收」與
// 「回收後 slot 可重用」這兩件事，且判定基準是**時間**。
//
// 這條路徑是唯一不需要有新連線來敲門就會發生的回收：使用者關掉所有分頁再也
// 不回來時，殭屍不會有人去踢它，得靠 GC 迴圈自己收。
func TestHub_MaxLifetime_ReclaimsZombiesAndFreesSlot(t *testing.T) {
	const uid = int64(777)
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 3
	cfg.MaxConnLifetime = 8 * time.Minute
	hub, clk := newClockedHub(cfg)

	conns := make([]*Connection, 0, 3)
	for i := 0; i < 3; i++ {
		c := userConn(t, uid)
		if _, _, err := hub.Subscribe(c); err != nil {
			t.Fatalf("conn %d subscribe: %v", i, err)
		}
		conns = append(conns, c)
	}
	if got := hub.CapGroupSize(uid, ""); got != 3 {
		t.Fatalf("前置：佔位數 = %d，want 3", got)
	}

	// --- 時間維度 (a)：還沒到上限，GC 不該動任何東西 ---
	clk.Advance(cfg.MaxConnLifetime - time.Second)
	hub.runGC()
	if got := hub.CapGroupSize(uid, ""); got != 3 {
		t.Fatalf("未達 MaxConnLifetime 就被回收：佔位數 = %d，want 3", got)
	}
	for i, c := range conns {
		if isClosed(c) {
			t.Fatalf("conn %d 在 %v 時就被關閉，但上限是 %v", i, cfg.MaxConnLifetime-time.Second, cfg.MaxConnLifetime)
		}
	}

	// --- 時間維度 (b)：跨過上限，GC 必須全部收掉 ---
	clk.Advance(2 * time.Second) // 現在 age = MaxConnLifetime + 1s
	hub.runGC()

	for i, c := range conns {
		if !isClosed(c) {
			t.Errorf("conn %d 超過 MaxConnLifetime 後仍未被回收", i)
		}
	}
	if got := hub.CapGroupSize(uid, ""); got != 0 {
		t.Errorf("回收後佔位數 = %d，want 0（位子沒有真的被釋放）", got)
	}
	if got := hub.ActiveConnections(); got != 0 {
		t.Errorf("回收後 ActiveConnections = %d，want 0", got)
	}

	// --- 回收後 slot 可重用：整組 cap 都能重新用滿 ---
	for i := 0; i < cfg.MaxConnPerUser; i++ {
		c := userConn(t, uid)
		if _, _, err := hub.Subscribe(c); err != nil {
			t.Fatalf("回收後第 %d 條重用 slot 失敗: %v", i, err)
		}
	}
	if got := hub.CapGroupSize(uid, ""); got != cfg.MaxConnPerUser {
		t.Errorf("重用後佔位數 = %d，want %d", got, cfg.MaxConnPerUser)
	}
}

// TestHub_MaxLifetime_Disabled 確認 0 代表停用（給需要關掉這個行為的環境）。
func TestHub_MaxLifetime_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConnLifetime = 0
	hub, clk := newClockedHub(cfg)

	c := userConn(t, 1)
	if _, _, err := hub.Subscribe(c); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	clk.Advance(72 * time.Hour)
	hub.runGC()
	if isClosed(c) {
		t.Error("MaxConnLifetime=0 時不該有任何以存活時間為由的回收")
	}
}

// TestHub_EvictBudget_ExhaustedStillRejects 守住 cap 的防濫用性質。
//
// 回收本身不會增加資源用量（同時存活數永遠 ≤ limit），但無限制回收會讓真的想
// 開超過 cap 條連線的客戶端陷入互踢迴圈。額度用完就必須回到 429，客戶端連兩次
// 失敗後才會停止重試、系統才會收斂。
func TestHub_EvictBudget_ExhaustedStillRejects(t *testing.T) {
	const uid = int64(555)
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 1 // budget = 2 * 1 = 2
	cfg.EvictWindow = 30 * time.Second
	hub, clk := newClockedHub(cfg)

	first := userConn(t, uid)
	if _, _, err := hub.Subscribe(first); err != nil {
		t.Fatalf("first: %v", err)
	}

	// 額度內的兩次回收都該成功。
	for i := 0; i < evictBudgetFor(cfg.MaxConnPerUser); i++ {
		clk.Advance(time.Second)
		c := userConn(t, uid)
		if _, _, err := hub.Subscribe(c); err != nil {
			t.Fatalf("視窗內第 %d 次回收應成功，卻得到 %v", i+1, err)
		}
	}

	// 第三次：額度用完 → 回到 429。
	clk.Advance(time.Second)
	over := userConn(t, uid)
	if _, _, err := hub.Subscribe(over); !errors.Is(err, ErrUserCapReached) {
		t.Fatalf("額度用完後應回 ErrUserCapReached，卻得到 %v", err)
	}

	// 全程佔位數不得超過 cap。
	if got := hub.CapGroupSize(uid, ""); got != cfg.MaxConnPerUser {
		t.Errorf("佔位數 = %d，want %d", got, cfg.MaxConnPerUser)
	}

	// --- 時間維度：跨過視窗後額度重置，服務自行復原 ---
	clk.Advance(cfg.EvictWindow)
	afterWindow := userConn(t, uid)
	if _, _, err := hub.Subscribe(afterWindow); err != nil {
		t.Fatalf("跨過 EvictWindow 後額度應重置，卻得到 %v", err)
	}
}

// TestHub_EvictedConnection_LateUnsubscribeIsNoop 守住重複釋放。
//
// 被回收的連線，其 ServeSSE goroutine 稍後仍會跑到自己的 defer Unsubscribe。
// 若不擋，totalConns 會被扣兩次、pod 層級容量統計會漂移成負數，最終讓
// MaxConnPerPod 形同虛設。
func TestHub_EvictedConnection_LateUnsubscribeIsNoop(t *testing.T) {
	const uid = int64(999)
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 1
	hub, clk := newClockedHub(cfg)

	victim := userConn(t, uid)
	if _, _, err := hub.Subscribe(victim); err != nil {
		t.Fatalf("victim: %v", err)
	}
	clk.Advance(time.Second)
	winner := userConn(t, uid)
	if _, _, err := hub.Subscribe(winner); err != nil {
		t.Fatalf("winner: %v", err)
	}
	if !isClosed(victim) {
		t.Fatal("前置：victim 應已被回收")
	}

	before := hub.ActiveConnections()
	// 模擬 victim 的 ServeSSE goroutine 事後跑到 defer Unsubscribe。
	hub.Unsubscribe(victim)
	hub.Unsubscribe(victim) // 再一次也必須是 no-op

	if got := hub.ActiveConnections(); got != before {
		t.Errorf("遲來的 Unsubscribe 改變了 ActiveConnections：%d → %d", before, got)
	}
	if got := hub.CapGroupSize(uid, ""); got != 1 {
		t.Errorf("遲來的 Unsubscribe 動到了佔位數：got %d，want 1（winner 仍在）", got)
	}

	// winner 正常退出後位子必須真的回到 0。
	hub.Unsubscribe(winner)
	if got := hub.CapGroupSize(uid, ""); got != 0 {
		t.Errorf("winner 退出後佔位數 = %d，want 0", got)
	}
	if got := hub.ActiveConnections(); got != 0 {
		t.Errorf("winner 退出後 ActiveConnections = %d，want 0", got)
	}
}

// TestHub_CapIsPerKey 確認回收與計數是分身分的，不會互相影響。
func TestHub_CapIsPerKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConnPerUser = 1
	hub, _ := newClockedHub(cfg)

	a := userConn(t, 1001)
	b := userConn(t, 1002)
	if _, _, err := hub.Subscribe(a); err != nil {
		t.Fatalf("a: %v", err)
	}
	if _, _, err := hub.Subscribe(b); err != nil {
		t.Fatalf("b: %v", err)
	}
	if isClosed(a) || isClosed(b) {
		t.Error("不同 accountId 之間不該互相回收")
	}
	if got := hub.CapGroupSize(1001, ""); got != 1 {
		t.Errorf("uid 1001 佔位數 = %d，want 1", got)
	}
	if got := hub.CapGroupSize(1002, ""); got != 1 {
		t.Errorf("uid 1002 佔位數 = %d，want 1", got)
	}
}

// TestConnGroup_NeverEvictsHalfConstructedConnection 守住一個很窄但會造成永久
// 洩漏的窗口：佔位在 Subscribe 最前面就完成，但「加入 topic／計入 totalConns／
// 標記 admitted」是之後幾行才做。若這段空隙內另一條同身分連線把它選成 victim，
// teardown 會因為 markReleased 看到 admitted=false 而整個略過 —— 位子被移出
// group、連線卻沒關也沒退出 topic。
//
// 這個窗口在真實併發下極難穩定重現，所以直接對 connGroup 做確定性驗證。
func TestConnGroup_NeverEvictsHalfConstructedConnection(t *testing.T) {
	clk := newFakeClock()
	g := newConnGroup(2)

	// ready：已完成訂閱、且是最舊的那條。
	ready := newConn(t, "t", "")
	ready.subscribedAt = clk.Now()
	ready.admitted.Store(true)

	// halfBuilt：更舊，但還沒跑完 Subscribe（admitted 仍為 false）。
	halfBuilt := newConn(t, "t", "")
	halfBuilt.subscribedAt = clk.Now().Add(-time.Hour)

	if res, _ := g.admit(ready, clk.Now(), 4, time.Minute); res != admitOK {
		t.Fatalf("前置 ready 入座失敗: %v", res)
	}
	if res, _ := g.admit(halfBuilt, clk.Now(), 4, time.Minute); res != admitOK {
		t.Fatalf("前置 halfBuilt 入座失敗: %v", res)
	}

	clk.Advance(time.Minute)
	incoming := newConn(t, "t", "")
	incoming.subscribedAt = clk.Now()
	res, victim := g.admit(incoming, clk.Now(), 4, time.Minute)
	if res != admitOK {
		t.Fatalf("incoming 應可回收位子入座，got %v", res)
	}
	if victim == halfBuilt {
		t.Fatal("回收了尚未完成訂閱的連線 —— 這條會既沒關閉也沒退出 topic，變成永久洩漏")
	}
	if victim != ready {
		t.Fatalf("victim 應為已就緒且最舊的 ready，got %v", victim)
	}
}

// TestConnGroup_AllHalfConstructed_Rejects：滿座且全部都還在就緒中時，沒有可回收
// 對象，必須回 admitRejected（那是真的滿了）。
func TestConnGroup_AllHalfConstructed_Rejects(t *testing.T) {
	clk := newFakeClock()
	g := newConnGroup(1)

	pending := newConn(t, "t", "")
	pending.subscribedAt = clk.Now()
	if res, _ := g.admit(pending, clk.Now(), 4, time.Minute); res != admitOK {
		t.Fatalf("前置入座失敗: %v", res)
	}

	clk.Advance(time.Minute)
	incoming := newConn(t, "t", "")
	incoming.subscribedAt = clk.Now()
	if res, victim := g.admit(incoming, clk.Now(), 4, time.Minute); res != admitRejected || victim != nil {
		t.Fatalf("want admitRejected/nil, got %v/%v", res, victim)
	}
}
