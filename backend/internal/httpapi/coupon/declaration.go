package coupon

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type declaration struct {
	capability seller.Capability
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const buyerOnlyReason = "只有買家身分能做的動作（店家沒有對應的動作），沒有賣家能力可宣告"

var declarations = map[string]declaration{
	couponv1alpha1connect.CouponAppServiceCreateCampaignProcedure: requires(seller.CapMarketing),
	couponv1alpha1connect.CouponAppServiceListCampaignsProcedure:  requires(seller.CapContentContract),
	couponv1alpha1connect.CouponAppServiceGetCampaignProcedure:    requires(seller.CapContentContract),
	couponv1alpha1connect.CouponAppServiceEndCampaignProcedure:    requires(seller.CapContentContract),

	couponv1alpha1connect.CouponAppServiceListMyVouchersProcedure:         exempt(buyerOnlyReason),
	couponv1alpha1connect.CouponAppServiceListClaimableCampaignsProcedure: exempt(buyerOnlyReason),
	couponv1alpha1connect.CouponAppServiceClaimCouponProcedure:            exempt(buyerOnlyReason),
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_coupon_v1alpha1_coupon_proto.Services().ByName("CouponAppService")
}

func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/coupon", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability != "" && !slices.Contains(seller.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		}
		return ""
	})
}
