// This is a generated file - do not edit.
//
// Generated from hawkr/review/v1alpha1/review.proto.

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
    as $1;

import '../../common/v1alpha1/image.pbjson.dart' as $0;

@$core.Deprecated('Use reviewAuthorDescriptor instead')
const ReviewAuthor$json = {
  '1': 'ReviewAuthor',
  '2': [
    {
      '1': 'name',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'name',
      '17': true
    },
    {
      '1': 'avatar',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'avatar'
    },
  ],
  '8': [
    {'1': '_name'},
  ],
};

/// Descriptor for `ReviewAuthor`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reviewAuthorDescriptor = $convert.base64Decode(
    'CgxSZXZpZXdBdXRob3ISHAoEbmFtZRgBIAEoCUID4EEDSABSBG5hbWWIAQESOQoGYXZhdGFyGA'
    'IgASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaGExLkltYWdlQgPgQQNSBmF2YXRhckIHCgVfbmFt'
    'ZQ==');

@$core.Deprecated('Use reviewReplyDescriptor instead')
const ReviewReply$json = {
  '1': 'ReviewReply',
  '2': [
    {'1': 'content', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'content'},
    {
      '1': 'reply_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'replyTime'
    },
  ],
};

/// Descriptor for `ReviewReply`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reviewReplyDescriptor = $convert.base64Decode(
    'CgtSZXZpZXdSZXBseRIdCgdjb250ZW50GAEgASgJQgPgQQNSB2NvbnRlbnQSPgoKcmVwbHlfdG'
    'ltZRgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1IJcmVwbHlUaW1l');

@$core.Deprecated('Use reviewDescriptor instead')
const Review$json = {
  '1': 'Review',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'product_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'store_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'rating', '3': 4, '4': 1, '5': 5, '8': {}, '10': 'rating'},
    {'1': 'content', '3': 5, '4': 1, '5': 9, '8': {}, '10': 'content'},
    {
      '1': 'images',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'images'
    },
    {'1': 'sku_name', '3': 7, '4': 1, '5': 9, '8': {}, '10': 'skuName'},
    {
      '1': 'item_image',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'itemImage'
    },
    {'1': 'price', '3': 9, '4': 1, '5': 3, '8': {}, '10': 'price'},
    {
      '1': 'create_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'reply',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.ReviewReply',
      '8': {},
      '9': 0,
      '10': 'reply',
      '17': true
    },
    {
      '1': 'author',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.ReviewAuthor',
      '8': {},
      '10': 'author'
    },
    {
      '1': 'order_no',
      '3': 13,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 1,
      '10': 'orderNo',
      '17': true
    },
    {
      '1': 'order_item_id',
      '3': 14,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 2,
      '10': 'orderItemId',
      '17': true
    },
    {'1': 'is_mine', '3': 15, '4': 1, '5': 8, '8': {}, '10': 'isMine'},
  ],
  '8': [
    {'1': '_reply'},
    {'1': '_order_no'},
    {'1': '_order_item_id'},
  ],
};

/// Descriptor for `Review`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reviewDescriptor = $convert.base64Decode(
    'CgZSZXZpZXcSEwoCaWQYASABKAlCA+BBCFICaWQSIgoKcHJvZHVjdF9pZBgCIAEoCUID4EEDUg'
    'lwcm9kdWN0SWQSHgoIc3RvcmVfaWQYAyABKAlCA+BBA1IHc3RvcmVJZBIbCgZyYXRpbmcYBCAB'
    'KAVCA+BBA1IGcmF0aW5nEh0KB2NvbnRlbnQYBSABKAlCA+BBA1IHY29udGVudBI5CgZpbWFnZX'
    'MYBiADKAsyHC5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VCA+BBA1IGaW1hZ2VzEh4KCHNr'
    'dV9uYW1lGAcgASgJQgPgQQNSB3NrdU5hbWUSQAoKaXRlbV9pbWFnZRgIIAEoCzIcLmhhd2tyLm'
    'NvbW1vbi52MWFscGhhMS5JbWFnZUID4EEDUglpdGVtSW1hZ2USGQoFcHJpY2UYCSABKANCA+BB'
    'A1IFcHJpY2USQAoLY3JlYXRlX3RpbWUYCiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW'
    '1wQgPgQQNSCmNyZWF0ZVRpbWUSQgoFcmVwbHkYCyABKAsyIi5oYXdrci5yZXZpZXcudjFhbHBo'
    'YTEuUmV2aWV3UmVwbHlCA+BBA0gAUgVyZXBseYgBARJACgZhdXRob3IYDCABKAsyIy5oYXdrci'
    '5yZXZpZXcudjFhbHBoYTEuUmV2aWV3QXV0aG9yQgPgQQNSBmF1dGhvchIjCghvcmRlcl9ubxgN'
    'IAEoCUID4EEDSAFSB29yZGVyTm+IAQESLAoNb3JkZXJfaXRlbV9pZBgOIAEoCUID4EEDSAJSC2'
    '9yZGVySXRlbUlkiAEBEhwKB2lzX21pbmUYDyABKAhCA+BBA1IGaXNNaW5lQggKBl9yZXBseUIL'
    'Cglfb3JkZXJfbm9CEAoOX29yZGVyX2l0ZW1faWQ=');

