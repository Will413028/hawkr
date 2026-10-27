package purge

import (
	"reflect"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 啟動即執行、每小時補跑，同一小時的排程與重啟合併（同 buyer 的清理）。
func TestPeriodicConfiguration(t *testing.T) {
	periodicJobs := New(Deps{}).PeriodicJobs()
	if len(periodicJobs) != 1 {
		t.Fatalf("periodic jobs=%d", len(periodicJobs))
	}
	// River 不提供設定讀取 API；只讀 reflection 固定重啟與間隔，不修改其內部值。
	configuration := reflect.ValueOf(periodicJobs[0]).Elem()
	options := configuration.FieldByName("opts").Elem()
	if !options.FieldByName("RunOnStart").Bool() || options.FieldByName("ID").String() != "seller.purge_pii" {
		t.Fatal("purge must run immediately after worker startup")
	}
	schedule := configuration.FieldByName("scheduleFunc").Elem().Elem()
	if interval := time.Duration(schedule.FieldByName("interval").Int()); interval != time.Hour {
		t.Fatalf("purge interval=%s", interval)
	}
	if opts := (Args{}).InsertOpts(); opts.UniqueOpts.ByPeriod != time.Hour {
		t.Fatalf("uniqueness=%+v", opts.UniqueOpts)
	}
	if model.StoreRetention != 4416*time.Hour || model.StoreRetentionHours != 4416 || model.AppealWindowHours != 4416 {
		t.Fatalf("retention=%s hours=%d appeal=%d", model.StoreRetention, model.StoreRetentionHours, model.AppealWindowHours)
	}
}
