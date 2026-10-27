package payuni

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// 永久的失敗（未知的店家）取消 job、不再重試（盤點 D14）；其他錯誤原樣回傳，讓 River 重試（盤點 D9）。
func TestJobErrorCancelsOnlyPermanentFailures(t *testing.T) {
	permanent := fmt.Errorf("%w：沒有 MerID HAWK1 的註冊", ErrPermanent)
	var cancel *river.JobCancelError
	if err := jobError(permanent); !errors.As(err, &cancel) {
		t.Fatalf("permanent failure should cancel the job: %v", err)
	}
	transient := errors.New("database down")
	if err := jobError(transient); !errors.Is(err, transient) || errors.As(err, &cancel) {
		t.Fatalf("transient failure must be returned as is: %v", err)
	}
	if jobError(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

// sync 與輪詢的唯一鍵不含 completed、一定含 River 要求的狀態；webhook 以 event_key 去重。
func TestUniqueStatesExcludeCompletedButKeepRiversRequiredOnes(t *testing.T) {
	for _, opts := range []river.InsertOpts{SyncArgs{}.InsertOpts(), PollArgs{}.InsertOpts()} {
		states := opts.UniqueOpts.ByState
		if !opts.UniqueOpts.ByArgs || slices.Contains(states, rivertype.JobStateCompleted) {
			t.Fatalf("opts=%+v", opts)
		}
		for _, required := range []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
		} {
			if !slices.Contains(states, required) {
				t.Fatalf("缺 River 要求的狀態 %s", required)
			}
		}
	}
	if opts := (ObserveArgs{}).InsertOpts(); !opts.UniqueOpts.ByArgs {
		t.Fatalf("opts=%+v", opts)
	}
}

func TestAddWorkersRegistersEveryJobKind(t *testing.T) {
	workers := river.NewWorkers()
	New(Deps{KYC: noKYC{}}).AddWorkers(workers)
	// 已登記的 kind 再登記一次會失敗：證明四種 job 都登記了。
	for name, add := range map[string]func() error{
		SyncArgs{}.Kind():    func() error { return river.AddWorkerSafely(workers, &syncWorker{}) },
		PollArgs{}.Kind():    func() error { return river.AddWorkerSafely(workers, &pollWorker{}) },
		ObserveArgs{}.Kind(): func() error { return river.AddWorkerSafely(workers, &observeWorker{}) },
		SweepArgs{}.Kind():   func() error { return river.AddWorkerSafely(workers, &sweepWorker{}) },
	} {
		if err := add(); err == nil {
			t.Errorf("%s 沒有登記", name)
		}
	}
	if got := len((&Service{}).PeriodicJobs()); got != 1 {
		t.Fatalf("periodic jobs=%d", got)
	}
}

type noKYC struct{}

func (noKYC) BindingData(context.Context, uuid.UUID) (BindingKYC, error) { return BindingKYC{}, nil }

func (noKYC) CompanyPhone(context.Context, uuid.UUID) (string, error) { return "", nil }
