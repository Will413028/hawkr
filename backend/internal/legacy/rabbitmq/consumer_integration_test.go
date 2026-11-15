//go:build integration

package rabbitmq

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

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

// recorder 記錄一個 queue 的 handler：成功處理過的訊息（以 body 區分，重送只算一次）、失敗次數與正在執行的數量。
type recorder struct {
	mu       sync.Mutex
	handled  map[string]bool
	failures int
	inFlight atomic.Int64
}

func (r *recorder) handle(ctx context.Context, body []byte) error {
	r.inFlight.Add(1)
	defer r.inFlight.Add(-1)
	select {
	case <-ctx.Done():
		r.mu.Lock()
		r.failures++
		r.mu.Unlock()
		return ctx.Err()
	case <-time.After(30 * time.Millisecond):
	}
	r.mu.Lock()
	r.handled[string(body)] = true
	r.mu.Unlock()
	return nil
}

func (r *recorder) counts() (handled, failures int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.handled), r.failures
}

// Run 在 ctx 取消後：處理中的訊息照樣處理完並 ack、返回時沒有 handler 還在執行、沒處理的訊息都留在 queue，且在有限
// 時間內返回。
func TestRunDrainsInFlightAndReturnsAfterCancel(t *testing.T) {
	url := startBroker(t)
	pub, err := New(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pub.Close() }()
	admin, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	adminCh, err := admin.Channel()
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	prefix := fmt.Sprintf("shutdown.%d", time.Now().UnixNano())

	const iterations, messages = 20, 50
	for i := range iterations {
		exchange := fmt.Sprintf("%s.ex.%d", prefix, i)
		queues := []string{fmt.Sprintf("%s.a.%d", prefix, i), fmt.Sprintf("%s.b.%d", prefix, i)}
		t.Cleanup(func() {
			for _, q := range queues {
				_, _ = adminCh.QueueDelete(q, false, false, false)
				_, _ = adminCh.QueueDelete(q+".dead", false, false, false)
			}
			_ = adminCh.ExchangeDelete(exchange, false, false)
		})
		recorders := []*recorder{{handled: map[string]bool{}}, {handled: map[string]bool{}}}
		bindings := []Binding{
			{Queue: queues[0], Exchange: exchange, Key: "k", Handle: recorders[0].handle},
			{Queue: queues[1], Exchange: exchange, Key: "k", Handle: recorders[1].handle},
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			Run(ctx, url, bindings, func(error) bool { return false }, log)
		}()
		waitFor(t, "兩個 queue 都有消費者", func() bool {
			for _, q := range queues {
				info, err := adminCh.QueueDeclarePassive(q, true, false, false, false, nil)
				if err != nil {
					// passive declare 找不到 queue 會關掉 channel，重開再試。
					if adminCh, err = admin.Channel(); err != nil {
						t.Fatal(err)
					}
					return false
				}
				if info.Consumers != 1 {
					return false
				}
			}
			return true
		})
		for n := range messages {
			if err := pub.Publish(context.Background(), exchange, "k", []byte(strconv.Itoa(n))); err != nil {
				cancel()
				t.Fatalf("送出：%v", err)
			}
		}
		waitFor(t, "訊息開始處理", func() bool {
			a, _ := recorders[0].counts()
			b, _ := recorders[1].counts()
			return a > 0 && b > 0
		})
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second): // 低於 closeTimeout，關閉卡住才會被抓到，不會被逾時上限遮住
			t.Fatalf("第 %d 次：ctx 取消後 Run 超過 3 秒沒有返回", i)
		}
		for qi, r := range recorders {
			if n := r.inFlight.Load(); n != 0 {
				t.Fatalf("第 %d 次：Run 返回時 %s 還有 %d 個 handler 在執行", i, queues[qi], n)
			}
			handled, failures := r.counts()
			if failures != 0 {
				t.Fatalf("第 %d 次：%s 處理中的訊息被關機取消了 %d 次", i, queues[qi], failures)
			}
			if handled == messages {
				t.Fatalf("第 %d 次：%s 取消前已處理完全部訊息，沒有打到處理中取消的情況", i, queues[qi])
			}
			// 未 ack 的預取訊息在連線關閉後才由 broker 放回 queue，輪詢等它到齊。
			waitFor(t, queues[qi]+" 的訊息守恆（已處理＋留在 queue＝送出）", func() bool {
				info, err := adminCh.QueueDeclarePassive(queues[qi], true, false, false, false, nil)
				return err == nil && handled+info.Messages == messages
			})
		}
	}
}
