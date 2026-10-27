package seller

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"connectrpc.com/connect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// batchRatings 是評價來源的替身：記錄每次呼叫收到的店家，只回 stats 裡有的店（沒有評價的店不在回傳裡）。
type batchRatings struct {
	stats map[int64]RatingStats
	err   error
	calls [][]int64
}

func (f *batchRatings) StoreRatingStats(_ context.Context, ids []int64) (map[int64]RatingStats, error) {
	f.calls = append(f.calls, slices.Clone(ids))
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64]RatingStats{}
	for _, id := range ids {
		if s, ok := f.stats[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

func cardFixture() []model.StoreCard {
	return []model.StoreCard{
		{StoreID: 5, StoreCode: 2600000005, Name: ptr("範例小農"), TownCode: ptr("63000050"), Status: model.CardOpen},
		{StoreID: 6, StoreCode: 2600000006, Name: ptr("無評價"), TownCode: ptr("63000060"), Status: model.CardOpen},
		// 個資清除之後：沒有店名、沒有所在地。
		{StoreID: 7, StoreCode: 2600000007, Status: model.CardClosed},
	}
}

func batchGet(t *testing.T, h *Public, ids ...string) (*pb.BatchGetStoresResponse, error) {
	t.Helper()
	resp, err := h.BatchGetStores(context.Background(), connect.NewRequest(&pb.BatchGetStoresRequest{Ids: ids}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// 賣家預覽自己的店：不論是否公開都讀得到統計；評價 port 失敗時只有評價欄位不設。
func TestGetMyStoreCarriesStatsEvenWhenTheStoreIsNotPublic(t *testing.T) {
	notExposable := &fakeModule{}
	followers := fakeFollowerCounts{counts: map[int64]int64{11: 5}}
	get := func(ratings Ratings) *pb.StoreStats {
		t.Helper()
		h := mustApp(t, notExposable, nil, rpc.Images{}, Dashboard{}, testStats(followers, ratings))
		resp, err := h.GetMyStore(ctxWithStore(currentStore()), connect.NewRequest(&pb.GetMyStoreRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		if listing := CapabilityStatusOf(t, resp.Msg.GetCapabilities(), pb.SellerCapability_SELLER_CAPABILITY_PUBLIC_LISTING); listing.GetActive() {
			t.Fatal("這個測試的店要是未公開的")
		}
		return resp.Msg.GetStats()
	}
	st := get(fakeRatings{stats: map[int64]RatingStats{11: {Count: 3, Average: 4.5}}})
	if st == nil || st.FollowerCount == nil || *st.FollowerCount != 5 || st.ReviewCount == nil || *st.ReviewCount != 3 || st.RatingAverage == nil || *st.RatingAverage != 4.5 {
		t.Fatalf("stats=%v", st)
	}
	st = get(fakeRatings{err: errors.New("review unavailable")})
	if st == nil || st.FollowerCount == nil || *st.FollowerCount != 5 || st.ReviewCount != nil || st.RatingAverage != nil {
		t.Fatalf("評價失敗時只有評價欄位不設：%v", st)
	}
}

func TestBatchGetStoresCardsCarryTownAndRatingsFromOneRead(t *testing.T) {
	ratings := &batchRatings{stats: map[int64]RatingStats{5: {Count: 12, Average: 4.5}}}
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h := mustPublic(t, &fakePublic{cards: cardFixture()}, testStats(fakeFollowerCounts{}, ratings), images)
	got, err := batchGet(t, h, "5", "6", "7")
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings.calls) != 1 || !slices.Equal(ratings.calls[0], []int64{5, 6, 7}) {
		t.Fatalf("評價讀了 %v，預期一次批次讀取三家", ratings.calls)
	}
	a, b, c := got.GetStores()[0], got.GetStores()[1], got.GetStores()[2]
	if a.GetTownCode() != "63000050" || a.GetReviewCount() != 12 || a.RatingAverage == nil || *a.RatingAverage != 4.5 {
		t.Fatalf("a=%v", a)
	}
	// 評價來源沒回的店是確實沒有評價：評價數 0，平均不設。
	if b.GetTownCode() != "63000060" || b.ReviewCount == nil || *b.ReviewCount != 0 || b.RatingAverage != nil {
		t.Fatalf("b=%v", b)
	}
	// 個資清除之後所在地是空字串，不是別的值。
	if c.GetTownCode() != "" || c.Name != nil {
		t.Fatalf("c=%v", c)
	}
}

func TestBatchGetStoresSurvivesAFailedRatingRead(t *testing.T) {
	ratings := &batchRatings{err: errors.New("review unavailable")}
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h := mustPublic(t, &fakePublic{cards: cardFixture()}, testStats(fakeFollowerCounts{}, ratings), images)
	got, err := batchGet(t, h, "5", "6", "7")
	if err != nil || len(got.GetStores()) != 3 {
		t.Fatalf("stores=%v err=%v", got.GetStores(), err)
	}
	for _, s := range got.GetStores() {
		if s.ReviewCount != nil || s.RatingAverage != nil {
			t.Fatalf("評價讀取失敗時評分兩欄都不設：%v", s)
		}
	}
	if got.GetStores()[0].GetName() != "範例小農" {
		t.Fatalf("卡片照常：%v", got.GetStores()[0])
	}
}

// 超過一批的上限時分批，每批各讀一次；每張卡片拿到自己那批的結果。
func TestRateCardsSplitsLargeBatches(t *testing.T) {
	const n = 250
	ids := make([]int64, n)
	cards := make([]*commonpb.StoreCard, n)
	stats := map[int64]RatingStats{}
	for i := range ids {
		ids[i] = int64(i + 1)
		cards[i] = &commonpb.StoreCard{}
		stats[ids[i]] = RatingStats{Count: ids[i], Average: 4}
	}
	ratings := &batchRatings{stats: stats}
	testStats(fakeFollowerCounts{}, ratings).RateCards(context.Background(), ids, cards)
	if len(ratings.calls) != 3 || len(ratings.calls[0]) != 100 || len(ratings.calls[1]) != 100 || len(ratings.calls[2]) != 50 {
		t.Fatalf("批次=%d", len(ratings.calls))
	}
	for i, c := range cards {
		if c.GetReviewCount() != int32(i+1) {
			t.Fatalf("card %d review_count=%d", i, c.GetReviewCount())
		}
	}
}

// 缺任何一個必要依賴時建構就失敗，執行期不再判斷 nil。
func TestNewStatsSourceRequiresEveryDependency(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, c := range map[string]struct {
		followers FollowerCounts
		ratings   Ratings
		log       *slog.Logger
	}{
		"no followers": {nil, fakeRatings{}, log},
		"no ratings":   {fakeFollowerCounts{}, nil, log},
		"no logger":    {fakeFollowerCounts{}, fakeRatings{}, nil},
	} {
		if _, err := NewStatsSource(c.followers, c.ratings, c.log); err == nil {
			t.Errorf("%s：應該回錯誤", name)
		}
	}
}

// 空批次不讀取。
func TestRateCardsWithAnEmptyBatchDoesNotRead(t *testing.T) {
	ratings := &batchRatings{}
	testStats(fakeFollowerCounts{}, ratings).RateCards(context.Background(), nil, nil)
	if len(ratings.calls) != 0 {
		t.Fatalf("空批次不該讀取：%v", ratings.calls)
	}
}
