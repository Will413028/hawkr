package worker

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// INV.pdf —— 發票 PDF 抓檔的排程面（§7.5.3）
//
// interval 60s、FetchBatch 20、租約 120s、MaxAttempts 8（7 段退避約 2.0 小時）。
//
// 租約落在 invoice_documents.pdf_fetch_next_at ＋ pdf_fetch_attempts
// （§5.1.5，**不另建 invoice_pdf_tasks 表** —— 全書不存在那張表，看到那個名字
// 指的是同一組東西）。
// ---------------------------------------------------------------------------

// pdfInterval 是 INV.pdf 的 tick 間隔（§7.5.3）。
const pdfInterval = 60 * time.Second

// pdfRunner 是 INV.pdf 的 usecase 面。
type pdfRunner interface {
	RunOnce(ctx context.Context) error
}

// pdfWorker 把 usecase 接上排程器。
type pdfWorker struct{ uc pdfRunner }

// RunOnce 由排程器每 60 秒呼叫一次。
func (w *pdfWorker) RunOnce(ctx context.Context) error { return w.uc.RunOnce(ctx) }
