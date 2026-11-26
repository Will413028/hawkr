package catalog

import (
	"context"
	"testing"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type noJobs struct{}

func (noJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

func (noJobs) EnqueueJob(context.Context, river.JobArgs, *river.InsertOpts) (int64, error) {
	return 0, nil
}

func (noJobs) JobState(context.Context, int64) (rivertype.JobState, bool, error) {
	return "", false, nil
}

var _ jobs.Tracker = noJobs{}

// Jobs 與 Restock 缺任何一個，組裝時就 panic，不留到上架或庫存寫入時才 nil 解參考。
func TestNewRequiresJobsAndRestock(t *testing.T) {
	for name, d := range map[string]Deps{
		"沒有 Jobs 與 Restock": {},
		"沒有 Restock":        {Jobs: noJobs{}},
		"沒有 Jobs":           {Restock: stubNotifier{}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s：應該 panic", name)
				}
			}()
			New(d)
		}()
	}
	New(Deps{Jobs: noJobs{}, Restock: stubNotifier{}})
}

type stubNotifier struct{}

func (stubNotifier) NotifyRestocked(context.Context, RestockedSKU) error { return nil }

func (stubNotifier) PurgeSKUSubscriptions(context.Context, []uuid.UUID) error { return nil }
