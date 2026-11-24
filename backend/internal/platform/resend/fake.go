package resend

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Sent 是 Fake 收下的一封信。
type Sent struct{ Email Email }

// Fake 是記憶體內的 Sender，給測試與沒有設定 Resend 的環境：預設全部成功；Script 依收件位址指定接下來幾次的結果（nil 是成功）。
// 同一個 IdempotencyKey 成功過就不再記一封（模擬 Resend 的冪等）。
type Fake struct {
	mu     sync.Mutex
	sent   []Sent
	keys   map[string]bool
	script map[string][]error
	log    *slog.Logger
}

// NewFake 建立 Fake；log 非 nil 時每封信記一行 Info（不含位址與內容）。
func NewFake(log *slog.Logger) *Fake {
	return &Fake{keys: map[string]bool{}, script: map[string][]error{}, log: log}
}

// Script 讓寄給 to 的接下來幾次依序得到 results。
func (f *Fake) Script(to string, results ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script[to] = append(f.script[to], results...)
}

// Sent 回傳已寄出的信。
func (f *Fake) Sent() []Sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Sent(nil), f.sent...)
}

// Send 實作 Sender。
func (f *Fake) Send(ctx context.Context, e Email) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w：%w", ErrTransient, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if q := f.script[e.To]; len(q) > 0 {
		err := q[0]
		f.script[e.To] = q[1:]
		if err != nil {
			return err
		}
	}
	if e.IdempotencyKey != "" {
		if f.keys[e.IdempotencyKey] {
			return nil
		}
		f.keys[e.IdempotencyKey] = true
	}
	f.sent = append(f.sent, Sent{Email: e})
	if f.log != nil {
		f.log.Info("resend fake：email 已記錄，沒有寄出")
	}
	return nil
}
