package measure

import (
	"reflect"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 結案掃描啟動即執行、每 5 分鐘補跑，同一個週期的排程與重啟合併。
func TestCloseEndedPeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	if len(periodicJobs) != 2 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	configuration := reflect.ValueOf(periodicJobs[0]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "seller.measure_close_ended_requests" {
		t.Fatal("the sweep must run immediately after worker startup")
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != 5*time.Minute {
		t.Fatalf("interval=%s", interval)
	}
	if opts := (CloseEndedArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != 5*time.Minute {
		t.Fatalf("uniqueness=%+v", opts.UniqueOpts)
	}
}

// 過期內容快照的清空啟動即執行、每日補跑，同一個時段的排程與重啟合併；保留期是 184 天。
func TestPurgeContentSnapshotsPeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	configuration := reflect.ValueOf(periodicJobs[1]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "seller.purge_content_snapshots" {
		t.Fatal("the purge must run immediately after worker startup")
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != 24*time.Hour {
		t.Fatalf("interval=%s", interval)
	}
	if opts := (PurgeContentSnapshotsArgs{}).InsertOpts(); opts.UniqueOpts.ByPeriod != 12*time.Hour {
		t.Fatalf("uniqueness=%+v", opts.UniqueOpts)
	}
	if model.ContentSnapshotRetention != 184*24*time.Hour {
		t.Fatalf("retention=%s", model.ContentSnapshotRetention)
	}
}
