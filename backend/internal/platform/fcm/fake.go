package fcm

import (
	"context"
	"log/slog"
	"sync"
)

// Call 是 Fake 收到的一次 Send。
type Call struct {
	Tokens  []string
	Message Message
}

// Fake 是記憶體內的 Sender，給測試與沒有設定 FCM 的環境：預設每個 token 都成功；Script 依 token 指定結果（先進先出，用完回到成功）。
// 每次 Send 都照 MaxBatch 切批，所以測試看得到呼叫端有沒有一次丟超過上限。
type Fake struct {
	mu     sync.Mutex
	calls  []Call
	script map[string][]Outcome
	log    *slog.Logger
	sent   int
}

// NewFake 建立 Fake；log 非 nil 時每次送出記一行 Info（不含 token 與內容），給本機環境看得到推播有沒有被送。
func NewFake(log *slog.Logger) *Fake { return &Fake{script: map[string][]Outcome{}, log: log} }

// Script 讓 token 的接下來幾次送出依序得到 outcomes。
func (f *Fake) Script(token string, outcomes ...Outcome) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script[token] = append(f.script[token], outcomes...)
}

// Calls 回傳到目前為止的呼叫（每個批次一筆）。
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// Delivered 回傳成功送出的 (token, 訊息) 數。
func (f *Fake) Delivered() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent
}

// Send 實作 Sender。
func (f *Fake) Send(ctx context.Context, tokens []string, m Message) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Result, 0, len(tokens))
	for _, batch := range Chunks(tokens) {
		f.calls = append(f.calls, Call{Tokens: append([]string(nil), batch...), Message: m})
		for _, t := range batch {
			o := OutcomeSent
			if q := f.script[t]; len(q) > 0 {
				o, f.script[t] = q[0], q[1:]
			}
			if o == OutcomeSent {
				f.sent++
			}
			out = append(out, Result{Token: t, Outcome: o, Detail: "fake"})
		}
	}
	if f.log != nil {
		f.log.Info("fcm fake：推播已記錄，沒有送出", "tokens", len(tokens))
	}
	return out, nil
}
