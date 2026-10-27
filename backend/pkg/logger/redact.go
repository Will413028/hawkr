package logger

import (
	"errors"
	"regexp"
	"strings"

	"go.uber.org/zap/zapcore"
)

// sensitiveQueryParamPattern 比對 URL query string 裡常見的密鑰參數名稱，
// 不分大小寫。命中的參數值一律換成 REDACTED，參數名本身保留（方便看出
// 這裡原本有一把什麼種類的密鑰，但值不外流）。
var sensitiveQueryParamPattern = regexp.MustCompile(
	`(?i)(key|password|passwd|token|apikey|api_key|secret|credential)=[^&\s"]+`,
)

// urlUserinfoPattern 比對 scheme://user:pass@host 形式的 URL userinfo 帳密。
// Go 標準庫 net/http 的 stripPassword 只對 *url.Error 生效，且只遮 userinfo；
// 這裡把同一種形狀套用到任意字串，涵蓋直接用 zap.String 塞連線字串的呼叫點
// （stripPassword 完全不會經過那種欄位）。
var urlUserinfoPattern = regexp.MustCompile(`://[^/\s@]+:[^/\s@]+@`)

// redactSecrets 掃描一段文字，把看起來像 URL query string 命名密鑰、或
// URL userinfo 帳密的片段換成 REDACTED。這是 pkg/logger 對所有輸出欄位的
// 最後一道防線——個別呼叫點仍應優先避免把憑證放進會被記錄的字串，這層只
// 攔截漏網的。快速路徑：字串裡沒有 '=' 也沒有 '@' 時，兩個 pattern 都不可能
// 命中，直接跳過正則掃描。
//
// 這層跟 gateway/internal/middleware/logger.go 的 access-log body/query
// 遮罩是互補、刻意不合併的兩層：這裡不知道任何欄位名稱，只能對任意
// service 塞進 zap.String 的自由文字比對「長得像密鑰」的 pattern；那邊看
// 得到已知 JSON schema 的欄位名，用 default-deny allowlist 做更嚴格的判定。
// 新增敏感欄位時，屬於某個已知 HTTP DTO 才進那邊的清單，跨 service 的通用
// 密鑰命名慣例才進這裡——不要把兩邊合併成一份清單。
func redactSecrets(s string) string {
	if !strings.ContainsAny(s, "=@") {
		return s
	}
	s = sensitiveQueryParamPattern.ReplaceAllString(s, "${1}=REDACTED")
	s = urlUserinfoPattern.ReplaceAllString(s, "://REDACTED@")
	return s
}

// redactingCore 包一層 zapcore.Core，在把 entry／fields 交給底層 core 之前
// 先用 redactSecrets 掃過 Message 與每個 String／Error 型別欄位的內容。
// 其他欄位型別原樣通過——見本檔開頭的範圍聲明。
type redactingCore struct {
	zapcore.Core
}

// newRedactingCore 包裝一個既有 core。掛法：zap.WrapCore(newRedactingCore)
// 當 cfg.Build 的其中一個 option。
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
