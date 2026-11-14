// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。
import { Code } from "@connectrpc/connect";
import { buyerGender, buyerProfileGenderUnset, commonError, commonPolicyCategory, commonStoreCardClosedStore, commonStoreCardStatus, commonUnknown, paymentPaymentAccountStatus, paymentRequirementCollection, sellerContentKind, sellerKYCApplicationDecideTime, sellerKYCApplicationSubmitTime, sellerKYCField, sellerKYCStatus, sellerMeasureAppealDeadline, sellerMeasureBasis, sellerMeasureKind, sellerMeasureMetaKind, sellerMeasureMetaSeverity, sellerMeasureMetaTerminationGround, sellerMeasureRejectionCooldownWarning, sellerMeasureRequestKind, sellerMeasureRequestKindSubmittedNote, sellerMeasureRequestReplyNote, sellerMeasureRequestState, sellerMeasureRequestStateWithKind, sellerMeasureState, sellerMeasureSuspensionActiveOpen, sellerMeasureSuspensionActiveUntil, sellerMeasureSuspensionEnded, sellerMeasureTerminationEffective, sellerMeasureTerminationEffectiveEnded, sellerMeasureTerminationNotice, sellerMeasureTitleFallback, sellerMeasureTitleRisk, sellerMeasureTitleRiskFallback, sellerOnboardingActionButton, sellerOnboardingTaskIdDescription, sellerOnboardingTaskIdPanelDescription, sellerOnboardingTaskIdPanelTitle, sellerPaymentAccountPayoutsInactiveNote, sellerRevokeReason, sellerRiskGround, sellerSellerRequirement, sellerStoreField, sellerTerminationGround, sellerViolationSeverity } from "../src/keys.gen";
import { sellerMeasureRequestStateWithKindStaff } from "../src/zh-tw.gen";
import type { UiMessage } from "../src/message";

export interface GoldenCase {
	key: string;
	fn: string;
	value?: string;
	audience: "default" | "staff";
	locale: string;
	params?: Record<string, string | number>;
	want: string | null;
}

