// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/seller.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

import 'package:protobuf/well_known_types/google/protobuf/duration.pbjson.dart'
    as $6;
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pbjson.dart'
    as $4;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $1;

import '../../../google/rpc/error_details.pbjson.dart' as $3;
import '../../../google/type/date.pbjson.dart' as $5;
import '../../common/v1alpha1/image.pbjson.dart' as $0;
import '../../common/v1alpha1/postal_address.pbjson.dart' as $2;
import '../../common/v1alpha1/store_card.pbjson.dart' as $7;

@$core.Deprecated('Use agreementKindDescriptor instead')
const AgreementKind$json = {
  '1': 'AgreementKind',
  '2': [
    {'1': 'AGREEMENT_KIND_UNSPECIFIED', '2': 0},
    {'1': 'AGREEMENT_KIND_SELLER_TERMS', '2': 1},
    {'1': 'AGREEMENT_KIND_KYC_PRIVACY_NOTICE', '2': 2},
  ],
  '4': [
    {'1': 3, '2': 3},
    {'1': 4, '2': 4},
  ],
  '5': [
    'AGREEMENT_KIND_PAYUNI_TERMS',
    'AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE'
  ],
};

/// Descriptor for `AgreementKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List agreementKindDescriptor = $convert.base64Decode(
    'Cg1BZ3JlZW1lbnRLaW5kEh4KGkFHUkVFTUVOVF9LSU5EX1VOU1BFQ0lGSUVEEAASHwobQUdSRU'
    'VNRU5UX0tJTkRfU0VMTEVSX1RFUk1TEAESJQohQUdSRUVNRU5UX0tJTkRfS1lDX1BSSVZBQ1lf'
    'Tk9USUNFEAIiBAgDEAMiBAgEEAQqG0FHUkVFTUVOVF9LSU5EX1BBWVVOSV9URVJNUyonQUdSRU'
    'VNRU5UX0tJTkRfRk9PRF9SRUdJU1RSQVRJT05fTk9USUNF');

@$core.Deprecated('Use storeStatusDescriptor instead')
const StoreStatus$json = {
  '1': 'StoreStatus',
  '2': [
    {'1': 'STORE_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'STORE_STATUS_ACTIVE', '2': 1},
    {'1': 'STORE_STATUS_PENDING_DELETION', '2': 2},
    {'1': 'STORE_STATUS_CLOSED', '2': 3},
  ],
};

/// Descriptor for `StoreStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List storeStatusDescriptor = $convert.base64Decode(
    'CgtTdG9yZVN0YXR1cxIcChhTVE9SRV9TVEFUVVNfVU5TUEVDSUZJRUQQABIXChNTVE9SRV9TVE'
    'FUVVNfQUNUSVZFEAESIQodU1RPUkVfU1RBVFVTX1BFTkRJTkdfREVMRVRJT04QAhIXChNTVE9S'
    'RV9TVEFUVVNfQ0xPU0VEEAM=');

@$core.Deprecated('Use sellerCapabilityDescriptor instead')
const SellerCapability$json = {
  '1': 'SellerCapability',
  '2': [
    {'1': 'SELLER_CAPABILITY_UNSPECIFIED', '2': 0},
    {'1': 'SELLER_CAPABILITY_ACCOUNT_SELF_READ', '2': 1},
    {'1': 'SELLER_CAPABILITY_STOREFRONT_WRITE', '2': 2},
    {'1': 'SELLER_CAPABILITY_PRODUCT_MANAGE', '2': 3},
    {'1': 'SELLER_CAPABILITY_ORDER_FULFILL', '2': 4},
    {'1': 'SELLER_CAPABILITY_ORDER_CONTRACT', '2': 5},
    {'1': 'SELLER_CAPABILITY_MARKETING', '2': 6},
    {'1': 'SELLER_CAPABILITY_CONTENT_CONTRACT', '2': 7},
    {'1': 'SELLER_CAPABILITY_CHAT_SEND', '2': 9},
    {'1': 'SELLER_CAPABILITY_CHAT_READ', '2': 10},
    {'1': 'SELLER_CAPABILITY_NOTIFICATION', '2': 11},
    {'1': 'SELLER_CAPABILITY_ACCOUNT_CLOSE', '2': 12},
    {'1': 'SELLER_CAPABILITY_INVOICE_MANAGE', '2': 13},
    {'1': 'SELLER_CAPABILITY_INVOICE_TAX_REMEDY', '2': 14},
    {'1': 'SELLER_CAPABILITY_SHIPPING_SETUP', '2': 15},
    {'1': 'SELLER_CAPABILITY_PUBLIC_LISTING', '2': 16},
  ],
  '4': [
    {'1': 8, '2': 8},
  ],
  '5': ['SELLER_CAPABILITY_GOVERNANCE_APPEAL'],
};

/// Descriptor for `SellerCapability`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sellerCapabilityDescriptor = $convert.base64Decode(
    'ChBTZWxsZXJDYXBhYmlsaXR5EiEKHVNFTExFUl9DQVBBQklMSVRZX1VOU1BFQ0lGSUVEEAASJw'
    'ojU0VMTEVSX0NBUEFCSUxJVFlfQUNDT1VOVF9TRUxGX1JFQUQQARImCiJTRUxMRVJfQ0FQQUJJ'
    'TElUWV9TVE9SRUZST05UX1dSSVRFEAISJAogU0VMTEVSX0NBUEFCSUxJVFlfUFJPRFVDVF9NQU'
    '5BR0UQAxIjCh9TRUxMRVJfQ0FQQUJJTElUWV9PUkRFUl9GVUxGSUxMEAQSJAogU0VMTEVSX0NB'
    'UEFCSUxJVFlfT1JERVJfQ09OVFJBQ1QQBRIfChtTRUxMRVJfQ0FQQUJJTElUWV9NQVJLRVRJTk'
    'cQBhImCiJTRUxMRVJfQ0FQQUJJTElUWV9DT05URU5UX0NPTlRSQUNUEAcSHwobU0VMTEVSX0NB'
    'UEFCSUxJVFlfQ0hBVF9TRU5EEAkSHwobU0VMTEVSX0NBUEFCSUxJVFlfQ0hBVF9SRUFEEAoSIg'
    'oeU0VMTEVSX0NBUEFCSUxJVFlfTk9USUZJQ0FUSU9OEAsSIwofU0VMTEVSX0NBUEFCSUxJVFlf'
    'QUNDT1VOVF9DTE9TRRAMEiQKIFNFTExFUl9DQVBBQklMSVRZX0lOVk9JQ0VfTUFOQUdFEA0SKA'
    'okU0VMTEVSX0NBUEFCSUxJVFlfSU5WT0lDRV9UQVhfUkVNRURZEA4SJAogU0VMTEVSX0NBUEFC'
    'SUxJVFlfU0hJUFBJTkdfU0VUVVAQDxIkCiBTRUxMRVJfQ0FQQUJJTElUWV9QVUJMSUNfTElTVE'
    'lORxAQIgQICBAIKiNTRUxMRVJfQ0FQQUJJTElUWV9HT1ZFUk5BTkNFX0FQUEVBTA==');

@$core.Deprecated('Use sellerRequirementDescriptor instead')
const SellerRequirement$json = {
  '1': 'SellerRequirement',
  '2': [
    {'1': 'SELLER_REQUIREMENT_UNSPECIFIED', '2': 0},
    {'1': 'SELLER_REQUIREMENT_STORE_ACTIVE', '2': 1},
    {'1': 'SELLER_REQUIREMENT_NOT_SUSPENDED', '2': 2},
    {'1': 'SELLER_REQUIREMENT_ONBOARDING_COMPLETE', '2': 3},
    {'1': 'SELLER_REQUIREMENT_STORE_NAME', '2': 4},
    {'1': 'SELLER_REQUIREMENT_LOGO', '2': 5},
    {'1': 'SELLER_REQUIREMENT_TOWN_CODE', '2': 6},
    {'1': 'SELLER_REQUIREMENT_NOT_PENDING_DELETION', '2': 7},
  ],
};

/// Descriptor for `SellerRequirement`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sellerRequirementDescriptor = $convert.base64Decode(
    'ChFTZWxsZXJSZXF1aXJlbWVudBIiCh5TRUxMRVJfUkVRVUlSRU1FTlRfVU5TUEVDSUZJRUQQAB'
    'IjCh9TRUxMRVJfUkVRVUlSRU1FTlRfU1RPUkVfQUNUSVZFEAESJAogU0VMTEVSX1JFUVVJUkVN'
    'RU5UX05PVF9TVVNQRU5ERUQQAhIqCiZTRUxMRVJfUkVRVUlSRU1FTlRfT05CT0FSRElOR19DT0'
    '1QTEVURRADEiEKHVNFTExFUl9SRVFVSVJFTUVOVF9TVE9SRV9OQU1FEAQSGwoXU0VMTEVSX1JF'
    'UVVJUkVNRU5UX0xPR08QBRIgChxTRUxMRVJfUkVRVUlSRU1FTlRfVE9XTl9DT0RFEAYSKwonU0'
    'VMTEVSX1JFUVVJUkVNRU5UX05PVF9QRU5ESU5HX0RFTEVUSU9OEAc=');

@$core.Deprecated('Use storeFieldDescriptor instead')
const StoreField$json = {
  '1': 'StoreField',
  '2': [
    {'1': 'STORE_FIELD_UNSPECIFIED', '2': 0},
    {'1': 'STORE_FIELD_ADDRESS', '2': 1},
    {'1': 'STORE_FIELD_CONTACT_NAME', '2': 2},
    {'1': 'STORE_FIELD_CONTACT_MOBILE', '2': 3},
    {'1': 'STORE_FIELD_SUPPORT_EMAIL', '2': 4},
    {'1': 'STORE_FIELD_CONTACT_PHONE', '2': 5},
  ],
};

/// Descriptor for `StoreField`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List storeFieldDescriptor = $convert.base64Decode(
    'CgpTdG9yZUZpZWxkEhsKF1NUT1JFX0ZJRUxEX1VOU1BFQ0lGSUVEEAASFwoTU1RPUkVfRklFTE'
    'RfQUREUkVTUxABEhwKGFNUT1JFX0ZJRUxEX0NPTlRBQ1RfTkFNRRACEh4KGlNUT1JFX0ZJRUxE'
    'X0NPTlRBQ1RfTU9CSUxFEAMSHQoZU1RPUkVfRklFTERfU1VQUE9SVF9FTUFJTBAEEh0KGVNUT1'
    'JFX0ZJRUxEX0NPTlRBQ1RfUEhPTkUQBQ==');

@$core.Deprecated('Use storeOpeningUnavailableReasonDescriptor instead')
const StoreOpeningUnavailableReason$json = {
  '1': 'StoreOpeningUnavailableReason',
  '2': [
    {'1': 'STORE_OPENING_UNAVAILABLE_REASON_UNSPECIFIED', '2': 0},
    {'1': 'STORE_OPENING_UNAVAILABLE_REASON_TERMINATED', '2': 1},
    {'1': 'STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED', '2': 2},
    {'1': 'STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING', '2': 3},
    {'1': 'STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS', '2': 4},
  ],
};

/// Descriptor for `StoreOpeningUnavailableReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List storeOpeningUnavailableReasonDescriptor = $convert.base64Decode(
    'Ch1TdG9yZU9wZW5pbmdVbmF2YWlsYWJsZVJlYXNvbhIwCixTVE9SRV9PUEVOSU5HX1VOQVZBSU'
    'xBQkxFX1JFQVNPTl9VTlNQRUNJRklFRBAAEi8KK1NUT1JFX09QRU5JTkdfVU5BVkFJTEFCTEVf'
    'UkVBU09OX1RFUk1JTkFURUQQARIuCipTVE9SRV9PUEVOSU5HX1VOQVZBSUxBQkxFX1JFQVNPTl'
    '9TVVNQRU5ERUQQAhI1CjFTVE9SRV9PUEVOSU5HX1VOQVZBSUxBQkxFX1JFQVNPTl9BQ0NPVU5U'
    'X0RFTEVUSU5HEAMSMQotU1RPUkVfT1BFTklOR19VTkFWQUlMQUJMRV9SRUFTT05fU1RPUkVfRV'
    'hJU1RTEAQ=');

@$core.Deprecated('Use onboardingTaskIdDescriptor instead')
const OnboardingTaskId$json = {
  '1': 'OnboardingTaskId',
  '2': [
    {'1': 'ONBOARDING_TASK_ID_UNSPECIFIED', '2': 0},
    {'1': 'ONBOARDING_TASK_ID_PROFILE', '2': 1},
    {'1': 'ONBOARDING_TASK_ID_PLATFORM_REVIEW', '2': 2},
    {'1': 'ONBOARDING_TASK_ID_PAYMENT_ACCOUNT', '2': 3},
  ],
};

/// Descriptor for `OnboardingTaskId`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List onboardingTaskIdDescriptor = $convert.base64Decode(
    'ChBPbmJvYXJkaW5nVGFza0lkEiIKHk9OQk9BUkRJTkdfVEFTS19JRF9VTlNQRUNJRklFRBAAEh'
    '4KGk9OQk9BUkRJTkdfVEFTS19JRF9QUk9GSUxFEAESJgoiT05CT0FSRElOR19UQVNLX0lEX1BM'
    'QVRGT1JNX1JFVklFVxACEiYKIk9OQk9BUkRJTkdfVEFTS19JRF9QQVlNRU5UX0FDQ09VTlQQAw'
    '==');

@$core.Deprecated('Use onboardingTaskStateDescriptor instead')
const OnboardingTaskState$json = {
  '1': 'OnboardingTaskState',
  '2': [
    {'1': 'ONBOARDING_TASK_STATE_UNSPECIFIED', '2': 0},
    {'1': 'ONBOARDING_TASK_STATE_COMPLETED', '2': 1},
    {'1': 'ONBOARDING_TASK_STATE_ACTION_REQUIRED', '2': 2},
    {'1': 'ONBOARDING_TASK_STATE_IN_PROGRESS', '2': 3},
    {'1': 'ONBOARDING_TASK_STATE_LOCKED', '2': 4},
    {'1': 'ONBOARDING_TASK_STATE_UNKNOWN', '2': 5},
  ],
};

/// Descriptor for `OnboardingTaskState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List onboardingTaskStateDescriptor = $convert.base64Decode(
    'ChNPbmJvYXJkaW5nVGFza1N0YXRlEiUKIU9OQk9BUkRJTkdfVEFTS19TVEFURV9VTlNQRUNJRk'
    'lFRBAAEiMKH09OQk9BUkRJTkdfVEFTS19TVEFURV9DT01QTEVURUQQARIpCiVPTkJPQVJESU5H'
    'X1RBU0tfU1RBVEVfQUNUSU9OX1JFUVVJUkVEEAISJQohT05CT0FSRElOR19UQVNLX1NUQVRFX0'
    'lOX1BST0dSRVNTEAMSIAocT05CT0FSRElOR19UQVNLX1NUQVRFX0xPQ0tFRBAEEiEKHU9OQk9B'
    'UkRJTkdfVEFTS19TVEFURV9VTktOT1dOEAU=');

@$core.Deprecated('Use onboardingActionDescriptor instead')
const OnboardingAction$json = {
  '1': 'OnboardingAction',
  '2': [
    {'1': 'ONBOARDING_ACTION_UNSPECIFIED', '2': 0},
    {'1': 'ONBOARDING_ACTION_NONE', '2': 1},
    {'1': 'ONBOARDING_ACTION_EDIT_PROFILE', '2': 2},
    {'1': 'ONBOARDING_ACTION_SUBMIT_CERTIFICATION', '2': 3},
    {'1': 'ONBOARDING_ACTION_START_PAYMENT_ONBOARDING', '2': 4},
    {'1': 'ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT', '2': 5},
    {'1': 'ONBOARDING_ACTION_CONTACT_SUPPORT', '2': 6},
  ],
};

/// Descriptor for `OnboardingAction`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List onboardingActionDescriptor = $convert.base64Decode(
    'ChBPbmJvYXJkaW5nQWN0aW9uEiEKHU9OQk9BUkRJTkdfQUNUSU9OX1VOU1BFQ0lGSUVEEAASGg'
    'oWT05CT0FSRElOR19BQ1RJT05fTk9ORRABEiIKHk9OQk9BUkRJTkdfQUNUSU9OX0VESVRfUFJP'
    'RklMRRACEioKJk9OQk9BUkRJTkdfQUNUSU9OX1NVQk1JVF9DRVJUSUZJQ0FUSU9OEAMSLgoqT0'
    '5CT0FSRElOR19BQ1RJT05fU1RBUlRfUEFZTUVOVF9PTkJPQVJESU5HEAQSLQopT05CT0FSRElO'
    'R19BQ1RJT05fUkVGUkVTSF9QQVlNRU5UX0FDQ09VTlQQBRIlCiFPTkJPQVJESU5HX0FDVElPTl'
    '9DT05UQUNUX1NVUFBPUlQQBg==');

@$core.Deprecated('Use kYCStatusDescriptor instead')
const KYCStatus$json = {
  '1': 'KYCStatus',
  '2': [
    {'1': 'KYC_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'KYC_STATUS_DRAFT', '2': 1},
    {'1': 'KYC_STATUS_SUBMITTED', '2': 2},
    {'1': 'KYC_STATUS_APPROVED', '2': 3},
    {'1': 'KYC_STATUS_REJECTED', '2': 4},
    {'1': 'KYC_STATUS_CANCELLED', '2': 5},
  ],
};

/// Descriptor for `KYCStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List kYCStatusDescriptor = $convert.base64Decode(
    'CglLWUNTdGF0dXMSGgoWS1lDX1NUQVRVU19VTlNQRUNJRklFRBAAEhQKEEtZQ19TVEFUVVNfRF'
    'JBRlQQARIYChRLWUNfU1RBVFVTX1NVQk1JVFRFRBACEhcKE0tZQ19TVEFUVVNfQVBQUk9WRUQQ'
    'AxIXChNLWUNfU1RBVFVTX1JFSkVDVEVEEAQSGAoUS1lDX1NUQVRVU19DQU5DRUxMRUQQBQ==');

@$core.Deprecated('Use kYCFieldDescriptor instead')
const KYCField$json = {
  '1': 'KYCField',
  '2': [
    {'1': 'KYC_FIELD_UNSPECIFIED', '2': 0},
    {'1': 'KYC_FIELD_PRODUCT_TYPE', '2': 10},
    {'1': 'KYC_FIELD_WEBSITE_URL', '2': 12},
    {'1': 'KYC_FIELD_BUSINESS_COMPANY_NAME', '2': 20},
    {'1': 'KYC_FIELD_BUSINESS_TAX_ID', '2': 21},
    {'1': 'KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE', '2': 22},
    {'1': 'KYC_FIELD_BUSINESS_COMPANY_PHONE', '2': 25},
    {'1': 'KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS', '2': 27},
    {'1': 'KYC_FIELD_BUSINESS_COMPANY_ADDRESS', '2': 28},
  ],
  '4': [
    {'1': 1, '2': 9},
    {'1': 11, '2': 11},
    {'1': 13, '2': 13},
    {'1': 14, '2': 14},
    {'1': 15, '2': 18},
    {'1': 19, '2': 19},
    {'1': 23, '2': 23},
    {'1': 24, '2': 24},
    {'1': 26, '2': 26},
    {'1': 29, '2': 29},
  ],
  '5': [
    'KYC_FIELD_REAL_NAME',
    'KYC_FIELD_BIRTH_DATE',
    'KYC_FIELD_ID_CARD_ISSUE_DATE',
    'KYC_FIELD_ID_CARD_ISSUE_TYPE',
    'KYC_FIELD_ID_CARD_ISSUE_LOCATION',
    'KYC_FIELD_ENGLISH_NAME',
    'KYC_FIELD_BANK_NAME',
    'KYC_FIELD_BANK_CODE',
    'KYC_FIELD_BANK_ACCOUNT',
    'KYC_FIELD_FOOD_BUSINESS',
    'KYC_FIELD_CONTACT_EMAIL',
    'KYC_FIELD_CONTACT_PHONE',
    'KYC_FIELD_ID_CARD_FRONT',
    'KYC_FIELD_ID_CARD_BACK',
    'KYC_FIELD_BANK_BOOK',
    'KYC_FIELD_INDIVIDUAL_ID_CARD_NO',
    'KYC_FIELD_BUSINESS_COMPANY_TYPE',
    'KYC_FIELD_BUSINESS_COMPANY_TOWN_CODE',
    'KYC_FIELD_BUSINESS_COMPANY_ADDRESS_DETAIL',
    'KYC_FIELD_BUSINESS_REP_ID_CARD_NO',
    'KYC_FIELD_PAYOUT_CONTACT_EMAIL'
  ],
};

