// dart format off
// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。

// ignore_for_file: unnecessary_parenthesis, unused_import

import 'package:connectrpc/connect.dart' show Code;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart' as hawkr_buyer_v1alpha1_buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart' as hawkr_common_v1alpha1_policy_category;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pbenum.dart' as hawkr_common_v1alpha1_store_card;
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart' as hawkr_moderation_v1alpha1_moderation;
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart' as hawkr_payment_v1alpha1_payment;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pbenum.dart' as hawkr_seller_v1alpha1_report;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart' as hawkr_seller_v1alpha1_seller;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart' as hawkr_social_v1alpha1_social;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

const _enumNumber = <String, int>{
  'hawkr.buyer.v1alpha1.Gender.GENDER_FEMALE': 2,
	'hawkr.buyer.v1alpha1.Gender.GENDER_MALE': 1,
	'hawkr.buyer.v1alpha1.Gender.GENDER_OTHER': 3,
	'hawkr.buyer.v1alpha1.Gender.GENDER_UNSPECIFIED': 0,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_COUNTERFEIT': 6,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_FAKE_CERTIFICATION': 5,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY': 1,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_FRAUD': 8,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_HARASSMENT': 15,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_HATE': 16,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_IMPERSONATION': 11,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_IP_INFRINGEMENT': 7,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_MISLEADING_INFO': 4,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_OFF_PLATFORM_TRADE': 9,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_OTHER': 13,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE': 12,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_PRIVACY': 10,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS': 2,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_SEXUAL_CONTENT': 17,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_SPAM': 14,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_UNLICENSED_SALE': 3,
	'hawkr.common.v1alpha1.PolicyCategory.POLICY_CATEGORY_UNSPECIFIED': 0,
	'hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_CLOSED': 3,
	'hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_OPEN': 1,
	'hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_PAUSED': 2,
	'hawkr.common.v1alpha1.StoreCardStatus.STORE_CARD_STATUS_UNSPECIFIED': 0,
	'hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_NO_VIOLATION': 3,
	'hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_PENDING': 1,
	'hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_UNSPECIFIED': 0,
	'hawkr.moderation.v1alpha1.ReportOutcome.REPORT_OUTCOME_VIOLATION': 2,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_BUYER': 4,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_CHAT_MESSAGE': 6,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_COMMENT': 2,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY': 3,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_POST': 1,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_REVIEW': 5,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_REVIEW_REPLY': 7,
	'hawkr.moderation.v1alpha1.ReportTargetKind.REPORT_TARGET_KIND_UNSPECIFIED': 0,
	'hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED': 3,
	'hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED': 5,
	'hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED': 4,
	'hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_NOT_CREATED': 1,
	'hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW': 2,
	'hawkr.payment.v1alpha1.PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNSPECIFIED': 0,
	'hawkr.payment.v1alpha1.RequirementCollection.REQUIREMENT_COLLECTION_PLATFORM': 2,
	'hawkr.payment.v1alpha1.RequirementCollection.REQUIREMENT_COLLECTION_PROVIDER': 1,
	'hawkr.payment.v1alpha1.RequirementCollection.REQUIREMENT_COLLECTION_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_CHAT_MESSAGE': 4,
	'hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_COMMENT': 2,
	'hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_FORUM_ENTRY': 3,
	'hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_POST': 1,
	'hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_REVIEW_REPLY': 5,
	'hawkr.seller.v1alpha1.ContentKind.CONTENT_KIND_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.InquiryKind.INQUIRY_KIND_EXPLANATION': 2,
	'hawkr.seller.v1alpha1.InquiryKind.INQUIRY_KIND_RECORDS': 1,
	'hawkr.seller.v1alpha1.InquiryKind.INQUIRY_KIND_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_EMAIL': 2,
	'hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_MESSAGING': 3,
	'hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_OTHER': 4,
	'hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_PHONE': 1,
	'hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_SELLER_CENTER': 5,
	'hawkr.seller.v1alpha1.InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_OPEN': 1,
	'hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_REPLIED': 2,
	'hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.InquiryState.INQUIRY_STATE_WITHDRAWN': 3,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS': 8,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS': 7,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME': 3,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE': 6,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE': 5,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_BUSINESS_TAX_ID': 4,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_PRODUCT_TYPE': 1,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.KYCField.KYC_FIELD_WEBSITE_URL': 2,
	'hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_APPROVED': 3,
	'hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_CANCELLED': 5,
	'hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_DRAFT': 1,
	'hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_REJECTED': 4,
	'hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_SUBMITTED': 2,
	'hawkr.seller.v1alpha1.KYCStatus.KYC_STATUS_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_CONTRACT': 3,
	'hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_RISK': 2,
	'hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.MeasureBasis.MEASURE_BASIS_VIOLATION': 1,
	'hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_CONTENT_TAKEDOWN': 4,
	'hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_SUSPENSION': 2,
	'hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_TERMINATION': 3,
	'hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.MeasureKind.MEASURE_KIND_WARNING': 1,
	'hawkr.seller.v1alpha1.MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL': 1,
	'hawkr.seller.v1alpha1.MeasureRequestKind.MEASURE_REQUEST_KIND_REMEDIATION': 2,
	'hawkr.seller.v1alpha1.MeasureRequestKind.MEASURE_REQUEST_KIND_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED': 2,
	'hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED': 4,
	'hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_PENDING': 1,
	'hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED': 3,
	'hawkr.seller.v1alpha1.MeasureRequestState.MEASURE_REQUEST_STATE_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_ACTIVE': 2,
	'hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_ENDED': 3,
	'hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_ISSUED': 1,
	'hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_REVOKED': 4,
	'hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_SCHEDULED': 5,
	'hawkr.seller.v1alpha1.MeasureState.MEASURE_STATE_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT': 5,
	'hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE': 2,
	'hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_NONE': 1,
	'hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING': 4,
	'hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_SUBMIT_CERTIFICATION': 3,
	'hawkr.seller.v1alpha1.OnboardingAction.ONBOARDING_ACTION_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT': 3,
	'hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW': 2,
	'hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_PROFILE': 1,
	'hawkr.seller.v1alpha1.OnboardingTaskId.ONBOARDING_TASK_ID_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_ABUSIVE': 5,
	'hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_ACTION_TAKEN': 1,
	'hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_DUPLICATE': 4,
	'hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_INSUFFICIENT': 3,
	'hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_NO_VIOLATION': 2,
	'hawkr.seller.v1alpha1.ReportOutcome.REPORT_OUTCOME_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_ADMIN_CORRECTION': 2,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_APPEAL_ACCEPTED': 1,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_EXTERNAL_DECISION': 3,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_FINDING_REVOKED': 5,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_OTHER': 4,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_SOURCE_REVOKED': 6,
	'hawkr.seller.v1alpha1.RevokeReason.REVOKE_REASON_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_EXTERNAL_ORDER': 8,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_FOOD_SAFETY': 2,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_FRAUD': 3,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_FULFILLMENT': 7,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_INFRINGEMENT': 5,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_PERSONAL_DATA': 4,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_SECURITY': 6,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_TRANSACTION': 1,
	'hawkr.seller.v1alpha1.RiskGround.RISK_GROUND_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_LOGO': 5,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION': 7,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED': 2,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE': 3,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE': 1,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_STORE_NAME': 4,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE': 6,
	'hawkr.seller.v1alpha1.SellerRequirement.SELLER_REQUIREMENT_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.StoreField.STORE_FIELD_ADDRESS': 1,
	'hawkr.seller.v1alpha1.StoreField.STORE_FIELD_CONTACT_MOBILE': 3,
	'hawkr.seller.v1alpha1.StoreField.STORE_FIELD_CONTACT_NAME': 2,
	'hawkr.seller.v1alpha1.StoreField.STORE_FIELD_CONTACT_PHONE': 5,
	'hawkr.seller.v1alpha1.StoreField.STORE_FIELD_SUPPORT_EMAIL': 4,
	'hawkr.seller.v1alpha1.StoreField.STORE_FIELD_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_BANKRUPTCY': 5,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_CEASED_BUSINESS': 3,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_DISSOLUTION': 4,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_FOOD_SAFETY_INCIDENT': 7,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_FRAUD': 8,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_LICENSE_REVOKED': 6,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT': 11,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_MATERIAL_BREACH': 2,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_NOTICE': 1,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_OTHER_SERIOUS': 10,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_PLATFORM_SECURITY': 9,
	'hawkr.seller.v1alpha1.TerminationGround.TERMINATION_GROUND_UNSPECIFIED': 0,
	'hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_CRITICAL': 4,
	'hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_HIGH': 3,
	'hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_LOW': 1,
	'hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_MEDIUM': 2,
	'hawkr.seller.v1alpha1.ViolationSeverity.VIOLATION_SEVERITY_UNSPECIFIED': 0,
	'hawkr.social.v1alpha1.CommentState.COMMENT_STATE_DELETED': 2,
	'hawkr.social.v1alpha1.CommentState.COMMENT_STATE_HIDDEN': 3,
	'hawkr.social.v1alpha1.CommentState.COMMENT_STATE_UNSPECIFIED': 0,
	'hawkr.social.v1alpha1.CommentState.COMMENT_STATE_VISIBLE': 1,
};