const ENUM_NUMBER: Readonly<Record<string, number>> = {
	"CONTENT_KIND_COMMENT": 2,
	"CONTENT_KIND_FORUM_ENTRY": 3,
	"CONTENT_KIND_POST": 1,
	"CONTENT_KIND_UNSPECIFIED": 0,
	"GENDER_FEMALE": 2,
	"GENDER_MALE": 1,
	"GENDER_OTHER": 3,
	"GENDER_UNSPECIFIED": 0,
	"KYC_FIELD_BUSINESS_COMPANY_ADDRESS": 8,
	"KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS": 7,
	"KYC_FIELD_BUSINESS_COMPANY_NAME": 3,
	"KYC_FIELD_BUSINESS_COMPANY_PHONE": 6,
	"KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE": 5,
	"KYC_FIELD_BUSINESS_TAX_ID": 4,
	"KYC_FIELD_PRODUCT_TYPE": 1,
	"KYC_FIELD_UNSPECIFIED": 0,
	"KYC_FIELD_WEBSITE_URL": 2,
	"KYC_STATUS_APPROVED": 3,
	"KYC_STATUS_CANCELLED": 5,
	"KYC_STATUS_DRAFT": 1,
	"KYC_STATUS_REJECTED": 4,
	"KYC_STATUS_SUBMITTED": 2,
	"KYC_STATUS_UNSPECIFIED": 0,
	"MEASURE_BASIS_CONTRACT": 3,
	"MEASURE_BASIS_RISK": 2,
	"MEASURE_BASIS_UNSPECIFIED": 0,
	"MEASURE_BASIS_VIOLATION": 1,
	"MEASURE_KIND_CONTENT_TAKEDOWN": 4,
	"MEASURE_KIND_SUSPENSION": 2,
	"MEASURE_KIND_TERMINATION": 3,
	"MEASURE_KIND_UNSPECIFIED": 0,
	"MEASURE_KIND_WARNING": 1,
	"MEASURE_REQUEST_KIND_APPEAL": 1,
	"MEASURE_REQUEST_KIND_REMEDIATION": 2,
	"MEASURE_REQUEST_KIND_UNSPECIFIED": 0,
	"MEASURE_REQUEST_STATE_ACCEPTED": 2,
	"MEASURE_REQUEST_STATE_CLOSED": 4,
	"MEASURE_REQUEST_STATE_PENDING": 1,
	"MEASURE_REQUEST_STATE_REJECTED": 3,
	"MEASURE_REQUEST_STATE_UNSPECIFIED": 0,
	"MEASURE_STATE_ACTIVE": 2,
	"MEASURE_STATE_ENDED": 3,
	"MEASURE_STATE_ISSUED": 1,
	"MEASURE_STATE_REVOKED": 4,
	"MEASURE_STATE_SCHEDULED": 5,
	"MEASURE_STATE_UNSPECIFIED": 0,
	"ONBOARDING_ACTION_CONTACT_SUPPORT": 5,
	"ONBOARDING_ACTION_EDIT_PROFILE": 2,
	"ONBOARDING_ACTION_NONE": 1,
	"ONBOARDING_ACTION_START_PAYMENT_ONBOARDING": 4,
	"ONBOARDING_ACTION_SUBMIT_CERTIFICATION": 3,
	"ONBOARDING_ACTION_UNSPECIFIED": 0,
	"ONBOARDING_TASK_ID_PAYMENT_ACCOUNT": 3,
	"ONBOARDING_TASK_ID_PLATFORM_REVIEW": 2,
	"ONBOARDING_TASK_ID_PROFILE": 1,
	"ONBOARDING_TASK_ID_UNSPECIFIED": 0,
	"PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED": 3,
	"PAYMENT_ACCOUNT_STATUS_DISABLED": 5,
	"PAYMENT_ACCOUNT_STATUS_ENABLED": 4,
	"PAYMENT_ACCOUNT_STATUS_NOT_CREATED": 1,
	"PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW": 2,
	"PAYMENT_ACCOUNT_STATUS_UNSPECIFIED": 0,
	"POLICY_CATEGORY_COUNTERFEIT": 6,
	"POLICY_CATEGORY_FAKE_CERTIFICATION": 5,
	"POLICY_CATEGORY_FOOD_SAFETY": 1,
	"POLICY_CATEGORY_FRAUD": 8,
	"POLICY_CATEGORY_HARASSMENT": 15,
	"POLICY_CATEGORY_HATE": 16,
	"POLICY_CATEGORY_IMPERSONATION": 11,
	"POLICY_CATEGORY_IP_INFRINGEMENT": 7,
	"POLICY_CATEGORY_MISLEADING_INFO": 4,
	"POLICY_CATEGORY_OFF_PLATFORM_TRADE": 9,
	"POLICY_CATEGORY_OTHER": 13,
	"POLICY_CATEGORY_PLATFORM_ABUSE": 12,
	"POLICY_CATEGORY_PRIVACY": 10,
	"POLICY_CATEGORY_PROHIBITED_GOODS": 2,
	"POLICY_CATEGORY_SPAM": 14,
	"POLICY_CATEGORY_UNLICENSED_SALE": 3,
	"POLICY_CATEGORY_UNSPECIFIED": 0,
	"REQUIREMENT_COLLECTION_PLATFORM": 2,
	"REQUIREMENT_COLLECTION_PROVIDER": 1,
	"REQUIREMENT_COLLECTION_UNSPECIFIED": 0,
	"REVOKE_REASON_ADMIN_CORRECTION": 2,
	"REVOKE_REASON_APPEAL_ACCEPTED": 1,
	"REVOKE_REASON_EXTERNAL_DECISION": 3,
	"REVOKE_REASON_FINDING_REVOKED": 5,
	"REVOKE_REASON_OTHER": 4,
	"REVOKE_REASON_SOURCE_REVOKED": 6,
	"REVOKE_REASON_UNSPECIFIED": 0,
	"RISK_GROUND_EXTERNAL_ORDER": 8,
	"RISK_GROUND_FOOD_SAFETY": 2,
	"RISK_GROUND_FRAUD": 3,
	"RISK_GROUND_FULFILLMENT": 7,
	"RISK_GROUND_INFRINGEMENT": 5,
	"RISK_GROUND_PERSONAL_DATA": 4,
	"RISK_GROUND_SECURITY": 6,
	"RISK_GROUND_TRANSACTION": 1,
	"RISK_GROUND_UNSPECIFIED": 0,
	"SELLER_REQUIREMENT_LOGO": 5,
	"SELLER_REQUIREMENT_NOT_PENDING_DELETION": 7,
	"SELLER_REQUIREMENT_NOT_SUSPENDED": 2,
	"SELLER_REQUIREMENT_ONBOARDING_COMPLETE": 3,
	"SELLER_REQUIREMENT_STORE_ACTIVE": 1,
	"SELLER_REQUIREMENT_STORE_NAME": 4,
	"SELLER_REQUIREMENT_TOWN_CODE": 6,
	"SELLER_REQUIREMENT_UNSPECIFIED": 0,
	"STORE_CARD_STATUS_CLOSED": 3,
	"STORE_CARD_STATUS_OPEN": 1,
	"STORE_CARD_STATUS_PAUSED": 2,
	"STORE_CARD_STATUS_UNSPECIFIED": 0,
	"STORE_FIELD_ADDRESS": 1,
	"STORE_FIELD_CONTACT_MOBILE": 3,
	"STORE_FIELD_CONTACT_NAME": 2,
	"STORE_FIELD_CONTACT_PHONE": 5,
	"STORE_FIELD_SUPPORT_EMAIL": 4,
	"STORE_FIELD_UNSPECIFIED": 0,
	"TERMINATION_GROUND_BANKRUPTCY": 5,
	"TERMINATION_GROUND_CEASED_BUSINESS": 3,
	"TERMINATION_GROUND_DISSOLUTION": 4,
	"TERMINATION_GROUND_FOOD_SAFETY_INCIDENT": 7,
	"TERMINATION_GROUND_FRAUD": 8,
	"TERMINATION_GROUND_LICENSE_REVOKED": 6,
	"TERMINATION_GROUND_LINKED_ACCOUNT": 11,
	"TERMINATION_GROUND_MATERIAL_BREACH": 2,
	"TERMINATION_GROUND_NOTICE": 1,
	"TERMINATION_GROUND_OTHER_SERIOUS": 10,
	"TERMINATION_GROUND_PLATFORM_SECURITY": 9,
	"TERMINATION_GROUND_UNSPECIFIED": 0,
	"VIOLATION_SEVERITY_CRITICAL": 4,
	"VIOLATION_SEVERITY_HIGH": 3,
	"VIOLATION_SEVERITY_LOW": 1,
	"VIOLATION_SEVERITY_MEDIUM": 2,
	"VIOLATION_SEVERITY_UNSPECIFIED": 0,
};

