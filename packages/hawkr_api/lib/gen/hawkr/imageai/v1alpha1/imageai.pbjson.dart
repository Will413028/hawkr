// This is a generated file - do not edit.
//
// Generated from hawkr/imageai/v1alpha1/imageai.proto.

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

import '../../common/v1alpha1/image.pbjson.dart' as $0;

@$core.Deprecated('Use aiStyleCategoryDescriptor instead')
const AiStyleCategory$json = {
  '1': 'AiStyleCategory',
  '2': [
    {'1': 'AI_STYLE_CATEGORY_UNSPECIFIED', '2': 0},
    {'1': 'AI_STYLE_CATEGORY_PRODUCT', '2': 1},
    {'1': 'AI_STYLE_CATEGORY_POST', '2': 2},
  ],
};

/// Descriptor for `AiStyleCategory`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List aiStyleCategoryDescriptor = $convert.base64Decode(
    'Cg9BaVN0eWxlQ2F0ZWdvcnkSIQodQUlfU1RZTEVfQ0FURUdPUllfVU5TUEVDSUZJRUQQABIdCh'
    'lBSV9TVFlMRV9DQVRFR09SWV9QUk9EVUNUEAESGgoWQUlfU1RZTEVfQ0FURUdPUllfUE9TVBAC');

@$core.Deprecated('Use aiStyleDescriptor instead')
const AiStyle$json = {
  '1': 'AiStyle',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'label', '3': 3, '4': 1, '5': 9, '10': 'label'},
    {'1': 'prompt', '3': 4, '4': 1, '5': 9, '10': 'prompt'},
    {
      '1': 'thumbnail',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'thumbnail'
    },
    {'1': 'display_order', '3': 6, '4': 1, '5': 5, '10': 'displayOrder'},
  ],
};

/// Descriptor for `AiStyle`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List aiStyleDescriptor = $convert.base64Decode(
    'CgdBaVN0eWxlEhMKAmlkGAEgASgJQgPgQQhSAmlkEhIKBG5hbWUYAiABKAlSBG5hbWUSFAoFbG'
    'FiZWwYAyABKAlSBWxhYmVsEhYKBnByb21wdBgEIAEoCVIGcHJvbXB0EjoKCXRodW1ibmFpbBgF'
    'IAEoCzIcLmhhd2tyLmNvbW1vbi52MWFscGhhMS5JbWFnZVIJdGh1bWJuYWlsEiMKDWRpc3BsYX'
    'lfb3JkZXIYBiABKAVSDGRpc3BsYXlPcmRlcg==');

@$core.Deprecated('Use transformImageRequestDescriptor instead')
const TransformImageRequest$json = {
  '1': 'TransformImageRequest',
  '2': [
    {'1': 'image_data', '3': 1, '4': 1, '5': 12, '8': {}, '10': 'imageData'},
    {'1': 'mime_type', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'mimeType'},
    {'1': 'prompt', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'prompt'},
  ],
};

/// Descriptor for `TransformImageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List transformImageRequestDescriptor = $convert.base64Decode(
    'ChVUcmFuc2Zvcm1JbWFnZVJlcXVlc3QSIgoKaW1hZ2VfZGF0YRgBIAEoDEID4EECUglpbWFnZU'
    'RhdGESIAoJbWltZV90eXBlGAIgASgJQgPgQQJSCG1pbWVUeXBlEhsKBnByb21wdBgDIAEoCUID'
    '4EECUgZwcm9tcHQ=');

@$core.Deprecated('Use transformImageResponseDescriptor instead')
const TransformImageResponse$json = {
  '1': 'TransformImageResponse',
  '2': [
    {'1': 'image_data', '3': 1, '4': 1, '5': 12, '10': 'imageData'},
    {'1': 'mime_type', '3': 2, '4': 1, '5': 9, '10': 'mimeType'},
    {'1': 'weekly_usage', '3': 3, '4': 1, '5': 5, '10': 'weeklyUsage'},
    {'1': 'weekly_limit', '3': 4, '4': 1, '5': 5, '10': 'weeklyLimit'},
  ],
};

/// Descriptor for `TransformImageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List transformImageResponseDescriptor = $convert.base64Decode(
    'ChZUcmFuc2Zvcm1JbWFnZVJlc3BvbnNlEh0KCmltYWdlX2RhdGEYASABKAxSCWltYWdlRGF0YR'
    'IbCgltaW1lX3R5cGUYAiABKAlSCG1pbWVUeXBlEiEKDHdlZWtseV91c2FnZRgDIAEoBVILd2Vl'
    'a2x5VXNhZ2USIQoMd2Vla2x5X2xpbWl0GAQgASgFUgt3ZWVrbHlMaW1pdA==');

