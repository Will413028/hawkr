// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。
import { Code } from "@connectrpc/connect";
import type { Gender } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import type { StoreCardStatus } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import type { PaymentAccountStatus, RequirementCollection } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import type { ContentKind, KYCField, KYCStatus, MeasureBasis, MeasureKind, MeasureRequestKind, MeasureRequestState, MeasureState, OnboardingAction, OnboardingTaskId, RevokeReason, RiskGround, SellerRequirement, StoreField, TerminationGround, ViolationSeverity } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { UiMessage } from "./message";

export type { DefaultErrorText, UiMessage } from "./message";

export type UiTextKey =
	| "buyer.Gender.GENDER_FEMALE"
	| "buyer.Gender.GENDER_MALE"
	| "buyer.Gender.GENDER_OTHER"
	| "buyer.Profile.gender_unset"
	| "common.PolicyCategory.POLICY_CATEGORY_COUNTERFEIT"
	| "common.PolicyCategory.POLICY_CATEGORY_FAKE_CERTIFICATION"
	| "common.PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY"
	| "common.PolicyCategory.POLICY_CATEGORY_FRAUD"
	| "common.PolicyCategory.POLICY_CATEGORY_HARASSMENT"
	| "common.PolicyCategory.POLICY_CATEGORY_HATE"
	| "common.PolicyCategory.POLICY_CATEGORY_IMPERSONATION"
	| "common.PolicyCategory.POLICY_CATEGORY_IP_INFRINGEMENT"
	| "common.PolicyCategory.POLICY_CATEGORY_MISLEADING_INFO"
	| "common.PolicyCategory.POLICY_CATEGORY_OFF_PLATFORM_TRADE"
	| "common.PolicyCategory.POLICY_CATEGORY_OTHER"
	| "common.PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE"
	| "common.PolicyCategory.POLICY_CATEGORY_PRIVACY"
	| "common.PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS"
	| "common.PolicyCategory.POLICY_CATEGORY_SPAM"
	| "common.PolicyCategory.POLICY_CATEGORY_UNLICENSED_SALE"
	| "common.StoreCard.closed_store"
	| "common.StoreCardStatus.STORE_CARD_STATUS_CLOSED"
	| "common.StoreCardStatus.STORE_CARD_STATUS_OPEN"
	| "common.StoreCardStatus.STORE_CARD_STATUS_PAUSED"
	| "common.error.ABORTED"
	| "common.error.ALREADY_EXISTS"
	| "common.error.DATA_LOSS"
	| "common.error.DEADLINE_EXCEEDED"
	| "common.error.FAILED_PRECONDITION"
	| "common.error.INTERNAL"
	| "common.error.INVALID_ARGUMENT"
	| "common.error.NOT_FOUND"
	| "common.error.OUT_OF_RANGE"
	| "common.error.PERMISSION_DENIED"
	| "common.error.RESOURCE_EXHAUSTED"
	| "common.error.UNAUTHENTICATED"
	| "common.error.UNAVAILABLE"
	| "common.error.UNIMPLEMENTED"
	| "common.error.UNKNOWN"
	| "common.unknown"
	| "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED"
	| "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED"
	| "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED"
	| "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_NOT_CREATED"
	| "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW"
	| "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED"
	| "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_DISABLED"
	| "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_NOT_CREATED"
	| "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW"
	| "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED"
	| "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_DISABLED"
	| "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_ENABLED"
	| "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_NOT_CREATED"
	| "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW"
	| "payment.RequirementCollection.REQUIREMENT_COLLECTION_PLATFORM"
	| "payment.RequirementCollection.REQUIREMENT_COLLECTION_PROVIDER"
	| "seller.ContentKind.CONTENT_KIND_COMMENT"
	| "seller.ContentKind.CONTENT_KIND_FORUM_ENTRY"
	| "seller.ContentKind.CONTENT_KIND_POST"
	| "seller.KYCApplication.decide_time"
	| "seller.KYCApplication.submit_time"
	| "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS"
	| "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS"
	| "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME"
	| "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE"
	| "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE"
	| "seller.KYCField.KYC_FIELD_BUSINESS_TAX_ID"
	| "seller.KYCField.KYC_FIELD_PRODUCT_TYPE"
	| "seller.KYCField.KYC_FIELD_WEBSITE_URL"
	| "seller.KYCStatus.KYC_STATUS_APPROVED"
	| "seller.KYCStatus.KYC_STATUS_CANCELLED"
	| "seller.KYCStatus.KYC_STATUS_DRAFT"
	| "seller.KYCStatus.KYC_STATUS_REJECTED"
	| "seller.KYCStatus.KYC_STATUS_SUBMITTED"
	| "seller.Measure.appeal_deadline"
	| "seller.Measure.meta_kind"
	| "seller.Measure.meta_severity"
	| "seller.Measure.meta_termination_ground"
	| "seller.Measure.rejection_cooldown_warning"
	| "seller.Measure.suspension_active_open"
	| "seller.Measure.suspension_active_until"
	| "seller.Measure.suspension_ended"
	| "seller.Measure.termination_effective"
	| "seller.Measure.termination_effective_ended"
	| "seller.Measure.termination_notice"
	| "seller.Measure.title_fallback"
	| "seller.Measure.title_risk"
	| "seller.Measure.title_risk_fallback"
	| "seller.MeasureBasis.MEASURE_BASIS_CONTRACT"
	| "seller.MeasureBasis.MEASURE_BASIS_RISK"
	| "seller.MeasureBasis.MEASURE_BASIS_VIOLATION"
	| "seller.MeasureKind.MEASURE_KIND_CONTENT_TAKEDOWN"
	| "seller.MeasureKind.MEASURE_KIND_SUSPENSION"
	| "seller.MeasureKind.MEASURE_KIND_TERMINATION"
	| "seller.MeasureKind.MEASURE_KIND_WARNING"
	| "seller.MeasureRequest.reply_note"
	| "seller.MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL"
	| "seller.MeasureRequestKind.MEASURE_REQUEST_KIND_REMEDIATION"
	| "seller.MeasureRequestKind.submitted_note.MEASURE_REQUEST_KIND_APPEAL"
	| "seller.MeasureRequestKind.submitted_note.MEASURE_REQUEST_KIND_REMEDIATION"
	| "seller.MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED"
	| "seller.MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED"
	| "seller.MeasureRequestState.MEASURE_REQUEST_STATE_PENDING"
	| "seller.MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED"
	| "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_ACCEPTED"
	| "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_CLOSED"
	| "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_PENDING"
	| "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_REJECTED"
	| "seller.MeasureState.MEASURE_STATE_ACTIVE"
	| "seller.MeasureState.MEASURE_STATE_ENDED"
	| "seller.MeasureState.MEASURE_STATE_ISSUED"
	| "seller.MeasureState.MEASURE_STATE_REVOKED"
	| "seller.MeasureState.MEASURE_STATE_SCHEDULED"
	| "seller.OnboardingAction.button.ONBOARDING_ACTION_CONTACT_SUPPORT"
	| "seller.OnboardingAction.button.ONBOARDING_ACTION_EDIT_PROFILE"
	| "seller.OnboardingAction.button.ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT"
	| "seller.OnboardingAction.button.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING"
	| "seller.OnboardingAction.button.ONBOARDING_ACTION_SUBMIT_CERTIFICATION"
	| "seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT"
	| "seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PLATFORM_REVIEW"
	| "seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PROFILE"
	| "seller.OnboardingTaskId.panel_description.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT"
	| "seller.OnboardingTaskId.panel_description.ONBOARDING_TASK_ID_PLATFORM_REVIEW"
	| "seller.OnboardingTaskId.panel_description.ONBOARDING_TASK_ID_PROFILE"
	| "seller.OnboardingTaskId.panel_title.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT"
	| "seller.OnboardingTaskId.panel_title.ONBOARDING_TASK_ID_PLATFORM_REVIEW"
	| "seller.OnboardingTaskId.panel_title.ONBOARDING_TASK_ID_PROFILE"
	| "seller.PaymentAccount.payouts_inactive_note"
	| "seller.RevokeReason.REVOKE_REASON_ADMIN_CORRECTION"
	| "seller.RevokeReason.REVOKE_REASON_APPEAL_ACCEPTED"
	| "seller.RevokeReason.REVOKE_REASON_EXTERNAL_DECISION"
	| "seller.RevokeReason.REVOKE_REASON_FINDING_REVOKED"
	| "seller.RevokeReason.REVOKE_REASON_OTHER"
	| "seller.RevokeReason.REVOKE_REASON_SOURCE_REVOKED"
	| "seller.RiskGround.RISK_GROUND_EXTERNAL_ORDER"
	| "seller.RiskGround.RISK_GROUND_FOOD_SAFETY"
	| "seller.RiskGround.RISK_GROUND_FRAUD"
	| "seller.RiskGround.RISK_GROUND_FULFILLMENT"
	| "seller.RiskGround.RISK_GROUND_INFRINGEMENT"
	| "seller.RiskGround.RISK_GROUND_PERSONAL_DATA"
	| "seller.RiskGround.RISK_GROUND_SECURITY"
	| "seller.RiskGround.RISK_GROUND_TRANSACTION"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_LOGO"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_STORE_NAME"
	| "seller.SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE"
	| "seller.StoreField.STORE_FIELD_ADDRESS"
	| "seller.StoreField.STORE_FIELD_CONTACT_MOBILE"
	| "seller.StoreField.STORE_FIELD_CONTACT_NAME"
	| "seller.StoreField.STORE_FIELD_CONTACT_PHONE"
	| "seller.StoreField.STORE_FIELD_SUPPORT_EMAIL"
	| "seller.TerminationGround.TERMINATION_GROUND_BANKRUPTCY"
	| "seller.TerminationGround.TERMINATION_GROUND_CEASED_BUSINESS"
	| "seller.TerminationGround.TERMINATION_GROUND_DISSOLUTION"
	| "seller.TerminationGround.TERMINATION_GROUND_FOOD_SAFETY_INCIDENT"
	| "seller.TerminationGround.TERMINATION_GROUND_FRAUD"
	| "seller.TerminationGround.TERMINATION_GROUND_LICENSE_REVOKED"
	| "seller.TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT"
	| "seller.TerminationGround.TERMINATION_GROUND_MATERIAL_BREACH"
	| "seller.TerminationGround.TERMINATION_GROUND_NOTICE"
	| "seller.TerminationGround.TERMINATION_GROUND_OTHER_SERIOUS"
	| "seller.TerminationGround.TERMINATION_GROUND_PLATFORM_SECURITY"
	| "seller.ViolationSeverity.VIOLATION_SEVERITY_CRITICAL"
	| "seller.ViolationSeverity.VIOLATION_SEVERITY_HIGH"
	| "seller.ViolationSeverity.VIOLATION_SEVERITY_LOW"
	| "seller.ViolationSeverity.VIOLATION_SEVERITY_MEDIUM"
	;

