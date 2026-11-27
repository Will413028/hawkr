//go:build integration

package buyerconsumer

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func TestDuplicateOrderCompletedEventEnqueuesOneRiverJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	c := Consumer{Jobs: queue}
	for _, body := range [][]byte{
		[]byte(`{"event_id":"evt-1","buyer_id":"71","store_id":"5"}`),
		[]byte(`{"event_id":"evt-1","buyer_id":"71","store_id":"5"}`),
	} {
		if err := c.OrderCompleted(ctx, body); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := d.Pool().QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'legacy.buyer_order_completed'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("重複 event_id 排入 %d 筆，預期 1 筆", count)
	}
}
