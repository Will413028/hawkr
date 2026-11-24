// Package procurementwire 是採購的 Connect API 兩端（買家 BuyerProcurementAppService、賣家 SellerProcurementAppService）共用的 proto 轉換：
// 賣家可見的詢價內容與報價內容兩邊看到的是同一個型別，轉換只在這裡。
package procurementwire

import (
	"fmt"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

// Enum 把模組的代碼（proto 列舉值名去掉前綴）轉成列舉；UNSPECIFIED 與不認得的代碼是不變量被破壞，回沒有分類的錯誤。
func Enum[E ~int32](values map[string]int32, prefix, kind, code string) (E, error) {
	v, ok := values[prefix+code]
	if !ok || v == 0 {
		return 0, fmt.Errorf("procurementwire：%s %q 沒有對應的列舉", kind, code)
	}
	return E(v), nil
}

// Date 把當日（UTC 午夜的時間）轉成 google.type.Date；nil 回 nil。
func Date(t *time.Time) *date.Date {
	if t == nil {
		return nil
	}
	return &date.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
}

// SellerFacing 把賣家看得到的詢價內容轉成 proto；publicationVersion 是這筆詢價的版本（買家的發布預覽還沒有發布，傳空字串）。
func SellerFacing(p procurement.SellerFacing, publicationVersion string) *pb.SellerFacingRfq {
	return &pb.SellerFacingRfq{
		Item: p.Item, ItemLabel: p.ItemLabel, Quantity: p.Quantity, Unit: p.Unit, Packaging: p.Packaging, DeliveryCity: p.DeliveryCity,
		DeliveryDate: Date(p.DeliveryDate), ResponseDeadline: Date(p.ResponseDate), BuyerNote: p.BuyerNote,
		BuyerRequestedRange: &pb.SellerFacingRequestedRange{
			BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit, LowerBoundAmount: p.LowerBoundAmount, UpperBoundAmount: p.UpperBoundAmount,
		},
		PublicationVersion: publicationVersion,
	}
}

// SellerQuote 把正式回價的報價內容轉成 proto。
func SellerQuote(q procurement.SellerQuote) (*pb.SellerQuote, error) {
	typ, err := Enum[pb.SellerResponseType](pb.SellerResponseType_value, "SELLER_RESPONSE_TYPE_", "回價方式", q.ResponseType)
	if err != nil {
		return nil, err
	}
	tax, err := Enum[pb.TaxInclusionBasis](pb.TaxInclusionBasis_value, "TAX_INCLUSION_BASIS_", "稅別", q.TaxInclusionBasis)
	if err != nil {
		return nil, err
	}
	out := &pb.SellerQuote{
		ResponseType: typ, SpecNote: q.SpecNote,
		UnitBasis: &pb.UnitBasis{Quantity: q.UnitBasisQuantity, Unit: q.UnitBasisUnit}, OfferedQuantity: q.OfferedQuantity, OfferedQuantityUnit: q.OfferedQuantityUnit,
		ConversionBasis: q.ConversionBasis, GoodsAmount: q.GoodsAmount, ShippingAmount: q.ShippingAmount, TaxAmount: q.TaxAmount, TaxInclusionBasis: tax,
		TotalAmount: q.TotalAmount, LeadTimeDays: q.LeadTimeDays, ValidUntilTime: timestamppb.New(q.ValidUntil), TermsNote: q.TermsNote, DeviationDisclosure: q.DeviationDisclosure,
	}
	if q.PromisedDelivery != nil {
		out.PromisedDeliveryTime = timestamppb.New(*q.PromisedDelivery)
	}
	return out, nil
}
