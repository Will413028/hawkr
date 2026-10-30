// This is a generated file - do not edit.
//
// Generated from hawkr/identity/v1alpha1/identity.proto.

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
    as $1;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $0;

@$core.Deprecated('Use roleDescriptor instead')
const Role$json = {
  '1': 'Role',
  '2': [
    {'1': 'ROLE_UNSPECIFIED', '2': 0},
    {'1': 'ROLE_SELLER', '2': 2},
  ],
  '4': [
    {'1': 1, '2': 1},
  ],
  '5': ['ROLE_BUYER'],
};

/// Descriptor for `Role`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List roleDescriptor = $convert.base64Decode(
    'CgRSb2xlEhQKEFJPTEVfVU5TUEVDSUZJRUQQABIPCgtST0xFX1NFTExFUhACIgQIARABKgpST0'
    'xFX0JVWUVS');

@$core.Deprecated('Use loginProviderDescriptor instead')
const LoginProvider$json = {
  '1': 'LoginProvider',
  '2': [
    {'1': 'LOGIN_PROVIDER_UNSPECIFIED', '2': 0},
    {'1': 'LOGIN_PROVIDER_GOOGLE', '2': 1},
    {'1': 'LOGIN_PROVIDER_LINE', '2': 2},
  ],
};

/// Descriptor for `LoginProvider`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List loginProviderDescriptor = $convert.base64Decode(
    'Cg1Mb2dpblByb3ZpZGVyEh4KGkxPR0lOX1BST1ZJREVSX1VOU1BFQ0lGSUVEEAASGQoVTE9HSU'
    '5fUFJPVklERVJfR09PR0xFEAESFwoTTE9HSU5fUFJPVklERVJfTElORRAC');

@$core.Deprecated('Use clientKindDescriptor instead')
const ClientKind$json = {
  '1': 'ClientKind',
  '2': [
    {'1': 'CLIENT_KIND_UNSPECIFIED', '2': 0},
    {'1': 'CLIENT_KIND_WEB', '2': 1},
    {'1': 'CLIENT_KIND_APP', '2': 2},
  ],
};

/// Descriptor for `ClientKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List clientKindDescriptor = $convert.base64Decode(
    'CgpDbGllbnRLaW5kEhsKF0NMSUVOVF9LSU5EX1VOU1BFQ0lGSUVEEAASEwoPQ0xJRU5UX0tJTk'
    'RfV0VCEAESEwoPQ0xJRU5UX0tJTkRfQVBQEAI=');

@$core.Deprecated('Use accountDescriptor instead')
const Account$json = {
  '1': 'Account',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'roles',
      '3': 2,
      '4': 3,
      '5': 14,
      '6': '.hawkr.identity.v1alpha1.Role',
      '8': {},
      '10': 'roles'
    },
  ],
  '9': [
    {'1': 3, '2': 4},
    {'1': 4, '2': 5},
    {'1': 5, '2': 6},
  ],
  '10': ['active_role', 'perspective'],
};

/// Descriptor for `Account`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List accountDescriptor = $convert.base64Decode(
    'CgdBY2NvdW50EhMKAmlkGAEgASgJQgPgQQhSAmlkEjgKBXJvbGVzGAIgAygOMh0uaGF3a3IuaW'
    'RlbnRpdHkudjFhbHBoYTEuUm9sZUID4EEDUgVyb2xlc0oECAMQBEoECAQQBUoECAUQBlILYWN0'
    'aXZlX3JvbGVSC3BlcnNwZWN0aXZl');

@$core.Deprecated('Use issueAppLoginNonceRequestDescriptor instead')
const IssueAppLoginNonceRequest$json = {
  '1': 'IssueAppLoginNonceRequest',
};

/// Descriptor for `IssueAppLoginNonceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueAppLoginNonceRequestDescriptor =
    $convert.base64Decode('ChlJc3N1ZUFwcExvZ2luTm9uY2VSZXF1ZXN0');

@$core.Deprecated('Use issueAppLoginNonceResponseDescriptor instead')
const IssueAppLoginNonceResponse$json = {
  '1': 'IssueAppLoginNonceResponse',
  '2': [
    {'1': 'nonce', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'nonce'},
    {
      '1': 'expire_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'expireTime'
    },
  ],
};

