// This is a generated file - do not edit.
//
// Generated from hawkr/payment/v1alpha1/payment.proto.

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

import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $0;

@$core.Deprecated('Use paymentStatusDescriptor instead')
const PaymentStatus$json = {
  '1': 'PaymentStatus',
  '2': [
    {'1': 'PAYMENT_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_STATUS_CREATING', '2': 1},
    {'1': 'PAYMENT_STATUS_AWAITING_PAYMENT', '2': 2},
    {'1': 'PAYMENT_STATUS_PROCESSING', '2': 3},
    {'1': 'PAYMENT_STATUS_SUCCEEDED', '2': 4},
    {'1': 'PAYMENT_STATUS_CANCELED', '2': 5},
    {'1': 'PAYMENT_STATUS_CREATE_FAILED', '2': 6},
    {'1': 'PAYMENT_STATUS_ANOMALY', '2': 7},
  ],
};

/// Descriptor for `PaymentStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentStatusDescriptor = $convert.base64Decode(
    'Cg1QYXltZW50U3RhdHVzEh4KGlBBWU1FTlRfU1RBVFVTX1VOU1BFQ0lGSUVEEAASGwoXUEFZTU'
    'VOVF9TVEFUVVNfQ1JFQVRJTkcQARIjCh9QQVlNRU5UX1NUQVRVU19BV0FJVElOR19QQVlNRU5U'
    'EAISHQoZUEFZTUVOVF9TVEFUVVNfUFJPQ0VTU0lORxADEhwKGFBBWU1FTlRfU1RBVFVTX1NVQ0'
    'NFRURFRBAEEhsKF1BBWU1FTlRfU1RBVFVTX0NBTkNFTEVEEAUSIAocUEFZTUVOVF9TVEFUVVNf'
    'Q1JFQVRFX0ZBSUxFRBAGEhoKFlBBWU1FTlRfU1RBVFVTX0FOT01BTFkQBw==');

@$core.Deprecated('Use paymentAccountStatusDescriptor instead')
const PaymentAccountStatus$json = {
  '1': 'PaymentAccountStatus',
  '2': [
    {'1': 'PAYMENT_ACCOUNT_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_ACCOUNT_STATUS_NOT_CREATED', '2': 1},
    {'1': 'PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW', '2': 2},
    {'1': 'PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED', '2': 3},
    {'1': 'PAYMENT_ACCOUNT_STATUS_ENABLED', '2': 4},
    {'1': 'PAYMENT_ACCOUNT_STATUS_DISABLED', '2': 5},
  ],
};

/// Descriptor for `PaymentAccountStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentAccountStatusDescriptor = $convert.base64Decode(
    'ChRQYXltZW50QWNjb3VudFN0YXR1cxImCiJQQVlNRU5UX0FDQ09VTlRfU1RBVFVTX1VOU1BFQ0'
    'lGSUVEEAASJgoiUEFZTUVOVF9BQ0NPVU5UX1NUQVRVU19OT1RfQ1JFQVRFRBABEicKI1BBWU1F'
    'TlRfQUNDT1VOVF9TVEFUVVNfVU5ERVJfUkVWSUVXEAISKgomUEFZTUVOVF9BQ0NPVU5UX1NUQV'
    'RVU19BQ1RJT05fUkVRVUlSRUQQAxIiCh5QQVlNRU5UX0FDQ09VTlRfU1RBVFVTX0VOQUJMRUQQ'
    'BBIjCh9QQVlNRU5UX0FDQ09VTlRfU1RBVFVTX0RJU0FCTEVEEAU=');

@$core.Deprecated('Use requirementCollectionDescriptor instead')
const RequirementCollection$json = {
  '1': 'RequirementCollection',
  '2': [
    {'1': 'REQUIREMENT_COLLECTION_UNSPECIFIED', '2': 0},
    {'1': 'REQUIREMENT_COLLECTION_PROVIDER', '2': 1},
    {'1': 'REQUIREMENT_COLLECTION_PLATFORM', '2': 2},
  ],
};

