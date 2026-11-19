// This is a generated file - do not edit.
//
// Generated from hawkr/moderation/v1alpha1/moderation.proto.

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

@$core.Deprecated('Use reportTargetKindDescriptor instead')
const ReportTargetKind$json = {
  '1': 'ReportTargetKind',
  '2': [
    {'1': 'REPORT_TARGET_KIND_UNSPECIFIED', '2': 0},
    {'1': 'REPORT_TARGET_KIND_POST', '2': 1},
    {'1': 'REPORT_TARGET_KIND_COMMENT', '2': 2},
    {'1': 'REPORT_TARGET_KIND_FORUM_ENTRY', '2': 3},
    {'1': 'REPORT_TARGET_KIND_BUYER', '2': 4},
  ],
};

/// Descriptor for `ReportTargetKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List reportTargetKindDescriptor = $convert.base64Decode(
    'ChBSZXBvcnRUYXJnZXRLaW5kEiIKHlJFUE9SVF9UQVJHRVRfS0lORF9VTlNQRUNJRklFRBAAEh'
    'sKF1JFUE9SVF9UQVJHRVRfS0lORF9QT1NUEAESHgoaUkVQT1JUX1RBUkdFVF9LSU5EX0NPTU1F'
    'TlQQAhIiCh5SRVBPUlRfVEFSR0VUX0tJTkRfRk9SVU1fRU5UUlkQAxIcChhSRVBPUlRfVEFSR0'
    'VUX0tJTkRfQlVZRVIQBA==');

@$core.Deprecated('Use reportOutcomeDescriptor instead')
const ReportOutcome$json = {
  '1': 'ReportOutcome',
  '2': [
    {'1': 'REPORT_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'REPORT_OUTCOME_PENDING', '2': 1},
    {'1': 'REPORT_OUTCOME_VIOLATION', '2': 2},
    {'1': 'REPORT_OUTCOME_NO_VIOLATION', '2': 3},
  ],
};

/// Descriptor for `ReportOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List reportOutcomeDescriptor = $convert.base64Decode(
    'Cg1SZXBvcnRPdXRjb21lEh4KGlJFUE9SVF9PVVRDT01FX1VOU1BFQ0lGSUVEEAASGgoWUkVQT1'
    'JUX09VVENPTUVfUEVORElORxABEhwKGFJFUE9SVF9PVVRDT01FX1ZJT0xBVElPThACEh8KG1JF'
    'UE9SVF9PVVRDT01FX05PX1ZJT0xBVElPThAD');

@$core.Deprecated('Use reportTargetDescriptor instead')
const ReportTarget$json = {
  '1': 'ReportTarget',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.moderation.v1alpha1.ReportTargetKind',
      '8': {},
      '10': 'kind'
    },
    {'1': 'id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'id'},
  ],
};

/// Descriptor for `ReportTarget`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportTargetDescriptor = $convert.base64Decode(
    'CgxSZXBvcnRUYXJnZXQSRAoEa2luZBgBIAEoDjIrLmhhd2tyLm1vZGVyYXRpb24udjFhbHBoYT'
    'EuUmVwb3J0VGFyZ2V0S2luZEID4EECUgRraW5kEhMKAmlkGAIgASgJQgPgQQJSAmlk');

@$core.Deprecated('Use submitReportRequestDescriptor instead')
const SubmitReportRequest$json = {
  '1': 'SubmitReportRequest',
  '2': [
    {
      '1': 'target',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.moderation.v1alpha1.ReportTarget',
      '8': {},
      '10': 'target'
    },
    {
      '1': 'category',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '8': {},
      '10': 'category'
    },
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
    {'1': 'request_id', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
};

/// Descriptor for `SubmitReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitReportRequestDescriptor = $convert.base64Decode(
    'ChNTdWJtaXRSZXBvcnRSZXF1ZXN0EkQKBnRhcmdldBgBIAEoCzInLmhhd2tyLm1vZGVyYXRpb2'
    '4udjFhbHBoYTEuUmVwb3J0VGFyZ2V0QgPgQQJSBnRhcmdldBJGCghjYXRlZ29yeRgCIAEoDjIl'
    'Lmhhd2tyLmNvbW1vbi52MWFscGhhMS5Qb2xpY3lDYXRlZ29yeUID4EECUghjYXRlZ29yeRIgCg'
    'tkZXNjcmlwdGlvbhgDIAEoCVILZGVzY3JpcHRpb24SJwoKcmVxdWVzdF9pZBgEIAEoCUII4ozP'
    '1wgCCAFSCXJlcXVlc3RJZA==');

@$core.Deprecated('Use submitReportResponseDescriptor instead')
const SubmitReportResponse$json = {
  '1': 'SubmitReportResponse',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '10': 'reportId'},
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

/// Descriptor for `SubmitReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitReportResponseDescriptor = $convert.base64Decode(
    'ChRTdWJtaXRSZXBvcnRSZXNwb25zZRIbCglyZXBvcnRfaWQYASABKAlSCHJlcG9ydElkEjsKC2'
    'NyZWF0ZV90aW1lGAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKY3JlYXRlVGlt'
    'ZQ==');

