// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/report.proto.

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

import '../../../google/rpc/error_details.pbjson.dart' as $1;

@$core.Deprecated('Use reportOutcomeDescriptor instead')
const ReportOutcome$json = {
  '1': 'ReportOutcome',
  '2': [
    {'1': 'REPORT_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'REPORT_OUTCOME_ACTION_TAKEN', '2': 1},
    {'1': 'REPORT_OUTCOME_NO_VIOLATION', '2': 2},
    {'1': 'REPORT_OUTCOME_INSUFFICIENT', '2': 3},
    {'1': 'REPORT_OUTCOME_DUPLICATE', '2': 4},
    {'1': 'REPORT_OUTCOME_ABUSIVE', '2': 5},
  ],
};

/// Descriptor for `ReportOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List reportOutcomeDescriptor = $convert.base64Decode(
    'Cg1SZXBvcnRPdXRjb21lEh4KGlJFUE9SVF9PVVRDT01FX1VOU1BFQ0lGSUVEEAASHwobUkVQT1'
    'JUX09VVENPTUVfQUNUSU9OX1RBS0VOEAESHwobUkVQT1JUX09VVENPTUVfTk9fVklPTEFUSU9O'
    'EAISHwobUkVQT1JUX09VVENPTUVfSU5TVUZGSUNJRU5UEAMSHAoYUkVQT1JUX09VVENPTUVfRF'
    'VQTElDQVRFEAQSGgoWUkVQT1JUX09VVENPTUVfQUJVU0lWRRAF');

@$core.Deprecated('Use storeReportDescriptor instead')
const StoreReport$json = {
  '1': 'StoreReport',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'store_id', '3': 2, '4': 1, '5': 9, '10': 'storeId'},
    {'1': 'product_id', '3': 3, '4': 1, '5': 9, '10': 'productId'},
    {'1': 'order_no', '3': 4, '4': 1, '5': 9, '10': 'orderNo'},
    {
      '1': 'category',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '10': 'category'
    },
    {'1': 'description', '3': 6, '4': 1, '5': 9, '10': 'description'},
    {'1': 'resolution_note', '3': 7, '4': 1, '5': 9, '10': 'resolutionNote'},
    {
      '1': 'create_time',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
    {
      '1': 'resolve_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'resolveTime'
    },
    {
      '1': 'outcome',
      '3': 10,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.ReportOutcome',
      '8': {},
      '9': 0,
      '10': 'outcome',
      '17': true
    },
  ],
  '8': [
    {'1': '_outcome'},
  ],
};

/// Descriptor for `StoreReport`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storeReportDescriptor = $convert.base64Decode(
    'CgtTdG9yZVJlcG9ydBITCgJpZBgBIAEoCUID4EEIUgJpZBIZCghzdG9yZV9pZBgCIAEoCVIHc3'
    'RvcmVJZBIdCgpwcm9kdWN0X2lkGAMgASgJUglwcm9kdWN0SWQSGQoIb3JkZXJfbm8YBCABKAlS'
    'B29yZGVyTm8SQQoIY2F0ZWdvcnkYBSABKA4yJS5oYXdrci5jb21tb24udjFhbHBoYTEuUG9saW'
    'N5Q2F0ZWdvcnlSCGNhdGVnb3J5EiAKC2Rlc2NyaXB0aW9uGAYgASgJUgtkZXNjcmlwdGlvbhIn'
    'Cg9yZXNvbHV0aW9uX25vdGUYByABKAlSDnJlc29sdXRpb25Ob3RlEjsKC2NyZWF0ZV90aW1lGA'
    'ggASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKY3JlYXRlVGltZRI9CgxyZXNvbHZl'
    'X3RpbWUYCSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgtyZXNvbHZlVGltZRJICg'
    'dvdXRjb21lGAogASgOMiQuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlJlcG9ydE91dGNvbWVCA+BB'
    'A0gAUgdvdXRjb21liAEBQgoKCF9vdXRjb21l');

