// This is a generated file - do not edit.
//
// Generated from hawkr/media/v1alpha1/media.proto.

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

@$core.Deprecated('Use uploadStateDescriptor instead')
const UploadState$json = {
  '1': 'UploadState',
  '2': [
    {'1': 'UPLOAD_STATE_UNSPECIFIED', '2': 0},
    {'1': 'UPLOAD_STATE_PENDING', '2': 1},
    {'1': 'UPLOAD_STATE_UPLOADED', '2': 2},
    {'1': 'UPLOAD_STATE_READY', '2': 3},
    {'1': 'UPLOAD_STATE_REJECTED', '2': 4},
    {'1': 'UPLOAD_STATE_CONFIRMED', '2': 5},
    {'1': 'UPLOAD_STATE_DELETING', '2': 6},
    {'1': 'UPLOAD_STATE_DELETED', '2': 7},
  ],
};

/// Descriptor for `UploadState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List uploadStateDescriptor = $convert.base64Decode(
    'CgtVcGxvYWRTdGF0ZRIcChhVUExPQURfU1RBVEVfVU5TUEVDSUZJRUQQABIYChRVUExPQURfU1'
    'RBVEVfUEVORElORxABEhkKFVVQTE9BRF9TVEFURV9VUExPQURFRBACEhYKElVQTE9BRF9TVEFU'
    'RV9SRUFEWRADEhkKFVVQTE9BRF9TVEFURV9SRUpFQ1RFRBAEEhoKFlVQTE9BRF9TVEFURV9DT0'
    '5GSVJNRUQQBRIZChVVUExPQURfU1RBVEVfREVMRVRJTkcQBhIYChRVUExPQURfU1RBVEVfREVM'
    'RVRFRBAH');

@$core.Deprecated('Use rejectReasonDescriptor instead')
const RejectReason$json = {
  '1': 'RejectReason',
  '2': [
    {'1': 'REJECT_REASON_UNSPECIFIED', '2': 0},
    {'1': 'REJECT_REASON_UNRECOGNIZED_FORMAT', '2': 1},
    {'1': 'REJECT_REASON_SIZE_MISMATCH', '2': 2},
    {'1': 'REJECT_REASON_CHECKSUM_MISMATCH', '2': 3},
    {'1': 'REJECT_REASON_PROCESSING_FAILED', '2': 4},
    {'1': 'REJECT_REASON_EXPIRED', '2': 5},
  ],
};

/// Descriptor for `RejectReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List rejectReasonDescriptor = $convert.base64Decode(
    'CgxSZWplY3RSZWFzb24SHQoZUkVKRUNUX1JFQVNPTl9VTlNQRUNJRklFRBAAEiUKIVJFSkVDVF'
    '9SRUFTT05fVU5SRUNPR05JWkVEX0ZPUk1BVBABEh8KG1JFSkVDVF9SRUFTT05fU0laRV9NSVNN'
    'QVRDSBACEiMKH1JFSkVDVF9SRUFTT05fQ0hFQ0tTVU1fTUlTTUFUQ0gQAxIjCh9SRUpFQ1RfUk'
    'VBU09OX1BST0NFU1NJTkdfRkFJTEVEEAQSGQoVUkVKRUNUX1JFQVNPTl9FWFBJUkVEEAU=');

@$core.Deprecated('Use beginUploadRequestDescriptor instead')
const BeginUploadRequest$json = {
  '1': 'BeginUploadRequest',
  '2': [
    {'1': 'size', '3': 1, '4': 1, '5': 3, '8': {}, '10': 'size'},
    {'1': 'content_type', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'contentType'},
    {'1': 'md5', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'md5'},
  ],
};

/// Descriptor for `BeginUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List beginUploadRequestDescriptor = $convert.base64Decode(
    'ChJCZWdpblVwbG9hZFJlcXVlc3QSFwoEc2l6ZRgBIAEoA0ID4EECUgRzaXplEiYKDGNvbnRlbn'
    'RfdHlwZRgCIAEoCUID4EECUgtjb250ZW50VHlwZRIVCgNtZDUYAyABKAlCA+BBAlIDbWQ1');

