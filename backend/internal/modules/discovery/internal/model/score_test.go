package model

import (
	"math"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// one 算單一店家（候選集合只有它自己）。
func one(t *testing.T, in Input) Score {
	t.Helper()
	got, err := Compute([]Input{in}, now)
	if err != nil {
		t.Fatal(err)
	}
	return got[0]
}

func TestDemandIsLogNormalizedByTheMaximum(t *testing.T) {
	got, err := Compute([]Input{
		{StoreID: 1, PublishedAt: now.AddDate(-1, 0, 0), DistinctBuyers: 99, QualifyingOrders: 99},
		{StoreID: 2, PublishedAt: now.AddDate(-1, 0, 0), DistinctBuyers: 9, QualifyingOrders: 9},
		{StoreID: 3, PublishedAt: now.AddDate(-1, 0, 0)},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !near(got[0].Demand, 1) || !near(got[1].Demand, math.Log(10)/math.Log(100)) || got[2].Demand != 0 {
		t.Fatalf("demand = %v %v %v", got[0].Demand, got[1].Demand, got[2].Demand)
	}
}

func TestDemandIsZeroWhenNobodyHasBuyers(t *testing.T) {
	s := one(t, Input{StoreID: 1, PublishedAt: now.AddDate(-1, 0, 0)})
	if s.Demand != 0 || math.IsNaN(s.Score) {
		t.Fatalf("%+v", s)
	}
}

func TestQualityIsBayesianAverageTowardTheWeightedGlobalMean(t *testing.T) {
	// 全域平均 = (100·5 + 0·0 + 10·3)/110 = 530/110；評價少的店向它收斂。
	got, err := Compute([]Input{
		{StoreID: 1, PublishedAt: now.AddDate(-1, 0, 0), RatingCount: 100, RatingAverage: 5},
		{StoreID: 2, PublishedAt: now.AddDate(-1, 0, 0)},
		{StoreID: 3, PublishedAt: now.AddDate(-1, 0, 0), RatingCount: 10, RatingAverage: 3},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	m := 530.0 / 110.0
	want := func(n, avg float64) float64 {
		return (((RatingPriorWeight*m + n*avg) / (RatingPriorWeight + n)) - 1) / 4
	}
	for i, w := range []float64{want(100, 5), want(0, 0), want(10, 3)} {
		if !near(got[i].Quality, w) {
			t.Fatalf("store %d quality = %v, want %v", i+1, got[i].Quality, w)
		}
	}
	// 沒有評價的店等於全站平均；一則 5 星不會贏過一百則 4.8 星。
	if !near(got[1].Quality, (m-1)/4) {
		t.Fatalf("no reviews: %v", got[1].Quality)
	}
}

func TestSingleFiveStarReviewDoesNotBeatManyGoodReviews(t *testing.T) {
	got, err := Compute([]Input{
		{StoreID: 1, PublishedAt: now.AddDate(-1, 0, 0), RatingCount: 1, RatingAverage: 5},
		{StoreID: 2, PublishedAt: now.AddDate(-1, 0, 0), RatingCount: 200, RatingAverage: 4.8},
		{StoreID: 3, PublishedAt: now.AddDate(-1, 0, 0), RatingCount: 200, RatingAverage: 2},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Quality >= got[1].Quality {
		t.Fatalf("quality 1 star-review %v >= 200 reviews %v", got[0].Quality, got[1].Quality)
	}
}

func TestQualityWithoutAnyReviewsUsesThePrior(t *testing.T) {
	s := one(t, Input{StoreID: 1, PublishedAt: now.AddDate(-1, 0, 0)})
	if !near(s.Quality, (RatingFallbackMean-1)/4) {
		t.Fatalf("quality = %v", s.Quality)
	}
}

func TestReliabilityPenaltyIsSmoothed(t *testing.T) {
	cases := []struct {
		name                string
		qualifying, cancels int64
		want                float64
	}{
		{"沒有訂單", 0, 0, 0},
		{"一筆取消不重罰", 0, 1, 1.0 / 11},
		{"一半取消", 100, 100, 100.0 / 210},
		{"沒有取消", 500, 0, 0},
		{"大量取消逼近 1", 0, 100000, 100000.0 / 100010},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := one(t, Input{StoreID: 1, PublishedAt: now.AddDate(-1, 0, 0), QualifyingOrders: c.qualifying, SellerCancelledOrders: c.cancels})
			if !near(s.ReliabilityPenalty, c.want) {
				t.Fatalf("penalty = %v, want %v", s.ReliabilityPenalty, c.want)
			}
		})
	}
}

func TestNewnessDecaysLinearlyOverThirtyDays(t *testing.T) {
	cases := []struct {
		name string
		age  time.Duration
		want float64
	}{
		{"剛開始曝光", 0, 1},
		{"十五天", 15 * 24 * time.Hour, 0.5},
		{"三十天", 30 * 24 * time.Hour, 0},
		{"超過三十天", 400 * 24 * time.Hour, 0},
		{"可曝光時間在未來（時鐘偏差）", -time.Hour, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := one(t, Input{StoreID: 1, PublishedAt: now.Add(-c.age)}).Newness; !near(got, c.want) {
				t.Fatalf("newness = %v, want %v", got, c.want)
			}
		})
	}
}

func TestScoreCombinesComponentsWithTheDocumentedWeights(t *testing.T) {
	got, err := Compute([]Input{
		{StoreID: 1, PublishedAt: now.Add(-15 * 24 * time.Hour), DistinctBuyers: 9, QualifyingOrders: 20, SellerCancelledOrders: 10, RatingCount: 10, RatingAverage: 5},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	s := got[0]
	want := 0.5*1 + 0.3*s.Quality - 0.5*(10.0/40) + 0.2*0.5
	if !near(s.Score, want) || !near(s.Demand, 1) || !near(s.ReliabilityPenalty, 0.25) || !near(s.Newness, 0.5) {
		t.Fatalf("%+v want score %v", s, want)
	}
	if s.FormulaVersion != 1 || !s.ComputedAt.Equal(now) {
		t.Fatalf("version/computed_at: %+v", s)
	}
}

func TestScoreStaysWithinDocumentedBounds(t *testing.T) {
	best := one(t, Input{StoreID: 1, PublishedAt: now, DistinctBuyers: 5, QualifyingOrders: 5, RatingCount: 1000, RatingAverage: 5})
	worst := one(t, Input{StoreID: 2, PublishedAt: now.AddDate(-5, 0, 0), SellerCancelledOrders: 1 << 40})
	if best.Score > 1 || worst.Score < -PenaltyWeight || worst.Score > 0 {
		t.Fatalf("best=%v worst=%v", best.Score, worst.Score)
	}
	for _, c := range []float64{best.Demand, best.Quality, best.Newness, worst.ReliabilityPenalty} {
		if c < 0 || c > 1 {
			t.Fatalf("component out of range: %v", c)
		}
	}
}

func TestZeroDataRanksByNewnessThenPublishedAt(t *testing.T) {
	// 沒有任何訂單與評價：所有店的需求、品質、扣分都相同，分數只差新店加分，所以越新的店分數越高。
	got, err := Compute([]Input{
		{StoreID: 1, PublishedAt: now.AddDate(0, 0, -10)},
		{StoreID: 2, PublishedAt: now.AddDate(0, 0, -2)},
		{StoreID: 3, PublishedAt: now.AddDate(0, 0, -90)},
		{StoreID: 4, PublishedAt: now.AddDate(0, 0, -60)},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !(got[1].Score > got[0].Score && got[0].Score > got[2].Score) {
		t.Fatalf("scores: %v %v %v", got[1].Score, got[0].Score, got[2].Score)
	}
	// 超過 30 天的店分數相同（同分），由讀取端的 published_at DESC、store_id DESC 決定順序。
	if got[2].Score != got[3].Score {
		t.Fatalf("old stores should tie: %v %v", got[2].Score, got[3].Score)
	}
}

func TestComputeKeepsInputOrderAndEchoesInputs(t *testing.T) {
	in := []Input{{StoreID: 9, PublishedAt: now}, {StoreID: 3, PublishedAt: now}}
	got, err := Compute(in, now)
	if err != nil || len(got) != 2 || got[0].StoreID != 9 || got[1].StoreID != 3 {
		t.Fatalf("%v %+v", err, got)
	}
	if empty, err := Compute(nil, now); err != nil || len(empty) != 0 {
		t.Fatalf("empty: %v %v", empty, err)
	}
}

func TestComputeRejectsBadUpstreamData(t *testing.T) {
	ok := Input{StoreID: 1, PublishedAt: now}
	for name, in := range map[string]Input{
		"store id":           {StoreID: 0, PublishedAt: now},
		"negative buyers":    {StoreID: 1, DistinctBuyers: -1},
		"buyers > orders":    {StoreID: 1, DistinctBuyers: 3, QualifyingOrders: 2},
		"negative cancelled": {StoreID: 1, SellerCancelledOrders: -1},
		"negative count":     {StoreID: 1, RatingCount: -1},
		"nan average":        {StoreID: 1, RatingCount: 1, RatingAverage: math.NaN()},
		"average over 5":     {StoreID: 1, RatingCount: 1, RatingAverage: 5.1},
		"average no reviews": {StoreID: 1, RatingAverage: 4},
	} {
		if _, err := Compute([]Input{in}, now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Compute([]Input{ok, ok}, now); err == nil {
		t.Error("duplicate store accepted")
	}
}