/** seller.MeasureKind 的值號到值名；ICU select 的分支用值名。 */
export const SELLER_MEASURE_KIND_NAME: Readonly<Record<number, string>> = {
	1: "MEASURE_KIND_WARNING",
	2: "MEASURE_KIND_SUSPENSION",
	3: "MEASURE_KIND_TERMINATION",
	4: "MEASURE_KIND_CONTENT_TAKEDOWN",
};

/** seller.MeasureRequestKind 的值號到值名；ICU select 的分支用值名。 */
export const SELLER_MEASURE_REQUEST_KIND_NAME: Readonly<Record<number, string>> = {
	1: "MEASURE_REQUEST_KIND_APPEAL",
	2: "MEASURE_REQUEST_KIND_REMEDIATION",
};

const BUYER_GENDER: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "buyer.Gender.GENDER_MALE",
	2: "buyer.Gender.GENDER_FEMALE",
	3: "buyer.Gender.GENDER_OTHER",
};

/** buyer.Gender 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function buyerGender(v: Gender): UiMessage | undefined {
	const key = BUYER_GENDER[v];
	if (key === undefined) return undefined;
	return { key };
}

/** buyer.Profile.gender_unset */
export function buyerProfileGenderUnset(): UiMessage {
	return { key: "buyer.Profile.gender_unset" };
}

