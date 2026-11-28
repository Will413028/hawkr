package review

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1/reviewv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type declaration struct {
	capability   seller.Capability
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const (
	buyerOnlyReason = "只有買家身分能做的動作（店家沒有對應的動作），沒有賣家能力可宣告"
	ownStoreReason  = "review.ListOwnStoreReviews 只以 handler 推導的店家 ID 讀這家店的公開可讀評價，模組不檢查賣家能力，停權的店家仍可讀自己店的評價"
)

var declarations = map[string]declaration{
	reviewv1alpha1connect.ReviewAppServiceReplyReviewProcedure:         requires(seller.CapMarketing),
	reviewv1alpha1connect.ReviewAppServiceListOwnStoreReviewsProcedure: exempt(ownStoreReason),

	reviewv1alpha1connect.ReviewAppServiceGetReviewProcedure:     exempt(buyerOnlyReason),
	reviewv1alpha1connect.ReviewAppServiceCreateReviewProcedure:  exempt(buyerOnlyReason),
	reviewv1alpha1connect.ReviewAppServiceListMyReviewsProcedure: exempt(buyerOnlyReason),
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_review_v1alpha1_review_proto.Services().ByName("ReviewAppService")
}

func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/review", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability != "" && !slices.Contains(seller.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		}
		return ""
	})
}