/// Descriptor for `IssueAppLoginNonceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueAppLoginNonceResponseDescriptor =
    $convert.base64Decode(
        'ChpJc3N1ZUFwcExvZ2luTm9uY2VSZXNwb25zZRIZCgVub25jZRgBIAEoCUID4EEDUgVub25jZR'
        'JACgtleHBpcmVfdGltZRgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1IK'
        'ZXhwaXJlVGltZQ==');

@$core.Deprecated('Use exchangeIdTokenRequestDescriptor instead')
const ExchangeIdTokenRequest$json = {
  '1': 'ExchangeIdTokenRequest',
  '2': [
    {
      '1': 'provider',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.identity.v1alpha1.LoginProvider',
      '8': {},
      '10': 'provider'
    },
    {'1': 'id_token', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'idToken'},
    {'1': 'nonce', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'nonce'},
  ],
};

/// Descriptor for `ExchangeIdTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exchangeIdTokenRequestDescriptor = $convert.base64Decode(
    'ChZFeGNoYW5nZUlkVG9rZW5SZXF1ZXN0EkcKCHByb3ZpZGVyGAEgASgOMiYuaGF3a3IuaWRlbn'
    'RpdHkudjFhbHBoYTEuTG9naW5Qcm92aWRlckID4EECUghwcm92aWRlchIeCghpZF90b2tlbhgC'
    'IAEoCUID4EECUgdpZFRva2VuEhkKBW5vbmNlGAMgASgJQgPgQQJSBW5vbmNl');

@$core.Deprecated('Use exchangeIdTokenResponseDescriptor instead')
const ExchangeIdTokenResponse$json = {
  '1': 'ExchangeIdTokenResponse',
  '2': [
    {
      '1': 'session_token',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'sessionToken'
    },
    {
      '1': 'absolute_expire_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'absoluteExpireTime'
    },
    {
      '1': 'idle_timeout',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '8': {},
      '10': 'idleTimeout'
    },
  ],
};

/// Descriptor for `ExchangeIdTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exchangeIdTokenResponseDescriptor = $convert.base64Decode(
    'ChdFeGNoYW5nZUlkVG9rZW5SZXNwb25zZRIoCg1zZXNzaW9uX3Rva2VuGAEgASgJQgPgQQNSDH'
    'Nlc3Npb25Ub2tlbhJRChRhYnNvbHV0ZV9leHBpcmVfdGltZRgCIAEoCzIaLmdvb2dsZS5wcm90'
    'b2J1Zi5UaW1lc3RhbXBCA+BBA1ISYWJzb2x1dGVFeHBpcmVUaW1lEkEKDGlkbGVfdGltZW91dB'
    'gDIAEoCzIZLmdvb2dsZS5wcm90b2J1Zi5EdXJhdGlvbkID4EEDUgtpZGxlVGltZW91dA==');

@$core.Deprecated('Use signOutRequestDescriptor instead')
const SignOutRequest$json = {
  '1': 'SignOutRequest',
};

/// Descriptor for `SignOutRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List signOutRequestDescriptor =
    $convert.base64Decode('Cg5TaWduT3V0UmVxdWVzdA==');

@$core.Deprecated('Use signOutResponseDescriptor instead')
const SignOutResponse$json = {
  '1': 'SignOutResponse',
};

/// Descriptor for `SignOutResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List signOutResponseDescriptor =
    $convert.base64Decode('Cg9TaWduT3V0UmVzcG9uc2U=');

@$core.Deprecated('Use linkIdentityRequestDescriptor instead')
const LinkIdentityRequest$json = {
  '1': 'LinkIdentityRequest',
  '2': [
    {
      '1': 'provider',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.identity.v1alpha1.LoginProvider',
      '8': {},
      '10': 'provider'
    },
    {'1': 'id_token', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'idToken'},
    {'1': 'nonce', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'nonce'},
  ],
};

/// Descriptor for `LinkIdentityRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List linkIdentityRequestDescriptor = $convert.base64Decode(
    'ChNMaW5rSWRlbnRpdHlSZXF1ZXN0EkcKCHByb3ZpZGVyGAEgASgOMiYuaGF3a3IuaWRlbnRpdH'
    'kudjFhbHBoYTEuTG9naW5Qcm92aWRlckID4EECUghwcm92aWRlchIeCghpZF90b2tlbhgCIAEo'
    'CUID4EECUgdpZFRva2VuEhkKBW5vbmNlGAMgASgJQgPgQQJSBW5vbmNl');

