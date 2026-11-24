//go:build integration

package paymentrelay

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func publishRows(t *testing.T, ctx context.Context, q func(string, ...any) ([][2]string, error)) [][2]string {
	t.Helper()
	rows, err := q(`SELECT args->>'exchange', args->>'routing_key' FROM river.river_job WHERE kind = 'legacy.rabbitmq_publish' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// 五種轉送的事件各排入一個送出的 job，exchange 與 routing key 與 events.csv 登記的相同；事件與 job 在同一個 transaction：發布的寫入回滾時 job 一起回滾。
func TestEachRelayedEventQueuesOnePublishJobInTheCallersTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	query := func(sql string, args ...any) ([][2]string, error) {
		rows, err := d.Pool().Query(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out [][2]string
		for rows.Next() {
			var a, b string
			if err := rows.Scan(&a, &b); err != nil {
				return nil, err
			}
			out = append(out, [2]string{a, b})
		}
		return out, rows.Err()
	}
	paid := occurred
	rolledBack := errors.New("rollback")
	err = d.InTx(ctx, func(ctx context.Context) error {
		for _, e := range []payment.PaymentChanged{
			{OccurredAt: occurred, PaymentID: uuid.NewV7(), OrderNo: "O1", BuyerID: 1, Amount: 10, Status: payment.PaymentSucceeded, PaidAt: &paid},
			{OccurredAt: occurred, PaymentID: uuid.NewV7(), OrderNo: "O2", BuyerID: 1, Amount: 10, Status: payment.PaymentCreateFailed},
			{OccurredAt: occurred, PaymentID: uuid.NewV7(), OrderNo: "O3", BuyerID: 1, Amount: 10, Status: payment.PaymentCanceled},
		} {
			if err := OnPaymentChanged(queue)(ctx, e); err != nil {
				return err
			}
		}
		for _, e := range []payment.RefundChanged{
			{OccurredAt: occurred, RefundID: uuid.NewV7(), OrderNo: "O1", RequestKey: "k1", Amount: 5, Status: payment.RefundSucceeded},
			{OccurredAt: occurred, RefundID: uuid.NewV7(), OrderNo: "O1", RequestKey: "k2", Amount: 5, Status: payment.RefundFailed},
		} {
			if err := OnRefundChanged(queue)(ctx, e); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got := publishRows(t, ctx, query)
	want := [][2]string{
		{"payment.events", "payment.success"}, {"payment.events", "payment.failed"}, {"payment.events", "payment.closed"},
		{"payment.events", "refund.succeeded"}, {"payment.events", "refund.failed"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("publish jobs = %v, want %v", got, want)
	}

	err = d.InTx(ctx, func(ctx context.Context) error {
		if err := OnPaymentChanged(queue)(ctx, payment.PaymentChanged{OccurredAt: occurred, PaymentID: uuid.NewV7(), OrderNo: "O9", BuyerID: 1, Amount: 10, Status: payment.PaymentCanceled}); err != nil {
			return err
		}
		return rolledBack
	})
	if !errors.Is(err, rolledBack) {
		t.Fatal(err)
	}
	if after := publishRows(t, ctx, query); len(after) != len(want) {
		t.Fatalf("a rolled-back event left %d publish jobs, want %d", len(after), len(want))
	}
}