/// Descriptor for `KYCField`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List kYCFieldDescriptor = $convert.base64Decode(
    'CghLWUNGaWVsZBIZChVLWUNfRklFTERfVU5TUEVDSUZJRUQQABIaChZLWUNfRklFTERfUFJPRF'
    'VDVF9UWVBFEAoSGQoVS1lDX0ZJRUxEX1dFQlNJVEVfVVJMEAwSIwofS1lDX0ZJRUxEX0JVU0lO'
    'RVNTX0NPTVBBTllfTkFNRRAUEh0KGUtZQ19GSUVMRF9CVVNJTkVTU19UQVhfSUQQFRIsCihLWU'
    'NfRklFTERfQlVTSU5FU1NfQ09NUEFOWV9SRUdJU1RFUl9EQVRFEBYSJAogS1lDX0ZJRUxEX0JV'
    'U0lORVNTX0NPTVBBTllfUEhPTkUQGRIoCiRLWUNfRklFTERfQlVTSU5FU1NfQ09NUEFOWV9ET0'
    'NVTUVOVFMQGxImCiJLWUNfRklFTERfQlVTSU5FU1NfQ09NUEFOWV9BRERSRVNTEBwiBAgBEAki'
    'BAgLEAsiBAgNEA0iBAgOEA4iBAgPEBIiBAgTEBMiBAgXEBciBAgYEBgiBAgaEBoiBAgdEB0qE0'
    'tZQ19GSUVMRF9SRUFMX05BTUUqFEtZQ19GSUVMRF9CSVJUSF9EQVRFKhxLWUNfRklFTERfSURf'
    'Q0FSRF9JU1NVRV9EQVRFKhxLWUNfRklFTERfSURfQ0FSRF9JU1NVRV9UWVBFKiBLWUNfRklFTE'
    'RfSURfQ0FSRF9JU1NVRV9MT0NBVElPTioWS1lDX0ZJRUxEX0VOR0xJU0hfTkFNRSoTS1lDX0ZJ'
    'RUxEX0JBTktfTkFNRSoTS1lDX0ZJRUxEX0JBTktfQ09ERSoWS1lDX0ZJRUxEX0JBTktfQUNDT1'
    'VOVCoXS1lDX0ZJRUxEX0ZPT0RfQlVTSU5FU1MqF0tZQ19GSUVMRF9DT05UQUNUX0VNQUlMKhdL'
    'WUNfRklFTERfQ09OVEFDVF9QSE9ORSoXS1lDX0ZJRUxEX0lEX0NBUkRfRlJPTlQqFktZQ19GSU'
    'VMRF9JRF9DQVJEX0JBQ0sqE0tZQ19GSUVMRF9CQU5LX0JPT0sqH0tZQ19GSUVMRF9JTkRJVklE'
    'VUFMX0lEX0NBUkRfTk8qH0tZQ19GSUVMRF9CVVNJTkVTU19DT01QQU5ZX1RZUEUqJEtZQ19GSU'
    'VMRF9CVVNJTkVTU19DT01QQU5ZX1RPV05fQ09ERSopS1lDX0ZJRUxEX0JVU0lORVNTX0NPTVBB'
    'TllfQUREUkVTU19ERVRBSUwqIUtZQ19GSUVMRF9CVVNJTkVTU19SRVBfSURfQ0FSRF9OTyoeS1'
    'lDX0ZJRUxEX1BBWU9VVF9DT05UQUNUX0VNQUlM');

@$core.Deprecated('Use kYCIssueCodeDescriptor instead')
const KYCIssueCode$json = {
  '1': 'KYCIssueCode',
  '2': [
    {'1': 'KYC_ISSUE_CODE_UNSPECIFIED', '2': 0},
    {'1': 'KYC_ISSUE_CODE_UNREADABLE', '2': 1},
    {'1': 'KYC_ISSUE_CODE_MISMATCH', '2': 2},
    {'1': 'KYC_ISSUE_CODE_EXPIRED', '2': 3},
    {'1': 'KYC_ISSUE_CODE_MISSING', '2': 4},
    {'1': 'KYC_ISSUE_CODE_INVALID', '2': 5},
    {'1': 'KYC_ISSUE_CODE_OTHER', '2': 6},
  ],
};

/// Descriptor for `KYCIssueCode`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List kYCIssueCodeDescriptor = $convert.base64Decode(
    'CgxLWUNJc3N1ZUNvZGUSHgoaS1lDX0lTU1VFX0NPREVfVU5TUEVDSUZJRUQQABIdChlLWUNfSV'
    'NTVUVfQ09ERV9VTlJFQURBQkxFEAESGwoXS1lDX0lTU1VFX0NPREVfTUlTTUFUQ0gQAhIaChZL'
    'WUNfSVNTVUVfQ09ERV9FWFBJUkVEEAMSGgoWS1lDX0lTU1VFX0NPREVfTUlTU0lORxAEEhoKFk'
    'tZQ19JU1NVRV9DT0RFX0lOVkFMSUQQBRIYChRLWUNfSVNTVUVfQ09ERV9PVEhFUhAG');

@$core.Deprecated('Use onboardingReturnDestinationDescriptor instead')
const OnboardingReturnDestination$json = {
  '1': 'OnboardingReturnDestination',
  '2': [
    {'1': 'ONBOARDING_RETURN_DESTINATION_UNSPECIFIED', '2': 0},
    {'1': 'ONBOARDING_RETURN_DESTINATION_WEBSITE_CERTIFICATION', '2': 1},
    {'1': 'ONBOARDING_RETURN_DESTINATION_SELLER_APP', '2': 2},
  ],
};

/// Descriptor for `OnboardingReturnDestination`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List onboardingReturnDestinationDescriptor = $convert.base64Decode(
    'ChtPbmJvYXJkaW5nUmV0dXJuRGVzdGluYXRpb24SLQopT05CT0FSRElOR19SRVRVUk5fREVTVE'
    'lOQVRJT05fVU5TUEVDSUZJRUQQABI3CjNPTkJPQVJESU5HX1JFVFVSTl9ERVNUSU5BVElPTl9X'
    'RUJTSVRFX0NFUlRJRklDQVRJT04QARIsCihPTkJPQVJESU5HX1JFVFVSTl9ERVNUSU5BVElPTl'
    '9TRUxMRVJfQVBQEAI=');

@$core.Deprecated('Use violationSeverityDescriptor instead')
const ViolationSeverity$json = {
  '1': 'ViolationSeverity',
  '2': [
    {'1': 'VIOLATION_SEVERITY_UNSPECIFIED', '2': 0},
    {'1': 'VIOLATION_SEVERITY_LOW', '2': 1},
    {'1': 'VIOLATION_SEVERITY_MEDIUM', '2': 2},
    {'1': 'VIOLATION_SEVERITY_HIGH', '2': 3},
    {'1': 'VIOLATION_SEVERITY_CRITICAL', '2': 4},
  ],
};

/// Descriptor for `ViolationSeverity`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List violationSeverityDescriptor = $convert.base64Decode(
    'ChFWaW9sYXRpb25TZXZlcml0eRIiCh5WSU9MQVRJT05fU0VWRVJJVFlfVU5TUEVDSUZJRUQQAB'
    'IaChZWSU9MQVRJT05fU0VWRVJJVFlfTE9XEAESHQoZVklPTEFUSU9OX1NFVkVSSVRZX01FRElV'
    'TRACEhsKF1ZJT0xBVElPTl9TRVZFUklUWV9ISUdIEAMSHwobVklPTEFUSU9OX1NFVkVSSVRZX0'
    'NSSVRJQ0FMEAQ=');

@$core.Deprecated('Use revokeReasonDescriptor instead')
const RevokeReason$json = {
  '1': 'RevokeReason',
  '2': [
    {'1': 'REVOKE_REASON_UNSPECIFIED', '2': 0},
    {'1': 'REVOKE_REASON_APPEAL_ACCEPTED', '2': 1},
    {'1': 'REVOKE_REASON_ADMIN_CORRECTION', '2': 2},
    {'1': 'REVOKE_REASON_EXTERNAL_DECISION', '2': 3},
    {'1': 'REVOKE_REASON_OTHER', '2': 4},
    {'1': 'REVOKE_REASON_FINDING_REVOKED', '2': 5},
    {'1': 'REVOKE_REASON_SOURCE_REVOKED', '2': 6},
  ],
};

/// Descriptor for `RevokeReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List revokeReasonDescriptor = $convert.base64Decode(
    'CgxSZXZva2VSZWFzb24SHQoZUkVWT0tFX1JFQVNPTl9VTlNQRUNJRklFRBAAEiEKHVJFVk9LRV'
    '9SRUFTT05fQVBQRUFMX0FDQ0VQVEVEEAESIgoeUkVWT0tFX1JFQVNPTl9BRE1JTl9DT1JSRUNU'
    'SU9OEAISIwofUkVWT0tFX1JFQVNPTl9FWFRFUk5BTF9ERUNJU0lPThADEhcKE1JFVk9LRV9SRU'
    'FTT05fT1RIRVIQBBIhCh1SRVZPS0VfUkVBU09OX0ZJTkRJTkdfUkVWT0tFRBAFEiAKHFJFVk9L'
    'RV9SRUFTT05fU09VUkNFX1JFVk9LRUQQBg==');

@$core.Deprecated('Use violationStatusDescriptor instead')
const ViolationStatus$json = {
  '1': 'ViolationStatus',
  '2': [
    {'1': 'VIOLATION_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'VIOLATION_STATUS_ENFORCED', '2': 1},
    {'1': 'VIOLATION_STATUS_REVOKED', '2': 2},
  ],
};

/// Descriptor for `ViolationStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List violationStatusDescriptor = $convert.base64Decode(
    'Cg9WaW9sYXRpb25TdGF0dXMSIAocVklPTEFUSU9OX1NUQVRVU19VTlNQRUNJRklFRBAAEh0KGV'
    'ZJT0xBVElPTl9TVEFUVVNfRU5GT1JDRUQQARIcChhWSU9MQVRJT05fU1RBVFVTX1JFVk9LRUQQ'
    'Ag==');

@$core.Deprecated('Use measureKindDescriptor instead')
const MeasureKind$json = {
  '1': 'MeasureKind',
  '2': [
    {'1': 'MEASURE_KIND_UNSPECIFIED', '2': 0},
    {'1': 'MEASURE_KIND_WARNING', '2': 1},
    {'1': 'MEASURE_KIND_SUSPENSION', '2': 2},
    {'1': 'MEASURE_KIND_TERMINATION', '2': 3},
    {'1': 'MEASURE_KIND_CONTENT_TAKEDOWN', '2': 4},
  ],
};

/// Descriptor for `MeasureKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List measureKindDescriptor = $convert.base64Decode(
    'CgtNZWFzdXJlS2luZBIcChhNRUFTVVJFX0tJTkRfVU5TUEVDSUZJRUQQABIYChRNRUFTVVJFX0'
    'tJTkRfV0FSTklORxABEhsKF01FQVNVUkVfS0lORF9TVVNQRU5TSU9OEAISHAoYTUVBU1VSRV9L'
    'SU5EX1RFUk1JTkFUSU9OEAMSIQodTUVBU1VSRV9LSU5EX0NPTlRFTlRfVEFLRURPV04QBA==');

@$core.Deprecated('Use contentKindDescriptor instead')
const ContentKind$json = {
  '1': 'ContentKind',
  '2': [
    {'1': 'CONTENT_KIND_UNSPECIFIED', '2': 0},
    {'1': 'CONTENT_KIND_POST', '2': 1},
    {'1': 'CONTENT_KIND_COMMENT', '2': 2},
    {'1': 'CONTENT_KIND_FORUM_ENTRY', '2': 3},
  ],
};

/// Descriptor for `ContentKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List contentKindDescriptor = $convert.base64Decode(
    'CgtDb250ZW50S2luZBIcChhDT05URU5UX0tJTkRfVU5TUEVDSUZJRUQQABIVChFDT05URU5UX0'
    'tJTkRfUE9TVBABEhgKFENPTlRFTlRfS0lORF9DT01NRU5UEAISHAoYQ09OVEVOVF9LSU5EX0ZP'
    'UlVNX0VOVFJZEAM=');

@$core.Deprecated('Use measureBasisDescriptor instead')
const MeasureBasis$json = {
  '1': 'MeasureBasis',
  '2': [
    {'1': 'MEASURE_BASIS_UNSPECIFIED', '2': 0},
    {'1': 'MEASURE_BASIS_VIOLATION', '2': 1},
    {'1': 'MEASURE_BASIS_RISK', '2': 2},
    {'1': 'MEASURE_BASIS_CONTRACT', '2': 3},
  ],
};

/// Descriptor for `MeasureBasis`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List measureBasisDescriptor = $convert.base64Decode(
    'CgxNZWFzdXJlQmFzaXMSHQoZTUVBU1VSRV9CQVNJU19VTlNQRUNJRklFRBAAEhsKF01FQVNVUk'
    'VfQkFTSVNfVklPTEFUSU9OEAESFgoSTUVBU1VSRV9CQVNJU19SSVNLEAISGgoWTUVBU1VSRV9C'
    'QVNJU19DT05UUkFDVBAD');

@$core.Deprecated('Use riskGroundDescriptor instead')
const RiskGround$json = {
  '1': 'RiskGround',
  '2': [
    {'1': 'RISK_GROUND_UNSPECIFIED', '2': 0},
    {'1': 'RISK_GROUND_TRANSACTION', '2': 1},
    {'1': 'RISK_GROUND_FOOD_SAFETY', '2': 2},
    {'1': 'RISK_GROUND_FRAUD', '2': 3},
    {'1': 'RISK_GROUND_PERSONAL_DATA', '2': 4},
    {'1': 'RISK_GROUND_INFRINGEMENT', '2': 5},
    {'1': 'RISK_GROUND_SECURITY', '2': 6},
    {'1': 'RISK_GROUND_FULFILLMENT', '2': 7},
    {'1': 'RISK_GROUND_EXTERNAL_ORDER', '2': 8},
  ],
};

/// Descriptor for `RiskGround`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List riskGroundDescriptor = $convert.base64Decode(
    'CgpSaXNrR3JvdW5kEhsKF1JJU0tfR1JPVU5EX1VOU1BFQ0lGSUVEEAASGwoXUklTS19HUk9VTk'
    'RfVFJBTlNBQ1RJT04QARIbChdSSVNLX0dST1VORF9GT09EX1NBRkVUWRACEhUKEVJJU0tfR1JP'
    'VU5EX0ZSQVVEEAMSHQoZUklTS19HUk9VTkRfUEVSU09OQUxfREFUQRAEEhwKGFJJU0tfR1JPVU'
    '5EX0lORlJJTkdFTUVOVBAFEhgKFFJJU0tfR1JPVU5EX1NFQ1VSSVRZEAYSGwoXUklTS19HUk9V'
    'TkRfRlVMRklMTE1FTlQQBxIeChpSSVNLX0dST1VORF9FWFRFUk5BTF9PUkRFUhAI');

@$core.Deprecated('Use measureStateDescriptor instead')
const MeasureState$json = {
  '1': 'MeasureState',
  '2': [
    {'1': 'MEASURE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'MEASURE_STATE_ISSUED', '2': 1},
    {'1': 'MEASURE_STATE_ACTIVE', '2': 2},
    {'1': 'MEASURE_STATE_ENDED', '2': 3},
    {'1': 'MEASURE_STATE_REVOKED', '2': 4},
    {'1': 'MEASURE_STATE_SCHEDULED', '2': 5},
  ],
};

/// Descriptor for `MeasureState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List measureStateDescriptor = $convert.base64Decode(
    'CgxNZWFzdXJlU3RhdGUSHQoZTUVBU1VSRV9TVEFURV9VTlNQRUNJRklFRBAAEhgKFE1FQVNVUk'
    'VfU1RBVEVfSVNTVUVEEAESGAoUTUVBU1VSRV9TVEFURV9BQ1RJVkUQAhIXChNNRUFTVVJFX1NU'
    'QVRFX0VOREVEEAMSGQoVTUVBU1VSRV9TVEFURV9SRVZPS0VEEAQSGwoXTUVBU1VSRV9TVEFURV'
    '9TQ0hFRFVMRUQQBQ==');

@$core.Deprecated('Use measureRequestKindDescriptor instead')
const MeasureRequestKind$json = {
  '1': 'MeasureRequestKind',
  '2': [
    {'1': 'MEASURE_REQUEST_KIND_UNSPECIFIED', '2': 0},
    {'1': 'MEASURE_REQUEST_KIND_APPEAL', '2': 1},
    {'1': 'MEASURE_REQUEST_KIND_REMEDIATION', '2': 2},
  ],
};

/// Descriptor for `MeasureRequestKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List measureRequestKindDescriptor = $convert.base64Decode(
    'ChJNZWFzdXJlUmVxdWVzdEtpbmQSJAogTUVBU1VSRV9SRVFVRVNUX0tJTkRfVU5TUEVDSUZJRU'
    'QQABIfChtNRUFTVVJFX1JFUVVFU1RfS0lORF9BUFBFQUwQARIkCiBNRUFTVVJFX1JFUVVFU1Rf'
    'S0lORF9SRU1FRElBVElPThAC');

@$core.Deprecated('Use measureRequestStateDescriptor instead')
const MeasureRequestState$json = {
  '1': 'MeasureRequestState',
  '2': [
    {'1': 'MEASURE_REQUEST_STATE_UNSPECIFIED', '2': 0},
    {'1': 'MEASURE_REQUEST_STATE_PENDING', '2': 1},
    {'1': 'MEASURE_REQUEST_STATE_ACCEPTED', '2': 2},
    {'1': 'MEASURE_REQUEST_STATE_REJECTED', '2': 3},
    {'1': 'MEASURE_REQUEST_STATE_CLOSED', '2': 4},
  ],
};

/// Descriptor for `MeasureRequestState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List measureRequestStateDescriptor = $convert.base64Decode(
    'ChNNZWFzdXJlUmVxdWVzdFN0YXRlEiUKIU1FQVNVUkVfUkVRVUVTVF9TVEFURV9VTlNQRUNJRk'
    'lFRBAAEiEKHU1FQVNVUkVfUkVRVUVTVF9TVEFURV9QRU5ESU5HEAESIgoeTUVBU1VSRV9SRVFV'
    'RVNUX1NUQVRFX0FDQ0VQVEVEEAISIgoeTUVBU1VSRV9SRVFVRVNUX1NUQVRFX1JFSkVDVEVEEA'
    'MSIAocTUVBU1VSRV9SRVFVRVNUX1NUQVRFX0NMT1NFRBAE');

@$core.Deprecated('Use remediationUnavailableReasonDescriptor instead')
const RemediationUnavailableReason$json = {
  '1': 'RemediationUnavailableReason',
  '2': [
    {'1': 'REMEDIATION_UNAVAILABLE_REASON_UNSPECIFIED', '2': 0},
    {'1': 'REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING', '2': 1},
    {'1': 'REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING', '2': 2},
    {'1': 'REMEDIATION_UNAVAILABLE_REASON_COOLDOWN', '2': 3},
    {'1': 'REMEDIATION_UNAVAILABLE_REASON_NOT_RISK', '2': 4},
    {'1': 'REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE', '2': 5},
  ],
};