@$core.Deprecated('Use createReportRequestDescriptor instead')
const CreateReportRequest$json = {
  '1': 'CreateReportRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {
      '1': 'category',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '8': {},
      '10': 'category'
    },
    {'1': 'description', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'description'},
    {'1': 'product_id', '3': 4, '4': 1, '5': 9, '10': 'productId'},
    {'1': 'order_no', '3': 5, '4': 1, '5': 9, '10': 'orderNo'},
    {
      '1': 'evidence_staged_tokens',
      '3': 6,
      '4': 3,
      '5': 9,
      '10': 'evidenceStagedTokens'
    },
  ],
};

/// Descriptor for `CreateReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createReportRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVSZXBvcnRSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3JlSWQSVg'
    'oIY2F0ZWdvcnkYAiABKA4yJS5oYXdrci5jb21tb24udjFhbHBoYTEuUG9saWN5Q2F0ZWdvcnlC'
    'E+BBAoq1GAwBAgMEBQYHCAkKCw1SCGNhdGVnb3J5EiUKC2Rlc2NyaXB0aW9uGAMgASgJQgPgQQ'
    'JSC2Rlc2NyaXB0aW9uEh0KCnByb2R1Y3RfaWQYBCABKAlSCXByb2R1Y3RJZBIZCghvcmRlcl9u'
    'bxgFIAEoCVIHb3JkZXJObxI0ChZldmlkZW5jZV9zdGFnZWRfdG9rZW5zGAYgAygJUhRldmlkZW'
    '5jZVN0YWdlZFRva2Vucw==');

@$core.Deprecated('Use createReportResponseDescriptor instead')
const CreateReportResponse$json = {
  '1': 'CreateReportResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.StoreReport',
      '10': 'report'
    },
    {
      '1': 'supplemented',
      '3': 2,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'supplemented'
    },
    {
      '1': 'notice',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.rpc.LocalizedMessage',
      '8': {},
      '10': 'notice'
    },
  ],
};

/// Descriptor for `CreateReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createReportResponseDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVSZXBvcnRSZXNwb25zZRI6CgZyZXBvcnQYASABKAsyIi5oYXdrci5zZWxsZXIudj'
    'FhbHBoYTEuU3RvcmVSZXBvcnRSBnJlcG9ydBInCgxzdXBwbGVtZW50ZWQYAiABKAhCA+BBA1IM'
    'c3VwcGxlbWVudGVkEjkKBm5vdGljZRgDIAEoCzIcLmdvb2dsZS5ycGMuTG9jYWxpemVkTWVzc2'
    'FnZUID4EEDUgZub3RpY2U=');

@$core.Deprecated('Use getMyReportRequestDescriptor instead')
const GetMyReportRequest$json = {
  '1': 'GetMyReportRequest',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'reportId'},
  ],
};

/// Descriptor for `GetMyReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyReportRequestDescriptor = $convert.base64Decode(
    'ChJHZXRNeVJlcG9ydFJlcXVlc3QSIAoJcmVwb3J0X2lkGAEgASgJQgPgQQJSCHJlcG9ydElk');

@$core.Deprecated('Use getMyReportResponseDescriptor instead')
const GetMyReportResponse$json = {
  '1': 'GetMyReportResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.StoreReport',
      '10': 'report'
    },
    {
      '1': 'supplements',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.ReportSupplement',
      '10': 'supplements'
    },
    {
      '1': 'evidence',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.ReportEvidence',
      '10': 'evidence'
    },
    {
      '1': 'can_supplement',
      '3': 4,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'canSupplement'
    },
    {
      '1': 'supplement_max',
      '3': 5,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'supplementMax'
    },
  ],
};

