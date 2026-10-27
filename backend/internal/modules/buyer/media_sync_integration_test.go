//go:build integration

package buyer_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func avatarRef(buyerID int64) media.Ref {
	return media.Ref{Type: media.RefTypeBuyerAvatar, ID: fmt.Sprint(buyerID)}
}

func syncAvatar(t *testing.T, f *fixture, buyerID int64) media.SyncResult {
	t.Helper()
	res, err := f.module.SyncMedia(context.Background(), avatarRef(buyerID))
	must(t, err)
	return res
}

func setAvatar(t *testing.T, f *fixture, p Profile, token string) Profile {
	t.Helper()
	updated, err := f.module.UpdateProfile(context.Background(), UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   p.Version,
		AvatarStagedToken: &token,
	})
	must(t, err)
	return updated
}

// 回歸（CR2-4）：同步 job 在「轉正之後、寫入 commit 之前」執行，不得留下業務列指著卻沒有引用的資產。
func TestSyncBetweenFinalizeAndCommitKeepsThePointedAtAsset(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 50)
	cur := setAvatar(t, f, p, "a")
	syncAvatar(t, f, p.BuyerID)
	ref := avatarRef(p.BuyerID)

	f.media.AfterFinalize = func(context.Context, media.Asset) {
		f.media.AfterFinalize = nil
		syncAvatar(t, f, p.BuyerID)
	}
	next := setAvatar(t, f, cur, "b")
	if next.Avatar == nil || *next.Avatar != "avatar/b" {
		t.Fatalf("avatar=%v", next.Avatar)
	}
	syncAvatar(t, f, p.BuyerID)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-b]" {
		t.Fatalf("references=%v，預期只剩 asset-b", got)
	}
}

func TestWriteDoesNotCreateReferencesAndSyncReplacesExactly(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 51)
	ref := avatarRef(p.BuyerID)
	cur := setAvatar(t, f, p, "a")
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("寫入不得建立引用：%v", got)
	}
	syncAvatar(t, f, p.BuyerID)
	must(t, f.media.Link(context.Background(), media.Ref{Type: ref.Type, ID: "other"}, []string{"asset-a"}))
	setAvatar(t, f, cur, "b")
	syncAvatar(t, f, p.BuyerID)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-b]" {
		t.Fatalf("references=%v", got)
	}
	if !f.media.HasRef(media.Ref{Type: ref.Type, ID: "other"}, "asset-a") {
		t.Fatal("解除只能動這個 ref 的退場資產")
	}
}

func TestRetiredAvatarCannotBeWrittenBack(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 52)
	cur := setAvatar(t, f, p, "a")
	cur = setAvatar(t, f, cur, "b")
	// 同一個 staged token 重新轉正會回到同一個資產：已被換掉，不能再寫回。
	_, err := f.module.UpdateProfile(context.Background(), UpdateProfileInput{
		BuyerID:           p.BuyerID,
		ExpectedVersion:   cur.Version,
		AvatarStagedToken: ptr("a"),
	})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v，預期 InvalidInput", err)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "AvatarStagedToken" {
		t.Fatalf("被拒絕的重新轉正要指到 AvatarStagedToken：%v", err)
	}
	got, err := f.module.Profile(context.Background(), p.AccountID)
	must(t, err)
	if got.Avatar == nil || *got.Avatar != "avatar/b" || got.Version != cur.Version {
		t.Fatalf("被拒絕的寫入改了資料：%+v", got)
	}
}

func TestLateLinkAfterUnlinkIsHealedByTheNextSync(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 53)
	ref := avatarRef(p.BuyerID)
	cur := setAvatar(t, f, p, "a")

	f.media.Zombie("link", true)
	if _, err := f.module.SyncMedia(context.Background(), ref); err == nil {
		t.Fatal("Link 逾時應回錯誤")
	}
	f.media.Zombie("link", false)
	cur = setAvatar(t, f, cur, "b")
	res := syncAvatar(t, f, p.BuyerID)
	if !res.Again || res.After != media.SyncHealDelay {
		t.Fatalf("第一次解除之後要再解除一次：%+v", res)
	}
	f.media.FlushZombies()
	if !f.media.HasRef(ref, "asset-a") {
		t.Fatal("逾時的 Link 在解除之後才生效，這是要被修復的狀態")
	}
	f.now = f.now.Add(media.SyncHealDelay + time.Second)
	res = syncAvatar(t, f, p.BuyerID)
	if res.Again {
		t.Fatalf("第二次解除之後不需要再跑：%+v", res)
	}
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-b]" {
		t.Fatalf("references=%v", got)
	}
	_ = cur
}

func TestSyncRunsAgainWhenTheBuyerChangesDuringTheRun(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 54)
	ref := avatarRef(p.BuyerID)
	cur := setAvatar(t, f, p, "a")
	f.media.BeforeApply = func(_ context.Context, op string, _ media.Ref, _ []string) {
		if op != "link" {
			return
		}
		f.media.BeforeApply = nil
		cur = setAvatar(t, f, cur, "b")
	}
	res := syncAvatar(t, f, p.BuyerID)
	if !res.Again || res.After != media.SyncResyncDelay {
		t.Fatalf("同步期間業務列改變，要再跑一輪：%+v", res)
	}
	res = syncAvatar(t, f, p.BuyerID)
	if res.Again && res.After == media.SyncResyncDelay {
		t.Fatalf("第二輪不該再因為版本改變而重跑：%+v", res)
	}
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-b]" {
		t.Fatalf("references=%v", got)
	}
}

