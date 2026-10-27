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

func TestDuplicateRestockEventEnqueuesOneRiverJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	c := Consumer{Jobs: queue}
	// payload 照上游 product-service 的 ProductRestockPayload（services/java/product-service/.../mq/dto/ProductRestockPayload.java）：
	// SnakeCase、product_id 是字串、store_id 是數字、occurred_at 是帶 Z 的 ISO 8601 UTC。
	for _, body := range [][]byte{
		[]byte(`{"event_id":"evt-1","event_type":"PRODUCT_RESTOCK","product_id":"9","product_name":"商品 A","image_key":"products/9/main.jpg","store_id":3,"occurred_at":"2026-09-30T01:02:03.456Z"}`),
		[]byte(`{"event_id":"evt-1","event_type":"PRODUCT_RESTOCK","product_id":"9","product_name":"更新後的名稱","image_key":"products/9/main.jpg","store_id":3,"occurred_at":"2026-09-30T01:02:04Z"}`),
	} {
		if err := c.ProductRestock(ctx, body); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := d.Pool().QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind = 'legacy.buyer_restock'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("重複 event_id 排入 %d 筆，預期 1 筆", count)
	}
}
