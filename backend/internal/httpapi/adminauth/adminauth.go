// Package adminauth 執行後台的逐方法授權（ADR 2026-10-04-admin-roles）：每支 admin 方法與每條 /api/files/admin/… 路由
// 宣告一個 platform/adminperm 的權限，或明列的豁免；啟動時對照 service 的 descriptor，缺漏、多餘、權限不在目錄裡都
// 不能啟動，沒有宣告的 procedure 一律拒絕（default deny）。誰是管理員由 Cloudflare Access 與 identity 的認證決定，
// 這裡只回答「這位管理員能不能做這件事」。
package adminauth

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1/reviewqueuev1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// declaration 是一支 admin 方法的授權宣告：要不然需要一個權限，要不然明列豁免與理由。
type declaration struct {
	permission adminperm.Permission
	// exemptReason 非空表示豁免。
	exemptReason string
}

func requires(p adminperm.Permission) declaration { return declaration{permission: p} }

func exempt(reason string) declaration { return declaration{exemptReason: reason} }

// sessionReason 是 GetSession 與 SignOut 豁免的理由：「我是誰」與登出對任何已登入的管理員都允許，不屬於任何權限。
const sessionReason = "回答「我是誰」與登出，對任何已通過驗證的管理員都允許，不屬於任何權限"

