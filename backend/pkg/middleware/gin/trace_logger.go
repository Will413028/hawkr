package ginmw

import (
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TraceLogger 是 Gin middleware，將 OTel trace_id / span_id 注入 context 中的 Zap logger。
// 必須放在上游 tracing middleware 之後，確保 span 已建立。
func TraceLogger(base *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var fields []zap.Field
		spanCtx := trace.SpanContextFromContext(ctx)
		if spanCtx.HasTraceID() {
			fields = append(fields, zap.String("trace_id", spanCtx.TraceID().String()))
		}
		if spanCtx.HasSpanID() {
			fields = append(fields, zap.String("span_id", spanCtx.SpanID().String()))
		}

		if len(fields) > 0 {
			enriched := base.With(fields...)
			ctx = pkglogger.WithLogger(ctx, enriched)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}