@$core.Deprecated('Use beginUploadResponseDescriptor instead')
const BeginUploadResponse$json = {
  '1': 'BeginUploadResponse',
  '2': [
    {'1': 'staged_token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'stagedToken'},
    {'1': 'upload_url', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'uploadUrl'},
    {
      '1': 'upload_headers',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.media.v1alpha1.BeginUploadResponse.UploadHeadersEntry',
      '8': {},
      '10': 'uploadHeaders'
    },
    {
      '1': 'expire_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'expireTime'
    },
  ],
  '3': [BeginUploadResponse_UploadHeadersEntry$json],
};

@$core.Deprecated('Use beginUploadResponseDescriptor instead')
const BeginUploadResponse_UploadHeadersEntry$json = {
  '1': 'UploadHeadersEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `BeginUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List beginUploadResponseDescriptor = $convert.base64Decode(
    'ChNCZWdpblVwbG9hZFJlc3BvbnNlEiYKDHN0YWdlZF90b2tlbhgBIAEoCUID4EEDUgtzdGFnZW'
    'RUb2tlbhIiCgp1cGxvYWRfdXJsGAIgASgJQgPgQQNSCXVwbG9hZFVybBJoCg51cGxvYWRfaGVh'
    'ZGVycxgDIAMoCzI8Lmhhd2tyLm1lZGlhLnYxYWxwaGExLkJlZ2luVXBsb2FkUmVzcG9uc2UuVX'
    'Bsb2FkSGVhZGVyc0VudHJ5QgPgQQNSDXVwbG9hZEhlYWRlcnMSQAoLZXhwaXJlX3RpbWUYBCAB'
    'KAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCmV4cGlyZVRpbWUaQAoSVXBsb2'
    'FkSGVhZGVyc0VudHJ5EhAKA2tleRgBIAEoCVIDa2V5EhQKBXZhbHVlGAIgASgJUgV2YWx1ZToC'
    'OAE=');

@$core.Deprecated('Use completeUploadRequestDescriptor instead')
const CompleteUploadRequest$json = {
  '1': 'CompleteUploadRequest',
  '2': [
    {'1': 'staged_token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'stagedToken'},
  ],
};

/// Descriptor for `CompleteUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeUploadRequestDescriptor = $convert.base64Decode(
    'ChVDb21wbGV0ZVVwbG9hZFJlcXVlc3QSJgoMc3RhZ2VkX3Rva2VuGAEgASgJQgPgQQJSC3N0YW'
    'dlZFRva2Vu');

@$core.Deprecated('Use completeUploadResponseDescriptor instead')
const CompleteUploadResponse$json = {
  '1': 'CompleteUploadResponse',
};

/// Descriptor for `CompleteUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeUploadResponseDescriptor =
    $convert.base64Decode('ChZDb21wbGV0ZVVwbG9hZFJlc3BvbnNl');

@$core.Deprecated('Use getUploadRequestDescriptor instead')
const GetUploadRequest$json = {
  '1': 'GetUploadRequest',
  '2': [
    {'1': 'staged_token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'stagedToken'},
  ],
};

/// Descriptor for `GetUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUploadRequestDescriptor = $convert.base64Decode(
    'ChBHZXRVcGxvYWRSZXF1ZXN0EiYKDHN0YWdlZF90b2tlbhgBIAEoCUID4EECUgtzdGFnZWRUb2'
    'tlbg==');

@$core.Deprecated('Use getUploadResponseDescriptor instead')
const GetUploadResponse$json = {
  '1': 'GetUploadResponse',
  '2': [
    {
      '1': 'state',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.media.v1alpha1.UploadState',
      '8': {},
      '10': 'state'
    },
    {
      '1': 'reject_reason',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.media.v1alpha1.RejectReason',
      '8': {},
      '9': 0,
      '10': 'rejectReason',
      '17': true
    },
    {
      '1': 'expire_time',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'expireTime'
    },
  ],
  '8': [
    {'1': '_reject_reason'},
  ],
};

/// Descriptor for `GetUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUploadResponseDescriptor = $convert.base64Decode(
    'ChFHZXRVcGxvYWRSZXNwb25zZRI8CgVzdGF0ZRgBIAEoDjIhLmhhd2tyLm1lZGlhLnYxYWxwaG'
    'ExLlVwbG9hZFN0YXRlQgPgQQNSBXN0YXRlElEKDXJlamVjdF9yZWFzb24YAiABKA4yIi5oYXdr'
    'ci5tZWRpYS52MWFscGhhMS5SZWplY3RSZWFzb25CA+BBA0gAUgxyZWplY3RSZWFzb26IAQESQA'
    'oLZXhwaXJlX3RpbWUYAyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCmV4'
    'cGlyZVRpbWVCEAoOX3JlamVjdF9yZWFzb24=');

@$core.Deprecated('Use beginSecureUploadRequestDescriptor instead')
const BeginSecureUploadRequest$json = {
  '1': 'BeginSecureUploadRequest',
  '2': [
    {'1': 'size', '3': 1, '4': 1, '5': 3, '8': {}, '10': 'size'},
  ],
};

/// Descriptor for `BeginSecureUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List beginSecureUploadRequestDescriptor =
    $convert.base64Decode(
        'ChhCZWdpblNlY3VyZVVwbG9hZFJlcXVlc3QSFwoEc2l6ZRgBIAEoA0ID4EECUgRzaXpl');

