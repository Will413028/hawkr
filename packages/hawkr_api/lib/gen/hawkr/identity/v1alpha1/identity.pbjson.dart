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
  '.hawkr.identity.v1alpha1.DeleteAccountRequest': DeleteAccountRequest$json,
  '.hawkr.identity.v1alpha1.DeleteAccountResponse': DeleteAccountResponse$json,
};

/// Descriptor for `IdentityAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List identityAppServiceDescriptor = $convert.base64Decode(
    'ChJJZGVudGl0eUFwcFNlcnZpY2USagoKR2V0U2Vzc2lvbhIqLmhhd2tyLmlkZW50aXR5LnYxYW'
    'xwaGExLkdldFNlc3Npb25SZXF1ZXN0GisuaGF3a3IuaWRlbnRpdHkudjFhbHBoYTEuR2V0U2Vz'
    'c2lvblJlc3BvbnNlIgOQAgEShQEKE0xpbmtBcHBsZUNyZWRlbnRpYWwSMy5oYXdrci5pZGVudG'
    'l0eS52MWFscGhhMS5MaW5rQXBwbGVDcmVkZW50aWFsUmVxdWVzdBo0Lmhhd2tyLmlkZW50aXR5'
    'LnYxYWxwaGExLkxpbmtBcHBsZUNyZWRlbnRpYWxSZXNwb25zZSIDkAICEm4KDURlbGV0ZUFjY2'
    '91bnQSLS5oYXdrci5pZGVudGl0eS52MWFscGhhMS5EZWxldGVBY2NvdW50UmVxdWVzdBouLmhh'
    'd2tyLmlkZW50aXR5LnYxYWxwaGExLkRlbGV0ZUFjY291bnRSZXNwb25zZQ==');