/// Descriptor for `RemediationUnavailableReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List remediationUnavailableReasonDescriptor = $convert.base64Decode(
    'ChxSZW1lZGlhdGlvblVuYXZhaWxhYmxlUmVhc29uEi4KKlJFTUVESUFUSU9OX1VOQVZBSUxBQk'
    'xFX1JFQVNPTl9VTlNQRUNJRklFRBAAEjYKMlJFTUVESUFUSU9OX1VOQVZBSUxBQkxFX1JFQVNP'
    'Tl9SRU1FRElBVElPTl9QRU5ESU5HEAESMQotUkVNRURJQVRJT05fVU5BVkFJTEFCTEVfUkVBU0'
    '9OX0FQUEVBTF9QRU5ESU5HEAISKwonUkVNRURJQVRJT05fVU5BVkFJTEFCTEVfUkVBU09OX0NP'
    'T0xET1dOEAMSKwonUkVNRURJQVRJT05fVU5BVkFJTEFCTEVfUkVBU09OX05PVF9SSVNLEAQSLQ'
    'opUkVNRURJQVRJT05fVU5BVkFJTEFCTEVfUkVBU09OX05PVF9BQ1RJVkUQBQ==');

@$core.Deprecated('Use terminationGroundDescriptor instead')
const TerminationGround$json = {
  '1': 'TerminationGround',
  '2': [
    {'1': 'TERMINATION_GROUND_UNSPECIFIED', '2': 0},
    {'1': 'TERMINATION_GROUND_NOTICE', '2': 1},
    {'1': 'TERMINATION_GROUND_MATERIAL_BREACH', '2': 2},
    {'1': 'TERMINATION_GROUND_CEASED_BUSINESS', '2': 3},
    {'1': 'TERMINATION_GROUND_DISSOLUTION', '2': 4},
    {'1': 'TERMINATION_GROUND_BANKRUPTCY', '2': 5},
    {'1': 'TERMINATION_GROUND_LICENSE_REVOKED', '2': 6},
    {'1': 'TERMINATION_GROUND_FOOD_SAFETY_INCIDENT', '2': 7},
    {'1': 'TERMINATION_GROUND_FRAUD', '2': 8},
    {'1': 'TERMINATION_GROUND_PLATFORM_SECURITY', '2': 9},
    {'1': 'TERMINATION_GROUND_OTHER_SERIOUS', '2': 10},
    {'1': 'TERMINATION_GROUND_LINKED_ACCOUNT', '2': 11},
  ],
};

/// Descriptor for `TerminationGround`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List terminationGroundDescriptor = $convert.base64Decode(
    'ChFUZXJtaW5hdGlvbkdyb3VuZBIiCh5URVJNSU5BVElPTl9HUk9VTkRfVU5TUEVDSUZJRUQQAB'
    'IdChlURVJNSU5BVElPTl9HUk9VTkRfTk9USUNFEAESJgoiVEVSTUlOQVRJT05fR1JPVU5EX01B'
    'VEVSSUFMX0JSRUFDSBACEiYKIlRFUk1JTkFUSU9OX0dST1VORF9DRUFTRURfQlVTSU5FU1MQAx'
    'IiCh5URVJNSU5BVElPTl9HUk9VTkRfRElTU09MVVRJT04QBBIhCh1URVJNSU5BVElPTl9HUk9V'
    'TkRfQkFOS1JVUFRDWRAFEiYKIlRFUk1JTkFUSU9OX0dST1VORF9MSUNFTlNFX1JFVk9LRUQQBh'
    'IrCidURVJNSU5BVElPTl9HUk9VTkRfRk9PRF9TQUZFVFlfSU5DSURFTlQQBxIcChhURVJNSU5B'
    'VElPTl9HUk9VTkRfRlJBVUQQCBIoCiRURVJNSU5BVElPTl9HUk9VTkRfUExBVEZPUk1fU0VDVV'
    'JJVFkQCRIkCiBURVJNSU5BVElPTl9HUk9VTkRfT1RIRVJfU0VSSU9VUxAKEiUKIVRFUk1JTkFU'
    'SU9OX0dST1VORF9MSU5LRURfQUNDT1VOVBAL');

@$core.Deprecated('Use appealUnavailableReasonDescriptor instead')
const AppealUnavailableReason$json = {
  '1': 'AppealUnavailableReason',
  '2': [
    {'1': 'APPEAL_UNAVAILABLE_REASON_UNSPECIFIED', '2': 0},
    {'1': 'APPEAL_UNAVAILABLE_REASON_REVOKED', '2': 1},
    {'1': 'APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED', '2': 2},
    {'1': 'APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED', '2': 3},
    {'1': 'APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING', '2': 4},
  ],
};

/// Descriptor for `AppealUnavailableReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List appealUnavailableReasonDescriptor = $convert.base64Decode(
    'ChdBcHBlYWxVbmF2YWlsYWJsZVJlYXNvbhIpCiVBUFBFQUxfVU5BVkFJTEFCTEVfUkVBU09OX1'
    'VOU1BFQ0lGSUVEEAASJQohQVBQRUFMX1VOQVZBSUxBQkxFX1JFQVNPTl9SRVZPS0VEEAESLgoq'
    'QVBQRUFMX1VOQVZBSUxBQkxFX1JFQVNPTl9BTFJFQURZX0FQUEVBTEVEEAISMQotQVBQRUFMX1'
    'VOQVZBSUxBQkxFX1JFQVNPTl9BUFBFQUxfV0lORE9XX0VOREVEEAMSLQopQVBQRUFMX1VOQVZB'
    'SUxBQkxFX1JFQVNPTl9SRVFVRVNUX1BFTkRJTkcQBA==');

@$core.Deprecated('Use measureRequestCloseCauseDescriptor instead')
const MeasureRequestCloseCause$json = {
  '1': 'MeasureRequestCloseCause',
  '2': [
    {'1': 'MEASURE_REQUEST_CLOSE_CAUSE_UNSPECIFIED', '2': 0},
    {'1': 'MEASURE_REQUEST_CLOSE_CAUSE_REVOKED', '2': 1},
    {'1': 'MEASURE_REQUEST_CLOSE_CAUSE_ENDED', '2': 2},
  ],
};

/// Descriptor for `MeasureRequestCloseCause`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List measureRequestCloseCauseDescriptor = $convert.base64Decode(
    'ChhNZWFzdXJlUmVxdWVzdENsb3NlQ2F1c2USKwonTUVBU1VSRV9SRVFVRVNUX0NMT1NFX0NBVV'
    'NFX1VOU1BFQ0lGSUVEEAASJwojTUVBU1VSRV9SRVFVRVNUX0NMT1NFX0NBVVNFX1JFVk9LRUQQ'
    'ARIlCiFNRUFTVVJFX1JFUVVFU1RfQ0xPU0VfQ0FVU0VfRU5ERUQQAg==');

@$core.Deprecated('Use agreementAcceptanceDescriptor instead')
const AgreementAcceptance$json = {
  '1': 'AgreementAcceptance',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.AgreementKind',
      '8': {},
      '10': 'kind'
    },
    {'1': 'version', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'version'},
  ],
};

/// Descriptor for `AgreementAcceptance`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agreementAcceptanceDescriptor = $convert.base64Decode(
    'ChNBZ3JlZW1lbnRBY2NlcHRhbmNlEj0KBGtpbmQYASABKA4yJC5oYXdrci5zZWxsZXIudjFhbH'
    'BoYTEuQWdyZWVtZW50S2luZEID4EECUgRraW5kEh0KB3ZlcnNpb24YAiABKAlCA+BBAlIHdmVy'
    'c2lvbg==');

@$core.Deprecated('Use sellerAgreementDescriptor instead')
const SellerAgreement$json = {
  '1': 'SellerAgreement',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.AgreementKind',
      '10': 'kind'
    },
    {'1': 'version', '3': 2, '4': 1, '5': 9, '10': 'version'},
    {'1': 'url', '3': 3, '4': 1, '5': 9, '10': 'url'},
  ],
};

/// Descriptor for `SellerAgreement`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerAgreementDescriptor = $convert.base64Decode(
    'Cg9TZWxsZXJBZ3JlZW1lbnQSOAoEa2luZBgBIAEoDjIkLmhhd2tyLnNlbGxlci52MWFscGhhMS'
    '5BZ3JlZW1lbnRLaW5kUgRraW5kEhgKB3ZlcnNpb24YAiABKAlSB3ZlcnNpb24SEAoDdXJsGAMg'
    'ASgJUgN1cmw=');

@$core.Deprecated('Use storeDescriptor instead')
const Store$json = {
  '1': 'Store',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'store_code', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'storeCode'},
    {'1': 'store_name', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'storeName'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {'1': 'contact_phone', '3': 5, '4': 1, '5': 9, '10': 'contactPhone'},
    {
      '1': 'address',
      '3': 21,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.PostalAddress',
      '10': 'address'
    },
    {'1': 'contact_name', '3': 23, '4': 1, '5': 9, '10': 'contactName'},
    {'1': 'contact_mobile', '3': 24, '4': 1, '5': 9, '10': 'contactMobile'},
    {'1': 'support_email', '3': 25, '4': 1, '5': 9, '10': 'supportEmail'},
    {'1': 'prize', '3': 8, '4': 1, '5': 9, '10': 'prize'},
    {'1': 'website_url', '3': 9, '4': 1, '5': 9, '10': 'websiteUrl'},
    {
      '1': 'onboarding_completed',
      '3': 12,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'onboardingCompleted'
    },
    {
      '1': 'logo',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'logo'
    },
    {
      '1': 'cover_image',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'coverImage'
    },
    {
      '1': 'status',
      '3': 15,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.StoreStatus',
      '8': {},
      '10': 'status'
    },
    {
      '1': 'create_time',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'update_time',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {'1': 'etag', '3': 18, '4': 1, '5': 9, '10': 'etag'},
    {
      '1': 'logo_staged_token',
      '3': 19,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'logoStagedToken'
    },
    {
      '1': 'cover_staged_token',
      '3': 20,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'coverStagedToken'
    },
  ],
  '9': [
    {'1': 6, '2': 7},
    {'1': 7, '2': 8},
    {'1': 10, '2': 11},
    {'1': 11, '2': 12},
    {'1': 22, '2': 23},
    {'1': 26, '2': 31},
  ],
  '10': [
    'town_code',
    'address_detail',
    'processed_food',
    'food_business_reg_no',
    'food_business',
    'open_food_registration',
    'approved_food_registration',
    'revoked_food_registration',
    'pending_food_registration',
    'rejected_food_registration'
  ],
};

/// Descriptor for `Store`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storeDescriptor = $convert.base64Decode(
    'CgVTdG9yZRITCgJpZBgBIAEoCUID4EEIUgJpZBIiCgpzdG9yZV9jb2RlGAIgASgJQgPgQQNSCX'
    'N0b3JlQ29kZRIiCgpzdG9yZV9uYW1lGAMgASgJQgPgQQJSCXN0b3JlTmFtZRIgCgtkZXNjcmlw'
    'dGlvbhgEIAEoCVILZGVzY3JpcHRpb24SIwoNY29udGFjdF9waG9uZRgFIAEoCVIMY29udGFjdF'
    'Bob25lEj4KB2FkZHJlc3MYFSABKAsyJC5oYXdrci5jb21tb24udjFhbHBoYTEuUG9zdGFsQWRk'
    'cmVzc1IHYWRkcmVzcxIhCgxjb250YWN0X25hbWUYFyABKAlSC2NvbnRhY3ROYW1lEiUKDmNvbn'
    'RhY3RfbW9iaWxlGBggASgJUg1jb250YWN0TW9iaWxlEiMKDXN1cHBvcnRfZW1haWwYGSABKAlS'
    'DHN1cHBvcnRFbWFpbBIUCgVwcml6ZRgIIAEoCVIFcHJpemUSHwoLd2Vic2l0ZV91cmwYCSABKA'
    'lSCndlYnNpdGVVcmwSNgoUb25ib2FyZGluZ19jb21wbGV0ZWQYDCABKAhCA+BBA1ITb25ib2Fy'
    'ZGluZ0NvbXBsZXRlZBI1CgRsb2dvGA0gASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaGExLkltYW'
    'dlQgPgQQNSBGxvZ28SQgoLY292ZXJfaW1hZ2UYDiABKAsyHC5oYXdrci5jb21tb24udjFhbHBo'
    'YTEuSW1hZ2VCA+BBA1IKY292ZXJJbWFnZRI/CgZzdGF0dXMYDyABKA4yIi5oYXdrci5zZWxsZX'
    'IudjFhbHBoYTEuU3RvcmVTdGF0dXNCA+BBA1IGc3RhdHVzEkAKC2NyZWF0ZV90aW1lGBAgASgL'
    'MhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpjcmVhdGVUaW1lEkAKC3VwZGF0ZV'
    '90aW1lGBEgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgp1cGRhdGVUaW1l'
    'EhIKBGV0YWcYEiABKAlSBGV0YWcSLwoRbG9nb19zdGFnZWRfdG9rZW4YEyABKAlCA+BBBFIPbG'
    '9nb1N0YWdlZFRva2VuEjEKEmNvdmVyX3N0YWdlZF90b2tlbhgUIAEoCUID4EEEUhBjb3ZlclN0'
    'YWdlZFRva2VuSgQIBhAHSgQIBxAISgQIChALSgQICxAMSgQIFhAXSgQIGhAfUgl0b3duX2NvZG'
    'VSDmFkZHJlc3NfZGV0YWlsUg5wcm9jZXNzZWRfZm9vZFIUZm9vZF9idXNpbmVzc19yZWdfbm9S'
    'DWZvb2RfYnVzaW5lc3NSFm9wZW5fZm9vZF9yZWdpc3RyYXRpb25SGmFwcHJvdmVkX2Zvb2Rfcm'
    'VnaXN0cmF0aW9uUhlyZXZva2VkX2Zvb2RfcmVnaXN0cmF0aW9uUhlwZW5kaW5nX2Zvb2RfcmVn'
    'aXN0cmF0aW9uUhpyZWplY3RlZF9mb29kX3JlZ2lzdHJhdGlvbg==');

@$core.Deprecated('Use capabilityStatusDescriptor instead')
const CapabilityStatus$json = {
  '1': 'CapabilityStatus',
  '2': [
    {
      '1': 'capability',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.SellerCapability',
      '10': 'capability'
    },
    {'1': 'active', '3': 2, '4': 1, '5': 8, '10': 'active'},
    {
      '1': 'missing',
      '3': 3,
      '4': 3,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.SellerRequirement',
      '10': 'missing'
    },
  ],
};

/// Descriptor for `CapabilityStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List capabilityStatusDescriptor = $convert.base64Decode(
    'ChBDYXBhYmlsaXR5U3RhdHVzEkcKCmNhcGFiaWxpdHkYASABKA4yJy5oYXdrci5zZWxsZXIudj'
    'FhbHBoYTEuU2VsbGVyQ2FwYWJpbGl0eVIKY2FwYWJpbGl0eRIWCgZhY3RpdmUYAiABKAhSBmFj'
    'dGl2ZRJCCgdtaXNzaW5nGAMgAygOMiguaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlNlbGxlclJlcX'
    'VpcmVtZW50UgdtaXNzaW5n');

@$core.Deprecated('Use applyStoreRequestDescriptor instead')
const ApplyStoreRequest$json = {
  '1': 'ApplyStoreRequest',
  '2': [
    {'1': 'store_name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeName'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'contact_phone', '3': 3, '4': 1, '5': 9, '10': 'contactPhone'},
    {'1': 'prize', '3': 6, '4': 1, '5': 9, '10': 'prize'},
    {
      '1': 'address',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.PostalAddress',
      '10': 'address'
    },
    {
      '1': 'accepted_agreements',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.AgreementAcceptance',
      '8': {},
      '10': 'acceptedAgreements'
    },
  ],
  '9': [
    {'1': 4, '2': 5},
    {'1': 5, '2': 6},
  ],
  '10': ['town_code', 'address_detail'],
};

/// Descriptor for `ApplyStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyStoreRequestDescriptor = $convert.base64Decode(
    'ChFBcHBseVN0b3JlUmVxdWVzdBIiCgpzdG9yZV9uYW1lGAEgASgJQgPgQQJSCXN0b3JlTmFtZR'
    'IgCgtkZXNjcmlwdGlvbhgCIAEoCVILZGVzY3JpcHRpb24SIwoNY29udGFjdF9waG9uZRgDIAEo'
    'CVIMY29udGFjdFBob25lEhQKBXByaXplGAYgASgJUgVwcml6ZRI+CgdhZGRyZXNzGAcgASgLMi'
    'QuaGF3a3IuY29tbW9uLnYxYWxwaGExLlBvc3RhbEFkZHJlc3NSB2FkZHJlc3MSYAoTYWNjZXB0'
    'ZWRfYWdyZWVtZW50cxgIIAMoCzIqLmhhd2tyLnNlbGxlci52MWFscGhhMS5BZ3JlZW1lbnRBY2'
    'NlcHRhbmNlQgPgQQJSEmFjY2VwdGVkQWdyZWVtZW50c0oECAQQBUoECAUQBlIJdG93bl9jb2Rl'
    'Ug5hZGRyZXNzX2RldGFpbA==');

@$core.Deprecated('Use applyStoreResponseDescriptor instead')
const ApplyStoreResponse$json = {
  '1': 'ApplyStoreResponse',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Store',
      '10': 'store'
    },
  ],
};

/// Descriptor for `ApplyStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyStoreResponseDescriptor = $convert.base64Decode(
    'ChJBcHBseVN0b3JlUmVzcG9uc2USMgoFc3RvcmUYASABKAsyHC5oYXdrci5zZWxsZXIudjFhbH'
    'BoYTEuU3RvcmVSBXN0b3Jl');

@$core.Deprecated('Use getMyStoreOpeningRequestDescriptor instead')
const GetMyStoreOpeningRequest$json = {
  '1': 'GetMyStoreOpeningRequest',
};

/// Descriptor for `GetMyStoreOpeningRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyStoreOpeningRequestDescriptor =
    $convert.base64Decode('ChhHZXRNeVN0b3JlT3BlbmluZ1JlcXVlc3Q=');

@$core.Deprecated('Use getMyStoreOpeningResponseDescriptor instead')
const GetMyStoreOpeningResponse$json = {
  '1': 'GetMyStoreOpeningResponse',
  '2': [
    {
      '1': 'can_apply_store',
      '3': 1,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'canApplyStore'
    },
    {
      '1': 'unavailabilities',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.StoreOpeningUnavailability',
      '8': {},
      '10': 'unavailabilities'
    },
  ],
  '9': [
    {'1': 2, '2': 3},
  ],
  '10': ['unavailable_reasons'],
};

/// Descriptor for `GetMyStoreOpeningResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyStoreOpeningResponseDescriptor = $convert.base64Decode(
    'ChlHZXRNeVN0b3JlT3BlbmluZ1Jlc3BvbnNlEisKD2Nhbl9hcHBseV9zdG9yZRgBIAEoCEID4E'
    'EDUg1jYW5BcHBseVN0b3JlEmIKEHVuYXZhaWxhYmlsaXRpZXMYAyADKAsyMS5oYXdrci5zZWxs'
    'ZXIudjFhbHBoYTEuU3RvcmVPcGVuaW5nVW5hdmFpbGFiaWxpdHlCA+BBA1IQdW5hdmFpbGFiaW'
    'xpdGllc0oECAIQA1ITdW5hdmFpbGFibGVfcmVhc29ucw==');

@$core.Deprecated('Use storeOpeningUnavailabilityDescriptor instead')
const StoreOpeningUnavailability$json = {
  '1': 'StoreOpeningUnavailability',
  '2': [
    {
      '1': 'reason',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.StoreOpeningUnavailableReason',
      '8': {},
      '10': 'reason'
    },
    {
      '1': 'message',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.rpc.LocalizedMessage',
      '8': {},
      '10': 'message'
    },
  ],
};

/// Descriptor for `StoreOpeningUnavailability`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storeOpeningUnavailabilityDescriptor = $convert.base64Decode(
    'ChpTdG9yZU9wZW5pbmdVbmF2YWlsYWJpbGl0eRJRCgZyZWFzb24YASABKA4yNC5oYXdrci5zZW'
    'xsZXIudjFhbHBoYTEuU3RvcmVPcGVuaW5nVW5hdmFpbGFibGVSZWFzb25CA+BBA1IGcmVhc29u'
    'EjsKB21lc3NhZ2UYAiABKAsyHC5nb29nbGUucnBjLkxvY2FsaXplZE1lc3NhZ2VCA+BBA1IHbW'
    'Vzc2FnZQ==');

@$core.Deprecated('Use getMyStoreRequestDescriptor instead')
const GetMyStoreRequest$json = {
  '1': 'GetMyStoreRequest',
};