@$core.Deprecated('Use listMyReportsRequestDescriptor instead')
const ListMyReportsRequest$json = {
  '1': 'ListMyReportsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListMyReportsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyReportsRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0TXlSZXBvcnRzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCn'
    'BhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listMyReportsResponseDescriptor instead')
const ListMyReportsResponse$json = {
  '1': 'ListMyReportsResponse',
  '2': [
    {
      '1': 'reports',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.moderation.v1alpha1.ReportRecord',
      '10': 'reports'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyReportsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyReportsResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0TXlSZXBvcnRzUmVzcG9uc2USQQoHcmVwb3J0cxgBIAMoCzInLmhhd2tyLm1vZGVyYX'
    'Rpb24udjFhbHBoYTEuUmVwb3J0UmVjb3JkUgdyZXBvcnRzEiYKD25leHRfcGFnZV90b2tlbhgC'
    'IAEoCVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use reportRecordDescriptor instead')
const ReportRecord$json = {
  '1': 'ReportRecord',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'target',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.moderation.v1alpha1.ReportTarget',
      '10': 'target'
    },
    {
      '1': 'category',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '10': 'category'
    },
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'outcome',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.moderation.v1alpha1.ReportOutcome',
      '10': 'outcome'
    },
    {
      '1': 'create_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
    {
      '1': 'resolve_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'resolveTime',
      '17': true
    },
  ],
  '8': [
    {'1': '_resolve_time'},
  ],
};

/// Descriptor for `ReportRecord`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportRecordDescriptor = $convert.base64Decode(
    'CgxSZXBvcnRSZWNvcmQSDgoCaWQYASABKAlSAmlkEj8KBnRhcmdldBgCIAEoCzInLmhhd2tyLm'
    '1vZGVyYXRpb24udjFhbHBoYTEuUmVwb3J0VGFyZ2V0UgZ0YXJnZXQSQQoIY2F0ZWdvcnkYAyAB'
    'KA4yJS5oYXdrci5jb21tb24udjFhbHBoYTEuUG9saWN5Q2F0ZWdvcnlSCGNhdGVnb3J5EiAKC2'
    'Rlc2NyaXB0aW9uGAQgASgJUgtkZXNjcmlwdGlvbhJCCgdvdXRjb21lGAUgASgOMiguaGF3a3Iu'
    'bW9kZXJhdGlvbi52MWFscGhhMS5SZXBvcnRPdXRjb21lUgdvdXRjb21lEjsKC2NyZWF0ZV90aW'
    '1lGAYgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKY3JlYXRlVGltZRJCCgxyZXNv'
    'bHZlX3RpbWUYByABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSABSC3Jlc29sdmVUaW'
    '1liAEBQg8KDV9yZXNvbHZlX3RpbWU=');

const $core.Map<$core.String, $core.dynamic> ModerationAppServiceBase$json = {
  '1': 'ModerationAppService',
  '2': [
    {
      '1': 'SubmitReport',
      '2': '.hawkr.moderation.v1alpha1.SubmitReportRequest',
      '3': '.hawkr.moderation.v1alpha1.SubmitReportResponse'
    },
    {
      '1': 'ListMyReports',
      '2': '.hawkr.moderation.v1alpha1.ListMyReportsRequest',
      '3': '.hawkr.moderation.v1alpha1.ListMyReportsResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use moderationAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    ModerationAppServiceBase$messageJson = {
  '.hawkr.moderation.v1alpha1.SubmitReportRequest': SubmitReportRequest$json,
  '.hawkr.moderation.v1alpha1.ReportTarget': ReportTarget$json,
  '.hawkr.moderation.v1alpha1.SubmitReportResponse': SubmitReportResponse$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.hawkr.moderation.v1alpha1.ListMyReportsRequest': ListMyReportsRequest$json,
  '.hawkr.moderation.v1alpha1.ListMyReportsResponse':
      ListMyReportsResponse$json,
  '.hawkr.moderation.v1alpha1.ReportRecord': ReportRecord$json,
};

/// Descriptor for `ModerationAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List moderationAppServiceDescriptor = $convert.base64Decode(
    'ChRNb2RlcmF0aW9uQXBwU2VydmljZRJvCgxTdWJtaXRSZXBvcnQSLi5oYXdrci5tb2RlcmF0aW'
    '9uLnYxYWxwaGExLlN1Ym1pdFJlcG9ydFJlcXVlc3QaLy5oYXdrci5tb2RlcmF0aW9uLnYxYWxw'
    'aGExLlN1Ym1pdFJlcG9ydFJlc3BvbnNlEncKDUxpc3RNeVJlcG9ydHMSLy5oYXdrci5tb2Rlcm'
    'F0aW9uLnYxYWxwaGExLkxpc3RNeVJlcG9ydHNSZXF1ZXN0GjAuaGF3a3IubW9kZXJhdGlvbi52'
    'MWFscGhhMS5MaXN0TXlSZXBvcnRzUmVzcG9uc2UiA5ACAQ==');