@$core.Deprecated('Use linkIdentityResponseDescriptor instead')
const LinkIdentityResponse$json = {
  '1': 'LinkIdentityResponse',
};

/// Descriptor for `LinkIdentityResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List linkIdentityResponseDescriptor =
    $convert.base64Decode('ChRMaW5rSWRlbnRpdHlSZXNwb25zZQ==');

@$core.Deprecated('Use listSessionsRequestDescriptor instead')
const ListSessionsRequest$json = {
  '1': 'ListSessionsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListSessionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSessionsRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0U2Vzc2lvbnNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQoKcG'
    'FnZV90b2tlbhgCIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listSessionsResponseDescriptor instead')
const ListSessionsResponse$json = {
  '1': 'ListSessionsResponse',
  '2': [
    {
      '1': 'sessions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.identity.v1alpha1.LoginSession',
      '10': 'sessions'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListSessionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSessionsResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0U2Vzc2lvbnNSZXNwb25zZRJBCghzZXNzaW9ucxgBIAMoCzIlLmhhd2tyLmlkZW50aX'
    'R5LnYxYWxwaGExLkxvZ2luU2Vzc2lvblIIc2Vzc2lvbnMSJgoPbmV4dF9wYWdlX3Rva2VuGAIg'
    'ASgJUg1uZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use revokeSessionRequestDescriptor instead')
const RevokeSessionRequest$json = {
  '1': 'RevokeSessionRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'sessionId'},
  ],
};

/// Descriptor for `RevokeSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeSessionRequestDescriptor = $convert.base64Decode(
    'ChRSZXZva2VTZXNzaW9uUmVxdWVzdBIiCgpzZXNzaW9uX2lkGAEgASgJQgPgQQJSCXNlc3Npb2'
    '5JZA==');

@$core.Deprecated('Use revokeSessionResponseDescriptor instead')
const RevokeSessionResponse$json = {
  '1': 'RevokeSessionResponse',
};

/// Descriptor for `RevokeSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeSessionResponseDescriptor =
    $convert.base64Decode('ChVSZXZva2VTZXNzaW9uUmVzcG9uc2U=');

@$core.Deprecated('Use loginSessionDescriptor instead')
const LoginSession$json = {
  '1': 'LoginSession',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'client_kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.identity.v1alpha1.ClientKind',
      '8': {},
      '10': 'clientKind'
    },
    {
      '1': 'create_time',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'last_use_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'lastUseTime'
    },
    {'1': 'current', '3': 5, '4': 1, '5': 8, '8': {}, '10': 'current'},
  ],
};

/// Descriptor for `LoginSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List loginSessionDescriptor = $convert.base64Decode(
    'CgxMb2dpblNlc3Npb24SEwoCaWQYASABKAlCA+BBCFICaWQSSQoLY2xpZW50X2tpbmQYAiABKA'
    '4yIy5oYXdrci5pZGVudGl0eS52MWFscGhhMS5DbGllbnRLaW5kQgPgQQNSCmNsaWVudEtpbmQS'
    'QAoLY3JlYXRlX3RpbWUYAyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCm'
    'NyZWF0ZVRpbWUSQwoNbGFzdF91c2VfdGltZRgEIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1l'
    'c3RhbXBCA+BBA1ILbGFzdFVzZVRpbWUSHQoHY3VycmVudBgFIAEoCEID4EEDUgdjdXJyZW50');

@$core.Deprecated('Use getSessionRequestDescriptor instead')
const GetSessionRequest$json = {
  '1': 'GetSessionRequest',
};

/// Descriptor for `GetSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSessionRequestDescriptor =
    $convert.base64Decode('ChFHZXRTZXNzaW9uUmVxdWVzdA==');

@$core.Deprecated('Use getSessionResponseDescriptor instead')
const GetSessionResponse$json = {
  '1': 'GetSessionResponse',
  '2': [
    {
      '1': 'account',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.identity.v1alpha1.Account',
      '10': 'account'
    },
    {
      '1': 'suspension',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.identity.v1alpha1.Suspension',
      '8': {},
      '10': 'suspension'
    },
  ],
  '9': [
    {'1': 1, '2': 2},
  ],
};