/// Descriptor for `GetMyStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyStoreRequestDescriptor =
    $convert.base64Decode('ChFHZXRNeVN0b3JlUmVxdWVzdA==');

@$core.Deprecated('Use getMyStoreResponseDescriptor instead')
const GetMyStoreResponse$json = {
  '1': 'GetMyStoreResponse',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Store',
      '10': 'store'
    },
    {
      '1': 'capabilities',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.CapabilityStatus',
      '10': 'capabilities'
    },
    {
      '1': 'stats',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.StoreStats',
      '10': 'stats'
    },
    {'1': 'suspended', '3': 6, '4': 1, '5': 8, '8': {}, '10': 'suspended'},
  ],
  '9': [
    {'1': 2, '2': 3},
    {'1': 4, '2': 5},
  ],
  '10': ['suspension', 'exposure'],
};

/// Descriptor for `GetMyStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyStoreResponseDescriptor = $convert.base64Decode(
    'ChJHZXRNeVN0b3JlUmVzcG9uc2USMgoFc3RvcmUYASABKAsyHC5oYXdrci5zZWxsZXIudjFhbH'
    'BoYTEuU3RvcmVSBXN0b3JlEksKDGNhcGFiaWxpdGllcxgDIAMoCzInLmhhd2tyLnNlbGxlci52'
    'MWFscGhhMS5DYXBhYmlsaXR5U3RhdHVzUgxjYXBhYmlsaXRpZXMSNwoFc3RhdHMYBSABKAsyIS'
    '5oYXdrci5zZWxsZXIudjFhbHBoYTEuU3RvcmVTdGF0c1IFc3RhdHMSIQoJc3VzcGVuZGVkGAYg'
    'ASgIQgPgQQNSCXN1c3BlbmRlZEoECAIQA0oECAQQBVIKc3VzcGVuc2lvblIIZXhwb3N1cmU=');

@$core.Deprecated('Use getMyOnboardingRequestDescriptor instead')
const GetMyOnboardingRequest$json = {
  '1': 'GetMyOnboardingRequest',
};

/// Descriptor for `GetMyOnboardingRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyOnboardingRequestDescriptor =
    $convert.base64Decode('ChZHZXRNeU9uYm9hcmRpbmdSZXF1ZXN0');

@$core.Deprecated('Use getMyOnboardingResponseDescriptor instead')
const GetMyOnboardingResponse$json = {
  '1': 'GetMyOnboardingResponse',
  '2': [
    {
      '1': 'tasks',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.OnboardingTask',
      '8': {},
      '10': 'tasks'
    },
    {'1': 'complete', '3': 2, '4': 1, '5': 8, '8': {}, '10': 'complete'},
  ],
};

/// Descriptor for `GetMyOnboardingResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyOnboardingResponseDescriptor = $convert.base64Decode(
    'ChdHZXRNeU9uYm9hcmRpbmdSZXNwb25zZRJACgV0YXNrcxgBIAMoCzIlLmhhd2tyLnNlbGxlci'
    '52MWFscGhhMS5PbmJvYXJkaW5nVGFza0ID4EEDUgV0YXNrcxIfCghjb21wbGV0ZRgCIAEoCEID'
    '4EEDUghjb21wbGV0ZQ==');

@$core.Deprecated('Use onboardingTaskDescriptor instead')
const OnboardingTask$json = {
  '1': 'OnboardingTask',
  '2': [
    {
      '1': 'id',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.OnboardingTaskId',
      '8': {},
      '10': 'id'
    },
    {
      '1': 'state',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.OnboardingTaskState',
      '8': {},
      '10': 'state'
    },
    {
      '1': 'action',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.OnboardingAction',
      '8': {},
      '10': 'action'
    },
    {
      '1': 'title',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.rpc.LocalizedMessage',
      '8': {},
      '10': 'title'
    },
    {
      '1': 'status',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.rpc.LocalizedMessage',
      '8': {},
      '10': 'status'
    },
  ],
};

/// Descriptor for `OnboardingTask`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List onboardingTaskDescriptor = $convert.base64Decode(
    'Cg5PbmJvYXJkaW5nVGFzaxI8CgJpZBgBIAEoDjInLmhhd2tyLnNlbGxlci52MWFscGhhMS5Pbm'
    'JvYXJkaW5nVGFza0lkQgPgQQNSAmlkEkUKBXN0YXRlGAIgASgOMiouaGF3a3Iuc2VsbGVyLnYx'
    'YWxwaGExLk9uYm9hcmRpbmdUYXNrU3RhdGVCA+BBA1IFc3RhdGUSRAoGYWN0aW9uGAMgASgOMi'
    'cuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLk9uYm9hcmRpbmdBY3Rpb25CA+BBA1IGYWN0aW9uEjcK'
    'BXRpdGxlGAQgASgLMhwuZ29vZ2xlLnJwYy5Mb2NhbGl6ZWRNZXNzYWdlQgPgQQNSBXRpdGxlEj'
    'kKBnN0YXR1cxgFIAEoCzIcLmdvb2dsZS5ycGMuTG9jYWxpemVkTWVzc2FnZUID4EEDUgZzdGF0'
    'dXM=');

@$core.Deprecated('Use updateMyStoreRequestDescriptor instead')
const UpdateMyStoreRequest$json = {
  '1': 'UpdateMyStoreRequest',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Store',
      '8': {},
      '10': 'store'
    },
    {
      '1': 'update_mask',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdateMyStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateMyStoreRequestDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVNeVN0b3JlUmVxdWVzdBI3CgVzdG9yZRgBIAEoCzIcLmhhd2tyLnNlbGxlci52MW'
    'FscGhhMS5TdG9yZUID4EECUgVzdG9yZRI7Cgt1cGRhdGVfbWFzaxgCIAEoCzIaLmdvb2dsZS5w'
    'cm90b2J1Zi5GaWVsZE1hc2tSCnVwZGF0ZU1hc2s=');

@$core.Deprecated('Use updateMyStoreResponseDescriptor instead')
const UpdateMyStoreResponse$json = {
  '1': 'UpdateMyStoreResponse',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Store',
      '10': 'store'
    },
  ],
};

/// Descriptor for `UpdateMyStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateMyStoreResponseDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVNeVN0b3JlUmVzcG9uc2USMgoFc3RvcmUYASABKAsyHC5oYXdrci5zZWxsZXIudj'
    'FhbHBoYTEuU3RvcmVSBXN0b3Jl');

@$core.Deprecated('Use refreshMyPaymentAccountRequestDescriptor instead')
const RefreshMyPaymentAccountRequest$json = {
  '1': 'RefreshMyPaymentAccountRequest',
};

/// Descriptor for `RefreshMyPaymentAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List refreshMyPaymentAccountRequestDescriptor =
    $convert.base64Decode('Ch5SZWZyZXNoTXlQYXltZW50QWNjb3VudFJlcXVlc3Q=');

@$core.Deprecated('Use refreshMyPaymentAccountResponseDescriptor instead')
const RefreshMyPaymentAccountResponse$json = {
  '1': 'RefreshMyPaymentAccountResponse',
};

/// Descriptor for `RefreshMyPaymentAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List refreshMyPaymentAccountResponseDescriptor =
    $convert.base64Decode('Ch9SZWZyZXNoTXlQYXltZW50QWNjb3VudFJlc3BvbnNl');

@$core.Deprecated('Use completeMyStoreOnboardingRequestDescriptor instead')
const CompleteMyStoreOnboardingRequest$json = {
  '1': 'CompleteMyStoreOnboardingRequest',
  '2': [
    {'1': 'etag', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `CompleteMyStoreOnboardingRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeMyStoreOnboardingRequestDescriptor =
    $convert.base64Decode(
        'CiBDb21wbGV0ZU15U3RvcmVPbmJvYXJkaW5nUmVxdWVzdBIXCgRldGFnGAEgASgJQgPgQQJSBG'
        'V0YWc=');

@$core.Deprecated('Use completeMyStoreOnboardingResponseDescriptor instead')
const CompleteMyStoreOnboardingResponse$json = {
  '1': 'CompleteMyStoreOnboardingResponse',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Store',
      '10': 'store'
    },
  ],
};

/// Descriptor for `CompleteMyStoreOnboardingResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeMyStoreOnboardingResponseDescriptor =
    $convert.base64Decode(
        'CiFDb21wbGV0ZU15U3RvcmVPbmJvYXJkaW5nUmVzcG9uc2USMgoFc3RvcmUYASABKAsyHC5oYX'
        'drci5zZWxsZXIudjFhbHBoYTEuU3RvcmVSBXN0b3Jl');

@$core.Deprecated('Use listMyFollowersRequestDescriptor instead')
const ListMyFollowersRequest$json = {
  '1': 'ListMyFollowersRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListMyFollowersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyFollowersRequestDescriptor =
    $convert.base64Decode(
        'ChZMaXN0TXlGb2xsb3dlcnNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQ'
        'oKcGFnZV90b2tlbhgCIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listMyFollowersResponseDescriptor instead')
const ListMyFollowersResponse$json = {
  '1': 'ListMyFollowersResponse',
  '2': [
    {
      '1': 'followers',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Follower',
      '10': 'followers'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyFollowersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyFollowersResponseDescriptor = $convert.base64Decode(
    'ChdMaXN0TXlGb2xsb3dlcnNSZXNwb25zZRI9Cglmb2xsb3dlcnMYASADKAsyHy5oYXdrci5zZW'
    'xsZXIudjFhbHBoYTEuRm9sbG93ZXJSCWZvbGxvd2VycxImCg9uZXh0X3BhZ2VfdG9rZW4YAiAB'
    'KAlSDW5leHRQYWdlVG9rZW4=');

@$core.Deprecated('Use buyerCardDescriptor instead')
const BuyerCard$json = {
  '1': 'BuyerCard',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'deleted', '3': 2, '4': 1, '5': 8, '10': 'deleted'},
    {'1': 'display_name', '3': 3, '4': 1, '5': 9, '10': 'displayName'},
    {
      '1': 'avatar',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'avatar'
    },
  ],
};

/// Descriptor for `BuyerCard`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List buyerCardDescriptor = $convert.base64Decode(
    'CglCdXllckNhcmQSDgoCaWQYASABKAlSAmlkEhgKB2RlbGV0ZWQYAiABKAhSB2RlbGV0ZWQSIQ'
    'oMZGlzcGxheV9uYW1lGAMgASgJUgtkaXNwbGF5TmFtZRI0CgZhdmF0YXIYBCABKAsyHC5oYXdr'
    'ci5jb21tb24udjFhbHBoYTEuSW1hZ2VSBmF2YXRhcg==');

@$core.Deprecated('Use followerDescriptor instead')
const Follower$json = {
  '1': 'Follower',
  '2': [
    {
      '1': 'buyer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.BuyerCard',
      '10': 'buyer'
    },
    {
      '1': 'create_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
  ],
};

/// Descriptor for `Follower`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List followerDescriptor = $convert.base64Decode(
    'CghGb2xsb3dlchI2CgVidXllchgBIAEoCzIgLmhhd2tyLnNlbGxlci52MWFscGhhMS5CdXllck'
    'NhcmRSBWJ1eWVyEjsKC2NyZWF0ZV90aW1lGAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVz'
    'dGFtcFIKY3JlYXRlVGltZQ==');

@$core.Deprecated('Use getMyDashboardRequestDescriptor instead')
const GetMyDashboardRequest$json = {
  '1': 'GetMyDashboardRequest',
};

/// Descriptor for `GetMyDashboardRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyDashboardRequestDescriptor =
    $convert.base64Decode('ChVHZXRNeURhc2hib2FyZFJlcXVlc3Q=');

@$core.Deprecated('Use getMyDashboardResponseDescriptor instead')
const GetMyDashboardResponse$json = {
  '1': 'GetMyDashboardResponse',
  '2': [
    {
      '1': 'dashboard',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Dashboard',
      '10': 'dashboard'
    },
  ],
};

/// Descriptor for `GetMyDashboardResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyDashboardResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRNeURhc2hib2FyZFJlc3BvbnNlEj4KCWRhc2hib2FyZBgBIAEoCzIgLmhhd2tyLnNlbG'
        'xlci52MWFscGhhMS5EYXNoYm9hcmRSCWRhc2hib2FyZA==');

@$core.Deprecated('Use dashboardDescriptor instead')
const Dashboard$json = {
  '1': 'Dashboard',
  '2': [
    {
      '1': 'current_period',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.DashboardPeriod',
      '10': 'currentPeriod'
    },
    {
      '1': 'previous_period',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.DashboardPeriod',
      '10': 'previousPeriod'
    },
    {
      '1': 'sales',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.DashboardSales',
      '10': 'sales'
    },
    {
      '1': 'followers',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.DashboardFollowers',
      '10': 'followers'
    },
    {
      '1': 'product_views',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PeriodCount',
      '10': 'productViews'
    },
    {
      '1': 'order_status_counts',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.OrderStatusCounts',
      '10': 'orderStatusCounts'
    },
  ],
};

/// Descriptor for `Dashboard`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dashboardDescriptor = $convert.base64Decode(
    'CglEYXNoYm9hcmQSTQoOY3VycmVudF9wZXJpb2QYASABKAsyJi5oYXdrci5zZWxsZXIudjFhbH'
    'BoYTEuRGFzaGJvYXJkUGVyaW9kUg1jdXJyZW50UGVyaW9kEk8KD3ByZXZpb3VzX3BlcmlvZBgC'
    'IAEoCzImLmhhd2tyLnNlbGxlci52MWFscGhhMS5EYXNoYm9hcmRQZXJpb2RSDnByZXZpb3VzUG'
    'VyaW9kEjsKBXNhbGVzGAMgASgLMiUuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkRhc2hib2FyZFNh'
    'bGVzUgVzYWxlcxJHCglmb2xsb3dlcnMYBCABKAsyKS5oYXdrci5zZWxsZXIudjFhbHBoYTEuRG'
    'FzaGJvYXJkRm9sbG93ZXJzUglmb2xsb3dlcnMSRwoNcHJvZHVjdF92aWV3cxgFIAEoCzIiLmhh'
    'd2tyLnNlbGxlci52MWFscGhhMS5QZXJpb2RDb3VudFIMcHJvZHVjdFZpZXdzElgKE29yZGVyX3'
    'N0YXR1c19jb3VudHMYBiABKAsyKC5oYXdrci5zZWxsZXIudjFhbHBoYTEuT3JkZXJTdGF0dXND'
    'b3VudHNSEW9yZGVyU3RhdHVzQ291bnRz');

@$core.Deprecated('Use dashboardPeriodDescriptor instead')
const DashboardPeriod$json = {
  '1': 'DashboardPeriod',
  '2': [
    {
      '1': 'first_date',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '10': 'firstDate'
    },
    {
      '1': 'last_date',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '10': 'lastDate'
    },
  ],
};

/// Descriptor for `DashboardPeriod`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dashboardPeriodDescriptor = $convert.base64Decode(
    'Cg9EYXNoYm9hcmRQZXJpb2QSMAoKZmlyc3RfZGF0ZRgBIAEoCzIRLmdvb2dsZS50eXBlLkRhdG'
    'VSCWZpcnN0RGF0ZRIuCglsYXN0X2RhdGUYAiABKAsyES5nb29nbGUudHlwZS5EYXRlUghsYXN0'
    'RGF0ZQ==');

@$core.Deprecated('Use periodAmountDescriptor instead')
const PeriodAmount$json = {
  '1': 'PeriodAmount',
  '2': [
    {'1': 'current', '3': 1, '4': 1, '5': 3, '10': 'current'},
    {'1': 'previous', '3': 2, '4': 1, '5': 3, '10': 'previous'},
  ],
};

/// Descriptor for `PeriodAmount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List periodAmountDescriptor = $convert.base64Decode(
    'CgxQZXJpb2RBbW91bnQSGAoHY3VycmVudBgBIAEoA1IHY3VycmVudBIaCghwcmV2aW91cxgCIA'
    'EoA1IIcHJldmlvdXM=');

@$core.Deprecated('Use periodCountDescriptor instead')
const PeriodCount$json = {
  '1': 'PeriodCount',
  '2': [
    {'1': 'current', '3': 1, '4': 1, '5': 5, '10': 'current'},
    {'1': 'previous', '3': 2, '4': 1, '5': 5, '10': 'previous'},
  ],
};

/// Descriptor for `PeriodCount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List periodCountDescriptor = $convert.base64Decode(
    'CgtQZXJpb2RDb3VudBIYCgdjdXJyZW50GAEgASgFUgdjdXJyZW50EhoKCHByZXZpb3VzGAIgAS'
    'gFUghwcmV2aW91cw==');

@$core.Deprecated('Use dashboardSalesDescriptor instead')
const DashboardSales$json = {
  '1': 'DashboardSales',
  '2': [
    {
      '1': 'revenue',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PeriodAmount',
      '10': 'revenue'
    },
    {
      '1': 'orders',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PeriodCount',
      '10': 'orders'
    },
    {
      '1': 'customers',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PeriodCount',
      '10': 'customers'
    },
  ],
};

/// Descriptor for `DashboardSales`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dashboardSalesDescriptor = $convert.base64Decode(
    'Cg5EYXNoYm9hcmRTYWxlcxI9CgdyZXZlbnVlGAEgASgLMiMuaGF3a3Iuc2VsbGVyLnYxYWxwaG'
    'ExLlBlcmlvZEFtb3VudFIHcmV2ZW51ZRI6CgZvcmRlcnMYAiABKAsyIi5oYXdrci5zZWxsZXIu'
    'djFhbHBoYTEuUGVyaW9kQ291bnRSBm9yZGVycxJACgljdXN0b21lcnMYAyABKAsyIi5oYXdrci'
    '5zZWxsZXIudjFhbHBoYTEuUGVyaW9kQ291bnRSCWN1c3RvbWVycw==');

@$core.Deprecated('Use dashboardFollowersDescriptor instead')
const DashboardFollowers$json = {
  '1': 'DashboardFollowers',
  '2': [
    {'1': 'total', '3': 1, '4': 1, '5': 5, '10': 'total'},
    {
      '1': 'new_followers',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PeriodCount',
      '10': 'newFollowers'
    },
  ],
};

/// Descriptor for `DashboardFollowers`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dashboardFollowersDescriptor = $convert.base64Decode(
    'ChJEYXNoYm9hcmRGb2xsb3dlcnMSFAoFdG90YWwYASABKAVSBXRvdGFsEkcKDW5ld19mb2xsb3'
    'dlcnMYAiABKAsyIi5oYXdrci5zZWxsZXIudjFhbHBoYTEuUGVyaW9kQ291bnRSDG5ld0ZvbGxv'
    'd2Vycw==');

@$core.Deprecated('Use orderStatusCountsDescriptor instead')
const OrderStatusCounts$json = {
  '1': 'OrderStatusCounts',
  '2': [
    {
      '1': 'pending_shipment_count',
      '3': 1,
      '4': 1,
      '5': 5,
      '10': 'pendingShipmentCount'
    },
    {'1': 'in_progress_count', '3': 2, '4': 1, '5': 5, '10': 'inProgressCount'},
    {'1': 'closed_count', '3': 3, '4': 1, '5': 5, '10': 'closedCount'},
    {'1': 'aftersales_count', '3': 4, '4': 1, '5': 5, '10': 'aftersalesCount'},
  ],
};

/// Descriptor for `OrderStatusCounts`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List orderStatusCountsDescriptor = $convert.base64Decode(
    'ChFPcmRlclN0YXR1c0NvdW50cxI0ChZwZW5kaW5nX3NoaXBtZW50X2NvdW50GAEgASgFUhRwZW'
    '5kaW5nU2hpcG1lbnRDb3VudBIqChFpbl9wcm9ncmVzc19jb3VudBgCIAEoBVIPaW5Qcm9ncmVz'
    'c0NvdW50EiEKDGNsb3NlZF9jb3VudBgDIAEoBVILY2xvc2VkQ291bnQSKQoQYWZ0ZXJzYWxlc1'
    '9jb3VudBgEIAEoBVIPYWZ0ZXJzYWxlc0NvdW50');

@$core.Deprecated('Use closeMyStoreRequestDescriptor instead')
const CloseMyStoreRequest$json = {
  '1': 'CloseMyStoreRequest',
};

