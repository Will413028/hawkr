package grpc

import (
	"strconv"

	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/review-service/internal/domain"
)

// toReplyInfo 將 domain reply 欄位轉為 proto ReplyInfo（#1174）。
// 已回 / 未回唯一判斷源 = RepliedAt 是否為零值；未回覆回 nil。
// 1:1 設計無獨立 reply PK，ReplyInfo.id 留空。
func toReplyInfo(r *domain.Review) *pb.ReplyInfo {
	if r.RepliedAt.IsZero() {
		return nil
	}
	return &pb.ReplyInfo{
		Content:   r.ReplyContent,
		CreatedAt: timeutil.FormatISO8601(r.RepliedAt),
	}
}

func toProtoReview(r *domain.Review) *pb.OrderReviewInfo {
	info := &pb.OrderReviewInfo{
		Id:             strconv.FormatInt(r.ID, 10),
		OrderNo:        r.OrderNo,
		OrderItemId:    strconv.FormatInt(r.OrderItemID, 10),
		BuyerId:        strconv.FormatInt(r.BuyerID, 10),
		StoreId:        strconv.FormatInt(r.StoreID, 10),
		Rating:         r.Rating,
		Content:        r.Content,
		ImageKeys:      r.ImageKeys, // objectKey 陣列（49 §6.6）；顯示 URL 由 Gateway 組
		SkuName:        r.SkuName,
		ItemImageKey:   r.ItemImageKey, // objectKey（49 §6.6）
		Price:          strconv.FormatInt(r.Price, 10),
		CreatedAt:      timeutil.FormatISO8601(r.CreatedAt),
		BuyerName:      r.BuyerName,
		BuyerAvatarKey: r.BuyerAvatar, // objectKey/外部 URL（49 §6.6 S2）；顯示 URL 由 Gateway 組
		ProductId:      strconv.FormatInt(r.ProductID, 10),
		Reply:          toReplyInfo(r),
	}
	if info.ImageKeys == nil {
		info.ImageKeys = []string{}
	}
	return info
}

func toProtoReviewInfo(r *domain.Review) *pb.ReviewInfo {
	info := &pb.ReviewInfo{
		Id:            strconv.FormatInt(r.ID, 10),
		ProductId:     strconv.FormatInt(r.ProductID, 10),
		BuyerId:       strconv.FormatInt(r.BuyerID, 10),
		UserName:      r.BuyerName,
		UserDeleted:   r.BuyerDeleted,
		UserAvatarKey: r.BuyerAvatar, // objectKey/外部 URL（49 §6.6 S2）；顯示 URL 由 Gateway 組
		Rating:        r.Rating,
		Content:       r.Content,
		ImageKeys:     r.ImageKeys, // objectKey 陣列（49 §6.6）；顯示 URL 由 Gateway 組
		CreatedAt:     timeutil.FormatISO8601(r.CreatedAt),
		Reply:         toReplyInfo(r),
	}
	if info.ImageKeys == nil {
		info.ImageKeys = []string{}
	}
	return info
}