/// Descriptor for `RequirementCollection`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List requirementCollectionDescriptor = $convert.base64Decode(
    'ChVSZXF1aXJlbWVudENvbGxlY3Rpb24SJgoiUkVRVUlSRU1FTlRfQ09MTEVDVElPTl9VTlNQRU'
    'NJRklFRBAAEiMKH1JFUVVJUkVNRU5UX0NPTExFQ1RJT05fUFJPVklERVIQARIjCh9SRVFVSVJF'
    'TUVOVF9DT0xMRUNUSU9OX1BMQVRGT1JNEAI=');

@$core.Deprecated('Use embeddedOnboardingRuntimeDescriptor instead')
const EmbeddedOnboardingRuntime$json = {
  '1': 'EmbeddedOnboardingRuntime',
  '2': [
    {'1': 'EMBEDDED_ONBOARDING_RUNTIME_UNSPECIFIED', '2': 0},
    {'1': 'EMBEDDED_ONBOARDING_RUNTIME_STRIPE_CONNECT_JS', '2': 1},
  ],
};

/// Descriptor for `EmbeddedOnboardingRuntime`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List embeddedOnboardingRuntimeDescriptor = $convert.base64Decode(
    'ChlFbWJlZGRlZE9uYm9hcmRpbmdSdW50aW1lEisKJ0VNQkVEREVEX09OQk9BUkRJTkdfUlVOVE'
    'lNRV9VTlNQRUNJRklFRBAAEjEKLUVNQkVEREVEX09OQk9BUkRJTkdfUlVOVElNRV9TVFJJUEVf'
    'Q09OTkVDVF9KUxAB');

@$core.Deprecated('Use paymentRefundStatusDescriptor instead')
const PaymentRefundStatus$json = {
  '1': 'PaymentRefundStatus',
  '2': [
    {'1': 'PAYMENT_REFUND_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_REFUND_STATUS_PENDING', '2': 1},
    {'1': 'PAYMENT_REFUND_STATUS_REFUNDING', '2': 2},
    {'1': 'PAYMENT_REFUND_STATUS_SUCCEEDED', '2': 3},
    {'1': 'PAYMENT_REFUND_STATUS_FAILED', '2': 4},
    {'1': 'PAYMENT_REFUND_STATUS_ANOMALY', '2': 5},
  ],
};

/// Descriptor for `PaymentRefundStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentRefundStatusDescriptor = $convert.base64Decode(
    'ChNQYXltZW50UmVmdW5kU3RhdHVzEiUKIVBBWU1FTlRfUkVGVU5EX1NUQVRVU19VTlNQRUNJRk'
    'lFRBAAEiEKHVBBWU1FTlRfUkVGVU5EX1NUQVRVU19QRU5ESU5HEAESIwofUEFZTUVOVF9SRUZV'
    'TkRfU1RBVFVTX1JFRlVORElORxACEiMKH1BBWU1FTlRfUkVGVU5EX1NUQVRVU19TVUNDRUVERU'
    'QQAxIgChxQQVlNRU5UX1JFRlVORF9TVEFUVVNfRkFJTEVEEAQSIQodUEFZTUVOVF9SRUZVTkRf'
    'U1RBVFVTX0FOT01BTFkQBQ==');

@$core.Deprecated('Use paymentTransferStatusDescriptor instead')
const PaymentTransferStatus$json = {
  '1': 'PaymentTransferStatus',
  '2': [
    {'1': 'PAYMENT_TRANSFER_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_TRANSFER_STATUS_PENDING', '2': 1},
    {'1': 'PAYMENT_TRANSFER_STATUS_SUCCEEDED', '2': 2},
    {'1': 'PAYMENT_TRANSFER_STATUS_FAILED', '2': 3},
    {'1': 'PAYMENT_TRANSFER_STATUS_ANOMALY', '2': 4},
  ],
};

