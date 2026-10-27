//go:build integration

package events_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type orderPaid struct{ OrderID int }

var errBoom = errors.New("boom")

func setup(t *testing.T) *db.DB {
	t.Helper()
	d := pg.NewDB(t)
	if _, err := d.Conn(context.Background()).Exec(context.Background(), "CREATE TABLE log (who text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	return d
}

func write(ctx context.Context, d *db.DB, who string) error {
	_, err := d.Conn(ctx).Exec(ctx, "INSERT INTO log (who) VALUES ($1)", who)
	return err
}

func rows(t *testing.T, d *db.DB) []string {
	t.Helper()
	r, err := d.Conn(context.Background()).Query(context.Background(), "SELECT who FROM log ORDER BY who")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var out []string
	for r.Next() {
		var s string
		if err := r.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestHandlerWritesCommitWithThePublisher(t *testing.T) {
	t.Parallel()
	d := setup(t)
	b := events.NewBus()
	events.Subscribe(b, func(ctx context.Context, e orderPaid) error { return write(ctx, d, "handler") })

	err := d.InTx(context.Background(), func(ctx context.Context) error {
		if err := write(ctx, d, "publisher"); err != nil {
			return err
		}
		return events.Publish(ctx, b, orderPaid{OrderID: 1})
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rows(t, d); !slices.Equal(got, []string{"handler", "publisher"}) {
		t.Fatalf("rows = %v", got)
	}
}

func TestHandlerErrorRollsBackThePublisher(t *testing.T) {
	t.Parallel()
	d := setup(t)
	b := events.NewBus()
	events.Subscribe(b, func(ctx context.Context, e orderPaid) error { return errBoom })

	err := d.InTx(context.Background(), func(ctx context.Context) error {
		if err := write(ctx, d, "publisher"); err != nil {
			return err
		}
		return events.Publish(ctx, b, orderPaid{OrderID: 1})
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v，預期 handler 的錯誤", err)
	}
	if got := rows(t, d); len(got) != 0 {
		t.Fatalf("rows = %v，handler 失敗時發布方的寫入也要 rollback", got)
	}
}

func TestHandlersRunInSubscriptionOrderAndStopAtFirstError(t *testing.T) {
	t.Parallel()
	d := setup(t)
	b := events.NewBus()
	var order []int
	events.Subscribe(b, func(context.Context, orderPaid) error { order = append(order, 1); return nil })
	events.Subscribe(b, func(context.Context, orderPaid) error { order = append(order, 2); return errBoom })
	events.Subscribe(b, func(context.Context, orderPaid) error { order = append(order, 3); return nil })

	_ = d.InTx(context.Background(), func(ctx context.Context) error {
		return events.Publish(ctx, b, orderPaid{OrderID: 1})
	})
	if !slices.Equal(order, []int{1, 2}) {
		t.Fatalf("執行順序 = %v，預期 [1 2]（第二個失敗後停止）", order)
	}
}
