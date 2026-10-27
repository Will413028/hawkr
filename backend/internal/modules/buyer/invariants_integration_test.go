//go:build integration

package buyer_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func TestProfilePartialUpdateAndPendingWrites(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	birth := fixture.now.AddDate(-20, 0, 0)
	buyer, err := fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:         buyer.BuyerID,
		ExpectedVersion: buyer.Version,
		Phone:           ptr("0912345678"),
		Gender:          ptr(Female),
		BirthDate:       &birth,
		City:            ptr("臺北市"),
		Job:             ptr("工程師"),
	})
	must(t, err)
	must(t, fixture.module.BeginDeletion(ctx, buyer.BuyerID))
	buyer, err = fixture.module.ProfileByBuyerID(ctx, buyer.BuyerID)
	must(t, err)
	updated, err := fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:         buyer.BuyerID,
		ExpectedVersion: buyer.Version,
		City:            ptr("臺南市"),
	})
	must(t, err)
	if updated.DisplayName != buyer.DisplayName || updated.Phone == nil || *updated.Phone != "0912345678" ||
		updated.Gender == nil || *updated.Gender != Female || updated.BirthDate == nil ||
		updated.City == nil || *updated.City != "臺南市" || updated.Job == nil || *updated.Job != "工程師" {
		t.Fatalf("omitted fields overwritten: %+v", updated)
	}
	updated, err = fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:         buyer.BuyerID,
		ExpectedVersion: updated.Version,
		Phone:           ptr(""),
		Gender:          ptr(Gender("")),
		BirthDate:       ptr(time.Time{}),
		City:            ptr(""),
		Job:             ptr(""),
	})
	must(t, err)
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND phone IS NULL AND gender IS NULL AND birth_date IS NULL AND city IS NULL AND job IS NULL", buyer.BuyerID) != 1 {
		t.Fatal("explicit empty fields must become NULL")
	}
	_, err = fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:         buyer.BuyerID,
		ExpectedVersion: updated.Version,
		DisplayName:     ptr(""),
	})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("empty display name: %v", err)
	}
	address, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
	input := addressInput(buyer.BuyerID)
	input.AddressID = address.AddressID
	input.ExpectedVersion = address.Version
	input.ReceiverName = "修改收件人"
	_, err = fixture.module.UpdateAddress(ctx, input)
	must(t, err)
	must(t, fixture.module.DeleteAddress(ctx, buyer.BuyerID, address.AddressID, 0))
	must(t, fixture.module.RecordVisit(ctx, buyer.BuyerID, 10))
	must(t, fixture.module.ClearVisits(ctx, buyer.BuyerID))
	must(t, fixture.module.SubscribeRestock(ctx, buyer.BuyerID, 10))
	must(t, fixture.module.UnsubscribeRestock(ctx, buyer.BuyerID, 10))
	_, err = fixture.module.AssignMembership(ctx, AssignMembershipInput{BuyerID: buyer.BuyerID, PlanCode: "basic", RequestID: "pending"})
	must(t, err)
	current, err := fixture.module.Profile(ctx, buyer.AccountID)
	must(t, err)
	token := "token"
	updatedAvatar, err := fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:           buyer.BuyerID,
		ExpectedVersion:   current.Version,
		AvatarStagedToken: &token,
	})
	must(t, err)
	if updatedAvatar.Avatar == nil || *updatedAvatar.Avatar != "avatar/token" {
		t.Fatal(updatedAvatar)
	}
	for _, write := range []func() error{
		func() error { return fixture.module.AddFavorite(ctx, buyer.BuyerID, 10) },
		func() error { return fixture.module.Follow(ctx, buyer.BuyerID, 10) },
	} {
		if err := write(); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("pending favorite/follow accepted: %v", err)
		}
	}
}