/// Descriptor for `CloseMyStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List closeMyStoreRequestDescriptor =
    $convert.base64Decode('ChNDbG9zZU15U3RvcmVSZXF1ZXN0');

@$core.Deprecated('Use closeMyStoreResponseDescriptor instead')
const CloseMyStoreResponse$json = {
  '1': 'CloseMyStoreResponse',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Store',
      '10': 'store'
    },
  ],
};

/// Descriptor for `CloseMyStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List closeMyStoreResponseDescriptor = $convert.base64Decode(
    'ChRDbG9zZU15U3RvcmVSZXNwb25zZRIyCgVzdG9yZRgBIAEoCzIcLmhhd2tyLnNlbGxlci52MW'
    'FscGhhMS5TdG9yZVIFc3RvcmU=');

@$core.Deprecated('Use kYCIssueDescriptor instead')
const KYCIssue$json = {
  '1': 'KYCIssue',
  '2': [
    {
      '1': 'field',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.KYCField',
      '10': 'field'
    },
    {'1': 'document_asset_id', '3': 2, '4': 1, '5': 9, '10': 'documentAssetId'},
    {
      '1': 'code',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.KYCIssueCode',
      '10': 'code'
    },
    {'1': 'message', '3': 4, '4': 1, '5': 9, '10': 'message'},
  ],
};

/// Descriptor for `KYCIssue`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List kYCIssueDescriptor = $convert.base64Decode(
    'CghLWUNJc3N1ZRI1CgVmaWVsZBgBIAEoDjIfLmhhd2tyLnNlbGxlci52MWFscGhhMS5LWUNGaW'
    'VsZFIFZmllbGQSKgoRZG9jdW1lbnRfYXNzZXRfaWQYAiABKAlSD2RvY3VtZW50QXNzZXRJZBI3'
    'CgRjb2RlGAMgASgOMiMuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLktZQ0lzc3VlQ29kZVIEY29kZR'
    'IYCgdtZXNzYWdlGAQgASgJUgdtZXNzYWdl');

@$core.Deprecated('Use kYCDocumentDescriptor instead')
const KYCDocument$json = {
  '1': 'KYCDocument',
  '2': [
    {'1': 'asset_id', '3': 1, '4': 1, '5': 9, '10': 'assetId'},
    {'1': 'download_uri', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'downloadUri'},
    {'1': 'staged_token', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'stagedToken'},
  ],
};

/// Descriptor for `KYCDocument`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List kYCDocumentDescriptor = $convert.base64Decode(
    'CgtLWUNEb2N1bWVudBIZCghhc3NldF9pZBgBIAEoCVIHYXNzZXRJZBImCgxkb3dubG9hZF91cm'
    'kYAiABKAlCA+BBA1ILZG93bmxvYWRVcmkSJgoMc3RhZ2VkX3Rva2VuGAMgASgJQgPgQQRSC3N0'
    'YWdlZFRva2Vu');

@$core.Deprecated('Use kYCRejectionDescriptor instead')
const KYCRejection$json = {
  '1': 'KYCRejection',
  '2': [
    {'1': 'reason', '3': 1, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'decide_time',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'decideTime'
    },
    {
      '1': 'issues',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCIssue',
      '10': 'issues'
    },
  ],
  '9': [
    {'1': 2, '2': 3},
  ],
  '10': ['required_fields'],
};

/// Descriptor for `KYCRejection`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List kYCRejectionDescriptor = $convert.base64Decode(
    'CgxLWUNSZWplY3Rpb24SFgoGcmVhc29uGAEgASgJUgZyZWFzb24SOwoLZGVjaWRlX3RpbWUYAy'
    'ABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpkZWNpZGVUaW1lEjcKBmlzc3VlcxgE'
    'IAMoCzIfLmhhd2tyLnNlbGxlci52MWFscGhhMS5LWUNJc3N1ZVIGaXNzdWVzSgQIAhADUg9yZX'
    'F1aXJlZF9maWVsZHM=');

@$core.Deprecated('Use individualKYCDescriptor instead')
const IndividualKYC$json = {
  '1': 'IndividualKYC',
  '9': [
    {'1': 1, '2': 2},
    {'1': 2, '2': 3},
  ],
  '10': ['id_card_no', 'id_card_no_masked'],
};

/// Descriptor for `IndividualKYC`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List individualKYCDescriptor = $convert.base64Decode(
    'Cg1JbmRpdmlkdWFsS1lDSgQIARACSgQIAhADUgppZF9jYXJkX25vUhFpZF9jYXJkX25vX21hc2'
    'tlZA==');

@$core.Deprecated('Use businessKYCDescriptor instead')
const BusinessKYC$json = {
  '1': 'BusinessKYC',
  '2': [
    {'1': 'company_name', '3': 2, '4': 1, '5': 9, '10': 'companyName'},
    {'1': 'tax_id', '3': 3, '4': 1, '5': 9, '10': 'taxId'},
    {
      '1': 'company_register_date',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '10': 'companyRegisterDate'
    },
    {'1': 'company_phone', '3': 7, '4': 1, '5': 9, '10': 'companyPhone'},
    {
      '1': 'company_documents',
      '3': 10,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCDocument',
      '10': 'companyDocuments'
    },
    {
      '1': 'company_address',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.PostalAddress',
      '10': 'companyAddress'
    },
  ],
  '9': [
    {'1': 1, '2': 2},
    {'1': 5, '2': 6},
    {'1': 6, '2': 7},
    {'1': 8, '2': 9},
    {'1': 9, '2': 10},
  ],
  '10': [
    'company_type',
    'company_town_code',
    'company_address_detail',
    'rep_id_card_no',
    'rep_id_card_no_masked'
  ],
};

/// Descriptor for `BusinessKYC`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List businessKYCDescriptor = $convert.base64Decode(
    'CgtCdXNpbmVzc0tZQxIhCgxjb21wYW55X25hbWUYAiABKAlSC2NvbXBhbnlOYW1lEhUKBnRheF'
    '9pZBgDIAEoCVIFdGF4SWQSRQoVY29tcGFueV9yZWdpc3Rlcl9kYXRlGAQgASgLMhEuZ29vZ2xl'
    'LnR5cGUuRGF0ZVITY29tcGFueVJlZ2lzdGVyRGF0ZRIjCg1jb21wYW55X3Bob25lGAcgASgJUg'
    'xjb21wYW55UGhvbmUSTwoRY29tcGFueV9kb2N1bWVudHMYCiADKAsyIi5oYXdrci5zZWxsZXIu'
    'djFhbHBoYTEuS1lDRG9jdW1lbnRSEGNvbXBhbnlEb2N1bWVudHMSTQoPY29tcGFueV9hZGRyZX'
    'NzGAsgASgLMiQuaGF3a3IuY29tbW9uLnYxYWxwaGExLlBvc3RhbEFkZHJlc3NSDmNvbXBhbnlB'
    'ZGRyZXNzSgQIARACSgQIBRAGSgQIBhAHSgQICBAJSgQICRAKUgxjb21wYW55X3R5cGVSEWNvbX'
    'BhbnlfdG93bl9jb2RlUhZjb21wYW55X2FkZHJlc3NfZGV0YWlsUg5yZXBfaWRfY2FyZF9ub1IV'
    'cmVwX2lkX2NhcmRfbm9fbWFza2Vk');

@$core.Deprecated('Use kYCApplicationDescriptor instead')
const KYCApplication$json = {
  '1': 'KYCApplication',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.KYCStatus',
      '8': {},
      '10': 'status'
    },
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '10': 'etag'},
    {'1': 'product_type', '3': 14, '4': 1, '5': 9, '10': 'productType'},
    {'1': 'website_url', '3': 16, '4': 1, '5': 9, '10': 'websiteUrl'},
    {
      '1': 'individual',
      '3': 22,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.IndividualKYC',
      '9': 0,
      '10': 'individual'
    },
    {
      '1': 'business',
      '3': 23,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.BusinessKYC',
      '9': 0,
      '10': 'business'
    },
    {
      '1': 'last_rejection',
      '3': 25,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCRejection',
      '8': {},
      '10': 'lastRejection'
    },
    {
      '1': 'missing_fields',
      '3': 26,
      '4': 3,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.KYCField',
      '8': {},
      '10': 'missingFields'
    },
    {
      '1': 'missing_store_fields',
      '3': 31,
      '4': 3,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.StoreField',
      '8': {},
      '10': 'missingStoreFields'
    },
    {
      '1': 'outstanding_issues',
      '3': 32,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCIssue',
      '8': {},
      '10': 'outstandingIssues'
    },
    {
      '1': 'submit_time',
      '3': 27,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'submitTime'
    },
    {
      '1': 'decide_time',
      '3': 28,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'decideTime'
    },
    {
      '1': 'create_time',
      '3': 29,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'update_time',
      '3': 30,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
  ],
  '8': [
    {'1': 'subject'},
  ],
  '9': [
    {'1': 15, '2': 16},
    {'1': 17, '2': 18},
    {'1': 18, '2': 19},
    {'1': 24, '2': 25},
    {'1': 4, '2': 14},
    {'1': 19, '2': 22},
    {'1': 33, '2': 34},
  ],
  '10': [
    'contact_email',
    'contact_phone',
    'food_business',
    'flow',
    'real_name',
    'birth_date',
    'id_card_issue_date',
    'id_card_issue_type',
    'id_card_issue_location',
    'english_name',
    'bank_name',
    'bank_code',
    'bank_account',
    'bank_account_masked',
    'id_card_front',
    'id_card_back',
    'bank_book',
    'payout_contact_email'
  ],
};

/// Descriptor for `KYCApplication`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List kYCApplicationDescriptor = $convert.base64Decode(
    'Cg5LWUNBcHBsaWNhdGlvbhITCgJpZBgBIAEoCUID4EEIUgJpZBI9CgZzdGF0dXMYAiABKA4yIC'
    '5oYXdrci5zZWxsZXIudjFhbHBoYTEuS1lDU3RhdHVzQgPgQQNSBnN0YXR1cxISCgRldGFnGAMg'
    'ASgJUgRldGFnEiEKDHByb2R1Y3RfdHlwZRgOIAEoCVILcHJvZHVjdFR5cGUSHwoLd2Vic2l0ZV'
    '91cmwYECABKAlSCndlYnNpdGVVcmwSRgoKaW5kaXZpZHVhbBgWIAEoCzIkLmhhd2tyLnNlbGxl'
    'ci52MWFscGhhMS5JbmRpdmlkdWFsS1lDSABSCmluZGl2aWR1YWwSQAoIYnVzaW5lc3MYFyABKA'
    'syIi5oYXdrci5zZWxsZXIudjFhbHBoYTEuQnVzaW5lc3NLWUNIAFIIYnVzaW5lc3MSTwoObGFz'
    'dF9yZWplY3Rpb24YGSABKAsyIy5oYXdrci5zZWxsZXIudjFhbHBoYTEuS1lDUmVqZWN0aW9uQg'
    'PgQQNSDWxhc3RSZWplY3Rpb24SSwoObWlzc2luZ19maWVsZHMYGiADKA4yHy5oYXdrci5zZWxs'
    'ZXIudjFhbHBoYTEuS1lDRmllbGRCA+BBA1INbWlzc2luZ0ZpZWxkcxJYChRtaXNzaW5nX3N0b3'
    'JlX2ZpZWxkcxgfIAMoDjIhLmhhd2tyLnNlbGxlci52MWFscGhhMS5TdG9yZUZpZWxkQgPgQQNS'
    'Em1pc3NpbmdTdG9yZUZpZWxkcxJTChJvdXRzdGFuZGluZ19pc3N1ZXMYICADKAsyHy5oYXdrci'
    '5zZWxsZXIudjFhbHBoYTEuS1lDSXNzdWVCA+BBA1IRb3V0c3RhbmRpbmdJc3N1ZXMSQAoLc3Vi'
    'bWl0X3RpbWUYGyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCnN1Ym1pdF'
    'RpbWUSQAoLZGVjaWRlX3RpbWUYHCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPg'
    'QQNSCmRlY2lkZVRpbWUSQAoLY3JlYXRlX3RpbWUYHSABKAsyGi5nb29nbGUucHJvdG9idWYuVG'
    'ltZXN0YW1wQgPgQQNSCmNyZWF0ZVRpbWUSQAoLdXBkYXRlX3RpbWUYHiABKAsyGi5nb29nbGUu'
    'cHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCnVwZGF0ZVRpbWVCCQoHc3ViamVjdEoECA8QEEoECB'
    'EQEkoECBIQE0oECBgQGUoECAQQDkoECBMQFkoECCEQIlINY29udGFjdF9lbWFpbFINY29udGFj'
    'dF9waG9uZVINZm9vZF9idXNpbmVzc1IEZmxvd1IJcmVhbF9uYW1lUgpiaXJ0aF9kYXRlUhJpZF'
    '9jYXJkX2lzc3VlX2RhdGVSEmlkX2NhcmRfaXNzdWVfdHlwZVIWaWRfY2FyZF9pc3N1ZV9sb2Nh'
    'dGlvblIMZW5nbGlzaF9uYW1lUgliYW5rX25hbWVSCWJhbmtfY29kZVIMYmFua19hY2NvdW50Uh'
    'NiYW5rX2FjY291bnRfbWFza2VkUg1pZF9jYXJkX2Zyb250UgxpZF9jYXJkX2JhY2tSCWJhbmtf'
    'Ym9va1IUcGF5b3V0X2NvbnRhY3RfZW1haWw=');

@$core.Deprecated('Use saveKYCDraftRequestDescriptor instead')
const SaveKYCDraftRequest$json = {
  '1': 'SaveKYCDraftRequest',
  '2': [
    {
      '1': 'application',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCApplication',
      '8': {},
      '10': 'application'
    },
    {
      '1': 'update_mask',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `SaveKYCDraftRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List saveKYCDraftRequestDescriptor = $convert.base64Decode(
    'ChNTYXZlS1lDRHJhZnRSZXF1ZXN0EkwKC2FwcGxpY2F0aW9uGAEgASgLMiUuaGF3a3Iuc2VsbG'
    'VyLnYxYWxwaGExLktZQ0FwcGxpY2F0aW9uQgPgQQJSC2FwcGxpY2F0aW9uEjsKC3VwZGF0ZV9t'
    'YXNrGAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLkZpZWxkTWFza1IKdXBkYXRlTWFzaw==');

@$core.Deprecated('Use saveKYCDraftResponseDescriptor instead')
const SaveKYCDraftResponse$json = {
  '1': 'SaveKYCDraftResponse',
  '2': [
    {
      '1': 'application',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCApplication',
      '10': 'application'
    },
  ],
};

/// Descriptor for `SaveKYCDraftResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List saveKYCDraftResponseDescriptor = $convert.base64Decode(
    'ChRTYXZlS1lDRHJhZnRSZXNwb25zZRJHCgthcHBsaWNhdGlvbhgBIAEoCzIlLmhhd2tyLnNlbG'
    'xlci52MWFscGhhMS5LWUNBcHBsaWNhdGlvblILYXBwbGljYXRpb24=');

@$core.Deprecated('Use paymentAccountDescriptor instead')
const PaymentAccount$json = {
  '1': 'PaymentAccount',
  '2': [
    {
      '1': 'status',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.payment.v1alpha1.PaymentAccountStatus',
      '10': 'status'
    },
    {
      '1': 'onboarding_surface',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.payment.v1alpha1.OnboardingSurface',
      '10': 'onboardingSurface'
    },
    {'1': 'payouts_active', '3': 5, '4': 1, '5': 8, '10': 'payoutsActive'},
    {
      '1': 'fetch_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'fetchTime'
    },
  ],
};

/// Descriptor for `PaymentAccount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List paymentAccountDescriptor = $convert.base64Decode(
    'Cg5QYXltZW50QWNjb3VudBJECgZzdGF0dXMYASABKA4yLC5oYXdrci5wYXltZW50LnYxYWxwaG'
    'ExLlBheW1lbnRBY2NvdW50U3RhdHVzUgZzdGF0dXMSWAoSb25ib2FyZGluZ19zdXJmYWNlGAMg'
    'ASgOMikuaGF3a3IucGF5bWVudC52MWFscGhhMS5PbmJvYXJkaW5nU3VyZmFjZVIRb25ib2FyZG'
    'luZ1N1cmZhY2USJQoOcGF5b3V0c19hY3RpdmUYBSABKAhSDXBheW91dHNBY3RpdmUSOQoKZmV0'
    'Y2hfdGltZRgHIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWZldGNoVGltZQ==');

@$core.Deprecated('Use getMyPaymentAccountRequestDescriptor instead')
const GetMyPaymentAccountRequest$json = {
  '1': 'GetMyPaymentAccountRequest',
};

/// Descriptor for `GetMyPaymentAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyPaymentAccountRequestDescriptor =
    $convert.base64Decode('ChpHZXRNeVBheW1lbnRBY2NvdW50UmVxdWVzdA==');

@$core.Deprecated('Use getMyPaymentAccountResponseDescriptor instead')
const GetMyPaymentAccountResponse$json = {
  '1': 'GetMyPaymentAccountResponse',
  '2': [
    {
      '1': 'account',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PaymentAccount',
      '10': 'account'
    },
  ],
};

/// Descriptor for `GetMyPaymentAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyPaymentAccountResponseDescriptor =
    $convert.base64Decode(
        'ChtHZXRNeVBheW1lbnRBY2NvdW50UmVzcG9uc2USPwoHYWNjb3VudBgBIAEoCzIlLmhhd2tyLn'
        'NlbGxlci52MWFscGhhMS5QYXltZW50QWNjb3VudFIHYWNjb3VudA==');

@$core.Deprecated('Use createPaymentOnboardingSessionRequestDescriptor instead')
const CreatePaymentOnboardingSessionRequest$json = {
  '1': 'CreatePaymentOnboardingSessionRequest',
  '2': [
    {
      '1': 'destination',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.OnboardingReturnDestination',
      '8': {},
      '10': 'destination'
    },
    {'1': 'locale', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'locale'},
  ],
};

/// Descriptor for `CreatePaymentOnboardingSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPaymentOnboardingSessionRequestDescriptor =
    $convert.base64Decode(
        'CiVDcmVhdGVQYXltZW50T25ib2FyZGluZ1Nlc3Npb25SZXF1ZXN0ElkKC2Rlc3RpbmF0aW9uGA'
        'MgASgOMjIuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLk9uYm9hcmRpbmdSZXR1cm5EZXN0aW5hdGlv'
        'bkID4EECUgtkZXN0aW5hdGlvbhIbCgZsb2NhbGUYBCABKAlCA+BBAlIGbG9jYWxl');

@$core
    .Deprecated('Use createPaymentOnboardingSessionResponseDescriptor instead')
const CreatePaymentOnboardingSessionResponse$json = {
  '1': 'CreatePaymentOnboardingSessionResponse',
  '2': [
    {
      '1': 'surface',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.payment.v1alpha1.OnboardingSurface',
      '10': 'surface'
    },
    {'1': 'url', '3': 2, '4': 1, '5': 9, '10': 'url'},
    {'1': 'client_secret', '3': 3, '4': 1, '5': 9, '10': 'clientSecret'},
  ],
};

/// Descriptor for `CreatePaymentOnboardingSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPaymentOnboardingSessionResponseDescriptor =
    $convert.base64Decode(
        'CiZDcmVhdGVQYXltZW50T25ib2FyZGluZ1Nlc3Npb25SZXNwb25zZRJDCgdzdXJmYWNlGAEgAS'
        'gOMikuaGF3a3IucGF5bWVudC52MWFscGhhMS5PbmJvYXJkaW5nU3VyZmFjZVIHc3VyZmFjZRIQ'
        'CgN1cmwYAiABKAlSA3VybBIjCg1jbGllbnRfc2VjcmV0GAMgASgJUgxjbGllbnRTZWNyZXQ=');

@$core.Deprecated('Use getMyKYCRequestDescriptor instead')
const GetMyKYCRequest$json = {
  '1': 'GetMyKYCRequest',
};

/// Descriptor for `GetMyKYCRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyKYCRequestDescriptor =
    $convert.base64Decode('Cg9HZXRNeUtZQ1JlcXVlc3Q=');

@$core.Deprecated('Use getMyKYCResponseDescriptor instead')
const GetMyKYCResponse$json = {
  '1': 'GetMyKYCResponse',
  '2': [
    {
      '1': 'application',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCApplication',
      '10': 'application'
    },
  ],
};

/// Descriptor for `GetMyKYCResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyKYCResponseDescriptor = $convert.base64Decode(
    'ChBHZXRNeUtZQ1Jlc3BvbnNlEkcKC2FwcGxpY2F0aW9uGAEgASgLMiUuaGF3a3Iuc2VsbGVyLn'
    'YxYWxwaGExLktZQ0FwcGxpY2F0aW9uUgthcHBsaWNhdGlvbg==');

@$core.Deprecated('Use submitKYCRequestDescriptor instead')
const SubmitKYCRequest$json = {
  '1': 'SubmitKYCRequest',
  '2': [
    {'1': 'etag', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'accepted_agreements',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.AgreementAcceptance',
      '8': {},
      '10': 'acceptedAgreements'
    },
  ],
};

/// Descriptor for `SubmitKYCRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitKYCRequestDescriptor = $convert.base64Decode(
    'ChBTdWJtaXRLWUNSZXF1ZXN0EhcKBGV0YWcYASABKAlCA+BBAlIEZXRhZxJgChNhY2NlcHRlZF'
    '9hZ3JlZW1lbnRzGAIgAygLMiouaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkFncmVlbWVudEFjY2Vw'
    'dGFuY2VCA+BBAlISYWNjZXB0ZWRBZ3JlZW1lbnRz');

