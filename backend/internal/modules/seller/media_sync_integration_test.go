//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 回歸（CR2-4）：同步 job 在「轉正之後、寫入 commit 之前」執行，不得留下店家列指著卻沒有引用的圖。
func TestSyncBetweenFinalizeAndCommitKeepsTheStoreImages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	logoRef := f.storeRef(media.RefTypeSellerLogo, s.StoreID)
	first, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID})
	must(t, err)
	f.syncMedia(t, logoRef)

	f.media.onFinalize = func() {
		f.syncMedia(t, logoRef)
	}
	second, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: first.Version, LogoToken: "logo2", UploaderID: accountID})
	must(t, err)
	f.syncMedia(t, logoRef)
	if got := f.media.Referenced(logoRef); fmt.Sprint(got) != "[asset-logo2]" {
		t.Fatalf("references=%v，預期只剩目前指著的 asset-logo2", got)
	}
	// 被換掉的 logo 不能寫回：同一個 staged token 重新轉正會回到同一個資產。
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: second.Version, LogoToken: "logo1", UploaderID: accountID})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("retired logo written back: %v", err)
	}
	current, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if current.Version != second.Version || *current.LogoKey != *second.LogoKey {
		t.Fatalf("被拒絕的寫入改了店家：%+v", current)
	}
}

// 回歸（CR2-4）：業者換圖。
func TestSyncBetweenFinalizeAndCommitKeepsTheProspectImage(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", nil)
	ref := f.prospectRef(media.RefTypeSellerProspectLogo, p.ID)
	first, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: "one"}})
	must(t, err)
	f.syncMedia(t, ref)

	f.media.onFinalize = func() { f.syncMedia(t, ref) }
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: first.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: "two"}})
	must(t, err)
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-two]" {
		t.Fatalf("references=%v", got)
	}
}

func TestSyncRunsAgainWhenTheStoreImageChangesDuringTheRun(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	logoRef := f.storeRef(media.RefTypeSellerLogo, s.StoreID)
	first, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID})
	must(t, err)
	f.media.BeforeApply = func(_ context.Context, op string, _ media.Ref, _ []string) {
		if op != "link" {
			return
		}
		f.media.BeforeApply = nil
		_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: first.Version, LogoToken: "logo2", UploaderID: accountID})
		must(t, err)
	}
	res := f.syncMedia(t, logoRef)
	if !res.Again || res.After != media.SyncResyncDelay {
		t.Fatalf("同步期間業務列改變，要再跑一輪：%+v", res)
	}
	f.syncMedia(t, logoRef)
	if got := f.media.Referenced(logoRef); fmt.Sprint(got) != "[asset-logo2]" {
		t.Fatalf("references=%v", got)
	}
}

// 同步 job 只解除退場的資產：同一個 ref 下別的資產（這裡是別人另外登記的）不受影響。
func TestSyncNeverUnlinksAnythingButRetiredAssets(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	logoRef := f.storeRef(media.RefTypeSellerLogo, s.StoreID)
	first, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID})
	must(t, err)
	must(t, f.media.Link(ctx, logoRef, []string{"asset-bystander"}))
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: first.Version, LogoToken: "logo2", UploaderID: accountID})
	must(t, err)
	f.syncMedia(t, logoRef)
	if got := f.media.Referenced(logoRef); fmt.Sprint(got) != "[asset-bystander asset-logo2]" {
		t.Fatalf("references=%v：不在退場名單的資產不得被解除", got)
	}
}