@$core.Deprecated('Use listProductReviewsRequestDescriptor instead')
const ListProductReviewsRequest$json = {
  '1': 'ListProductReviewsRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
    {'1': 'rating', '3': 4, '4': 1, '5': 5, '9': 0, '10': 'rating', '17': true},
    {'1': 'with_images', '3': 5, '4': 1, '5': 8, '10': 'withImages'},
  ],
  '8': [
    {'1': '_rating'},
  ],
};

/// Descriptor for `ListProductReviewsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listProductReviewsRequestDescriptor = $convert.base64Decode(
    'ChlMaXN0UHJvZHVjdFJldmlld3NSZXF1ZXN0EiIKCnByb2R1Y3RfaWQYASABKAlCA+BBAlIJcH'
    'JvZHVjdElkEhsKCXBhZ2Vfc2l6ZRgCIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgDIAEo'
    'CVIJcGFnZVRva2VuEhsKBnJhdGluZxgEIAEoBUgAUgZyYXRpbmeIAQESHwoLd2l0aF9pbWFnZX'
    'MYBSABKAhSCndpdGhJbWFnZXNCCQoHX3JhdGluZw==');

@$core.Deprecated('Use listProductReviewsResponseDescriptor instead')
const ListProductReviewsResponse$json = {
  '1': 'ListProductReviewsResponse',
  '2': [
    {
      '1': 'reviews',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'reviews'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListProductReviewsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listProductReviewsResponseDescriptor =
    $convert.base64Decode(
        'ChpMaXN0UHJvZHVjdFJldmlld3NSZXNwb25zZRI3CgdyZXZpZXdzGAEgAygLMh0uaGF3a3Iucm'
        'V2aWV3LnYxYWxwaGExLlJldmlld1IHcmV2aWV3cxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlS'
        'DW5leHRQYWdlVG9rZW4=');

@$core.Deprecated('Use listStoreReviewsRequestDescriptor instead')
const ListStoreReviewsRequest$json = {
  '1': 'ListStoreReviewsRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
    {'1': 'rating', '3': 4, '4': 1, '5': 5, '9': 0, '10': 'rating', '17': true},
    {'1': 'with_images', '3': 5, '4': 1, '5': 8, '10': 'withImages'},
  ],
  '8': [
    {'1': '_rating'},
  ],
};

/// Descriptor for `ListStoreReviewsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreReviewsRequestDescriptor = $convert.base64Decode(
    'ChdMaXN0U3RvcmVSZXZpZXdzUmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG9yZU'
    'lkEhsKCXBhZ2Vfc2l6ZRgCIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgDIAEoCVIJcGFn'
    'ZVRva2VuEhsKBnJhdGluZxgEIAEoBUgAUgZyYXRpbmeIAQESHwoLd2l0aF9pbWFnZXMYBSABKA'
    'hSCndpdGhJbWFnZXNCCQoHX3JhdGluZw==');

@$core.Deprecated('Use listStoreReviewsResponseDescriptor instead')
const ListStoreReviewsResponse$json = {
  '1': 'ListStoreReviewsResponse',
  '2': [
    {
      '1': 'reviews',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'reviews'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListStoreReviewsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreReviewsResponseDescriptor = $convert.base64Decode(
    'ChhMaXN0U3RvcmVSZXZpZXdzUmVzcG9uc2USNwoHcmV2aWV3cxgBIAMoCzIdLmhhd2tyLnJldm'
    'lldy52MWFscGhhMS5SZXZpZXdSB3Jldmlld3MSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJUg1u'
    'ZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use getStoreReviewStatsRequestDescriptor instead')
const GetStoreReviewStatsRequest$json = {
  '1': 'GetStoreReviewStatsRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
  ],
};

/// Descriptor for `GetStoreReviewStatsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreReviewStatsRequestDescriptor =
    $convert.base64Decode(
        'ChpHZXRTdG9yZVJldmlld1N0YXRzUmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG'
        '9yZUlk');

@$core.Deprecated('Use ratingCountDescriptor instead')
const RatingCount$json = {
  '1': 'RatingCount',
  '2': [
    {'1': 'rating', '3': 1, '4': 1, '5': 5, '10': 'rating'},
    {'1': 'count', '3': 2, '4': 1, '5': 5, '10': 'count'},
  ],
};

/// Descriptor for `RatingCount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ratingCountDescriptor = $convert.base64Decode(
    'CgtSYXRpbmdDb3VudBIWCgZyYXRpbmcYASABKAVSBnJhdGluZxIUCgVjb3VudBgCIAEoBVIFY2'
    '91bnQ=');

@$core.Deprecated('Use getStoreReviewStatsResponseDescriptor instead')
const GetStoreReviewStatsResponse$json = {
  '1': 'GetStoreReviewStatsResponse',
  '2': [
    {'1': 'count', '3': 1, '4': 1, '5': 5, '10': 'count'},
    {'1': 'average', '3': 2, '4': 1, '5': 1, '10': 'average'},
    {
      '1': 'distribution',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.RatingCount',
      '10': 'distribution'
    },
  ],
};

/// Descriptor for `GetStoreReviewStatsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreReviewStatsResponseDescriptor =
    $convert.base64Decode(
        'ChtHZXRTdG9yZVJldmlld1N0YXRzUmVzcG9uc2USFAoFY291bnQYASABKAVSBWNvdW50EhgKB2'
        'F2ZXJhZ2UYAiABKAFSB2F2ZXJhZ2USRgoMZGlzdHJpYnV0aW9uGAMgAygLMiIuaGF3a3IucmV2'
        'aWV3LnYxYWxwaGExLlJhdGluZ0NvdW50UgxkaXN0cmlidXRpb24=');

@$core.Deprecated('Use getProductReviewStatsRequestDescriptor instead')
const GetProductReviewStatsRequest$json = {
  '1': 'GetProductReviewStatsRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `GetProductReviewStatsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProductReviewStatsRequestDescriptor =
    $convert.base64Decode(
        'ChxHZXRQcm9kdWN0UmV2aWV3U3RhdHNSZXF1ZXN0EiIKCnByb2R1Y3RfaWQYASABKAlCA+BBAl'
        'IJcHJvZHVjdElk');

@$core.Deprecated('Use getProductReviewStatsResponseDescriptor instead')
const GetProductReviewStatsResponse$json = {
  '1': 'GetProductReviewStatsResponse',
  '2': [
    {'1': 'count', '3': 1, '4': 1, '5': 5, '10': 'count'},
    {'1': 'average', '3': 2, '4': 1, '5': 1, '10': 'average'},
    {
      '1': 'distribution',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.RatingCount',
      '10': 'distribution'
    },
    {'1': 'with_images_count', '3': 4, '4': 1, '5': 5, '10': 'withImagesCount'},
  ],
};

/// Descriptor for `GetProductReviewStatsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProductReviewStatsResponseDescriptor = $convert.base64Decode(
    'Ch1HZXRQcm9kdWN0UmV2aWV3U3RhdHNSZXNwb25zZRIUCgVjb3VudBgBIAEoBVIFY291bnQSGA'
    'oHYXZlcmFnZRgCIAEoAVIHYXZlcmFnZRJGCgxkaXN0cmlidXRpb24YAyADKAsyIi5oYXdrci5y'
    'ZXZpZXcudjFhbHBoYTEuUmF0aW5nQ291bnRSDGRpc3RyaWJ1dGlvbhIqChF3aXRoX2ltYWdlc1'
    '9jb3VudBgEIAEoBVIPd2l0aEltYWdlc0NvdW50');

@$core.Deprecated('Use getReviewRequestDescriptor instead')
const GetReviewRequest$json = {
  '1': 'GetReviewRequest',
  '2': [
    {'1': 'review_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'reviewId'},
  ],
};

/// Descriptor for `GetReviewRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getReviewRequestDescriptor = $convert.base64Decode(
    'ChBHZXRSZXZpZXdSZXF1ZXN0EiAKCXJldmlld19pZBgBIAEoCUID4EECUghyZXZpZXdJZA==');

@$core.Deprecated('Use getReviewResponseDescriptor instead')
const GetReviewResponse$json = {
  '1': 'GetReviewResponse',
  '2': [
    {
      '1': 'review',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'review'
    },
  ],
};

/// Descriptor for `GetReviewResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getReviewResponseDescriptor = $convert.base64Decode(
    'ChFHZXRSZXZpZXdSZXNwb25zZRI1CgZyZXZpZXcYASABKAsyHS5oYXdrci5yZXZpZXcudjFhbH'
    'BoYTEuUmV2aWV3UgZyZXZpZXc=');

@$core.Deprecated('Use replyReviewRequestDescriptor instead')
const ReplyReviewRequest$json = {
  '1': 'ReplyReviewRequest',
  '2': [
    {'1': 'review_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'reviewId'},
    {'1': 'content', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'content'},
  ],
};

/// Descriptor for `ReplyReviewRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List replyReviewRequestDescriptor = $convert.base64Decode(
    'ChJSZXBseVJldmlld1JlcXVlc3QSIAoJcmV2aWV3X2lkGAEgASgJQgPgQQJSCHJldmlld0lkEh'
    '0KB2NvbnRlbnQYAiABKAlCA+BBAlIHY29udGVudA==');

@$core.Deprecated('Use replyReviewResponseDescriptor instead')
const ReplyReviewResponse$json = {
  '1': 'ReplyReviewResponse',
  '2': [
    {
      '1': 'review',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'review'
    },
  ],
};

/// Descriptor for `ReplyReviewResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List replyReviewResponseDescriptor = $convert.base64Decode(
    'ChNSZXBseVJldmlld1Jlc3BvbnNlEjUKBnJldmlldxgBIAEoCzIdLmhhd2tyLnJldmlldy52MW'
    'FscGhhMS5SZXZpZXdSBnJldmlldw==');

