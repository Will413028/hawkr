package buyer_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var validUUID = uuid.MustParse("0190a000-0000-7000-8000-000000000001")

// UUID 型的 ID 不能是零值：在存取資料庫前拒絕。
func TestNilUUIDsRejectedBeforeDatabaseAccess(t *testing.T) {
	module := New(Deps{Now: time.Now})
	ctx := context.Background()
	nilID := uuid.UUID{}
	for name, check := range map[string]func() error{
		"add favorite":      func() error { return module.AddFavorite(ctx, 1, nilID) },
		"remove favorite":   func() error { return module.RemoveFavorite(ctx, 1, nilID) },
		"check favorite":    func() error { _, err := module.CheckFavorite(ctx, 1, nilID); return err },
		"record visit":      func() error { return module.RecordVisit(ctx, 1, nilID) },
		"subscribe product": func() error { return module.SubscribeRestock(ctx, 1, nilID, validUUID) },
		"subscribe sku":     func() error { return module.SubscribeRestock(ctx, 1, validUUID, nilID) },
		"unsubscribe":       func() error { return module.UnsubscribeRestock(ctx, 1, nilID) },
		"purge":             func() error { return module.PurgeRestockSKUs(ctx, []uuid.UUID{validUUID, nilID}) },
		"fanout product": func() error {
			_, err := module.FanOut(ctx, FanOutInput{SKUID: validUUID, Limit: 100, OccurredAt: time.Now()})
			return err
		},
		"fanout sku": func() error {
			_, err := module.FanOut(ctx, FanOutInput{ProductID: validUUID, Limit: 100, OccurredAt: time.Now()})
			return err
		},
		"favorite list": func() error {
			_, err := module.ListFavorites(ctx, ProductListInput{BuyerID: 1, After: &ProductCursor{Time: time.Now()}})
			return err
		},
		"restock list": func() error {
			_, err := module.ListRestock(ctx, RestockListInput{BuyerID: 1, After: &RestockCursor{Time: time.Now()}})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := check(); apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestInvalidIDsRejectedBeforeDatabaseAccess(t *testing.T) {
	module := New(Deps{Now: time.Now})
	ctx := context.Background()
	for _, invalidID := range []int64{0, -1} {
		checks := map[string]func() error{
			"profile":               func() error { _, err := module.Profile(ctx, invalidID); return err },
			"profile by buyer":      func() error { _, err := module.ProfileByBuyerID(ctx, invalidID); return err },
			"lock active":           func() error { return module.LockActive(ctx, invalidID) },
			"lock not closed":       func() error { return module.LockNotClosed(ctx, invalidID) },
			"account stints":        func() error { _, err := module.BuyerIDsByAccount(ctx, invalidID); return err },
			"addresses":             func() error { _, err := module.ListAddresses(ctx, invalidID); return err },
			"address owner":         func() error { _, err := module.GetAddress(ctx, invalidID, 1); return err },
			"address ID":            func() error { _, err := module.GetAddress(ctx, 1, invalidID); return err },
			"remove favorite buyer": func() error { return module.RemoveFavorite(ctx, invalidID, validUUID) },
			"add favorite buyer":    func() error { return module.AddFavorite(ctx, invalidID, validUUID) },
			"record visit buyer":    func() error { return module.RecordVisit(ctx, invalidID, validUUID) },
			"check favorite buyer":  func() error { _, err := module.CheckFavorite(ctx, invalidID, validUUID); return err },
			"unfollow buyer":        func() error { return module.Unfollow(ctx, invalidID, 1) },
			"unfollow store":        func() error { return module.Unfollow(ctx, 1, invalidID) },
			"check follow":          func() error { _, err := module.CheckFollow(ctx, 1, invalidID); return err },
			"unsubscribe buyer":     func() error { return module.UnsubscribeRestock(ctx, invalidID, validUUID) },
			"subscribe buyer":       func() error { return module.SubscribeRestock(ctx, invalidID, validUUID, validUUID) },
			"clear visits":          func() error { return module.ClearVisits(ctx, invalidID) },
			"subscriber counts":     func() error { _, err := module.SubscriberCounts(ctx, []int64{invalidID}); return err },
			"subscriber stats": func() error {
				_, err := module.SubscriberStats(ctx, invalidID, []TimeWindow{{Start: time.Unix(1, 0), End: time.Unix(2, 0)}})
				return err
			},
			"entitlement":       func() error { _, err := module.Entitlement(ctx, invalidID); return err },
			"complete deletion": func() error { return module.CloseForDeletion(ctx, invalidID) },
		}
		for name, check := range checks {
			t.Run(name, func(t *testing.T) {
				if err := check(); apperr.KindOf(err) != apperr.KindInvalidInput {
					t.Fatalf("ID %d: %v", invalidID, err)
				}
			})
		}
	}
}

func TestSubscriberStatsRejectsBadWindowsBeforeDatabaseAccess(t *testing.T) {
	module := New(Deps{Now: time.Now})
	at := time.Unix(100, 0)
	ok := TimeWindow{Start: at, End: at.Add(time.Hour)}
	for name, windows := range map[string][]TimeWindow{
		"none":     nil,
		"too many": {ok, ok, ok, ok, ok},
		"empty":    {{Start: at, End: at}},
		"reversed": {ok, {Start: at, End: at.Add(-time.Second)}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := module.SubscriberStats(context.Background(), 1, windows); apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Fatalf("windows %s: %v", name, err)
			}
		})
	}
}

func TestBatchBoundariesRejectedBeforeDatabaseAccess(t *testing.T) {
	module := New(Deps{Now: time.Now})
	ctx := context.Background()
	oversized := make([]int64, MaxBatchSize+1)
	for index := range oversized {
		oversized[index] = int64(index + 1)
	}
	for _, ids := range [][]int64{{1, 0}, {1, -1}, oversized} {
		checks := map[string]func() error{
			"display":    func() error { _, err := module.BatchDisplay(ctx, ids); return err },
			"meta":       func() error { _, err := module.BatchMeta(ctx, ids); return err },
			"principals": func() error { _, err := module.ResolvePrincipals(ctx, ids); return err },
			"counts":     func() error { _, err := module.SubscriberCounts(ctx, ids); return err },
		}
		for name, check := range checks {
			t.Run(name, func(t *testing.T) {
				if err := check(); apperr.KindOf(err) != apperr.KindInvalidInput {
					t.Fatalf("batch %v: %v", ids, err)
				}
			})
		}
	}
}

func TestUUIDBatchBoundariesRejectedBeforeDatabaseAccess(t *testing.T) {
	module := New(Deps{Now: time.Now})
	ctx := context.Background()
	oversized := make([]uuid.UUID, MaxBatchSize+1)
	for index := range oversized {
		oversized[index] = uuid.NewV7()
	}
	for _, ids := range [][]uuid.UUID{{validUUID, {}}, oversized} {
		for name, check := range map[string]func() error{
			"favorites": func() error { _, err := module.BatchCheckFavorites(ctx, 1, ids); return err },
			"restock":   func() error { _, err := module.BatchCheckRestock(ctx, 1, ids); return err },
		} {
			t.Run(name, func(t *testing.T) {
				if err := check(); apperr.KindOf(err) != apperr.KindInvalidInput {
					t.Fatalf("batch of %d: %v", len(ids), err)
				}
			})
		}
	}
}

func TestFanOutLimitRejectedBeforeDatabaseAccess(t *testing.T) {
	module := New(Deps{Now: time.Now})
	for _, limit := range []int32{-1, 0, MaxBatchSize + 1} {
		more, err := module.FanOut(context.Background(), FanOutInput{ProductID: validUUID, SKUID: validUUID, Limit: limit, OccurredAt: time.Now()})
		if more || apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("limit %d: more=%v err=%v", limit, more, err)
		}
	}
}

// 補貨事件的發生時間必填：零值不能悄悄退回處理當下的時間（M4 的缺陷）。
func TestFanOutRejectsZeroOccurrenceTime(t *testing.T) {
	module := New(Deps{Now: time.Now})
	more, err := module.FanOut(context.Background(), FanOutInput{ProductID: validUUID, SKUID: validUUID, Limit: 100, ProductName: "蘋果"})
	if more || apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("zero occurred_at: more=%v err=%v", more, err)
	}
}
