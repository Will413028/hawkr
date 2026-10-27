package client

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"go.opentelemetry.io/otel/trace"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// #3031：correlation_id 是 ProviderExchangeItem.request_id 與
// SellerProviderAccountData.last_request_id 兩個對外欄位的**唯一**來源。
//
// 🔴 dev 實測發現它從來沒有被填過（2397 列全 NULL）⇒ 那兩欄接上之後仍然恆為空。
// 「有賦值但值恆為空」是 producer gate 第一種擋不住的形態，這一支測試補那個缺口。
func TestCorrelationIDOf(t *testing.T) {
	t.Run("gRPC 請求路徑：取 interceptor 種在 ctx 的值", func(t *testing.T) {
		ctx := interceptor.WithCorrelationID(context.Background(), "corr-from-request")
		if got := correlationIDOf(ctx); got != "corr-from-request" {
			t.Fatalf("got %q, want corr-from-request", got)
		}
	})

	t.Run("排程 worker 路徑：沒有請求可種 ⇒ 退回 trace id", func(t *testing.T) {
		tid, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		if err != nil {
			t.Fatal(err)
		}
		ctx := trace.ContextWithSpanContext(context.Background(),
			trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}}))
		if got := correlationIDOf(ctx); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Fatalf("got %q, want the trace id", got)
		}
	})

	t.Run("兩者都沒有 ⇒ 空字串（欄位保持 NULL，不塞編出來的值）", func(t *testing.T) {
		if got := correlationIDOf(context.Background()); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	// 🔴 優先序：有請求 id 時**不得**被 trace id 蓋掉——請求 id 才是跨服務追同一次
	// 使用者操作的那個值，trace id 只是這一跳的。
	t.Run("兩者都有時取請求 id", func(t *testing.T) {
		tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		ctx := trace.ContextWithSpanContext(
			interceptor.WithCorrelationID(context.Background(), "corr-wins"),
			trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: trace.SpanID{1}}))
		if got := correlationIDOf(ctx); got != "corr-wins" {
			t.Fatalf("got %q, want corr-wins", got)
		}
	})
}

type capturingAuditRepo struct {
	domain.AuditRepository
	got []domain.NewProviderAuditLogEntry
}

func (c *capturingAuditRepo) AppendProviderAuditLog(_ context.Context, in domain.NewProviderAuditLogEntry) error {
	c.got = append(c.got, in)
	return nil
}

// 🔴 上一支測的是 correlationIDOf 這個純函式；**這一支測的是它有沒有真的被接上**。
//
// 兩者缺一都會讓「拿掉 entry.CorrelationID 的賦值」這個變異溜過去——而那正是
// dev 上實際存在了 2397 列的形態（欄位存在、程式碼看起來完整、值永遠是 NULL）。
func TestRepoAuditSink_RecordCarriesCorrelationID(t *testing.T) {
	repo := &capturingAuditRepo{}
	sink := &repoAuditSink{repo: repo}

	ctx := interceptor.WithCorrelationID(context.Background(), "corr-xyz")
	sink.Record(ctx, amego.Exchange{
		IssuerID: 1, Endpoint: "/json/f0401", AttemptSeq: 1,
		RequestMasked: map[string]string{}, ResponseMasked: "{}", OccurredAt: time.Unix(1000, 0).UTC(),
	})

	if len(repo.got) != 1 {
		t.Fatalf("應落一列，實得 %d", len(repo.got))
	}
	e := repo.got[0]
	if e.CorrelationID == nil || *e.CorrelationID != "corr-xyz" {
		t.Fatalf("🔴 entry 沒帶上 correlation_id：%v —— request_id／last_request_id 兩個對外欄位會恆為空", e.CorrelationID)
	}

	// 沒有來源時保持 NULL，不塞編出來的值。
	repo.got = nil
	sink.Record(context.Background(), amego.Exchange{
		IssuerID: 1, Endpoint: "/json/f0401", AttemptSeq: 2,
		RequestMasked: map[string]string{}, ResponseMasked: "{}", OccurredAt: time.Unix(1000, 0).UTC(),
	})
	if repo.got[0].CorrelationID != nil {
		t.Fatalf("沒有來源時應為 NULL，實得 %q", *repo.got[0].CorrelationID)
	}
}
