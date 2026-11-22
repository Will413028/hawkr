// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。
import { Code } from "@connectrpc/connect";
import { assistantAssistantTool, buyerGender, buyerProfileGenderUnset, chatPreviewErased, chatPreviewHidden, chatPreviewImage, commonError, commonPolicyCategory, commonStoreCardClosedStore, commonStoreCardStatus, commonUnknown, moderationReportOutcome, moderationReportTargetKind, paymentPaymentAccountStatus, paymentRequirementCollection, procurementRejectionReason, sellerContentKind, sellerInquiryKind, sellerInquiryReplyChannel, sellerInquiryState, sellerKYCApplicationDecideTime, sellerKYCApplicationSubmitTime, sellerKYCField, sellerKYCStatus, sellerMeasureAppealDeadline, sellerMeasureBasis, sellerMeasureKind, sellerMeasureMetaKind, sellerMeasureMetaSeverity, sellerMeasureMetaTerminationGround, sellerMeasureRejectionCooldownWarning, sellerMeasureRemovedContentsCleared, sellerMeasureRemovedContentsHeading, sellerMeasureRequestKind, sellerMeasureRequestKindSubmittedNote, sellerMeasureRequestReplyNote, sellerMeasureRequestState, sellerMeasureRequestStateWithKind, sellerMeasureState, sellerMeasureSuspensionActiveOpen, sellerMeasureSuspensionActiveUntil, sellerMeasureSuspensionEnded, sellerMeasureTerminationEffective, sellerMeasureTerminationEffectiveEnded, sellerMeasureTerminationNotice, sellerMeasureTitleFallback, sellerMeasureTitleRisk, sellerMeasureTitleRiskFallback, sellerMeasureViewContentLink, sellerOnboardingActionButton, sellerOnboardingTaskIdDescription, sellerOnboardingTaskIdPanelDescription, sellerOnboardingTaskIdPanelTitle, sellerPaymentAccountPayoutsInactiveNote, sellerReportOutcome, sellerRevokeReason, sellerRiskGround, sellerSellerRequirement, sellerStoreField, sellerStoreReportOutcomeUnset, sellerTerminationGround, sellerViolationSeverity, socialCommentStatePlaceholder } from "../src/keys.gen";
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
	"hawkr.assistant.v1alpha1.AssistantTool.ASSISTANT_TOOL_GET_BUYER_ORDER_DETAIL": 2,
	"hawkr.assistant.v1alpha1.AssistantTool.ASSISTANT_TOOL_GET_MY_ORDERS": 1,
	"hawkr.assistant.v1alpha1.AssistantTool.ASSISTANT_TOOL_TRACK_MY_PACKAGE": 3,
	"hawkr.assistant.v1alpha1.AssistantTool.ASSISTANT_TOOL_UNSPECIFIED": 0,
	"hawkr.buyer.v1alpha1.Gender.GENDER_FEMALE": 2,
	"hawkr.buyer.v1alpha1.Gender.GENDER_MALE": 1,
	"hawkr.buyer.v1alpha1.Gender.GENDER_OTHER": 3,
	"hawkr.buyer.v1alpha1.Gender.GENDER_UNSPECIFIED": 0,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_COUNTERFEIT": 6,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_FAKE_CERTIFICATION": 5,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY": 1,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_FRAUD": 8,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_HARASSMENT": 15,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_HATE": 16,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_IMPERSONATION": 11,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_IP_INFRINGEMENT": 7,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_MISLEADING_INFO": 4,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_OFF_PLATFORM_TRADE": 9,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_OTHER": 13,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE": 12,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_PRIVACY": 10,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS": 2,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_SEXUAL_CONTENT": 17,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_SPAM": 14,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_UNLICENSED_SALE": 3,
	"hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_UNSPECIFIED": 0,
	"hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_CLOSED": 3,
	"hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_OPEN": 1,
	"hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_PAUSED": 2,
	"hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_UNSPECIFIED": 0,
	"hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_NO_VIOLATION": 3,
	"hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_PENDING": 1,
	"hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_UNSPECIFIED": 0,
	"hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_VIOLATION": 2,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_BUYER": 4,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_CHAT_MESSAGE": 6,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_COMMENT": 2,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY": 3,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_POST": 1,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_REVIEW": 5,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_REVIEW_REPLY": 7,
	"hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_UNSPECIFIED": 0,
	"hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED": 3,
	"hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED": 5,
	"hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED": 4,
	"hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_NOT_CREATED": 1,
	"hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW": 2,
	"hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNSPECIFIED": 0,
	"hawkr.payment.v1alpha1.RequirementCollection.REQUIREMENT_COLLECTION_PLATFORM": 2,
	"hawkr.payment.v1alpha1.RequirementCollection.REQUIREMENT_COLLECTION_PROVIDER": 1,
	"hawkr.payment.v1alpha1.RequirementCollection.REQUIREMENT_COLLECTION_UNSPECIFIED": 0,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_BUDGET_RANGE": 4,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_CITY_UNRECOGNIZED": 6,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_DATE_FORMAT": 7,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_DATE_MISSING_YEAR": 8,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_DATE_PAST": 9,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_DATE_TOO_FAR": 10,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_EMPTY": 1,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_QUANTITY_RANGE": 3,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_RESOLUTION_NO_ITEM": 11,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_RESOLUTION_NO_LABEL": 12,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_RESOLUTION_UNKNOWN_KIND": 13,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_TRUNCATED": 2,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_UNIT_NOT_ALLOWED": 5,
	"hawkr.procurement.v1alpha1.RejectionReason.REJECTION_REASON_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_CHAT_MESSAGE": 4,
	"hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_COMMENT": 2,
	"hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_FORUM_ENTRY": 3,
	"hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_POST": 1,
	"hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_REVIEW_REPLY": 5,
	"hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.InquiryKind.INQUIRY_KIND_EXPLANATION": 2,
	"hawkr.seller.v1alpha1.InquiryKind.INQUIRY_KIND_RECORDS": 1,
	"hawkr.seller.v1alpha1.InquiryKind.INQUIRY_KIND_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_EMAIL": 2,
	"hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_MESSAGING": 3,
	"hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_OTHER": 4,
	"hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_PHONE": 1,
	"hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_SELLER_CENTER": 5,
	"hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_OPEN": 1,
	"hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_REPLIED": 2,
	"hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_WITHDRAWN": 3,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS": 8,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS": 7,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME": 3,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE": 6,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE": 5,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_TAX_ID": 4,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_PRODUCT_TYPE": 1,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.KYCField.KYC_FIELD_WEBSITE_URL": 2,
	"hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_APPROVED": 3,
	"hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_CANCELLED": 5,
	"hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_DRAFT": 1,
	"hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_REJECTED": 4,
	"hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_SUBMITTED": 2,
	"hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_CONTRACT": 3,
	"hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_RISK": 2,
	"hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_VIOLATION": 1,
	"hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_CONTENT_TAKEDOWN": 4,
	"hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_SUSPENSION": 2,
	"hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_TERMINATION": 3,
	"hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_WARNING": 1,
	"hawkr.seller.v1alpha1.MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL": 1,
	"hawkr.seller.v1alpha1.MeasureRequestKind.MEASURE_REQUEST_KIND_REMEDIATION": 2,
	"hawkr.seller.v1alpha1.MeasureRequestKind.MEASURE_REQUEST_KIND_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED": 2,
	"hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED": 4,
	"hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_PENDING": 1,
	"hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED": 3,
	"hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_ACTIVE": 2,
	"hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_ENDED": 3,
	"hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_ISSUED": 1,
	"hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_REVOKED": 4,
	"hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_SCHEDULED": 5,
	"hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT": 5,
	"hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE": 2,
	"hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_NONE": 1,
	"hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING": 4,
	"hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_SUBMIT_CERTIFICATION": 3,
	"hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT": 3,
	"hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW": 2,
	"hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_PROFILE": 1,
	"hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_ABUSIVE": 5,
	"hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_ACTION_TAKEN": 1,
	"hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_DUPLICATE": 4,
	"hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_INSUFFICIENT": 3,
	"hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_NO_VIOLATION": 2,
	"hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_ADMIN_CORRECTION": 2,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_APPEAL_ACCEPTED": 1,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_EXTERNAL_DECISION": 3,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_FINDING_REVOKED": 5,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_OTHER": 4,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_SOURCE_REVOKED": 6,
	"hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_EXTERNAL_ORDER": 8,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_FOOD_SAFETY": 2,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_FRAUD": 3,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_FULFILLMENT": 7,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_INFRINGEMENT": 5,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_PERSONAL_DATA": 4,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_SECURITY": 6,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_TRANSACTION": 1,
	"hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_LOGO": 5,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION": 7,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED": 2,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE": 3,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE": 1,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_STORE_NAME": 4,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE": 6,
	"hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.StoreField.STORE_FIELD_ADDRESS": 1,
	"hawkr.seller.v1alpha1.StoreField.STORE_FIELD_CONTACT_MOBILE": 3,
	"hawkr.seller.v1alpha1.StoreField.STORE_FIELD_CONTACT_NAME": 2,
	"hawkr.seller.v1alpha1.StoreField.STORE_FIELD_CONTACT_PHONE": 5,
	"hawkr.seller.v1alpha1.StoreField.STORE_FIELD_SUPPORT_EMAIL": 4,
	"hawkr.seller.v1alpha1.StoreField.STORE_FIELD_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_BANKRUPTCY": 5,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_CEASED_BUSINESS": 3,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_DISSOLUTION": 4,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_FOOD_SAFETY_INCIDENT": 7,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_FRAUD": 8,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_LICENSE_REVOKED": 6,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT": 11,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_MATERIAL_BREACH": 2,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_NOTICE": 1,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_OTHER_SERIOUS": 10,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_PLATFORM_SECURITY": 9,
	"hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_UNSPECIFIED": 0,
	"hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_CRITICAL": 4,
	"hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_HIGH": 3,
	"hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_LOW": 1,
	"hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_MEDIUM": 2,
	"hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_UNSPECIFIED": 0,
	"hawkr.social.v1alpha1.CommentState.COMMENT_STATE_DELETED": 2,
	"hawkr.social.v1alpha1.CommentState.COMMENT_STATE_HIDDEN": 3,
	"hawkr.social.v1alpha1.CommentState.COMMENT_STATE_UNSPECIFIED": 0,
	"hawkr.social.v1alpha1.CommentState.COMMENT_STATE_VISIBLE": 1,
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

