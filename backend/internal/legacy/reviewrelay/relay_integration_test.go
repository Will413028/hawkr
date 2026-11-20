//go:build integration

package reviewrelay

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// relay job 在排入送出的 job 之後才失敗（逾時、worker 當掉）時 River 會重跑整個 relay；同一則事件只能有一個送出的 job，
// 否則 Java product 與 notification 收到兩次。unique 只看 event_id：統計在兩次之間改變時 payload 不同，仍然只有一個。
func TestRelayRetryEnqueuesOnePublishJobPerEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	w, reviews, _, _ := newFixture()
	w.Jobs = queue

	createdArgs := ReviewCreatedArgs{eventFields: eventFields{EventID: "event-created", Occurred: occurredAt}, ReviewID: reviewID.String()}
	changedArgs := ReviewAggregateChangedArgs{eventFields: eventFields{EventID: "event-changed", Occurred: occurredAt}, StoreID: 91, ProductID: 700}
	for range 2 {
		if err := w.reviewCreated(ctx, createdArgs); err != nil {
			t.Fatal(err)
		}
		if err := w.reviewAggregateChanged(ctx, changedArgs); err != nil {
			t.Fatal(err)
		}
		reviews.product[700] = review.ProductStats{Count: 31, Average: 4.5}
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
	if len(got) != 2 {
		t.Fatalf("publish jobs = %v，預期 2 個（created、changed 各一個）", got)
	}
	if got[0] != [2]string{"event-created", "review.created"} || got[1] != [2]string{"event-changed", "review.hidden"} {
		t.Fatalf("publish jobs = %v", got)
	}
	ids := map[string]bool{}
	for _, g := range got {
		if ids[g[0]] {
			t.Fatalf("event_id 重複：%v", got)
		}
		ids[g[0]] = true
	}
}
