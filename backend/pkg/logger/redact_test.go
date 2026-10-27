package logger

import (
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRedactSecrets_QueryStringNamedKeys(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase key param",
			input: `Get "https://generativelanguage.googleapis.com/v1/models?key=AIzaSyABCDEF123456": dial tcp: i/o timeout`, // gitleaks:allow（遮罩測試的假 key）
			want:  `Get "https://generativelanguage.googleapis.com/v1/models?key=REDACTED": dial tcp: i/o timeout`,
		},
		{
			name:  "mitake-style multiple params, case-insensitive Password",
			input: `mitake SmSend request: Get "https://smsapi.mitake.com.tw/api/mtk/SmSend?username=acct01&Password=hunter2&dstaddr=0912345678&smbody=your+code+is+123456": context deadline exceeded`,
			want:  `mitake SmSend request: Get "https://smsapi.mitake.com.tw/api/mtk/SmSend?username=acct01&Password=REDACTED&dstaddr=0912345678&smbody=your+code+is+123456": context deadline exceeded`,
		},
		{
			name:  "api_key param",
			input: `parse "https://data.moa.gov.tw/api/v1/CASProductInquiryType/?Emblem_ID=<bad>&$top=1&api_key=TESTFAKEAPIKEY0000000000000000": invalid character "<" in query`,
			want:  `parse "https://data.moa.gov.tw/api/v1/CASProductInquiryType/?Emblem_ID=<bad>&$top=1&api_key=REDACTED": invalid character "<" in query`,
		},
		{
			name:  "no secret-shaped content passes through unchanged",
			input: `dial tcp 10.0.0.1:5432: connect: connection refused`,
			want:  `dial tcp 10.0.0.1:5432: connect: connection refused`,
		},
		{
			name:  "non-secret query param untouched",
			input: `https://example.com/search?q=hello&page=2`,
			want:  `https://example.com/search?q=hello&page=2`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSecrets(tc.input)
			if got != tc.want {
				t.Errorf("redactSecrets(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRedactSecrets_URLUserinfo(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "amqp userinfo",
			input: `dial tcp: amqp://guest:REDACTED@rabbitmq.hawkr.svc:5672/: connection refused`,
			want:  `dial tcp: amqp://REDACTED@rabbitmq.hawkr.svc:5672/: connection refused`,
		},
		{
			name:  "postgres userinfo",
			input: `failed to connect: postgres://dbadmin:REDACTED@10.0.0.5:5432/hawkr_account`,
			want:  `failed to connect: postgres://REDACTED@10.0.0.5:5432/hawkr_account`,
		},
		{
			name:  "no userinfo, host only, untouched",
			input: `https://data.moa.gov.tw/api/v1/`,
			want:  `https://data.moa.gov.tw/api/v1/`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactSecrets(tc.input)
			if got != tc.want {
				t.Errorf("redactSecrets(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRedactingCore_ErrorFieldRedacted(t *testing.T) {
	observedCore, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(newRedactingCore(observedCore))

	leaky := errors.New(`mitake SmSend request: Get "https://smsapi.mitake.com.tw/api/mtk/SmSend?username=acct01&password=hunter2": timeout`)
	logger.Warn("OTP SMS send failed", zap.Error(leaky))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	ctxMap := entries[0].ContextMap()
	errVal, ok := ctxMap["error"].(string)
	if !ok {
		t.Fatalf("expected error field to be a string in encoded output, got %T: %v", ctxMap["error"], ctxMap["error"])
	}
	if strings.Contains(errVal, "hunter2") {
		t.Errorf("error field leaks password: %s", errVal)
	}
	if !strings.Contains(errVal, "password=REDACTED") {
		t.Errorf("expected redacted marker in error field, got: %s", errVal)
	}
}

func TestRedactingCore_StringFieldRedacted(t *testing.T) {
	observedCore, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(newRedactingCore(observedCore))

	logger.Warn("notification mq publisher init failed",
		zap.String("url", "amqp://guest:REDACTED@rabbitmq.hawkr.svc:5672/"))

	entries := logs.All()
	ctxMap := entries[0].ContextMap()
	urlVal, _ := ctxMap["url"].(string)
	if strings.Contains(urlVal, "MyRealPass123") {
		t.Errorf("url field leaks password: %s", urlVal)
	}
}

func TestRedactingCore_MessageItselfRedacted(t *testing.T) {
	observedCore, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(newRedactingCore(observedCore))

	// 用 URL userinfo 帳密形狀驗證 entry.Message 本身也會走 redactSecrets——
	// 這條路徑跟 zap.Error/zap.String 欄位是不同程式碼路徑（Write 裡直接改
	// entry.Message，不經過 redactField）。原本這裡放的是 Discord webhook
	// 範例，但 Discord 的 token 在 URL path 裡（/api/webhooks/{id}/{token}），
	// 不是 query string 命名密鑰或 userinfo 帳密，不在本 task 宣告的兩種遮蔽
	// 形狀內；Task 4 對 Discord webhook 是用完全不同的手法（丟棄整個 URL，見
	// docs/superpowers/plans/2026-08-22-credential-leak-sweep.md 第 604 行），
	// 不靠這裡的正則。換成本 task 實際涵蓋的形狀，語意不變。
	logger.Warn(`mq publish failed: dial tcp: amqp://guest:REDACTED@rabbitmq.hawkr.svc:5672/: connection refused`)

	entries := logs.All()
	if strings.Contains(entries[0].Message, "MyRealPass123") {
		t.Errorf("log message leaks password: %s", entries[0].Message)
	}
}

func TestRedactingCore_NonSecretFieldsUntouched(t *testing.T) {
	observedCore, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(newRedactingCore(observedCore))

	logger.Info("order created",
		zap.String("order_no", "HK20260822001"),
		zap.Int64("user_id", 123456789))

	entries := logs.All()
	ctxMap := entries[0].ContextMap()
	if ctxMap["order_no"] != "HK20260822001" {
		t.Errorf("non-secret string field was altered: %v", ctxMap["order_no"])
	}
}

func TestRedactingCore_WithPreservesRedaction(t *testing.T) {
	observedCore, logs := observer.New(zapcore.DebugLevel)
	base := zap.New(newRedactingCore(observedCore))
	child := base.With(zap.String("component", "sms"))

	child.Warn("send failed", zap.String("url", "amqp://guest:REDACTED@host/"))

	entries := logs.All()
	ctxMap := entries[0].ContextMap()
	urlVal, _ := ctxMap["url"].(string)
	if strings.Contains(urlVal, "MyRealPass123") {
		t.Errorf(".With() 之後的 logger 沒有繼續套用遮蔽: %s", urlVal)
	}
}
