//go:build integration

package couponconsumer

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type openStores struct{}

func (openStores) StoreSuspended(context.Context, int64) (bool, error) { return false, nil }

type allBuyers struct{}

func (allBuyers) Existing(_ context.Context, ids []int64) ([]int64, error) { return ids, nil }

type openDirectory struct{}

func (openDirectory) StoreState(context.Context, int64) (coupon.StoreState, error) {
	return coupon.StoreState{Exists: true}, nil
}

var now = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

// 重放冪等（端到端）：同一筆 order.completed 送兩次，投影只有一列；另一家店的事件各自一列；
// MEMBER 活動的領取在該店的事件抵達後才成立，別家店的消費不算。
func TestReplayedOrderCompletedProjectsOnceAndDrivesMemberEligibility(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "coupon", Dir: coupon.Migrations()},
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
	))
	m := coupon.New(coupon.Deps{DB: d, Stores: openStores{}, Buyers: allBuyers{}, StoreDirectory: openDirectory{}, NewID: uuid.NewV7, Now: func() time.Time { return now }})
	c := Consumer{Coupon: m}
	ctx := context.Background()

	camp, err := m.CreateCampaign(ctx, coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "會員券", DiscountType: coupon.DiscountCash, DiscountAmount: 50, StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour)},
		TargetType:   coupon.TargetMember, StoreID: 5, Issuer: coupon.Issuer{StoreID: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Claim(ctx, camp.ID, 71); coupon.ReasonOf(err) != coupon.ReasonNotEligible {
		t.Fatalf("事件抵達前：%v，要 NOT_ELIGIBLE", err)
	}
	for _, body := range []string{
		`{"event_id":"e1","order_no":"O1","buyer_id":"71","store_id":"5"}`,
		`{"event_id":"e1","order_no":"O1","buyer_id":"71","store_id":"5"}`,
		`{"event_id":"e2","order_no":"O2","buyer_id":"72","store_id":"6"}`,
	} {
		if err := c.OrderCompleted(ctx, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := d.Pool().QueryRow(ctx, "SELECT count(*) FROM coupon.store_buyer_membership").Scan(&n); err != nil || n != 2 {
		t.Fatalf("投影 %d 列（err=%v），要 2", n, err)
	}
	if _, err := m.Claim(ctx, camp.ID, 71); err != nil {
		t.Fatalf("事件抵達後：%v", err)
	}
	if _, err := m.Claim(ctx, camp.ID, 72); coupon.ReasonOf(err) != coupon.ReasonNotEligible {
		t.Fatalf("在別家店消費不算：%v", err)
	}
}