@$core.Deprecated('Use submitKYCResponseDescriptor instead')
const SubmitKYCResponse$json = {
  '1': 'SubmitKYCResponse',
  '2': [
    {
      '1': 'application',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCApplication',
      '10': 'application'
    },
  ],
};

/// Descriptor for `SubmitKYCResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitKYCResponseDescriptor = $convert.base64Decode(
    'ChFTdWJtaXRLWUNSZXNwb25zZRJHCgthcHBsaWNhdGlvbhgBIAEoCzIlLmhhd2tyLnNlbGxlci'
    '52MWFscGhhMS5LWUNBcHBsaWNhdGlvblILYXBwbGljYXRpb24=');

@$core.Deprecated('Use cancelKYCRequestDescriptor instead')
const CancelKYCRequest$json = {
  '1': 'CancelKYCRequest',
  '2': [
    {'1': 'etag', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `CancelKYCRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelKYCRequestDescriptor = $convert.base64Decode(
    'ChBDYW5jZWxLWUNSZXF1ZXN0EhcKBGV0YWcYASABKAlCA+BBAlIEZXRhZxIWCgZyZWFzb24YAi'
    'ABKAlSBnJlYXNvbg==');

@$core.Deprecated('Use cancelKYCResponseDescriptor instead')
const CancelKYCResponse$json = {
  '1': 'CancelKYCResponse',
  '2': [
    {
      '1': 'application',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.KYCApplication',
      '10': 'application'
    },
  ],
};

/// Descriptor for `CancelKYCResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelKYCResponseDescriptor = $convert.base64Decode(
    'ChFDYW5jZWxLWUNSZXNwb25zZRJHCgthcHBsaWNhdGlvbhgBIAEoCzIlLmhhd2tyLnNlbGxlci'
    '52MWFscGhhMS5LWUNBcHBsaWNhdGlvblILYXBwbGljYXRpb24=');

@$core.Deprecated('Use businessRegistrationDescriptor instead')
const BusinessRegistration$json = {
  '1': 'BusinessRegistration',
  '2': [
    {'1': 'tax_id', '3': 1, '4': 1, '5': 9, '10': 'taxId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'address', '3': 3, '4': 1, '5': 9, '10': 'address'},
    {
      '1': 'setup_date',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '10': 'setupDate'
    },
    {
      '1': 'organization_type',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'organizationType'
    },
    {
      '1': 'postal_address',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.PostalAddress',
      '10': 'postalAddress'
    },
  ],
  '9': [
    {'1': 4, '2': 5},
    {'1': 5, '2': 6},
  ],
  '10': ['town_code', 'address_detail'],
};

/// Descriptor for `BusinessRegistration`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List businessRegistrationDescriptor = $convert.base64Decode(
    'ChRCdXNpbmVzc1JlZ2lzdHJhdGlvbhIVCgZ0YXhfaWQYASABKAlSBXRheElkEhIKBG5hbWUYAi'
    'ABKAlSBG5hbWUSGAoHYWRkcmVzcxgDIAEoCVIHYWRkcmVzcxIwCgpzZXR1cF9kYXRlGAYgASgL'
    'MhEuZ29vZ2xlLnR5cGUuRGF0ZVIJc2V0dXBEYXRlEisKEW9yZ2FuaXphdGlvbl90eXBlGAcgAS'
    'gJUhBvcmdhbml6YXRpb25UeXBlEksKDnBvc3RhbF9hZGRyZXNzGAggASgLMiQuaGF3a3IuY29t'
    'bW9uLnYxYWxwaGExLlBvc3RhbEFkZHJlc3NSDXBvc3RhbEFkZHJlc3NKBAgEEAVKBAgFEAZSCX'
    'Rvd25fY29kZVIOYWRkcmVzc19kZXRhaWw=');

@$core.Deprecated('Use lookupBusinessRegistrationRequestDescriptor instead')
const LookupBusinessRegistrationRequest$json = {
  '1': 'LookupBusinessRegistrationRequest',
  '2': [
    {'1': 'tax_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'taxId'},
  ],
};

/// Descriptor for `LookupBusinessRegistrationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lookupBusinessRegistrationRequestDescriptor =
    $convert.base64Decode(
        'CiFMb29rdXBCdXNpbmVzc1JlZ2lzdHJhdGlvblJlcXVlc3QSGgoGdGF4X2lkGAEgASgJQgPgQQ'
        'JSBXRheElk');

@$core.Deprecated('Use lookupBusinessRegistrationResponseDescriptor instead')
const LookupBusinessRegistrationResponse$json = {
  '1': 'LookupBusinessRegistrationResponse',
  '2': [
    {
      '1': 'registration',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.BusinessRegistration',
      '10': 'registration'
    },
  ],
};

/// Descriptor for `LookupBusinessRegistrationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lookupBusinessRegistrationResponseDescriptor =
    $convert.base64Decode(
        'CiJMb29rdXBCdXNpbmVzc1JlZ2lzdHJhdGlvblJlc3BvbnNlEk8KDHJlZ2lzdHJhdGlvbhgBIA'
        'EoCzIrLmhhd2tyLnNlbGxlci52MWFscGhhMS5CdXNpbmVzc1JlZ2lzdHJhdGlvblIMcmVnaXN0'
        'cmF0aW9u');

@$core.Deprecated('Use contentRefDescriptor instead')
const ContentRef$json = {
  '1': 'ContentRef',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.ContentKind',
      '8': {},
      '10': 'kind'
    },
    {'1': 'id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'id'},
  ],
};

/// Descriptor for `ContentRef`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List contentRefDescriptor = $convert.base64Decode(
    'CgpDb250ZW50UmVmEjsKBGtpbmQYASABKA4yIi5oYXdrci5zZWxsZXIudjFhbHBoYTEuQ29udG'
    'VudEtpbmRCA+BBAlIEa2luZBITCgJpZBgCIAEoCUID4EECUgJpZA==');

@$core.Deprecated('Use remediationUnavailabilityDescriptor instead')
const RemediationUnavailability$json = {
  '1': 'RemediationUnavailability',
  '2': [
    {
      '1': 'reason',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.RemediationUnavailableReason',
      '8': {},
      '10': 'reason'
    },
    {
      '1': 'message',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.rpc.LocalizedMessage',
      '8': {},
      '10': 'message'
    },
  ],
};

/// Descriptor for `RemediationUnavailability`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List remediationUnavailabilityDescriptor = $convert.base64Decode(
    'ChlSZW1lZGlhdGlvblVuYXZhaWxhYmlsaXR5ElAKBnJlYXNvbhgBIAEoDjIzLmhhd2tyLnNlbG'
    'xlci52MWFscGhhMS5SZW1lZGlhdGlvblVuYXZhaWxhYmxlUmVhc29uQgPgQQNSBnJlYXNvbhI7'
    'CgdtZXNzYWdlGAIgASgLMhwuZ29vZ2xlLnJwYy5Mb2NhbGl6ZWRNZXNzYWdlQgPgQQNSB21lc3'
    'NhZ2U=');

@$core.Deprecated('Use appealUnavailabilityDescriptor instead')
const AppealUnavailability$json = {
  '1': 'AppealUnavailability',
  '2': [
    {
      '1': 'reason',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.AppealUnavailableReason',
      '8': {},
      '10': 'reason'
    },
    {
      '1': 'message',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.rpc.LocalizedMessage',
      '8': {},
      '10': 'message'
    },
  ],
};

/// Descriptor for `AppealUnavailability`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List appealUnavailabilityDescriptor = $convert.base64Decode(
    'ChRBcHBlYWxVbmF2YWlsYWJpbGl0eRJLCgZyZWFzb24YASABKA4yLi5oYXdrci5zZWxsZXIudj'
    'FhbHBoYTEuQXBwZWFsVW5hdmFpbGFibGVSZWFzb25CA+BBA1IGcmVhc29uEjsKB21lc3NhZ2UY'
    'AiABKAsyHC5nb29nbGUucnBjLkxvY2FsaXplZE1lc3NhZ2VCA+BBA1IHbWVzc2FnZQ==');

@$core.Deprecated('Use measureFindingDescriptor instead')
const MeasureFinding$json = {
  '1': 'MeasureFinding',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'category',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '10': 'category'
    },
    {
      '1': 'severity',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.ViolationSeverity',
      '10': 'severity'
    },
    {
      '1': 'create_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
  ],
};

/// Descriptor for `MeasureFinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List measureFindingDescriptor = $convert.base64Decode(
    'Cg5NZWFzdXJlRmluZGluZxITCgJpZBgBIAEoCUID4EEIUgJpZBJBCghjYXRlZ29yeRgCIAEoDj'
    'IlLmhhd2tyLmNvbW1vbi52MWFscGhhMS5Qb2xpY3lDYXRlZ29yeVIIY2F0ZWdvcnkSRAoIc2V2'
    'ZXJpdHkYAyABKA4yKC5oYXdrci5zZWxsZXIudjFhbHBoYTEuVmlvbGF0aW9uU2V2ZXJpdHlSCH'
    'NldmVyaXR5EjsKC2NyZWF0ZV90aW1lGAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFt'
    'cFIKY3JlYXRlVGltZQ==');

@$core.Deprecated('Use measurePeriodDescriptor instead')
const MeasurePeriod$json = {
  '1': 'MeasurePeriod',
  '2': [
    {
      '1': 'start_time',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'startTime'
    },
    {
      '1': 'end_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'endTime'
    },
  ],
};

/// Descriptor for `MeasurePeriod`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List measurePeriodDescriptor = $convert.base64Decode(
    'Cg1NZWFzdXJlUGVyaW9kEj4KCnN0YXJ0X3RpbWUYASABKAsyGi5nb29nbGUucHJvdG9idWYuVG'
    'ltZXN0YW1wQgPgQQNSCXN0YXJ0VGltZRI6CghlbmRfdGltZRgCIAEoCzIaLmdvb2dsZS5wcm90'
    'b2J1Zi5UaW1lc3RhbXBCA+BBA1IHZW5kVGltZQ==');

@$core.Deprecated('Use measureRequestDescriptor instead')
const MeasureRequest$json = {
  '1': 'MeasureRequest',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'state',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureRequestState',
      '8': {},
      '10': 'state'
    },
    {'1': 'reason', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'reason'},
    {
      '1': 'submit_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'submitTime'
    },
    {
      '1': 'decision_reason',
      '3': 5,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'decisionReason'
    },
    {
      '1': 'decide_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'decideTime'
    },
    {
      '1': 'close_cause',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureRequestCloseCause',
      '8': {},
      '10': 'closeCause'
    },
    {
      '1': 'kind',
      '3': 8,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureRequestKind',
      '8': {},
      '10': 'kind'
    },
  ],
};

/// Descriptor for `MeasureRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List measureRequestDescriptor = $convert.base64Decode(
    'Cg5NZWFzdXJlUmVxdWVzdBITCgJpZBgBIAEoCUID4EEIUgJpZBJFCgVzdGF0ZRgCIAEoDjIqLm'
    'hhd2tyLnNlbGxlci52MWFscGhhMS5NZWFzdXJlUmVxdWVzdFN0YXRlQgPgQQNSBXN0YXRlEhsK'
    'BnJlYXNvbhgDIAEoCUID4EEDUgZyZWFzb24SQAoLc3VibWl0X3RpbWUYBCABKAsyGi5nb29nbG'
    'UucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCnN1Ym1pdFRpbWUSLAoPZGVjaXNpb25fcmVhc29u'
    'GAUgASgJQgPgQQNSDmRlY2lzaW9uUmVhc29uEkAKC2RlY2lkZV90aW1lGAYgASgLMhouZ29vZ2'
    'xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpkZWNpZGVUaW1lElUKC2Nsb3NlX2NhdXNlGAcg'
    'ASgOMi8uaGF3a3Iuc2VsbGVyLnYxYWxwaGExLk1lYXN1cmVSZXF1ZXN0Q2xvc2VDYXVzZUID4E'
    'EDUgpjbG9zZUNhdXNlEkIKBGtpbmQYCCABKA4yKS5oYXdrci5zZWxsZXIudjFhbHBoYTEuTWVh'
    'c3VyZVJlcXVlc3RLaW5kQgPgQQNSBGtpbmQ=');

@$core.Deprecated('Use measureDescriptor instead')
const Measure$json = {
  '1': 'Measure',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureKind',
      '8': {},
      '10': 'kind'
    },
    {
      '1': 'basis',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureBasis',
      '8': {},
      '10': 'basis'
    },
    {
      '1': 'findings',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.MeasureFinding',
      '8': {},
      '10': 'findings'
    },
    {
      '1': 'risk_ground',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.RiskGround',
      '8': {},
      '10': 'riskGround'
    },
    {'1': 'statement', '3': 6, '4': 1, '5': 9, '8': {}, '10': 'statement'},
    {
      '1': 'state',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureState',
      '8': {},
      '10': 'state'
    },
    {
      '1': 'period',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.MeasurePeriod',
      '8': {},
      '10': 'period'
    },
    {
      '1': 'create_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {'1': 'etag', '3': 11, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {'1': 'can_appeal', '3': 12, '4': 1, '5': 8, '8': {}, '10': 'canAppeal'},
    {
      '1': 'revoke_reason',
      '3': 13,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.RevokeReason',
      '8': {},
      '10': 'revokeReason'
    },
    {
      '1': 'can_request_remediation',
      '3': 15,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'canRequestRemediation'
    },
    {
      '1': 'next_remediation_time',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'nextRemediationTime'
    },
    {
      '1': 'rejection_cooldown',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '8': {},
      '10': 'rejectionCooldown'
    },
    {
      '1': 'requests',
      '3': 18,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.MeasureRequest',
      '8': {},
      '10': 'requests'
    },
    {
      '1': 'remediation_unavailabilities',
      '3': 23,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.RemediationUnavailability',
      '8': {},
      '10': 'remediationUnavailabilities'
    },
    {
      '1': 'termination_ground',
      '3': 20,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.TerminationGround',
      '8': {},
      '10': 'terminationGround'
    },
    {
      '1': 'appeal_deadline_time',
      '3': 21,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'appealDeadlineTime'
    },
    {
      '1': 'appeal_unavailabilities',
      '3': 24,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.AppealUnavailability',
      '8': {},
      '10': 'appealUnavailabilities'
    },
    {
      '1': 'contents',
      '3': 25,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.ContentRef',
      '8': {},
      '10': 'contents'
    },
  ],
  '9': [
    {'1': 9, '2': 10},
    {'1': 14, '2': 15},
    {'1': 19, '2': 20},
    {'1': 22, '2': 23},
  ],
  '10': [
    'appeal',
    'remediation_requests',
    'remediation_unavailable_reasons',
    'appeal_unavailable_reasons'
  ],
};

/// Descriptor for `Measure`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List measureDescriptor = $convert.base64Decode(
    'CgdNZWFzdXJlEhMKAmlkGAEgASgJQgPgQQhSAmlkEjsKBGtpbmQYAiABKA4yIi5oYXdrci5zZW'
    'xsZXIudjFhbHBoYTEuTWVhc3VyZUtpbmRCA+BBA1IEa2luZBI+CgViYXNpcxgDIAEoDjIjLmhh'
    'd2tyLnNlbGxlci52MWFscGhhMS5NZWFzdXJlQmFzaXNCA+BBA1IFYmFzaXMSRgoIZmluZGluZ3'
    'MYBCADKAsyJS5oYXdrci5zZWxsZXIudjFhbHBoYTEuTWVhc3VyZUZpbmRpbmdCA+BBA1IIZmlu'
    'ZGluZ3MSRwoLcmlza19ncm91bmQYBSABKA4yIS5oYXdrci5zZWxsZXIudjFhbHBoYTEuUmlza0'
    'dyb3VuZEID4EEDUgpyaXNrR3JvdW5kEiEKCXN0YXRlbWVudBgGIAEoCUID4EEDUglzdGF0ZW1l'
    'bnQSPgoFc3RhdGUYByABKA4yIy5oYXdrci5zZWxsZXIudjFhbHBoYTEuTWVhc3VyZVN0YXRlQg'
    'PgQQNSBXN0YXRlEkEKBnBlcmlvZBgIIAEoCzIkLmhhd2tyLnNlbGxlci52MWFscGhhMS5NZWFz'
    'dXJlUGVyaW9kQgPgQQNSBnBlcmlvZBJACgtjcmVhdGVfdGltZRgKIAEoCzIaLmdvb2dsZS5wcm'
    '90b2J1Zi5UaW1lc3RhbXBCA+BBA1IKY3JlYXRlVGltZRIXCgRldGFnGAsgASgJQgPgQQNSBGV0'
    'YWcSIgoKY2FuX2FwcGVhbBgMIAEoCEID4EEDUgljYW5BcHBlYWwSTQoNcmV2b2tlX3JlYXNvbh'
    'gNIAEoDjIjLmhhd2tyLnNlbGxlci52MWFscGhhMS5SZXZva2VSZWFzb25CA+BBA1IMcmV2b2tl'
    'UmVhc29uEjsKF2Nhbl9yZXF1ZXN0X3JlbWVkaWF0aW9uGA8gASgIQgPgQQNSFWNhblJlcXVlc3'
    'RSZW1lZGlhdGlvbhJTChVuZXh0X3JlbWVkaWF0aW9uX3RpbWUYECABKAsyGi5nb29nbGUucHJv'
    'dG9idWYuVGltZXN0YW1wQgPgQQNSE25leHRSZW1lZGlhdGlvblRpbWUSTQoScmVqZWN0aW9uX2'
    'Nvb2xkb3duGBEgASgLMhkuZ29vZ2xlLnByb3RvYnVmLkR1cmF0aW9uQgPgQQNSEXJlamVjdGlv'
    'bkNvb2xkb3duEkYKCHJlcXVlc3RzGBIgAygLMiUuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLk1lYX'
    'N1cmVSZXF1ZXN0QgPgQQNSCHJlcXVlc3RzEngKHHJlbWVkaWF0aW9uX3VuYXZhaWxhYmlsaXRp'
    'ZXMYFyADKAsyMC5oYXdrci5zZWxsZXIudjFhbHBoYTEuUmVtZWRpYXRpb25VbmF2YWlsYWJpbG'
    'l0eUID4EEDUhtyZW1lZGlhdGlvblVuYXZhaWxhYmlsaXRpZXMSXAoSdGVybWluYXRpb25fZ3Jv'
    'dW5kGBQgASgOMiguaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlRlcm1pbmF0aW9uR3JvdW5kQgPgQQ'
    'NSEXRlcm1pbmF0aW9uR3JvdW5kElEKFGFwcGVhbF9kZWFkbGluZV90aW1lGBUgASgLMhouZ29v'
    'Z2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUhJhcHBlYWxEZWFkbGluZVRpbWUSaQoXYXBwZW'
    'FsX3VuYXZhaWxhYmlsaXRpZXMYGCADKAsyKy5oYXdrci5zZWxsZXIudjFhbHBoYTEuQXBwZWFs'
    'VW5hdmFpbGFiaWxpdHlCA+BBA1IWYXBwZWFsVW5hdmFpbGFiaWxpdGllcxJCCghjb250ZW50cx'
    'gZIAMoCzIhLmhhd2tyLnNlbGxlci52MWFscGhhMS5Db250ZW50UmVmQgPgQQNSCGNvbnRlbnRz'
    'SgQICRAKSgQIDhAPSgQIExAUSgQIFhAXUgZhcHBlYWxSFHJlbWVkaWF0aW9uX3JlcXVlc3RzUh'
    '9yZW1lZGlhdGlvbl91bmF2YWlsYWJsZV9yZWFzb25zUhphcHBlYWxfdW5hdmFpbGFibGVfcmVh'
    'c29ucw==');

