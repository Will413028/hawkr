package social

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

type declaration struct {
	capability seller.Capability
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const (
	buyerOnlyReason    = "只有買家身分能做的動作（店家沒有對應的動作），沒有賣家能力可宣告"
	scheduleListReason = "social.ListPendingSchedules 只以 handler 推導的店家 ID 讀這家店待發布的排程（含代行排的，店主要能看見並取消），模組不檢查賣家能力；取消（CancelPostSchedule）才要 CapMarketing"
	byResourceReason   = "互動者由伺服器依資源歸屬推導（呼叫者此刻的店擁有該貼文或討論區時是店家，否則是買家）；是店家身分時模組才檢查賣家能力（seller.Authorize），不是則不查，所以無法靜態宣告"
)

var declarations = map[string]declaration{
	socialv1alpha1connect.SocialAppServiceCreatePostProcedure:            requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceUpdatePostProcedure:            requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServicePublishPostProcedure:           requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceSchedulePostProcedure:          requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceUnpublishPostProcedure:         requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceCancelPostScheduleProcedure:    requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceListPendingSchedulesProcedure:  exempt(scheduleListReason),
	socialv1alpha1connect.SocialAppServiceDeletePostProcedure:            requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceGetMyPostProcedure:             requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceListMyPostsProcedure:           requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceListStoreCommentsProcedure:     requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceGetStoreSocialSummaryProcedure: requires(seller.CapContentContract),

	socialv1alpha1connect.SocialAppServiceListMySanctionsProcedure:     exempt(buyerOnlyReason),
	socialv1alpha1connect.SocialAppServiceListMyHiddenContentProcedure: exempt(buyerOnlyReason),

	socialv1alpha1connect.SocialAppServiceSharePostProcedure:        exempt(buyerOnlyReason),
	socialv1alpha1connect.SocialAppServiceLikePostProcedure:         exempt(buyerOnlyReason),
	socialv1alpha1connect.SocialAppServiceUnlikePostProcedure:       exempt(buyerOnlyReason),
	socialv1alpha1connect.SocialAppServiceLikeCommentProcedure:      exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceUnlikeCommentProcedure:    exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceCreateCommentProcedure:    exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceUpdateCommentProcedure:    exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceDeleteCommentProcedure:    exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceCreateForumEntryProcedure: exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceUpdateForumEntryProcedure: exempt(byResourceReason),
	socialv1alpha1connect.SocialAppServiceDeleteForumEntryProcedure: exempt(byResourceReason),
}

func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialAppService")
}

func checkDeclarations(table map[string]declaration, svc protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/social", table, []protoreflect.ServiceDescriptor{svc}, func(d declaration) string {
		switch {
		case (d.capability == "") == (d.exemptReason == ""):
			return "的宣告必須是一個能力或一個有理由的豁免"
		case d.capability != "" && !slices.Contains(seller.Capabilities(), d.capability):
			return "宣告了不存在的能力 " + string(d.capability)
		}
		return ""
	})
}
