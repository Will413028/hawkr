package live

import "time"

// Backoff 是重連的退避：第 n 次的上限是 min(Max, Base·2^n)，實際值在上限的一半到上限之間（jitter）。
type Backoff struct {
	Base, Max time.Duration
	// Rand 回傳 [0,1)；注入以便測試。
	Rand func() float64
	n    int
}

// Next 回傳下一次要等多久，並前進。
func (b *Backoff) Next() time.Duration {
	d := b.Max
	if b.n < 62 {
		if shifted := b.Base << b.n; shifted > 0 && shifted < b.Max {
			d = shifted
		}
	}
	b.n++
	return time.Duration(float64(d) * (0.5 + 0.5*b.Rand()))
}

// Reset 在連線成功後呼叫，下一次從 Base 重新開始。
func (b *Backoff) Reset() { b.n = 0 }
