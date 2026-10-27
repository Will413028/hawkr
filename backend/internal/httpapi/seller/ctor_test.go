package seller

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

const testWebOrigin = "https://web.example.test"

func mustWebOrigin(t testing.TB) rpc.WebOrigin {
	t.Helper()
	web, err := rpc.NewWebOrigin(testWebOrigin)
	if err != nil {
		t.Fatal(err)
	}
	return web
}

func mustAgreementURL(t testing.TB, ref string) string {
	t.Helper()
	u, err := mustWebOrigin(t).URL(ref)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// mustApp 與 mustPublic 是測試的建構：StatsSource 是必要的（零值被建構子擋下），其餘照 NewApp、NewPublic。
func mustApp(t testing.TB, m Module, followers Followers, images rpc.Images, dash Dashboard, stats *StatsSource) *App {
	t.Helper()
	h, err := NewApp(m, followers, images, dash, stats)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func mustPublic(t testing.TB, m PublicModule, stats *StatsSource, images rpc.Images) *Public {
	t.Helper()
	h, err := NewPublic(m, stats, images, mustWebOrigin(t))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// StatsSource 只能由 NewStatsSource 取得（指標，nil 不可用）：建構子收到 nil 回錯誤，不是等到 RateCards 對 nil 介面 panic。
func TestConstructorsRejectANilStatsSource(t *testing.T) {
	if _, err := NewApp(&fakeModule{}, nil, rpc.Images{}, Dashboard{}, nil); err == nil {
		t.Error("NewApp 收下了 nil 的 StatsSource")
	}
	if _, err := NewPublic(&fakePublic{}, nil, rpc.Images{}, mustWebOrigin(t)); err == nil {
		t.Error("NewPublic 收下了 nil 的 StatsSource")
	}
	if _, err := NewPublic(&fakePublic{}, testStats(fakeFollowerCounts{}, fakeRatings{}), rpc.Images{}, rpc.WebOrigin{}); err == nil {
		t.Error("NewPublic 收下了沒有設定的網站網域")
	}
	if _, err := NewStatsSource(nil, fakeRatings{}, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Error("NewStatsSource 收下了 nil 的粉絲來源")
	}
}
