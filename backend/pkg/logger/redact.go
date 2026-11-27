package logger

import (
	"errors"
	"regexp"
	"strings"

	"go.uber.org/zap/zapcore"
)

var sensitiveQueryParamPattern = regexp.MustCompile(
	`(?i)(key|password|passwd|token|apikey|api_key|secret|credential)=[^&\s"]+`,
)

var urlUserinfoPattern = regexp.MustCompile(`://[^/\s@]+:[^/\s@]+@`)

func redactSecrets(s string) string {
	if !strings.ContainsAny(s, "=@") {
		return s
	}
	s = sensitiveQueryParamPattern.ReplaceAllString(s, "${1}=REDACTED")
	s = urlUserinfoPattern.ReplaceAllString(s, "://REDACTED@")
	return s
}

type redactingCore struct {
	zapcore.Core
}

func newRedactingCore(core zapcore.Core) zapcore.Core {
	return &redactingCore{Core: core}
}

func (c *redactingCore) With(fields []zapcore.Field) zapcore.Core {
	return &redactingCore{Core: c.Core.With(redactFields(fields))}
}

// Check 一定要覆寫。zapcore.Core 是 interface embedding，若不覆寫，
// promoted 方法會呼叫底層 core 自己的 Check，而底層實作內部一律
// 用自己的 receiver 呼叫 ce.AddCore(entry, <底層 core 本身>)，等於把
// 底層 core 直接註冊成 entry writer，完全繞過這層的 Write，遮蔽形同虛設。
func (c *redactingCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *redactingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = redactSecrets(entry.Message)
	return c.Core.Write(entry, redactFields(fields))
}

func redactFields(fields []zapcore.Field) []zapcore.Field {
	out := make([]zapcore.Field, len(fields))
	for i, f := range fields {
		out[i] = redactField(f)
	}
	return out
}

func redactField(f zapcore.Field) zapcore.Field {
	switch f.Type {
	case zapcore.StringType:
		f.String = redactSecrets(f.String)
		return f
	case zapcore.ErrorType:
		if err, ok := f.Interface.(error); ok && err != nil {
			if redacted := redactSecrets(err.Error()); redacted != err.Error() {
				f.Interface = errors.New(redacted)
			}
		}
		return f
	default:
		return f
	}
}
