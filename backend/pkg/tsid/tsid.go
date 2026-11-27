package tsid

import (
	"sync"
	"time"
)

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
		now = g.lastMs
	}
	if now == g.lastMs {
		g.seq++
		if g.seq >= 1<<22 {
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
// 犧牲唯一性——見 2026-08-07 批次匯入撞號事故：舊版純隨機設計
// 在網路 round trip 天然把呼叫拉開間距時撞號機率趨近於零，但批次寫入在同一毫秒內
// 緊密呼叫上千次時（生日悖論）撞號機率逼近 100%。
func NewGenerator() func() int64 {
	g := &generator{}
	return g.next
}
