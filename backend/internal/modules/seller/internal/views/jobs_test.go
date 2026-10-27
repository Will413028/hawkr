package views

import (
	"reflect"
	"testing"
	"time"
)

// 彙總每 5 分鐘、清理每小時，都啟動即執行；同一個間隔內的排程與重啟合併。
func TestPeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	if len(periodicJobs) != 2 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	for i, want := range []struct {
		id       string
		interval time.Duration
		unique   time.Duration
	}{
		{"seller.product_views_rollup", 5 * time.Minute, (RollupArgs{}).InsertOpts().UniqueOpts.ByPeriod},
		{"seller.product_views_purge", time.Hour, (PurgeArgs{}).InsertOpts().UniqueOpts.ByPeriod},
	} {
		// River 不提供設定讀取 API；只讀 reflection 固定重啟與間隔，不修改其內部值。
		configuration := reflect.ValueOf(periodicJobs[i]).Elem()
		options := configuration.FieldByName("opts").Elem()
		if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != want.id {
			t.Fatalf("job %d: id=%q runOnStart=%v", i, options.FieldByName("ID").String(), options.FieldByName("RunOnStart").Bool())
		}
		schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
		if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != want.interval || want.unique != want.interval {
			t.Fatalf("job %s: interval=%s unique=%s, want %s for both", want.id, interval, want.unique, want.interval)
		}
	}
}
