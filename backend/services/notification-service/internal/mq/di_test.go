package mq

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/config"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestNewPublisher_ConnectionFailureDoesNotLogRawURL 證明連線失敗時的 warn log
// 不會帶出含明文密碼的 RabbitMQ 連線字串。127.0.0.1:1 是保留 port，連線會快速被拒絕
// （connection refused），不需等待真實逾時；newPublisher 在這個失敗路徑上不會呼叫
// lc.Append，因此這裡用 fxtest.NewLifecycle（pkg/server/grpc_test.go 已有的同 repo
// 先例）即可，不需要真的 Start/Stop 它。
func TestNewPublisher_ConnectionFailureDoesNotLogRawURL(t *testing.T) {
	observedCore, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(observedCore)

	cfg := &config.Config{
		RabbitMQ: config.RabbitMQConfig{
			URL: "amqp://realuser:REDACTED@127.0.0.1:1/nonexistent-vhost-forces-fast-fail",
		},
	}

	lc := fxtest.NewLifecycle(t)

	p := newPublisher(lc, cfg, logger)
	if p != nil {
		t.Fatal("expected nil publisher when RabbitMQ connection fails against an unroutable address")
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	for _, f := range entries[0].Context {
		if f.Key == "url" {
			t.Fatalf("url field must not be logged, found: %+v", f)
		}
	}
	// zap.Error(err) 本身留著沒關係，斷言它不含明文密碼即可（防止底層行為改變的迴歸）。
	full := entries[0].ContextMap()
	if errStr, ok := full["error"].(string); ok && strings.Contains(errStr, "realpass123") {
		t.Errorf("zap.Error(err) field unexpectedly leaks password: %s", errStr)
	}
}