func TestConcurrentProfilePatchesPreserveBothFields(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	buyer := profile(t, fixture, 1)
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, patch := range []func(int64) UpdateProfileInput{
		func(v int64) UpdateProfileInput {
			return UpdateProfileInput{BuyerID: buyer.BuyerID, ExpectedVersion: v, City: ptr("臺南市")}
		},
		func(v int64) UpdateProfileInput {
			return UpdateProfileInput{BuyerID: buyer.BuyerID, ExpectedVersion: v, Job: ptr("工程師")}
		},
	} {
		go func(buildInput func(int64) UpdateProfileInput) {
			<-start
			for {
				cur, err := fixture.module.ProfileByBuyerID(ctx, buyer.BuyerID)
				if err != nil {
					results <- err
					return
				}
				_, err = fixture.module.UpdateProfile(ctx, buildInput(cur.Version))
				if err == nil {
					results <- nil
					return
				}
				if apperr.KindOf(err) != apperr.KindVersionConflict {
					results <- err
					return
				}
			}
		}(patch)
	}
	close(start)
	for range 2 {
		must(t, <-results)
	}
	updated, err := fixture.module.ProfileByBuyerID(ctx, buyer.BuyerID)
	must(t, err)
	if updated.City == nil || *updated.City != "臺南市" || updated.Job == nil || *updated.Job != "工程師" {
		t.Fatalf("lost concurrent patch: %+v", updated)
	}
}

func TestLongSuggestedNicknameAndClosedPrincipals(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	must(t, fixture.module.OnAccountCreated(ctx, identity.AccountCreated{AccountID: 1, Name: strings.Repeat("名", 101)}))
	buyer, err := fixture.module.Profile(ctx, 1)
	must(t, err)
	if buyer.DisplayName != strings.Repeat("名", 100) {
		t.Fatal("nickname was not truncated at the rune limit")
	}
	must(t, fixture.module.CloseForDeletion(ctx, buyer.BuyerID))
	current := profile(t, fixture, 2)
	resolved, err := fixture.module.ResolvePrincipals(ctx, []int64{buyer.BuyerID, current.BuyerID, 999999})
	must(t, err)
	if len(resolved) != 1 || resolved[current.BuyerID] != 2 {
		t.Fatalf("closed or missing principal resolved: %v", resolved)
	}
}

func TestSeededEntitlementAndPlanErrors(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	entitlement, err := fixture.module.Entitlement(ctx, buyer.BuyerID)
	must(t, err)
	if entitlement.PlanCode != "free" || entitlement.MonthlyAnswerLimit != 100 {
		t.Fatal(entitlement)
	}
	_, err = fixture.module.AssignMembership(ctx, AssignMembershipInput{BuyerID: buyer.BuyerID, PlanCode: "missing", RequestID: "invalid"})
	var domainError *apperr.Error
	if apperr.KindOf(err) != apperr.KindInvalidInput || !errors.As(err, &domainError) || len(domainError.FieldViolations()) != 1 || domainError.FieldViolations()[0].Field != "plan_code" {
		t.Fatalf("assign unknown plan: %v", err)
	}
	_, err = fixture.module.UpdatePlan(ctx, UpdatePlanInput{PlanCode: "missing", MonthlyAnswerLimit: ptr(int32(100))})
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("update unknown plan: %v", err)
	}
	_, err = fixture.database.Conn(ctx).Exec(ctx, "DELETE FROM buyer.membership_plans WHERE plan_code='free'")
	must(t, err)
	_, err = fixture.module.Entitlement(ctx, buyer.BuyerID)
	if apperr.KindOf(err) != apperr.KindNotFound || !strings.Contains(err.Error(), "會員方案") || strings.Contains(err.Error(), "買家資料") {
		t.Fatalf("missing plan mislabeled: %v", err)
	}
}

func TestDeletionPIIBoundaries(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	boundary := profile(t, fixture, 1)
	young := profile(t, fixture, 2)
	pending := profile(t, fixture, 3)
	must(t, fixture.module.BeginDeletion(ctx, boundary.BuyerID))
	must(t, fixture.module.CloseForDeletion(ctx, boundary.BuyerID))
	must(t, fixture.module.BeginDeletion(ctx, pending.BuyerID))
	fixture.now = fixture.now.Add(time.Second)
	must(t, fixture.module.CloseForDeletion(ctx, young.BuyerID))
	fixture.now = fixture.now.Add(30*24*time.Hour - time.Second)
	must(t, fixture.module.CleanupPII(ctx))
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND pii_cleared_at IS NULL", boundary.BuyerID) != 1 {
		t.Fatal("exactly 30 days must remain retained")
	}
	fixture.now = fixture.now.Add(time.Microsecond)
	must(t, fixture.module.CleanupPII(ctx))
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND pii_cleared_at IS NOT NULL", boundary.BuyerID) != 1 {
		t.Fatal("older than 30 days must be eligible")
	}
	for _, buyerID := range []int64{young.BuyerID, pending.BuyerID} {
		if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND pii_cleared_at IS NULL AND display_name='測試買家'", buyerID) != 1 {
			t.Fatal("young closed or pending PII cleared", buyerID)
		}
	}
}

