//go:build integration

package buyer_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestProfileVersionCAS(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	first := "第一版"
	updated, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{BuyerID: p.BuyerID, ExpectedVersion: p.Version, DisplayName: &first})
	must(t, err)
	if updated.Version != p.Version+1 {
		t.Fatalf("version=%d", updated.Version)
	}
	stale := "過期寫入"
	_, err = f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{BuyerID: p.BuyerID, ExpectedVersion: p.Version, DisplayName: &stale})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale profile error=%v", err)
	}
	actual, err := f.module.Profile(ctx, p.AccountID)
	must(t, err)
	if actual.DisplayName != first || actual.Version != updated.Version {
		t.Fatalf("stale write applied: %+v", actual)
	}
}

func TestAddressVersionCASAndDefaultBumps(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	input := buyer.AddressInput{BuyerID: p.BuyerID, ReceiverName: "王小明", ReceiverPhone: "0912345678", TownCode: "63000050", AddressDetail: "忠孝東路一號"}
	first, err := f.module.AddAddress(ctx, input)
	must(t, err)
	second, err := f.module.AddAddress(ctx, input)
	must(t, err)
	input.AddressID, input.ExpectedVersion, input.IsDefault = second.AddressID, second.Version, true
	second, err = f.module.UpdateAddress(ctx, input)
	must(t, err)
	changedFirst, err := f.module.GetAddress(ctx, p.BuyerID, first.AddressID)
	must(t, err)
	if changedFirst.IsDefault || changedFirst.Version != first.Version+1 || !second.IsDefault {
		t.Fatalf("default versions: first=%+v second=%+v", changedFirst, second)
	}
	input.ExpectedVersion = second.Version - 1
	input.AddressDetail = "過期地址"
	_, err = f.module.UpdateAddress(ctx, input)
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale address error=%v", err)
	}
	err = f.module.DeleteAddress(ctx, p.BuyerID, second.AddressID, second.Version-1)
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale delete error=%v", err)
	}
	actual, err := f.module.GetAddress(ctx, p.BuyerID, second.AddressID)
	must(t, err)
	if actual.AddressDetail == input.AddressDetail || actual.Version != second.Version {
		t.Fatalf("stale address write applied: %+v", actual)
	}
	must(t, f.module.DeleteAddress(ctx, p.BuyerID, second.AddressID, second.Version))
	promoted, err := f.module.GetAddress(ctx, p.BuyerID, first.AddressID)
	must(t, err)
	if !promoted.IsDefault || promoted.Version != changedFirst.Version+1 {
		t.Fatalf("promotion version=%+v", promoted)
	}
}

func TestAddressKeysetPaging(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	input := buyer.AddressInput{BuyerID: p.BuyerID, ReceiverName: "王小明", ReceiverPhone: "0912345678", TownCode: "63000050", AddressDetail: "忠孝東路一號"}
	// 上限 20 筆（W4）以內，頁大小取不整除的 7，走完三頁。
	for range buyer.MaxAddresses {
		_, err := f.module.AddAddress(ctx, input)
		must(t, err)
	}
	seen := map[int64]bool{}
	var after *buyer.AddressCursor
	for {
		page, err := f.module.PageAddresses(ctx, buyer.AddressListInput{BuyerID: p.BuyerID, PageSize: 7, After: after})
		must(t, err)
		for _, address := range page.Items {
			if seen[address.AddressID] {
				t.Fatalf("duplicate address %d", address.AddressID)
			}
			seen[address.AddressID] = true
			if len(seen) == 1 && !address.IsDefault {
				t.Fatal("default address not first")
			}
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if len(seen) != buyer.MaxAddresses {
		t.Fatalf("page count=%d", len(seen))
	}
}

func TestPlanVersionCAS(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	plans, err := f.module.ListPlans(ctx)
	must(t, err)
	if len(plans) == 0 {
		t.Fatal("missing seed plans")
	}
	plan := plans[0]
	limit := plan.MonthlyAnswerLimit + 1
	updated, err := f.module.UpdatePlan(ctx, buyer.UpdatePlanInput{PlanCode: plan.PlanCode, ExpectedVersion: plan.Version, MonthlyAnswerLimit: &limit})
	must(t, err)
	if updated.Version != plan.Version+1 {
		t.Fatalf("version=%d", updated.Version)
	}
	stale := limit + 1
	_, err = f.module.UpdatePlan(ctx, buyer.UpdatePlanInput{PlanCode: plan.PlanCode, ExpectedVersion: plan.Version, MonthlyAnswerLimit: &stale})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale plan error=%v", err)
	}
	plans, err = f.module.ListPlans(ctx)
	must(t, err)
	for _, got := range plans {
		if got.PlanCode == plan.PlanCode && (got.MonthlyAnswerLimit != limit || got.Version != updated.Version) {
			t.Fatalf("stale plan write applied: %+v", got)
		}
	}
}

func TestAvatarVersionCAS(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	token := "staged"
	updated, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		AvatarStagedToken: &token,
	})
	must(t, err)
	if updated.Version != p.Version+1 {
		t.Fatalf("version=%d", updated.Version)
	}
	before := fmt.Sprint(f.media.Calls())
	_, err = f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		AvatarStagedToken: &token,
	})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale avatar error=%v", err)
	}
	if fmt.Sprint(f.media.Calls()) != before {
		t.Fatal("stale avatar promoted")
	}
}

func TestNoOpUpdateDoesNotBumpVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)

	// Profile no-op 更新：值沒有變動時不遞增版本
	sameProfile, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:         p.BuyerID,
		ExpectedVersion: p.Version,
		DisplayName:     &p.DisplayName,
	})
	must(t, err)
	if sameProfile.Version != p.Version {
		t.Fatalf("profile version bumped on no-op: got %d, want %d", sameProfile.Version, p.Version)
	}

	// Address no-op 更新：值沒有變動時不遞增版本
	addr, err := f.module.AddAddress(ctx, buyer.AddressInput{
		BuyerID:       p.BuyerID,
		ReceiverName:  "測試人",
		ReceiverPhone: "0912345678",
		TownCode:      "63000050",
		AddressDetail: "測試街 1 號",
	})
	must(t, err)
	sameAddr, err := f.module.UpdateAddress(ctx, buyer.AddressInput{
		BuyerID:         p.BuyerID,
		AddressID:       addr.AddressID,
		ExpectedVersion: addr.Version,
		ReceiverName:    addr.ReceiverName,
		ReceiverPhone:   addr.ReceiverPhone,
		TownCode:        addr.TownCode,
		AddressDetail:   addr.AddressDetail,
		IsDefault:       addr.IsDefault,
	})
	must(t, err)
	if sameAddr.Version != addr.Version {
		t.Fatalf("address version bumped on no-op: got %d, want %d", sameAddr.Version, addr.Version)
	}
}

func TestAvatarAndTextInOneUpdate(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	newName := "新暱稱"
	token := "staged-combined"
	updated, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		DisplayName:       &newName,
		AvatarStagedToken: &token,
	})
	must(t, err)
	if updated.DisplayName != "新暱稱" || updated.Avatar == nil || *updated.Avatar != "avatar/staged-combined" {
		t.Fatalf("avatar + text update incomplete: %+v", updated)
	}
	if updated.Version != p.Version+2 {
		// text update bumps version by 1, avatar update bumps version by 1 in the same transaction
		t.Fatalf("expected version %d, got %d", p.Version+2, updated.Version)
	}
}

func TestDeletionParticipantIdempotency(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	must(t, f.module.BeginDeletion(ctx, p.BuyerID))
	pending, err := f.module.Profile(ctx, p.AccountID)
	must(t, err)
	must(t, f.module.BeginDeletion(ctx, p.BuyerID))
	again, err := f.module.Profile(ctx, p.AccountID)
	must(t, err)
	if again.Version != pending.Version {
		t.Fatal("repeat request wrote pending stint")
	}
	must(t, f.module.CloseForDeletion(ctx, p.BuyerID))
	must(t, f.module.CloseForDeletion(ctx, p.BuyerID))
}
