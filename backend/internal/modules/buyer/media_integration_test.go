//go:build integration

package buyer_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func setAvatar(t *testing.T, f *fixture, p Profile, token string) (Profile, error) {
	t.Helper()
	return f.module.UpdateProfile(context.Background(), UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		AvatarStagedToken: &token,
	})
}

func avatarAssetOf(t *testing.T, f *fixture, buyerID int64) *uuid.UUID {
	t.Helper()
	var id *uuid.UUID
	must(t, f.database.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT avatar_asset_id FROM buyer.buyers WHERE buyer_id=$1", buyerID).Scan(&id))
	return id
}

func uploadState(t *testing.T, f *fixture, assetID uuid.UUID) string {
	t.Helper()
	var state string
	must(t, f.database.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT state FROM media.staged_uploads WHERE asset_id=$1", assetID).Scan(&state))
	return state
}

// 換圖：新資產在同一個 transaction 內確認並成為目前頭像，被換掉的資產標為刪除。
func TestReplacingTheAvatarConfirmsTheNewAssetAndMarksTheOldDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 50)
	firstToken, first := stageAvatar(t, f, p.AccountID)
	cur, err := setAvatar(t, f, p, firstToken)
	must(t, err)
	if got := assetState(t, f, first); got != "ACTIVE" {
		t.Fatalf("第一張頭像 state=%s，預期 ACTIVE", got)
	}
	secondToken, second := stageAvatar(t, f, p.AccountID)
	cur, err = setAvatar(t, f, cur, secondToken)
	must(t, err)
	if got := assetState(t, f, second); got != "ACTIVE" {
		t.Fatalf("新頭像 state=%s，預期 ACTIVE", got)
	}
	if got := assetState(t, f, first); got != "DELETING" {
		t.Fatalf("被換掉的頭像 state=%s，預期 DELETING", got)
	}
	if id := avatarAssetOf(t, f, p.BuyerID); id == nil || *id != second {
		t.Fatalf("avatar_asset_id=%v，預期 %s", id, second)
	}
	if cur.Avatar == nil {
		t.Fatal("avatar object key 為空")
	}
}

// 第一次設定頭像沒有舊資產可刪；同一張圖重複確認不會把自己標為刪除。
func TestFirstAvatarMarksNothingDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 51)
	token, asset := stageAvatar(t, f, p.AccountID)
	_, err := setAvatar(t, f, p, token)
	must(t, err)
	if got := assetState(t, f, asset); got != "ACTIVE" {
		t.Fatalf("state=%s，預期 ACTIVE", got)
	}
	if n := notActiveAssets(t, f); n != 0 {
		t.Fatalf("有 %d 個資產不是 ACTIVE", n)
	}
}

// 寫入回滾：換圖在 MarkDeleted 排 job 失敗而整筆回滾時，新資產仍是暫存、舊資產仍有效、業務列不變。
func TestRolledBackAvatarWriteLeavesTheNewAssetStaged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 52)
	firstToken, first := stageAvatar(t, f, p.AccountID)
	cur, err := setAvatar(t, f, p, firstToken)
	must(t, err)
	secondToken, second := stageAvatar(t, f, p.AccountID)

	f.jobs.fail = true
	if _, err := setAvatar(t, f, cur, secondToken); err == nil {
		t.Fatal("排 job 失敗應讓換圖回滾")
	}
	f.jobs.fail = false

	if got := assetState(t, f, second); got != "STAGED" {
		t.Fatalf("回滾後新資產 state=%s，預期 STAGED", got)
	}
	if got := uploadState(t, f, second); got != "READY" {
		t.Fatalf("回滾後新暫存 state=%s，預期 READY（未確認）", got)
	}
	if got := assetState(t, f, first); got != "ACTIVE" {
		t.Fatalf("回滾後舊資產 state=%s，預期 ACTIVE", got)
	}
	if id := avatarAssetOf(t, f, p.BuyerID); id == nil || *id != first {
		t.Fatalf("回滾後 avatar_asset_id=%v，預期 %s", id, first)
	}
	if _, err := setAvatar(t, f, cur, secondToken); err != nil {
		t.Fatalf("回滾後同一個 token 重送應成功：%v", err)
	}
}

