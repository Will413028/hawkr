//go:build integration

package content_test

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func (f *fixture) syncJobs(t *testing.T, ref media.Ref) int {
	t.Helper()
	var n int
	err := f.db.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM river.river_job WHERE kind = 'content.media_sync' AND args->>'ref_id' = $1
		   AND state IN ('available', 'pending', 'running', 'retryable', 'scheduled')`, ref.ID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// 回歸（CR2-4）：同步 job 在「轉正之後、寫入 commit 之前」執行，不得留下業務列指著卻沒有引用的圖。
func TestSyncBetweenFinalizeAndCommitKeepsThePointedAtBannerImages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	f.sync(t, ref)

	f.media.AfterFinalize = func(context.Context, media.Asset) {
		f.media.AfterFinalize = nil
		f.sync(t, ref)
	}
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "p2", "q2"
	if _, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in}); err != nil {
		t.Fatal(err)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-p2 asset-q2]" {
		t.Fatalf("references=%v，預期只剩目前指著的兩張", got)
	}
}

func TestSyncJobIsUniquePerRefAcrossWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "p2", "q2"
	if _, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in}); err != nil {
		t.Fatal(err)
	}
	if n := f.syncJobs(t, ref); n != 1 {
		t.Fatalf("同一個 ref 的同步 job = %d，預期 1", n)
	}
}

func TestSyncRunsAgainWhenTheBannerChangesDuringTheRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	f.media.BeforeApply = func(_ context.Context, op string, _ media.Ref, _ []string) {
		if op != "link" {
			return
		}
		f.media.BeforeApply = nil
		in := bannerInput()
		in.PreviewImageToken, in.PopupImageToken = "p2", "q2"
		if _, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in}); err != nil {
			t.Error(err)
		}
	}
	res := f.sync(t, ref)
	if !res.Again || res.After != media.SyncResyncDelay {
		t.Fatalf("同步期間業務列改變，要再跑一輪：%+v", res)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-p2 asset-q2]" {
		t.Fatalf("references=%v", got)
	}
}

// asset ID 與 key 成對、圖片必填：寫入端永遠兩者一起寫，資料庫拒絕只有 key 沒有 asset 的列。
func TestImageKeysAndAssetIDsArePaired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	if _, err := f.db.Pool().Exec(ctx, `INSERT INTO content.banners (id, title, preview_image_key, popup_image_key, action_type) VALUES ($1, 't', 'k', 'k', 'popup')`, uuid.NewV7()); err == nil {
		t.Fatal("沒有 asset ID 的 banner 列應該被拒絕")
	}
}

// 回歸：預覽與彈窗用同一個 asset（同一個 token 放兩個欄位），只換其中一張：舊的 asset 仍被另一張指著，不能退場，
// 同步要登記新圖、保留舊圖。
func TestSharedAssetAcrossBannerColumnsIsNotRetiredWhileStillPointedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "same", "same"
	b, err := f.m.CreateBanner(ctx, adminID, in)
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-same]" {
		t.Fatalf("references=%v", got)
	}
	in.PreviewImageToken, in.PopupImageToken = "new", ""
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if res := f.sync(t, ref); res.Again && res.After == media.SyncResyncDelay {
		t.Fatalf("同步不該因版本改變重跑：%+v", res)
	}
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-new asset-same]" {
		t.Fatalf("references=%v，預期新圖與仍被彈窗指著的舊圖", got)
	}
	var tombstones int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM content.retired_assets WHERE ref_id = $1`, ref.ID).Scan(&tombstones); err != nil || tombstones != 0 {
		t.Fatalf("仍被指著的資產不得有退場紀錄：%d %v", tombstones, err)
	}
	// 再換掉彈窗：舊圖這時才退場。
	in.PreviewImageToken, in.PopupImageToken = "", "new2"
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-new asset-new2]" {
		t.Fatalf("references=%v", got)
	}
	// 刪除：兩個欄位指著同一個資產時只退場一次。
	if err := f.m.DeleteBanner(ctx, b.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("references=%v", got)
	}
}

