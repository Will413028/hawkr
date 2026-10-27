package buyer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type noFollowers struct{}

func (noFollowers) SubscriberCounts(context.Context, []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

// testStats 建立測試用的 StatsSource（來源與 logger 都是必要的；買家清單只用到評價）。
func testStats(ratings httpseller.Ratings) *httpseller.StatsSource {
	s, err := httpseller.NewStatsSource(noFollowers{}, ratings, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	return s
}

// countedRatings 是評價來源的替身：記錄每次呼叫收到的店家，只回 stats 裡有的店（沒有評價的店不在回傳裡）。
type countedRatings struct {
	stats map[int64]httpseller.RatingStats
	err   error
	calls [][]int64
}

func (f *countedRatings) StoreRatingStats(_ context.Context, ids []int64) (map[int64]httpseller.RatingStats, error) {
	f.calls = append(f.calls, slices.Clone(ids))
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64]httpseller.RatingStats{}
	for _, id := range ids {
		if s, ok := f.stats[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

// followModule 回固定的追蹤清單。
type followModule struct {
	fakeModule
	items []model.Item
}

func (f followModule) ListFollows(context.Context, model.ListInput) (model.Page, error) {
	return model.Page{Items: f.items}, nil
}
func (f followModule) FollowCount(context.Context, int64) (int64, error) {
	return int64(len(f.items)), nil
}

func followsFixture(t *testing.T, ratings *countedRatings) (*App, context.Context) {
	town := "63000050"
	cards := []seller.StoreCard{openStoreCard(1, "a", "stores/a"), openStoreCard(2, "b", "stores/b")}
	cards[0].TownCode = &town
	m := followModule{fakeModule: fakeModule{profile: model.Profile{BuyerID: 7, AccountID: 1}}, items: []model.Item{{ID: 1, Time: time.Unix(1, 0)}, {ID: 2, Time: time.Unix(2, 0)}}}
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	stats := testStats(ratings)
	return mustApp(t, m, rpc.Images{}, Ports{Stores: fakeStores{cards: cards}}, stats), ctx
}

// 追蹤清單的卡片帶所在地與評分；評價來源沒回的店是確實沒有評價；整份清單只讀一次評價。
func TestFollowCardsCarryTownAndRatingsFromOneRead(t *testing.T) {
	ratings := &countedRatings{stats: map[int64]httpseller.RatingStats{1: {Count: 8, Average: 4.25}}}
	h, ctx := followsFixture(t, ratings)
	resp, err := h.ListFollows(ctx, connect.NewRequest(&pb.ListFollowsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings.calls) != 1 || len(ratings.calls[0]) != 2 {
		t.Fatalf("評價讀了 %v，預期一次批次讀取兩家", ratings.calls)
	}
	a, b := resp.Msg.GetFollows()[0].GetStore(), resp.Msg.GetFollows()[1].GetStore()
	if a.GetTownCode() != "63000050" || a.GetReviewCount() != 8 || a.RatingAverage == nil || *a.RatingAverage != 4.25 {
		t.Fatalf("a=%v", a)
	}
	if b.GetTownCode() != "" || b.ReviewCount == nil || *b.ReviewCount != 0 || b.RatingAverage != nil {
		t.Fatalf("b=%v", b)
	}
}

// 評價失敗不影響清單：卡片照常、評分不設、RPC 成功（不轉成 Unavailable）。
func TestAFailedRatingReadDoesNotFailTheList(t *testing.T) {
	ratings := &countedRatings{err: errors.New("review unavailable")}
	h, ctx := followsFixture(t, ratings)
	resp, err := h.ListFollows(ctx, connect.NewRequest(&pb.ListFollowsRequest{}))
	if err != nil {
		t.Fatalf("評價讀取失敗不該讓清單失敗：%v", err)
	}
	for _, f := range resp.Msg.GetFollows() {
		if f.GetStore().GetName() == "" || f.GetStore().ReviewCount != nil || f.GetStore().RatingAverage != nil {
			t.Fatalf("卡片照常、評分不設：%v", f)
		}
	}
	if len(ratings.calls) != 1 {
		t.Fatalf("評價讀了 %d 次，預期一次", len(ratings.calls))
	}
}
