//go:build integration

package buyer_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 已關閉的買家再收到關閉或進入等待（重送、遲到的訂單結論）時不動作，也不影響其他買家。
func TestDeletionOfClosedBuyerIsNoOp(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	old := profile(t, fixture, 1)
	other := profile(t, fixture, 2)
	must(t, fixture.module.CloseForDeletion(ctx, old.BuyerID))
	for range 2 {
		must(t, fixture.module.CloseForDeletion(ctx, old.BuyerID))
	}
	must(t, fixture.module.BeginDeletion(ctx, old.BuyerID))
	if n := count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND status='CLOSED'", old.BuyerID); n != 1 {
		t.Fatal("closed buyer reopened")
	}
	actual, err := fixture.module.Profile(ctx, 2)
	must(t, err)
	if actual.BuyerID != other.BuyerID || actual.Status != Active {
		t.Fatalf("repeated deletion affected another buyer: %+v", actual)
	}
}

func TestDeletionRejectsCloseWhileWaiting(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	current := profile(t, fixture, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	completed := make(chan error, 1)
	err := fixture.database.InTx(ctx, func(transaction context.Context) error {
		if err := fixture.module.CloseForDeletion(transaction, current.BuyerID); err != nil {
			return err
		}
		go func() {
			err := fixture.module.BeginDeletion(ctx, current.BuyerID)
			completed <- err
		}()
		return waitForBuyerLock(ctx, fixture)
	})
	must(t, err)
	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("concurrent close failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestRestockBatchesAndRollback(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	var buyerIDs []int64
	for accountID := int64(1); accountID <= 5; accountID++ {
		buyer := profile(t, fixture, accountID)
		buyerIDs = append(buyerIDs, buyer.BuyerID)
		must(t, fixture.module.SubscribeRestock(ctx, buyer.BuyerID, 10))
	}
	firstPage, err := fixture.module.ListRestock(ctx, RestockListInput{BuyerID: buyerIDs[0]})
	must(t, err)
	if len(firstPage.Items) != 1 || firstPage.Items[0].ProductID != 10 {
		t.Fatal(firstPage)
	}
	for _, productID := range []int64{11, 12} {
		must(t, fixture.module.SubscribeRestock(ctx, buyerIDs[0], productID))
	}
	var cursor *RestockCursor
	for _, expected := range []int64{12, 11, 10} {
		page, err := fixture.module.ListRestock(ctx, RestockListInput{BuyerID: buyerIDs[0], PageSize: 1, After: cursor})
		must(t, err)
		if len(page.Items) != 1 || page.Items[0].ProductID != expected {
			t.Fatalf("restock cursor skipped a product: %+v", page)
		}
		cursor = page.Next
	}
	if cursor != nil {
		t.Fatal("restock cursor should end")
	}
	for _, productID := range []int64{11, 12} {
		must(t, fixture.module.UnsubscribeRestock(ctx, buyerIDs[0], productID))
	}
	targeted := 0
	reject := true
	events.Subscribe(fixture.bus, func(context.Context, RestockTargeted) error {
		if reject {
			return errors.New("relay failed")
		}
		targeted++
		return nil
	})
	more, err := fixture.module.FanOut(ctx, FanOutInput{ProductID: 10, Limit: 2, OccurredAt: fixture.now})
	if err == nil || more || count(t, fixture, "SELECT count(*) FROM buyer.restock_subscriptions WHERE product_id=10") != 5 {
		t.Fatal("failed event did not roll back subscriptions", more, err)
	}
	reject = false
	for index, expectedMore := range []bool{true, true, false} {
		before := targeted
		more, err := fixture.module.FanOut(ctx, FanOutInput{ProductID: 10, Limit: 2, OccurredAt: fixture.now})
		must(t, err)
		if more != expectedMore || targeted-before > 2 || targeted != min((index+1)*2, 5) {
			t.Fatalf("batch %d: more=%v targeted=%d", index, more, targeted)
		}
	}
	more, err = fixture.module.FanOut(ctx, FanOutInput{ProductID: 10, Limit: 2, OccurredAt: fixture.now})
	must(t, err)
	if more || targeted != 5 || count(t, fixture, "SELECT count(*) FROM buyer.restock_subscriptions") != 0 {
		t.Fatal("fanout retained history or repeated events")
	}
	must(t, fixture.module.SubscribeRestock(ctx, buyerIDs[0], 10))
	must(t, fixture.module.UnsubscribeRestock(ctx, buyerIDs[0], 10))
	if count(t, fixture, "SELECT count(*) FROM buyer.restock_subscriptions") != 0 {
		t.Fatal("unsubscribe retained history")
	}
	err = fixture.database.InTx(ctx, func(ctx context.Context) error {
		_, err := fixture.module.FanOut(ctx, FanOutInput{ProductID: 10, Limit: 2, OccurredAt: fixture.now})
		return err
	})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatal("fanout must own its transaction", err)
	}
}

// 總數與每個時間窗的新增數來自同一句查詢：窗左閉右開、以追蹤時間計，窗可以重疊，只算指定的店，取消追蹤的不再計入。
func TestSubscriberStatsCountsEachWindowHalfOpenForTheStore(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	start := time.Date(2026, 9, 27, 0, 0, 0, 0, twtime.Taipei)
	end := start.AddDate(0, 0, 7)
	previous := TimeWindow{Start: start.AddDate(0, 0, -7), End: start}
	current := TimeWindow{Start: start, End: end}
	var buyerIDs []int64
	for index, follow := range []struct {
		at    time.Time
		store int64
	}{
		{previous.Start.Add(-time.Second), 20}, // 兩個窗之外
		{previous.Start, 20},                   // 前期起點：前期
		{start.Add(-time.Second), 20},          // 前期終點前一秒：前期
		{start, 20},                            // 當期起點：當期
		{end.Add(-time.Second), 20},            // 當期終點前一秒：當期
		{end, 20},                              // 當期終點：窗外
		{start.Add(time.Hour), 21},             // 別家店
	} {
		fixture.now = follow.at
		buyer := profile(t, fixture, int64(index+1))
		must(t, fixture.module.Follow(ctx, buyer.BuyerID, follow.store))
		buyerIDs = append(buyerIDs, buyer.BuyerID)
	}
	got, err := fixture.module.SubscriberStats(ctx, 20, []TimeWindow{current, previous})
	must(t, err)
	if got.Total != 6 || len(got.New) != 2 || got.New[0] != 2 || got.New[1] != 2 {
		t.Fatalf("stats=%+v, want total 6 and new [2 2] in request order", got)
	}
	// 窗的順序就是回傳的順序，也可以重疊與重複。
	got, err = fixture.module.SubscriberStats(ctx, 20, []TimeWindow{previous, current, {Start: previous.Start, End: end}, previous})
	must(t, err)
	if got.Total != 6 || len(got.New) != 4 || got.New[0] != 2 || got.New[1] != 2 || got.New[2] != 4 || got.New[3] != 2 {
		t.Fatalf("overlapping windows: %+v", got)
	}
	// 沒有追蹤的區間是 0，不是沒有列；沒有粉絲的店總數也是 0。
	got, err = fixture.module.SubscriberStats(ctx, 20, []TimeWindow{{Start: end.AddDate(0, 0, 7), End: end.AddDate(0, 0, 14)}})
	must(t, err)
	if got.Total != 6 || len(got.New) != 1 || got.New[0] != 0 {
		t.Fatalf("empty window: %+v", got)
	}
	if got, err = fixture.module.SubscriberStats(ctx, 22, []TimeWindow{current}); err != nil || got.Total != 0 || len(got.New) != 1 || got.New[0] != 0 {
		t.Fatalf("store without followers: %+v %v", got, err)
	}
	if got, err = fixture.module.SubscriberStats(ctx, 21, []TimeWindow{current}); err != nil || got.Total != 1 || got.New[0] != 1 {
		t.Fatalf("other store: %+v %v", got, err)
	}
	must(t, fixture.module.Unfollow(ctx, buyerIDs[3], 20))
	if got, err = fixture.module.SubscriberStats(ctx, 20, []TimeWindow{current}); err != nil || got.Total != 5 || got.New[0] != 1 {
		t.Fatalf("after unfollow: %+v %v", got, err)
	}
}

func TestReadErrorsReturnNilCollections(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	buyer := profile(t, fixture, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assertNil := func(value any, err error) {
		t.Helper()
		if err == nil || !reflect.ValueOf(value).IsNil() {
			t.Fatalf("failed read returned defaults: %#v, %v", value, err)
		}
	}
	displays, err := fixture.module.BatchDisplay(ctx, []int64{buyer.BuyerID})
	assertNil(displays, err)
	profiles, err := fixture.module.BatchMeta(ctx, []int64{buyer.BuyerID})
	assertNil(profiles, err)
	principals, err := fixture.module.ResolvePrincipals(ctx, []int64{buyer.BuyerID})
	assertNil(principals, err)
	addresses, err := fixture.module.ListAddresses(ctx, buyer.BuyerID)
	assertNil(addresses, err)
	favorites, err := fixture.module.BatchCheckFavorites(ctx, buyer.BuyerID, []int64{10})
	assertNil(favorites, err)
	restock, err := fixture.module.BatchCheckRestock(ctx, buyer.BuyerID, []int64{10})
	assertNil(restock, err)
	counts, err := fixture.module.SubscriberCounts(ctx, []int64{20})
	assertNil(counts, err)
	if stats, err := fixture.module.SubscriberStats(ctx, 20, []TimeWindow{{Start: time.Unix(1, 0), End: time.Unix(2, 0)}}); err == nil || stats.New != nil || stats.Total != 0 {
		t.Fatalf("failed read returned stats: %+v, %v", stats, err)
	}
	plans, err := fixture.module.ListPlans(ctx)
	assertNil(plans, err)
}

func TestMissingAddressNamesCorrectEntity(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	input := addressInput(buyer.BuyerID)
	input.AddressID = 999999
	input.ExpectedVersion = 1
	checks := []func() error{
		func() error { _, err := fixture.module.GetAddress(ctx, buyer.BuyerID, input.AddressID); return err },
		func() error { _, err := fixture.module.UpdateAddress(ctx, input); return err },
		func() error { return fixture.module.DeleteAddress(ctx, buyer.BuyerID, input.AddressID, 0) },
	}
	for _, check := range checks {
		err := check()
		if apperr.KindOf(err) != apperr.KindNotFound || !strings.Contains(err.Error(), "地址") || strings.Contains(err.Error(), "買家資料") {
			t.Fatal(err)
		}
	}
}

func waitForBuyerLock(ctx context.Context, fixture *fixture) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := fixture.database.Conn(ctx).QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM pg_stat_activity
    WHERE datname = current_database() AND wait_event_type = 'Lock'
      AND query LIKE '%-- name: LockBuyer :one%'
)`).Scan(&waiting)
		if err != nil {
			return err
		}
		if waiting {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
