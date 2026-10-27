package ranking

import (
	"reflect"
	"testing"
	"time"
)

// 重算每 15 分鐘、啟動即執行；同一個間隔內的排程與重啟合併，單次上限小於間隔。
func TestPeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	if len(periodicJobs) != 1 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	configuration := reflect.ValueOf(periodicJobs[0]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "discovery.recompute_seller_scores" {
		t.Fatalf("id=%q runOnStart=%v", options.FieldByName("ID").String(), options.FieldByName("RunOnStart").Bool())
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	interval := time.Duration(schedule.FieldByName("interval").Int())
	if interval != 15*time.Minute || (RecomputeArgs{}).InsertOpts().UniqueOpts.ByPeriod != interval {
		t.Fatalf("interval=%s unique=%s", interval, (RecomputeArgs{}).InsertOpts().UniqueOpts.ByPeriod)
	}
	if recomputeTimeout >= interval {
		t.Fatalf("timeout %s must be shorter than the interval %s", recomputeTimeout, interval)
	}
}
