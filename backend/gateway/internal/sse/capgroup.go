package sse

import (
	"sync"
	"time"
)

// connGroup tracks the live connections sharing one cap key —— 登入使用者是
// accountId、匿名是 rate-limit subject。它取代了原本的 `sync.Map[key]*atomic.Int64`
// 純計數器：光有計數沒有連線本身，就沒辦法在額度用完時判斷「佔著位子的那條
// 還值不值得留」。
//
// # 為什麼需要「回收位子」而不是單純拒絕（issue #2040）
//
// SSE 是單向的：server 從頭到尾不從 client 讀任何位元組，因此**沒有任何存活
// 訊號**。瀏覽器離開後，只要中間層（Cloudflare／Traefik）還肯把 heartbeat 收
// 下來，origin 這端的 `ctx.Done()` 不會觸發、`w.Write` 不會失敗、
// `SetWriteDeadline`（stream.go 每次寫入前都重設）也永遠碰不到——這條連線就
// 一路佔著位子，直到中間層自己在 600 秒把 TCP 斷掉。實測 log 還原出的每一筆
// 429 當下，都恰好有 2 條壽命 600.0 秒的殭屍連線常駐，於是 cap=3 實際等於 1，
// 賣家訂單頁（單頁需要 2 條）第 2 條必吃 429。
//
// 因為缺的是「存活訊號」而不是「逾時設定」，純粹把 lifetime 調短救不了：前端
// 連吃兩次 429（間隔 8 秒）就會把狀態設成 unavailable 停止重試，殭屍窗口再短
// 也早就把使用者踢進靜默降級了。唯一能讓 cap 名副其實的做法，是讓**新連線有
// 辦法把最舊的位子收回來**。
//
// # 為什麼可以安全地回收
//
// 回收不會提高資源用量：無論怎麼換，同時存活的連線數永遠 ≤ limit。cap 想擋的
// 是「一個身分佔用無限資源」，那件事由 limit 本身保證，不是由「拒絕」保證。
//
// # 為什麼還是要保留額度上限
//
// 若無限制回收，一個真的想開超過 cap 條連線的使用者（例如同時開兩個賣家訂單
// 頁 = 4 條 > 3）會進入互踢迴圈：被踢的那條 800ms 後重連、再踢掉別人，永無止
// 境且客戶端永遠看不到 429、永遠不會收手。所以每個 key 每個時間窗只給定量的
// 回收額度；額度用完就回到原本的 429，客戶端連兩次失敗後自行停止重試，系統收
// 斂到穩定狀態。
type connGroup struct {
	mu    sync.Mutex
	limit int

	conns map[*Connection]struct{}

	// 回收額度（滑動重置的固定窗）。
	windowStart time.Time
	evictUsed   int

	// removed 在 group 清空、即將自 hub 的 key map 刪除時於鎖內設為 true。
	// 併發的 admit 可能正好持有這個已作廢的 group 指標，看到旗標就重取一次，
	// 避免把連線記到一個沒人會再讀到的 group 上（原子計數器版本存在同樣的
	// 競態：decrement 到 0 刪 key 的同時，別人可能正拿著同一個計數器 +1，
	// 那次 +1 就此消失、額度被永久多發）。
	removed bool
}

func newConnGroup(limit int) *connGroup {
	return &connGroup{
		limit: limit,
		conns: make(map[*Connection]struct{}),
	}
}

// admitResult 描述 admit 的三種結局。
type admitResult int

const (
	// admitOK：已入座（victim 若非 nil，代表是把它的位子收回來換到的）。
	admitOK admitResult = iota
	// admitRejected：滿了且無法回收（額度用完或沒有可收的對象）。
	admitRejected
	// admitStaleGroup：這個 group 已被移出 hub，呼叫端必須重取後再試。
	admitStaleGroup
)

// admit 嘗試讓 conn 入座。
//
// budget 是本時間窗允許的回收次數；window 是窗長。回傳的 victim 由呼叫端在
// **釋放 group 鎖之後**才做真正的拆除（Close／退出 topic／扣總數），避免
// group 鎖與 hub 主鎖交叉持有造成鎖序反轉。
func (g *connGroup) admit(conn *Connection, now time.Time, budget int, window time.Duration) (admitResult, *Connection) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.removed {
		return admitStaleGroup, nil
	}
	if len(g.conns) < g.limit {
		g.conns[conn] = struct{}{}
		return admitOK, nil
	}

	// 滿了 → 看能不能把最舊的位子收回來。
	if g.windowStart.IsZero() || now.Sub(g.windowStart) >= window {
		g.windowStart = now
		g.evictUsed = 0
	}
	if g.evictUsed >= budget {
		return admitRejected, nil
	}
	victim := g.oldestLocked()
	if victim == nil {
		return admitRejected, nil
	}

	delete(g.conns, victim)
	g.evictUsed++
	g.conns[conn] = struct{}{}
	return admitOK, victim
}

// release 把 conn 移出 group。回傳 true 代表 group 已空並被標記為 removed，
// 呼叫端應把這個 key 自 hub 的 map 刪掉。
//
// 對不在 group 內的連線是 no-op：被回收（evict）的那條在 admit 當下就已經移
// 除，它自己的 goroutine 稍後跑到 defer Unsubscribe 時不該再扣一次。
func (g *connGroup) release(conn *Connection) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.conns[conn]; !ok {
		return false
	}
	delete(g.conns, conn)
	if len(g.conns) == 0 && !g.removed {
		g.removed = true
		return true
	}
	return false
}

// size 回傳目前佔用的位子數（測試與 metrics 用）。
func (g *connGroup) size() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.conns)
}

// oldestLocked 取入座時間最早、且**已完成訂閱**的連線。
//
// 為什麼挑最舊：殭屍是「上一個頁面留下來的」，必然早於當前頁面剛建立的那幾條，
// 所以「最舊」在真正要救的情境裡精準命中殭屍；同時也保證剛入座的新連線不可能
// 被自己這一輪的下一條請求踢掉（它是最新的）。
// 時間相同時以 ID 決定，讓行為在測試與 log 中可重現。
//
// 為什麼要跳過 admitted == false：佔位在 Subscribe 的最前面就完成，但「加入
// topic、計入 totalConns、標記 admitted」是之後幾行才做的。若這個空隙裡另一條
// 同身分的連線進來把它選為 victim，teardown 會因為 markReleased 看到尚未
// admitted 而整個略過——位子被移出 group、連線卻沒關、也沒退出 topic，變成永久
// 洩漏。只把「已完全就緒」的連線視為可回收對象即可完全避開這個窗口；代價是滿座
// 且全部都還在就緒中時回 429，那本來就是正確答案（那是真的滿了）。
func (g *connGroup) oldestLocked() *Connection {
	var oldest *Connection
	for c := range g.conns {
		if !c.admitted.Load() {
			continue
		}
		switch {
		case oldest == nil:
			oldest = c
		case c.subscribedAt.Before(oldest.subscribedAt):
			oldest = c
		case c.subscribedAt.Equal(oldest.subscribedAt) && c.ID < oldest.ID:
			oldest = c
		}
	}
	return oldest
}
