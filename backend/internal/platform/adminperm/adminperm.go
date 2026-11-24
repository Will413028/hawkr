// Package adminperm 是後台權限的目錄與角色對權限的對應（ADR 2026-10-04-admin-roles）。
//
// 權限的名稱是 <模組>.<資源>.<動詞>：模組是擁有資料的模組，資源是複數的 snake_case，標準方法的動詞是 get、list、
// create、update、delete，自訂方法用該動作的 snake_case 動詞。一支 admin 方法或一條 /api/files/admin 路由對應一個權限。
// 角色與權限的對應寫在這裡，資料庫只存「誰有哪個角色」。
// 跨模組的讀取組裝（沒有自己的資料，讀各模組發布的 pub_*）以組裝的名稱為前綴（`legacy`、`reviewqueue`）；它的權限只決定能不能呼叫，
// 看得到哪些內容由呼叫者對各來源的讀取權限決定。
package adminperm

import "slices"

// Permission 是後台的一個權限。
type Permission string

// Role 是管理員的角色。值等於 identity.admin_roles.role 的 CHECK。
type Role string

// RoleOwner 擁有目錄裡的全部權限。
const RoleOwner Role = "OWNER"

// Roles 回傳所有角色。
func Roles() []Role { return []Role{RoleOwner} }