// 關閉買家的副作用：補貨取消、BuyerDeleted 恰好一次（時間是關閉當下）、排一個頭像釋放；重送不重複。
func TestCloseForDeletionEffects(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	must(t, fixture.module.SubscribeRestock(ctx, buyer.BuyerID, 10))
	var deleted []BuyerDeleted
	events.Subscribe(fixture.bus, func(_ context.Context, event BuyerDeleted) error {
		deleted = append(deleted, event)
		return nil
	})
	for range 2 {
		must(t, fixture.module.CloseForDeletion(ctx, buyer.BuyerID))
	}
	if len(deleted) != 1 || deleted[0].BuyerID != buyer.BuyerID || !deleted[0].OccurredAt.Equal(fixture.now) {
		t.Fatal(deleted)
	}
	if count(t, fixture, "SELECT count(*) FROM buyer.restock_subscriptions WHERE buyer_id=$1", buyer.BuyerID) != 0 {
		t.Fatal("closing left restock active")
	}
	if len(fixture.jobs.args) != 1 || fixture.jobs.args[0].Kind() != "buyer.media_sync" {
		t.Fatal("closing did not enqueue exactly one avatar release", fixture.jobs.args)
	}
}

func TestConcurrentMembershipRequestAcrossBuyers(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	first, second := profile(t, fixture, 1), profile(t, fixture, 2)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, buyerID := range []int64{first.BuyerID, second.BuyerID} {
		go func() {
			<-start
			_, err := fixture.module.AssignMembership(ctx, AssignMembershipInput{BuyerID: buyerID, PlanCode: "basic", RequestID: "shared"})
			results <- err
		}()
	}
	close(start)
	var succeeded, rejected int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case apperr.KindOf(err) == apperr.KindInvalidInput:
			rejected++
		default:
			t.Fatal(err)
		}
	}
	if succeeded != 1 || rejected != 1 || count(t, fixture, "SELECT count(*) FROM buyer.memberships WHERE request_id='shared'") != 1 {
		t.Fatalf("succeeded=%d rejected=%d", succeeded, rejected)
	}
}

func TestEventsUseInjectedOccurrenceTime(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	timestamps := make(map[string]time.Time)
	events.Subscribe(fixture.bus, func(_ context.Context, event FavoriteAdded) error {
		timestamps["favorite.added"] = event.OccurredAt
		return nil
	})
	events.Subscribe(fixture.bus, func(_ context.Context, event FavoriteRemoved) error {
		timestamps["favorite.removed"] = event.OccurredAt
		return nil
	})
	events.Subscribe(fixture.bus, func(_ context.Context, event SellerFollowed) error {
		timestamps["seller.followed"] = event.OccurredAt
		return nil
	})
	events.Subscribe(fixture.bus, func(_ context.Context, event ProductViewed) error {
		timestamps["product.viewed"] = event.OccurredAt
		return nil
	})
	events.Subscribe(fixture.bus, func(_ context.Context, event RestockTargeted) error {
		timestamps["restock.targeted"] = event.OccurredAt
		return nil
	})
	events.Subscribe(fixture.bus, func(_ context.Context, event BuyerDeleted) error {
		timestamps["buyer.deleted"] = event.OccurredAt
		return nil
	})
	must(t, fixture.module.AddFavorite(ctx, buyer.BuyerID, 10))
	must(t, fixture.module.Follow(ctx, buyer.BuyerID, 20))
	must(t, fixture.module.RecordVisit(ctx, buyer.BuyerID, 10))
	must(t, fixture.module.SubscribeRestock(ctx, buyer.BuyerID, 10))
	{
		_, err := fixture.module.FanOut(ctx, FanOutInput{ProductID: 10, Limit: 100, OccurredAt: fixture.now})
		must(t, err)
	}
	must(t, fixture.module.CloseForDeletion(ctx, buyer.BuyerID))
	for _, name := range []string{"favorite.added", "favorite.removed", "seller.followed", "product.viewed", "restock.targeted", "buyer.deleted"} {
		if !timestamps[name].Equal(fixture.now) {
			t.Errorf("%s occurred at %s, want %s", name, timestamps[name], fixture.now)
		}
	}
}
