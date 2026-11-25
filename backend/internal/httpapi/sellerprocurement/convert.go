package sellerprocurement

import (
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/procurementwire"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

// versionString 是版本號對外的不透明字串；0（沒有這個版本）是空字串。
func versionString(v int32) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(int64(v), 10)
}

// codeOf 把 proto 列舉轉成模組的代碼（值名去掉前綴）；UNSPECIFIED 是空字串（沒填），不認得的數值留給模組以「不是有效的值」擋下。
func codeOf(name, prefix string, zero bool) string {
	if zero {
		return ""
	}
	return strings.TrimPrefix(name, prefix)
}

// timeOf 取出選填的時間戳；沒設是 nil，不是合法時間戳回 INVALID_ARGUMENT，指向 path。
func timeOf(path string, ts *timestamppb.Timestamp) (*time.Time, error) {
	if ts == nil {
		return nil, nil
	}
	if err := ts.CheckValid(); err != nil {
		return nil, rpc.RequiredField(path, path+" 不是合法的時間")
	}
	t := ts.AsTime()
	return &t, nil
}

// quoteInputFromProto 把請求的報價內容轉成模組的輸入；prefix 是它在請求裡的欄位路徑（draft 或 quote）。內容的驗證（草稿只驗形狀與長度、正式回價驗完整）在模組。
func quoteInputFromProto(q *pb.SellerQuote, prefix string) (procurement.SellerQuoteInput, error) {
	promised, err := timeOf(prefix+".promised_delivery_time", q.GetPromisedDeliveryTime())
	if err != nil {
		return procurement.SellerQuoteInput{}, err
	}
	validUntil, err := timeOf(prefix+".valid_until_time", q.GetValidUntilTime())
	if err != nil {
		return procurement.SellerQuoteInput{}, err
	}
	return procurement.SellerQuoteInput{
		ResponseType:      codeOf(q.GetResponseType().String(), "SELLER_RESPONSE_TYPE_", q.GetResponseType() == pb.SellerResponseType_SELLER_RESPONSE_TYPE_UNSPECIFIED),
		SpecNote:          q.GetSpecNote(),
		UnitBasisQuantity: q.GetUnitBasis().GetQuantity(), UnitBasisUnit: q.GetUnitBasis().GetUnit(),
		OfferedQuantity: q.GetOfferedQuantity(), OfferedQuantityUnit: q.GetOfferedQuantityUnit(), ConversionBasis: q.GetConversionBasis(),
		GoodsAmount: q.GetGoodsAmount(), ShippingAmount: q.GetShippingAmount(), TaxAmount: q.GetTaxAmount(),
		TaxInclusionBasis: codeOf(q.GetTaxInclusionBasis().String(), "TAX_INCLUSION_BASIS_", q.GetTaxInclusionBasis() == pb.TaxInclusionBasis_TAX_INCLUSION_BASIS_UNSPECIFIED),
		TotalAmount:       q.GetTotalAmount(), LeadTimeDays: q.LeadTimeDays, PromisedDelivery: promised, ValidUntil: validUntil,
		TermsNote: q.GetTermsNote(), DeviationDisclosure: q.GetDeviationDisclosure(),
	}, nil
}

// quoteInputToProto 把草稿的內容轉回 proto（沒填的欄位是零值，讓 client 預填表單）；儲存的代碼不認得是不變量被破壞。
func quoteInputToProto(q procurement.SellerQuoteInput) (*pb.SellerQuote, error) {
	out := &pb.SellerQuote{
		SpecNote: q.SpecNote, OfferedQuantity: q.OfferedQuantity, OfferedQuantityUnit: q.OfferedQuantityUnit, ConversionBasis: q.ConversionBasis,
		GoodsAmount: q.GoodsAmount, ShippingAmount: q.ShippingAmount, TaxAmount: q.TaxAmount, TotalAmount: q.TotalAmount, LeadTimeDays: q.LeadTimeDays,
		TermsNote: q.TermsNote, DeviationDisclosure: q.DeviationDisclosure,
	}
	if q.UnitBasisQuantity != "" || q.UnitBasisUnit != "" {
		out.UnitBasis = &pb.UnitBasis{Quantity: q.UnitBasisQuantity, Unit: q.UnitBasisUnit}
	}
	var err error
	if q.ResponseType != "" {
		if out.ResponseType, err = procurementwire.Enum[pb.SellerResponseType](pb.SellerResponseType_value, "SELLER_RESPONSE_TYPE_", "回價方式", q.ResponseType); err != nil {
			return nil, err
		}
	}
	if q.TaxInclusionBasis != "" {
		if out.TaxInclusionBasis, err = procurementwire.Enum[pb.TaxInclusionBasis](pb.TaxInclusionBasis_value, "TAX_INCLUSION_BASIS_", "稅別", q.TaxInclusionBasis); err != nil {
			return nil, err
		}
	}
	if q.PromisedDelivery != nil {
		out.PromisedDeliveryTime = timestamppb.New(*q.PromisedDelivery)
	}
	if q.ValidUntil != nil {
		out.ValidUntilTime = timestamppb.New(*q.ValidUntil)
	}
	return out, nil
}