@$core.Deprecated('Use createReviewRequestDescriptor instead')
const CreateReviewRequest$json = {
  '1': 'CreateReviewRequest',
  '2': [
    {'1': 'order_no', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'orderNo'},
    {
      '1': 'order_item_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'orderItemId'
    },
    {'1': 'rating', '3': 3, '4': 1, '5': 5, '8': {}, '10': 'rating'},
    {'1': 'content', '3': 4, '4': 1, '5': 9, '10': 'content'},
    {
      '1': 'image_staged_tokens',
      '3': 5,
      '4': 3,
      '5': 9,
      '8': {},
      '10': 'imageStagedTokens'
    },
  ],
};

/// Descriptor for `CreateReviewRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createReviewRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVSZXZpZXdSZXF1ZXN0Eh4KCG9yZGVyX25vGAEgASgJQgPgQQJSB29yZGVyTm8SJw'
    'oNb3JkZXJfaXRlbV9pZBgCIAEoCUID4EECUgtvcmRlckl0ZW1JZBIbCgZyYXRpbmcYAyABKAVC'
    'A+BBAlIGcmF0aW5nEhgKB2NvbnRlbnQYBCABKAlSB2NvbnRlbnQSMwoTaW1hZ2Vfc3RhZ2VkX3'
    'Rva2VucxgFIAMoCUID4EEEUhFpbWFnZVN0YWdlZFRva2Vucw==');