@$core.Deprecated('Use listMyMeasuresRequestDescriptor instead')
const ListMyMeasuresRequest$json = {
  '1': 'ListMyMeasuresRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'state',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.MeasureState',
      '10': 'state'
    },
  ],
};

/// Descriptor for `ListMyMeasuresRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyMeasuresRequestDescriptor = $convert.base64Decode(
    'ChVMaXN0TXlNZWFzdXJlc1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZRIdCg'
    'pwYWdlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4SOQoFc3RhdGUYAyABKA4yIy5oYXdrci5zZWxs'
    'ZXIudjFhbHBoYTEuTWVhc3VyZVN0YXRlUgVzdGF0ZQ==');

@$core.Deprecated('Use listMyMeasuresResponseDescriptor instead')
const ListMyMeasuresResponse$json = {
  '1': 'ListMyMeasuresResponse',
  '2': [
    {
      '1': 'measures',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Measure',
      '10': 'measures'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyMeasuresResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyMeasuresResponseDescriptor = $convert.base64Decode(
    'ChZMaXN0TXlNZWFzdXJlc1Jlc3BvbnNlEjoKCG1lYXN1cmVzGAEgAygLMh4uaGF3a3Iuc2VsbG'
    'VyLnYxYWxwaGExLk1lYXN1cmVSCG1lYXN1cmVzEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVIN'
    'bmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use getMyMeasureRequestDescriptor instead')
const GetMyMeasureRequest$json = {
  '1': 'GetMyMeasureRequest',
  '2': [
    {'1': 'measure_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'measureId'},
  ],
};

/// Descriptor for `GetMyMeasureRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyMeasureRequestDescriptor = $convert.base64Decode(
    'ChNHZXRNeU1lYXN1cmVSZXF1ZXN0EiIKCm1lYXN1cmVfaWQYASABKAlCA+BBAlIJbWVhc3VyZU'
    'lk');

@$core.Deprecated('Use getMyMeasureResponseDescriptor instead')
const GetMyMeasureResponse$json = {
  '1': 'GetMyMeasureResponse',
  '2': [
    {
      '1': 'measure',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Measure',
      '10': 'measure'
    },
  ],
};

/// Descriptor for `GetMyMeasureResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyMeasureResponseDescriptor = $convert.base64Decode(
    'ChRHZXRNeU1lYXN1cmVSZXNwb25zZRI4CgdtZWFzdXJlGAEgASgLMh4uaGF3a3Iuc2VsbGVyLn'
    'YxYWxwaGExLk1lYXN1cmVSB21lYXN1cmU=');

@$core.Deprecated('Use submitAppealRequestDescriptor instead')
const SubmitAppealRequest$json = {
  '1': 'SubmitAppealRequest',
  '2': [
    {'1': 'measure_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'measureId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'reason'},
    {
      '1': 'evidence_staged_tokens',
      '3': 3,
      '4': 3,
      '5': 9,
      '10': 'evidenceStagedTokens'
    },
    {'1': 'etag', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `SubmitAppealRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitAppealRequestDescriptor = $convert.base64Decode(
    'ChNTdWJtaXRBcHBlYWxSZXF1ZXN0EiIKCm1lYXN1cmVfaWQYASABKAlCA+BBAlIJbWVhc3VyZU'
    'lkEhsKBnJlYXNvbhgCIAEoCUID4EECUgZyZWFzb24SNAoWZXZpZGVuY2Vfc3RhZ2VkX3Rva2Vu'
    'cxgDIAMoCVIUZXZpZGVuY2VTdGFnZWRUb2tlbnMSFwoEZXRhZxgEIAEoCUID4EECUgRldGFn');

@$core.Deprecated('Use submitAppealResponseDescriptor instead')
const SubmitAppealResponse$json = {
  '1': 'SubmitAppealResponse',
  '2': [
    {
      '1': 'measure',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Measure',
      '10': 'measure'
    },
  ],
};

/// Descriptor for `SubmitAppealResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitAppealResponseDescriptor = $convert.base64Decode(
    'ChRTdWJtaXRBcHBlYWxSZXNwb25zZRI4CgdtZWFzdXJlGAEgASgLMh4uaGF3a3Iuc2VsbGVyLn'
    'YxYWxwaGExLk1lYXN1cmVSB21lYXN1cmU=');

@$core.Deprecated('Use requestRemediationRequestDescriptor instead')
const RequestRemediationRequest$json = {
  '1': 'RequestRemediationRequest',
  '2': [
    {'1': 'measure_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'measureId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'reason'},
    {
      '1': 'evidence_staged_tokens',
      '3': 3,
      '4': 3,
      '5': 9,
      '10': 'evidenceStagedTokens'
    },
    {'1': 'etag', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `RequestRemediationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List requestRemediationRequestDescriptor = $convert.base64Decode(
    'ChlSZXF1ZXN0UmVtZWRpYXRpb25SZXF1ZXN0EiIKCm1lYXN1cmVfaWQYASABKAlCA+BBAlIJbW'
    'Vhc3VyZUlkEhsKBnJlYXNvbhgCIAEoCUID4EECUgZyZWFzb24SNAoWZXZpZGVuY2Vfc3RhZ2Vk'
    'X3Rva2VucxgDIAMoCVIUZXZpZGVuY2VTdGFnZWRUb2tlbnMSFwoEZXRhZxgEIAEoCUID4EECUg'
    'RldGFn');

@$core.Deprecated('Use requestRemediationResponseDescriptor instead')
const RequestRemediationResponse$json = {
  '1': 'RequestRemediationResponse',
  '2': [
    {
      '1': 'measure',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.Measure',
      '10': 'measure'
    },
  ],
};

/// Descriptor for `RequestRemediationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List requestRemediationResponseDescriptor =
    $convert.base64Decode(
        'ChpSZXF1ZXN0UmVtZWRpYXRpb25SZXNwb25zZRI4CgdtZWFzdXJlGAEgASgLMh4uaGF3a3Iuc2'
        'VsbGVyLnYxYWxwaGExLk1lYXN1cmVSB21lYXN1cmU=');

@$core.Deprecated('Use publicStoreDescriptor instead')
const PublicStore$json = {
  '1': 'PublicStore',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'store_code', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'storeCode'},
    {'1': 'store_name', '3': 3, '4': 1, '5': 9, '10': 'storeName'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {'1': 'town_code', '3': 5, '4': 1, '5': 9, '10': 'townCode'},
    {'1': 'prize', '3': 6, '4': 1, '5': 9, '10': 'prize'},
    {'1': 'website_url', '3': 7, '4': 1, '5': 9, '10': 'websiteUrl'},
    {
      '1': 'logo',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'logo'
    },
    {
      '1': 'cover_image',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'coverImage'
    },
    {
      '1': 'update_time',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
  ],
  '9': [
    {'1': 10, '2': 11},
    {'1': 11, '2': 12},
    {'1': 12, '2': 13},
  ],
  '10': ['seo_title', 'seo_description', 'food_business_registration_number'],
};

/// Descriptor for `PublicStore`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publicStoreDescriptor = $convert.base64Decode(
    'CgtQdWJsaWNTdG9yZRITCgJpZBgBIAEoCUID4EEIUgJpZBIiCgpzdG9yZV9jb2RlGAIgASgJQg'
    'PgQQNSCXN0b3JlQ29kZRIdCgpzdG9yZV9uYW1lGAMgASgJUglzdG9yZU5hbWUSIAoLZGVzY3Jp'
    'cHRpb24YBCABKAlSC2Rlc2NyaXB0aW9uEhsKCXRvd25fY29kZRgFIAEoCVIIdG93bkNvZGUSFA'
    'oFcHJpemUYBiABKAlSBXByaXplEh8KC3dlYnNpdGVfdXJsGAcgASgJUgp3ZWJzaXRlVXJsEjAK'
    'BGxvZ28YCCABKAsyHC5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VSBGxvZ28SPQoLY292ZX'
    'JfaW1hZ2UYCSABKAsyHC5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VSCmNvdmVySW1hZ2US'
    'QAoLdXBkYXRlX3RpbWUYDSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCn'
    'VwZGF0ZVRpbWVKBAgKEAtKBAgLEAxKBAgMEA1SCXNlb190aXRsZVIPc2VvX2Rlc2NyaXB0aW9u'
    'UiFmb29kX2J1c2luZXNzX3JlZ2lzdHJhdGlvbl9udW1iZXI=');

@$core.Deprecated('Use storeStatsDescriptor instead')
const StoreStats$json = {
  '1': 'StoreStats',
  '2': [
    {
      '1': 'follower_count',
      '3': 1,
      '4': 1,
      '5': 5,
      '9': 0,
      '10': 'followerCount',
      '17': true
    },
    {
      '1': 'review_count',
      '3': 2,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'reviewCount',
      '17': true
    },
    {
      '1': 'rating_average',
      '3': 3,
      '4': 1,
      '5': 2,
      '9': 2,
      '10': 'ratingAverage',
      '17': true
    },
  ],
  '8': [
    {'1': '_follower_count'},
    {'1': '_review_count'},
    {'1': '_rating_average'},
  ],
};

/// Descriptor for `StoreStats`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storeStatsDescriptor = $convert.base64Decode(
    'CgpTdG9yZVN0YXRzEioKDmZvbGxvd2VyX2NvdW50GAEgASgFSABSDWZvbGxvd2VyQ291bnSIAQ'
    'ESJgoMcmV2aWV3X2NvdW50GAIgASgFSAFSC3Jldmlld0NvdW50iAEBEioKDnJhdGluZ19hdmVy'
    'YWdlGAMgASgCSAJSDXJhdGluZ0F2ZXJhZ2WIAQFCEQoPX2ZvbGxvd2VyX2NvdW50Qg8KDV9yZX'
    'ZpZXdfY291bnRCEQoPX3JhdGluZ19hdmVyYWdl');

@$core.Deprecated('Use getStoreRequestDescriptor instead')
const GetStoreRequest$json = {
  '1': 'GetStoreRequest',
  '2': [
    {'1': 'store_code', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeCode'},
  ],
};

/// Descriptor for `GetStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreRequestDescriptor = $convert.base64Decode(
    'Cg9HZXRTdG9yZVJlcXVlc3QSIgoKc3RvcmVfY29kZRgBIAEoCUID4EECUglzdG9yZUNvZGU=');

@$core.Deprecated('Use getStoreResponseDescriptor instead')
const GetStoreResponse$json = {
  '1': 'GetStoreResponse',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PublicStore',
      '10': 'store'
    },
    {
      '1': 'stats',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.StoreStats',
      '10': 'stats'
    },
  ],
};

/// Descriptor for `GetStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreResponseDescriptor = $convert.base64Decode(
    'ChBHZXRTdG9yZVJlc3BvbnNlEjgKBXN0b3JlGAEgASgLMiIuaGF3a3Iuc2VsbGVyLnYxYWxwaG'
    'ExLlB1YmxpY1N0b3JlUgVzdG9yZRI3CgVzdGF0cxgCIAEoCzIhLmhhd2tyLnNlbGxlci52MWFs'
    'cGhhMS5TdG9yZVN0YXRzUgVzdGF0cw==');

@$core.Deprecated('Use listStoresRequestDescriptor instead')
const ListStoresRequest$json = {
  '1': 'ListStoresRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListStoresRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoresRequestDescriptor = $convert.base64Decode(
    'ChFMaXN0U3RvcmVzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCnBhZ2'
    'VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listStoresResponseDescriptor instead')
const ListStoresResponse$json = {
  '1': 'ListStoresResponse',
  '2': [
    {
      '1': 'stores',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.PublicStore',
      '10': 'stores'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListStoresResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoresResponseDescriptor = $convert.base64Decode(
    'ChJMaXN0U3RvcmVzUmVzcG9uc2USOgoGc3RvcmVzGAEgAygLMiIuaGF3a3Iuc2VsbGVyLnYxYW'
    'xwaGExLlB1YmxpY1N0b3JlUgZzdG9yZXMSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJUg1uZXh0'
    'UGFnZVRva2Vu');

@$core.Deprecated('Use batchGetStoresRequestDescriptor instead')
const BatchGetStoresRequest$json = {
  '1': 'BatchGetStoresRequest',
  '2': [
    {'1': 'ids', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'ids'},
  ],
};

/// Descriptor for `BatchGetStoresRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchGetStoresRequestDescriptor =
    $convert.base64Decode(
        'ChVCYXRjaEdldFN0b3Jlc1JlcXVlc3QSFQoDaWRzGAEgAygJQgPgQQJSA2lkcw==');

@$core.Deprecated('Use batchGetStoresResponseDescriptor instead')
const BatchGetStoresResponse$json = {
  '1': 'BatchGetStoresResponse',
  '2': [
    {
      '1': 'stores',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'stores'
    },
  ],
};

/// Descriptor for `BatchGetStoresResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchGetStoresResponseDescriptor =
    $convert.base64Decode(
        'ChZCYXRjaEdldFN0b3Jlc1Jlc3BvbnNlEjgKBnN0b3JlcxgBIAMoCzIgLmhhd2tyLmNvbW1vbi'
        '52MWFscGhhMS5TdG9yZUNhcmRSBnN0b3Jlcw==');

@$core.Deprecated('Use getSellerAgreementsRequestDescriptor instead')
const GetSellerAgreementsRequest$json = {
  '1': 'GetSellerAgreementsRequest',
};

/// Descriptor for `GetSellerAgreementsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSellerAgreementsRequestDescriptor =
    $convert.base64Decode('ChpHZXRTZWxsZXJBZ3JlZW1lbnRzUmVxdWVzdA==');

@$core.Deprecated('Use getSellerAgreementsResponseDescriptor instead')
const GetSellerAgreementsResponse$json = {
  '1': 'GetSellerAgreementsResponse',
  '2': [
    {
      '1': 'agreements',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.SellerAgreement',
      '10': 'agreements'
    },
  ],
};

/// Descriptor for `GetSellerAgreementsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSellerAgreementsResponseDescriptor =
    $convert.base64Decode(
        'ChtHZXRTZWxsZXJBZ3JlZW1lbnRzUmVzcG9uc2USRgoKYWdyZWVtZW50cxgBIAMoCzImLmhhd2'
        'tyLnNlbGxlci52MWFscGhhMS5TZWxsZXJBZ3JlZW1lbnRSCmFncmVlbWVudHM=');

@$core.Deprecated('Use recordProductViewRequestDescriptor instead')
const RecordProductViewRequest$json = {
  '1': 'RecordProductViewRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `RecordProductViewRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordProductViewRequestDescriptor =
    $convert.base64Decode(
        'ChhSZWNvcmRQcm9kdWN0Vmlld1JlcXVlc3QSIgoKcHJvZHVjdF9pZBgBIAEoCUID4EECUglwcm'
        '9kdWN0SWQ=');

@$core.Deprecated('Use recordProductViewResponseDescriptor instead')
const RecordProductViewResponse$json = {
  '1': 'RecordProductViewResponse',
};

/// Descriptor for `RecordProductViewResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordProductViewResponseDescriptor =
    $convert.base64Decode('ChlSZWNvcmRQcm9kdWN0Vmlld1Jlc3BvbnNl');

const $core.Map<$core.String, $core.dynamic> SellerAppServiceBase$json = {
  '1': 'SellerAppService',
  '2': [
    {
      '1': 'ApplyStore',
      '2': '.hawkr.seller.v1alpha1.ApplyStoreRequest',
      '3': '.hawkr.seller.v1alpha1.ApplyStoreResponse'
    },
    {
      '1': 'GetMyStoreOpening',
      '2': '.hawkr.seller.v1alpha1.GetMyStoreOpeningRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyStoreOpeningResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetMyStore',
      '2': '.hawkr.seller.v1alpha1.GetMyStoreRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyStoreResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetMyOnboarding',
      '2': '.hawkr.seller.v1alpha1.GetMyOnboardingRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyOnboardingResponse',
      '4': {'34': 1},
    },
    {
      '1': 'RefreshMyPaymentAccount',
      '2': '.hawkr.seller.v1alpha1.RefreshMyPaymentAccountRequest',
      '3': '.hawkr.seller.v1alpha1.RefreshMyPaymentAccountResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UpdateMyStore',
      '2': '.hawkr.seller.v1alpha1.UpdateMyStoreRequest',
      '3': '.hawkr.seller.v1alpha1.UpdateMyStoreResponse'
    },
    {
      '1': 'CompleteMyStoreOnboarding',
      '2': '.hawkr.seller.v1alpha1.CompleteMyStoreOnboardingRequest',
      '3': '.hawkr.seller.v1alpha1.CompleteMyStoreOnboardingResponse',
      '4': {'34': 2},
    },
    {
      '1': 'ListMyFollowers',
      '2': '.hawkr.seller.v1alpha1.ListMyFollowersRequest',
      '3': '.hawkr.seller.v1alpha1.ListMyFollowersResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetMyDashboard',
      '2': '.hawkr.seller.v1alpha1.GetMyDashboardRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyDashboardResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CloseMyStore',
      '2': '.hawkr.seller.v1alpha1.CloseMyStoreRequest',
      '3': '.hawkr.seller.v1alpha1.CloseMyStoreResponse'
    },
    {
      '1': 'SaveKYCDraft',
      '2': '.hawkr.seller.v1alpha1.SaveKYCDraftRequest',
      '3': '.hawkr.seller.v1alpha1.SaveKYCDraftResponse'
    },
    {
      '1': 'GetMyKYC',
      '2': '.hawkr.seller.v1alpha1.GetMyKYCRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyKYCResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SubmitKYC',
      '2': '.hawkr.seller.v1alpha1.SubmitKYCRequest',
      '3': '.hawkr.seller.v1alpha1.SubmitKYCResponse'
    },
    {
      '1': 'CancelKYC',
      '2': '.hawkr.seller.v1alpha1.CancelKYCRequest',
      '3': '.hawkr.seller.v1alpha1.CancelKYCResponse'
    },
    {
      '1': 'GetMyPaymentAccount',
      '2': '.hawkr.seller.v1alpha1.GetMyPaymentAccountRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyPaymentAccountResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CreatePaymentOnboardingSession',
      '2': '.hawkr.seller.v1alpha1.CreatePaymentOnboardingSessionRequest',
      '3': '.hawkr.seller.v1alpha1.CreatePaymentOnboardingSessionResponse'
    },
    {
      '1': 'LookupBusinessRegistration',
      '2': '.hawkr.seller.v1alpha1.LookupBusinessRegistrationRequest',
      '3': '.hawkr.seller.v1alpha1.LookupBusinessRegistrationResponse'
    },
    {
      '1': 'ListMyMeasures',
      '2': '.hawkr.seller.v1alpha1.ListMyMeasuresRequest',
      '3': '.hawkr.seller.v1alpha1.ListMyMeasuresResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetMyMeasure',
      '2': '.hawkr.seller.v1alpha1.GetMyMeasureRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyMeasureResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SubmitAppeal',
      '2': '.hawkr.seller.v1alpha1.SubmitAppealRequest',
      '3': '.hawkr.seller.v1alpha1.SubmitAppealResponse'
    },
    {
      '1': 'RequestRemediation',
      '2': '.hawkr.seller.v1alpha1.RequestRemediationRequest',
      '3': '.hawkr.seller.v1alpha1.RequestRemediationResponse'
    },
  ],
};

@$core.Deprecated('Use sellerAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerAppServiceBase$messageJson = {
  '.hawkr.seller.v1alpha1.ApplyStoreRequest': ApplyStoreRequest$json,
  '.hawkr.common.v1alpha1.PostalAddress': $2.PostalAddress$json,
  '.hawkr.seller.v1alpha1.AgreementAcceptance': AgreementAcceptance$json,
  '.hawkr.seller.v1alpha1.ApplyStoreResponse': ApplyStoreResponse$json,
  '.hawkr.seller.v1alpha1.Store': Store$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.seller.v1alpha1.GetMyStoreOpeningRequest':
      GetMyStoreOpeningRequest$json,
  '.hawkr.seller.v1alpha1.GetMyStoreOpeningResponse':
      GetMyStoreOpeningResponse$json,
  '.hawkr.seller.v1alpha1.StoreOpeningUnavailability':
      StoreOpeningUnavailability$json,
  '.google.rpc.LocalizedMessage': $3.LocalizedMessage$json,
  '.hawkr.seller.v1alpha1.GetMyStoreRequest': GetMyStoreRequest$json,
  '.hawkr.seller.v1alpha1.GetMyStoreResponse': GetMyStoreResponse$json,
  '.hawkr.seller.v1alpha1.CapabilityStatus': CapabilityStatus$json,
  '.hawkr.seller.v1alpha1.StoreStats': StoreStats$json,
  '.hawkr.seller.v1alpha1.GetMyOnboardingRequest': GetMyOnboardingRequest$json,
  '.hawkr.seller.v1alpha1.GetMyOnboardingResponse':
      GetMyOnboardingResponse$json,
  '.hawkr.seller.v1alpha1.OnboardingTask': OnboardingTask$json,
  '.hawkr.seller.v1alpha1.RefreshMyPaymentAccountRequest':
      RefreshMyPaymentAccountRequest$json,
  '.hawkr.seller.v1alpha1.RefreshMyPaymentAccountResponse':
      RefreshMyPaymentAccountResponse$json,
  '.hawkr.seller.v1alpha1.UpdateMyStoreRequest': UpdateMyStoreRequest$json,
  '.google.protobuf.FieldMask': $4.FieldMask$json,
  '.hawkr.seller.v1alpha1.UpdateMyStoreResponse': UpdateMyStoreResponse$json,
  '.hawkr.seller.v1alpha1.CompleteMyStoreOnboardingRequest':
      CompleteMyStoreOnboardingRequest$json,
  '.hawkr.seller.v1alpha1.CompleteMyStoreOnboardingResponse':
      CompleteMyStoreOnboardingResponse$json,
  '.hawkr.seller.v1alpha1.ListMyFollowersRequest': ListMyFollowersRequest$json,
  '.hawkr.seller.v1alpha1.ListMyFollowersResponse':
      ListMyFollowersResponse$json,
  '.hawkr.seller.v1alpha1.Follower': Follower$json,
  '.hawkr.seller.v1alpha1.BuyerCard': BuyerCard$json,
  '.hawkr.seller.v1alpha1.GetMyDashboardRequest': GetMyDashboardRequest$json,
  '.hawkr.seller.v1alpha1.GetMyDashboardResponse': GetMyDashboardResponse$json,
  '.hawkr.seller.v1alpha1.Dashboard': Dashboard$json,
  '.hawkr.seller.v1alpha1.DashboardPeriod': DashboardPeriod$json,
  '.google.type.Date': $5.Date$json,
  '.hawkr.seller.v1alpha1.DashboardSales': DashboardSales$json,
  '.hawkr.seller.v1alpha1.PeriodAmount': PeriodAmount$json,
  '.hawkr.seller.v1alpha1.PeriodCount': PeriodCount$json,
  '.hawkr.seller.v1alpha1.DashboardFollowers': DashboardFollowers$json,
  '.hawkr.seller.v1alpha1.OrderStatusCounts': OrderStatusCounts$json,
  '.hawkr.seller.v1alpha1.CloseMyStoreRequest': CloseMyStoreRequest$json,
  '.hawkr.seller.v1alpha1.CloseMyStoreResponse': CloseMyStoreResponse$json,
  '.hawkr.seller.v1alpha1.SaveKYCDraftRequest': SaveKYCDraftRequest$json,
  '.hawkr.seller.v1alpha1.KYCApplication': KYCApplication$json,
  '.hawkr.seller.v1alpha1.IndividualKYC': IndividualKYC$json,
  '.hawkr.seller.v1alpha1.BusinessKYC': BusinessKYC$json,
  '.hawkr.seller.v1alpha1.KYCDocument': KYCDocument$json,
  '.hawkr.seller.v1alpha1.KYCRejection': KYCRejection$json,
  '.hawkr.seller.v1alpha1.KYCIssue': KYCIssue$json,
  '.hawkr.seller.v1alpha1.SaveKYCDraftResponse': SaveKYCDraftResponse$json,
  '.hawkr.seller.v1alpha1.GetMyKYCRequest': GetMyKYCRequest$json,
  '.hawkr.seller.v1alpha1.GetMyKYCResponse': GetMyKYCResponse$json,
  '.hawkr.seller.v1alpha1.SubmitKYCRequest': SubmitKYCRequest$json,
  '.hawkr.seller.v1alpha1.SubmitKYCResponse': SubmitKYCResponse$json,
  '.hawkr.seller.v1alpha1.CancelKYCRequest': CancelKYCRequest$json,
  '.hawkr.seller.v1alpha1.CancelKYCResponse': CancelKYCResponse$json,
  '.hawkr.seller.v1alpha1.GetMyPaymentAccountRequest':
      GetMyPaymentAccountRequest$json,
  '.hawkr.seller.v1alpha1.GetMyPaymentAccountResponse':
      GetMyPaymentAccountResponse$json,
  '.hawkr.seller.v1alpha1.PaymentAccount': PaymentAccount$json,
  '.hawkr.seller.v1alpha1.CreatePaymentOnboardingSessionRequest':
      CreatePaymentOnboardingSessionRequest$json,
  '.hawkr.seller.v1alpha1.CreatePaymentOnboardingSessionResponse':
      CreatePaymentOnboardingSessionResponse$json,
  '.hawkr.seller.v1alpha1.LookupBusinessRegistrationRequest':
      LookupBusinessRegistrationRequest$json,
  '.hawkr.seller.v1alpha1.LookupBusinessRegistrationResponse':
      LookupBusinessRegistrationResponse$json,
  '.hawkr.seller.v1alpha1.BusinessRegistration': BusinessRegistration$json,
  '.hawkr.seller.v1alpha1.ListMyMeasuresRequest': ListMyMeasuresRequest$json,
  '.hawkr.seller.v1alpha1.ListMyMeasuresResponse': ListMyMeasuresResponse$json,
  '.hawkr.seller.v1alpha1.Measure': Measure$json,
  '.hawkr.seller.v1alpha1.MeasureFinding': MeasureFinding$json,
  '.hawkr.seller.v1alpha1.MeasurePeriod': MeasurePeriod$json,
  '.google.protobuf.Duration': $6.Duration$json,
  '.hawkr.seller.v1alpha1.MeasureRequest': MeasureRequest$json,
  '.hawkr.seller.v1alpha1.RemediationUnavailability':
      RemediationUnavailability$json,
  '.hawkr.seller.v1alpha1.AppealUnavailability': AppealUnavailability$json,
  '.hawkr.seller.v1alpha1.ContentRef': ContentRef$json,
  '.hawkr.seller.v1alpha1.GetMyMeasureRequest': GetMyMeasureRequest$json,
  '.hawkr.seller.v1alpha1.GetMyMeasureResponse': GetMyMeasureResponse$json,
  '.hawkr.seller.v1alpha1.SubmitAppealRequest': SubmitAppealRequest$json,
  '.hawkr.seller.v1alpha1.SubmitAppealResponse': SubmitAppealResponse$json,
  '.hawkr.seller.v1alpha1.RequestRemediationRequest':
      RequestRemediationRequest$json,
  '.hawkr.seller.v1alpha1.RequestRemediationResponse':
      RequestRemediationResponse$json,
};

/// Descriptor for `SellerAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerAppServiceDescriptor = $convert.base64Decode(
    'ChBTZWxsZXJBcHBTZXJ2aWNlEmEKCkFwcGx5U3RvcmUSKC5oYXdrci5zZWxsZXIudjFhbHBoYT'
    'EuQXBwbHlTdG9yZVJlcXVlc3QaKS5oYXdrci5zZWxsZXIudjFhbHBoYTEuQXBwbHlTdG9yZVJl'
    'c3BvbnNlEnsKEUdldE15U3RvcmVPcGVuaW5nEi8uaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkdldE'
    '15U3RvcmVPcGVuaW5nUmVxdWVzdBowLmhhd2tyLnNlbGxlci52MWFscGhhMS5HZXRNeVN0b3Jl'
    'T3BlbmluZ1Jlc3BvbnNlIgOQAgESZgoKR2V0TXlTdG9yZRIoLmhhd2tyLnNlbGxlci52MWFscG'
    'hhMS5HZXRNeVN0b3JlUmVxdWVzdBopLmhhd2tyLnNlbGxlci52MWFscGhhMS5HZXRNeVN0b3Jl'
    'UmVzcG9uc2UiA5ACARJ1Cg9HZXRNeU9uYm9hcmRpbmcSLS5oYXdrci5zZWxsZXIudjFhbHBoYT'
    'EuR2V0TXlPbmJvYXJkaW5nUmVxdWVzdBouLmhhd2tyLnNlbGxlci52MWFscGhhMS5HZXRNeU9u'
    'Ym9hcmRpbmdSZXNwb25zZSIDkAIBEo0BChdSZWZyZXNoTXlQYXltZW50QWNjb3VudBI1Lmhhd2'
    'tyLnNlbGxlci52MWFscGhhMS5SZWZyZXNoTXlQYXltZW50QWNjb3VudFJlcXVlc3QaNi5oYXdr'
    'ci5zZWxsZXIudjFhbHBoYTEuUmVmcmVzaE15UGF5bWVudEFjY291bnRSZXNwb25zZSIDkAICEm'
    'oKDVVwZGF0ZU15U3RvcmUSKy5oYXdrci5zZWxsZXIudjFhbHBoYTEuVXBkYXRlTXlTdG9yZVJl'
    'cXVlc3QaLC5oYXdrci5zZWxsZXIudjFhbHBoYTEuVXBkYXRlTXlTdG9yZVJlc3BvbnNlEpMBCh'
    'lDb21wbGV0ZU15U3RvcmVPbmJvYXJkaW5nEjcuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkNvbXBs'
    'ZXRlTXlTdG9yZU9uYm9hcmRpbmdSZXF1ZXN0GjguaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkNvbX'
    'BsZXRlTXlTdG9yZU9uYm9hcmRpbmdSZXNwb25zZSIDkAICEnUKD0xpc3RNeUZvbGxvd2VycxIt'
    'Lmhhd2tyLnNlbGxlci52MWFscGhhMS5MaXN0TXlGb2xsb3dlcnNSZXF1ZXN0Gi4uaGF3a3Iuc2'
    'VsbGVyLnYxYWxwaGExLkxpc3RNeUZvbGxvd2Vyc1Jlc3BvbnNlIgOQAgEScgoOR2V0TXlEYXNo'
    'Ym9hcmQSLC5oYXdrci5zZWxsZXIudjFhbHBoYTEuR2V0TXlEYXNoYm9hcmRSZXF1ZXN0Gi0uaG'
    'F3a3Iuc2VsbGVyLnYxYWxwaGExLkdldE15RGFzaGJvYXJkUmVzcG9uc2UiA5ACARJnCgxDbG9z'
    'ZU15U3RvcmUSKi5oYXdrci5zZWxsZXIudjFhbHBoYTEuQ2xvc2VNeVN0b3JlUmVxdWVzdBorLm'
    'hhd2tyLnNlbGxlci52MWFscGhhMS5DbG9zZU15U3RvcmVSZXNwb25zZRJnCgxTYXZlS1lDRHJh'
    'ZnQSKi5oYXdrci5zZWxsZXIudjFhbHBoYTEuU2F2ZUtZQ0RyYWZ0UmVxdWVzdBorLmhhd2tyLn'
    'NlbGxlci52MWFscGhhMS5TYXZlS1lDRHJhZnRSZXNwb25zZRJgCghHZXRNeUtZQxImLmhhd2ty'
    'LnNlbGxlci52MWFscGhhMS5HZXRNeUtZQ1JlcXVlc3QaJy5oYXdrci5zZWxsZXIudjFhbHBoYT'
    'EuR2V0TXlLWUNSZXNwb25zZSIDkAIBEl4KCVN1Ym1pdEtZQxInLmhhd2tyLnNlbGxlci52MWFs'
    'cGhhMS5TdWJtaXRLWUNSZXF1ZXN0GiguaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlN1Ym1pdEtZQ1'
    'Jlc3BvbnNlEl4KCUNhbmNlbEtZQxInLmhhd2tyLnNlbGxlci52MWFscGhhMS5DYW5jZWxLWUNS'
    'ZXF1ZXN0GiguaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkNhbmNlbEtZQ1Jlc3BvbnNlEoEBChNHZX'
    'RNeVBheW1lbnRBY2NvdW50EjEuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkdldE15UGF5bWVudEFj'
    'Y291bnRSZXF1ZXN0GjIuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkdldE15UGF5bWVudEFjY291bn'
    'RSZXNwb25zZSIDkAIBEp0BCh5DcmVhdGVQYXltZW50T25ib2FyZGluZ1Nlc3Npb24SPC5oYXdr'
    'ci5zZWxsZXIudjFhbHBoYTEuQ3JlYXRlUGF5bWVudE9uYm9hcmRpbmdTZXNzaW9uUmVxdWVzdB'
    'o9Lmhhd2tyLnNlbGxlci52MWFscGhhMS5DcmVhdGVQYXltZW50T25ib2FyZGluZ1Nlc3Npb25S'
    'ZXNwb25zZRKRAQoaTG9va3VwQnVzaW5lc3NSZWdpc3RyYXRpb24SOC5oYXdrci5zZWxsZXIudj'
    'FhbHBoYTEuTG9va3VwQnVzaW5lc3NSZWdpc3RyYXRpb25SZXF1ZXN0GjkuaGF3a3Iuc2VsbGVy'
    'LnYxYWxwaGExLkxvb2t1cEJ1c2luZXNzUmVnaXN0cmF0aW9uUmVzcG9uc2UScgoOTGlzdE15TW'
    'Vhc3VyZXMSLC5oYXdrci5zZWxsZXIudjFhbHBoYTEuTGlzdE15TWVhc3VyZXNSZXF1ZXN0Gi0u'
    'aGF3a3Iuc2VsbGVyLnYxYWxwaGExLkxpc3RNeU1lYXN1cmVzUmVzcG9uc2UiA5ACARJsCgxHZX'
    'RNeU1lYXN1cmUSKi5oYXdrci5zZWxsZXIudjFhbHBoYTEuR2V0TXlNZWFzdXJlUmVxdWVzdBor'
    'Lmhhd2tyLnNlbGxlci52MWFscGhhMS5HZXRNeU1lYXN1cmVSZXNwb25zZSIDkAIBEmcKDFN1Ym'
    '1pdEFwcGVhbBIqLmhhd2tyLnNlbGxlci52MWFscGhhMS5TdWJtaXRBcHBlYWxSZXF1ZXN0Gisu'
    'aGF3a3Iuc2VsbGVyLnYxYWxwaGExLlN1Ym1pdEFwcGVhbFJlc3BvbnNlEnkKElJlcXVlc3RSZW'
    '1lZGlhdGlvbhIwLmhhd2tyLnNlbGxlci52MWFscGhhMS5SZXF1ZXN0UmVtZWRpYXRpb25SZXF1'
    'ZXN0GjEuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlJlcXVlc3RSZW1lZGlhdGlvblJlc3BvbnNl');

const $core.Map<$core.String, $core.dynamic> SellerPublicServiceBase$json = {
  '1': 'SellerPublicService',
  '2': [
    {
      '1': 'GetStore',
      '2': '.hawkr.seller.v1alpha1.GetStoreRequest',
      '3': '.hawkr.seller.v1alpha1.GetStoreResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListStores',
      '2': '.hawkr.seller.v1alpha1.ListStoresRequest',
      '3': '.hawkr.seller.v1alpha1.ListStoresResponse',
      '4': {'34': 1},
    },
    {
      '1': 'BatchGetStores',
      '2': '.hawkr.seller.v1alpha1.BatchGetStoresRequest',
      '3': '.hawkr.seller.v1alpha1.BatchGetStoresResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetSellerAgreements',
      '2': '.hawkr.seller.v1alpha1.GetSellerAgreementsRequest',
      '3': '.hawkr.seller.v1alpha1.GetSellerAgreementsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'RecordProductView',
      '2': '.hawkr.seller.v1alpha1.RecordProductViewRequest',
      '3': '.hawkr.seller.v1alpha1.RecordProductViewResponse'
    },
  ],
};

@$core.Deprecated('Use sellerPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerPublicServiceBase$messageJson = {
  '.hawkr.seller.v1alpha1.GetStoreRequest': GetStoreRequest$json,
  '.hawkr.seller.v1alpha1.GetStoreResponse': GetStoreResponse$json,
  '.hawkr.seller.v1alpha1.PublicStore': PublicStore$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.seller.v1alpha1.StoreStats': StoreStats$json,
  '.hawkr.seller.v1alpha1.ListStoresRequest': ListStoresRequest$json,
  '.hawkr.seller.v1alpha1.ListStoresResponse': ListStoresResponse$json,
  '.hawkr.seller.v1alpha1.BatchGetStoresRequest': BatchGetStoresRequest$json,
  '.hawkr.seller.v1alpha1.BatchGetStoresResponse': BatchGetStoresResponse$json,
  '.hawkr.common.v1alpha1.StoreCard': $7.StoreCard$json,
  '.hawkr.seller.v1alpha1.GetSellerAgreementsRequest':
      GetSellerAgreementsRequest$json,
  '.hawkr.seller.v1alpha1.GetSellerAgreementsResponse':
      GetSellerAgreementsResponse$json,
  '.hawkr.seller.v1alpha1.SellerAgreement': SellerAgreement$json,
  '.hawkr.seller.v1alpha1.RecordProductViewRequest':
      RecordProductViewRequest$json,
  '.hawkr.seller.v1alpha1.RecordProductViewResponse':
      RecordProductViewResponse$json,
};

/// Descriptor for `SellerPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerPublicServiceDescriptor = $convert.base64Decode(
    'ChNTZWxsZXJQdWJsaWNTZXJ2aWNlEmAKCEdldFN0b3JlEiYuaGF3a3Iuc2VsbGVyLnYxYWxwaG'
    'ExLkdldFN0b3JlUmVxdWVzdBonLmhhd2tyLnNlbGxlci52MWFscGhhMS5HZXRTdG9yZVJlc3Bv'
    'bnNlIgOQAgESZgoKTGlzdFN0b3JlcxIoLmhhd2tyLnNlbGxlci52MWFscGhhMS5MaXN0U3Rvcm'
    'VzUmVxdWVzdBopLmhhd2tyLnNlbGxlci52MWFscGhhMS5MaXN0U3RvcmVzUmVzcG9uc2UiA5AC'
    'ARJyCg5CYXRjaEdldFN0b3JlcxIsLmhhd2tyLnNlbGxlci52MWFscGhhMS5CYXRjaEdldFN0b3'
    'Jlc1JlcXVlc3QaLS5oYXdrci5zZWxsZXIudjFhbHBoYTEuQmF0Y2hHZXRTdG9yZXNSZXNwb25z'
    'ZSIDkAIBEoEBChNHZXRTZWxsZXJBZ3JlZW1lbnRzEjEuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLk'
    'dldFNlbGxlckFncmVlbWVudHNSZXF1ZXN0GjIuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkdldFNl'
    'bGxlckFncmVlbWVudHNSZXNwb25zZSIDkAIBEnYKEVJlY29yZFByb2R1Y3RWaWV3Ei8uaGF3a3'
    'Iuc2VsbGVyLnYxYWxwaGExLlJlY29yZFByb2R1Y3RWaWV3UmVxdWVzdBowLmhhd2tyLnNlbGxl'
    'ci52MWFscGhhMS5SZWNvcmRQcm9kdWN0Vmlld1Jlc3BvbnNl');
