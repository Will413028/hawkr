package profile

import (
	"reflect"
	"testing"
	"time"
)

// 啟動即執行、每 15 分鐘補跑，同一個週期的排程與重啟合併。
func TestReconcilePeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	if len(periodicJobs) != 1 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	configuration := reflect.ValueOf(periodicJobs[0]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "seller.reconcile_exposure" {
		t.Fatal("reconcile job must run immediately after worker startup")
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != 15*time.Minute {
		t.Fatalf("interval=%s", interval)
	}
	if opts := (ReconcileArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != 15*time.Minute {
		t.Fatalf("uniqueness=%+v", opts.UniqueOpts)
	}
}