@$core.Deprecated('Use createReviewResponseDescriptor instead')
const CreateReviewResponse$json = {
  '1': 'CreateReviewResponse',
  '2': [
    {
      '1': 'review',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'review'
    },
  ],
};

/// Descriptor for `CreateReviewResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createReviewResponseDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVSZXZpZXdSZXNwb25zZRI1CgZyZXZpZXcYASABKAsyHS5oYXdrci5yZXZpZXcudj'
    'FhbHBoYTEuUmV2aWV3UgZyZXZpZXc=');

@$core.Deprecated('Use listMyReviewsRequestDescriptor instead')
const ListMyReviewsRequest$json = {
  '1': 'ListMyReviewsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'order_item_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'orderItemId',
      '17': true
    },
  ],
  '8': [
    {'1': '_order_item_id'},
  ],
};

/// Descriptor for `ListMyReviewsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyReviewsRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0TXlSZXZpZXdzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCn'
    'BhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbhInCg1vcmRlcl9pdGVtX2lkGAMgASgJSABSC29y'
    'ZGVySXRlbUlkiAEBQhAKDl9vcmRlcl9pdGVtX2lk');

@$core.Deprecated('Use listMyReviewsResponseDescriptor instead')
const ListMyReviewsResponse$json = {
  '1': 'ListMyReviewsResponse',
  '2': [
    {
      '1': 'reviews',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'reviews'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyReviewsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyReviewsResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0TXlSZXZpZXdzUmVzcG9uc2USNwoHcmV2aWV3cxgBIAMoCzIdLmhhd2tyLnJldmlldy'
    '52MWFscGhhMS5SZXZpZXdSB3Jldmlld3MSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJUg1uZXh0'
    'UGFnZVRva2Vu');

@$core.Deprecated('Use listOwnStoreReviewsRequestDescriptor instead')
const ListOwnStoreReviewsRequest$json = {
  '1': 'ListOwnStoreReviewsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {'1': 'rating', '3': 3, '4': 1, '5': 5, '9': 0, '10': 'rating', '17': true},
    {'1': 'with_images', '3': 4, '4': 1, '5': 8, '10': 'withImages'},
  ],
  '8': [
    {'1': '_rating'},
  ],
};

/// Descriptor for `ListOwnStoreReviewsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listOwnStoreReviewsRequestDescriptor =
    $convert.base64Decode(
        'ChpMaXN0T3duU3RvcmVSZXZpZXdzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaX'
        'plEh0KCnBhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbhIbCgZyYXRpbmcYAyABKAVIAFIGcmF0'
        'aW5niAEBEh8KC3dpdGhfaW1hZ2VzGAQgASgIUgp3aXRoSW1hZ2VzQgkKB19yYXRpbmc=');

@$core.Deprecated('Use listOwnStoreReviewsResponseDescriptor instead')
const ListOwnStoreReviewsResponse$json = {
  '1': 'ListOwnStoreReviewsResponse',
  '2': [
    {
      '1': 'reviews',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.review.v1alpha1.Review',
      '10': 'reviews'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListOwnStoreReviewsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listOwnStoreReviewsResponseDescriptor =
    $convert.base64Decode(
        'ChtMaXN0T3duU3RvcmVSZXZpZXdzUmVzcG9uc2USNwoHcmV2aWV3cxgBIAMoCzIdLmhhd2tyLn'
        'Jldmlldy52MWFscGhhMS5SZXZpZXdSB3Jldmlld3MSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJ'
        'Ug1uZXh0UGFnZVRva2Vu');

const $core.Map<$core.String, $core.dynamic> ReviewPublicServiceBase$json = {
  '1': 'ReviewPublicService',
  '2': [
    {
      '1': 'ListProductReviews',
      '2': '.hawkr.review.v1alpha1.ListProductReviewsRequest',
      '3': '.hawkr.review.v1alpha1.ListProductReviewsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListStoreReviews',
      '2': '.hawkr.review.v1alpha1.ListStoreReviewsRequest',
      '3': '.hawkr.review.v1alpha1.ListStoreReviewsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetStoreReviewStats',
      '2': '.hawkr.review.v1alpha1.GetStoreReviewStatsRequest',
      '3': '.hawkr.review.v1alpha1.GetStoreReviewStatsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetProductReviewStats',
      '2': '.hawkr.review.v1alpha1.GetProductReviewStatsRequest',
      '3': '.hawkr.review.v1alpha1.GetProductReviewStatsResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use reviewPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    ReviewPublicServiceBase$messageJson = {
  '.hawkr.review.v1alpha1.ListProductReviewsRequest':
      ListProductReviewsRequest$json,
  '.hawkr.review.v1alpha1.ListProductReviewsResponse':
      ListProductReviewsResponse$json,
  '.hawkr.review.v1alpha1.Review': Review$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.review.v1alpha1.ReviewReply': ReviewReply$json,
  '.hawkr.review.v1alpha1.ReviewAuthor': ReviewAuthor$json,
  '.hawkr.review.v1alpha1.ListStoreReviewsRequest':
      ListStoreReviewsRequest$json,
  '.hawkr.review.v1alpha1.ListStoreReviewsResponse':
      ListStoreReviewsResponse$json,
  '.hawkr.review.v1alpha1.GetStoreReviewStatsRequest':
      GetStoreReviewStatsRequest$json,
  '.hawkr.review.v1alpha1.GetStoreReviewStatsResponse':
      GetStoreReviewStatsResponse$json,
  '.hawkr.review.v1alpha1.RatingCount': RatingCount$json,
  '.hawkr.review.v1alpha1.GetProductReviewStatsRequest':
      GetProductReviewStatsRequest$json,
  '.hawkr.review.v1alpha1.GetProductReviewStatsResponse':
      GetProductReviewStatsResponse$json,
};

/// Descriptor for `ReviewPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List reviewPublicServiceDescriptor = $convert.base64Decode(
    'ChNSZXZpZXdQdWJsaWNTZXJ2aWNlEn4KEkxpc3RQcm9kdWN0UmV2aWV3cxIwLmhhd2tyLnJldm'
    'lldy52MWFscGhhMS5MaXN0UHJvZHVjdFJldmlld3NSZXF1ZXN0GjEuaGF3a3IucmV2aWV3LnYx'
    'YWxwaGExLkxpc3RQcm9kdWN0UmV2aWV3c1Jlc3BvbnNlIgOQAgESeAoQTGlzdFN0b3JlUmV2aW'
    'V3cxIuLmhhd2tyLnJldmlldy52MWFscGhhMS5MaXN0U3RvcmVSZXZpZXdzUmVxdWVzdBovLmhh'
    'd2tyLnJldmlldy52MWFscGhhMS5MaXN0U3RvcmVSZXZpZXdzUmVzcG9uc2UiA5ACARKBAQoTR2'
    'V0U3RvcmVSZXZpZXdTdGF0cxIxLmhhd2tyLnJldmlldy52MWFscGhhMS5HZXRTdG9yZVJldmll'
    'd1N0YXRzUmVxdWVzdBoyLmhhd2tyLnJldmlldy52MWFscGhhMS5HZXRTdG9yZVJldmlld1N0YX'
    'RzUmVzcG9uc2UiA5ACARKHAQoVR2V0UHJvZHVjdFJldmlld1N0YXRzEjMuaGF3a3IucmV2aWV3'
    'LnYxYWxwaGExLkdldFByb2R1Y3RSZXZpZXdTdGF0c1JlcXVlc3QaNC5oYXdrci5yZXZpZXcudj'
    'FhbHBoYTEuR2V0UHJvZHVjdFJldmlld1N0YXRzUmVzcG9uc2UiA5ACAQ==');

const $core.Map<$core.String, $core.dynamic> ReviewAppServiceBase$json = {
  '1': 'ReviewAppService',
  '2': [
    {
      '1': 'GetReview',
      '2': '.hawkr.review.v1alpha1.GetReviewRequest',
      '3': '.hawkr.review.v1alpha1.GetReviewResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ReplyReview',
      '2': '.hawkr.review.v1alpha1.ReplyReviewRequest',
      '3': '.hawkr.review.v1alpha1.ReplyReviewResponse',
      '4': {'34': 2},
    },
    {
      '1': 'CreateReview',
      '2': '.hawkr.review.v1alpha1.CreateReviewRequest',
      '3': '.hawkr.review.v1alpha1.CreateReviewResponse'
    },
    {
      '1': 'ListMyReviews',
      '2': '.hawkr.review.v1alpha1.ListMyReviewsRequest',
      '3': '.hawkr.review.v1alpha1.ListMyReviewsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListOwnStoreReviews',
      '2': '.hawkr.review.v1alpha1.ListOwnStoreReviewsRequest',
      '3': '.hawkr.review.v1alpha1.ListOwnStoreReviewsResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use reviewAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    ReviewAppServiceBase$messageJson = {
  '.hawkr.review.v1alpha1.GetReviewRequest': GetReviewRequest$json,
  '.hawkr.review.v1alpha1.GetReviewResponse': GetReviewResponse$json,
  '.hawkr.review.v1alpha1.Review': Review$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.review.v1alpha1.ReviewReply': ReviewReply$json,
  '.hawkr.review.v1alpha1.ReviewAuthor': ReviewAuthor$json,
  '.hawkr.review.v1alpha1.ReplyReviewRequest': ReplyReviewRequest$json,
  '.hawkr.review.v1alpha1.ReplyReviewResponse': ReplyReviewResponse$json,
  '.hawkr.review.v1alpha1.CreateReviewRequest': CreateReviewRequest$json,
  '.hawkr.review.v1alpha1.CreateReviewResponse': CreateReviewResponse$json,
  '.hawkr.review.v1alpha1.ListMyReviewsRequest': ListMyReviewsRequest$json,
  '.hawkr.review.v1alpha1.ListMyReviewsResponse': ListMyReviewsResponse$json,
  '.hawkr.review.v1alpha1.ListOwnStoreReviewsRequest':
      ListOwnStoreReviewsRequest$json,
  '.hawkr.review.v1alpha1.ListOwnStoreReviewsResponse':
      ListOwnStoreReviewsResponse$json,
};

/// Descriptor for `ReviewAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List reviewAppServiceDescriptor = $convert.base64Decode(
    'ChBSZXZpZXdBcHBTZXJ2aWNlEmMKCUdldFJldmlldxInLmhhd2tyLnJldmlldy52MWFscGhhMS'
    '5HZXRSZXZpZXdSZXF1ZXN0GiguaGF3a3IucmV2aWV3LnYxYWxwaGExLkdldFJldmlld1Jlc3Bv'
    'bnNlIgOQAgESaQoLUmVwbHlSZXZpZXcSKS5oYXdrci5yZXZpZXcudjFhbHBoYTEuUmVwbHlSZX'
    'ZpZXdSZXF1ZXN0GiouaGF3a3IucmV2aWV3LnYxYWxwaGExLlJlcGx5UmV2aWV3UmVzcG9uc2Ui'
    'A5ACAhJnCgxDcmVhdGVSZXZpZXcSKi5oYXdrci5yZXZpZXcudjFhbHBoYTEuQ3JlYXRlUmV2aW'
    'V3UmVxdWVzdBorLmhhd2tyLnJldmlldy52MWFscGhhMS5DcmVhdGVSZXZpZXdSZXNwb25zZRJv'
    'Cg1MaXN0TXlSZXZpZXdzEisuaGF3a3IucmV2aWV3LnYxYWxwaGExLkxpc3RNeVJldmlld3NSZX'
    'F1ZXN0GiwuaGF3a3IucmV2aWV3LnYxYWxwaGExLkxpc3RNeVJldmlld3NSZXNwb25zZSIDkAIB'
    'EoEBChNMaXN0T3duU3RvcmVSZXZpZXdzEjEuaGF3a3IucmV2aWV3LnYxYWxwaGExLkxpc3RPd2'
    '5TdG9yZVJldmlld3NSZXF1ZXN0GjIuaGF3a3IucmV2aWV3LnYxYWxwaGExLkxpc3RPd25TdG9y'
    'ZVJldmlld3NSZXNwb25zZSIDkAIB');
