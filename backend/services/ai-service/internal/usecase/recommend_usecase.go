package usecase

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"
)

type RecommendUseCase struct {
	productClient ProductClient
	sellerClient  SellerClient
	cache         *cache.MemoryCache
}

func NewRecommendUseCase(
	productClient ProductClient,
	sellerClient SellerClient,
) *RecommendUseCase {
	return &RecommendUseCase{
		productClient: productClient,
		sellerClient:  sellerClient,
		cache:         cache.New(5 * time.Minute),
	}
}

// scoredProduct holds a product with its computed recommendation score.
type scoredProduct struct {
	product     *ProductItem
	sellerMeta  *SellerMetaItem
	sellerScore float64
	finalScore  float64
}

// fetchAndScore fetches products + seller data and returns scored products.
func (uc *RecommendUseCase) fetchAndScore(ctx context.Context) ([]scoredProduct, error) {
	products, _, err := uc.productClient.GetProductList(ctx, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("fetch products: %w", err)
	}
	if len(products) == 0 {
		return nil, nil
	}

	// Collect unique store IDs (ProductItem.StoreID)
	storeIDSet := make(map[identity.StoreID]bool)
	for _, p := range products {
		storeIDSet[p.StoreID] = true
	}
	sellerIDs := make([]identity.StoreID, 0, len(storeIDSet))
	for id := range storeIDSet {
		sellerIDs = append(sellerIDs, id)
	}

	// Fetch seller stats + meta in parallel. syncutil.Group：panic 會被
	// recover 並記錄，不會讓整個 ai-service process crash（不用
	// WithContext — 兩邊都是 best-effort，互不 cancel）。
	var (
		statsMap = make(map[identity.StoreID]*SellerStatsItem)
		metaMap  map[identity.StoreID]*SellerMetaItem
		mu       sync.Mutex
	)

	g := syncutil.NewGroup(ctx)

	g.Go(func() error {
		for _, sid := range sellerIDs {
			stats, err := uc.sellerClient.GetSellerStats(ctx, sid)
			if err != nil {
				pkglogger.Warn(ctx, "GetSellerStats failed", zap.Int64("seller_id", int64(sid)), zap.Error(err))
				continue
			}
			mu.Lock()
			statsMap[sid] = stats
			mu.Unlock()
		}
		return nil
	})

	g.Go(func() error {
		m, err := uc.sellerClient.BatchGetSellerMeta(ctx, sellerIDs)
		if err != nil {
			pkglogger.Warn(ctx, "BatchGetSellerMeta failed", zap.Error(err))
			return nil
		}
		mu.Lock()
		metaMap = m
		mu.Unlock()
		return nil
	})

	_ = g.Wait()

	// Build seller score inputs
	sellerInputs := make(map[identity.StoreID]*SellerScoreInput, len(sellerIDs))
	for _, sid := range sellerIDs {
		stats, ok := statsMap[sid]
		if !ok {
			continue
		}
		sellerInputs[sid] = &SellerScoreInput{
			TotalSales:  stats.TotalSales,
			TotalOrders: stats.TotalOrders,
			Rating:      stats.Rating,
			Followers:   stats.Followers,
			CreatedAt:   time.Now(), // seller created_at not available via stats RPC
		}
	}

	sellerScores := ComputeSellerScores(sellerInputs)

	// Score each product
	scored := make([]scoredProduct, 0, len(products))
	for _, p := range products {
		ss := sellerScores[p.StoreID]
		fs := ComputeProductScore(ProductScoreInput{
			Rating:      p.Rating,
			ReviewCount: 0,
			SellerScore: ss,
		})

		var meta *SellerMetaItem
		if metaMap != nil {
			meta = metaMap[p.StoreID]
		}

		scored = append(scored, scoredProduct{
			product:     p,
			sellerMeta:  meta,
			sellerScore: ss,
			finalScore:  fs,
		})
	}

	return scored, nil
}

func toDTO(sp scoredProduct) dto.RecommendedProduct {
	reason := "精選推薦"
	if sp.sellerScore >= 0.8 {
		reason = "熱門賣家推薦"
	} else if sp.sellerScore >= 0.5 {
		reason = "優質賣家推薦"
	}

	var sellerName, sellerLogo string
	if sp.sellerMeta != nil {
		sellerName = sp.sellerMeta.SellerName
		sellerLogo = sp.sellerMeta.LogoKey
	}

	return dto.RecommendedProduct{
		ProductID:  sp.product.ID,
		Name:       sp.product.Name,
		ImageURL:   sp.product.ImageURL,
		PriceMin:   sp.product.PriceMin,
		PriceMax:   sp.product.PriceMax,
		Rating:     sp.product.Rating,
		SellerName: sellerName,
		SellerLogo: sellerLogo,
		Score:      sp.finalScore,
		Reason:     reason,
	}
}

func topN(scored []scoredProduct, n int) []dto.RecommendedProduct {
	if n > len(scored) {
		n = len(scored)
	}
	result := make([]dto.RecommendedProduct, n)
	for i := 0; i < n; i++ {
		result[i] = toDTO(scored[i])
	}
	return result
}

// Personalized returns three recommendation sections with real data.
func (uc *RecommendUseCase) Personalized(ctx context.Context, userID int64) (dto.PersonalizedResponse, error) {
	pkglogger.Info(ctx, "Personalized", zap.Int64("user_id", userID))

	cacheKey := "personalized:global"
	if cached, ok := uc.cache.Get(cacheKey); ok {
		return cached.(dto.PersonalizedResponse), nil
	}

	scored, err := uc.fetchAndScore(ctx)
	if err != nil {
		return dto.PersonalizedResponse{}, err
	}

	// Section 1: 精選推薦 — by recommendation score
	featured := make([]scoredProduct, len(scored))
	copy(featured, scored)
	sort.Slice(featured, func(i, j int) bool {
		return featured[i].finalScore > featured[j].finalScore
	})

	// Section 2: 熱門商品 — by seller score (proxy for popularity)
	hot := make([]scoredProduct, len(scored))
	copy(hot, scored)
	sort.Slice(hot, func(i, j int) bool {
		return hot[i].sellerScore > hot[j].sellerScore
	})

	// Section 3: 新鮮上市 — by created_at descending
	newArrivals := make([]scoredProduct, len(scored))
	copy(newArrivals, scored)
	sort.Slice(newArrivals, func(i, j int) bool {
		return newArrivals[i].product.CreatedAt > newArrivals[j].product.CreatedAt
	})

	maxItems := 10
	resp := dto.PersonalizedResponse{
		Sections: []dto.PersonalizedSection{
			{Title: "精選推薦", Type: "featured", Products: topN(featured, maxItems)},
			{Title: "熱門商品", Type: "hot", Products: topN(hot, maxItems)},
			{Title: "新鮮上市", Type: "new_arrivals", Products: topN(newArrivals, maxItems)},
		},
		ModelVersion: domain.ModelVersion,
	}
	uc.cache.Set(cacheKey, resp)
	return resp, nil
}