@$core.Deprecated('Use beginSecureUploadResponseDescriptor instead')
const BeginSecureUploadResponse$json = {
  '1': 'BeginSecureUploadResponse',
  '2': [
    {'1': 'staged_token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'stagedToken'},
    {'1': 'chunk_size', '3': 2, '4': 1, '5': 5, '8': {}, '10': 'chunkSize'},
    {'1': 'chunk_count', '3': 3, '4': 1, '5': 5, '8': {}, '10': 'chunkCount'},
    {
      '1': 'expire_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'expireTime'
    },
  ],
};

/// Descriptor for `BeginSecureUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List beginSecureUploadResponseDescriptor = $convert.base64Decode(
    'ChlCZWdpblNlY3VyZVVwbG9hZFJlc3BvbnNlEiYKDHN0YWdlZF90b2tlbhgBIAEoCUID4EEDUg'
    'tzdGFnZWRUb2tlbhIiCgpjaHVua19zaXplGAIgASgFQgPgQQNSCWNodW5rU2l6ZRIkCgtjaHVu'
    'a19jb3VudBgDIAEoBUID4EEDUgpjaHVua0NvdW50EkAKC2V4cGlyZV90aW1lGAQgASgLMhouZ2'
    '9vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpleHBpcmVUaW1l');

const $core.Map<$core.String, $core.dynamic> MediaAppServiceBase$json = {
  '1': 'MediaAppService',
  '2': [
    {
      '1': 'BeginUpload',
      '2': '.hawkr.media.v1alpha1.BeginUploadRequest',
      '3': '.hawkr.media.v1alpha1.BeginUploadResponse'
    },
    {
      '1': 'CompleteUpload',
      '2': '.hawkr.media.v1alpha1.CompleteUploadRequest',
      '3': '.hawkr.media.v1alpha1.CompleteUploadResponse',
      '4': {'34': 2},
    },
    {
      '1': 'GetUpload',
      '2': '.hawkr.media.v1alpha1.GetUploadRequest',
      '3': '.hawkr.media.v1alpha1.GetUploadResponse'
    },
    {
      '1': 'BeginSecureUpload',
      '2': '.hawkr.media.v1alpha1.BeginSecureUploadRequest',
      '3': '.hawkr.media.v1alpha1.BeginSecureUploadResponse'
    },
  ],
};

@$core.Deprecated('Use mediaAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    MediaAppServiceBase$messageJson = {
  '.hawkr.media.v1alpha1.BeginUploadRequest': BeginUploadRequest$json,
  '.hawkr.media.v1alpha1.BeginUploadResponse': BeginUploadResponse$json,
  '.hawkr.media.v1alpha1.BeginUploadResponse.UploadHeadersEntry':
      BeginUploadResponse_UploadHeadersEntry$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.media.v1alpha1.CompleteUploadRequest': CompleteUploadRequest$json,
  '.hawkr.media.v1alpha1.CompleteUploadResponse': CompleteUploadResponse$json,
  '.hawkr.media.v1alpha1.GetUploadRequest': GetUploadRequest$json,
  '.hawkr.media.v1alpha1.GetUploadResponse': GetUploadResponse$json,
  '.hawkr.media.v1alpha1.BeginSecureUploadRequest':
      BeginSecureUploadRequest$json,
  '.hawkr.media.v1alpha1.BeginSecureUploadResponse':
      BeginSecureUploadResponse$json,
};

/// Descriptor for `MediaAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List mediaAppServiceDescriptor = $convert.base64Decode(
    'Cg9NZWRpYUFwcFNlcnZpY2USYgoLQmVnaW5VcGxvYWQSKC5oYXdrci5tZWRpYS52MWFscGhhMS'
    '5CZWdpblVwbG9hZFJlcXVlc3QaKS5oYXdrci5tZWRpYS52MWFscGhhMS5CZWdpblVwbG9hZFJl'
    'c3BvbnNlEnAKDkNvbXBsZXRlVXBsb2FkEisuaGF3a3IubWVkaWEudjFhbHBoYTEuQ29tcGxldG'
    'VVcGxvYWRSZXF1ZXN0GiwuaGF3a3IubWVkaWEudjFhbHBoYTEuQ29tcGxldGVVcGxvYWRSZXNw'
    'b25zZSIDkAICElwKCUdldFVwbG9hZBImLmhhd2tyLm1lZGlhLnYxYWxwaGExLkdldFVwbG9hZF'
    'JlcXVlc3QaJy5oYXdrci5tZWRpYS52MWFscGhhMS5HZXRVcGxvYWRSZXNwb25zZRJ0ChFCZWdp'
    'blNlY3VyZVVwbG9hZBIuLmhhd2tyLm1lZGlhLnYxYWxwaGExLkJlZ2luU2VjdXJlVXBsb2FkUm'
    'VxdWVzdBovLmhhd2tyLm1lZGlhLnYxYWxwaGExLkJlZ2luU2VjdXJlVXBsb2FkUmVzcG9uc2U=');
