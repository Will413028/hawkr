// Package discovery 是 discovery 模組的 Connect handler：前台的 hawkr.discovery.v1alpha1。這裡只做欄位的轉換與
// 顯示資料的組裝（店名與 logo 來自 seller、粉絲數來自 buyer，評價與卡片一起批次讀取），排行與曝光資格在 internal/modules/discovery。
package discovery

import (
	"context"
	"log/slog"
	"math"

	"connectrpc.com/connect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/discovery/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/discovery/v1alpha1/discoveryv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// DefaultMaxResults 是請求沒有指定 max_results 時回傳的店家數（契約：SuggestRecommendedSellersRequest.max_results）。
const DefaultMaxResults = 10

// Ranking 是排行的來源（*discovery.Module）。
type Ranking interface {
	RecommendedSellers(ctx context.Context, limit int) (discovery.Recommendation, error)
}

// Cards 是店家卡片的來源（*seller.Module）。
type Cards interface {
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
}

// FollowerCounts 是店家粉絲數的來源（*buyer.Module）：沒有粉絲的店家是 0。
type FollowerCounts interface {
	SubscriberCounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error)
}

// Public 實作 DiscoveryPublicService。
type Public struct {
	ranking   Ranking
	cards     Cards
	followers FollowerCounts
	stats     *httpseller.StatsSource
	images    rpc.Images
	log       *slog.Logger
}

var _ discoveryv1alpha1connect.DiscoveryPublicServiceHandler = (*Public)(nil)

// NewPublic 建立前台的 handler；stats 是卡片評分的組裝（httpseller.NewStatsSource）；log 是 nil 時不記錄。
func NewPublic(ranking Ranking, cards Cards, followers FollowerCounts, stats *httpseller.StatsSource, images rpc.Images, log *slog.Logger) (*Public, error) {
	if stats == nil {
		return nil, httpseller.ErrStatsSourceRequired
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Public{ranking: ranking, cards: cards, followers: followers, stats: stats, images: images, log: log}, nil
}

func (h *Public) SuggestRecommendedSellers(ctx context.Context, req *connect.Request[pb.SuggestRecommendedSellersRequest]) (*connect.Response[pb.SuggestRecommendedSellersResponse], error) {
	limit := int(req.Msg.GetMaxResults())
	switch {
	case limit == 0:
		limit = DefaultMaxResults
	case limit < 0:
		return nil, rpc.RequiredField("max_results", "max_results 不可以是負數")
	case limit > discovery.MaxLimit:
		limit = discovery.MaxLimit
	}
	rec, err := h.ranking.RecommendedSellers(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := &pb.SuggestRecommendedSellersResponse{Sellers: make([]*pb.RecommendedSeller, 0, len(rec.Sellers))}
	if len(rec.Sellers) == 0 {
		if rec.ComputedAt == nil {
			h.log.WarnContext(ctx, "discovery：推薦排行還沒有成功重算過，回傳空清單")
		}
		return connect.NewResponse(out), nil
	}
	ids := make([]int64, len(rec.Sellers))
	for i, s := range rec.Sellers {
		ids[i] = s.StoreID
	}
	cards, err := h.cards.StoreCards(ctx, ids)
	if err != nil {
		return nil, err
	}
	cardOf := make(map[int64]seller.StoreCard, len(cards))
	for _, c := range cards {
		cardOf[c.StoreID] = c
	}
	followers, err := h.followers.SubscriberCounts(ctx, ids)
	if err != nil {
		h.log.WarnContext(ctx, "discovery：讀取粉絲數失敗，follower_count 不設", "error", err)
		followers = nil
	}
	var (
		kept     []discovery.RankedSeller
		keptIDs  []int64
		keptCard []*commonpb.StoreCard
	)
	for _, s := range rec.Sellers {
		c, ok := cardOf[s.StoreID]
		if !ok || c.Status != seller.CardOpen {
			h.log.WarnContext(ctx, "discovery：排行上的店家沒有營業中的卡片，略過", "store_id", s.StoreID, "has_card", ok)
			continue
		}
		store, err := httpseller.ToStoreCard(h.images, c)
		if err != nil {
			return nil, err
		}
		kept, keptIDs, keptCard = append(kept, s), append(keptIDs, s.StoreID), append(keptCard, store)
	}
	// 評分跟著卡片一次批次讀取（評價數與平均在 store 上，排名用的評價只留在快照內）；讀不到時只有評分不設。
	h.stats.RateCards(ctx, keptIDs, keptCard)
	for i, s := range kept {
		item := &pb.RecommendedSeller{Store: keptCard[i]}
		if n, ok := followers[s.StoreID]; ok {
			item.FollowerCount = count32(n)
		}
		out.Sellers = append(out.Sellers, item)
	}
	return connect.NewResponse(out), nil
}

// count32 把計數放進契約的 int32；放不下時不設（回 nil），與店面頁的統計同一個政策：不回一個錯的數字。
func count32(n int64) *int32 {
	if n < 0 || n > math.MaxInt32 {
		return nil
	}
	v := int32(n)
	return &v
}