const (
	BizcalendarHolidayCalendarsImport      Permission = "bizcalendar.holiday_calendars.import"
	BizcalendarHolidayCalendarsPreview     Permission = "bizcalendar.holiday_calendars.preview"
	BizcalendarHolidayCalendarCoveragesGet Permission = "bizcalendar.holiday_calendar_coverages.get"
	BuyerBuyerStatsGet                     Permission = "buyer.buyer_stats.get"
	ContentBannersList                     Permission = "content.banners.list"
	ContentBannersGet                      Permission = "content.banners.get"
	ContentBannersCreate                   Permission = "content.banners.create"
	ContentBannersUpdate                   Permission = "content.banners.update"
	ContentBannersDelete                   Permission = "content.banners.delete"
	ContentAppVersionGatesList             Permission = "content.app_version_gates.list"
	ContentAppVersionGatesCreate           Permission = "content.app_version_gates.create"
	ContentAppVersionGatesUpdate           Permission = "content.app_version_gates.update"
	DelegationSessionsCreate               Permission = "delegation.sessions.create"
	IdentityAccountsList                   Permission = "identity.accounts.list"
	IdentityAccountsGet                    Permission = "identity.accounts.get"
	IdentityAccountsDelete                 Permission = "identity.accounts.delete"
	ImageaiAiStylesList                    Permission = "imageai.ai_styles.list"
	ImageaiAiStylesCreate                  Permission = "imageai.ai_styles.create"
	ImageaiAiStylesUpdate                  Permission = "imageai.ai_styles.update"
	ImageaiAiStylesDelete                  Permission = "imageai.ai_styles.delete"
	ImageaiAiStylesSetDefault              Permission = "imageai.ai_styles.set_default"
	ImageaiAiQuotasGet                     Permission = "imageai.ai_quotas.get"
	ImageaiAiQuotasUpdate                  Permission = "imageai.ai_quotas.update"
	ImageaiAiQuotasDelete                  Permission = "imageai.ai_quotas.delete"
	LegacyGatewayForward                   Permission = "legacy.gateway.forward"
	ModerationCasesList                    Permission = "moderation.cases.list"
	ModerationCasesGet                     Permission = "moderation.cases.get"
	ModerationCasesDecide                  Permission = "moderation.cases.decide"
	ModerationCasesReverse                 Permission = "moderation.cases.reverse"
	NotificationAnnouncementsList          Permission = "notification.announcements.list"
	NotificationAnnouncementsCreate        Permission = "notification.announcements.create"
	PaymentAccountsGet                     Permission = "payment.accounts.get"
	PaymentAccountsRefresh                 Permission = "payment.accounts.refresh"
	PaymentAccountsRetryCreate             Permission = "payment.accounts.retry_create"
	PaymentAccountsRetryPurge              Permission = "payment.accounts.retry_purge"
	PaymentAccountsDeleteMapping           Permission = "payment.accounts.delete_mapping"
	PaymentPaymentsList                    Permission = "payment.payments.list"
	PaymentTransfersList                   Permission = "payment.transfers.list"
	PaymentRefundsList                     Permission = "payment.refunds.list"
	PaymentAnomaliesResolve                Permission = "payment.anomalies.resolve"
	ReviewQueueItemsList                   Permission = "reviewqueue.items.list"
	SellerStoresList                       Permission = "seller.stores.list"
	SellerStoresGet                        Permission = "seller.stores.get"
	SellerStoreStatsGet                    Permission = "seller.store_stats.get"
	SellerStoreOnboardingGet               Permission = "seller.store_onboarding.get"
	SellerKYCApplicationsList              Permission = "seller.kyc_applications.list"
	SellerKYCApplicationsGet               Permission = "seller.kyc_applications.get"
	SellerKYCApplicationsApprove           Permission = "seller.kyc_applications.approve"
	SellerKYCApplicationsReject            Permission = "seller.kyc_applications.reject"
	SellerKYCApplicationsRequery           Permission = "seller.kyc_applications.requery_registration"
	SellerKYCDocumentAccessesList          Permission = "seller.kyc_document_accesses.list"
	SellerKYCDocumentsGet                  Permission = "seller.kyc_documents.get"
	SellerReportsList                      Permission = "seller.reports.list"
	SellerReportsGet                       Permission = "seller.reports.get"
	SellerReportsMarkSupplementsSeen       Permission = "seller.reports.mark_supplements_seen"
	SellerReportsTriage                    Permission = "seller.reports.triage"
	SellerReportsResolve                   Permission = "seller.reports.resolve"
	SellerReportsMove                      Permission = "seller.reports.move"
	SellerCasesList                        Permission = "seller.cases.list"
	SellerCasesGet                         Permission = "seller.cases.get"
	SellerCasesDecide                      Permission = "seller.cases.decide"
	SellerCasesChangeSeverity              Permission = "seller.cases.change_severity"
	SellerCaseNotesCreate                  Permission = "seller.case_notes.create"
	SellerCaseNotesList                    Permission = "seller.case_notes.list"
	SellerCaseInquiriesCreate              Permission = "seller.case_inquiries.create"
	SellerCaseInquiriesList                Permission = "seller.case_inquiries.list"
	SellerCaseInquiriesRecordReply         Permission = "seller.case_inquiries.record_reply"
	SellerCaseInquiriesWithdraw            Permission = "seller.case_inquiries.withdraw"
	SellerCaseInquiryEvidenceGet           Permission = "seller.case_inquiry_evidence.get"
	SellerAuthorityOrdersCreate            Permission = "seller.authority_orders.create"
	SellerAuthorityOrdersList              Permission = "seller.authority_orders.list"
	SellerAuthorityOrdersGet               Permission = "seller.authority_orders.get"
	SellerAuthorityOrdersComplete          Permission = "seller.authority_orders.complete"
	SellerAuthorityOrdersExtendHold        Permission = "seller.authority_orders.extend_hold"
	SellerAuthorityOrdersRecordNotice      Permission = "seller.authority_orders.record_target_notice"
	SellerAuthorityOrdersRevoke            Permission = "seller.authority_orders.revoke"
	SellerReportEvidenceGet                Permission = "seller.report_evidence.get"
	SellerViolationsList                   Permission = "seller.violations.list"
	SellerViolationsGet                    Permission = "seller.violations.get"
	SellerMeasuresList                     Permission = "seller.measures.list"
	SellerMeasuresGet                      Permission = "seller.measures.get"
	SellerMeasuresCreate                   Permission = "seller.measures.create"
	SellerMeasuresUpdate                   Permission = "seller.measures.update"
	SellerMeasuresEnd                      Permission = "seller.measures.end"
	SellerMeasuresRevoke                   Permission = "seller.measures.revoke"
	SellerMeasuresScheduleReview           Permission = "seller.measures.schedule_review"
	SellerMeasuresReviewLinkedAccount      Permission = "seller.measures.review_linked_account"
	SellerMeasuresDecideAppeal             Permission = "seller.measures.decide_appeal"
	SellerMeasuresDecideRemediation        Permission = "seller.measures.decide_remediation"
	SellerMeasureNotesCreate               Permission = "seller.measure_notes.create"
	SellerMeasureNotesList                 Permission = "seller.measure_notes.list"
	SellerMeasureRequestEvidenceGet        Permission = "seller.measure_request_evidence.get"
	SellerProspectsList                    Permission = "seller.prospects.list"
	SellerProspectsGet                     Permission = "seller.prospects.get"
	SellerProspectsCreate                  Permission = "seller.prospects.create"
	SellerProspectsPreview                 Permission = "seller.prospects.preview"
	SellerProspectsUpdate                  Permission = "seller.prospects.update"
	SellerProspectsArchive                 Permission = "seller.prospects.archive"
	SellerProspectsRestore                 Permission = "seller.prospects.restore"
	SellerProspectsMerge                   Permission = "seller.prospects.merge"
	SellerProspectsConvert                 Permission = "seller.prospects.convert"
	SellerProspectDuplicatesList           Permission = "seller.prospect_duplicates.list"
	SellerProspectTaxIDMatchesList         Permission = "seller.prospect_tax_id_matches.list"
	SellerProspectDistinctPairsCreate      Permission = "seller.prospect_distinct_pairs.create"
	SocialPostsList                        Permission = "social.posts.list"
)