const COMMON_POLICY_CATEGORY: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "common.PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY",
	2: "common.PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS",
	3: "common.PolicyCategory.POLICY_CATEGORY_UNLICENSED_SALE",
	4: "common.PolicyCategory.POLICY_CATEGORY_MISLEADING_INFO",
	5: "common.PolicyCategory.POLICY_CATEGORY_FAKE_CERTIFICATION",
	6: "common.PolicyCategory.POLICY_CATEGORY_COUNTERFEIT",
	7: "common.PolicyCategory.POLICY_CATEGORY_IP_INFRINGEMENT",
	8: "common.PolicyCategory.POLICY_CATEGORY_FRAUD",
	9: "common.PolicyCategory.POLICY_CATEGORY_OFF_PLATFORM_TRADE",
	10: "common.PolicyCategory.POLICY_CATEGORY_PRIVACY",
	11: "common.PolicyCategory.POLICY_CATEGORY_IMPERSONATION",
	12: "common.PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE",
	13: "common.PolicyCategory.POLICY_CATEGORY_OTHER",
	14: "common.PolicyCategory.POLICY_CATEGORY_SPAM",
	15: "common.PolicyCategory.POLICY_CATEGORY_HARASSMENT",
	16: "common.PolicyCategory.POLICY_CATEGORY_HATE",
};