// declarations 是每個 admin 方法的宣告，key 是生成的 procedure 常數。方法與宣告必須一一對應（Check 在啟動時檢查）。
var declarations = map[string]declaration{
	buyerv1alpha1connect.BuyerAdminServiceGetBuyerStatsProcedure:        requires(adminperm.BuyerBuyerStatsGet),
	buyerv1alpha1connect.BuyerAdminServiceListMembershipPlansProcedure:  requires(adminperm.BuyerMembershipPlansList),
	buyerv1alpha1connect.BuyerAdminServiceUpdateMembershipPlanProcedure: requires(adminperm.BuyerMembershipPlansUpdate),

	contentv1alpha1connect.ContentAdminServiceListBannersProcedure:          requires(adminperm.ContentBannersList),
	contentv1alpha1connect.ContentAdminServiceGetBannerProcedure:            requires(adminperm.ContentBannersGet),
	contentv1alpha1connect.ContentAdminServiceCreateBannerProcedure:         requires(adminperm.ContentBannersCreate),
	contentv1alpha1connect.ContentAdminServiceUpdateBannerProcedure:         requires(adminperm.ContentBannersUpdate),
	contentv1alpha1connect.ContentAdminServiceDeleteBannerProcedure:         requires(adminperm.ContentBannersDelete),
	contentv1alpha1connect.ContentAdminServiceListAppVersionGatesProcedure:  requires(adminperm.ContentAppVersionGatesList),
	contentv1alpha1connect.ContentAdminServiceCreateAppVersionGateProcedure: requires(adminperm.ContentAppVersionGatesCreate),
	contentv1alpha1connect.ContentAdminServiceUpdateAppVersionGateProcedure: requires(adminperm.ContentAppVersionGatesUpdate),

	identityv1alpha1connect.IdentityAdminServiceGetSessionProcedure:    exempt(sessionReason),
	identityv1alpha1connect.IdentityAdminServiceListAccountsProcedure:  requires(adminperm.IdentityAccountsList),
	identityv1alpha1connect.IdentityAdminServiceGetAccountProcedure:    requires(adminperm.IdentityAccountsGet),
	identityv1alpha1connect.IdentityAdminServiceDeleteAccountProcedure: requires(adminperm.IdentityAccountsDelete),
	identityv1alpha1connect.IdentityAdminServiceSignOutProcedure:       exempt(sessionReason),

	sellerv1alpha1connect.SellerAdminServiceListStoresProcedure:              requires(adminperm.SellerStoresList),
	sellerv1alpha1connect.SellerAdminServiceGetStoreStatsProcedure:           requires(adminperm.SellerStoreStatsGet),
	sellerv1alpha1connect.SellerAdminServiceGetStoreProcedure:                requires(adminperm.SellerStoresGet),
	sellerv1alpha1connect.SellerAdminServiceListKYCApplicationsProcedure:     requires(adminperm.SellerKYCApplicationsList),
	sellerv1alpha1connect.SellerAdminServiceGetKYCApplicationProcedure:       requires(adminperm.SellerKYCApplicationsGet),
	sellerv1alpha1connect.SellerAdminServiceListKYCRevealsProcedure:          requires(adminperm.SellerKYCRevealsList),
	sellerv1alpha1connect.SellerAdminServiceListKYCDocumentAccessesProcedure: requires(adminperm.SellerKYCDocumentAccessesList),
	sellerv1alpha1connect.SellerAdminServiceApproveKYCProcedure:              requires(adminperm.SellerKYCApplicationsApprove),
	sellerv1alpha1connect.SellerAdminServiceRejectKYCProcedure:               requires(adminperm.SellerKYCApplicationsReject),
	sellerv1alpha1connect.SellerAdminServiceRequeryKYCRegistrationProcedure:  requires(adminperm.SellerKYCApplicationsRequery),
	sellerv1alpha1connect.SellerAdminServiceRevealKYCSensitiveProcedure:      requires(adminperm.SellerKYCRevealsCreate),
	sellerv1alpha1connect.SellerAdminServiceGetPayUniRegistrationProcedure:   requires(adminperm.SellerPayUniRegistrationsGet),
	sellerv1alpha1connect.SellerAdminServiceRebindPayUniProcedure:            requires(adminperm.SellerPayUniRegistrationsRebind),
	sellerv1alpha1connect.SellerAdminServiceListPayUniObservationsProcedure:  requires(adminperm.SellerPayUniObservationsList),
	sellerv1alpha1connect.SellerAdminServiceRefreshPayUniStatusProcedure:     requires(adminperm.SellerPayUniRegistrationsRefresh),

	sellerv1alpha1connect.SellerGovernanceAdminServiceListReportsProcedure:                 requires(adminperm.SellerReportsList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetReportProcedure:                   requires(adminperm.SellerReportsGet),
	sellerv1alpha1connect.SellerGovernanceAdminServiceMarkReportSupplementsSeenProcedure:   requires(adminperm.SellerReportsMarkSupplementsSeen),
	sellerv1alpha1connect.SellerGovernanceAdminServiceTriageReportProcedure:                requires(adminperm.SellerReportsTriage),
	sellerv1alpha1connect.SellerGovernanceAdminServiceResolveReportProcedure:               requires(adminperm.SellerReportsResolve),
	sellerv1alpha1connect.SellerGovernanceAdminServiceMoveReportProcedure:                  requires(adminperm.SellerReportsMove),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListCasesProcedure:                   requires(adminperm.SellerCasesList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetCaseProcedure:                     requires(adminperm.SellerCasesGet),
	sellerv1alpha1connect.SellerGovernanceAdminServiceDecideCaseProcedure:                  requires(adminperm.SellerCasesDecide),
	sellerv1alpha1connect.SellerGovernanceAdminServiceChangeCaseSeverityProcedure:          requires(adminperm.SellerCasesChangeSeverity),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateCaseInquiryProcedure:           requires(adminperm.SellerCaseInquiriesCreate),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListCaseInquiriesProcedure:           requires(adminperm.SellerCaseInquiriesList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRecordCaseInquiryReplyProcedure:      requires(adminperm.SellerCaseInquiriesRecordReply),
	sellerv1alpha1connect.SellerGovernanceAdminServiceWithdrawCaseInquiryProcedure:         requires(adminperm.SellerCaseInquiriesWithdraw),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateAuthorityOrderProcedure:        requires(adminperm.SellerAuthorityOrdersCreate),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListAuthorityOrdersProcedure:         requires(adminperm.SellerAuthorityOrdersList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetAuthorityOrderProcedure:           requires(adminperm.SellerAuthorityOrdersGet),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCompleteAuthorityOrderProcedure:      requires(adminperm.SellerAuthorityOrdersComplete),
	sellerv1alpha1connect.SellerGovernanceAdminServiceExtendAuthorityOrderHoldProcedure:    requires(adminperm.SellerAuthorityOrdersExtendHold),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRecordAuthorityTargetNoticeProcedure: requires(adminperm.SellerAuthorityOrdersRecordNotice),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRevokeAuthorityOrderProcedure:        requires(adminperm.SellerAuthorityOrdersRevoke),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateCaseNoteProcedure:              requires(adminperm.SellerCaseNotesCreate),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListCaseNotesProcedure:               requires(adminperm.SellerCaseNotesList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetViolationProcedure:                requires(adminperm.SellerViolationsGet),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListViolationsProcedure:              requires(adminperm.SellerViolationsList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureProcedure:               requires(adminperm.SellerMeasuresCreate),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetMeasureProcedure:                  requires(adminperm.SellerMeasuresGet),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasuresProcedure:                requires(adminperm.SellerMeasuresList),
	sellerv1alpha1connect.SellerGovernanceAdminServiceUpdateMeasureProcedure:               requires(adminperm.SellerMeasuresUpdate),
	sellerv1alpha1connect.SellerGovernanceAdminServiceEndMeasureProcedure:                  requires(adminperm.SellerMeasuresEnd),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRevokeMeasureProcedure:               requires(adminperm.SellerMeasuresRevoke),
	sellerv1alpha1connect.SellerGovernanceAdminServiceScheduleMeasureReviewProcedure:       requires(adminperm.SellerMeasuresScheduleReview),
	sellerv1alpha1connect.SellerGovernanceAdminServiceDecideAppealProcedure:                requires(adminperm.SellerMeasuresDecideAppeal),
	sellerv1alpha1connect.SellerGovernanceAdminServiceDecideRemediationProcedure:           requires(adminperm.SellerMeasuresDecideRemediation),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureNoteProcedure:           requires(adminperm.SellerMeasureNotesCreate),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasureNotesProcedure:            requires(adminperm.SellerMeasureNotesList),

	reviewqueuev1alpha1connect.ReviewQueueAdminServiceListReviewQueueProcedure: requires(adminperm.ReviewQueueItemsList),

	sellerv1alpha1connect.SellerProspectAdminServiceListProspectsProcedure:            requires(adminperm.SellerProspectsList),
	sellerv1alpha1connect.SellerProspectAdminServiceCreateProspectProcedure:           requires(adminperm.SellerProspectsCreate),
	sellerv1alpha1connect.SellerProspectAdminServicePreviewProspectProcedure:          requires(adminperm.SellerProspectsPreview),
	sellerv1alpha1connect.SellerProspectAdminServiceGetProspectProcedure:              requires(adminperm.SellerProspectsGet),
	sellerv1alpha1connect.SellerProspectAdminServiceUpdateProspectProcedure:           requires(adminperm.SellerProspectsUpdate),
	sellerv1alpha1connect.SellerProspectAdminServiceArchiveProspectProcedure:          requires(adminperm.SellerProspectsArchive),
	sellerv1alpha1connect.SellerProspectAdminServiceRestoreProspectProcedure:          requires(adminperm.SellerProspectsRestore),
	sellerv1alpha1connect.SellerProspectAdminServiceMergeProspectsProcedure:           requires(adminperm.SellerProspectsMerge),
	sellerv1alpha1connect.SellerProspectAdminServiceConvertProspectProcedure:          requires(adminperm.SellerProspectsConvert),
	sellerv1alpha1connect.SellerProspectAdminServiceListProspectDuplicatesProcedure:   requires(adminperm.SellerProspectDuplicatesList),
	sellerv1alpha1connect.SellerProspectAdminServiceListProspectTaxIDMatchesProcedure: requires(adminperm.SellerProspectTaxIDMatchesList),
	sellerv1alpha1connect.SellerProspectAdminServiceMarkProspectsDistinctProcedure:    requires(adminperm.SellerProspectDistinctPairsCreate),
}

// checkDeclarations 雙向對照宣告表與 admin service 的方法（rpc.CheckDeclarations），並檢查每個宣告恰好是一個目錄裡的
// 權限或一個有理由的豁免。
func checkDeclarations(table map[string]declaration, svcs []protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/adminauth", table, svcs, func(d declaration) string {
		switch {
		case (d.permission == "") == (d.exemptReason == ""):
			return "的宣告必須是一個權限或一個有理由的豁免"
		case d.permission != "" && !adminperm.InCatalog(d.permission):
			return "宣告了不在目錄裡的權限 " + string(d.permission)
		}
		return ""
	})
}

// Check 對照宣告表與實際掛載的 admin service（ADR 2026-10-04-admin-roles：「宣告＝掛載」）：每個掛載的方法都有宣告、
// 沒有指向未掛載方法的宣告、每個宣告合法。掛載的 service 由呼叫方從實際掛上的 handler 取得，不由這個套件列舉。
func Check(mounted []protoreflect.ServiceDescriptor) error {
	return checkDeclarations(declarations, mounted)
}

// Procedures 是宣告表裡的全部 procedure，已排序。啟動時 Check 保證它與掛載的 admin 方法一致，測試以它逐一探測。
func Procedures() []string {
	return slices.Sorted(maps.Keys(declarations))
}

// NewInterceptor 建立掛在每個 Connect service 的 interceptor，要放在 rpc.NewInterceptor 之後（內層），錯誤才會被轉換並記
// log。非 admin audience 的 procedure 直接放行；admin 的每個請求依宣告表處理：需要權限的方法檢查 principal 的角色是否
// 允許，不允許回 permission_denied，並以 log 記下管理員與 procedure；豁免的方法不查；沒有宣告的 procedure 一律拒絕。
// 新增的 admin service 不必自己選用它，宣告表對不上掛載的方法則由 Check 擋下。
func NewInterceptor(log *slog.Logger) connect.Interceptor {
	return newInterceptor(declarations, log)
}

func newInterceptor(table map[string]declaration, log *slog.Logger) connect.Interceptor {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &interceptor{table: table, log: log}
}

type interceptor struct {
	table map[string]declaration
	log   *slog.Logger
}

// errDenied 是給使用者的拒絕，文字是共用文案目錄的 PERMISSION_DENIED（rpc.CommonText，語系取自 ctx）。
func errDenied(ctx context.Context) error {
	return apperr.PermissionDenied("%s", rpc.CommonText(ctx, connect.CodePermissionDenied).Text)
}

func (i *interceptor) enter(ctx context.Context, procedure string) error {
	if audience, _ := rpc.AudienceOf(procedure); audience != rpc.AudienceAdmin {
		return nil
	}
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAdmin {
		return errDenied(ctx)
	}
	d, ok := i.table[procedure]
	if !ok {
		i.deny(ctx, p, procedure, "沒有授權宣告")
		return errDenied(ctx)
	}
	if d.exemptReason != "" {
		return nil
	}
	if !p.AdminAllows(d.permission) {
		i.deny(ctx, p, procedure, string(d.permission))
		return errDenied(ctx)
	}
	return nil
}

func (i *interceptor) deny(ctx context.Context, p identity.Principal, procedure, need string) {
	i.log.WarnContext(ctx, "adminauth：拒絕", "admin_id", strconv.FormatInt(p.ID, 10), "procedure", procedure, "need", need)
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.enter(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.enter(ctx, conn.Spec().Procedure); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
