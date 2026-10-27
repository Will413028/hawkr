package deletion

import (
	"reflect"
	"testing"
	"time"
)

func TestCleanupPeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	if len(periodicJobs) != 1 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	// River 不提供設定讀取 API；只讀 reflection 固定重啟與間隔，不修改其內部值。
	configuration := reflect.ValueOf(periodicJobs[0]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "buyer.cleanup_pii" {
		t.Fatal("cleanup must run immediately after worker startup")
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != time.Hour {
		t.Fatalf("cleanup interval=%s", interval)
	}
	if options := (CleanupArgs{}).InsertOpts(); options.UniqueOpts.ByPeriod != time.Hour {
		t.Fatalf("cleanup uniqueness=%+v", options.UniqueOpts)
	}
}