/// Descriptor for `GetMyReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyReportResponseDescriptor = $convert.base64Decode(
    'ChNHZXRNeVJlcG9ydFJlc3BvbnNlEjoKBnJlcG9ydBgBIAEoCzIiLmhhd2tyLnNlbGxlci52MW'
    'FscGhhMS5TdG9yZVJlcG9ydFIGcmVwb3J0EkkKC3N1cHBsZW1lbnRzGAIgAygLMicuaGF3a3Iu'
    'c2VsbGVyLnYxYWxwaGExLlJlcG9ydFN1cHBsZW1lbnRSC3N1cHBsZW1lbnRzEkEKCGV2aWRlbm'
    'NlGAMgAygLMiUuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlJlcG9ydEV2aWRlbmNlUghldmlkZW5j'
    'ZRIqCg5jYW5fc3VwcGxlbWVudBgEIAEoCEID4EEDUg1jYW5TdXBwbGVtZW50EioKDnN1cHBsZW'
    '1lbnRfbWF4GAUgASgFQgPgQQNSDXN1cHBsZW1lbnRNYXg=');

@$core.Deprecated('Use reportSupplementDescriptor instead')
const ReportSupplement$json = {
  '1': 'ReportSupplement',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'create_time',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
    {
      '1': 'evidence_positions',
      '3': 4,
      '4': 3,
      '5': 5,
      '10': 'evidencePositions'
    },
  ],
};

/// Descriptor for `ReportSupplement`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportSupplementDescriptor = $convert.base64Decode(
    'ChBSZXBvcnRTdXBwbGVtZW50Eg4KAmlkGAEgASgJUgJpZBIgCgtkZXNjcmlwdGlvbhgCIAEoCV'
    'ILZGVzY3JpcHRpb24SOwoLY3JlYXRlX3RpbWUYAyABKAsyGi5nb29nbGUucHJvdG9idWYuVGlt'
    'ZXN0YW1wUgpjcmVhdGVUaW1lEi0KEmV2aWRlbmNlX3Bvc2l0aW9ucxgEIAMoBVIRZXZpZGVuY2'
    'VQb3NpdGlvbnM=');

@$core.Deprecated('Use reportEvidenceDescriptor instead')
const ReportEvidence$json = {
  '1': 'ReportEvidence',
  '2': [
    {'1': 'position', '3': 1, '4': 1, '5': 5, '10': 'position'},
  ],
};

/// Descriptor for `ReportEvidence`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportEvidenceDescriptor = $convert.base64Decode(
    'Cg5SZXBvcnRFdmlkZW5jZRIaCghwb3NpdGlvbhgBIAEoBVIIcG9zaXRpb24=');

@$core.Deprecated('Use addReportSupplementRequestDescriptor instead')
const AddReportSupplementRequest$json = {
  '1': 'AddReportSupplementRequest',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'reportId'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'description'},
    {
      '1': 'evidence_staged_tokens',
      '3': 3,
      '4': 3,
      '5': 9,
      '10': 'evidenceStagedTokens'
    },
  ],
};

/// Descriptor for `AddReportSupplementRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addReportSupplementRequestDescriptor =
    $convert.base64Decode(
        'ChpBZGRSZXBvcnRTdXBwbGVtZW50UmVxdWVzdBIgCglyZXBvcnRfaWQYASABKAlCA+BBAlIIcm'
        'Vwb3J0SWQSJQoLZGVzY3JpcHRpb24YAiABKAlCA+BBAlILZGVzY3JpcHRpb24SNAoWZXZpZGVu'
        'Y2Vfc3RhZ2VkX3Rva2VucxgDIAMoCVIUZXZpZGVuY2VTdGFnZWRUb2tlbnM=');

@$core.Deprecated('Use addReportSupplementResponseDescriptor instead')
const AddReportSupplementResponse$json = {
  '1': 'AddReportSupplementResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.StoreReport',
      '10': 'report'
    },
    {
      '1': 'supplements',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.ReportSupplement',
      '10': 'supplements'
    },
    {
      '1': 'evidence',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.ReportEvidence',
      '10': 'evidence'
    },
    {
      '1': 'can_supplement',
      '3': 4,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'canSupplement'
    },
    {
      '1': 'supplement_max',
      '3': 5,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'supplementMax'
    },
  ],
};

