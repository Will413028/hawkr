//go:build integration

package characterization

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// monolith 起單體完整的 HTTP 層（自己的空 database，handler 與 production 一樣由 httpapi.NewHandler 組出）；media 是真的模組
// 加記憶體的物件儲存與測試用的 KEK 金鑰環。呼叫者的身分是 identity 的 dev session：每個買家一個新帳號。
func monolith(t *testing.T) *connectDriver {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
	))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	auth := ch.DevAuth(t)
	id := identity.New(identity.Deps{
		DB: d, Events: events.NewBus(), Providers: auth.Providers,
		Now: time.Now, Logger: log,
	})
	del := delegation.New(delegation.Deps{DB: d, Identity: id, Seller: seller.NewReader(d, time.Now), Social: &social.Module{}, Now: time.Now, Logger: log})
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	kek := base64.StdEncoding.EncodeToString(make([]byte, 32))
	keyring, err := media.ParseKeyring("characterize:" + kek)
	if err != nil {
		t.Fatal(err)
	}
	mediaModule := media.New(media.Deps{
		DB: d, Jobs: jc, Store: media.NewMemoryStore(), NewID: uuid.NewV7, Now: time.Now, Logger: log, Keyring: keyring,
	})
	h, err := httpapi.NewHandler(httpapi.Deps{
		Ready: d.Ping, Identity: id, Delegation: del, Media: mediaModule, Logger: log,
		// NewHandler 要求的其他模組：特性測試只打 media 的端點，這些以零值或替身佔位，不會被呼叫。
		Buyer: &buyer.Module{}, Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}, Discovery: &discovery.Module{},
		ReviewQueue: &reviewqueue.Module{}, Payment: &payment.Module{}, Moderation: &moderation.Module{}, Social: &social.Module{}, Stats: ch.Stats(t), WebOrigin: ch.WebOrigin(t), Images: ch.Images(t), DashboardOrders: noDashboardOrders{},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	tokens := map[ch.Caller]string{ch.Buyer: ch.BuyerSession(t, id), ch.Buyer2: ch.BuyerSession(t, id)}
	return newConnectDriver(srv.Client(), srv.URL, ch.Bearer(tokens))
}

// TestMonolith 對單體跑 B1-1 同一份情境（D8）：REST 對 Java 的情境與 Connect 對單體的是同一個 scenarios 清單。
func TestMonolith(t *testing.T) {
	ch.Run[Driver](t, monolith(t), scenarios)
}

// TestMonolithSecure 跑只有單體測得到的機敏分段上傳情境（B1-D12 的規格由 test-map.md 第二節給）。
func TestMonolithSecure(t *testing.T) {
	ch.Run[MonolithDriver](t, monolith(t), monolithScenarios)
}

type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}