const _codes = <String, Code>{
  'UNKNOWN': Code.unknown,
  'INVALID_ARGUMENT': Code.invalidArgument,
  'DEADLINE_EXCEEDED': Code.deadlineExceeded,
  'NOT_FOUND': Code.notFound,
  'ALREADY_EXISTS': Code.alreadyExists,
  'PERMISSION_DENIED': Code.permissionDenied,
  'RESOURCE_EXHAUSTED': Code.resourceExhausted,
  'FAILED_PRECONDITION': Code.failedPrecondition,
  'ABORTED': Code.aborted,
  'OUT_OF_RANGE': Code.outOfRange,
  'UNIMPLEMENTED': Code.unimplemented,
  'INTERNAL': Code.internal,
  'UNAVAILABLE': Code.unavailable,
  'DATA_LOSS': Code.dataLoss,
  'UNAUTHENTICATED': Code.unauthenticated,
};

int _n(String enumName, Object? name) => _enumNumber['$enumName.$name']!;

/// 呼叫 fn 對應的生成函式；null 表示「沒有文字」。
String? goldenText(String fn, String value, Map<String, Object?> params, UiLocale l) {
  switch (fn) {
    case 'buyer.Gender':
      return hawkr_buyer_v1alpha1_buyer.Gender.valueOf(_n('hawkr.buyer.v1alpha1.Gender', value))!.uiText(l);
    case 'buyer.Profile.gender_unset':
      return buyerProfileGenderUnset(l);
    case 'chat.Preview.erased':
      return chatPreviewErased(l);
    case 'chat.Preview.hidden':
      return chatPreviewHidden(l);
    case 'chat.Preview.image':
      return chatPreviewImage(l);
    case 'common.PolicyCategory':
      return hawkr_common_v1alpha1_policy_category.PolicyCategory.valueOf(_n('hawkr.common.v1alpha1.PolicyCategory', value))!.uiText(l);
    case 'common.StoreCard.closed_store':
      return commonStoreCardClosedStore(l);
    case 'common.StoreCardStatus':
      return hawkr_common_v1alpha1_store_card.StoreCardStatus.valueOf(_n('hawkr.common.v1alpha1.StoreCardStatus', value))!.uiText(l);
    case 'common.error':
      return commonErrorText(_codes[value]!, l);
    case 'common.unknown':
      return commonUnknown(l);
    case 'moderation.ReportOutcome':
      return hawkr_moderation_v1alpha1_moderation.ReportOutcome.valueOf(_n('hawkr.moderation.v1alpha1.ReportOutcome', value))!.uiText(l);
    case 'moderation.ReportTargetKind':
      return hawkr_moderation_v1alpha1_moderation.ReportTargetKind.valueOf(_n('hawkr.moderation.v1alpha1.ReportTargetKind', value))!.uiText(l);
    case 'payment.PaymentAccountStatus':
      return hawkr_payment_v1alpha1_payment.PaymentAccountStatus.valueOf(_n('hawkr.payment.v1alpha1.PaymentAccountStatus', value))!.uiText(l);
    case 'payment.RequirementCollection':
      return hawkr_payment_v1alpha1_payment.RequirementCollection.valueOf(_n('hawkr.payment.v1alpha1.RequirementCollection', value))!.uiText(l);
    case 'seller.ContentKind':
      return hawkr_seller_v1alpha1_seller.ContentKind.valueOf(_n('hawkr.seller.v1alpha1.ContentKind', value))!.uiText(l);
    case 'seller.InquiryKind':
      return hawkr_seller_v1alpha1_seller.InquiryKind.valueOf(_n('hawkr.seller.v1alpha1.InquiryKind', value))!.uiText(l);
    case 'seller.InquiryReplyChannel':
      return hawkr_seller_v1alpha1_seller.InquiryReplyChannel.valueOf(_n('hawkr.seller.v1alpha1.InquiryReplyChannel', value))!.uiText(l);
    case 'seller.InquiryState':
      return hawkr_seller_v1alpha1_seller.InquiryState.valueOf(_n('hawkr.seller.v1alpha1.InquiryState', value))!.uiText(l);
    case 'seller.KYCApplication.decide_time':
      return sellerKYCApplicationDecideTime(l, time: params['time']! as String);
    case 'seller.KYCApplication.submit_time':
      return sellerKYCApplicationSubmitTime(l, time: params['time']! as String);
    case 'seller.KYCField':
      return hawkr_seller_v1alpha1_seller.KYCField.valueOf(_n('hawkr.seller.v1alpha1.KYCField', value))!.uiText(l);
    case 'seller.KYCStatus':
      return hawkr_seller_v1alpha1_seller.KYCStatus.valueOf(_n('hawkr.seller.v1alpha1.KYCStatus', value))!.uiText(l);
    case 'seller.Measure.appeal_deadline':
      return sellerMeasureAppealDeadline(l, deadline: params['deadline']! as String);
    case 'seller.Measure.meta_kind':
      return sellerMeasureMetaKind(l, kind: hawkr_seller_v1alpha1_seller.MeasureKind.valueOf(_n('hawkr.seller.v1alpha1.MeasureKind', params['kind']))!);
    case 'seller.Measure.meta_severity':
      return sellerMeasureMetaSeverity(l, severity: hawkr_seller_v1alpha1_seller.ViolationSeverity.valueOf(_n('hawkr.seller.v1alpha1.ViolationSeverity', params['severity']))!);
    case 'seller.Measure.meta_termination_ground':
      return sellerMeasureMetaTerminationGround(l, ground: hawkr_seller_v1alpha1_seller.TerminationGround.valueOf(_n('hawkr.seller.v1alpha1.TerminationGround', params['ground']))!);
    case 'seller.Measure.rejection_cooldown_warning':
      return sellerMeasureRejectionCooldownWarning(l, days: (params['days']! as num).toInt());
    case 'seller.Measure.removed_contents_cleared':
      return sellerMeasureRemovedContentsCleared(l);
    case 'seller.Measure.removed_contents_heading':
      return sellerMeasureRemovedContentsHeading(l);
    case 'seller.Measure.suspension_active_open':
      return sellerMeasureSuspensionActiveOpen(l);
    case 'seller.Measure.suspension_active_until':
      return sellerMeasureSuspensionActiveUntil(l, end: params['end']! as String);
    case 'seller.Measure.suspension_ended':
      return sellerMeasureSuspensionEnded(l, end: params['end']! as String);
    case 'seller.Measure.termination_effective':
      return sellerMeasureTerminationEffective(l, start: params['start']! as String);
    case 'seller.Measure.termination_effective_ended':
      return sellerMeasureTerminationEffectiveEnded(l, start: params['start']! as String, end: params['end']! as String);
    case 'seller.Measure.termination_notice':
      return sellerMeasureTerminationNotice(l, start: params['start']! as String);
    case 'seller.Measure.title_fallback':
      return sellerMeasureTitleFallback(l);
    case 'seller.Measure.title_risk':
      return sellerMeasureTitleRisk(l, ground: hawkr_seller_v1alpha1_seller.RiskGround.valueOf(_n('hawkr.seller.v1alpha1.RiskGround', params['ground']))!);
    case 'seller.Measure.title_risk_fallback':
      return sellerMeasureTitleRiskFallback(l);
    case 'seller.Measure.view_content_link':
      return sellerMeasureViewContentLink(l);
    case 'seller.MeasureBasis':
      return hawkr_seller_v1alpha1_seller.MeasureBasis.valueOf(_n('hawkr.seller.v1alpha1.MeasureBasis', value))!.uiText(l);
    case 'seller.MeasureKind':
      return hawkr_seller_v1alpha1_seller.MeasureKind.valueOf(_n('hawkr.seller.v1alpha1.MeasureKind', value))!.uiText(l);
    case 'seller.MeasureRequest.reply_note':
      return sellerMeasureRequestReplyNote(l, reason: params['reason']! as String);
    case 'seller.MeasureRequestKind':
      return hawkr_seller_v1alpha1_seller.MeasureRequestKind.valueOf(_n('hawkr.seller.v1alpha1.MeasureRequestKind', value))!.uiText(l);
    case 'seller.MeasureRequestKind.submitted_note':
      return hawkr_seller_v1alpha1_seller.MeasureRequestKind.valueOf(_n('hawkr.seller.v1alpha1.MeasureRequestKind', value))!.submittedNote(l);
    case 'seller.MeasureRequestState':
      return hawkr_seller_v1alpha1_seller.MeasureRequestState.valueOf(_n('hawkr.seller.v1alpha1.MeasureRequestState', value))!.uiText(l);
    case 'seller.MeasureRequestState.with_kind':
      return hawkr_seller_v1alpha1_seller.MeasureRequestState.valueOf(_n('hawkr.seller.v1alpha1.MeasureRequestState', value))!.withKind(l, kind: hawkr_seller_v1alpha1_seller.MeasureRequestKind.valueOf(_n('hawkr.seller.v1alpha1.MeasureRequestKind', params['kind']))!);
    case 'seller.MeasureState':
      return hawkr_seller_v1alpha1_seller.MeasureState.valueOf(_n('hawkr.seller.v1alpha1.MeasureState', value))!.uiText(l, kind: hawkr_seller_v1alpha1_seller.MeasureKind.valueOf(_n('hawkr.seller.v1alpha1.MeasureKind', params['kind']))!);
    case 'seller.OnboardingAction.button':
      return hawkr_seller_v1alpha1_seller.OnboardingAction.valueOf(_n('hawkr.seller.v1alpha1.OnboardingAction', value))!.button(l);
    case 'seller.OnboardingTaskId.description':
      return hawkr_seller_v1alpha1_seller.OnboardingTaskId.valueOf(_n('hawkr.seller.v1alpha1.OnboardingTaskId', value))!.description(l);
    case 'seller.OnboardingTaskId.panel_description':
      return hawkr_seller_v1alpha1_seller.OnboardingTaskId.valueOf(_n('hawkr.seller.v1alpha1.OnboardingTaskId', value))!.panelDescription(l);
    case 'seller.OnboardingTaskId.panel_title':
      return hawkr_seller_v1alpha1_seller.OnboardingTaskId.valueOf(_n('hawkr.seller.v1alpha1.OnboardingTaskId', value))!.panelTitle(l);
    case 'seller.PaymentAccount.payouts_inactive_note':
      return sellerPaymentAccountPayoutsInactiveNote(l);
    case 'seller.ReportOutcome':
      return hawkr_seller_v1alpha1_report.ReportOutcome.valueOf(_n('hawkr.seller.v1alpha1.ReportOutcome', value))!.uiText(l);
    case 'seller.RevokeReason':
      return hawkr_seller_v1alpha1_seller.RevokeReason.valueOf(_n('hawkr.seller.v1alpha1.RevokeReason', value))!.uiText(l);
    case 'seller.RiskGround':
      return hawkr_seller_v1alpha1_seller.RiskGround.valueOf(_n('hawkr.seller.v1alpha1.RiskGround', value))!.uiText(l);
    case 'seller.SellerRequirement':
      return hawkr_seller_v1alpha1_seller.SellerRequirement.valueOf(_n('hawkr.seller.v1alpha1.SellerRequirement', value))!.uiText(l);
    case 'seller.StoreField':
      return hawkr_seller_v1alpha1_seller.StoreField.valueOf(_n('hawkr.seller.v1alpha1.StoreField', value))!.uiText(l);
    case 'seller.StoreReport.outcome_unset':
      return sellerStoreReportOutcomeUnset(l);
    case 'seller.TerminationGround':
      return hawkr_seller_v1alpha1_seller.TerminationGround.valueOf(_n('hawkr.seller.v1alpha1.TerminationGround', value))!.uiText(l);
    case 'seller.ViolationSeverity':
      return hawkr_seller_v1alpha1_seller.ViolationSeverity.valueOf(_n('hawkr.seller.v1alpha1.ViolationSeverity', value))!.uiText(l);
    case 'social.CommentState.placeholder':
      return hawkr_social_v1alpha1_social.CommentState.valueOf(_n('hawkr.social.v1alpha1.CommentState', value))!.placeholder(l);
  }
  throw ArgumentError('unknown golden fn $fn');
}
