//go:build integration

package sellerrelay

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// relay job 在排入送出的 job 之後才失敗（逾時、worker 當掉）時 River 會重跑整個 relay；同一則事件只能有一個送出的 job，
// 否則下游收到兩次（Java 的消費者、notification 的通知）。unique 只看 event_id（payload 重跑時可能不同：停權原因在兩次之間改了）。
func TestRelayRetryEnqueuesOnePublishJobPerEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	stores := &fakeStores{state: riskStore(seller.RiskFraud)}
	w := NewWorker(stores, queue, nil)
	suspended := StoreSuspendedArgs{storeFields: store(eventFields{EventID: "event-1"}, 91, 7), PeriodID: 55}
	for range 2 {
		if err := w.storeSuspended(ctx, suspended); err != nil {
			t.Fatal(err)
		}
	}
	stores.state = seller.Store{Sanctions: seller.NoSanctions()}
	if err := w.storeSuspended(ctx, suspended); err != nil {
		t.Fatal(err)
	}
	if err := w.storeExposureChanged(ctx, StoreExposureChangedArgs{eventFields: eventFields{EventID: "event-2"}, StoreID: 91, Exposable: true, Version: 1}); err != nil {
		t.Fatal(err)
	}
	rows, err := d.Pool().Query(ctx, `SELECT args->>'event_id', args->>'routing_key' FROM river.river_job WHERE kind = 'legacy.rabbitmq_publish' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got [][2]string
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			t.Fatal(err)
		}
		got = append(got, [2]string{id, key})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != [2]string{"event-1", "seller.suspended"} || got[1] != [2]string{"event-2", "seller.exposure.changed"} {
		t.Fatalf("publish jobs = %v，預期每則事件一個", got)
	}
}
