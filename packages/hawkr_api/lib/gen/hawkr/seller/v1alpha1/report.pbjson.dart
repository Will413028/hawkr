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

@$core.Deprecated('Use myReportDescriptor instead')
const MyReport$json = {
  '1': 'MyReport',
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
    {'1': 'resolution_note', '3': 8, '4': 1, '5': 9, '10': 'resolutionNote'},
    {
      '1': 'create_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
    {
      '1': 'resolve_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'resolveTime'
    },
    {
      '1': 'outcome',
      '3': 11,
      '4': 1,
      '5': 14,
      '6': '.hawkr.seller.v1alpha1.ReportOutcome',
      '8': {},
      '10': 'outcome'
    },
  ],
  '9': [
    {'1': 7, '2': 8},
  ],
  '10': ['status'],
};

/// Descriptor for `MyReport`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List myReportDescriptor = $convert.base64Decode(
    'CghNeVJlcG9ydBITCgJpZBgBIAEoCUID4EEIUgJpZBIZCghzdG9yZV9pZBgCIAEoCVIHc3Rvcm'
    'VJZBIdCgpwcm9kdWN0X2lkGAMgASgJUglwcm9kdWN0SWQSGQoIb3JkZXJfbm8YBCABKAlSB29y'
    'ZGVyTm8SQQoIY2F0ZWdvcnkYBSABKA4yJS5oYXdrci5jb21tb24udjFhbHBoYTEuUG9saWN5Q2'
    'F0ZWdvcnlSCGNhdGVnb3J5EiAKC2Rlc2NyaXB0aW9uGAYgASgJUgtkZXNjcmlwdGlvbhInCg9y'
    'ZXNvbHV0aW9uX25vdGUYCCABKAlSDnJlc29sdXRpb25Ob3RlEjsKC2NyZWF0ZV90aW1lGAkgAS'
    'gLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKY3JlYXRlVGltZRI9CgxyZXNvbHZlX3Rp'
    'bWUYCiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgtyZXNvbHZlVGltZRJDCgdvdX'
    'Rjb21lGAsgASgOMiQuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlJlcG9ydE91dGNvbWVCA+BBA1IH'
    'b3V0Y29tZUoECAcQCFIGc3RhdHVz');

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
    'ChNDcmVhdGVSZXBvcnRSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3JlSWQSRg'
    'oIY2F0ZWdvcnkYAiABKA4yJS5oYXdrci5jb21tb24udjFhbHBoYTEuUG9saWN5Q2F0ZWdvcnlC'
    'A+BBAlIIY2F0ZWdvcnkSJQoLZGVzY3JpcHRpb24YAyABKAlCA+BBAlILZGVzY3JpcHRpb24SHQ'
    'oKcHJvZHVjdF9pZBgEIAEoCVIJcHJvZHVjdElkEhkKCG9yZGVyX25vGAUgASgJUgdvcmRlck5v'
    'EjQKFmV2aWRlbmNlX3N0YWdlZF90b2tlbnMYBiADKAlSFGV2aWRlbmNlU3RhZ2VkVG9rZW5z');

@$core.Deprecated('Use createReportResponseDescriptor instead')
const CreateReportResponse$json = {
  '1': 'CreateReportResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.seller.v1alpha1.MyReport',
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
    'ChRDcmVhdGVSZXBvcnRSZXNwb25zZRI3CgZyZXBvcnQYASABKAsyHy5oYXdrci5zZWxsZXIudj'
    'FhbHBoYTEuTXlSZXBvcnRSBnJlcG9ydBInCgxzdXBwbGVtZW50ZWQYAiABKAhCA+BBA1IMc3Vw'
    'cGxlbWVudGVkEjkKBm5vdGljZRgDIAEoCzIcLmdvb2dsZS5ycGMuTG9jYWxpemVkTWVzc2FnZU'
    'ID4EEDUgZub3RpY2U=');

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
      '6': '.hawkr.seller.v1alpha1.MyReport',
      '10': 'reports'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyReportsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyReportsResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0TXlSZXBvcnRzUmVzcG9uc2USOQoHcmVwb3J0cxgBIAMoCzIfLmhhd2tyLnNlbGxlci'
    '52MWFscGhhMS5NeVJlcG9ydFIHcmVwb3J0cxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlSDW5l'
    'eHRQYWdlVG9rZW4=');

@$core.Deprecated('Use submitFoodSafetyReportRequestDescriptor instead')
const SubmitFoodSafetyReportRequest$json = {
  '1': 'SubmitFoodSafetyReportRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'description'},
    {'1': 'product_id', '3': 3, '4': 1, '5': 9, '10': 'productId'},
    {'1': 'order_no', '3': 4, '4': 1, '5': 9, '10': 'orderNo'},
    {'1': 'phone', '3': 5, '4': 1, '5': 9, '10': 'phone'},
    {'1': 'email', '3': 6, '4': 1, '5': 9, '10': 'email'},
    {'1': 'name', '3': 7, '4': 1, '5': 9, '10': 'name'},
  ],
};