/** common.PolicyCategory 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function commonPolicyCategory(v: PolicyCategory): UiMessage | undefined {
	const key = COMMON_POLICY_CATEGORY[v];
	if (key === undefined) return undefined;
	return { key };
}

/** common.StoreCard.closed_store */
export function commonStoreCardClosedStore(): UiMessage {
	return { key: "common.StoreCard.closed_store" };
}

const COMMON_STORE_CARD_STATUS: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "common.StoreCardStatus.STORE_CARD_STATUS_OPEN",
	2: "common.StoreCardStatus.STORE_CARD_STATUS_PAUSED",
	3: "common.StoreCardStatus.STORE_CARD_STATUS_CLOSED",
};

/** common.StoreCardStatus 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function commonStoreCardStatus(v: StoreCardStatus): UiMessage | undefined {
	const key = COMMON_STORE_CARD_STATUS[v];
	if (key === undefined) return undefined;
	return { key };
}

const COMMON_ERROR: Readonly<Partial<Record<Code, UiTextKey>>> = {
	[Code.Aborted]: "common.error.ABORTED",
	[Code.AlreadyExists]: "common.error.ALREADY_EXISTS",
	[Code.DataLoss]: "common.error.DATA_LOSS",
	[Code.DeadlineExceeded]: "common.error.DEADLINE_EXCEEDED",
	[Code.FailedPrecondition]: "common.error.FAILED_PRECONDITION",
	[Code.Internal]: "common.error.INTERNAL",
	[Code.InvalidArgument]: "common.error.INVALID_ARGUMENT",
	[Code.NotFound]: "common.error.NOT_FOUND",
	[Code.OutOfRange]: "common.error.OUT_OF_RANGE",
	[Code.PermissionDenied]: "common.error.PERMISSION_DENIED",
	[Code.ResourceExhausted]: "common.error.RESOURCE_EXHAUSTED",
	[Code.Unauthenticated]: "common.error.UNAUTHENTICATED",
	[Code.Unavailable]: "common.error.UNAVAILABLE",
	[Code.Unimplemented]: "common.error.UNIMPLEMENTED",
	[Code.Unknown]: "common.error.UNKNOWN",
};

/** Connect code 的預設文字；Canceled 不顯示，回 undefined。 */
export function commonError(code: Code): UiMessage | undefined {
	const key = COMMON_ERROR[code];
	return key === undefined ? undefined : { key };
}

/** 不認得的值與 UNSPECIFIED 一律顯示的文字。 */
export function commonUnknown(): UiMessage {
	return { key: "common.unknown" };
}

const PAYMENT_PAYMENT_ACCOUNT_STATUS: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_NOT_CREATED",
	2: "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW",
	3: "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED",
	4: "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED",
	5: "payment.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED",
};