const CODES: Readonly<Record<string, Code>> = {
	"UNKNOWN": Code.Unknown,
	"INVALID_ARGUMENT": Code.InvalidArgument,
	"DEADLINE_EXCEEDED": Code.DeadlineExceeded,
	"NOT_FOUND": Code.NotFound,
	"ALREADY_EXISTS": Code.AlreadyExists,
	"PERMISSION_DENIED": Code.PermissionDenied,
	"RESOURCE_EXHAUSTED": Code.ResourceExhausted,
	"FAILED_PRECONDITION": Code.FailedPrecondition,
	"ABORTED": Code.Aborted,
	"OUT_OF_RANGE": Code.OutOfRange,
	"UNIMPLEMENTED": Code.Unimplemented,
	"INTERNAL": Code.Internal,
	"UNAVAILABLE": Code.Unavailable,
	"DATA_LOSS": Code.DataLoss,
	"UNAUTHENTICATED": Code.Unauthenticated,
};

function num(name: string | number | undefined): number {
	const n = typeof name === "string" ? ENUM_NUMBER[name] : undefined;
	if (n === undefined) throw new Error("unknown enum value " + String(name));
	return n;
}

/** 呼叫 c.fn 對應的生成函式；undefined 表示「沒有文字」。 */
export function goldenMessage(c: GoldenCase): UiMessage | undefined {
	const p = c.params ?? {};
	switch (c.fn) {
	case "buyer.Gender": {
		return buyerGender(num(c.value));
	}
	case "buyer.Profile.gender_unset": {
		return buyerProfileGenderUnset();
	}
	case "common.PolicyCategory": {
		return commonPolicyCategory(num(c.value));
	}
	case "common.StoreCard.closed_store": {
		return commonStoreCardClosedStore();
	}
	case "common.StoreCardStatus": {
		return commonStoreCardStatus(num(c.value));
	}
	case "common.error": {
		return commonError(CODES[c.value ?? ""]);
	}
	case "common.unknown": {
		return commonUnknown();
	}
	case "payment.PaymentAccountStatus": {
		return paymentPaymentAccountStatus(num(c.value));
	}
	case "payment.RequirementCollection": {
		return paymentRequirementCollection(num(c.value));
	}
	case "seller.ContentKind": {
		return sellerContentKind(num(c.value));
	}
	case "seller.KYCApplication.decide_time": {
		return sellerKYCApplicationDecideTime({ time: p.time as string });
	}
	case "seller.KYCApplication.submit_time": {
		return sellerKYCApplicationSubmitTime({ time: p.time as string });
	}
	case "seller.KYCField": {
		return sellerKYCField(num(c.value));
	}
	case "seller.KYCStatus": {
		return sellerKYCStatus(num(c.value));
	}
	case "seller.Measure.appeal_deadline": {
		return sellerMeasureAppealDeadline({ deadline: p.deadline as string });
	}
	case "seller.Measure.meta_kind": {
		return sellerMeasureMetaKind({ kind: num(p.kind) });
	}
	case "seller.Measure.meta_severity": {
		return sellerMeasureMetaSeverity({ severity: num(p.severity) });
	}
	case "seller.Measure.meta_termination_ground": {
		return sellerMeasureMetaTerminationGround({ ground: num(p.ground) });
	}
	case "seller.Measure.rejection_cooldown_warning": {
		return sellerMeasureRejectionCooldownWarning({ days: p.days as number });
	}
	case "seller.Measure.suspension_active_open": {
		return sellerMeasureSuspensionActiveOpen();
	}
	case "seller.Measure.suspension_active_until": {
		return sellerMeasureSuspensionActiveUntil({ end: p.end as string });
	}
	case "seller.Measure.suspension_ended": {
		return sellerMeasureSuspensionEnded({ end: p.end as string });
	}
	case "seller.Measure.termination_effective": {
		return sellerMeasureTerminationEffective({ start: p.start as string });
	}
	case "seller.Measure.termination_effective_ended": {
		return sellerMeasureTerminationEffectiveEnded({ start: p.start as string, end: p.end as string });
	}
	case "seller.Measure.termination_notice": {
		return sellerMeasureTerminationNotice({ start: p.start as string });
	}
	case "seller.Measure.title_fallback": {
		return sellerMeasureTitleFallback();
	}
	case "seller.Measure.title_risk": {
		return sellerMeasureTitleRisk({ ground: num(p.ground) });
	}
	case "seller.Measure.title_risk_fallback": {
		return sellerMeasureTitleRiskFallback();
	}
	case "seller.MeasureBasis": {
		return sellerMeasureBasis(num(c.value));
	}
	case "seller.MeasureKind": {
		return sellerMeasureKind(num(c.value));
	}
	case "seller.MeasureRequest.reply_note": {
		return sellerMeasureRequestReplyNote({ reason: p.reason as string });
	}
	case "seller.MeasureRequestKind": {
		return sellerMeasureRequestKind(num(c.value));
	}
	case "seller.MeasureRequestKind.submitted_note": {
		return sellerMeasureRequestKindSubmittedNote(num(c.value));
	}
	case "seller.MeasureRequestState": {
		return sellerMeasureRequestState(num(c.value));
	}
	case "seller.MeasureRequestState.with_kind": {
		if (c.audience === "staff") return sellerMeasureRequestStateWithKindStaff(num(c.value), { kind: num(p.kind) });
		return sellerMeasureRequestStateWithKind(num(c.value), { kind: num(p.kind) });
	}
	case "seller.MeasureState": {
		return sellerMeasureState(num(c.value), { kind: num(p.kind) });
	}
	case "seller.OnboardingAction.button": {
		return sellerOnboardingActionButton(num(c.value));
	}
	case "seller.OnboardingTaskId.description": {
		return sellerOnboardingTaskIdDescription(num(c.value));
	}
	case "seller.OnboardingTaskId.panel_description": {
		return sellerOnboardingTaskIdPanelDescription(num(c.value));
	}
	case "seller.OnboardingTaskId.panel_title": {
		return sellerOnboardingTaskIdPanelTitle(num(c.value));
	}
	case "seller.PaymentAccount.payouts_inactive_note": {
		return sellerPaymentAccountPayoutsInactiveNote();
	}
	case "seller.RevokeReason": {
		return sellerRevokeReason(num(c.value));
	}
	case "seller.RiskGround": {
		return sellerRiskGround(num(c.value));
	}
	case "seller.SellerRequirement": {
		return sellerSellerRequirement(num(c.value));
	}
	case "seller.StoreField": {
		return sellerStoreField(num(c.value));
	}
	case "seller.TerminationGround": {
		return sellerTerminationGround(num(c.value));
	}
	case "seller.ViolationSeverity": {
		return sellerViolationSeverity(num(c.value));
	}
	default:
		throw new Error("unknown golden fn " + c.fn);
	}
}
