package worker

import (
	"context"
	"time"
)

const rcInterval = 60 * time.Second

type rcRunner interface {
	RunOnce(ctx context.Context) error
}

type recheckWorker struct{ uc rcRunner }

// RunOnce 由排程器每 60 秒呼叫一次。
func (w *recheckWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
