// dart format off
// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。

// ignore_for_file: unnecessary_parenthesis, unused_import

import 'package:connectrpc/connect.dart' show Code;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart' as hawkr_buyer_v1alpha1_buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart' as hawkr_common_v1alpha1_policy_category;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pbenum.dart' as hawkr_common_v1alpha1_store_card;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart' as hawkr_seller_v1alpha1_seller;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

const _enumNumber = <String, int>{
  'CONTENT_KIND_COMMENT': 2,
	'CONTENT_KIND_FORUM_ENTRY': 3,
	'CONTENT_KIND_POST': 1,
	'CONTENT_KIND_UNSPECIFIED': 0,
	'GENDER_FEMALE': 2,
	'GENDER_MALE': 1,
	'GENDER_OTHER': 3,
	'GENDER_UNSPECIFIED': 0,
	'KYC_FIELD_BANK_ACCOUNT': 9,
	'KYC_FIELD_BANK_BOOK': 17,
	'KYC_FIELD_BANK_CODE': 8,
	'KYC_FIELD_BANK_NAME': 7,
	'KYC_FIELD_BIRTH_DATE': 2,
	'KYC_FIELD_BUSINESS_COMPANY_ADDRESS': 28,
	'KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS': 27,
	'KYC_FIELD_BUSINESS_COMPANY_NAME': 20,
	'KYC_FIELD_BUSINESS_COMPANY_PHONE': 25,
	'KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE': 22,
	'KYC_FIELD_BUSINESS_REP_ID_CARD_NO': 26,
	'KYC_FIELD_BUSINESS_TAX_ID': 21,
	'KYC_FIELD_ENGLISH_NAME': 6,
	'KYC_FIELD_ID_CARD_BACK': 16,
	'KYC_FIELD_ID_CARD_FRONT': 15,
	'KYC_FIELD_ID_CARD_ISSUE_DATE': 3,
	'KYC_FIELD_ID_CARD_ISSUE_LOCATION': 5,
	'KYC_FIELD_ID_CARD_ISSUE_TYPE': 4,
	'KYC_FIELD_INDIVIDUAL_ID_CARD_NO': 18,
	'KYC_FIELD_PRODUCT_TYPE': 10,
	'KYC_FIELD_REAL_NAME': 1,
	'KYC_FIELD_UNSPECIFIED': 0,
	'KYC_FIELD_WEBSITE_URL': 12,
	'KYC_FLOW_STATUS_APPROVED': 10,
	'KYC_FLOW_STATUS_BINDING': 5,
	'KYC_FLOW_STATUS_BIND_FAILED': 6,
	'KYC_FLOW_STATUS_DRAFT': 2,
	'KYC_FLOW_STATUS_FORWARDED': 7,
	'KYC_FLOW_STATUS_NOT_STARTED': 1,
	'KYC_FLOW_STATUS_PAYUNI_REJECTED': 11,
	'KYC_FLOW_STATUS_PENDING_INFO': 9,
	'KYC_FLOW_STATUS_REJECTED': 4,
	'KYC_FLOW_STATUS_SUBMITTED': 3,
	'KYC_FLOW_STATUS_UNDER_REVIEW': 8,
	'KYC_FLOW_STATUS_UNSPECIFIED': 0,
	'MEASURE_BASIS_CONTRACT': 3,
	'MEASURE_BASIS_RISK': 2,
	'MEASURE_BASIS_UNSPECIFIED': 0,
	'MEASURE_BASIS_VIOLATION': 1,
	'MEASURE_KIND_CONTENT_TAKEDOWN': 4,
	'MEASURE_KIND_SUSPENSION': 2,
	'MEASURE_KIND_TERMINATION': 3,
	'MEASURE_KIND_UNSPECIFIED': 0,
	'MEASURE_KIND_WARNING': 1,
	'MEASURE_REQUEST_KIND_APPEAL': 1,
	'MEASURE_REQUEST_KIND_REMEDIATION': 2,
	'MEASURE_REQUEST_KIND_UNSPECIFIED': 0,
	'MEASURE_REQUEST_STATE_ACCEPTED': 2,
	'MEASURE_REQUEST_STATE_CLOSED': 4,
	'MEASURE_REQUEST_STATE_PENDING': 1,
	'MEASURE_REQUEST_STATE_REJECTED': 3,
	'MEASURE_REQUEST_STATE_UNSPECIFIED': 0,
	'MEASURE_STATE_ACTIVE': 2,
	'MEASURE_STATE_ENDED': 3,
	'MEASURE_STATE_ISSUED': 1,
	'MEASURE_STATE_REVOKED': 4,
	'MEASURE_STATE_SCHEDULED': 5,
	'MEASURE_STATE_UNSPECIFIED': 0,
	'PAY_UNI_STATE_APPROVED': 8,
	'PAY_UNI_STATE_CONTACT_SUPPORT': 5,
	'PAY_UNI_STATE_FIXABLE': 3,
	'PAY_UNI_STATE_FIX_VIA_SUPPORT': 4,
	'PAY_UNI_STATE_PENDING_INFO': 2,
	'PAY_UNI_STATE_RETRY_LATER': 6,
	'PAY_UNI_STATE_REVIEWING': 1,
	'PAY_UNI_STATE_TERMINATED': 7,
	'PAY_UNI_STATE_UNSPECIFIED': 0,
	'POLICY_CATEGORY_COUNTERFEIT': 6,
	'POLICY_CATEGORY_FAKE_CERTIFICATION': 5,
	'POLICY_CATEGORY_FOOD_SAFETY': 1,
	'POLICY_CATEGORY_FRAUD': 8,
	'POLICY_CATEGORY_HARASSMENT': 15,
	'POLICY_CATEGORY_HATE': 16,
	'POLICY_CATEGORY_IMPERSONATION': 11,
	'POLICY_CATEGORY_IP_INFRINGEMENT': 7,
	'POLICY_CATEGORY_MISLEADING_INFO': 4,
	'POLICY_CATEGORY_OFF_PLATFORM_TRADE': 9,
	'POLICY_CATEGORY_OTHER': 13,
	'POLICY_CATEGORY_PLATFORM_ABUSE': 12,
	'POLICY_CATEGORY_PRIVACY': 10,
	'POLICY_CATEGORY_PROHIBITED_GOODS': 2,
	'POLICY_CATEGORY_SPAM': 14,
	'POLICY_CATEGORY_UNLICENSED_SALE': 3,
	'POLICY_CATEGORY_UNSPECIFIED': 0,
	'REVOKE_REASON_ADMIN_CORRECTION': 2,
	'REVOKE_REASON_APPEAL_ACCEPTED': 1,
	'REVOKE_REASON_EXTERNAL_DECISION': 3,
	'REVOKE_REASON_FINDING_REVOKED': 5,
	'REVOKE_REASON_OTHER': 4,
	'REVOKE_REASON_SOURCE_REVOKED': 6,
	'REVOKE_REASON_UNSPECIFIED': 0,
	'RISK_GROUND_EXTERNAL_ORDER': 8,
	'RISK_GROUND_FOOD_SAFETY': 2,
	'RISK_GROUND_FRAUD': 3,
	'RISK_GROUND_FULFILLMENT': 7,
	'RISK_GROUND_INFRINGEMENT': 5,
	'RISK_GROUND_PERSONAL_DATA': 4,
	'RISK_GROUND_SECURITY': 6,
	'RISK_GROUND_TRANSACTION': 1,
	'RISK_GROUND_UNSPECIFIED': 0,
	'SELLER_REQUIREMENT_KYC_APPROVED': 3,
	'SELLER_REQUIREMENT_LOGO': 5,
	'SELLER_REQUIREMENT_NOT_PENDING_DELETION': 7,
	'SELLER_REQUIREMENT_NOT_SUSPENDED': 2,
	'SELLER_REQUIREMENT_STORE_ACTIVE': 1,
	'SELLER_REQUIREMENT_STORE_NAME': 4,
	'SELLER_REQUIREMENT_TOWN_CODE': 6,
	'SELLER_REQUIREMENT_UNSPECIFIED': 0,
	'STORE_CARD_STATUS_CLOSED': 3,
	'STORE_CARD_STATUS_OPEN': 1,
	'STORE_CARD_STATUS_PAUSED': 2,
	'STORE_CARD_STATUS_UNSPECIFIED': 0,
	'STORE_FIELD_ADDRESS': 1,
	'STORE_FIELD_CONTACT_MOBILE': 3,
	'STORE_FIELD_CONTACT_NAME': 2,
	'STORE_FIELD_CONTACT_PHONE': 5,
	'STORE_FIELD_SUPPORT_EMAIL': 4,
	'STORE_FIELD_UNSPECIFIED': 0,
	'TERMINATION_GROUND_BANKRUPTCY': 5,
	'TERMINATION_GROUND_CEASED_BUSINESS': 3,
	'TERMINATION_GROUND_DISSOLUTION': 4,
	'TERMINATION_GROUND_FOOD_SAFETY_INCIDENT': 7,
	'TERMINATION_GROUND_FRAUD': 8,
	'TERMINATION_GROUND_LICENSE_REVOKED': 6,
	'TERMINATION_GROUND_LINKED_ACCOUNT': 11,
	'TERMINATION_GROUND_MATERIAL_BREACH': 2,
	'TERMINATION_GROUND_NOTICE': 1,
	'TERMINATION_GROUND_OTHER_SERIOUS': 10,
	'TERMINATION_GROUND_PLATFORM_SECURITY': 9,
	'TERMINATION_GROUND_UNSPECIFIED': 0,
	'VIOLATION_SEVERITY_CRITICAL': 4,
	'VIOLATION_SEVERITY_HIGH': 3,
	'VIOLATION_SEVERITY_LOW': 1,
	'VIOLATION_SEVERITY_MEDIUM': 2,
	'VIOLATION_SEVERITY_UNSPECIFIED': 0,
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

int _n(Object? name) => _enumNumber[name]!;

/// 呼叫 fn 對應的生成函式；null 表示「沒有文字」。
String? goldenText(String fn, String value, Map<String, Object?> params, UiLocale l) {
  switch (fn) {
    case 'buyer.Gender':
      return hawkr_buyer_v1alpha1_buyer.Gender.valueOf(_n(value))!.uiText(l);
    case 'buyer.Profile.gender_unset':
      return buyerProfileGenderUnset(l);
    case 'common.PolicyCategory':
      return hawkr_common_v1alpha1_policy_category.PolicyCategory.valueOf(_n(value))!.uiText(l);
    case 'common.StoreCard.closed_store':
      return commonStoreCardClosedStore(l);
    case 'common.StoreCardStatus':
      return hawkr_common_v1alpha1_store_card.StoreCardStatus.valueOf(_n(value))!.uiText(l);
    case 'common.error':
      return commonErrorText(_codes[value]!, l);
    case 'common.unknown':
      return commonUnknown(l);
    case 'seller.ContentKind':
      return hawkr_seller_v1alpha1_seller.ContentKind.valueOf(_n(value))!.uiText(l);
    case 'seller.KYCField':
      return hawkr_seller_v1alpha1_seller.KYCField.valueOf(_n(value))!.uiText(l);
    case 'seller.KYCFlowStatus':
      return hawkr_seller_v1alpha1_seller.KYCFlowStatus.valueOf(_n(value))!.uiText(l);
    case 'seller.KYCFlowStatus.action':
      return hawkr_seller_v1alpha1_seller.KYCFlowStatus.valueOf(_n(value))!.action(l);
    case 'seller.KYCFlowStatus.banner_description':
      return hawkr_seller_v1alpha1_seller.KYCFlowStatus.valueOf(_n(value))!.bannerDescription(l);
    case 'seller.KYCFlowStatus.banner_title':
      return hawkr_seller_v1alpha1_seller.KYCFlowStatus.valueOf(_n(value))!.bannerTitle(l);
    case 'seller.KYCFlowStatus.gate_heading':
      return hawkr_seller_v1alpha1_seller.KYCFlowStatus.valueOf(_n(value))!.gateHeading(l);
    case 'seller.KYCFlowStatus.gate_message':
      return hawkr_seller_v1alpha1_seller.KYCFlowStatus.valueOf(_n(value))!.gateMessage(l, subject: params['subject']! as String);
    case 'seller.Measure.appeal_deadline':
      return sellerMeasureAppealDeadline(l, deadline: params['deadline']! as String);
    case 'seller.Measure.meta_kind':
      return sellerMeasureMetaKind(l, kind: hawkr_seller_v1alpha1_seller.MeasureKind.valueOf(_n(params['kind']))!);
    case 'seller.Measure.meta_severity':
      return sellerMeasureMetaSeverity(l, severity: hawkr_seller_v1alpha1_seller.ViolationSeverity.valueOf(_n(params['severity']))!);
    case 'seller.Measure.meta_termination_ground':
      return sellerMeasureMetaTerminationGround(l, ground: hawkr_seller_v1alpha1_seller.TerminationGround.valueOf(_n(params['ground']))!);
    case 'seller.Measure.rejection_cooldown_warning':
      return sellerMeasureRejectionCooldownWarning(l, days: (params['days']! as num).toInt());
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
      return sellerMeasureTitleRisk(l, ground: hawkr_seller_v1alpha1_seller.RiskGround.valueOf(_n(params['ground']))!);
    case 'seller.Measure.title_risk_fallback':
      return sellerMeasureTitleRiskFallback(l);
    case 'seller.MeasureBasis':
      return hawkr_seller_v1alpha1_seller.MeasureBasis.valueOf(_n(value))!.uiText(l);
    case 'seller.MeasureKind':
      return hawkr_seller_v1alpha1_seller.MeasureKind.valueOf(_n(value))!.uiText(l);
    case 'seller.MeasureRequest.reply_note':
      return sellerMeasureRequestReplyNote(l, reason: params['reason']! as String);
    case 'seller.MeasureRequestKind':
      return hawkr_seller_v1alpha1_seller.MeasureRequestKind.valueOf(_n(value))!.uiText(l);
    case 'seller.MeasureRequestKind.submitted_note':
      return hawkr_seller_v1alpha1_seller.MeasureRequestKind.valueOf(_n(value))!.submittedNote(l);
    case 'seller.MeasureRequestState':
      return hawkr_seller_v1alpha1_seller.MeasureRequestState.valueOf(_n(value))!.uiText(l);
    case 'seller.MeasureRequestState.with_kind':
      return hawkr_seller_v1alpha1_seller.MeasureRequestState.valueOf(_n(value))!.withKind(l, kind: hawkr_seller_v1alpha1_seller.MeasureRequestKind.valueOf(_n(params['kind']))!);
    case 'seller.MeasureState':
      return hawkr_seller_v1alpha1_seller.MeasureState.valueOf(_n(value))!.uiText(l, kind: hawkr_seller_v1alpha1_seller.MeasureKind.valueOf(_n(params['kind']))!);
    case 'seller.PayUniState':
      return hawkr_seller_v1alpha1_seller.PayUniState.valueOf(_n(value))!.uiText(l);
    case 'seller.RevokeReason':
      return hawkr_seller_v1alpha1_seller.RevokeReason.valueOf(_n(value))!.uiText(l);
    case 'seller.RiskGround':
      return hawkr_seller_v1alpha1_seller.RiskGround.valueOf(_n(value))!.uiText(l);
    case 'seller.SellerPayUni.fix_fields_heading':
      return sellerSellerPayUniFixFieldsHeading(l);
    case 'seller.SellerPayUni.reason_heading':
      return sellerSellerPayUniReasonHeading(l);
    case 'seller.SellerRequirement':
      return hawkr_seller_v1alpha1_seller.SellerRequirement.valueOf(_n(value))!.uiText(l);
    case 'seller.StoreField':
      return hawkr_seller_v1alpha1_seller.StoreField.valueOf(_n(value))!.uiText(l);
    case 'seller.TerminationGround':
      return hawkr_seller_v1alpha1_seller.TerminationGround.valueOf(_n(value))!.uiText(l);
    case 'seller.ViolationSeverity':
      return hawkr_seller_v1alpha1_seller.ViolationSeverity.valueOf(_n(value))!.uiText(l);
  }
  throw ArgumentError('unknown golden fn $fn');
}
