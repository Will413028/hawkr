package usecase

import (
	"math"
	"math/rand"
	"time"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

// Seller score weights
const (
	wSales     = 0.30
	wOrders    = 0.25
	wRating    = 0.25
	wFollowers = 0.20

	newSellerBoost    = 0.10
	newSellerDays     = 30
	randomFactorRange = 0.05 // ±5%
)

// Product score weights
const (
	wSellerScore    = 0.70
	wProductQuality = 0.30
	wProductRating  = 0.60
	wProductReviews = 0.40
)

// SellerScoreInput holds the raw data needed to score a seller.
type SellerScoreInput struct {
	CreatedAt   time.Time
	TotalSales  float64
	Rating      float64
	TotalOrders int32
	Followers   int32
}

// ProductScoreInput holds the raw data needed to score a product.
type ProductScoreInput struct {
	Rating      float64
	ReviewCount int32
	SellerScore float64
}

// ComputeSellerScores computes normalized seller scores for a batch.
func ComputeSellerScores(sellers map[identity.StoreID]*SellerScoreInput) map[identity.StoreID]float64 {
	if len(sellers) == 0 {
		return nil
	}

	// Collect min/max for normalization
	var minSales, maxSales float64
	var minOrders, maxOrders int32
	var minRating, maxRating float64
	var minFollowers, maxFollowers int32
	first := true

	for _, s := range sellers {
		if first {
			minSales, maxSales = s.TotalSales, s.TotalSales
			minOrders, maxOrders = s.TotalOrders, s.TotalOrders
			minRating, maxRating = s.Rating, s.Rating
			minFollowers, maxFollowers = s.Followers, s.Followers
			first = false
			continue
		}
		minSales = math.Min(minSales, s.TotalSales)
		maxSales = math.Max(maxSales, s.TotalSales)
		if s.TotalOrders < minOrders {
			minOrders = s.TotalOrders
		}
		if s.TotalOrders > maxOrders {
			maxOrders = s.TotalOrders
		}
		minRating = math.Min(minRating, s.Rating)
		maxRating = math.Max(maxRating, s.Rating)
		if s.Followers < minFollowers {
			minFollowers = s.Followers
		}
		if s.Followers > maxFollowers {
			maxFollowers = s.Followers
		}
	}

	scores := make(map[identity.StoreID]float64, len(sellers))
	for id, s := range sellers {
		normSales := normalize(s.TotalSales, minSales, maxSales)
		normOrders := normalize(float64(s.TotalOrders), float64(minOrders), float64(maxOrders))
		normRating := normalize(s.Rating, minRating, maxRating)
		normFollowers := normalize(float64(s.Followers), float64(minFollowers), float64(maxFollowers))

		score := normSales*wSales +
			normOrders*wOrders +
			normRating*wRating +
			normFollowers*wFollowers

		// New seller boost
		if time.Since(s.CreatedAt) < newSellerDays*24*time.Hour {
			score = math.Min(1.0, score+newSellerBoost)
		}

		scores[id] = score
	}

	return scores
}

// ComputeProductScore computes the final recommendation score for a product.
func ComputeProductScore(input ProductScoreInput) float64 {
	// Product quality: rating (0-5 mapped to 0-1) and review count (capped at 100)
	ratingNorm := math.Min(input.Rating/5.0, 1.0)
	reviewNorm := math.Min(float64(input.ReviewCount)/100.0, 1.0)
	quality := ratingNorm*wProductRating + reviewNorm*wProductReviews

	score := input.SellerScore*wSellerScore + quality*wProductQuality

	// Random factor ±5%
	factor := 1.0 + (rand.Float64()*2-1)*randomFactorRange
	score *= factor

	return math.Min(1.0, math.Max(0.0, score))
}

func normalize(val, min, max float64) float64 {
	if max == min {
		return 0.5 // all same → middle
	}
	return (val - min) / (max - min)
}
