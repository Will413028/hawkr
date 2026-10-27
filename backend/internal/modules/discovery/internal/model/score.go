// Package model 是推薦賣家排行的型別與分數公式。公式是純函式：同樣的輸入與時間必得同樣的分數，沒有 I/O。
package model

import (
	"fmt"
	"math"
	"time"
)

// 公式 v1（ADR 2026-10-03-recommended-sellers-scoring）。權重與先驗都在這裡；改任何一個值，或改公式的形狀，
// 都要把 FormulaVersion 加一，並更新 score_test.go 裡鎖住行為的案例。
const (
	// FormulaVersion 存進每一列，讓一份快照能說明它是用哪一版公式算的。
	FormulaVersion = 1

	// 分數 = DemandWeight·需求 + QualityWeight·品質 + NewnessWeight·新店 − PenaltyWeight·可靠度扣分。
	// 加分項的權重合計 1，所以分數的範圍是 [−PenaltyWeight, 1]。
	DemandWeight  = 0.5
	QualityWeight = 0.3
	NewnessWeight = 0.2
	PenaltyWeight = 0.5

	// StatsWindowDays 是訂單統計的時間窗（天）。
	StatsWindowDays = 30

	// RatingPriorWeight 是評分貝氏平均的先驗權重 C：相當於「全站平均」的虛擬評價數，評價少的店向全站平均收斂。
	RatingPriorWeight = 10.0
	// RatingFallbackMean 是所有候選店都沒有評價時的全站平均（評分的中點偏上，沒有資料時不偏袒任何店）。
	RatingFallbackMean = 4.0
	// RatingMin 與 RatingMax 是評分的刻度，品質以此正規化到 0..1。
	RatingMin = 1.0
	RatingMax = 5.0

	// PenaltyPrior 是可靠度扣分的平滑分母：取消數 /（成立數 + 取消數 + PenaltyPrior）。一筆取消不會讓幾乎沒有訂單的店
	// 立刻被重罰，取消占比要在訂單量夠大時才逼近 1。
	PenaltyPrior = 10.0

	// NewnessDays 是新店加分線性遞減到 0 的天數（從第一次可公開曝光起算）。
	NewnessDays = 30
)

// Input 是一家合格店家的原始訊號。
type Input struct {
	StoreID int64
	// PublishedAt 是這家店的公開時間（seller 的 ExposableStore.PublishedAt：第一次可公開曝光的時間），不是開店、申請
	// 或核准的時間：審核拖得久、或核准後才補齊 logo 的店不該在第一次曝光之前就用掉新店期。新店加分與同分時的排序都以它為準。
	PublishedAt time.Time

	DistinctBuyers        int64 // 時間窗內成立訂單的不重複買家數
	QualifyingOrders      int64 // 時間窗內成立的訂單數（賣家成交）
	SellerCancelledOrders int64 // 時間窗內賣家取消的訂單數

	RatingCount   int64
	RatingAverage float64 // RatingCount 為 0 時必須是 0
}

// Score 是一家店的分數與各成分（存進 discovery.seller_scores 的一列）。各成分都在 0..1。
type Score struct {
	Input
	Score              float64
	Demand             float64
	Quality            float64
	ReliabilityPenalty float64
	Newness            float64
	FormulaVersion     int
	ComputedAt         time.Time
}

// Validate 檢查上游給的訊號：負數、NaN 或超出刻度的評分都是上游的錯，回傳錯誤而不是默默修正成預設值。
func (in Input) Validate() error {
	switch {
	case in.StoreID <= 0:
		return fmt.Errorf("store_id 必須是正數：%d", in.StoreID)
	case in.DistinctBuyers < 0 || in.QualifyingOrders < 0 || in.SellerCancelledOrders < 0 || in.RatingCount < 0:
		return fmt.Errorf("店家 %d 的統計不可為負數", in.StoreID)
	case in.DistinctBuyers > in.QualifyingOrders:
		// 每個不重複買家至少有一筆成立訂單；違反代表上游的口徑或彙總有誤。
		return fmt.Errorf("店家 %d 的買家數 %d 大於成立訂單數 %d", in.StoreID, in.DistinctBuyers, in.QualifyingOrders)
	case math.IsNaN(in.RatingAverage) || in.RatingAverage < 0 || in.RatingAverage > RatingMax:
		return fmt.Errorf("店家 %d 的平均評分不合法：%v", in.StoreID, in.RatingAverage)
	case in.RatingCount == 0 && in.RatingAverage != 0:
		return fmt.Errorf("店家 %d 沒有評價卻有平均評分 %v", in.StoreID, in.RatingAverage)
	}
	return nil
}

