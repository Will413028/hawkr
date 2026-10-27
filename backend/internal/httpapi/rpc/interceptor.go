package rpc

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// 單一請求的處理時間。client 以 Connect-Timeout-Ms 帶自己的 deadline；沒帶時用預設值，
// 超過上限時以上限計，避免沒有 deadline 的請求一直佔著資料庫連線。只套用在 unary：串流的長度
// 由各方法自己決定。
const (
	defaultTimeout = 15 * time.Second
	maxTimeout     = 60 * time.Second
)

// NewInterceptor 回傳每個 Connect handler 都要掛的 interceptor。一個 interceptor 依序做完：
// 授權（依 service 的使用者類別；呼叫者由入口的認證中介層放進 ctx）→ 逾時 → 呼叫 handler → 錯誤轉換 →
// log，所以 log 看得到最後回給 client 的錯誤碼，授權失敗也會被記錄。
func NewInterceptor(log *slog.Logger) connect.Interceptor {
	return &interceptor{log: log}
}

type interceptor struct {
	log *slog.Logger
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		procedure := req.Spec().Procedure
		ctx, cancel := bounded(ctx)
		defer cancel()
		err := authorize(ctx, procedure)
		var resp connect.AnyResponse
		if err == nil {
			resp, err = next(ctx, req)
		}
		return resp, i.finish(ctx, procedure, start, err)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		procedure := conn.Spec().Procedure
		err := authorize(ctx, procedure)
		if err == nil {
			err = next(ctx, conn)
		}
		return i.finish(ctx, procedure, start, err)
	}
}

// finish 把錯誤轉成 Connect 的錯誤，並為每個請求記一筆 log。沒有分類的錯誤只有 log 看得到原因。
func (i *interceptor) finish(ctx context.Context, procedure string, start time.Time, err error) error {
	markLogged(ctx)
	c := toConnect(ctx, err)
	ce := c.err
	attrs := []any{
		"procedure", procedure,
		"duration_ms", time.Since(start).Milliseconds(),
		"trace_id", TraceID(ctx),
	}
	if p, ok := PrincipalFrom(ctx); ok {
		attrs = append(attrs, "principal", principalLabel(p))
	}
	if len(c.unmapped) > 0 {
		i.log.ErrorContext(ctx, "rpc：欄位沒有附給 client：欄位沒有對應到請求的路徑（handler 要以 rpc.WithFieldPaths 補上）", append(attrs, "fields", c.unmapped)...)
	}
	switch {
	case ce == nil:
		i.log.InfoContext(ctx, "rpc", append(attrs, "code", "ok")...)
		return nil
	case c.internal:
		i.log.ErrorContext(ctx, "rpc", append(attrs, "code", ce.Code().String(), "error", err)...)
	default:
		i.log.InfoContext(ctx, "rpc", append(attrs, "code", ce.Code().String(), "error", err)...)
	}
	return ce
}

// bounded 在 client 沒帶 deadline 時套用預設值，帶了但超過上限時縮到上限。
func bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	limit := defaultTimeout
	if d, ok := ctx.Deadline(); ok {
		if time.Until(d) <= maxTimeout {
			return context.WithCancel(ctx)
		}
		limit = maxTimeout
	}
	return context.WithTimeout(ctx, limit)
}

// principalLabel 是給 log 的呼叫者，例如 account:7、admin:3（兩者的 ID 各自發號，要連同類型才能識別）。
func principalLabel(p identity.Principal) string {
	if p.Kind == identity.PrincipalAdmin {
		return "admin:" + strconv.FormatInt(p.ID, 10)
	}
	return "account:" + strconv.FormatInt(p.ID, 10)
}