/// Descriptor for `PaymentTransferStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentTransferStatusDescriptor = $convert.base64Decode(
    'ChVQYXltZW50VHJhbnNmZXJTdGF0dXMSJwojUEFZTUVOVF9UUkFOU0ZFUl9TVEFUVVNfVU5TUE'
    'VDSUZJRUQQABIjCh9QQVlNRU5UX1RSQU5TRkVSX1NUQVRVU19QRU5ESU5HEAESJQohUEFZTUVO'
    'VF9UUkFOU0ZFUl9TVEFUVVNfU1VDQ0VFREVEEAISIgoeUEFZTUVOVF9UUkFOU0ZFUl9TVEFUVV'
    'NfRkFJTEVEEAMSIwofUEFZTUVOVF9UUkFOU0ZFUl9TVEFUVVNfQU5PTUFMWRAE');

@$core.Deprecated('Use paymentReversalStatusDescriptor instead')
const PaymentReversalStatus$json = {
  '1': 'PaymentReversalStatus',
  '2': [
    {'1': 'PAYMENT_REVERSAL_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_REVERSAL_STATUS_NONE', '2': 1},
    {'1': 'PAYMENT_REVERSAL_STATUS_WAITING', '2': 2},
    {'1': 'PAYMENT_REVERSAL_STATUS_REVERSING', '2': 3},
    {'1': 'PAYMENT_REVERSAL_STATUS_REVERSED', '2': 4},
    {'1': 'PAYMENT_REVERSAL_STATUS_RELEASED', '2': 5},
    {'1': 'PAYMENT_REVERSAL_STATUS_ANOMALY', '2': 6},
  ],
};

/// Descriptor for `PaymentReversalStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentReversalStatusDescriptor = $convert.base64Decode(
    'ChVQYXltZW50UmV2ZXJzYWxTdGF0dXMSJwojUEFZTUVOVF9SRVZFUlNBTF9TVEFUVVNfVU5TUE'
    'VDSUZJRUQQABIgChxQQVlNRU5UX1JFVkVSU0FMX1NUQVRVU19OT05FEAESIwofUEFZTUVOVF9S'
    'RVZFUlNBTF9TVEFUVVNfV0FJVElORxACEiUKIVBBWU1FTlRfUkVWRVJTQUxfU1RBVFVTX1JFVk'
    'VSU0lORxADEiQKIFBBWU1FTlRfUkVWRVJTQUxfU1RBVFVTX1JFVkVSU0VEEAQSJAogUEFZTUVO'
    'VF9SRVZFUlNBTF9TVEFUVVNfUkVMRUFTRUQQBRIjCh9QQVlNRU5UX1JFVkVSU0FMX1NUQVRVU1'
    '9BTk9NQUxZEAY=');

@$core.Deprecated('Use paymentTopupStatusDescriptor instead')
const PaymentTopupStatus$json = {
  '1': 'PaymentTopupStatus',
  '2': [
    {'1': 'PAYMENT_TOPUP_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_TOPUP_STATUS_NONE', '2': 1},
    {'1': 'PAYMENT_TOPUP_STATUS_WAITING', '2': 2},
    {'1': 'PAYMENT_TOPUP_STATUS_SENDING', '2': 3},
    {'1': 'PAYMENT_TOPUP_STATUS_DONE', '2': 4},
    {'1': 'PAYMENT_TOPUP_STATUS_RELEASED', '2': 5},
    {'1': 'PAYMENT_TOPUP_STATUS_ANOMALY', '2': 6},
  ],
};

/// Descriptor for `PaymentTopupStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentTopupStatusDescriptor = $convert.base64Decode(
    'ChJQYXltZW50VG9wdXBTdGF0dXMSJAogUEFZTUVOVF9UT1BVUF9TVEFUVVNfVU5TUEVDSUZJRU'
    'QQABIdChlQQVlNRU5UX1RPUFVQX1NUQVRVU19OT05FEAESIAocUEFZTUVOVF9UT1BVUF9TVEFU'
    'VVNfV0FJVElORxACEiAKHFBBWU1FTlRfVE9QVVBfU1RBVFVTX1NFTkRJTkcQAxIdChlQQVlNRU'
    '5UX1RPUFVQX1NUQVRVU19ET05FEAQSIQodUEFZTUVOVF9UT1BVUF9TVEFUVVNfUkVMRUFTRUQQ'
    'BRIgChxQQVlNRU5UX1RPUFVQX1NUQVRVU19BTk9NQUxZEAY=');

