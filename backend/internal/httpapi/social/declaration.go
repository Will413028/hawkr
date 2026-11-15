package social

import (
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// declaration 是 SocialAppService 一個方法的能力宣告（ADR 2026-09-29-seller-suspension-and-business-identity-in-monolith 文末
// 「SocialAppService 的能力宣告（A7-2c）」）：要不然是以店家身分動作時需要的賣家能力，要不然明列「依資源判斷」的豁免與理由。
//
// 宣告表只記錄與對照，不執行：能力的檢查在 social 模組裡（seller.Authorize），帳號路徑與代行路徑走同一個入口；handler 不檢查。
// 宣告與模組的實際檢查由 TestOwnerDeclarationsMatchTheModuleChecks 對齊。
type declaration struct {
	capability seller.Capability
	// exemptReason 非空表示豁免。
	exemptReason string
}

func requires(c seller.Capability) declaration { return declaration{capability: c} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

const (
	buyerOnlyReason  = "只有買家身分能做的動作（店家沒有對應的動作），沒有賣家能力可宣告"
	byResourceReason = "互動者由伺服器依資源歸屬推導（呼叫者此刻的店擁有該貼文或討論區時是店家，否則是買家）；是店家身分時模組才檢查賣家能力（seller.Authorize），不是則不查，所以無法靜態宣告"
)

// declarations 是每個 SocialAppService 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應
// （checkDeclarations 在 NewApp 時檢查，缺了或多了都不能啟動）。
var declarations = map[string]declaration{
	// 店主的方法：店家由 seller.CurrentStore 推導，能力與模組的檢查一致。新增與修改內容、發布、排程是行銷經營（CapMarketing，停權時被擋）；
	// 下架、刪除與讀取是收縮類（CapContentContract，停權時放行，ADR D3：收縮需要的讀取放行）。
	socialv1alpha1connect.SocialAppServiceCreatePostProcedure:            requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceUpdatePostProcedure:            requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServicePublishPostProcedure:           requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceSchedulePostProcedure:          requires(seller.CapMarketing),
	socialv1alpha1connect.SocialAppServiceUnpublishPostProcedure:         requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceDeletePostProcedure:            requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceGetMyPostProcedure:             requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceListMyPostsProcedure:           requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceListStoreCommentsProcedure:     requires(seller.CapContentContract),
	socialv1alpha1connect.SocialAppServiceGetStoreSocialSummaryProcedure: requires(seller.CapContentContract),

	// 作者讀自己的處分與被隱藏的內容（A7-D9）：只有買家身分的讀取，停權的帳號也要讀得到（效果宣告 EffectRead，internal/httpapi/effects.go）。
	socialv1alpha1connect.SocialAppServiceListMySanctionsProcedure:     exempt(buyerOnlyReason),
	socialv1alpha1connect.SocialAppServiceListMyHiddenContentProcedure: exempt(buyerOnlyReason),

	// 買家與店家共用的方法：依資源判斷。
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

// serviceDescriptor 是 SocialAppService 的 descriptor，宣告表以它為準。
func serviceDescriptor() protoreflect.ServiceDescriptor {
	return pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialAppService")
}

// checkDeclarations 雙向對照宣告表與 service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個存在的能力或一個有理由的豁免。
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