func TestClosingRetiresTheAvatarAndSyncUnlinksIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 55)
	ref := avatarRef(p.BuyerID)
	setAvatar(t, f, p, "a")
	syncAvatar(t, f, p.BuyerID)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-a]" {
		t.Fatalf("references=%v", got)
	}
	must(t, f.module.CloseForDeletion(context.Background(), p.BuyerID))
	syncAvatar(t, f, p.BuyerID)
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("關閉之後引用應全部解除：%v", got)
	}
}

func TestTombstonesAreKeptUntilUnlinkedAndOlderThanRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 56)
	cur := setAvatar(t, f, p, "a")
	setAvatar(t, f, cur, "b")
	syncAvatar(t, f, p.BuyerID)
	tombstones := func() int64 {
		return count(t, f, "SELECT count(*) FROM buyer.retired_assets WHERE ref_id=$1", fmt.Sprint(p.BuyerID))
	}
	if tombstones() != 1 {
		t.Fatal("退場紀錄應保留")
	}
	f.now = f.now.Add(media.SyncHealDelay + time.Second)
	syncAvatar(t, f, p.BuyerID)
	_, err := f.module.SweepMedia(context.Background())
	must(t, err)
	if tombstones() != 1 {
		t.Fatal("沒過保留期不得清掉：同一個 token 還能重新轉正")
	}
	f.now = f.now.Add(media.TombstoneRetention)
	syncAvatar(t, f, p.BuyerID)
	if tombstones() != 1 {
		t.Fatal("清除只在保底掃描，同步本身不清")
	}
	_, err = f.module.SweepMedia(context.Background())
	must(t, err)
	if tombstones() != 0 {
		t.Fatal("過了保留期且已解除的退場紀錄要由掃描清掉")
	}
}

func TestSyncJobIsUniquePerRefIncludingRunning(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 57)
	setAvatar(t, f, p, "a")
	if len(f.jobs.opts) != 1 || f.jobs.opts[0] == nil || !f.jobs.opts[0].UniqueOpts.ByArgs {
		t.Fatalf("同步 job 必須以 ref 去重：%+v", f.jobs.opts)
	}
	states := f.jobs.opts[0].UniqueOpts.ByState
	var hasRunning bool
	for _, s := range states {
		hasRunning = hasRunning || s == "running"
	}
	if !hasRunning {
		t.Fatalf("唯一鍵必須涵蓋執行中：%v", states)
	}
}

// 遺失的排入（寫入 commit 了、同步 job 卻沒有排上，或重試用盡被丟棄）由保底掃描補排。
func TestSweepRecoversALostEnqueue(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 58)
	ref := avatarRef(p.BuyerID)
	setAvatar(t, f, p, "a")
	f.jobs.args, f.jobs.opts = nil, nil // 排入的 job 遺失
	f.jobs.outsideOK = true
	// 剛變動的 ref 不補排（給正常的 job 先處理的時間）。
	n, err := f.module.SweepMedia(context.Background())
	must(t, err)
	if n != 0 || len(f.jobs.args) != 0 {
		t.Fatalf("閒置不夠久就補排：n=%d", n)
	}
	f.now = f.now.Add(media.SyncSweepIdle + time.Minute)
	n, err = f.module.SweepMedia(context.Background())
	must(t, err)
	if n != 1 || len(f.jobs.args) != 1 || f.jobs.args[0].Kind() != "buyer.media_sync" {
		t.Fatalf("掃描要補排遺失的同步：n=%d jobs=%v", n, f.jobs.args)
	}
	syncAvatar(t, f, p.BuyerID)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-a]" {
		t.Fatalf("references=%v", got)
	}
	f.jobs.args = nil
	if n, err = f.module.SweepMedia(context.Background()); err != nil || n != 0 {
		t.Fatalf("同步完成之後不再補排：n=%d err=%v", n, err)
	}
}

// 不變式被破壞（目前指著的資產同時是退場）：仍然 Link 目前的、不解除重疊的資產、完成，不是回錯誤重試幾週。
func TestOverlapLinksCurrentAssetsSkipsTheOverlapAndCompletes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := profile(t, f, 59)
	ref := avatarRef(p.BuyerID)
	setAvatar(t, f, p, "a")
	_, err := f.database.Conn(context.Background()).Exec(context.Background(),
		`INSERT INTO buyer.retired_assets (ref_type, ref_id, asset_id, retired_at) VALUES ($1, $2, 'asset-a', $3)`, ref.Type, ref.ID, f.now)
	must(t, err)
	res, err := f.module.SyncMedia(context.Background(), ref)
	if err != nil {
		t.Fatalf("重疊不該讓同步失敗：%v", err)
	}
	if res.Again {
		t.Fatalf("重疊的資產不解除，也不需要再解除一次：%+v", res)
	}
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-a]" {
		t.Fatalf("目前指著的資產要被登記且不被解除：%v", got)
	}
}