@$core.Deprecated('Use paymentAnomalyKindDescriptor instead')
const PaymentAnomalyKind$json = {
  '1': 'PaymentAnomalyKind',
  '2': [
    {'1': 'PAYMENT_ANOMALY_KIND_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_ANOMALY_KIND_PAYMENT', '2': 1},
    {'1': 'PAYMENT_ANOMALY_KIND_TRANSFER', '2': 2},
    {'1': 'PAYMENT_ANOMALY_KIND_REFUND', '2': 3},
    {'1': 'PAYMENT_ANOMALY_KIND_REVERSAL', '2': 4},
    {'1': 'PAYMENT_ANOMALY_KIND_TOPUP', '2': 5},
  ],
};

/// Descriptor for `PaymentAnomalyKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentAnomalyKindDescriptor = $convert.base64Decode(
    'ChJQYXltZW50QW5vbWFseUtpbmQSJAogUEFZTUVOVF9BTk9NQUxZX0tJTkRfVU5TUEVDSUZJRU'
    'QQABIgChxQQVlNRU5UX0FOT01BTFlfS0lORF9QQVlNRU5UEAESIQodUEFZTUVOVF9BTk9NQUxZ'
    'X0tJTkRfVFJBTlNGRVIQAhIfChtQQVlNRU5UX0FOT01BTFlfS0lORF9SRUZVTkQQAxIhCh1QQV'
    'lNRU5UX0FOT01BTFlfS0lORF9SRVZFUlNBTBAEEh4KGlBBWU1FTlRfQU5PTUFMWV9LSU5EX1RP'
    'UFVQEAU=');

@$core.Deprecated('Use paymentAnomalyOutcomeDescriptor instead')
const PaymentAnomalyOutcome$json = {
  '1': 'PaymentAnomalyOutcome',
  '2': [
    {'1': 'PAYMENT_ANOMALY_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'PAYMENT_ANOMALY_OUTCOME_SUCCEEDED', '2': 1},
    {'1': 'PAYMENT_ANOMALY_OUTCOME_FAILED', '2': 2},
  ],
};

/// Descriptor for `PaymentAnomalyOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentAnomalyOutcomeDescriptor = $convert.base64Decode(
    'ChVQYXltZW50QW5vbWFseU91dGNvbWUSJwojUEFZTUVOVF9BTk9NQUxZX09VVENPTUVfVU5TUE'
    'VDSUZJRUQQABIlCiFQQVlNRU5UX0FOT01BTFlfT1VUQ09NRV9TVUNDRUVERUQQARIiCh5QQVlN'
    'RU5UX0FOT01BTFlfT1VUQ09NRV9GQUlMRUQQAg==');

@$core.Deprecated('Use paymentAnomalyResolutionReasonDescriptor instead')
const PaymentAnomalyResolutionReason$json = {
  '1': 'PaymentAnomalyResolutionReason',
  '2': [
    {'1': 'PAYMENT_ANOMALY_RESOLUTION_REASON_UNSPECIFIED', '2': 0},
    {
      '1': 'PAYMENT_ANOMALY_RESOLUTION_REASON_PROVIDER_EXECUTED_NOT_RECORDED',
      '2': 1
    },
    {'1': 'PAYMENT_ANOMALY_RESOLUTION_REASON_PROVIDER_NOT_EXECUTED', '2': 2},
    {'1': 'PAYMENT_ANOMALY_RESOLUTION_REASON_DUPLICATE', '2': 3},
    {'1': 'PAYMENT_ANOMALY_RESOLUTION_REASON_OTHER', '2': 4},
  ],
};

