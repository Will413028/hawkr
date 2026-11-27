package worker

import (
	"context"
	"time"
)

const pdfInterval = 60 * time.Second

type pdfRunner interface {
	RunOnce(ctx context.Context) error
}

type pdfWorker struct{ uc pdfRunner }

// RunOnce 由排程器每 60 秒呼叫一次。
func (w *pdfWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
