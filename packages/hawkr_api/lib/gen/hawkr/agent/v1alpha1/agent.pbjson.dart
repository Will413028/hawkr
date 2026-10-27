// This is a generated file - do not edit.
//
// Generated from hawkr/agent/v1alpha1/agent.proto.

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

@$core.Deprecated('Use sessionVerificationMethodDescriptor instead')
const SessionVerificationMethod$json = {
  '1': 'SessionVerificationMethod',
  '2': [
    {'1': 'SESSION_VERIFICATION_METHOD_UNSPECIFIED', '2': 0},
    {'1': 'SESSION_VERIFICATION_METHOD_GOOGLE', '2': 1},
  ],
};

/// Descriptor for `SessionVerificationMethod`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sessionVerificationMethodDescriptor = $convert.base64Decode(
    'ChlTZXNzaW9uVmVyaWZpY2F0aW9uTWV0aG9kEisKJ1NFU1NJT05fVkVSSUZJQ0FUSU9OX01FVE'
    'hPRF9VTlNQRUNJRklFRBAAEiYKIlNFU1NJT05fVkVSSUZJQ0FUSU9OX01FVEhPRF9HT09HTEUQ'
    'AQ==');

@$core.Deprecated('Use sessionVerificationDescriptor instead')
const SessionVerification$json = {
  '1': 'SessionVerification',
  '2': [
    {
      '1': 'method',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.agent.v1alpha1.SessionVerificationMethod',
      '10': 'method'
    },
    {
      '1': 'verify_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'verifyTime'
    },
  ],
};

/// Descriptor for `SessionVerification`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionVerificationDescriptor = $convert.base64Decode(
    'ChNTZXNzaW9uVmVyaWZpY2F0aW9uEkcKBm1ldGhvZBgBIAEoDjIvLmhhd2tyLmFnZW50LnYxYW'
    'xwaGExLlNlc3Npb25WZXJpZmljYXRpb25NZXRob2RSBm1ldGhvZBI7Cgt2ZXJpZnlfdGltZRgC'
    'IAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCnZlcmlmeVRpbWU=');

@$core.Deprecated('Use previewInviteRequestDescriptor instead')
const PreviewInviteRequest$json = {
  '1': 'PreviewInviteRequest',
  '2': [
    {'1': 'secret', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'secret'},
  ],
};

/// Descriptor for `PreviewInviteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewInviteRequestDescriptor =
    $convert.base64Decode(
        'ChRQcmV2aWV3SW52aXRlUmVxdWVzdBIbCgZzZWNyZXQYASABKAlCA+BBAlIGc2VjcmV0');

@$core.Deprecated('Use previewInviteResponseDescriptor instead')
const PreviewInviteResponse$json = {
  '1': 'PreviewInviteResponse',
  '2': [
    {'1': 'masked_email', '3': 1, '4': 1, '5': 9, '10': 'maskedEmail'},
    {
      '1': 'expire_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expireTime'
    },
  ],
};

/// Descriptor for `PreviewInviteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewInviteResponseDescriptor = $convert.base64Decode(
    'ChVQcmV2aWV3SW52aXRlUmVzcG9uc2USIQoMbWFza2VkX2VtYWlsGAEgASgJUgttYXNrZWRFbW'
    'FpbBI7CgtleHBpcmVfdGltZRgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCmV4'
    'cGlyZVRpbWU=');

@$core.Deprecated('Use exchangeInviteRequestDescriptor instead')
const ExchangeInviteRequest$json = {
  '1': 'ExchangeInviteRequest',
  '2': [
    {'1': 'secret', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'secret'},
  ],
};

/// Descriptor for `ExchangeInviteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exchangeInviteRequestDescriptor = $convert.base64Decode(
    'ChVFeGNoYW5nZUludml0ZVJlcXVlc3QSGwoGc2VjcmV0GAEgASgJQgPgQQJSBnNlY3JldA==');

@$core.Deprecated('Use exchangeInviteResponseDescriptor instead')
const ExchangeInviteResponse$json = {
  '1': 'ExchangeInviteResponse',
  '2': [
    {'1': 'rfq_recipient_id', '3': 1, '4': 1, '5': 9, '10': 'rfqRecipientId'},
    {
      '1': 'verification',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.agent.v1alpha1.SessionVerification',
      '10': 'verification'
    },
    {'1': 'google_nonce', '3': 3, '4': 1, '5': 9, '10': 'googleNonce'},
  ],
};