/** payment.PaymentAccountStatus 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function paymentPaymentAccountStatus(v: PaymentAccountStatus): UiMessage | undefined {
	const key = PAYMENT_PAYMENT_ACCOUNT_STATUS[v];
	if (key === undefined) return undefined;
	return { key };
}

const PAYMENT_PAYMENT_ACCOUNT_STATUS_ACTION: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_NOT_CREATED",
	2: "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW",
	3: "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED",
	5: "payment.PaymentAccountStatus.action.PAYMENT_ACCOUNT_STATUS_DISABLED",
};

/** payment.PaymentAccountStatus.action；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function paymentPaymentAccountStatusAction(v: PaymentAccountStatus): UiMessage | undefined {
	const key = PAYMENT_PAYMENT_ACCOUNT_STATUS_ACTION[v];
	if (key === undefined) return undefined;
	return { key };
}

const PAYMENT_PAYMENT_ACCOUNT_STATUS_DESCRIPTION: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_NOT_CREATED",
	2: "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW",
	3: "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED",
	4: "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_ENABLED",
	5: "payment.PaymentAccountStatus.description.PAYMENT_ACCOUNT_STATUS_DISABLED",
};

/** payment.PaymentAccountStatus.description；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function paymentPaymentAccountStatusDescription(v: PaymentAccountStatus): UiMessage | undefined {
	const key = PAYMENT_PAYMENT_ACCOUNT_STATUS_DESCRIPTION[v];
	if (key === undefined) return undefined;
	return { key };
}

const PAYMENT_REQUIREMENT_COLLECTION: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "payment.RequirementCollection.REQUIREMENT_COLLECTION_PROVIDER",
	2: "payment.RequirementCollection.REQUIREMENT_COLLECTION_PLATFORM",
};

/** payment.RequirementCollection 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function paymentRequirementCollection(v: RequirementCollection): UiMessage | undefined {
	const key = PAYMENT_REQUIREMENT_COLLECTION[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_CONTENT_KIND: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.ContentKind.CONTENT_KIND_POST",
	2: "seller.ContentKind.CONTENT_KIND_COMMENT",
	3: "seller.ContentKind.CONTENT_KIND_FORUM_ENTRY",
};

/** seller.ContentKind 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerContentKind(v: ContentKind): UiMessage | undefined {
	const key = SELLER_CONTENT_KIND[v];
	if (key === undefined) return undefined;
	return { key };
}

/** seller.KYCApplication.decide_time */
export function sellerKYCApplicationDecideTime(a: { time: string }): UiMessage {
	return { key: "seller.KYCApplication.decide_time", values: { time: a.time } };
}

/** seller.KYCApplication.submit_time */
export function sellerKYCApplicationSubmitTime(a: { time: string }): UiMessage {
	return { key: "seller.KYCApplication.submit_time", values: { time: a.time } };
}

const SELLER_KYCFIELD: Readonly<Partial<Record<number, UiTextKey>>> = {
	10: "seller.KYCField.KYC_FIELD_PRODUCT_TYPE",
	12: "seller.KYCField.KYC_FIELD_WEBSITE_URL",
	20: "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME",
	21: "seller.KYCField.KYC_FIELD_BUSINESS_TAX_ID",
	22: "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE",
	25: "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE",
	27: "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS",
	28: "seller.KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS",
};

/** seller.KYCField 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerKYCField(v: KYCField): UiMessage | undefined {
	const key = SELLER_KYCFIELD[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_KYCSTATUS: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.KYCStatus.KYC_STATUS_DRAFT",
	2: "seller.KYCStatus.KYC_STATUS_SUBMITTED",
	3: "seller.KYCStatus.KYC_STATUS_APPROVED",
	4: "seller.KYCStatus.KYC_STATUS_REJECTED",
	5: "seller.KYCStatus.KYC_STATUS_CANCELLED",
};

/** seller.KYCStatus 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerKYCStatus(v: KYCStatus): UiMessage | undefined {
	const key = SELLER_KYCSTATUS[v];
	if (key === undefined) return undefined;
	return { key };
}

/** seller.Measure.appeal_deadline */
export function sellerMeasureAppealDeadline(a: { deadline: string }): UiMessage {
	return { key: "seller.Measure.appeal_deadline", values: { deadline: a.deadline } };
}

/** seller.Measure.meta_kind */
export function sellerMeasureMetaKind(a: { kind: MeasureKind }): UiMessage | undefined {
	const kindKey = SELLER_MEASURE_KIND[a.kind];
	if (kindKey === undefined) return undefined;
	return { key: "seller.Measure.meta_kind", values: { kind: { key: kindKey } } };
}

/** seller.Measure.meta_severity */
export function sellerMeasureMetaSeverity(a: { severity: ViolationSeverity }): UiMessage | undefined {
	const severityKey = SELLER_VIOLATION_SEVERITY[a.severity];
	if (severityKey === undefined) return undefined;
	return { key: "seller.Measure.meta_severity", values: { severity: { key: severityKey } } };
}

/** seller.Measure.meta_termination_ground */
export function sellerMeasureMetaTerminationGround(a: { ground: TerminationGround }): UiMessage | undefined {
	const groundKey = SELLER_TERMINATION_GROUND[a.ground];
	if (groundKey === undefined) return undefined;
	return { key: "seller.Measure.meta_termination_ground", values: { ground: { key: groundKey } } };
}

/** seller.Measure.rejection_cooldown_warning */
export function sellerMeasureRejectionCooldownWarning(a: { days: number }): UiMessage {
	return { key: "seller.Measure.rejection_cooldown_warning", values: { days: a.days } };
}

/** seller.Measure.suspension_active_open */
export function sellerMeasureSuspensionActiveOpen(): UiMessage {
	return { key: "seller.Measure.suspension_active_open" };
}

