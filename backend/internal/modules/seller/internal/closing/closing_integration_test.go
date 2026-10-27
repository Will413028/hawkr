//go:build integration

package closing

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type accounts struct{}

func (accounts) LockAccount(context.Context, int64) error                   { return nil }
func (accounts) RemoveRole(context.Context, identity.RemoveRoleInput) error { return nil }
func (accounts) GrantRole(context.Context, identity.GrantRoleInput) error   { return nil }
func (accounts) Account(_ context.Context, id int64) (identity.Account, error) {
	return identity.Account{ID: id, Lifecycle: identity.LifecycleActive}, nil
}

type noOrders struct{}

func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type queue struct{ args []river.JobArgs }

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	q.args = append(q.args, args)
	return nil
}

// 關店可逆：logo 與封面連同 media 的引用留著（不記成退場、不排入同步），到個資清理才釋放（purge 的測試）。
func TestCloseKeepsTheImagesAndTheirMediaReferences(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "seller", Dir: os.DirFS("../../db/migrations")}))
	const accountID = 900
	var storeID int64
	if err := d.Conn(ctx).QueryRow(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, cover_key, cover_asset_id, created_at, updated_at)
		VALUES (2600000001, $1, '小店', 'logo', 'logo-asset', 'cover', 'cover-asset', now(), now()) RETURNING store_id`, accountID).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Conn(ctx).Exec(ctx, "INSERT INTO seller.store_exposures (store_id) VALUES ($1)", storeID); err != nil {
		t.Fatal(err)
	}
	fake := media.NewFake()
	logoRef := mediaref.StoreRef(media.RefTypeSellerLogo, storeID)
	coverRef := mediaref.StoreRef(media.RefTypeSellerCover, storeID)
	if err := fake.Link(ctx, logoRef, []string{"logo-asset"}); err != nil {
		t.Fatal(err)
	}
	if err := fake.Link(ctx, coverRef, []string{"cover-asset"}); err != nil {
		t.Fatal(err)
	}
	jobs := &queue{}
	bus := events.NewBus()
	s := New(Deps{DB: d, Events: bus, Jobs: jobs, Identity: accounts{}, Orders: noOrders{}, Exposure: exposure.New(exposure.Deps{Events: bus}), Now: time.Now})
	closed, err := s.CloseCurrent(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != "CLOSED" || closed.LogoKey == nil || closed.CoverKey == nil {
		t.Fatalf("關店之後 logo 與封面仍在：%+v", closed)
	}
	for _, a := range jobs.args {
		if _, ok := a.(mediaref.SyncArgs); ok {
			t.Fatalf("關店不該排入 media 引用的同步：%+v", a)
		}
	}
	syncer := mediaref.NewSyncer(d, fake, jobs, time.Now, nil)
	for _, ref := range []media.Ref{logoRef, coverRef} {
		if _, err := syncer.Sync(ctx, ref); err != nil {
			t.Fatal(err)
		}
		if got := fake.Referenced(ref); len(got) != 1 {
			t.Fatalf("%s 的引用 %v：關店之後同步仍指著業務列目前的資產", ref.Type, got)
		}
	}
	if want := strconv.FormatInt(storeID, 10); logoRef.ID != want {
		t.Fatalf("ref_id=%s want store_id %s", logoRef.ID, want)
	}
}