/// Descriptor for `PaymentAnomalyResolutionReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List paymentAnomalyResolutionReasonDescriptor = $convert.base64Decode(
    'Ch5QYXltZW50QW5vbWFseVJlc29sdXRpb25SZWFzb24SMQotUEFZTUVOVF9BTk9NQUxZX1JFU0'
    '9MVVRJT05fUkVBU09OX1VOU1BFQ0lGSUVEEAASRApAUEFZTUVOVF9BTk9NQUxZX1JFU09MVVRJ'
    'T05fUkVBU09OX1BST1ZJREVSX0VYRUNVVEVEX05PVF9SRUNPUkRFRBABEjsKN1BBWU1FTlRfQU'
    '5PTUFMWV9SRVNPTFVUSU9OX1JFQVNPTl9QUk9WSURFUl9OT1RfRVhFQ1VURUQQAhIvCitQQVlN'
    'RU5UX0FOT01BTFlfUkVTT0xVVElPTl9SRUFTT05fRFVQTElDQVRFEAMSKwonUEFZTUVOVF9BTk'
    '9NQUxZX1JFU09MVVRJT05fUkVBU09OX09USEVSEAQ=');

@$core.Deprecated('Use earningsEntryTypeDescriptor instead')
const EarningsEntryType$json = {
  '1': 'EarningsEntryType',
  '2': [
    {'1': 'EARNINGS_ENTRY_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'EARNINGS_ENTRY_TYPE_TRANSFER', '2': 1},
    {'1': 'EARNINGS_ENTRY_TYPE_REVERSAL', '2': 2},
    {'1': 'EARNINGS_ENTRY_TYPE_TOP_UP', '2': 3},
  ],
};

/// Descriptor for `EarningsEntryType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List earningsEntryTypeDescriptor = $convert.base64Decode(
    'ChFFYXJuaW5nc0VudHJ5VHlwZRIjCh9FQVJOSU5HU19FTlRSWV9UWVBFX1VOU1BFQ0lGSUVEEA'
    'ASIAocRUFSTklOR1NfRU5UUllfVFlQRV9UUkFOU0ZFUhABEiAKHEVBUk5JTkdTX0VOVFJZX1RZ'
    'UEVfUkVWRVJTQUwQAhIeChpFQVJOSU5HU19FTlRSWV9UWVBFX1RPUF9VUBAD');

@$core.Deprecated('Use earningsEntryStatusDescriptor instead')
const EarningsEntryStatus$json = {
  '1': 'EarningsEntryStatus',
  '2': [
    {'1': 'EARNINGS_ENTRY_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'EARNINGS_ENTRY_STATUS_PENDING', '2': 1},
    {'1': 'EARNINGS_ENTRY_STATUS_COMPLETED', '2': 2},
    {'1': 'EARNINGS_ENTRY_STATUS_FAILED', '2': 3},
  ],
};

/// Descriptor for `EarningsEntryStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List earningsEntryStatusDescriptor = $convert.base64Decode(
    'ChNFYXJuaW5nc0VudHJ5U3RhdHVzEiUKIUVBUk5JTkdTX0VOVFJZX1NUQVRVU19VTlNQRUNJRk'
    'lFRBAAEiEKHUVBUk5JTkdTX0VOVFJZX1NUQVRVU19QRU5ESU5HEAESIwofRUFSTklOR1NfRU5U'
    'UllfU1RBVFVTX0NPTVBMRVRFRBACEiAKHEVBUk5JTkdTX0VOVFJZX1NUQVRVU19GQUlMRUQQAw'
    '==');

@$core.Deprecated('Use paymentDescriptor instead')
const Payment$json = {
  '1': 'Payment',
  '2': [
    {'1': 'order_no', '3': 1, '4': 1, '5': 9, '10': 'orderNo'},
    {
      '1': 'status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.payment.v1alpha1.PaymentStatus',
      '10': 'status'
    },
    {'1': 'amount', '3': 3, '4': 1, '5': 3, '10': 'amount'},
    {
      '1': 'paid_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'paidTime'
    },
    {
      '1': 'session',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.payment.v1alpha1.PaymentSession',
      '8': {},
      '10': 'session'
    },
  ],
};