/// Descriptor for `ExchangeInviteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exchangeInviteResponseDescriptor = $convert.base64Decode(
    'ChZFeGNoYW5nZUludml0ZVJlc3BvbnNlEigKEHJmcV9yZWNpcGllbnRfaWQYASABKAlSDnJmcV'
    'JlY2lwaWVudElkEk0KDHZlcmlmaWNhdGlvbhgCIAEoCzIpLmhhd2tyLmFnZW50LnYxYWxwaGEx'
    'LlNlc3Npb25WZXJpZmljYXRpb25SDHZlcmlmaWNhdGlvbhIhCgxnb29nbGVfbm9uY2UYAyABKA'
    'lSC2dvb2dsZU5vbmNl');

@$core.Deprecated('Use getSessionRequestDescriptor instead')
const GetSessionRequest$json = {
  '1': 'GetSessionRequest',
  '2': [
    {
      '1': 'rfq_recipient_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'rfqRecipientId'
    },
  ],
};

/// Descriptor for `GetSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSessionRequestDescriptor = $convert.base64Decode(
    'ChFHZXRTZXNzaW9uUmVxdWVzdBItChByZnFfcmVjaXBpZW50X2lkGAEgASgJQgPgQQJSDnJmcV'
    'JlY2lwaWVudElk');

@$core.Deprecated('Use getSessionResponseDescriptor instead')
const GetSessionResponse$json = {
  '1': 'GetSessionResponse',
  '2': [
    {
      '1': 'verification',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.agent.v1alpha1.SessionVerification',
      '10': 'verification'
    },
    {'1': 'google_nonce', '3': 2, '4': 1, '5': 9, '10': 'googleNonce'},
  ],
};

/// Descriptor for `GetSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSessionResponseDescriptor = $convert.base64Decode(
    'ChJHZXRTZXNzaW9uUmVzcG9uc2USTQoMdmVyaWZpY2F0aW9uGAEgASgLMikuaGF3a3IuYWdlbn'
    'QudjFhbHBoYTEuU2Vzc2lvblZlcmlmaWNhdGlvblIMdmVyaWZpY2F0aW9uEiEKDGdvb2dsZV9u'
    'b25jZRgCIAEoCVILZ29vZ2xlTm9uY2U=');

@$core.Deprecated('Use verifySessionRequestDescriptor instead')
const VerifySessionRequest$json = {
  '1': 'VerifySessionRequest',
  '2': [
    {
      '1': 'rfq_recipient_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'rfqRecipientId'
    },
    {
      '1': 'google_id_token',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'googleIdToken'
    },
  ],
};

/// Descriptor for `VerifySessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verifySessionRequestDescriptor = $convert.base64Decode(
    'ChRWZXJpZnlTZXNzaW9uUmVxdWVzdBItChByZnFfcmVjaXBpZW50X2lkGAEgASgJQgPgQQJSDn'
    'JmcVJlY2lwaWVudElkEisKD2dvb2dsZV9pZF90b2tlbhgCIAEoCUID4EECUg1nb29nbGVJZFRv'
    'a2Vu');

@$core.Deprecated('Use verifySessionResponseDescriptor instead')
const VerifySessionResponse$json = {
  '1': 'VerifySessionResponse',
  '2': [
    {'1': 'rfq_recipient_id', '3': 1, '4': 1, '5': 9, '10': 'rfqRecipientId'},
    {
      '1': 'verification',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.agent.v1alpha1.SessionVerification',
      '10': 'verification'
    },
  ],
};

/// Descriptor for `VerifySessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verifySessionResponseDescriptor = $convert.base64Decode(
    'ChVWZXJpZnlTZXNzaW9uUmVzcG9uc2USKAoQcmZxX3JlY2lwaWVudF9pZBgBIAEoCVIOcmZxUm'
    'VjaXBpZW50SWQSTQoMdmVyaWZpY2F0aW9uGAIgASgLMikuaGF3a3IuYWdlbnQudjFhbHBoYTEu'
    'U2Vzc2lvblZlcmlmaWNhdGlvblIMdmVyaWZpY2F0aW9u');