@$core.Deprecated('Use listAiStylesRequestDescriptor instead')
const ListAiStylesRequest$json = {
  '1': 'ListAiStylesRequest',
  '2': [
    {
      '1': 'category',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.imageai.v1alpha1.AiStyleCategory',
      '9': 0,
      '10': 'category',
      '17': true
    },
  ],
  '8': [
    {'1': '_category'},
  ],
};

/// Descriptor for `ListAiStylesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAiStylesRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0QWlTdHlsZXNSZXF1ZXN0EkgKCGNhdGVnb3J5GAEgASgOMicuaGF3a3IuaW1hZ2VhaS'
    '52MWFscGhhMS5BaVN0eWxlQ2F0ZWdvcnlIAFIIY2F0ZWdvcnmIAQFCCwoJX2NhdGVnb3J5');

@$core.Deprecated('Use listAiStylesResponseDescriptor instead')
const ListAiStylesResponse$json = {
  '1': 'ListAiStylesResponse',
  '2': [
    {'1': 'default_prompt', '3': 1, '4': 1, '5': 9, '10': 'defaultPrompt'},
    {'1': 'weekly_usage', '3': 2, '4': 1, '5': 5, '10': 'weeklyUsage'},
    {'1': 'weekly_limit', '3': 3, '4': 1, '5': 5, '10': 'weeklyLimit'},
    {
      '1': 'styles',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.imageai.v1alpha1.AiStyle',
      '10': 'styles'
    },
  ],
};

/// Descriptor for `ListAiStylesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAiStylesResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0QWlTdHlsZXNSZXNwb25zZRIlCg5kZWZhdWx0X3Byb21wdBgBIAEoCVINZGVmYXVsdF'
    'Byb21wdBIhCgx3ZWVrbHlfdXNhZ2UYAiABKAVSC3dlZWtseVVzYWdlEiEKDHdlZWtseV9saW1p'
    'dBgDIAEoBVILd2Vla2x5TGltaXQSNwoGc3R5bGVzGAQgAygLMh8uaGF3a3IuaW1hZ2VhaS52MW'
    'FscGhhMS5BaVN0eWxlUgZzdHlsZXM=');

const $core.Map<$core.String, $core.dynamic> ImageAiAppServiceBase$json = {
  '1': 'ImageAiAppService',
  '2': [
    {
      '1': 'TransformImage',
      '2': '.hawkr.imageai.v1alpha1.TransformImageRequest',
      '3': '.hawkr.imageai.v1alpha1.TransformImageResponse'
    },
    {
      '1': 'ListAiStyles',
      '2': '.hawkr.imageai.v1alpha1.ListAiStylesRequest',
      '3': '.hawkr.imageai.v1alpha1.ListAiStylesResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use imageAiAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    ImageAiAppServiceBase$messageJson = {
  '.hawkr.imageai.v1alpha1.TransformImageRequest': TransformImageRequest$json,
  '.hawkr.imageai.v1alpha1.TransformImageResponse': TransformImageResponse$json,
  '.hawkr.imageai.v1alpha1.ListAiStylesRequest': ListAiStylesRequest$json,
  '.hawkr.imageai.v1alpha1.ListAiStylesResponse': ListAiStylesResponse$json,
  '.hawkr.imageai.v1alpha1.AiStyle': AiStyle$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
};

/// Descriptor for `ImageAiAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List imageAiAppServiceDescriptor = $convert.base64Decode(
    'ChFJbWFnZUFpQXBwU2VydmljZRJvCg5UcmFuc2Zvcm1JbWFnZRItLmhhd2tyLmltYWdlYWkudj'
    'FhbHBoYTEuVHJhbnNmb3JtSW1hZ2VSZXF1ZXN0Gi4uaGF3a3IuaW1hZ2VhaS52MWFscGhhMS5U'
    'cmFuc2Zvcm1JbWFnZVJlc3BvbnNlEm4KDExpc3RBaVN0eWxlcxIrLmhhd2tyLmltYWdlYWkudj'
    'FhbHBoYTEuTGlzdEFpU3R5bGVzUmVxdWVzdBosLmhhd2tyLmltYWdlYWkudjFhbHBoYTEuTGlz'
    'dEFpU3R5bGVzUmVzcG9uc2UiA5ACAQ==');