/// Descriptor for `Payment`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List paymentDescriptor = $convert.base64Decode(
    'CgdQYXltZW50EhkKCG9yZGVyX25vGAEgASgJUgdvcmRlck5vEj0KBnN0YXR1cxgCIAEoDjIlLm'
    'hhd2tyLnBheW1lbnQudjFhbHBoYTEuUGF5bWVudFN0YXR1c1IGc3RhdHVzEhYKBmFtb3VudBgD'
    'IAEoA1IGYW1vdW50EjcKCXBhaWRfdGltZRgEIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3'
    'RhbXBSCHBhaWRUaW1lEkUKB3Nlc3Npb24YBSABKAsyJi5oYXdrci5wYXltZW50LnYxYWxwaGEx'
    'LlBheW1lbnRTZXNzaW9uQgPgQQNSB3Nlc3Npb24=');

@$core.Deprecated('Use paymentSessionDescriptor instead')
const PaymentSession$json = {
  '1': 'PaymentSession',
  '2': [
    {
      '1': 'stripe',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.payment.v1alpha1.StripePaymentSession',
      '9': 0,
      '10': 'stripe'
    },
  ],
  '8': [
    {'1': 'provider'},
  ],
};

/// Descriptor for `PaymentSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List paymentSessionDescriptor = $convert.base64Decode(
    'Cg5QYXltZW50U2Vzc2lvbhJGCgZzdHJpcGUYASABKAsyLC5oYXdrci5wYXltZW50LnYxYWxwaG'
    'ExLlN0cmlwZVBheW1lbnRTZXNzaW9uSABSBnN0cmlwZUIKCghwcm92aWRlcg==');

@$core.Deprecated('Use stripePaymentSessionDescriptor instead')
const StripePaymentSession$json = {
  '1': 'StripePaymentSession',
  '2': [
    {'1': 'client_secret', '3': 1, '4': 1, '5': 9, '10': 'clientSecret'},
    {'1': 'publishable_key', '3': 2, '4': 1, '5': 9, '10': 'publishableKey'},
  ],
};

/// Descriptor for `StripePaymentSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List stripePaymentSessionDescriptor = $convert.base64Decode(
    'ChRTdHJpcGVQYXltZW50U2Vzc2lvbhIjCg1jbGllbnRfc2VjcmV0GAEgASgJUgxjbGllbnRTZW'
    'NyZXQSJwoPcHVibGlzaGFibGVfa2V5GAIgASgJUg5wdWJsaXNoYWJsZUtleQ==');

@$core.Deprecated('Use createPaymentRequestDescriptor instead')
const CreatePaymentRequest$json = {
  '1': 'CreatePaymentRequest',
  '2': [
    {'1': 'order_no', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'orderNo'},
  ],
};

/// Descriptor for `CreatePaymentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPaymentRequestDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVQYXltZW50UmVxdWVzdBIeCghvcmRlcl9ubxgBIAEoCUID4EECUgdvcmRlck5v');

@$core.Deprecated('Use createPaymentResponseDescriptor instead')
const CreatePaymentResponse$json = {
  '1': 'CreatePaymentResponse',
  '2': [
    {
      '1': 'payment',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.payment.v1alpha1.Payment',
      '10': 'payment'
    },
  ],
};

/// Descriptor for `CreatePaymentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPaymentResponseDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVQYXltZW50UmVzcG9uc2USOQoHcGF5bWVudBgBIAEoCzIfLmhhd2tyLnBheW1lbn'
    'QudjFhbHBoYTEuUGF5bWVudFIHcGF5bWVudA==');

@$core.Deprecated('Use getPaymentRequestDescriptor instead')
const GetPaymentRequest$json = {
  '1': 'GetPaymentRequest',
  '2': [
    {'1': 'order_no', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'orderNo'},
  ],
};

/// Descriptor for `GetPaymentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPaymentRequestDescriptor = $convert.base64Decode(
    'ChFHZXRQYXltZW50UmVxdWVzdBIeCghvcmRlcl9ubxgBIAEoCUID4EECUgdvcmRlck5v');

@$core.Deprecated('Use getPaymentResponseDescriptor instead')
const GetPaymentResponse$json = {
  '1': 'GetPaymentResponse',
  '2': [
    {
      '1': 'payment',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.payment.v1alpha1.Payment',
      '10': 'payment'
    },
  ],
};