/// Descriptor for `AddReportSupplementResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addReportSupplementResponseDescriptor = $convert.base64Decode(
    'ChtBZGRSZXBvcnRTdXBwbGVtZW50UmVzcG9uc2USOgoGcmVwb3J0GAEgASgLMiIuaGF3a3Iuc2'
    'VsbGVyLnYxYWxwaGExLlN0b3JlUmVwb3J0UgZyZXBvcnQSSQoLc3VwcGxlbWVudHMYAiADKAsy'
    'Jy5oYXdrci5zZWxsZXIudjFhbHBoYTEuUmVwb3J0U3VwcGxlbWVudFILc3VwcGxlbWVudHMSQQ'
    'oIZXZpZGVuY2UYAyADKAsyJS5oYXdrci5zZWxsZXIudjFhbHBoYTEuUmVwb3J0RXZpZGVuY2VS'
    'CGV2aWRlbmNlEioKDmNhbl9zdXBwbGVtZW50GAQgASgIQgPgQQNSDWNhblN1cHBsZW1lbnQSKg'
    'oOc3VwcGxlbWVudF9tYXgYBSABKAVCA+BBA1INc3VwcGxlbWVudE1heA==');

const $core.Map<$core.String, $core.dynamic> SellerReportAppServiceBase$json = {
  '1': 'SellerReportAppService',
  '2': [
    {
      '1': 'CreateReport',
      '2': '.hawkr.seller.v1alpha1.CreateReportRequest',
      '3': '.hawkr.seller.v1alpha1.CreateReportResponse'
    },
    {
      '1': 'GetMyReport',
      '2': '.hawkr.seller.v1alpha1.GetMyReportRequest',
      '3': '.hawkr.seller.v1alpha1.GetMyReportResponse',
      '4': {'34': 1},
    },
    {
      '1': 'AddReportSupplement',
      '2': '.hawkr.seller.v1alpha1.AddReportSupplementRequest',
      '3': '.hawkr.seller.v1alpha1.AddReportSupplementResponse'
    },
  ],
};

@$core.Deprecated('Use sellerReportAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerReportAppServiceBase$messageJson = {
  '.hawkr.seller.v1alpha1.CreateReportRequest': CreateReportRequest$json,
  '.hawkr.seller.v1alpha1.CreateReportResponse': CreateReportResponse$json,
  '.hawkr.seller.v1alpha1.StoreReport': StoreReport$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.google.rpc.LocalizedMessage': $1.LocalizedMessage$json,
  '.hawkr.seller.v1alpha1.GetMyReportRequest': GetMyReportRequest$json,
  '.hawkr.seller.v1alpha1.GetMyReportResponse': GetMyReportResponse$json,
  '.hawkr.seller.v1alpha1.ReportSupplement': ReportSupplement$json,
  '.hawkr.seller.v1alpha1.ReportEvidence': ReportEvidence$json,
  '.hawkr.seller.v1alpha1.AddReportSupplementRequest':
      AddReportSupplementRequest$json,
  '.hawkr.seller.v1alpha1.AddReportSupplementResponse':
      AddReportSupplementResponse$json,
};

/// Descriptor for `SellerReportAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerReportAppServiceDescriptor = $convert.base64Decode(
    'ChZTZWxsZXJSZXBvcnRBcHBTZXJ2aWNlEmcKDENyZWF0ZVJlcG9ydBIqLmhhd2tyLnNlbGxlci'
    '52MWFscGhhMS5DcmVhdGVSZXBvcnRSZXF1ZXN0GisuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkNy'
    'ZWF0ZVJlcG9ydFJlc3BvbnNlEmkKC0dldE15UmVwb3J0EikuaGF3a3Iuc2VsbGVyLnYxYWxwaG'
    'ExLkdldE15UmVwb3J0UmVxdWVzdBoqLmhhd2tyLnNlbGxlci52MWFscGhhMS5HZXRNeVJlcG9y'
    'dFJlc3BvbnNlIgOQAgESfAoTQWRkUmVwb3J0U3VwcGxlbWVudBIxLmhhd2tyLnNlbGxlci52MW'
    'FscGhhMS5BZGRSZXBvcnRTdXBwbGVtZW50UmVxdWVzdBoyLmhhd2tyLnNlbGxlci52MWFscGhh'
    'MS5BZGRSZXBvcnRTdXBwbGVtZW50UmVzcG9uc2U=');
