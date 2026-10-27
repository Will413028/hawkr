package tsid

import (
	"sync"
	"time"
)

// generator holds the mutable state behind a single NewGenerator() instance.
// Pulled out as an unexported struct (rather than closure-local variables) so
// package-internal white-box tests can construct a generator with seq primed
// near the 2^22 overflow boundary and exercise the busy-wait/rollover branch
// deterministically, without actually issuing ~4.19M calls in one millisecond.
type generator struct {
	mu     sync.Mutex
	lastMs int64
	seq    int64
}

func (g *generator) next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < g.lastMs {
		// 時鐘回撥（NTP 調整等）：不讓時間倒退，沿用 lastMs 繼續遞增序列，
		// 序列滿了會落到下面的忙等分支，等真實時間追上 lastMs 為止。
		now = g.lastMs
	}
	if now == g.lastMs {
		g.seq++
		if g.seq >= 1<<22 {
			// 刻意的正確性優先設計，不加 timeout：這個迴圈只有兩種收斂方式，
			// 要嘛等到下一毫秒、要嘛不等。加 timeout 之後，逾時要嘛回傳一個
			// 可能撞號的 id（違反這個函式存在的唯一目的——見上面的事故背景），
			// 要嘛回傳 error（會改變 func() int64 簽章，波及全部 17+ 呼叫端）。
			// 兩者都比「系統時鐘停滯這種實務上幾乎不會發生的極端情境下卡住」更糟，
			// 所以選擇讓它自旋到時間真的往前走為止。
			for now <= g.lastMs {
				now = time.Now().UnixMilli()
			}
			g.seq = 0
		}
	} else {
		g.seq = 0
	}
	g.lastMs = now
	return now<<22 | g.seq
}

// NewGenerator 回傳 TSID 生成函式（42-bit ms 時間戳 + 22-bit 同毫秒內遞增序列）。
// 產出的 int64 具時間排序性、同一個 generator 實例產出的值互不重複，適合做分散式主鍵。
// 併發安全：多個 goroutine 可同時呼叫同一個回傳的函式。
//
// 同毫秒內序列滿了（每毫秒最多 4,194,304 個）時忙等到下一毫秒，寧可短暫阻塞也不
// 犧牲唯一性——見 2026-08-07 pricebook-service 批次匯入撞號事故：舊版純隨機設計
// 在網路 round trip 天然把呼叫拉開間距時撞號機率趨近於零，但批次寫入在同一毫秒內
// 緊密呼叫上千次時（生日悖論）撞號機率逼近 100%。
func NewGenerator() func() int64 {
	g := &generator{}
	return g.next
}