/// Descriptor for `GetPaymentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPaymentResponseDescriptor = $convert.base64Decode(
    'ChJHZXRQYXltZW50UmVzcG9uc2USOQoHcGF5bWVudBgBIAEoCzIfLmhhd2tyLnBheW1lbnQudj'
    'FhbHBoYTEuUGF5bWVudFIHcGF5bWVudA==');

@$core.Deprecated('Use earningsEntryDescriptor instead')
const EarningsEntry$json = {
  '1': 'EarningsEntry',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.payment.v1alpha1.EarningsEntryType',
      '10': 'type'
    },
    {'1': 'order_no', '3': 3, '4': 1, '5': 9, '10': 'orderNo'},
    {'1': 'amount', '3': 4, '4': 1, '5': 3, '10': 'amount'},
    {
      '1': 'status',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.payment.v1alpha1.EarningsEntryStatus',
      '10': 'status'
    },
    {
      '1': 'create_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
  ],
};

/// Descriptor for `EarningsEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List earningsEntryDescriptor = $convert.base64Decode(
    'Cg1FYXJuaW5nc0VudHJ5EhMKAmlkGAEgASgJQgPgQQhSAmlkEj0KBHR5cGUYAiABKA4yKS5oYX'
    'drci5wYXltZW50LnYxYWxwaGExLkVhcm5pbmdzRW50cnlUeXBlUgR0eXBlEhkKCG9yZGVyX25v'
    'GAMgASgJUgdvcmRlck5vEhYKBmFtb3VudBgEIAEoA1IGYW1vdW50EkMKBnN0YXR1cxgFIAEoDj'
    'IrLmhhd2tyLnBheW1lbnQudjFhbHBoYTEuRWFybmluZ3NFbnRyeVN0YXR1c1IGc3RhdHVzEkAK'
    'C2NyZWF0ZV90aW1lGAYgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpjcm'
    'VhdGVUaW1l');

const $core.Map<$core.String, $core.dynamic> PaymentAppServiceBase$json = {
  '1': 'PaymentAppService',
  '2': [
    {
      '1': 'CreatePayment',
      '2': '.hawkr.payment.v1alpha1.CreatePaymentRequest',
      '3': '.hawkr.payment.v1alpha1.CreatePaymentResponse',
      '4': {'34': 2},
    },
    {
      '1': 'GetPayment',
      '2': '.hawkr.payment.v1alpha1.GetPaymentRequest',
      '3': '.hawkr.payment.v1alpha1.GetPaymentResponse'
    },
  ],
};

@$core.Deprecated('Use paymentAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    PaymentAppServiceBase$messageJson = {
  '.hawkr.payment.v1alpha1.CreatePaymentRequest': CreatePaymentRequest$json,
  '.hawkr.payment.v1alpha1.CreatePaymentResponse': CreatePaymentResponse$json,
  '.hawkr.payment.v1alpha1.Payment': Payment$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.payment.v1alpha1.PaymentSession': PaymentSession$json,
  '.hawkr.payment.v1alpha1.StripePaymentSession': StripePaymentSession$json,
  '.hawkr.payment.v1alpha1.GetPaymentRequest': GetPaymentRequest$json,
  '.hawkr.payment.v1alpha1.GetPaymentResponse': GetPaymentResponse$json,
};

/// Descriptor for `PaymentAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List paymentAppServiceDescriptor = $convert.base64Decode(
    'ChFQYXltZW50QXBwU2VydmljZRJxCg1DcmVhdGVQYXltZW50EiwuaGF3a3IucGF5bWVudC52MW'
    'FscGhhMS5DcmVhdGVQYXltZW50UmVxdWVzdBotLmhhd2tyLnBheW1lbnQudjFhbHBoYTEuQ3Jl'
    'YXRlUGF5bWVudFJlc3BvbnNlIgOQAgISYwoKR2V0UGF5bWVudBIpLmhhd2tyLnBheW1lbnQudj'
    'FhbHBoYTEuR2V0UGF5bWVudFJlcXVlc3QaKi5oYXdrci5wYXltZW50LnYxYWxwaGExLkdldFBh'
    'eW1lbnRSZXNwb25zZQ==');