/** seller.Measure.suspension_active_until */
export function sellerMeasureSuspensionActiveUntil(a: { end: string }): UiMessage {
	return { key: "seller.Measure.suspension_active_until", values: { end: a.end } };
}

/** seller.Measure.suspension_ended */
export function sellerMeasureSuspensionEnded(a: { end: string }): UiMessage {
	return { key: "seller.Measure.suspension_ended", values: { end: a.end } };
}

/** seller.Measure.termination_effective */
export function sellerMeasureTerminationEffective(a: { start: string }): UiMessage {
	return { key: "seller.Measure.termination_effective", values: { start: a.start } };
}

/** seller.Measure.termination_effective_ended */
export function sellerMeasureTerminationEffectiveEnded(a: { start: string; end: string }): UiMessage {
	return { key: "seller.Measure.termination_effective_ended", values: { start: a.start, end: a.end } };
}

/** seller.Measure.termination_notice */
export function sellerMeasureTerminationNotice(a: { start: string }): UiMessage {
	return { key: "seller.Measure.termination_notice", values: { start: a.start } };
}

/** seller.Measure.title_fallback */
export function sellerMeasureTitleFallback(): UiMessage {
	return { key: "seller.Measure.title_fallback" };
}

/** seller.Measure.title_risk */
export function sellerMeasureTitleRisk(a: { ground: RiskGround }): UiMessage | undefined {
	const groundKey = SELLER_RISK_GROUND[a.ground];
	if (groundKey === undefined) return undefined;
	return { key: "seller.Measure.title_risk", values: { ground: { key: groundKey } } };
}

/** seller.Measure.title_risk_fallback */
export function sellerMeasureTitleRiskFallback(): UiMessage {
	return { key: "seller.Measure.title_risk_fallback" };
}

const SELLER_MEASURE_BASIS: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureBasis.MEASURE_BASIS_VIOLATION",
	2: "seller.MeasureBasis.MEASURE_BASIS_RISK",
	3: "seller.MeasureBasis.MEASURE_BASIS_CONTRACT",
};

/** seller.MeasureBasis 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerMeasureBasis(v: MeasureBasis): UiMessage | undefined {
	const key = SELLER_MEASURE_BASIS[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_MEASURE_KIND: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureKind.MEASURE_KIND_WARNING",
	2: "seller.MeasureKind.MEASURE_KIND_SUSPENSION",
	3: "seller.MeasureKind.MEASURE_KIND_TERMINATION",
	4: "seller.MeasureKind.MEASURE_KIND_CONTENT_TAKEDOWN",
};

/** seller.MeasureKind 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerMeasureKind(v: MeasureKind): UiMessage | undefined {
	const key = SELLER_MEASURE_KIND[v];
	if (key === undefined) return undefined;
	return { key };
}

/** seller.MeasureRequest.reply_note */
export function sellerMeasureRequestReplyNote(a: { reason: string }): UiMessage {
	return { key: "seller.MeasureRequest.reply_note", values: { reason: a.reason } };
}

const SELLER_MEASURE_REQUEST_KIND: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL",
	2: "seller.MeasureRequestKind.MEASURE_REQUEST_KIND_REMEDIATION",
};

