package coupon

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declaration 是 CouponAppService 一個方法的能力宣告（同 httpapi/review）：以店家身分動作的方法宣告需要的賣家能力，買家的方法明列豁免與理由。
// 宣告表是執行的依據：賣家方法由 actingStore 依它的能力呼叫 seller.ActingStore（停權與前置條件在那裡判斷），不另外寫第二份。
type declaration struct {
	capability seller.Capability
	// exemptReason 非空表示豁免。
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const buyerOnlyReason = "只有買家身分能做的動作（店家沒有對應的動作），沒有賣家能力可宣告"

// declarations 是每個 CouponAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應（NewApp 時檢查，缺了或多了都不能啟動）。
// 建立、列表、明細是行銷經營（CAP_MARKETING，唯讀也要，同舊 gateway 的路由）；結束活動是收縮型動作，停權的店家仍要能自清（CAP_CONTENT_CONTRACT）。
var declarations = map[string]declaration{
	couponv1alpha1connect.CouponAppServiceCreateCampaignProcedure: requires(seller.CapMarketing),
	couponv1alpha1connect.CouponAppServiceListCampaignsProcedure:  requires(seller.CapMarketing),
	couponv1alpha1connect.CouponAppServiceGetCampaignProcedure:    requires(seller.CapMarketing),
	couponv1alpha1connect.CouponAppServiceEndCampaignProcedure:    requires(seller.CapContentContract),

	couponv1alpha1connect.CouponAppServiceListMyVouchersProcedure:         exempt(buyerOnlyReason),
	couponv1alpha1connect.CouponAppServiceListClaimableCampaignsProcedure: exempt(buyerOnlyReason),
	couponv1alpha1connect.CouponAppServiceClaimCouponProcedure:            exempt(buyerOnlyReason),
}

// serviceDescriptor 是 CouponAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_coupon_v1alpha1_coupon_proto.Services().ByName("CouponAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個存在的能力或一個有理由的豁免。
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