/// Descriptor for `GetSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSessionResponseDescriptor = $convert.base64Decode(
    'ChJHZXRTZXNzaW9uUmVzcG9uc2USOgoHYWNjb3VudBgCIAEoCzIgLmhhd2tyLmlkZW50aXR5Ln'
    'YxYWxwaGExLkFjY291bnRSB2FjY291bnQSSAoKc3VzcGVuc2lvbhgDIAEoCzIjLmhhd2tyLmlk'
    'ZW50aXR5LnYxYWxwaGExLlN1c3BlbnNpb25CA+BBA1IKc3VzcGVuc2lvbkoECAEQAg==');

@$core.Deprecated('Use suspensionDescriptor instead')
const Suspension$json = {
  '1': 'Suspension',
  '2': [
    {
      '1': 'suspended_at',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'suspendedAt'
    },
  ],
};

/// Descriptor for `Suspension`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suspensionDescriptor = $convert.base64Decode(
    'CgpTdXNwZW5zaW9uEkIKDHN1c3BlbmRlZF9hdBgBIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW'
    '1lc3RhbXBCA+BBA1ILc3VzcGVuZGVkQXQ=');

@$core.Deprecated('Use linkAppleCredentialRequestDescriptor instead')
const LinkAppleCredentialRequest$json = {
  '1': 'LinkAppleCredentialRequest',
  '2': [
    {'1': 'id_token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'idToken'},
    {
      '1': 'authorization_code',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'authorizationCode'
    },
  ],
};

/// Descriptor for `LinkAppleCredentialRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List linkAppleCredentialRequestDescriptor =
    $convert.base64Decode(
        'ChpMaW5rQXBwbGVDcmVkZW50aWFsUmVxdWVzdBIeCghpZF90b2tlbhgBIAEoCUID4EECUgdpZF'
        'Rva2VuEjIKEmF1dGhvcml6YXRpb25fY29kZRgCIAEoCUID4EECUhFhdXRob3JpemF0aW9uQ29k'
        'ZQ==');

@$core.Deprecated('Use linkAppleCredentialResponseDescriptor instead')
const LinkAppleCredentialResponse$json = {
  '1': 'LinkAppleCredentialResponse',
};

/// Descriptor for `LinkAppleCredentialResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List linkAppleCredentialResponseDescriptor =
    $convert.base64Decode('ChtMaW5rQXBwbGVDcmVkZW50aWFsUmVzcG9uc2U=');

@$core.Deprecated('Use deleteAccountRequestDescriptor instead')
const DeleteAccountRequest$json = {
  '1': 'DeleteAccountRequest',
};

/// Descriptor for `DeleteAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAccountRequestDescriptor =
    $convert.base64Decode('ChREZWxldGVBY2NvdW50UmVxdWVzdA==');

@$core.Deprecated('Use deleteAccountResponseDescriptor instead')
const DeleteAccountResponse$json = {
  '1': 'DeleteAccountResponse',
  '2': [
    {
      '1': 'state',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.identity.v1alpha1.DeleteAccountResponse.DeletionState',
      '10': 'state'
    },
  ],
  '4': [DeleteAccountResponse_DeletionState$json],
};

@$core.Deprecated('Use deleteAccountResponseDescriptor instead')
const DeleteAccountResponse_DeletionState$json = {
  '1': 'DeletionState',
  '2': [
    {'1': 'DELETION_STATE_UNSPECIFIED', '2': 0},
    {'1': 'DELETION_STATE_DELETED', '2': 1},
    {'1': 'DELETION_STATE_PENDING_ORDERS', '2': 2},
  ],
};

/// Descriptor for `DeleteAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAccountResponseDescriptor = $convert.base64Decode(
    'ChVEZWxldGVBY2NvdW50UmVzcG9uc2USUgoFc3RhdGUYASABKA4yPC5oYXdrci5pZGVudGl0eS'
    '52MWFscGhhMS5EZWxldGVBY2NvdW50UmVzcG9uc2UuRGVsZXRpb25TdGF0ZVIFc3RhdGUibgoN'
    'RGVsZXRpb25TdGF0ZRIeChpERUxFVElPTl9TVEFURV9VTlNQRUNJRklFRBAAEhoKFkRFTEVUSU'
    '9OX1NUQVRFX0RFTEVURUQQARIhCh1ERUxFVElPTl9TVEFURV9QRU5ESU5HX09SREVSUxAC');

