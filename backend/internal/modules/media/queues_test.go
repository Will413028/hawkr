package media_test

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/reclaim"
)

func TestQueuesIncludesTheProcessQueue(t *testing.T) {
	if got, ok := media.Queues()[media.ProcessQueue]; !ok || got != media.ProcessQueueWorkers {
		t.Fatalf("Queues() = %v，預期含 %s=%d", media.Queues(), media.ProcessQueue, media.ProcessQueueWorkers)
	}
}

func TestPeriodicJobsRunDeletionApplyAtTheApplyIntervalAndGCAtTheSweepInterval(t *testing.T) {
	if got := len(reclaim.PeriodicJobs()); got != 2 {
		t.Fatalf("PeriodicJobs 有 %d 個，預期 2（media.apply_deletions、media.gc_uploads）", got)
	}
	if opts := (reclaim.ApplyDeletionsArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != reclaim.ApplyInterval {
		t.Fatalf("media.apply_deletions 的唯一性時間窗 = %v，預期 ApplyInterval %v", opts.UniqueOpts.ByPeriod, reclaim.ApplyInterval)
	}
	if (reclaim.ApplyDeletionsArgs{}).Kind() != "media.apply_deletions" || (reclaim.GCArgs{}).Kind() != "media.gc_uploads" {
		t.Fatal("job 種類不符")
	}
	if opts := (reclaim.GCArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != reclaim.SweepInterval {
		t.Fatalf("media.gc_uploads 的唯一性時間窗 = %v，預期 SweepInterval", opts.UniqueOpts.ByPeriod)
	}
}
