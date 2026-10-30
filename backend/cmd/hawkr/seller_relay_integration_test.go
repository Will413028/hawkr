//go:build integration

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type sellerRelayFixture struct {
	d       *db.DB
	sellers *seller.Module
	account int64
	store   seller.Store
	// now 是測試的時鐘（改它就能讓關店滿保留期）。
	now *time.Time
}

func newSellerRelayFixture(t *testing.T) *sellerRelayFixture {
	t.Helper()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()}))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	accounts := identity.New(identity.Deps{DB: d, Events: bus, Jobs: queue, Now: clock, Logger: logger})
	kycKeys, kycMAC := sealtest.Keys(t)
	sellers := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: accounts, Media: noMedia{}, Orders: noOrders{}, ProductStores: &productStoresStub{}, Registry: noRegistry{},
		KYCKeys: kycKeys, KYCMAC: kycMAC, Now: clock, Logger: logger,
	})
	subscribeSellerRelay(bus, queue)
	var account int64
	if err := d.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&account); err != nil {
		t.Fatal(err)
	}
	store, err := sellers.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: account, StoreName: "範例小農", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號"})
	if err != nil {
		t.Fatal(err)
	}
	return &sellerRelayFixture{d: d, sellers: sellers, account: account, store: store, now: &now}
}

func (f *sellerRelayFixture) relayJobs(t *testing.T, kind string) []map[string]any {
	t.Helper()
	ctx := context.Background()
	rows, err := f.d.Pool().Query(ctx, "SELECT args FROM river.river_job WHERE kind = $1 ORDER BY id", kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var args map[string]any
		if err := rows.Scan(&args); err != nil {
			t.Fatal(err)
		}
		out = append(out, args)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// 關店可逆：StoreClosed 不轉成 seller.deleted。不可逆的點是個資清理（保留期滿）發的 StoreRetired：它在清理的 transaction 內轉成 relay job，
// 帶模組事件的穩定 ID、store_id 與 account_id。
func TestStoreClosedIsNotRelayedAndStoreRetiredIsRelayedAtThePurge(t *testing.T) {
	ctx := context.Background()
	f := newSellerRelayFixture(t)
	if _, err := f.sellers.CloseCurrent(ctx, f.account); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"legacy.seller_store_closed", "legacy.seller_store_retired"} {
		if got := f.relayJobs(t, kind); len(got) != 0 {
			t.Fatalf("%s 在關店時排入了 %v：seller.deleted 只在個資清理時發", kind, got)
		}
	}
	if n, err := f.sellers.PurgePII(ctx); err != nil || n != 0 {
		t.Fatalf("保留期內清理了 %d 家（err=%v）", n, err)
	}
	*f.now = f.now.Add(seller.StoreRetention + time.Second)
	if n, err := f.sellers.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("保留期滿清理了 %d 家（err=%v）", n, err)
	}
	got := f.relayJobs(t, "legacy.seller_store_retired")
	if len(got) != 1 {
		t.Fatalf("relay jobs=%v", got)
	}
	if got[0]["store_id"] != float64(f.store.StoreID) || got[0]["account_id"] != float64(f.account) {
		t.Fatalf("args=%v, want store %d account %d", got[0], f.store.StoreID, f.account)
	}
	if id, _ := got[0]["event_id"].(string); id == "" || id[14] != '7' {
		t.Fatalf("event_id=%v, want the module event's UUIDv7", got[0]["event_id"])
	}
}

// 發布的 transaction 回滾時 relay job 一起回滾（不會有「下游收到了、狀態卻沒寫」）：清理在外層 transaction 內執行後回滾，沒有 relay job，店家也沒被清理。
func TestRelayJobRollsBackWithTheWrite(t *testing.T) {
	ctx := context.Background()
	f := newSellerRelayFixture(t)
	if _, err := f.sellers.CloseCurrent(ctx, f.account); err != nil {
		t.Fatal(err)
	}
	*f.now = f.now.Add(seller.StoreRetention + time.Second)
	boom := errors.New("later step failed")
	err := f.d.InTx(ctx, func(ctx context.Context) error {
		if n, err := f.sellers.PurgePII(ctx); err != nil || n != 1 {
			t.Errorf("清理了 %d 家（err=%v）", n, err)
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if got := f.relayJobs(t, "legacy.seller_store_retired"); len(got) != 0 {
		t.Fatalf("a rolled back purge left relay jobs: %v", got)
	}
	if s, err := f.sellers.Store(ctx, f.store.StoreID); err != nil || s.StoreName == "" {
		t.Fatalf("store=%+v err=%v", s, err)
	}
}

// 可曝光的變化也從對帳 job 發出：直接把公告改掉（模擬漏接的寫入），跑一次對帳就排入 seller.exposure.changed 的 relay job。
func TestReconcileExposureAnnouncementIsRelayed(t *testing.T) {
	ctx := context.Background()
	f := newSellerRelayFixture(t)
	if _, err := f.sellers.ReconcileExposure(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.relayJobs(t, "legacy.seller_store_exposure_changed"); len(got) != 0 {
		t.Fatalf("a store that was never exposable announced %v", got)
	}
	if _, err := f.d.Conn(ctx).Exec(ctx, "UPDATE seller.store_exposures SET exposable=true, version=1, exposable_changed_at=now() WHERE store_id=$1", f.store.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sellers.ReconcileExposure(ctx); err != nil {
		t.Fatal(err)
	}
	got := f.relayJobs(t, "legacy.seller_store_exposure_changed")
	if len(got) != 1 || got[0]["store_id"] != float64(f.store.StoreID) || got[0]["exposable"] != false || got[0]["version"] != float64(2) {
		t.Fatalf("relay jobs=%v, want one not-exposable announcement at version 2", got)
	}
}