/** seller.MeasureRequestKind 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerMeasureRequestKind(v: MeasureRequestKind): UiMessage | undefined {
	const key = SELLER_MEASURE_REQUEST_KIND[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_MEASURE_REQUEST_KIND_SUBMITTED_NOTE: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureRequestKind.submitted_note.MEASURE_REQUEST_KIND_APPEAL",
	2: "seller.MeasureRequestKind.submitted_note.MEASURE_REQUEST_KIND_REMEDIATION",
};

/** seller.MeasureRequestKind.submitted_note；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerMeasureRequestKindSubmittedNote(v: MeasureRequestKind): UiMessage | undefined {
	const key = SELLER_MEASURE_REQUEST_KIND_SUBMITTED_NOTE[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_MEASURE_REQUEST_STATE: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureRequestState.MEASURE_REQUEST_STATE_PENDING",
	2: "seller.MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED",
	3: "seller.MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED",
	4: "seller.MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED",
};

/** seller.MeasureRequestState 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerMeasureRequestState(v: MeasureRequestState): UiMessage | undefined {
	const key = SELLER_MEASURE_REQUEST_STATE[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_MEASURE_REQUEST_STATE_WITH_KIND: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_PENDING",
	2: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_ACCEPTED",
	3: "seller.MeasureRequestState.with_kind.MEASURE_REQUEST_STATE_REJECTED",
};

/** seller.MeasureRequestState.with_kind；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerMeasureRequestStateWithKind(v: MeasureRequestState, a: { kind: MeasureRequestKind }): UiMessage | undefined {
	const key = SELLER_MEASURE_REQUEST_STATE_WITH_KIND[v];
	if (key === undefined) return undefined;
	return { key, values: { kind: SELLER_MEASURE_REQUEST_KIND_NAME[a.kind] ?? "" } };
}

const SELLER_MEASURE_STATE: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.MeasureState.MEASURE_STATE_ISSUED",
	2: "seller.MeasureState.MEASURE_STATE_ACTIVE",
	3: "seller.MeasureState.MEASURE_STATE_ENDED",
	4: "seller.MeasureState.MEASURE_STATE_REVOKED",
	5: "seller.MeasureState.MEASURE_STATE_SCHEDULED",
};

/** seller.MeasureState 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerMeasureState(v: MeasureState, a: { kind: MeasureKind }): UiMessage | undefined {
	const key = SELLER_MEASURE_STATE[v];
	if (key === undefined) return undefined;
	return { key, values: { kind: SELLER_MEASURE_KIND_NAME[a.kind] ?? "" } };
}

const SELLER_ONBOARDING_ACTION_BUTTON: Readonly<Partial<Record<number, UiTextKey>>> = {
	2: "seller.OnboardingAction.button.ONBOARDING_ACTION_EDIT_PROFILE",
	3: "seller.OnboardingAction.button.ONBOARDING_ACTION_SUBMIT_CERTIFICATION",
	4: "seller.OnboardingAction.button.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING",
	5: "seller.OnboardingAction.button.ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT",
	6: "seller.OnboardingAction.button.ONBOARDING_ACTION_CONTACT_SUPPORT",
};

/** seller.OnboardingAction.button；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerOnboardingActionButton(v: OnboardingAction): UiMessage | undefined {
	const key = SELLER_ONBOARDING_ACTION_BUTTON[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_ONBOARDING_TASK_ID_DESCRIPTION: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PROFILE",
	2: "seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PLATFORM_REVIEW",
	3: "seller.OnboardingTaskId.description.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT",
};

/** seller.OnboardingTaskId.description；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerOnboardingTaskIdDescription(v: OnboardingTaskId): UiMessage | undefined {
	const key = SELLER_ONBOARDING_TASK_ID_DESCRIPTION[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_ONBOARDING_TASK_ID_PANEL_DESCRIPTION: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.OnboardingTaskId.panel_description.ONBOARDING_TASK_ID_PROFILE",
	2: "seller.OnboardingTaskId.panel_description.ONBOARDING_TASK_ID_PLATFORM_REVIEW",
	3: "seller.OnboardingTaskId.panel_description.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT",
};

/** seller.OnboardingTaskId.panel_description；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerOnboardingTaskIdPanelDescription(v: OnboardingTaskId): UiMessage | undefined {
	const key = SELLER_ONBOARDING_TASK_ID_PANEL_DESCRIPTION[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_ONBOARDING_TASK_ID_PANEL_TITLE: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.OnboardingTaskId.panel_title.ONBOARDING_TASK_ID_PROFILE",
	2: "seller.OnboardingTaskId.panel_title.ONBOARDING_TASK_ID_PLATFORM_REVIEW",
	3: "seller.OnboardingTaskId.panel_title.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT",
};

/** seller.OnboardingTaskId.panel_title；default 受眾沒有文字的值（omit）、UNSPECIFIED 與不認得的值回 undefined。 */
export function sellerOnboardingTaskIdPanelTitle(v: OnboardingTaskId): UiMessage | undefined {
	const key = SELLER_ONBOARDING_TASK_ID_PANEL_TITLE[v];
	if (key === undefined) return undefined;
	return { key };
}

/** seller.PaymentAccount.payouts_inactive_note */
export function sellerPaymentAccountPayoutsInactiveNote(): UiMessage {
	return { key: "seller.PaymentAccount.payouts_inactive_note" };
}