const $core.Map<$core.String, $core.dynamic>
    VendorPortalPublicServiceBase$json = {
  '1': 'VendorPortalPublicService',
  '2': [
    {
      '1': 'PreviewInvite',
      '2': '.hawkr.agent.v1alpha1.PreviewInviteRequest',
      '3': '.hawkr.agent.v1alpha1.PreviewInviteResponse'
    },
    {
      '1': 'ExchangeInvite',
      '2': '.hawkr.agent.v1alpha1.ExchangeInviteRequest',
      '3': '.hawkr.agent.v1alpha1.ExchangeInviteResponse'
    },
  ],
};

@$core.Deprecated('Use vendorPortalPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    VendorPortalPublicServiceBase$messageJson = {
  '.hawkr.agent.v1alpha1.PreviewInviteRequest': PreviewInviteRequest$json,
  '.hawkr.agent.v1alpha1.PreviewInviteResponse': PreviewInviteResponse$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.agent.v1alpha1.ExchangeInviteRequest': ExchangeInviteRequest$json,
  '.hawkr.agent.v1alpha1.ExchangeInviteResponse': ExchangeInviteResponse$json,
  '.hawkr.agent.v1alpha1.SessionVerification': SessionVerification$json,
};

/// Descriptor for `VendorPortalPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List vendorPortalPublicServiceDescriptor = $convert.base64Decode(
    'ChlWZW5kb3JQb3J0YWxQdWJsaWNTZXJ2aWNlEmgKDVByZXZpZXdJbnZpdGUSKi5oYXdrci5hZ2'
    'VudC52MWFscGhhMS5QcmV2aWV3SW52aXRlUmVxdWVzdBorLmhhd2tyLmFnZW50LnYxYWxwaGEx'
    'LlByZXZpZXdJbnZpdGVSZXNwb25zZRJrCg5FeGNoYW5nZUludml0ZRIrLmhhd2tyLmFnZW50Ln'
    'YxYWxwaGExLkV4Y2hhbmdlSW52aXRlUmVxdWVzdBosLmhhd2tyLmFnZW50LnYxYWxwaGExLkV4'
    'Y2hhbmdlSW52aXRlUmVzcG9uc2U=');

const $core.Map<$core.String, $core.dynamic>
    VendorPortalInviteeServiceBase$json = {
  '1': 'VendorPortalInviteeService',
  '2': [
    {
      '1': 'GetSession',
      '2': '.hawkr.agent.v1alpha1.GetSessionRequest',
      '3': '.hawkr.agent.v1alpha1.GetSessionResponse',
      '4': {'34': 1},
    },
    {
      '1': 'VerifySession',
      '2': '.hawkr.agent.v1alpha1.VerifySessionRequest',
      '3': '.hawkr.agent.v1alpha1.VerifySessionResponse'
    },
  ],
};

@$core.Deprecated('Use vendorPortalInviteeServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    VendorPortalInviteeServiceBase$messageJson = {
  '.hawkr.agent.v1alpha1.GetSessionRequest': GetSessionRequest$json,
  '.hawkr.agent.v1alpha1.GetSessionResponse': GetSessionResponse$json,
  '.hawkr.agent.v1alpha1.SessionVerification': SessionVerification$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.agent.v1alpha1.VerifySessionRequest': VerifySessionRequest$json,
  '.hawkr.agent.v1alpha1.VerifySessionResponse': VerifySessionResponse$json,
};

/// Descriptor for `VendorPortalInviteeService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List vendorPortalInviteeServiceDescriptor = $convert.base64Decode(
    'ChpWZW5kb3JQb3J0YWxJbnZpdGVlU2VydmljZRJkCgpHZXRTZXNzaW9uEicuaGF3a3IuYWdlbn'
    'QudjFhbHBoYTEuR2V0U2Vzc2lvblJlcXVlc3QaKC5oYXdrci5hZ2VudC52MWFscGhhMS5HZXRT'
    'ZXNzaW9uUmVzcG9uc2UiA5ACARJoCg1WZXJpZnlTZXNzaW9uEiouaGF3a3IuYWdlbnQudjFhbH'
    'BoYTEuVmVyaWZ5U2Vzc2lvblJlcXVlc3QaKy5oYXdrci5hZ2VudC52MWFscGhhMS5WZXJpZnlT'
    'ZXNzaW9uUmVzcG9uc2U=');