/// Descriptor for `SubmitFoodSafetyReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitFoodSafetyReportRequestDescriptor = $convert.base64Decode(
    'Ch1TdWJtaXRGb29kU2FmZXR5UmVwb3J0UmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUg'
    'dzdG9yZUlkEiUKC2Rlc2NyaXB0aW9uGAIgASgJQgPgQQJSC2Rlc2NyaXB0aW9uEh0KCnByb2R1'
    'Y3RfaWQYAyABKAlSCXByb2R1Y3RJZBIZCghvcmRlcl9ubxgEIAEoCVIHb3JkZXJObxIUCgVwaG'
    '9uZRgFIAEoCVIFcGhvbmUSFAoFZW1haWwYBiABKAlSBWVtYWlsEhIKBG5hbWUYByABKAlSBG5h'
    'bWU=');

@$core.Deprecated('Use submitFoodSafetyReportResponseDescriptor instead')
const SubmitFoodSafetyReportResponse$json = {
  '1': 'SubmitFoodSafetyReportResponse',
};

/// Descriptor for `SubmitFoodSafetyReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitFoodSafetyReportResponseDescriptor =
    $convert.base64Decode('Ch5TdWJtaXRGb29kU2FmZXR5UmVwb3J0UmVzcG9uc2U=');

const $core.Map<$core.String, $core.dynamic> SellerReportAppServiceBase$json = {
  '1': 'SellerReportAppService',
  '2': [
    {
      '1': 'CreateReport',
      '2': '.hawkr.seller.v1alpha1.CreateReportRequest',
      '3': '.hawkr.seller.v1alpha1.CreateReportResponse'
    },
    {
      '1': 'ListMyReports',
      '2': '.hawkr.seller.v1alpha1.ListMyReportsRequest',
      '3': '.hawkr.seller.v1alpha1.ListMyReportsResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use sellerReportAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerReportAppServiceBase$messageJson = {
  '.hawkr.seller.v1alpha1.CreateReportRequest': CreateReportRequest$json,
  '.hawkr.seller.v1alpha1.CreateReportResponse': CreateReportResponse$json,
  '.hawkr.seller.v1alpha1.MyReport': MyReport$json,
  '.google.protobuf.Timestamp': $0.Timestamp$json,
  '.google.rpc.LocalizedMessage': $1.LocalizedMessage$json,
  '.hawkr.seller.v1alpha1.ListMyReportsRequest': ListMyReportsRequest$json,
  '.hawkr.seller.v1alpha1.ListMyReportsResponse': ListMyReportsResponse$json,
};

/// Descriptor for `SellerReportAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerReportAppServiceDescriptor = $convert.base64Decode(
    'ChZTZWxsZXJSZXBvcnRBcHBTZXJ2aWNlEmcKDENyZWF0ZVJlcG9ydBIqLmhhd2tyLnNlbGxlci'
    '52MWFscGhhMS5DcmVhdGVSZXBvcnRSZXF1ZXN0GisuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLkNy'
    'ZWF0ZVJlcG9ydFJlc3BvbnNlEm8KDUxpc3RNeVJlcG9ydHMSKy5oYXdrci5zZWxsZXIudjFhbH'
    'BoYTEuTGlzdE15UmVwb3J0c1JlcXVlc3QaLC5oYXdrci5zZWxsZXIudjFhbHBoYTEuTGlzdE15'
    'UmVwb3J0c1Jlc3BvbnNlIgOQAgE=');

const $core.Map<$core.String, $core.dynamic>
    SellerReportPublicServiceBase$json = {
  '1': 'SellerReportPublicService',
  '2': [
    {
      '1': 'SubmitFoodSafetyReport',
      '2': '.hawkr.seller.v1alpha1.SubmitFoodSafetyReportRequest',
      '3': '.hawkr.seller.v1alpha1.SubmitFoodSafetyReportResponse'
    },
  ],
};

@$core.Deprecated('Use sellerReportPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerReportPublicServiceBase$messageJson = {
  '.hawkr.seller.v1alpha1.SubmitFoodSafetyReportRequest':
      SubmitFoodSafetyReportRequest$json,
  '.hawkr.seller.v1alpha1.SubmitFoodSafetyReportResponse':
      SubmitFoodSafetyReportResponse$json,
};

/// Descriptor for `SellerReportPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerReportPublicServiceDescriptor = $convert.base64Decode(
    'ChlTZWxsZXJSZXBvcnRQdWJsaWNTZXJ2aWNlEoUBChZTdWJtaXRGb29kU2FmZXR5UmVwb3J0Ej'
    'QuaGF3a3Iuc2VsbGVyLnYxYWxwaGExLlN1Ym1pdEZvb2RTYWZldHlSZXBvcnRSZXF1ZXN0GjUu'
    'aGF3a3Iuc2VsbGVyLnYxYWxwaGExLlN1Ym1pdEZvb2RTYWZldHlSZXBvcnRSZXNwb25zZQ==');
