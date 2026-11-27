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

	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendarv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	catalogv1alpha1connect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/catalog/v1alpha1/catalogv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	couponv1alpha1connect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/coupon/v1alpha1/couponv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	imageaiv1alpha1connect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/imageai/v1alpha1/imageaiv1alpha1connect"
	moderationv1alpha1connect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/moderation/v1alpha1/moderationv1alpha1connect"
	notificationv1alpha1connect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1/notificationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/payment/v1alpha1/paymentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/reviewqueue/v1alpha1/reviewqueuev1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	socialv1alpha1connect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type auditMode string

const (
	auditAudited  auditMode = "audited"
	auditReadOnly auditMode = "readOnly"
	auditPending  auditMode = "auditPending"
)

type declaration struct {
	permission adminperm.Permission
	exemptReason string
	audit        auditMode
}

func requires(p adminperm.Permission, audit auditMode) declaration {
	return declaration{permission: p, audit: audit}
}

func exempt(reason string, audit auditMode) declaration {
	return declaration{exemptReason: reason, audit: audit}
}

const sessionReason = "回答「我是誰」與登出，對任何已通過驗證的管理員都允許，不屬於任何權限"

var declarations = map[string]declaration{
	bizcalendarv1alpha1connect.BizCalendarAdminServiceImportHolidayCalendarProcedure:      requires(adminperm.BizcalendarHolidayCalendarsImport, auditAudited),
	bizcalendarv1alpha1connect.BizCalendarAdminServicePreviewHolidayCalendarProcedure:     requires(adminperm.BizcalendarHolidayCalendarsPreview, auditReadOnly),
	bizcalendarv1alpha1connect.BizCalendarAdminServiceGetHolidayCalendarCoverageProcedure: requires(adminperm.BizcalendarHolidayCalendarCoveragesGet, auditReadOnly),

	buyerv1alpha1connect.BuyerAdminServiceGetBuyerStatsProcedure: requires(adminperm.BuyerBuyerStatsGet, auditReadOnly),

	catalogv1alpha1connect.CatalogAdminServiceListProductsProcedure: requires(adminperm.CatalogProductsList, auditReadOnly),
	catalogv1alpha1connect.CatalogAdminServiceGetProductProcedure:   requires(adminperm.CatalogProductsGet, auditReadOnly),

	contentv1alpha1connect.ContentAdminServiceListBannersProcedure:          requires(adminperm.ContentBannersList, auditReadOnly),
	contentv1alpha1connect.ContentAdminServiceGetBannerProcedure:            requires(adminperm.ContentBannersGet, auditReadOnly),
	contentv1alpha1connect.ContentAdminServiceCreateBannerProcedure:         requires(adminperm.ContentBannersCreate, auditPending),
	contentv1alpha1connect.ContentAdminServiceUpdateBannerProcedure:         requires(adminperm.ContentBannersUpdate, auditPending),
	contentv1alpha1connect.ContentAdminServiceDeleteBannerProcedure:         requires(adminperm.ContentBannersDelete, auditPending),
	contentv1alpha1connect.ContentAdminServiceListAppVersionGatesProcedure:  requires(adminperm.ContentAppVersionGatesList, auditReadOnly),
	contentv1alpha1connect.ContentAdminServiceCreateAppVersionGateProcedure: requires(adminperm.ContentAppVersionGatesCreate, auditPending),
	contentv1alpha1connect.ContentAdminServiceUpdateAppVersionGateProcedure: requires(adminperm.ContentAppVersionGatesUpdate, auditPending),

	couponv1alpha1connect.CouponAdminServiceCreateCampaignProcedure:    requires(adminperm.CouponCampaignsCreate, auditAudited),
	couponv1alpha1connect.CouponAdminServicePublishCampaignProcedure:   requires(adminperm.CouponCampaignsPublish, auditAudited),
	couponv1alpha1connect.CouponAdminServiceEndCampaignProcedure:       requires(adminperm.CouponCampaignsEnd, auditAudited),
	couponv1alpha1connect.CouponAdminServiceGrantVouchersProcedure:     requires(adminperm.CouponVouchersGrant, auditAudited),
	couponv1alpha1connect.CouponAdminServiceVoidVoucherProcedure:       requires(adminperm.CouponVouchersVoid, auditAudited),
	couponv1alpha1connect.CouponAdminServiceSetSellerQuotaProcedure:    requires(adminperm.CouponSellerQuotasUpdate, auditAudited),
	couponv1alpha1connect.CouponAdminServiceListCampaignsProcedure:     requires(adminperm.CouponCampaignsList, auditReadOnly),
	couponv1alpha1connect.CouponAdminServiceGetCampaignProcedure:       requires(adminperm.CouponCampaignsGet, auditReadOnly),
	couponv1alpha1connect.CouponAdminServiceListBuyerVouchersProcedure: requires(adminperm.CouponVouchersList, auditReadOnly),

	imageaiv1alpha1connect.ImageAiAdminServiceListAiStylesForAdminProcedure: requires(adminperm.ImageaiAiStylesList, auditReadOnly),
	imageaiv1alpha1connect.ImageAiAdminServiceCreateAiStyleProcedure:        requires(adminperm.ImageaiAiStylesCreate, auditAudited),
	imageaiv1alpha1connect.ImageAiAdminServiceUpdateAiStyleProcedure:        requires(adminperm.ImageaiAiStylesUpdate, auditAudited),
	imageaiv1alpha1connect.ImageAiAdminServiceSetDefaultAiStyleProcedure:    requires(adminperm.ImageaiAiStylesSetDefault, auditAudited),
	imageaiv1alpha1connect.ImageAiAdminServiceDeleteAiStyleProcedure:        requires(adminperm.ImageaiAiStylesDelete, auditAudited),
	imageaiv1alpha1connect.ImageAiAdminServiceGetAiQuotaProcedure:           requires(adminperm.ImageaiAiQuotasGet, auditReadOnly),
	imageaiv1alpha1connect.ImageAiAdminServiceUpdateAiQuotaProcedure:        requires(adminperm.ImageaiAiQuotasUpdate, auditAudited),
	imageaiv1alpha1connect.ImageAiAdminServiceDeleteAiQuotaProcedure:        requires(adminperm.ImageaiAiQuotasDelete, auditAudited),

	moderationv1alpha1connect.ModerationAdminServiceListCasesProcedure:   requires(adminperm.ModerationCasesList, auditReadOnly),
	moderationv1alpha1connect.ModerationAdminServiceGetCaseProcedure:     requires(adminperm.ModerationCasesGet, auditReadOnly),
	moderationv1alpha1connect.ModerationAdminServiceDecideCaseProcedure:  requires(adminperm.ModerationCasesDecide, auditAudited),
	moderationv1alpha1connect.ModerationAdminServiceReverseHideProcedure: requires(adminperm.ModerationCasesReverse, auditAudited),

	notificationv1alpha1connect.NotificationAdminServiceCreateAnnouncementProcedure:         requires(adminperm.NotificationAnnouncementsCreate, auditAudited),
	notificationv1alpha1connect.NotificationAdminServiceListAnnouncementsProcedure:          requires(adminperm.NotificationAnnouncementsList, auditReadOnly),
	notificationv1alpha1connect.NotificationAdminServiceResendDeliveriesProcedure:           requires(adminperm.NotificationDeliveriesResend, auditAudited),
	notificationv1alpha1connect.NotificationAdminServiceListUnreachedNotificationsProcedure: requires(adminperm.NotificationUnreachedNotificationsList, auditReadOnly),

	socialv1alpha1connect.SocialAdminServiceListPostsProcedure: requires(adminperm.SocialPostsList, auditReadOnly),

	paymentv1alpha1connect.PaymentAdminServiceGetPaymentAccountProcedure:           requires(adminperm.PaymentAccountsGet, auditReadOnly),
	paymentv1alpha1connect.PaymentAdminServiceRefreshPaymentAccountProcedure:       requires(adminperm.PaymentAccountsRefresh, auditReadOnly),
	paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountCreateProcedure:   requires(adminperm.PaymentAccountsRetryCreate, auditAudited),
	paymentv1alpha1connect.PaymentAdminServiceRetryPaymentAccountPurgeProcedure:    requires(adminperm.PaymentAccountsRetryPurge, auditAudited),
	paymentv1alpha1connect.PaymentAdminServiceDeletePaymentAccountMappingProcedure: requires(adminperm.PaymentAccountsDeleteMapping, auditAudited),
	paymentv1alpha1connect.PaymentAdminServiceListPaymentsProcedure:                requires(adminperm.PaymentPaymentsList, auditReadOnly),
	paymentv1alpha1connect.PaymentAdminServiceListPaymentTransfersProcedure:        requires(adminperm.PaymentTransfersList, auditReadOnly),
	paymentv1alpha1connect.PaymentAdminServiceListPaymentRefundsProcedure:          requires(adminperm.PaymentRefundsList, auditReadOnly),
	paymentv1alpha1connect.PaymentAdminServiceResolvePaymentAnomalyProcedure:       requires(adminperm.PaymentAnomaliesResolve, auditAudited),
	identityv1alpha1connect.IdentityAdminServiceGetSessionProcedure:                exempt(sessionReason, auditReadOnly),
	identityv1alpha1connect.IdentityAdminServiceListAccountsProcedure:              requires(adminperm.IdentityAccountsList, auditReadOnly),
	identityv1alpha1connect.IdentityAdminServiceGetAccountProcedure:                requires(adminperm.IdentityAccountsGet, auditReadOnly),
	identityv1alpha1connect.IdentityAdminServiceDeleteAccountProcedure:             requires(adminperm.IdentityAccountsDelete, auditPending),
	identityv1alpha1connect.IdentityAdminServiceSignOutProcedure:                   exempt(sessionReason, auditReadOnly),

	sellerv1alpha1connect.SellerAdminServiceListStoresProcedure:              requires(adminperm.SellerStoresList, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceGetStoreStatsProcedure:           requires(adminperm.SellerStoreStatsGet, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceGetStoreProcedure:                requires(adminperm.SellerStoresGet, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceGetStoreOnboardingProcedure:      requires(adminperm.SellerStoreOnboardingGet, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceListKYCApplicationsProcedure:     requires(adminperm.SellerKYCApplicationsList, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceGetKYCApplicationProcedure:       requires(adminperm.SellerKYCApplicationsGet, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceListKYCDocumentAccessesProcedure: requires(adminperm.SellerKYCDocumentAccessesList, auditReadOnly),
	sellerv1alpha1connect.SellerAdminServiceApproveKYCProcedure:              requires(adminperm.SellerKYCApplicationsApprove, auditPending),
	sellerv1alpha1connect.SellerAdminServiceRejectKYCProcedure:               requires(adminperm.SellerKYCApplicationsReject, auditPending),
	sellerv1alpha1connect.SellerAdminServiceRequeryKYCRegistrationProcedure:  requires(adminperm.SellerKYCApplicationsRequery, auditPending),

	sellerv1alpha1connect.SellerGovernanceAdminServiceListReportsProcedure:                 requires(adminperm.SellerReportsList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetReportProcedure:                   requires(adminperm.SellerReportsGet, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceMarkReportSupplementsSeenProcedure:   requires(adminperm.SellerReportsMarkSupplementsSeen, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceTriageReportProcedure:                requires(adminperm.SellerReportsTriage, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceResolveReportProcedure:               requires(adminperm.SellerReportsResolve, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceMoveReportProcedure:                  requires(adminperm.SellerReportsMove, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListCasesProcedure:                   requires(adminperm.SellerCasesList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetCaseProcedure:                     requires(adminperm.SellerCasesGet, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceDecideCaseProcedure:                  requires(adminperm.SellerCasesDecide, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceChangeCaseSeverityProcedure:          requires(adminperm.SellerCasesChangeSeverity, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateCaseInquiryProcedure:           requires(adminperm.SellerCaseInquiriesCreate, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListCaseInquiriesProcedure:           requires(adminperm.SellerCaseInquiriesList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRecordCaseInquiryReplyProcedure:      requires(adminperm.SellerCaseInquiriesRecordReply, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceWithdrawCaseInquiryProcedure:         requires(adminperm.SellerCaseInquiriesWithdraw, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateAuthorityOrderProcedure:        requires(adminperm.SellerAuthorityOrdersCreate, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListAuthorityOrdersProcedure:         requires(adminperm.SellerAuthorityOrdersList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetAuthorityOrderProcedure:           requires(adminperm.SellerAuthorityOrdersGet, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCompleteAuthorityOrderProcedure:      requires(adminperm.SellerAuthorityOrdersComplete, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceExtendAuthorityOrderHoldProcedure:    requires(adminperm.SellerAuthorityOrdersExtendHold, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRecordAuthorityTargetNoticeProcedure: requires(adminperm.SellerAuthorityOrdersRecordNotice, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRevokeAuthorityOrderProcedure:        requires(adminperm.SellerAuthorityOrdersRevoke, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateCaseNoteProcedure:              requires(adminperm.SellerCaseNotesCreate, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListCaseNotesProcedure:               requires(adminperm.SellerCaseNotesList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetViolationProcedure:                requires(adminperm.SellerViolationsGet, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListViolationsProcedure:              requires(adminperm.SellerViolationsList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureProcedure:               requires(adminperm.SellerMeasuresCreate, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceGetMeasureProcedure:                  requires(adminperm.SellerMeasuresGet, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasuresProcedure:                requires(adminperm.SellerMeasuresList, auditReadOnly),
	sellerv1alpha1connect.SellerGovernanceAdminServiceUpdateMeasureProcedure:               requires(adminperm.SellerMeasuresUpdate, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceEndMeasureProcedure:                  requires(adminperm.SellerMeasuresEnd, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceRevokeMeasureProcedure:               requires(adminperm.SellerMeasuresRevoke, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceScheduleMeasureReviewProcedure:       requires(adminperm.SellerMeasuresScheduleReview, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceMarkLinkedAccountReviewedProcedure:   requires(adminperm.SellerMeasuresReviewLinkedAccount, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceDecideAppealProcedure:                requires(adminperm.SellerMeasuresDecideAppeal, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceDecideRemediationProcedure:           requires(adminperm.SellerMeasuresDecideRemediation, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceCreateMeasureNoteProcedure:           requires(adminperm.SellerMeasureNotesCreate, auditPending),
	sellerv1alpha1connect.SellerGovernanceAdminServiceListMeasureNotesProcedure:            requires(adminperm.SellerMeasureNotesList, auditReadOnly),

	reviewqueuev1alpha1connect.ReviewQueueAdminServiceListReviewQueueProcedure: requires(adminperm.ReviewQueueItemsList, auditReadOnly),

	sellerv1alpha1connect.SellerProspectAdminServiceListProspectsProcedure:            requires(adminperm.SellerProspectsList, auditReadOnly),
	sellerv1alpha1connect.SellerProspectAdminServiceCreateProspectProcedure:           requires(adminperm.SellerProspectsCreate, auditPending),
	sellerv1alpha1connect.SellerProspectAdminServicePreviewProspectProcedure:          requires(adminperm.SellerProspectsPreview, auditReadOnly),
	sellerv1alpha1connect.SellerProspectAdminServiceGetProspectProcedure:              requires(adminperm.SellerProspectsGet, auditReadOnly),
	sellerv1alpha1connect.SellerProspectAdminServiceUpdateProspectProcedure:           requires(adminperm.SellerProspectsUpdate, auditPending),
	sellerv1alpha1connect.SellerProspectAdminServiceArchiveProspectProcedure:          requires(adminperm.SellerProspectsArchive, auditPending),
	sellerv1alpha1connect.SellerProspectAdminServiceRestoreProspectProcedure:          requires(adminperm.SellerProspectsRestore, auditPending),
	sellerv1alpha1connect.SellerProspectAdminServiceMergeProspectsProcedure:           requires(adminperm.SellerProspectsMerge, auditPending),
	sellerv1alpha1connect.SellerProspectAdminServiceConvertProspectProcedure:          requires(adminperm.SellerProspectsConvert, auditPending),
	sellerv1alpha1connect.SellerProspectAdminServiceListProspectDuplicatesProcedure:   requires(adminperm.SellerProspectDuplicatesList, auditReadOnly),
	sellerv1alpha1connect.SellerProspectAdminServiceListProspectTaxIDMatchesProcedure: requires(adminperm.SellerProspectTaxIDMatchesList, auditReadOnly),
	sellerv1alpha1connect.SellerProspectAdminServiceMarkProspectsDistinctProcedure:    requires(adminperm.SellerProspectDistinctPairsCreate, auditPending),
}

func checkDeclarations(table map[string]declaration, svcs []protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations("httpapi/adminauth", table, svcs, func(d declaration) string {
		switch {
		case (d.permission == "") == (d.exemptReason == ""):
			return "的宣告必須是一個權限或一個有理由的豁免"
		case d.permission != "" && !adminperm.InCatalog(d.permission):
			return "宣告了不在目錄裡的權限 " + string(d.permission)
		case d.audit != auditAudited && d.audit != auditReadOnly && d.audit != auditPending:
			return "沒有宣告後台動作紀錄的模式（audited、readOnly 或 auditPending，D62）"
		}
		return ""
	})
}

// Check 對照宣告表與實際掛載的 admin service（ADR 2026-10-04-admin-roles：「宣告＝掛載」）：每個掛載的方法都有宣告、
// 沒有指向未掛載方法的宣告、每個宣告合法。掛載的 service 由呼叫方從實際掛上的 handler 取得，不由這個套件列舉。
// rpc.DeferredServices 裡還沒掛的 service 不在比對之內。
func Check(mounted []protoreflect.ServiceDescriptor) error {
	return checkDeclarations(rpc.WithoutUnmountedDeferred(declarations, mounted), mounted)
}

// Procedures 是宣告表裡的全部 procedure，已排序。啟動時 Check 保證它與掛載的 admin 方法一致，測試以它逐一探測。
func Procedures() []string {
	return slices.Sorted(maps.Keys(declarations))
}

// AuditedProcedures 是宣告為 audited 的 procedure（已排序）：它們的 use case 必須在同一個 transaction 內寫後台紀錄，各模組的整合測試逐一驗證。
func AuditedProcedures() []string {
	return proceduresWith(auditAudited)
}

// AuditPendingProcedures 是宣告為 auditPending 的 procedure（已排序）：既有的寫入方法還沒遷移到後台紀錄（D62 延後項）。
func AuditPendingProcedures() []string {
	return proceduresWith(auditPending)
}

func proceduresWith(mode auditMode) []string {
	var out []string
	for p, d := range declarations {
		if d.audit == mode {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
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
