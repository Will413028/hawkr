package discovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/discovery/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type fakeRanking struct {
	rec    discovery.Recommendation
	limits []int
}

func (f *fakeRanking) RecommendedSellers(_ context.Context, limit int) (discovery.Recommendation, error) {
	f.limits = append(f.limits, limit)
	return f.rec, nil
}

type fakeCards struct {
	cards []seller.StoreCard
	calls int
}

func (f *fakeCards) StoreCards(context.Context, []int64) ([]seller.StoreCard, error) {
	f.calls++
	return f.cards, nil
}

type fakeFollowers struct {
	counts map[int64]int64
	err    error
}

func (f fakeFollowers) SubscriberCounts(context.Context, []int64) (map[int64]int64, error) {
	return f.counts, f.err
}

// fakeRatings 是評價來源的替身：記錄每次呼叫收到的店家，只回 stats 裡有的店（沒有評價的店不在回傳裡）。
type fakeRatings struct {
	stats map[int64]httpseller.RatingStats
	err   error
	calls [][]int64
}

func (f *fakeRatings) StoreRatingStats(_ context.Context, ids []int64) (map[int64]httpseller.RatingStats, error) {
	f.calls = append(f.calls, ids)
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

func rankedFixture() (*fakeRanking, *fakeCards) {
	computed := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	name, logo, town := "範例小農", "stores/logo", "63000010"
	return &fakeRanking{rec: discovery.Recommendation{ComputedAt: &computed, Sellers: []discovery.RankedSeller{
		{StoreID: 5, RecentOrders: 9},
		{StoreID: 3},
	}}}, &fakeCards{cards: []seller.StoreCard{
		{StoreID: 3, StoreCode: 2600000003, Name: &name, TownCode: &town, Status: seller.CardOpen},
		{StoreID: 5, StoreCode: 2600000005, Name: &name, LogoKey: &logo, TownCode: &town, Status: seller.CardOpen},
	}}
}

func newHandler(t *testing.T, r Ranking, c Cards, f FollowerCounts) *Public {
	t.Helper()
	return newHandlerWithRatings(t, r, c, f, &fakeRatings{stats: map[int64]httpseller.RatingStats{5: {Count: 12, Average: 4.5}}})
}

func newHandlerWithRatings(t *testing.T, r Ranking, c Cards, f FollowerCounts, ratings httpseller.Ratings) *Public {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	stats, err := httpseller.NewStatsSource(f, ratings, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewPublic(r, c, f, stats, images, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func suggest(h *Public, max int32) (*pb.SuggestRecommendedSellersResponse, error) {
	resp, err := h.SuggestRecommendedSellers(context.Background(), connect.NewRequest(&pb.SuggestRecommendedSellersRequest{MaxResults: max}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// 契約的預設是 10：沒帶 max_results 時問排行 10 家，帶了就照帶的，超過上限以上限計（同 page_size）；負數不送到排行。
func TestMaxResultsDefaultsToTenAndIsBounded(t *testing.T) {
	ranking, cards := rankedFixture()
	h := newHandler(t, ranking, cards, fakeFollowers{counts: map[int64]int64{}})
	for _, c := range []struct{ asked, want int32 }{{0, 10}, {1, 1}, {7, 7}, {50, 50}, {51, 50}, {1000, 50}} {
		if _, err := suggest(h, c.asked); err != nil {
			t.Fatal(err)
		}
		if got := ranking.limits[len(ranking.limits)-1]; got != int(c.want) {
			t.Errorf("max_results=%d 問排行 %d 家，預期 %d", c.asked, got, c.want)
		}
	}
	calls := len(ranking.limits)
	for _, bad := range []int32{-1, -50} {
		if _, err := suggest(h, bad); err == nil {
			t.Errorf("max_results=%d 應被拒絕", bad)
		}
	}
	if len(ranking.limits) != calls {
		t.Fatal("負數的請求不該送到排行")
	}
}

// 卡片帶所在地與評分（評價數與平均在 store 上，不在 RecommendedSeller）：店家資料與評價各一次批次讀取；評價來源沒回的店是確實
// 沒有評價（評價數 0、平均不設）。
func TestCardsCarryTownAndRatingsFromOneBatchRead(t *testing.T) {
	ranking, cards := rankedFixture()
	ratings := &fakeRatings{stats: map[int64]httpseller.RatingStats{5: {Count: 12, Average: 4.5}}}
	h := newHandlerWithRatings(t, ranking, cards, fakeFollowers{counts: map[int64]int64{5: 40, 3: 0}}, ratings)
	got, err := suggest(h, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cards.calls != 1 {
		t.Fatalf("店家資料讀了 %d 次，預期一次批次讀取", cards.calls)
	}
	if len(got.GetSellers()) != 2 || got.GetSellers()[0].GetStore().GetId() != "5" || got.GetSellers()[1].GetStore().GetId() != "3" {
		t.Fatalf("順序要照排行：%v", got.GetSellers())
	}
	if len(ratings.calls) != 1 || len(ratings.calls[0]) != 2 {
		t.Fatalf("評價讀了 %v，預期一次批次讀取兩家", ratings.calls)
	}
	first, second := got.GetSellers()[0], got.GetSellers()[1]
	if first.GetStore().GetTownCode() != "63000010" || second.GetStore().GetTownCode() != "63000010" {
		t.Fatalf("卡片要帶 town_code：%v %v", first, second)
	}
	if first.GetStore().GetReviewCount() != 12 || first.GetStore().RatingAverage == nil || *first.GetStore().RatingAverage != 4.5 || first.GetStore().GetName() != "範例小農" ||
		first.GetStore().GetStoreCode() != "2600000005" || first.GetStore().GetLogo().GetUriTemplate() != "https://media.example.test/i/{preset}/stores/logo" ||
		first.FollowerCount == nil || *first.FollowerCount != 40 {
		t.Fatalf("first=%v", first)
	}
	if second.GetStore().ReviewCount == nil || *second.GetStore().ReviewCount != 0 || second.GetStore().RatingAverage != nil || second.FollowerCount == nil || *second.FollowerCount != 0 || second.GetStore().GetLogo() != nil {
		t.Fatalf("second=%v", second)
	}
}

// 粉絲數讀不到不是 0：欄位不設，排行本身照樣回傳。
func TestAFailedFollowerReadIsNotZero(t *testing.T) {
	ranking, cards := rankedFixture()
	h := newHandler(t, ranking, cards, fakeFollowers{err: errors.New("buyer unavailable")})
	got, err := suggest(h, 0)
	if err != nil || len(got.GetSellers()) != 2 {
		t.Fatalf("sellers=%v err=%v", got.GetSellers(), err)
	}
	for _, s := range got.GetSellers() {
		if s.FollowerCount != nil {
			t.Fatalf("粉絲數讀取失敗時不設：%v", s)
		}
	}
}

// 排行上的店沒有卡片、或卡片不是營業中，就略過（記 warn），不是整個請求失敗：清單上的每家店都有 OPEN 的卡片。
func TestSellersWithoutAnOpenCardAreSkipped(t *testing.T) {
	for name, mutate := range map[string]func(c *fakeCards){
		"missing card": func(c *fakeCards) { c.cards = c.cards[:1] },
		"not open":     func(c *fakeCards) { c.cards[1].Status = seller.CardPaused },
	} {
		t.Run(name, func(t *testing.T) {
			ranking, cards := rankedFixture()
			mutate(cards)
			h := newHandler(t, ranking, cards, fakeFollowers{})
			got, err := suggest(h, 0)
			if err != nil {
				t.Fatalf("不該整個請求失敗：%v", err)
			}
			if len(got.GetSellers()) != 1 || got.GetSellers()[0].GetStore().GetId() != "3" {
				t.Fatalf("sellers=%v, want only store 3", got.GetSellers())
			}
			if got.GetSellers()[0].GetStore().GetStatus() != commonpb.StoreCardStatus_STORE_CARD_STATUS_OPEN {
				t.Fatalf("清單上的卡片都是 OPEN：%v", got.GetSellers()[0])
			}
		})
	}
}

// 評價來源失敗時，卡片照常回、評分兩欄都不設、RPC 成功；仍然只讀一次。
func TestAFailedRatingReadLeavesTheCardsWithoutRatings(t *testing.T) {
	ranking, cards := rankedFixture()
	ratings := &fakeRatings{err: errors.New("review unavailable")}
	h := newHandlerWithRatings(t, ranking, cards, fakeFollowers{}, ratings)
	got, err := suggest(h, 0)
	if err != nil || len(got.GetSellers()) != 2 {
		t.Fatalf("sellers=%v err=%v", got.GetSellers(), err)
	}
	for _, s := range got.GetSellers() {
		if s.GetStore().ReviewCount != nil || s.GetStore().RatingAverage != nil || s.GetStore().GetName() == "" {
			t.Fatalf("評價讀取失敗時評分不設、卡片照常：%v", s)
		}
	}
	if len(ratings.calls) != 1 {
		t.Fatalf("評價讀了 %d 次，預期一次", len(ratings.calls))
	}
}

// 評價數放不進 int32 時不設，與店面頁同一個政策。
func TestAnOverflowingReviewCountIsUnset(t *testing.T) {
	ranking, cards := rankedFixture()
	ratings := &fakeRatings{stats: map[int64]httpseller.RatingStats{5: {Count: 1 << 40, Average: 4.5}}}
	h := newHandlerWithRatings(t, ranking, cards, fakeFollowers{}, ratings)
	got, err := suggest(h, 0)
	if err != nil || got.GetSellers()[0].GetStore().ReviewCount != nil {
		t.Fatalf("sellers=%v err=%v", got.GetSellers(), err)
	}
}

func TestNoQualifiedStoresIsAnEmptyList(t *testing.T) {
	computed := time.Now()
	for name, rec := range map[string]discovery.Recommendation{"computed, none qualify": {ComputedAt: &computed}, "never computed": {}} {
		cards := &fakeCards{}
		h := newHandler(t, &fakeRanking{rec: rec}, cards, fakeFollowers{})
		got, err := suggest(h, 0)
		if err != nil || len(got.GetSellers()) != 0 || cards.calls != 0 {
			t.Errorf("%s: sellers=%v err=%v cardCalls=%d", name, got.GetSellers(), err, cards.calls)
		}
	}
}

func TestNewPublicRejectsANilStatsSource(t *testing.T) {
	if _, err := NewPublic(nil, nil, nil, nil, rpc.Images{}, nil); err == nil {
		t.Fatal("NewPublic 收下了 nil 的 StatsSource")
	}
}