// startWorker 以真的 River 執行同步 job（結算與完成同一個 transaction 要靠真的 River 才驗得到）。
func (f *fixture) startWorker(t *testing.T) {
	t.Helper()
	workers := river.NewWorkers()
	f.m.AddWorkers(workers)
	c, err := jobs.NewWorker(f.db, workers, map[string]int{river.QueueDefault: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

// quiescent 等到 ref 的同步完成：版本等於已同步、沒有排隊或執行中的 job（排程中的是第二次解除的 snooze：測試的時鐘是凍結的，
// 解除時間永遠在 HealDelay 之內，所以它會一直排下一輪，這裡不等它）。
func (f *fixture) quiescent(t *testing.T, ref media.Ref) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		var unsynced, open int
		if err := f.db.Pool().QueryRow(context.Background(),
			`SELECT count(*) FROM content.media_ref_sync WHERE ref_id = $1 AND synced_seq < seq`, ref.ID).Scan(&unsynced); err != nil {
			t.Fatal(err)
		}
		if err := f.db.Pool().QueryRow(context.Background(),
			`SELECT count(*) FROM river.river_job WHERE kind = 'content.media_sync' AND args->>'ref_id' = $1
			   AND state NOT IN ('completed', 'discarded', 'cancelled', 'scheduled')`, ref.ID).Scan(&open); err != nil {
			t.Fatal(err)
		}
		if unsynced == 0 && open == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	rows, _ := f.db.Pool().Query(context.Background(), `SELECT state, attempt, coalesce(errors::text,'') FROM river.river_job WHERE kind='content.media_sync'`)
	for rows.Next() {
		var st, er string
		var at int
		_ = rows.Scan(&st, &at, &er)
		t.Logf("job state=%s attempt=%d errors=%s", st, at, er)
	}
	var seq, synced int64
	_ = f.db.Pool().QueryRow(context.Background(), `SELECT seq, synced_seq FROM content.media_ref_sync WHERE ref_id=$1`, ref.ID).Scan(&seq, &synced)
	t.Logf("seq=%d synced=%d", seq, synced)
	t.Fatalf("同步沒有在期限內完成：%+v", ref)
}

// 結算與完成同一個 transaction：job 在 River 裡正常完成（沒有 JobCompleteTx 的錯誤、沒有卡住），版本已同步，引用正確。
func TestSyncJobCompletesInTheSettleTransactionWithRealRiver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.startWorker(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	f.quiescent(t, ref)
	var state string
	if err := f.db.Pool().QueryRow(ctx, `SELECT state FROM river.river_job WHERE kind = 'content.media_sync' AND args->>'ref_id' = $1`, ref.ID).Scan(&state); err != nil || state != "completed" {
		t.Fatalf("job state=%q err=%v", state, err)
	}
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-p1 asset-q1]" {
		t.Fatalf("references=%v", got)
	}
}

// 寫入與執行中的同步交錯（寫入端鎖序：版本列、再 river_job 的唯一索引；結算同一個順序）：連續換圖不死鎖，最後沒有遺失的
// 更新，引用等於目前指著的資產。
func TestSyncStaysCorrectWhileWritesInterleaveWithRealRiver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	f.startWorker(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	for i := range 12 {
		in := bannerInput()
		in.PreviewImageToken, in.PopupImageToken = fmt.Sprintf("p%d", i+10), fmt.Sprintf("q%d", i+10)
		b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
		if err != nil {
			t.Fatal(err)
		}
		if i%3 == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	f.quiescent(t, ref)
	want := fmt.Sprintf("[asset-p%d asset-q%d]", 21, 21)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != want {
		t.Fatalf("references=%v，預期 %s", got, want)
	}
}

// 保底掃描：版本比已同步的新、job 卻不存在（遺失的排入）→ 掃描補排，真的 River 執行後同步完成。
func TestSweepRecoversALostEnqueueWithRealRiver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	if _, err := f.db.Pool().Exec(ctx, `DELETE FROM river.river_job WHERE kind = 'content.media_sync'`); err != nil {
		t.Fatal(err)
	}
	if n := f.syncJobs(t, ref); n != 0 {
		t.Fatalf("job 應該已經遺失：%d", n)
	}
	f.now = f.now.Add(media.SyncSweepIdle + time.Minute)
	n, err := f.m.SweepMedia(ctx)
	if err != nil || n != 1 {
		t.Fatalf("掃描補排 n=%d err=%v", n, err)
	}
	if got := f.syncJobs(t, ref); got != 1 {
		t.Fatalf("補排的 job = %d，預期 1", got)
	}
	if n, err = f.m.SweepMedia(ctx); err != nil || f.syncJobs(t, ref) != 1 {
		t.Fatalf("重複掃描不得重複排入（唯一鍵）：n=%d jobs=%d err=%v", n, f.syncJobs(t, ref), err)
	}
	f.startWorker(t)
	f.quiescent(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-p1 asset-q1]" {
		t.Fatalf("references=%v", got)
	}
}
