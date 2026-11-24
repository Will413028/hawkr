package coupon

import (
	"context"
	"slices"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// specOf 把請求的活動內容轉成模組的規則：折抵必須設 cash 或 percent 其中一個；時間必填。欄位格式的細節由模組檢查。
func specOf(in *pb.CampaignInput) (coupon.CampaignSpec, error) {
	spec := coupon.CampaignSpec{Title: in.GetTitle(), ThresholdAmount: in.GetThresholdAmount()}
	switch d := in.GetDiscount().(type) {
	case *pb.CampaignInput_Cash:
		spec.DiscountType, spec.DiscountAmount = coupon.DiscountCash, d.Cash.GetAmount()
	case *pb.CampaignInput_Percent:
		spec.DiscountType, spec.DiscountPercent, spec.MaxDiscountAmount = coupon.DiscountPercent, d.Percent.GetPercent(), d.Percent.GetMaxAmount()
	default:
		return spec, rpc.RequiredField("campaign.cash", "cash 或 percent 必須設定其中一個")
	}
	if in.GetStartTime() == nil {
		return spec, rpc.RequiredField("campaign.start_time", "start_time 必填")
	}
	if in.GetEndTime() == nil {
		return spec, rpc.RequiredField("campaign.end_time", "end_time 必填")
	}
	spec.StartAt, spec.EndAt = in.GetStartTime().AsTime(), in.GetEndTime().AsTime()
	return spec, nil
}

// parseBuyerIDs 解析名單裡的買家 ID；格式錯誤指出是第幾個。
func parseBuyerIDs(raw []string) ([]int64, error) {
	out := make([]int64, 0, len(raw))
	for i, r := range raw {
		id, err := rpc.ParseID("campaign.designated_buyer_ids["+strconv.Itoa(i)+"]", r)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func voucherStoreIDs(vs []coupon.VoucherView) []int64 {
	var ids []int64
	for _, v := range vs {
		if v.Campaign.StoreID != nil && !slices.Contains(ids, *v.Campaign.StoreID) {
			ids = append(ids, *v.Campaign.StoreID)
		}
	}
	return ids
}

func campaignStoreIDs(cs []coupon.Campaign) []int64 {
	var ids []int64
	for _, c := range cs {
		if c.StoreID != nil && !slices.Contains(ids, *c.StoreID) {
			ids = append(ids, *c.StoreID)
		}
	}
	return ids
}

// storeCards 讀取店家卡片（key 是 store_id），沒有卡片（從沒公開過或不存在）的店家不在 map 裡。讀取失敗整個請求失敗（Unavailable），不降級成沒有店家。
func (h *App) storeCards(ctx context.Context, ids []int64) (map[int64]*commonpb.StoreCard, error) {
	out := make(map[int64]*commonpb.StoreCard, len(ids))
	for start := 0; start < len(ids); start += seller.MaxBatchSize {
		cards, err := h.stores.StoreCards(ctx, ids[start:min(start+seller.MaxBatchSize, len(ids))])
		if err != nil {
			return nil, apperr.WithCause(apperr.Unavailable("暫時無法讀取店家"), err)
		}
		for _, c := range cards {
			card, err := httpseller.ToStoreCard(h.images, c)
			if err != nil {
				return nil, err
			}
			out[c.StoreID] = card
		}
	}
	return out, nil
}

// designated 把名單的買家 ID 補上暱稱（buyer 的顯示資料）；讀取失敗整個請求失敗。已刪除的買家不設名稱。
func (h *App) designated(ctx context.Context, ids []int64) ([]*pb.DesignatedBuyer, error) {
	displays := make(map[int64]buyer.Display, len(ids))
	for start := 0; start < len(ids); start += buyer.MaxBatchSize {
		rows, err := h.buyers.BatchDisplay(ctx, ids[start:min(start+buyer.MaxBatchSize, len(ids))])
		if err != nil {
			return nil, err
		}
		for _, d := range rows {
			displays[d.BuyerID] = d
		}
	}
	out := make([]*pb.DesignatedBuyer, len(ids))
	for i, id := range ids {
		d := displays[id]
		b := &pb.DesignatedBuyer{BuyerId: rpc.FormatID(id)}
		if !d.Deleted {
			b.Name = d.DisplayName
		}
		out[i] = b
	}
	return out, nil
}

func toCampaign(c coupon.Campaign, cards map[int64]*commonpb.StoreCard) *pb.Campaign {
	out := &pb.Campaign{
		Id: c.ID.String(), Title: c.Title, ThresholdAmount: c.ThresholdAmount,
		TargetType: rpc.EnumToProto[pb.TargetType](c.TargetType),
		StartTime:  timestamppb.New(c.StartAt), EndTime: timestamppb.New(c.EndAt),
	}
	switch c.DiscountType {
	case coupon.DiscountCash:
		out.Discount = &pb.Campaign_Cash{Cash: &pb.CashDiscount{Amount: c.DiscountAmount}}
	case coupon.DiscountPercent:
		out.Discount = &pb.Campaign_Percent{Percent: &pb.PercentDiscount{Percent: c.DiscountPercent, MaxAmount: c.MaxDiscountAmount}}
	}
	if c.StoreID != nil {
		id := rpc.FormatID(*c.StoreID)
		out.StoreId = &id
		out.Store = cards[*c.StoreID]
	}
	return out
}

func toVoucher(v coupon.VoucherView, cards map[int64]*commonpb.StoreCard, checkout bool) *pb.Voucher {
	out := &pb.Voucher{
		Id: v.Voucher.ID.String(), Campaign: toCampaign(v.Campaign, cards),
		Status: rpc.EnumToProto[pb.VoucherStatus](v.Voucher.Status), Origin: rpc.EnumToProto[pb.VoucherOrigin](v.Voucher.Origin),
		Expired: v.Expired, CreateTime: timestamppb.New(v.Voucher.CreatedAt),
	}
	if checkout {
		d := v.EstimatedDiscount
		out.EstimatedDiscount = &d
	}
	return out
}

// toSellerCampaign 轉成店家看到的活動。店家看的都是自己的活動，不附店家卡片（只有 store_id）；expired 用 Campaign.Expired（到期的唯一定義）對 now 判斷。
func toSellerCampaign(c coupon.Campaign, now time.Time) *pb.SellerCampaign {
	out := &pb.SellerCampaign{
		Campaign: toCampaign(c, nil), Status: rpc.EnumToProto[pb.CampaignStatus](c.Status),
		Expired: c.Expired(now), IssuedCount: c.IssuedCount, CreateTime: timestamppb.New(c.CreatedAt),
	}
	if c.TotalLimit != nil {
		l := *c.TotalLimit
		out.TotalIssueLimit = &l
	}
	if c.EndedAt != nil {
		out.EndedTime = timestamppb.New(*c.EndedAt)
	}
	return out
}
