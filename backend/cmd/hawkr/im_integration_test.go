//go:build integration

package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type imTestBuyers struct{}

func (imTestBuyers) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) {
	return 1000 + id, nil
}
func (imTestBuyers) ProfileByBuyerID(context.Context, int64) (buyer.Profile, error) {
	return buyer.Profile{}, nil
}
func (imTestBuyers) IsBlockingStore(context.Context, int64, int64) (bool, error) { return false, nil }

type imTestStores struct{}

func (imTestStores) Store(_ context.Context, id int64) (seller.Store, error) {
	return seller.Store{
		StoreID: id, AccountID: 2000 + id, Status: seller.Active, Sanctions: seller.NoSanctions(),
		Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true},
	}, nil
}
func (imTestStores) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }

type imNoOrders struct{}

func (imNoOrders) HasOrderBetween(context.Context, int64, int64) (bool, string, error) {
	return false, "", nil
}
func (imNoOrders) OrderParties(context.Context, string) (orderservice.OrderParties, error) {
	return orderservice.OrderParties{}, nil
}

// 刪帳時機（A6-D14 W4 (ii)）：newIM 只訂 BuyerDeleted。發 BuyerDeletionRequested 之後買家仍能送出、訊息不被抹除、沒有複本與抹除 job；
// BuyerDeleted 之後才寫複本、排抹除 job、擋送出，抹除 job 跑完才抹除。這是正式組裝（newIM）的測試：加訂 BuyerDeletionRequested 時它失敗。
func TestIMDeletionTimingSubscribesOnlyToBuyerDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "im", Dir: im.Migrations()}))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	m := newIM(d, bus, queue, imTestBuyers{}, imTestStores{}, media.NewFake(), imNoOrders{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	const buyerID, storeID = int64(11), int64(22)
	send := func(text string) error {
		_, err := m.Send(ctx, im.SendInput{Sender: im.Party{Kind: im.PartyBuyer, BuyerID: buyerID}, CounterpartID: storeID, Type: im.TypeText, Text: text})
		return err
	}
	count := func(sql string) int64 {
		var n int64
		if err := d.Conn(ctx).QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	const eraseJobs = "SELECT count(*) FROM river.river_job WHERE kind = 'im.erase_party'"
	if err := send("before"); err != nil {
		t.Fatal(err)
	}

	if err := d.InTx(ctx, func(ctx context.Context) error {
		return events.Publish(ctx, bus, buyer.BuyerDeletionRequested{EventID: uuid.NewV7(), OccurredAt: time.Now(), BuyerID: buyerID, AccountID: 1000 + buyerID})
	}); err != nil {
		t.Fatal(err)
	}
	if err := send("while pending deletion"); err != nil {
		t.Fatalf("等待刪除的買家仍能為開啟中的訂單聯絡店家：%v", err)
	}
	if n := count("SELECT count(*) FROM im.inactive_parties") + count(eraseJobs) + count("SELECT count(*) FROM im.messages WHERE erased_at IS NOT NULL"); n != 0 {
		t.Fatalf("BuyerDeletionRequested 不該寫複本、排 job 或抹除：%d", n)
	}

	if err := d.InTx(ctx, func(ctx context.Context) error {
		return events.Publish(ctx, bus, buyer.BuyerDeleted{EventID: uuid.NewV7(), OccurredAt: time.Now(), BuyerID: buyerID, AccountID: 1000 + buyerID})
	}); err != nil {
		t.Fatal(err)
	}
	if n := count("SELECT count(*) FROM im.inactive_parties WHERE kind = 'BUYER'"); n != 1 {
		t.Fatalf("BuyerDeleted 要寫複本：%d", n)
	}
	if n := count(eraseJobs); n != 1 {
		t.Fatalf("BuyerDeleted 要排 1 個抹除 job：%d", n)
	}
	if err := send("after"); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("BuyerDeleted 之後要擋送出：%v", err)
	}
	if n := count("SELECT count(*) FROM im.messages WHERE erased_at IS NOT NULL"); n != 0 {
		t.Fatalf("抹除由 job 做，訂閱者不抹：%d", n)
	}
	if err := m.ErasePartyMessages(ctx, im.PartyBuyer, buyerID); err != nil {
		t.Fatal(err)
	}
	if n := count("SELECT count(*) FROM im.messages WHERE erased_at IS NOT NULL"); n != 2 {
		t.Fatalf("job 跑完抹除 2 則：%d", n)
	}
}
