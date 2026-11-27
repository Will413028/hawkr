package worker

import (
	"context"
	"time"
)

const hcInterval = 60 * time.Second

type hcRunner interface {
	RunOnce(ctx context.Context) error
}

type healthCheckWorker struct{ uc hcRunner }

// RunOnce 由排程器每 60 秒呼叫一次。
func (w *healthCheckWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