const SELLER_REVOKE_REASON: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.RevokeReason.REVOKE_REASON_APPEAL_ACCEPTED",
	2: "seller.RevokeReason.REVOKE_REASON_ADMIN_CORRECTION",
	3: "seller.RevokeReason.REVOKE_REASON_EXTERNAL_DECISION",
	4: "seller.RevokeReason.REVOKE_REASON_OTHER",
	5: "seller.RevokeReason.REVOKE_REASON_FINDING_REVOKED",
	6: "seller.RevokeReason.REVOKE_REASON_SOURCE_REVOKED",
};

/** seller.RevokeReason 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerRevokeReason(v: RevokeReason): UiMessage | undefined {
	const key = SELLER_REVOKE_REASON[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_RISK_GROUND: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.RiskGround.RISK_GROUND_TRANSACTION",
	2: "seller.RiskGround.RISK_GROUND_FOOD_SAFETY",
	3: "seller.RiskGround.RISK_GROUND_FRAUD",
	4: "seller.RiskGround.RISK_GROUND_PERSONAL_DATA",
	5: "seller.RiskGround.RISK_GROUND_INFRINGEMENT",
	6: "seller.RiskGround.RISK_GROUND_SECURITY",
	7: "seller.RiskGround.RISK_GROUND_FULFILLMENT",
	8: "seller.RiskGround.RISK_GROUND_EXTERNAL_ORDER",
};

/** seller.RiskGround 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerRiskGround(v: RiskGround): UiMessage | undefined {
	const key = SELLER_RISK_GROUND[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_SELLER_REQUIREMENT: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE",
	2: "seller.SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED",
	3: "seller.SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE",
	4: "seller.SellerRequirement.SELLER_REQUIREMENT_STORE_NAME",
	5: "seller.SellerRequirement.SELLER_REQUIREMENT_LOGO",
	6: "seller.SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE",
	7: "seller.SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION",
};

/** seller.SellerRequirement 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerSellerRequirement(v: SellerRequirement): UiMessage | undefined {
	const key = SELLER_SELLER_REQUIREMENT[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_STORE_FIELD: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.StoreField.STORE_FIELD_ADDRESS",
	2: "seller.StoreField.STORE_FIELD_CONTACT_NAME",
	3: "seller.StoreField.STORE_FIELD_CONTACT_MOBILE",
	4: "seller.StoreField.STORE_FIELD_SUPPORT_EMAIL",
	5: "seller.StoreField.STORE_FIELD_CONTACT_PHONE",
};

/** seller.StoreField 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerStoreField(v: StoreField): UiMessage | undefined {
	const key = SELLER_STORE_FIELD[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_TERMINATION_GROUND: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.TerminationGround.TERMINATION_GROUND_NOTICE",
	2: "seller.TerminationGround.TERMINATION_GROUND_MATERIAL_BREACH",
	3: "seller.TerminationGround.TERMINATION_GROUND_CEASED_BUSINESS",
	4: "seller.TerminationGround.TERMINATION_GROUND_DISSOLUTION",
	5: "seller.TerminationGround.TERMINATION_GROUND_BANKRUPTCY",
	6: "seller.TerminationGround.TERMINATION_GROUND_LICENSE_REVOKED",
	7: "seller.TerminationGround.TERMINATION_GROUND_FOOD_SAFETY_INCIDENT",
	8: "seller.TerminationGround.TERMINATION_GROUND_FRAUD",
	9: "seller.TerminationGround.TERMINATION_GROUND_PLATFORM_SECURITY",
	10: "seller.TerminationGround.TERMINATION_GROUND_OTHER_SERIOUS",
	11: "seller.TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT",
};

/** seller.TerminationGround 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerTerminationGround(v: TerminationGround): UiMessage | undefined {
	const key = SELLER_TERMINATION_GROUND[v];
	if (key === undefined) return undefined;
	return { key };
}

const SELLER_VIOLATION_SEVERITY: Readonly<Partial<Record<number, UiTextKey>>> = {
	1: "seller.ViolationSeverity.VIOLATION_SEVERITY_LOW",
	2: "seller.ViolationSeverity.VIOLATION_SEVERITY_MEDIUM",
	3: "seller.ViolationSeverity.VIOLATION_SEVERITY_HIGH",
	4: "seller.ViolationSeverity.VIOLATION_SEVERITY_CRITICAL",
};

/** seller.ViolationSeverity 的名詞；UNSPECIFIED 與不認得的值回 undefined，要顯示「未知」的呼叫端自己用 commonUnknown()。 */
export function sellerViolationSeverity(v: ViolationSeverity): UiMessage | undefined {
	const key = SELLER_VIOLATION_SEVERITY[v];
	if (key === undefined) return undefined;
	return { key };
}

