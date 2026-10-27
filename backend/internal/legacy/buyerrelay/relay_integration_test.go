//go:build integration

package buyerrelay

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// relay job 在排入 publish job 之後才失敗（逾時、worker 當掉）時 River 會重跑整個 relay；同一則事件只能有一個送出的
// job，否則賣家收到兩則推播、瀏覽數重複計。payload 在重跑時可能不同（名稱在兩次之間改了），所以 unique 只看 event_id。
func TestRelayRetryEnqueuesOnePublishJobPerEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorker(relayBuyers{}, &relayProducts{}, relayStores{}, queue, nil)
	for _, account := range []int64{99, 100} {
		if err := w.buyerDeleted(ctx, BuyerDeletedArgs{eventFields: fields("event-1"), AccountID: account}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.favoriteAdded(ctx, FavoriteAddedArgs{eventFields: fields("event-2"), ProductID: 11}); err != nil {
		t.Fatal(err)
	}
	rows, err := d.Pool().Query(ctx, `SELECT args->>'event_id' FROM river.river_job WHERE kind = 'legacy.rabbitmq_publish' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "event-1" || got[1] != "event-2" {
		t.Fatalf("publish jobs = %v，預期每則事件一個（event-1、event-2）", got)
	}
}
