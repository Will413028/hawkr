//go:build integration

package rabbitmq

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startBroker 回傳 broker 網址：有設 HAWKR_TEST_AMQP_URL 就用既有的 broker，否則起一個容器。
func startBroker(t *testing.T) string {
	t.Helper()
	if url := os.Getenv("HAWKR_TEST_AMQP_URL"); url != "" {
		return url
	}
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "rabbitmq:3.13-alpine",
			ExposedPorts: []string{"5672/tcp"},
			WaitingFor:   wait.ForLog("Server startup complete").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("啟動 RabbitMQ：%v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := c.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())
}

// waitFor 每 20ms 檢查一次 cond，最多等 10 秒。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("等不到%s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Run 在 ctx 取消後必須在有限時間內返回，取消時消費者還在處理訊息也一樣。
func TestRunReturnsPromptlyAfterCancel(t *testing.T) {
	url := startBroker(t)
	pub, err := New(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pub.Close() }()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	const iterations = 20
	for i := range iterations {
		var handled atomic.Int64
		handle := func(context.Context, []byte) error {
			time.Sleep(20 * time.Millisecond)
			handled.Add(1)
			return nil
		}
		exchange := fmt.Sprintf("shutdown.ex.%d", i)
		bindings := []Binding{
			{Queue: fmt.Sprintf("shutdown.a.%d", i), Exchange: exchange, Key: "k", Handle: handle},
			{Queue: fmt.Sprintf("shutdown.b.%d", i), Exchange: exchange, Key: "k", Handle: handle},
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			Run(ctx, url, bindings, func(error) bool { return false }, log)
		}()
		// Exchange 由 Run 宣告；第一則送得出去且被處理，代表兩個 queue 都已綁好、消費者在跑。
		waitFor(t, "消費者就緒", func() bool {
			return pub.Publish(context.Background(), exchange, "k", []byte(`{}`)) == nil && func() bool {
				deadline := time.Now().Add(500 * time.Millisecond)
				for handled.Load() == 0 && time.Now().Before(deadline) {
					time.Sleep(10 * time.Millisecond)
				}
				return handled.Load() > 0
			}()
		})
		for range 50 {
			if err := pub.Publish(context.Background(), exchange, "k", []byte(`{}`)); err != nil {
				cancel()
				t.Fatalf("送出：%v", err)
			}
		}
		before := handled.Load()
		waitFor(t, "訊息開始處理", func() bool { return handled.Load() > before })
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second): // 低於 closeTimeout，關閉卡住才會被抓到，不會被逾時上限遮住
			t.Fatalf("第 %d 次：ctx 取消後 Run 超過 3 秒沒有返回", i)
		}
		if n := handled.Load(); n >= 102 {
			t.Fatalf("第 %d 次：取消時已處理完全部 %d 則，沒有打到處理中取消的情況", i, n)
		}
	}
}
