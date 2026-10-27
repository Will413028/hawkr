package usecase

import (
	"math"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestComputeSellerScores_Empty(t *testing.T) {
	result := ComputeSellerScores(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestComputeSellerScores_SingleSeller(t *testing.T) {
	sellers := map[identity.StoreID]*SellerScoreInput{
		1: {
			TotalSales:  10000,
			TotalOrders: 50,
			Rating:      4.5,
			Followers:   100,
			CreatedAt:   time.Now().AddDate(-1, 0, 0), // old seller
		},
	}

	scores := ComputeSellerScores(sellers)
	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}

	// Single seller: all normalized values are 0.5 (since min==max → normalize returns 0.5)
	// score = 0.5*0.30 + 0.5*0.25 + 0.5*0.25 + 0.5*0.20 = 0.50
	expected := 0.50
	if math.Abs(scores[1]-expected) > 0.001 {
		t.Errorf("single seller score = %f, want ~%f", scores[1], expected)
	}
}

func TestComputeSellerScores_MultipleSellers(t *testing.T) {
	sellers := map[identity.StoreID]*SellerScoreInput{
		1: {
			TotalSales:  100000,
			TotalOrders: 200,
			Rating:      4.8,
			Followers:   500,
			CreatedAt:   time.Now().AddDate(-1, 0, 0),
		},
		2: {
			TotalSales:  10000,
			TotalOrders: 20,
			Rating:      3.0,
			Followers:   50,
			CreatedAt:   time.Now().AddDate(-1, 0, 0),
		},
	}

	scores := ComputeSellerScores(sellers)
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}

	// Seller 1 should have a higher score (max everything)
	if scores[1] <= scores[2] {
		t.Errorf("seller 1 (score=%f) should be higher than seller 2 (score=%f)", scores[1], scores[2])
	}

	// Seller 1: all normalized to 1.0
	// 1.0*0.30 + 1.0*0.25 + 1.0*0.25 + 1.0*0.20 = 1.0
	if math.Abs(scores[1]-1.0) > 0.001 {
		t.Errorf("top seller score = %f, want ~1.0", scores[1])
	}

	// Seller 2: all normalized to 0.0
	// 0.0*0.30 + 0.0*0.25 + 0.0*0.25 + 0.0*0.20 = 0.0
	if math.Abs(scores[2]-0.0) > 0.001 {
		t.Errorf("bottom seller score = %f, want ~0.0", scores[2])
	}
}

func TestComputeSellerScores_NewSellerBoost(t *testing.T) {
	sellers := map[identity.StoreID]*SellerScoreInput{
		1: {
			TotalSales:  5000,
			TotalOrders: 10,
			Rating:      3.5,
			Followers:   20,
			CreatedAt:   time.Now().Add(-24 * time.Hour), // 1 day old = new seller
		},
		2: {
			TotalSales:  5000,
			TotalOrders: 10,
			Rating:      3.5,
			Followers:   20,
			CreatedAt:   time.Now().AddDate(-1, 0, 0), // 1 year old
		},
	}

	scores := ComputeSellerScores(sellers)

	// Both have identical stats but seller 1 is new → gets +0.10 boost
	if scores[1] <= scores[2] {
		t.Errorf("new seller (score=%f) should be higher than old seller (score=%f)", scores[1], scores[2])
	}

	boost := scores[1] - scores[2]
	if math.Abs(boost-newSellerBoost) > 0.001 {
		t.Errorf("new seller boost = %f, want %f", boost, newSellerBoost)
	}
}

func TestComputeSellerScores_NewSellerBoostCappedAt1(t *testing.T) {
	sellers := map[identity.StoreID]*SellerScoreInput{
		1: {
			TotalSales:  100000,
			TotalOrders: 200,
			Rating:      5.0,
			Followers:   1000,
			CreatedAt:   time.Now().Add(-24 * time.Hour), // new seller with max stats
		},
	}

	scores := ComputeSellerScores(sellers)
	if scores[1] > 1.0 {
		t.Errorf("score should be capped at 1.0, got %f", scores[1])
	}
}

func TestComputeProductScore_Basic(t *testing.T) {
	score := ComputeProductScore(ProductScoreInput{
		Rating:      5.0,
		ReviewCount: 100,
		SellerScore: 1.0,
	})

	// seller_score * 0.70 + quality * 0.30
	// quality = (5.0/5.0)*0.60 + (100/100)*0.40 = 1.0
	// base = 1.0*0.70 + 1.0*0.30 = 1.0
	// With ±5% random factor, should be in [0.95, 1.0] (capped at 1.0)
	if score < 0.90 || score > 1.0 {
		t.Errorf("max product score = %f, want in [0.90, 1.0]", score)
	}
}

func TestComputeProductScore_ZeroInputs(t *testing.T) {
	score := ComputeProductScore(ProductScoreInput{
		Rating:      0,
		ReviewCount: 0,
		SellerScore: 0,
	})

	// base = 0, with ±5% random → clamped to [0, 1]
	if score < 0 || score > 0.10 {
		t.Errorf("zero input score = %f, want near 0", score)
	}
}

func TestComputeProductScore_SellerWeightDominates(t *testing.T) {
	// High seller score, low product quality
	highSeller := ComputeProductScore(ProductScoreInput{
		Rating:      1.0,
		ReviewCount: 0,
		SellerScore: 1.0,
	})

	// Low seller score, high product quality
	highProduct := ComputeProductScore(ProductScoreInput{
		Rating:      5.0,
		ReviewCount: 100,
		SellerScore: 0.0,
	})

	// Seller weight (70%) should generally dominate over product quality (30%)
	// highSeller base: 1.0*0.70 + (0.2*0.60+0.0*0.40)*0.30 = 0.70 + 0.036 = 0.736
	// highProduct base: 0.0*0.70 + 1.0*0.30 = 0.30
	// Even with ±5% random, highSeller should be notably higher
	if highSeller < 0.60 {
		t.Errorf("high seller score = %f, expected > 0.60", highSeller)
	}
	if highProduct > 0.40 {
		t.Errorf("high product score = %f, expected < 0.40", highProduct)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name          string
		val, min, max float64
		want          float64
	}{
		{"middle", 50, 0, 100, 0.5},
		{"min", 0, 0, 100, 0.0},
		{"max", 100, 0, 100, 1.0},
		{"same min max", 42, 42, 42, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize(tt.val, tt.min, tt.max)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("normalize(%f, %f, %f) = %f, want %f", tt.val, tt.min, tt.max, got, tt.want)
			}
		})
	}
}
