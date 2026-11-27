//go:build integration

package buyer_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

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

func TestAvatarVersionCAS(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)
	token, _ := stageAvatar(t, f, p.AccountID)
	updated, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		AvatarStagedToken: &token,
	})
	must(t, err)
	if updated.Version != p.Version+1 {
		t.Fatalf("version=%d", updated.Version)
	}
	next, nextAsset := stageAvatar(t, f, p.AccountID)
	_, err = f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		AvatarStagedToken: &next,
	})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale avatar error=%v", err)
	}
	if got := assetState(t, f, nextAsset); got != "STAGED" {
		t.Fatalf("stale avatar confirmed the new asset: %s", got)
	}
}

func TestNoOpUpdateDoesNotBumpVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 1)

	sameProfile, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:         p.BuyerID,
		ExpectedVersion: p.Version,
		DisplayName:     &p.DisplayName,
	})
	must(t, err)
	if sameProfile.Version != p.Version {
		t.Fatalf("profile version bumped on no-op: got %d, want %d", sameProfile.Version, p.Version)
	}

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
	token, _ := stageAvatar(t, f, p.AccountID)
	updated, err := f.module.UpdateProfile(ctx, buyer.UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		DisplayName:       &newName,
		AvatarStagedToken: &token,
	})
	must(t, err)
	if updated.DisplayName != "新暱稱" || updated.Avatar == nil || !strings.HasPrefix(*updated.Avatar, "public/") {
		t.Fatalf("avatar + text update incomplete: %+v", updated)
	}
	if updated.Version != p.Version+2 {
		t.Fatalf("expected version %d, got %d", p.Version+2, updated.Version)
	}
	ds, err := f.module.BatchDisplay(ctx, []int64{p.BuyerID})
	must(t, err)
	var stored uuid.UUID
	if err := f.database.Conn(ctx).QueryRow(ctx, "SELECT avatar_asset_id FROM buyer.buyers WHERE buyer_id = $1", p.BuyerID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].AvatarAssetID == nil || *ds[0].AvatarAssetID != stored {
		t.Fatalf("BatchDisplay avatar asset = %v, want %v", ds, stored)
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