// Catalog 回傳目錄裡的全部權限。
func Catalog() []Permission {
	return []Permission{
		BizcalendarHolidayCalendarsImport, BizcalendarHolidayCalendarsPreview, BizcalendarHolidayCalendarCoveragesGet,
		BuyerBuyerStatsGet,
		ContentBannersList, ContentBannersGet, ContentBannersCreate, ContentBannersUpdate, ContentBannersDelete,
		ContentAppVersionGatesList, ContentAppVersionGatesCreate, ContentAppVersionGatesUpdate,
		DelegationSessionsCreate,
		IdentityAccountsList, IdentityAccountsGet, IdentityAccountsDelete,
		ImageaiAiStylesList, ImageaiAiStylesCreate, ImageaiAiStylesUpdate, ImageaiAiStylesDelete, ImageaiAiStylesSetDefault,
		ImageaiAiQuotasGet, ImageaiAiQuotasUpdate, ImageaiAiQuotasDelete,
		LegacyGatewayForward,
		ModerationCasesList, ModerationCasesGet, ModerationCasesDecide, ModerationCasesReverse,
		NotificationAnnouncementsList, NotificationAnnouncementsCreate,
		PaymentAccountsGet, PaymentAccountsRefresh, PaymentAccountsRetryCreate, PaymentAccountsRetryPurge, PaymentAccountsDeleteMapping,
		PaymentPaymentsList, PaymentTransfersList, PaymentRefundsList, PaymentAnomaliesResolve,
		ReviewQueueItemsList,
		SellerStoresList, SellerStoresGet,
		SellerStoreStatsGet, SellerStoreOnboardingGet,
		SellerKYCApplicationsList, SellerKYCApplicationsGet, SellerKYCApplicationsApprove, SellerKYCApplicationsReject,
		SellerKYCApplicationsRequery,
		SellerKYCDocumentAccessesList, SellerKYCDocumentsGet,
		SellerReportsList, SellerReportsGet, SellerReportsMarkSupplementsSeen, SellerReportsTriage, SellerReportsResolve, SellerReportsMove,
		SellerCasesList, SellerCasesGet, SellerCasesDecide, SellerCasesChangeSeverity,
		SellerCaseNotesCreate, SellerCaseNotesList, SellerReportEvidenceGet,
		SellerCaseInquiriesCreate, SellerCaseInquiriesList, SellerCaseInquiriesRecordReply, SellerCaseInquiriesWithdraw, SellerCaseInquiryEvidenceGet,
		SellerAuthorityOrdersCreate, SellerAuthorityOrdersList, SellerAuthorityOrdersGet, SellerAuthorityOrdersComplete, SellerAuthorityOrdersExtendHold,
		SellerAuthorityOrdersRecordNotice, SellerAuthorityOrdersRevoke,
		SellerViolationsList, SellerViolationsGet,
		SellerMeasuresList, SellerMeasuresGet, SellerMeasuresCreate, SellerMeasuresUpdate, SellerMeasuresEnd, SellerMeasuresRevoke,
		SellerMeasuresScheduleReview, SellerMeasuresReviewLinkedAccount, SellerMeasuresDecideAppeal, SellerMeasuresDecideRemediation, SellerMeasureNotesCreate, SellerMeasureNotesList,
		SellerMeasureRequestEvidenceGet,
		SellerProspectsList, SellerProspectsGet, SellerProspectsCreate, SellerProspectsPreview, SellerProspectsUpdate,
		SellerProspectsArchive, SellerProspectsRestore, SellerProspectsMerge, SellerProspectsConvert,
		SellerProspectDuplicatesList, SellerProspectTaxIDMatchesList, SellerProspectDistinctPairsCreate,
		SocialPostsList,
	}
}

// InCatalog 回傳 p 是不是目錄裡的權限。
func InCatalog(p Permission) bool { return slices.Contains(Catalog(), p) }

// Allows 回傳持有 roles 的管理員是否有權限 p。不在目錄裡的權限一律不允許；沒有角色的管理員什麼都不允許。
func Allows(roles []Role, p Permission) bool {
	if !InCatalog(p) {
		return false
	}
	for _, r := range roles {
		if r == RoleOwner {
			return true
		}
	}
	return false
}
