//go:build integration

package notification

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type nopReportingJobs struct{}

func (nopReportingJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }
func (nopReportingJobs) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

type dirNoRegistry struct{}

func (dirNoRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type dirNoProducts struct{}

func (dirNoProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type dirNoOrders struct{}

func (dirNoOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (dirNoOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

// 正式的 NewDirectory 把真的 buyer 與 seller 組成公告的名單：買家的排序鍵是 buyer_id、店家的是 store_id（不是帳號 ID），店家帶 store_id，
// 分頁不足一頁就是最後一頁。
// mutation：Buyers 的 Cursor 改成 AccountID，或 Stores 漏填 StoreID → 這個測試紅。
func TestNewDirectoryListsBuyersAndStoresWithTheirOwnCursors(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
	))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewBus()
	q := nopReportingJobs{}
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	b := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: time.Now, Logger: log})
	events.Subscribe(bus, b.OnAccountCreated)
	sl := seller.New(seller.Deps{
		KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: id, Media: media.NewFake(), Orders: dirNoOrders{}, ProductStores: dirNoProducts{}, Registry: dirNoRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: q, Now: time.Now, Logger: log}),
	})
	dir := NewDirectory(b, sl)
	ctx := context.Background()

	var accounts []int64
	for range 3 {
		a, err := id.DevNewAccount(ctx)
		if err != nil {
			t.Fatal(err)
		}
		accounts = append(accounts, a)
	}
	var wantBuyers []announce.Recipient
	for _, a := range accounts {
		bid, err := b.CurrentBuyerID(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		wantBuyers = append(wantBuyers, announce.Recipient{AccountID: a, Cursor: bid})
	}
	slices.SortFunc(wantBuyers, func(x, y announce.Recipient) int { return int(x.Cursor - y.Cursor) })

	first, err := dir.Buyers(ctx, 0, 2)
	if err != nil || !slices.Equal(first, wantBuyers[:2]) {
		t.Fatalf("第一頁買家 = %+v（%v），預期 %+v", first, err, wantBuyers[:2])
	}
	rest, err := dir.Buyers(ctx, first[1].Cursor, 2)
	if err != nil || !slices.Equal(rest, wantBuyers[2:]) {
		t.Fatalf("第二頁買家 = %+v（%v），預期 %+v", rest, err, wantBuyers[2:])
	}

	var wantStores []announce.Recipient
	for _, a := range accounts[:2] {
		s, err := sl.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: a, StoreName: "測試小店"})
		if err != nil {
			t.Fatal(err)
		}
		wantStores = append(wantStores, announce.Recipient{AccountID: a, StoreID: s.StoreID, Cursor: s.StoreID})
	}
	slices.SortFunc(wantStores, func(x, y announce.Recipient) int { return int(x.Cursor - y.Cursor) })
	stores, err := dir.Stores(ctx, 0, 5)
	if err != nil || !slices.Equal(stores, wantStores) {
		t.Fatalf("店家 = %+v（%v），預期 %+v", stores, err, wantStores)
	}
	if after, err := dir.Stores(ctx, wantStores[0].Cursor, 5); err != nil || !slices.Equal(after, wantStores[1:]) {
		t.Fatalf("第一間之後的店家 = %+v（%v），預期 %+v", after, err, wantStores[1:])
	}
}
