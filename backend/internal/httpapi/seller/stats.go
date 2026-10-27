package seller

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
)

// statsBudget 是店面頁讀每一項統計（粉絲、評價）的時間上限：統計是店面的附屬資訊，讀不到就不設欄位，不能讓一個慢的上游拖住
// 店面。沿用舊 seller-service 評價統計的 300 ms（reviewStatsTimeout）；粉絲數用同一個額度。店家卡片的批次評價讀取也用這個額度。
const statsBudget = 300 * time.Millisecond

// maxRatingBatch 是 Ratings.StoreRatingStats 一次最多接受的店家數（review-service 的上限）；卡片比這多時分批。
const maxRatingBatch = 100

// StatsSource 是店家統計與卡片評分的組裝：前台店面頁（GetStore）、賣家自己的店（GetMyStore）與所有顯示店家卡片的清單共用這一份，
// 讀不到的欄位一律不設，不變成 0，也不讓呼叫方失敗。三個依賴都是必要的，只能由 NewStatsSource 建立（建構處檢查，執行期不再判斷 nil）。
type StatsSource struct {
	followers FollowerCounts
	ratings   Ratings
	log       *slog.Logger
}

// ErrStatsSourceRequired 是建構子沒有收到 StatsSource（nil）：組裝錯誤，啟動就該失敗。StatsSource 只能由 NewStatsSource 取得（回
// 指標，非 nil 即可用），不是可以直接寫字面值的型別。
var ErrStatsSourceRequired = errors.New("httpapi/seller：需要由 NewStatsSource 建立的 StatsSource")

// ErrWebOriginRequired 是建構子沒有收到網站網域（零值）：入駐文件的網址一律是絕對網址，組裝錯誤，啟動就該失敗。
var ErrWebOriginRequired = errors.New("httpapi/seller：需要由 rpc.NewWebOrigin 建立的網站網域")

// NewStatsSource 建立統計組裝；粉絲來源、評價來源與 logger 缺任何一個都回錯誤（組裝錯誤，啟動就該失敗）。
func NewStatsSource(followers FollowerCounts, ratings Ratings, log *slog.Logger) (*StatsSource, error) {
	switch {
	case followers == nil:
		return nil, errors.New("httpapi/seller：StatsSource 需要粉絲數的來源")
	case ratings == nil:
		return nil, errors.New("httpapi/seller：StatsSource 需要評價統計的來源")
	case log == nil:
		return nil, errors.New("httpapi/seller：StatsSource 需要 logger")
	}
	return &StatsSource{followers: followers, ratings: ratings, log: log}, nil
}

// count32 把計數轉成契約的 int32；放不下時不設（回 nil），與「讀取失敗」同樣不會變成一個錯的數字。
func count32(n int64) *int32 {
	if n < 0 || n > math.MaxInt32 {
		return nil
	}
	v := int32(n)
	return &v
}

// Store 組出一家店的統計。粉絲來自 buyer、評價來自 review，兩個同時讀、各自有 statsBudget：任何一個失敗或逾時只讓它的欄位不設
// （記 warn），不會變成 0，也不拖垮或拖慢呼叫它的 RPC。兩個 goroutine 各寫自己的欄位。不檢查店家是否公開：呼叫方決定能不能讀。
func (s *StatsSource) Store(ctx context.Context, storeID int64) *pb.StoreStats {
	out := &pb.StoreStats{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(ctx, statsBudget)
		defer cancel()
		out.FollowerCount = s.followerCount(ctx, storeID)
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(ctx, statsBudget)
		defer cancel()
		out.ReviewCount, out.RatingAverage = s.reviewStats(ctx, storeID)
	}()
	wg.Wait()
	return out
}

func (s *StatsSource) followerCount(ctx context.Context, storeID int64) *int32 {
	counts, err := s.followers.SubscriberCounts(ctx, []int64{storeID})
	if err != nil {
		s.log.WarnContext(ctx, "seller：讀取粉絲數失敗，stats.follower_count 不設", "store_id", storeID, "error", err)
		return nil
	}
	n, ok := counts[storeID]
	if !ok {
		s.log.ErrorContext(ctx, "seller：粉絲數的回應沒有這家店，stats.follower_count 不設", "store_id", storeID)
		return nil
	}
	return count32(n)
}

func (s *StatsSource) reviewStats(ctx context.Context, storeID int64) (count *int32, average *float32) {
	ratings, err := s.ratings.StoreRatingStats(ctx, []int64{storeID})
	if err != nil {
		s.log.WarnContext(ctx, "seller：讀取評價統計失敗，stats.review_count 與 rating_average 不設", "store_id", storeID, "error", err)
		return nil, nil
	}
	return ratingFields(ratings, storeID)
}

// ratingFields 把評價來源對一家店的回答轉成契約的欄位：沒有出現在結果裡的店是確實沒有評價（評價數 0、平均不設）。
func ratingFields(ratings map[int64]RatingStats, storeID int64) (count *int32, average *float32) {
	r, reviewed := ratings[storeID]
	if !reviewed {
		return count32(0), nil
	}
	if count = count32(r.Count); count == nil {
		return nil, nil
	}
	a := float32(r.Average)
	return count, &a
}

// RateCards 替一批卡片補上評分（review_count、rating_average）。storeIDs 與 cards 一一對應（cards[i] 是 storeIDs[i] 的卡片，
// 呼叫方保證沒有重複的店家）。整批對評價來源只讀一次（超過 maxRatingBatch 家時每 maxRatingBatch 家一次），共用一個 statsBudget；
// 讀取成功時沒有出現在結果裡的店是確實沒有評價，失敗或逾時時那一批的兩個欄位都不設（記 warn），不讓呼叫方失敗。
func (s *StatsSource) RateCards(ctx context.Context, storeIDs []int64, cards []*commonpb.StoreCard) {
	if len(storeIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, statsBudget)
	defer cancel()
	for start := 0; start < len(storeIDs); start += maxRatingBatch {
		end := min(start+maxRatingBatch, len(storeIDs))
		ratings, err := s.ratings.StoreRatingStats(ctx, storeIDs[start:end])
		if err != nil {
			s.log.WarnContext(ctx, "seller：讀取店家卡片的評價統計失敗，這一批卡片的 review_count 與 rating_average 不設", "stores", end-start, "error", err)
			continue
		}
		for i := start; i < end; i++ {
			if cards[i] != nil {
				cards[i].ReviewCount, cards[i].RatingAverage = ratingFields(ratings, storeIDs[i])
			}
		}
	}
}
