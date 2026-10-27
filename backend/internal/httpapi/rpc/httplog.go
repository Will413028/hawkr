package rpc

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
)

type loggedKey struct{}

// markLogged 告訴 LogUnhandled 這個請求已經由 interceptor 記過 log。
func markLogged(ctx context.Context) {
	if done, ok := ctx.Value(loggedKey{}).(*atomic.Bool); ok {
		done.Store(true)
	}
}

// LogUnhandled 為 interceptor 沒有經手的錯誤回應補一筆 log（ADR 2026-09-25-api-error-model D5）：
// connect-go 在呼叫 interceptor 之前就回的錯誤（請求無法解碼、超過大小上限、不支援的 Content-Type、
// 不存在的方法）不會經過 interceptor。
func LogUnhandled(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done := new(atomic.Bool)
		sw := &statusWriter{ResponseWriter: w}
		ctx := context.WithValue(r.Context(), loggedKey{}, done)
		next.ServeHTTP(sw, r.WithContext(ctx))
		if done.Load() || sw.status < http.StatusBadRequest {
			return
		}
		level := slog.LevelInfo
		if sw.status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		log.Log(ctx, level, "rpc：interceptor 之前的錯誤",
			"procedure", r.URL.Path,
			"http_status", sw.status,
			"trace_id", TraceID(ctx))
	})
}

// statusWriter 記下回應的 HTTP status。串流要能 flush，所以實作 Flush 與 Unwrap。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
