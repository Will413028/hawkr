package kyc

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// 期限是建立查詢時寫入的固定長度（15 分鐘），不由 River 預設退避的公式反推；掃描提前一個掃描間隔收掉過期的，所以終態一定在期限之前。
func TestRegistryDeadlineIsAFixedWindow(t *testing.T) {
	if registryDeadline != 15*time.Minute {
		t.Fatalf("registryDeadline=%v", registryDeadline)
	}
	requested := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	if got := registryCheckDeadline(requested); !got.Equal(requested.Add(15 * time.Minute)) {
		t.Fatalf("deadline=%v", got)
	}
	// 掃描的間隔遠小於期限，閒置補排在期限內還有機會跑完。
	if registrySweepInterval*3 > registryDeadline || registryResyncIdle >= registryDeadline {
		t.Fatalf("sweep=%v idle=%v deadline=%v", registrySweepInterval, registryResyncIdle, registryDeadline)
	}
}

// job 以 check_id 為唯一鍵，狀態集合包含 River 要求的四個、不含 discarded（被丟棄的 job 之後要能補排）與 completed。
func TestRegistryCheckJobIsUniqueByCheckID(t *testing.T) {
	opts := (RegistryCheckArgs{}).InsertOpts()
	if !opts.UniqueOpts.ByArgs || opts.MaxAttempts != registryMaxAttempts {
		t.Fatalf("opts=%+v", opts)
	}
	states := opts.UniqueOpts.ByState
	for _, required := range []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled} {
		if !slices.Contains(states, required) {
			t.Fatalf("缺 River 要求的狀態 %s", required)
		}
	}
	for _, excluded := range []rivertype.JobState{rivertype.JobStateDiscarded, rivertype.JobStateCompleted, rivertype.JobStateCancelled} {
		if slices.Contains(states, excluded) {
			t.Fatalf("唯一鍵不該涵蓋 %s", excluded)
		}
	}
}

// 掃描是啟動即執行、每 registrySweepInterval 一次的排程；同一段時間內的排程與重啟合併；worker 已登記。
func TestRegistrySweepIsPeriodicAndRegistered(t *testing.T) {
	periodic := (&Service{}).PeriodicJobs()
	if len(periodic) != 1 {
		t.Fatalf("periodic jobs=%d", len(periodic))
	}
	// River 不提供設定讀取 API；只讀 reflection 固定重啟與間隔，不修改其內部值。
	configuration := reflect.ValueOf(periodic[0]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "seller.kyc_registry_sweep" {
		t.Fatalf("id=%q runOnStart=%v", options.FieldByName("ID").String(), options.FieldByName("RunOnStart").Bool())
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != registrySweepInterval {
		t.Fatalf("interval=%v", interval)
	}
	if unique := (RegistrySweepArgs{}).InsertOpts().UniqueOpts.ByPeriod; unique <= 0 || unique > registrySweepInterval {
		t.Fatalf("unique=%v", unique)
	}
	workers := river.NewWorkers()
	(&Service{}).AddWorkers(workers)
	for name, add := range map[string]func() error{
		RegistryCheckArgs{}.Kind(): func() error { return river.AddWorkerSafely(workers, &registryCheckWorker{}) },
		RegistrySweepArgs{}.Kind(): func() error { return river.AddWorkerSafely(workers, &registrySweepWorker{}) },
	} {
		if err := add(); err == nil {
			t.Errorf("%s 沒有登記", name)
		}
	}
}
