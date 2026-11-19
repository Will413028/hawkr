package seller

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// 沒有指定封鎖來源的測試：來源都在，但不會被呼叫（呼叫就是測試寫錯了）。
type unusedBlocking struct{}

func (unusedBlocking) ContentAuthorFor(context.Context, social.Caller, social.Target) (social.ContentAuthor, error) {
	panic("unused")
}
func (unusedBlocking) AccountIDByBuyerID(context.Context, int64) (int64, error) { panic("unused") }
func (unusedBlocking) Store(context.Context, int64) (model.Store, error)        { panic("unused") }

func unusedSources() blocks.Sources {
	return blocks.Sources{Authors: unusedBlocking{}, Identities: blocks.Identities{Buyers: unusedBlocking{}, Stores: unusedBlocking{}}}
}

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
	h, err := NewApp(m, &paymentModule{}, followers, images, dash, stats, fakeLinks{}, mustWebOrigin(t), unusedSources(), nil)
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
	if _, err := NewApp(&fakeModule{}, &paymentModule{}, nil, rpc.Images{}, Dashboard{}, nil, fakeLinks{}, mustWebOrigin(t), unusedSources(), nil); err == nil {
		t.Error("NewApp 收下了 nil 的 StatsSource")
	}
	if _, err := NewApp(&fakeModule{}, nil, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}), fakeLinks{}, mustWebOrigin(t), unusedSources(), nil); !errors.Is(err, ErrPaymentsRequired) {
		t.Errorf("NewApp 收下了 nil 的收款帳戶來源：%v", err)
	}
	if _, err := NewApp(&fakeModule{}, &paymentModule{}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}), fakeLinks{}, rpc.WebOrigin{}, unusedSources(), nil); !errors.Is(err, ErrWebOriginRequired) {
		t.Errorf("NewApp 收下了零值的網站網域：%v", err)
	}
	if _, err := NewApp(&fakeModule{}, &paymentModule{}, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}), fakeLinks{}, mustWebOrigin(t), blocks.Sources{}, nil); !errors.Is(err, ErrBlockingRequired) {
		t.Errorf("NewApp 收下了缺的封鎖來源：%v", err)
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