// Compute 以公式 v1 算出每家店的分數，順序與 inputs 相同。需求以全體候選的最大值正規化、評分向全體候選的加權平均
// 收斂，所以分數是候選集合的函式：同一批 inputs 才有可比的分數。
func Compute(inputs []Input, now time.Time) ([]Score, error) {
	seen := make(map[int64]bool, len(inputs))
	var maxLogBuyers, ratingSum, ratingN float64
	for _, in := range inputs {
		if err := in.Validate(); err != nil {
			return nil, err
		}
		if seen[in.StoreID] {
			return nil, fmt.Errorf("店家 %d 重複", in.StoreID)
		}
		seen[in.StoreID] = true
		maxLogBuyers = max(maxLogBuyers, math.Log1p(float64(in.DistinctBuyers)))
		ratingSum += float64(in.RatingCount) * in.RatingAverage
		ratingN += float64(in.RatingCount)
	}
	mean := RatingFallbackMean
	if ratingN > 0 {
		mean = ratingSum / ratingN
	}
	out := make([]Score, len(inputs))
	for i, in := range inputs {
		s := Score{Input: in, FormulaVersion: FormulaVersion, ComputedAt: now.UTC()}
		s.Demand = demand(in.DistinctBuyers, maxLogBuyers)
		s.Quality = quality(in.RatingCount, in.RatingAverage, mean)
		s.ReliabilityPenalty = penalty(in.QualifyingOrders, in.SellerCancelledOrders)
		s.Newness = newness(in.PublishedAt, now)
		s.Score = clamp(DemandWeight*s.Demand+QualityWeight*s.Quality+NewnessWeight*s.Newness-PenaltyWeight*s.ReliabilityPenalty, -PenaltyWeight, 1)
		out[i] = s
	}
	return out, nil
}

// demand 是 log1p(買家數) 除以全體的最大值：取 log 讓頭部不會壓過其餘，max 為 0（沒有人有成交）時是 0。
func demand(buyers int64, maxLog float64) float64 {
	if maxLog == 0 {
		return 0
	}
	return clamp(math.Log1p(float64(buyers))/maxLog, 0, 1)
}

// quality 是評分的貝氏平均 (C·m + n·avg)/(C + n)，再從 [RatingMin, RatingMax] 正規化到 0..1。沒有評價時等於全站平均 m。
func quality(n int64, avg, mean float64) float64 {
	bayes := (RatingPriorWeight*mean + float64(n)*avg) / (RatingPriorWeight + float64(n))
	return clamp((bayes-RatingMin)/(RatingMax-RatingMin), 0, 1)
}

// penalty 是賣家取消的平滑占比：取消 /（成立 + 取消 + PenaltyPrior）。
func penalty(qualifying, cancelled int64) float64 {
	return float64(cancelled) / (float64(qualifying) + float64(cancelled) + PenaltyPrior)
}

// newness 是第一次可曝光起 NewnessDays 天內線性遞減的加分；時間在未來（時鐘偏差）時當作剛開始曝光。
func newness(publishedAt, now time.Time) float64 {
	age := now.Sub(publishedAt)
	return clamp(1-float64(age)/float64(NewnessDays*24*time.Hour), 0, 1)
}

func clamp(v, lo, hi float64) float64 { return min(max(v, lo), hi) }

// OrderStats 是一家店在統計時間窗內的訂單統計（port Orders 的回傳）。口徑由訂單的擁有者定義：成立訂單是已付款且沒有
// 全額退款的訂單，賣家取消是賣家造成的取消；discovery 不重新猜狀態集合。
type OrderStats struct {
	StoreID               int64
	DistinctBuyers        int64
	QualifyingOrders      int64
	SellerCancelledOrders int64
}

// RatingStats 是一家店的評價統計（port Reviews 的回傳）。沒有評價時 Count 與 Average 都是 0。
type RatingStats struct {
	Count   int64
	Average float64
}