func summaryToProto(s procurement.SellerPublicationSummary) *pb.SellerPublicationSummary {
	return &pb.SellerPublicationSummary{
		PublicationId: s.PublicationID.String(), PublicationVersion: versionString(s.Version), SellerFacingPayloadHash: s.SellerFacingHash,
		PublishTime: timestamppb.New(s.PublishedAt),
	}
}

func itemToProto(it procurement.SellerPublicationItem) (*pb.SellerPublicationListItem, error) {
	out := &pb.SellerPublicationListItem{
		Publication: summaryToProto(it.Summary), SellerFacingPayload: procurementwire.SellerFacing(it.Payload, versionString(it.Summary.Version)), HasDraft: it.HasDraft,
	}
	if it.LatestResponseStatus != nil {
		status, err := procurementwire.Enum[pb.SellerResponseStatus](pb.SellerResponseStatus_value, "SELLER_RESPONSE_STATUS_", "回價狀態", *it.LatestResponseStatus)
		if err != nil {
			return nil, err
		}
		out.LatestResponseStatus = &status
	}
	return out, nil
}

func draftToProto(d *procurement.SellerDraft) (*pb.SellerResponseDraft, error) {
	if d == nil {
		return nil, nil
	}
	q, err := quoteInputToProto(d.Quote)
	if err != nil {
		return nil, err
	}
	return &pb.SellerResponseDraft{DraftVersion: versionString(d.Version), UpdateTime: timestamppb.New(d.UpdatedAt), Draft: q}, nil
}

func versionToProto(v procurement.SellerResponseVersion) (*pb.SellerResponseVersion, error) {
	status, err := procurementwire.Enum[pb.SellerResponseStatus](pb.SellerResponseStatus_value, "SELLER_RESPONSE_STATUS_", "回價狀態", v.Status)
	if err != nil {
		return nil, err
	}
	q, err := procurementwire.SellerQuote(v.Quote)
	if err != nil {
		return nil, err
	}
	return &pb.SellerResponseVersion{
		SellerResponseId: v.ResponseID.String(), SellerResponseVersion: versionString(v.Version), PublicationId: v.PublicationID.String(), Status: status,
		PayloadHash: v.PayloadHash, SubmitTime: timestamppb.New(v.SubmittedAt), SupersededByVersion: versionString(v.SupersededByVersion),
		OutsideBuyerRequestedRange: v.OutsideRange, Quote: q, Replayed: v.Replayed,
	}, nil
}

func detailToProto(d procurement.SellerPublicationDetail) (*pb.GetSellerPublicationResponse, error) {
	var err error
	out := &pb.GetSellerPublicationResponse{Publication: summaryToProto(d.Summary), SellerFacingPayload: procurementwire.SellerFacing(d.Payload, versionString(d.Summary.Version))}
	if d.Draft != nil {
		if out.Draft, err = draftToProto(d.Draft); err != nil {
			return nil, err
		}
	}
	if d.Latest != nil {
		if out.LatestResponse, err = versionToProto(*d.Latest); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func closureToProto(c procurement.SellerResponseClosure) *pb.SellerResponseClosure {
	out := &pb.SellerResponseClosure{
		SellerResponseId: c.ResponseID.String(), PublicationId: c.PublicationID.String(), LatestVersion: versionString(c.LatestVersion),
		WithdrawnVersion: versionString(c.WithdrawnVersion), WriteClosedReason: c.WriteClosedReason,
	}
	if c.WriteClosedAt != nil {
		out.WriteClosedTime = timestamppb.New(*c.WriteClosedAt)
	}
	return out
}
