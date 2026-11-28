package media_test

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/reclaim"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/upload"
)

func TestQueuesIncludesTheProcessQueue(t *testing.T) {
	if got, ok := media.Queues()[media.ProcessQueue]; !ok || got != media.ProcessQueueWorkers {
		t.Fatalf("Queues() = %v，預期含 %s=%d", media.Queues(), media.ProcessQueue, media.ProcessQueueWorkers)
	}
}

func TestPeriodicJobsRunDeletionApplyAtTheApplyIntervalAndGCAtTheSweepInterval(t *testing.T) {
	if got := len(reclaim.PeriodicJobs()); got != 3 {
		t.Fatalf("PeriodicJobs 有 %d 個，預期 3（media.apply_deletions、media.gc_uploads、media.expire_holds）", got)
	}
	if opts := (reclaim.ApplyDeletionsArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != reclaim.ApplyInterval {
		t.Fatalf("media.apply_deletions 的唯一性時間窗 = %v，預期 ApplyInterval %v", opts.UniqueOpts.ByPeriod, reclaim.ApplyInterval)
	}
	if (reclaim.ApplyDeletionsArgs{}).Kind() != "media.apply_deletions" || (reclaim.GCArgs{}).Kind() != "media.gc_uploads" {
		t.Fatal("job 種類不符")
	}
	if (reclaim.ExpireHoldsArgs{}).Kind() != "media.expire_holds" || (upload.ProcessCopiesArgs{}).Kind() != "media.copy_to_secure" {
		t.Fatal("保全整理與副本複製的 job 種類不符")
	}
	if got := len(upload.PeriodicJobs()); got != 1 {
		t.Fatalf("upload.PeriodicJobs 有 %d 個，預期 1（media.copy_to_secure）", got)
	}
	if opts := (upload.ProcessCopiesArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != upload.CopyInterval {
		t.Fatalf("media.copy_to_secure 的唯一性時間窗 = %v，預期 CopyInterval", opts.UniqueOpts.ByPeriod)
	}
	if opts := (reclaim.ExpireHoldsArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != reclaim.ExpireHoldsInterval {
		t.Fatalf("media.expire_holds 的唯一性時間窗 = %v，預期 ExpireHoldsInterval", opts.UniqueOpts.ByPeriod)
	}
	if opts := (reclaim.GCArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != reclaim.SweepInterval {
		t.Fatalf("media.gc_uploads 的唯一性時間窗 = %v，預期 SweepInterval", opts.UniqueOpts.ByPeriod)
	}
}
