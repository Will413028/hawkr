//go:build integration

package worker_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/notification-service/internal/worker"
)

// 單體送出的每個 route（清單讀單體的 manifest，見 relay_routes_test.go）在真的 notification_templates（重放全部 migrations 後）上要成立其一：
// 有 enabled 的 template，而且 notification 訂得到（template 驅動的訂閱的 queue 有綁到它）；或有靜態訂閱處理；或在
// notifyAllowlist（明白不通知，且確實沒有 template）。三者都不是就是新增了一個沒人接的事件；allowlist 的條目一旦有了 template 就過期。
func TestPublishedRoutesHaveNotificationHandler(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTemplateDB(ctx, t)
	defer cleanup()
	templates := loadEnabledTemplateRoutes(ctx, t, pool)

	allow := map[string]worker.NotifyAllowlistEntry{}
	for _, e := range worker.NotifyAllowlist() {
		allow[e.Exchange+"/"+e.Key] = e
	}
	for _, r := range worker.MonolithPublishRoutes(t) {
		id := r.Exchange + "/" + r.Key
		t.Run(id, func(t *testing.T) {
			hasTemplate := templates[id]
			_, allowed := allow[id]
			if hasTemplate && allowed {
				t.Fatalf("%s 已有 enabled 的 template，allowlist 這筆過期：刪掉它", id)
			}
			switch {
			case hasTemplate:
				queue, ok := worker.DynamicQueue(r.Exchange)
				if !ok {
					t.Fatalf("%s 有 template，但 notification 沒有訂 exchange %s（notificationQueueByExchange）", id, r.Exchange)
				}
				if !slices.Contains(worker.TopologyBound(t, r.Exchange, r.Key), queue) {
					t.Fatalf("%s 有 template，但 topology.yaml 沒有把 %s 綁到它", id, queue)
				}
			case allowed:
			default:
				if _, ok := worker.StaticHandles(r.Exchange, r.Key); !ok {
					t.Fatalf("%s（%s）沒有 template、沒有靜態訂閱、也不在 notifyAllowlist：notification 收不到或不處理它。"+
						"要通知就加 template；刻意不通知就加進 allowlist 並寫由誰消費", id, r.Pkg)
				}
			}
		})
	}
}

func loadEnabledTemplateRoutes(ctx context.Context, t *testing.T, pool *pgxpool.Pool) map[string]bool {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT DISTINCT exchange, routing_key FROM notification_templates WHERE enabled`)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var ex, key string
		require.NoError(t, rows.Scan(&ex, &key))
		out[ex+"/"+key] = true
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, out)
	return out
}
