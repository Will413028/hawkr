// Package reviewservice 呼叫遷移期的 review-service。
package reviewservice

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"connectrpc.com/connect"

	reviewpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/review"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/review/reviewconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
)

const timeout = 5 * time.Second

// Client 讀取評論統計。
type Client struct {
	rpc reviewconnect.ReviewServiceClient
}

// RatingStats 是一家有評論的店的已解析統計（Count 至少 1）。
type RatingStats struct {
	Count   int64
	Average float64 // 0 到 5；舊服務只給到小數兩位
}

// New 建立以 h2c 呼叫並簽章的 client。
func New(addr, secret string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("reviewservice：缺少位址")
	}
	return &Client{rpc: reviewconnect.NewReviewServiceClient(grpcclient.HTTPClient(), "http://"+addr, connect.WithGRPC(), connect.WithInterceptors(grpcclient.Signer(secret, nil)))}, nil
}

// MaxStoreIDs 是 StoreRatingStats 一次最多接受的店家數（review-service ListStoreRatingStats 的上限）。
const MaxStoreIDs = 100

// StoreRatingStats 批次回傳店家的評論數與平均評分（最多 MaxStoreIDs 家，呼叫端分批）。回傳只含有評論的店，沒有出現
// 在 map 的店沒有評論。任何錯誤或不合法的回應（沒問的店、重複的店、不合法的數值）都回錯誤，不推定為沒有評論。
func (c *Client) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]RatingStats, error) {
	if len(storeIDs) == 0 {
		return map[int64]RatingStats{}, nil
	}
	if len(storeIDs) > MaxStoreIDs {
		return nil, fmt.Errorf("reviewservice：一次最多 %d 家店，收到 %d", MaxStoreIDs, len(storeIDs))
	}
	asked := make(map[int64]bool, len(storeIDs))
	raw := make([]string, 0, len(storeIDs))
	for _, id := range storeIDs {
		if id <= 0 {
			return nil, fmt.Errorf("reviewservice：店家 ID 不合法: %d", id)
		}
		if !asked[id] {
			asked[id] = true
			raw = append(raw, strconv.FormatInt(id, 10))
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.ListStoreRatingStats(ctx, connect.NewRequest(&reviewpb.ListStoreRatingStatsRequest{StoreIds: raw}))
	if err != nil {
		return nil, fmt.Errorf("reviewservice：ListStoreRatingStats：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("reviewservice：ListStoreRatingStats code=%d", resp.Msg.GetCode())
	}
	out := make(map[int64]RatingStats, len(resp.Msg.GetStats()))
	for _, st := range resp.Msg.GetStats() {
		id, err := strconv.ParseInt(st.GetStoreId(), 10, 64)
		if err != nil || !asked[id] {
			return nil, fmt.Errorf("reviewservice：回傳了沒有詢問的店家: %q", st.GetStoreId())
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("reviewservice：店家 %d 重複", id)
		}
		count, err := strconv.ParseInt(st.GetTotalCount(), 10, 64)
		if err != nil || count < 1 {
			return nil, fmt.Errorf("reviewservice：店家 %d 的評論數不合法: %q", id, st.GetTotalCount())
		}
		avg, err := strconv.ParseFloat(st.GetAvgRating(), 64)
		if err != nil || math.IsNaN(avg) || avg < 0 || avg > 5 {
			return nil, fmt.Errorf("reviewservice：店家 %d 的平均評分不合法: %q", id, st.GetAvgRating())
		}
		out[id] = RatingStats{Count: count, Average: avg}
	}
	return out, nil
}
