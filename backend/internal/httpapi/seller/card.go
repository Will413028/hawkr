package seller

import (
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// ToStoreCard 把 seller 模組的店家卡片轉成契約的 StoreCard：SellerPublicService、買家的清單、推薦排行共用這一個轉換。
// 評分不在這裡：卡片的評分由 StatsSource.RateCards 對一批卡片批次組裝。個資清除之後沒有店名與所在地（town_code 是空字串），logo 與封面沒有時不設；圖片欄位存的不是 object key 時回 Unavailable。
func ToStoreCard(images rpc.Images, c model.StoreCard) (*commonpb.StoreCard, error) {
	out := &commonpb.StoreCard{
		Id: rpc.FormatID(c.StoreID), StoreCode: rpc.FormatID(c.StoreCode), Name: c.Name,
		Status: rpc.EnumToProto[commonpb.StoreCardStatus](c.Status), TownCode: deref(c.TownCode),
	}
	for _, image := range []struct {
		key *string
		dst **commonpb.Image
	}{{c.LogoKey, &out.Logo}, {c.CoverKey, &out.CoverImage}} {
		if image.key == nil {
			continue
		}
		if err := rpc.ObjectKey(*image.key); err != nil {
			return nil, err
		}
		*image.dst = images.Image(*image.key)
	}
	return out, nil
}