function num(enumName: string, name: string | number | undefined): number {
	const n = typeof name === "string" ? ENUM_NUMBER[enumName + "." + name] : undefined;
	if (n === undefined) throw new Error("unknown enum value " + String(name));
	return n;
}

/** 呼叫 c.fn 對應的生成函式；undefined 表示「沒有文字」。 */
export function goldenMessage(c: GoldenCase): UiMessage | undefined {
	const p = c.params ?? {};
	switch (c.fn) {
	case "assistant.AssistantTool": {
		return assistantAssistantTool(num("hawkr.assistant.v1alpha1.AssistantTool", c.value));
	}
	case "buyer.Gender": {
		return buyerGender(num("hawkr.buyer.v1alpha1.Gender", c.value));
	}
	case "buyer.Profile.gender_unset": {
		return buyerProfileGenderUnset();
	}
	case "chat.Preview.erased": {
		return chatPreviewErased();
	}
	case "chat.Preview.hidden": {
		return chatPreviewHidden();
	}
	case "chat.Preview.image": {
		return chatPreviewImage();
	}
	case "common.PolicyCategory": {
		return commonPolicyCategory(num("hawkr.common.v1alpha1.PolicyCategory", c.value));
	}
	case "common.StoreCard.closed_store": {
		return commonStoreCardClosedStore();
	}
	case "common.StoreCardStatus": {
		return commonStoreCardStatus(num("hawkr.common.v1alpha1.StoreCardStatus", c.value));
	}
	case "common.error": {
		return commonError(CODES[c.value ?? ""]);
	}
	case "common.unknown": {
		return commonUnknown();
	}
	case "moderation.ReportOutcome": {
		return moderationReportOutcome(num("hawkr.moderation.v1alpha1.ReportOutcome", c.value));
	}
	case "moderation.ReportTargetKind": {
		return moderationReportTargetKind(num("hawkr.moderation.v1alpha1.ReportTargetKind", c.value));
	}
	case "payment.PaymentAccountStatus": {
		return paymentPaymentAccountStatus(num("hawkr.payment.v1alpha1.PaymentAccountStatus", c.value));
	}
	case "payment.RequirementCollection": {
		return paymentRequirementCollection(num("hawkr.payment.v1alpha1.RequirementCollection", c.value));
	}
	case "procurement.RejectionReason": {
		return procurementRejectionReason(num("hawkr.procurement.v1alpha1.RejectionReason", c.value));
	}
	case "seller.ContentKind": {
		return sellerContentKind(num("hawkr.seller.v1alpha1.ContentKind", c.value));
	}
	case "seller.InquiryKind": {
		return sellerInquiryKind(num("hawkr.seller.v1alpha1.InquiryKind", c.value));
	}
	case "seller.InquiryReplyChannel": {
		return sellerInquiryReplyChannel(num("hawkr.seller.v1alpha1.InquiryReplyChannel", c.value));
	}
	case "seller.InquiryState": {
		return sellerInquiryState(num("hawkr.seller.v1alpha1.InquiryState", c.value));
	}
	case "seller.KYCApplication.decide_time": {
		return sellerKYCApplicationDecideTime({ time: p.time as string });
	}
	case "seller.KYCApplication.submit_time": {
		return sellerKYCApplicationSubmitTime({ time: p.time as string });
	}
	case "seller.KYCField": {
		return sellerKYCField(num("hawkr.seller.v1alpha1.KYCField", c.value));
	}
	case "seller.KYCStatus": {
		return sellerKYCStatus(num("hawkr.seller.v1alpha1.KYCStatus", c.value));
	}
	case "seller.Measure.appeal_deadline": {
		return sellerMeasureAppealDeadline({ deadline: p.deadline as string });
	}
	case "seller.Measure.meta_kind": {
		return sellerMeasureMetaKind({ kind: num("hawkr.seller.v1alpha1.MeasureKind", p.kind) });
	}
	case "seller.Measure.meta_severity": {
		return sellerMeasureMetaSeverity({ severity: num("hawkr.seller.v1alpha1.ViolationSeverity", p.severity) });
	}
	case "seller.Measure.meta_termination_ground": {
		return sellerMeasureMetaTerminationGround({ ground: num("hawkr.seller.v1alpha1.TerminationGround", p.ground) });
	}
	case "seller.Measure.rejection_cooldown_warning": {
		return sellerMeasureRejectionCooldownWarning({ days: p.days as number });
	}
	case "seller.Measure.removed_contents_cleared": {
		return sellerMeasureRemovedContentsCleared();
	}
	case "seller.Measure.removed_contents_heading": {
		return sellerMeasureRemovedContentsHeading();
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
		return sellerMeasureTitleRisk({ ground: num("hawkr.seller.v1alpha1.RiskGround", p.ground) });
	}
	case "seller.Measure.title_risk_fallback": {
		return sellerMeasureTitleRiskFallback();
	}
	case "seller.Measure.view_content_link": {
		return sellerMeasureViewContentLink();
	}
	case "seller.MeasureBasis": {
		return sellerMeasureBasis(num("hawkr.seller.v1alpha1.MeasureBasis", c.value));
	}
	case "seller.MeasureKind": {
		return sellerMeasureKind(num("hawkr.seller.v1alpha1.MeasureKind", c.value));
	}
	case "seller.MeasureRequest.reply_note": {
		return sellerMeasureRequestReplyNote({ reason: p.reason as string });
	}
	case "seller.MeasureRequestKind": {
		return sellerMeasureRequestKind(num("hawkr.seller.v1alpha1.MeasureRequestKind", c.value));
	}
	case "seller.MeasureRequestKind.submitted_note": {
		return sellerMeasureRequestKindSubmittedNote(num("hawkr.seller.v1alpha1.MeasureRequestKind", c.value));
	}
	case "seller.MeasureRequestState": {
		return sellerMeasureRequestState(num("hawkr.seller.v1alpha1.MeasureRequestState", c.value));
	}
	case "seller.MeasureRequestState.with_kind": {
		if (c.audience === "staff") return sellerMeasureRequestStateWithKindStaff(num("hawkr.seller.v1alpha1.MeasureRequestState", c.value), { kind: num("hawkr.seller.v1alpha1.MeasureRequestKind", p.kind) });
		return sellerMeasureRequestStateWithKind(num("hawkr.seller.v1alpha1.MeasureRequestState", c.value), { kind: num("hawkr.seller.v1alpha1.MeasureRequestKind", p.kind) });
	}
	case "seller.MeasureState": {
		return sellerMeasureState(num("hawkr.seller.v1alpha1.MeasureState", c.value), { kind: num("hawkr.seller.v1alpha1.MeasureKind", p.kind) });
	}
	case "seller.OnboardingAction.button": {
		return sellerOnboardingActionButton(num("hawkr.seller.v1alpha1.OnboardingAction", c.value));
	}
	case "seller.OnboardingTaskId.description": {
		return sellerOnboardingTaskIdDescription(num("hawkr.seller.v1alpha1.OnboardingTaskId", c.value));
	}
	case "seller.OnboardingTaskId.panel_description": {
		return sellerOnboardingTaskIdPanelDescription(num("hawkr.seller.v1alpha1.OnboardingTaskId", c.value));
	}
	case "seller.OnboardingTaskId.panel_title": {
		return sellerOnboardingTaskIdPanelTitle(num("hawkr.seller.v1alpha1.OnboardingTaskId", c.value));
	}
	case "seller.PaymentAccount.payouts_inactive_note": {
		return sellerPaymentAccountPayoutsInactiveNote();
	}
	case "seller.ReportOutcome": {
		return sellerReportOutcome(num("hawkr.seller.v1alpha1.ReportOutcome", c.value));
	}
	case "seller.RevokeReason": {
		return sellerRevokeReason(num("hawkr.seller.v1alpha1.RevokeReason", c.value));
	}
	case "seller.RiskGround": {
		return sellerRiskGround(num("hawkr.seller.v1alpha1.RiskGround", c.value));
	}
	case "seller.SellerRequirement": {
		return sellerSellerRequirement(num("hawkr.seller.v1alpha1.SellerRequirement", c.value));
	}
	case "seller.StoreField": {
		return sellerStoreField(num("hawkr.seller.v1alpha1.StoreField", c.value));
	}
	case "seller.StoreReport.outcome_unset": {
		return sellerStoreReportOutcomeUnset();
	}
	case "seller.TerminationGround": {
		return sellerTerminationGround(num("hawkr.seller.v1alpha1.TerminationGround", c.value));
	}
	case "seller.ViolationSeverity": {
		return sellerViolationSeverity(num("hawkr.seller.v1alpha1.ViolationSeverity", c.value));
	}
	case "social.CommentState.placeholder": {
		return socialCommentStatePlaceholder(num("hawkr.social.v1alpha1.CommentState", c.value));
	}
	default:
		throw new Error("unknown golden fn " + c.fn);
	}
}
