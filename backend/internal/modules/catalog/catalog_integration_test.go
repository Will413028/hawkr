//go:build integration

package catalog

import (
	"context"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// setup 給這支測試一個套用了本模組與它依賴的 migration（含 River 的表）的 database：integration 測試只跑在自己的 schema 上。
// 庫存增加時 catalog 讀 seller 的 pub_exposable_stores 判斷可不可買，所以 seller 與 River 的表一定要在。獨立的測試開頭呼叫 t.Parallel()。
func setup(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: Migrations()},
	))
}

// recordingNotifier 記下 catalog 的 job 呼叫補貨 port 的每一次呼叫。
type recordingNotifier struct {
	mu       sync.Mutex
	notified []RestockedSKU
	purged   [][]uuid.UUID
	// onNotify 在通知呼叫內執行（在 mu 之外），模擬 job 執行中發生的並行寫入。
	onNotify func()
}

func (n *recordingNotifier) NotifyRestocked(_ context.Context, sku RestockedSKU) error {
	n.mu.Lock()
	n.notified = append(n.notified, sku)
	hook := n.onNotify
	n.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func (n *recordingNotifier) PurgeSKUSubscriptions(_ context.Context, ids []uuid.UUID) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.purged = append(n.purged, ids)
	return nil
}

func (n *recordingNotifier) skus() []uuid.UUID {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []uuid.UUID
	for _, s := range n.notified {
		out = append(out, s.SKUID)
	}
	return out
}

func newJobs(t *testing.T, d *db.DB) *jobs.Client {
	t.Helper()
	c, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