// 版本衝突與驗證失敗發生在 Confirm 之前，新資產維持暫存。
func TestRejectedAvatarWriteDoesNotConfirm(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 53)
	token, asset := stageAvatar(t, f, p.AccountID)
	stale := p
	stale.Version = p.Version + 5
	if _, err := setAvatar(t, f, stale, token); err == nil {
		t.Fatal("版本不符應失敗")
	}
	if got := assetState(t, f, asset); got != "STAGED" {
		t.Fatalf("state=%s，預期 STAGED", got)
	}
}

// 別人的 token 不能確認：回錯誤，資產維持暫存。
func TestAvatarTokenOfAnotherAccountIsRefused(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 54)
	other := profile(t, f, 55)
	token, asset := stageAvatar(t, f, other.AccountID)
	if _, err := setAvatar(t, f, p, token); err == nil {
		t.Fatal("別的帳號的 token 應被拒絕")
	}
	if got := assetState(t, f, asset); got != "STAGED" {
		t.Fatalf("state=%s，預期 STAGED", got)
	}
}

// 終態：關閉買家時目前的頭像標為刪除；沒有頭像的買家關閉不動任何資產；重送冪等。
func TestClosingTheBuyerMarksTheAvatarDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 56)
	token, asset := stageAvatar(t, f, p.AccountID)
	_, err := setAvatar(t, f, p, token)
	must(t, err)
	bare := profile(t, f, 57)

	for range 2 {
		must(t, f.module.CloseForDeletion(ctx, p.BuyerID))
	}
	must(t, f.module.CloseForDeletion(ctx, bare.BuyerID))
	if got := assetState(t, f, asset); got != "DELETING" {
		t.Fatalf("關閉後頭像 state=%s，預期 DELETING", got)
	}
	if n := count(t, f, "SELECT count(*) FROM media.assets"); n != 1 {
		t.Fatalf("assets=%d，預期 1", n)
	}
}

// 關閉的 transaction 回滾時頭像不被標為刪除。
func TestRolledBackCloseKeepsTheAvatarActive(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := profile(t, f, 58)
	token, asset := stageAvatar(t, f, p.AccountID)
	_, err := setAvatar(t, f, p, token)
	must(t, err)
	boom := errors.New("boom")
	err = f.database.InTx(ctx, func(ctx context.Context) error {
		must(t, f.module.CloseForDeletion(ctx, p.BuyerID))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if got := assetState(t, f, asset); got != "ACTIVE" {
		t.Fatalf("回滾後 state=%s，預期 ACTIVE", got)
	}
}

// 機敏資產不能當頭像：回 InvalidInput 指到 AvatarStagedToken，資產維持暫存。
func TestSecureAssetCannotBeTheAvatar(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 59)
	data := pngBytes(t)
	put, err := f.media.PutSecure(context.Background(), media.Uploader{ID: p.AccountID}, bytes.NewReader(data), int64(len(data)))
	must(t, err)
	_, err = setAvatar(t, f, p, put.Token)
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v，預期 InvalidInput", err)
	}
	if n := activeAssets(t, f); n != 0 {
		t.Fatalf("機敏資產被確認：%d", n)
	}
	if id := avatarAssetOf(t, f, p.BuyerID); id != nil {
		t.Fatalf("avatar_asset_id=%v，預期 NULL", id)
	}
}

func activeAssets(t *testing.T, f *fixture) int {
	t.Helper()
	n, err := media.CountAssetsInState(context.Background(), f.database, media.AssetActive)
	must(t, err)
	return n
}

// notActiveAssets 是推導後不是 ACTIVE 的資產數（STAGED、DELETING、DELETED）。
func notActiveAssets(t *testing.T, f *fixture) int {
	t.Helper()
	total := 0
	for _, st := range []media.AssetState{media.AssetStaged, media.AssetDeleting, media.AssetDeleted} {
		n, err := media.CountAssetsInState(context.Background(), f.database, st)
		must(t, err)
		total += n
	}
	return total
}