const $core.Map<$core.String, $core.dynamic> IdentityPublicServiceBase$json = {
  '1': 'IdentityPublicService',
  '2': [
    {
      '1': 'IssueAppLoginNonce',
      '2': '.hawkr.identity.v1alpha1.IssueAppLoginNonceRequest',
      '3': '.hawkr.identity.v1alpha1.IssueAppLoginNonceResponse'
    },
    {
      '1': 'ExchangeIdToken',
      '2': '.hawkr.identity.v1alpha1.ExchangeIdTokenRequest',
      '3': '.hawkr.identity.v1alpha1.ExchangeIdTokenResponse'
    },
  ],
};

@$core.Deprecated('Use identityPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    IdentityPublicServiceBase$messageJson = {
  '.hawkr.identity.v1alpha1.IssueAppLoginNonceRequest':
      IssueAppLoginNonceRequest$json,
  '.hawkr.identity.v1alpha1.IssueAppLoginNonceResponse':
      IssueAppLoginNonceResponse$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.identity.v1alpha1.ExchangeIdTokenRequest':
      ExchangeIdTokenRequest$json,
  '.hawkr.identity.v1alpha1.ExchangeIdTokenResponse':
      ExchangeIdTokenResponse$json,
  '.google.protobuf.Duration': $1.Duration$json,
};

/// Descriptor for `IdentityPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List identityPublicServiceDescriptor = $convert.base64Decode(
    'ChVJZGVudGl0eVB1YmxpY1NlcnZpY2USfQoSSXNzdWVBcHBMb2dpbk5vbmNlEjIuaGF3a3IuaW'
    'RlbnRpdHkudjFhbHBoYTEuSXNzdWVBcHBMb2dpbk5vbmNlUmVxdWVzdBozLmhhd2tyLmlkZW50'
    'aXR5LnYxYWxwaGExLklzc3VlQXBwTG9naW5Ob25jZVJlc3BvbnNlEnQKD0V4Y2hhbmdlSWRUb2'
    'tlbhIvLmhhd2tyLmlkZW50aXR5LnYxYWxwaGExLkV4Y2hhbmdlSWRUb2tlblJlcXVlc3QaMC5o'
    'YXdrci5pZGVudGl0eS52MWFscGhhMS5FeGNoYW5nZUlkVG9rZW5SZXNwb25zZQ==');

const $core.Map<$core.String, $core.dynamic> IdentityAppServiceBase$json = {
  '1': 'IdentityAppService',
  '2': [
    {
      '1': 'GetSession',
      '2': '.hawkr.identity.v1alpha1.GetSessionRequest',
      '3': '.hawkr.identity.v1alpha1.GetSessionResponse',
      '4': {'34': 1},
    },
    {
      '1': 'LinkAppleCredential',
      '2': '.hawkr.identity.v1alpha1.LinkAppleCredentialRequest',
      '3': '.hawkr.identity.v1alpha1.LinkAppleCredentialResponse',
      '4': {'34': 2},
    },
    {
      '1': 'SignOut',
      '2': '.hawkr.identity.v1alpha1.SignOutRequest',
      '3': '.hawkr.identity.v1alpha1.SignOutResponse'
    },
    {
      '1': 'LinkIdentity',
      '2': '.hawkr.identity.v1alpha1.LinkIdentityRequest',
      '3': '.hawkr.identity.v1alpha1.LinkIdentityResponse'
    },
    {
      '1': 'ListSessions',
      '2': '.hawkr.identity.v1alpha1.ListSessionsRequest',
      '3': '.hawkr.identity.v1alpha1.ListSessionsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'RevokeSession',
      '2': '.hawkr.identity.v1alpha1.RevokeSessionRequest',
      '3': '.hawkr.identity.v1alpha1.RevokeSessionResponse'
    },
    {
      '1': 'DeleteAccount',
      '2': '.hawkr.identity.v1alpha1.DeleteAccountRequest',
      '3': '.hawkr.identity.v1alpha1.DeleteAccountResponse'
    },
  ],
};

@$core.Deprecated('Use identityAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    IdentityAppServiceBase$messageJson = {
  '.hawkr.identity.v1alpha1.GetSessionRequest': GetSessionRequest$json,
  '.hawkr.identity.v1alpha1.GetSessionResponse': GetSessionResponse$json,
  '.hawkr.identity.v1alpha1.Account': Account$json,
  '.hawkr.identity.v1alpha1.Suspension': Suspension$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.identity.v1alpha1.LinkAppleCredentialRequest':
      LinkAppleCredentialRequest$json,
  '.hawkr.identity.v1alpha1.LinkAppleCredentialResponse':
      LinkAppleCredentialResponse$json,
  '.hawkr.identity.v1alpha1.SignOutRequest': SignOutRequest$json,
  '.hawkr.identity.v1alpha1.SignOutResponse': SignOutResponse$json,
  '.hawkr.identity.v1alpha1.LinkIdentityRequest': LinkIdentityRequest$json,
  '.hawkr.identity.v1alpha1.LinkIdentityResponse': LinkIdentityResponse$json,
  '.hawkr.identity.v1alpha1.ListSessionsRequest': ListSessionsRequest$json,
  '.hawkr.identity.v1alpha1.ListSessionsResponse': ListSessionsResponse$json,
  '.hawkr.identity.v1alpha1.LoginSession': LoginSession$json,
  '.hawkr.identity.v1alpha1.RevokeSessionRequest': RevokeSessionRequest$json,
  '.hawkr.identity.v1alpha1.RevokeSessionResponse': RevokeSessionResponse$json,
  '.hawkr.identity.v1alpha1.DeleteAccountRequest': DeleteAccountRequest$json,
  '.hawkr.identity.v1alpha1.DeleteAccountResponse': DeleteAccountResponse$json,
};

/// Descriptor for `IdentityAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List identityAppServiceDescriptor = $convert.base64Decode(
    'ChJJZGVudGl0eUFwcFNlcnZpY2USagoKR2V0U2Vzc2lvbhIqLmhhd2tyLmlkZW50aXR5LnYxYW'
    'xwaGExLkdldFNlc3Npb25SZXF1ZXN0GisuaGF3a3IuaWRlbnRpdHkudjFhbHBoYTEuR2V0U2Vz'
    'c2lvblJlc3BvbnNlIgOQAgEShQEKE0xpbmtBcHBsZUNyZWRlbnRpYWwSMy5oYXdrci5pZGVudG'
    'l0eS52MWFscGhhMS5MaW5rQXBwbGVDcmVkZW50aWFsUmVxdWVzdBo0Lmhhd2tyLmlkZW50aXR5'
    'LnYxYWxwaGExLkxpbmtBcHBsZUNyZWRlbnRpYWxSZXNwb25zZSIDkAICElwKB1NpZ25PdXQSJy'
    '5oYXdrci5pZGVudGl0eS52MWFscGhhMS5TaWduT3V0UmVxdWVzdBooLmhhd2tyLmlkZW50aXR5'
    'LnYxYWxwaGExLlNpZ25PdXRSZXNwb25zZRJrCgxMaW5rSWRlbnRpdHkSLC5oYXdrci5pZGVudG'
    'l0eS52MWFscGhhMS5MaW5rSWRlbnRpdHlSZXF1ZXN0Gi0uaGF3a3IuaWRlbnRpdHkudjFhbHBo'
    'YTEuTGlua0lkZW50aXR5UmVzcG9uc2UScAoMTGlzdFNlc3Npb25zEiwuaGF3a3IuaWRlbnRpdH'
    'kudjFhbHBoYTEuTGlzdFNlc3Npb25zUmVxdWVzdBotLmhhd2tyLmlkZW50aXR5LnYxYWxwaGEx'
    'Lkxpc3RTZXNzaW9uc1Jlc3BvbnNlIgOQAgESbgoNUmV2b2tlU2Vzc2lvbhItLmhhd2tyLmlkZW'
    '50aXR5LnYxYWxwaGExLlJldm9rZVNlc3Npb25SZXF1ZXN0Gi4uaGF3a3IuaWRlbnRpdHkudjFh'
    'bHBoYTEuUmV2b2tlU2Vzc2lvblJlc3BvbnNlEm4KDURlbGV0ZUFjY291bnQSLS5oYXdrci5pZG'
    'VudGl0eS52MWFscGhhMS5EZWxldGVBY2NvdW50UmVxdWVzdBouLmhhd2tyLmlkZW50aXR5LnYx'
    'YWxwaGExLkRlbGV0ZUFjY291bnRSZXNwb25zZQ==');
