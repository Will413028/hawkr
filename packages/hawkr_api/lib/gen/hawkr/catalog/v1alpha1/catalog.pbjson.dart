// This is a generated file - do not edit.
//
// Generated from hawkr/catalog/v1alpha1/catalog.proto.

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

import 'package:protobuf/well_known_types/google/protobuf/field_mask.pbjson.dart'
    as $4;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $2;

import '../../common/v1alpha1/delegated_operation.pbjson.dart' as $3;
import '../../common/v1alpha1/image.pbjson.dart' as $0;
import '../../common/v1alpha1/store_card.pbjson.dart' as $1;

@$core.Deprecated('Use productStatusDescriptor instead')
const ProductStatus$json = {
  '1': 'ProductStatus',
  '2': [
    {'1': 'PRODUCT_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PRODUCT_STATUS_DRAFT', '2': 1},
    {'1': 'PRODUCT_STATUS_ACTIVE', '2': 2},
    {'1': 'PRODUCT_STATUS_ARCHIVED', '2': 3},
  ],
};

/// Descriptor for `ProductStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List productStatusDescriptor = $convert.base64Decode(
    'Cg1Qcm9kdWN0U3RhdHVzEh4KGlBST0RVQ1RfU1RBVFVTX1VOU1BFQ0lGSUVEEAASGAoUUFJPRF'
    'VDVF9TVEFUVVNfRFJBRlQQARIZChVQUk9EVUNUX1NUQVRVU19BQ1RJVkUQAhIbChdQUk9EVUNU'
    'X1NUQVRVU19BUkNISVZFRBAD');

@$core.Deprecated('Use productAvailabilityDescriptor instead')
const ProductAvailability$json = {
  '1': 'ProductAvailability',
  '2': [
    {'1': 'PRODUCT_AVAILABILITY_UNSPECIFIED', '2': 0},
    {'1': 'PRODUCT_AVAILABILITY_AVAILABLE', '2': 1},
    {'1': 'PRODUCT_AVAILABILITY_UNAVAILABLE', '2': 2},
  ],
};

/// Descriptor for `ProductAvailability`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List productAvailabilityDescriptor = $convert.base64Decode(
    'ChNQcm9kdWN0QXZhaWxhYmlsaXR5EiQKIFBST0RVQ1RfQVZBSUxBQklMSVRZX1VOU1BFQ0lGSU'
    'VEEAASIgoeUFJPRFVDVF9BVkFJTEFCSUxJVFlfQVZBSUxBQkxFEAESJAogUFJPRFVDVF9BVkFJ'
    'TEFCSUxJVFlfVU5BVkFJTEFCTEUQAg==');

@$core.Deprecated('Use stockStatusDescriptor instead')
const StockStatus$json = {
  '1': 'StockStatus',
  '2': [
    {'1': 'STOCK_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'STOCK_STATUS_LOW', '2': 1},
    {'1': 'STOCK_STATUS_OUT_OF_STOCK', '2': 2},
  ],
};

/// Descriptor for `StockStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List stockStatusDescriptor = $convert.base64Decode(
    'CgtTdG9ja1N0YXR1cxIcChhTVE9DS19TVEFUVVNfVU5TUEVDSUZJRUQQABIUChBTVE9DS19TVE'
    'FUVVNfTE9XEAESHQoZU1RPQ0tfU1RBVFVTX09VVF9PRl9TVE9DSxAC');

@$core.Deprecated('Use productSortDescriptor instead')
const ProductSort$json = {
  '1': 'ProductSort',
  '2': [
    {'1': 'PRODUCT_SORT_UNSPECIFIED', '2': 0},
    {'1': 'PRODUCT_SORT_LATEST', '2': 1},
    {'1': 'PRODUCT_SORT_PRICE_ASC', '2': 2},
    {'1': 'PRODUCT_SORT_PRICE_DESC', '2': 3},
    {'1': 'PRODUCT_SORT_SALES', '2': 4},
    {'1': 'PRODUCT_SORT_RATING', '2': 5},
    {'1': 'PRODUCT_SORT_RELEVANCE', '2': 6},
  ],
};

/// Descriptor for `ProductSort`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List productSortDescriptor = $convert.base64Decode(
    'CgtQcm9kdWN0U29ydBIcChhQUk9EVUNUX1NPUlRfVU5TUEVDSUZJRUQQABIXChNQUk9EVUNUX1'
    'NPUlRfTEFURVNUEAESGgoWUFJPRFVDVF9TT1JUX1BSSUNFX0FTQxACEhsKF1BST0RVQ1RfU09S'
    'VF9QUklDRV9ERVNDEAMSFgoSUFJPRFVDVF9TT1JUX1NBTEVTEAQSFwoTUFJPRFVDVF9TT1JUX1'
    'JBVElORxAFEhoKFlBST0RVQ1RfU09SVF9SRUxFVkFOQ0UQBg==');

@$core.Deprecated('Use hashtagMatchDescriptor instead')
const HashtagMatch$json = {
  '1': 'HashtagMatch',
  '2': [
    {'1': 'HASHTAG_MATCH_UNSPECIFIED', '2': 0},
    {'1': 'HASHTAG_MATCH_ANY', '2': 1},
    {'1': 'HASHTAG_MATCH_ALL', '2': 2},
  ],
};

/// Descriptor for `HashtagMatch`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List hashtagMatchDescriptor = $convert.base64Decode(
    'CgxIYXNodGFnTWF0Y2gSHQoZSEFTSFRBR19NQVRDSF9VTlNQRUNJRklFRBAAEhUKEUhBU0hUQU'
    'dfTUFUQ0hfQU5ZEAESFQoRSEFTSFRBR19NQVRDSF9BTEwQAg==');

@$core.Deprecated('Use productFilterDescriptor instead')
const ProductFilter$json = {
  '1': 'ProductFilter',
  '2': [
    {'1': 'category_ids', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'categoryIds'},
    {
      '1': 'price_min',
      '3': 2,
      '4': 1,
      '5': 3,
      '9': 0,
      '10': 'priceMin',
      '17': true
    },
    {
      '1': 'price_max',
      '3': 3,
      '4': 1,
      '5': 3,
      '9': 1,
      '10': 'priceMax',
      '17': true
    },
    {'1': 'hashtags', '3': 4, '4': 3, '5': 9, '8': {}, '10': 'hashtags'},
    {
      '1': 'hashtag_match',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.HashtagMatch',
      '10': 'hashtagMatch'
    },
    {'1': 'store_ids', '3': 6, '4': 3, '5': 9, '8': {}, '10': 'storeIds'},
  ],
  '8': [
    {'1': '_price_min'},
    {'1': '_price_max'},
  ],
};

/// Descriptor for `ProductFilter`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productFilterDescriptor = $convert.base64Decode(
    'Cg1Qcm9kdWN0RmlsdGVyEikKDGNhdGVnb3J5X2lkcxgBIAMoCUIGkrUYAggUUgtjYXRlZ29yeU'
    'lkcxIgCglwcmljZV9taW4YAiABKANIAFIIcHJpY2VNaW6IAQESIAoJcHJpY2VfbWF4GAMgASgD'
    'SAFSCHByaWNlTWF4iAEBEiIKCGhhc2h0YWdzGAQgAygJQgaStRgCCApSCGhhc2h0YWdzEkkKDW'
    'hhc2h0YWdfbWF0Y2gYBSABKA4yJC5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkhhc2h0YWdNYXRj'
    'aFIMaGFzaHRhZ01hdGNoEiMKCXN0b3JlX2lkcxgGIAMoCUIGkrUYAggyUghzdG9yZUlkc0IMCg'
    'pfcHJpY2VfbWluQgwKCl9wcmljZV9tYXg=');

@$core.Deprecated('Use listProductsRequestDescriptor instead')
const ListProductsRequest$json = {
  '1': 'ListProductsRequest',
  '2': [
    {
      '1': 'filter',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductFilter',
      '10': 'filter'
    },
    {
      '1': 'sort',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductSort',
      '10': 'sort'
    },
    {'1': 'page_size', '3': 3, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 4, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListProductsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listProductsRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0UHJvZHVjdHNSZXF1ZXN0Ej0KBmZpbHRlchgBIAEoCzIlLmhhd2tyLmNhdGFsb2cudj'
    'FhbHBoYTEuUHJvZHVjdEZpbHRlclIGZmlsdGVyEjcKBHNvcnQYAiABKA4yIy5oYXdrci5jYXRh'
    'bG9nLnYxYWxwaGExLlByb2R1Y3RTb3J0UgRzb3J0EhsKCXBhZ2Vfc2l6ZRgDIAEoBVIIcGFnZV'
    'NpemUSHQoKcGFnZV90b2tlbhgEIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listProductsResponseDescriptor instead')
const ListProductsResponse$json = {
  '1': 'ListProductsResponse',
  '2': [
    {
      '1': 'products',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductSummary',
      '10': 'products'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListProductsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listProductsResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0UHJvZHVjdHNSZXNwb25zZRJCCghwcm9kdWN0cxgBIAMoCzImLmhhd2tyLmNhdGFsb2'
    'cudjFhbHBoYTEuUHJvZHVjdFN1bW1hcnlSCHByb2R1Y3RzEiYKD25leHRfcGFnZV90b2tlbhgC'
    'IAEoCVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use searchProductsRequestDescriptor instead')
const SearchProductsRequest$json = {
  '1': 'SearchProductsRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'query'},
    {
      '1': 'filter',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductFilter',
      '10': 'filter'
    },
    {
      '1': 'sort',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductSort',
      '10': 'sort'
    },
    {'1': 'page_size', '3': 4, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 5, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `SearchProductsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchProductsRequestDescriptor = $convert.base64Decode(
    'ChVTZWFyY2hQcm9kdWN0c1JlcXVlc3QSHwoFcXVlcnkYASABKAlCCeBBApK1GAIQZFIFcXVlcn'
    'kSPQoGZmlsdGVyGAIgASgLMiUuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9kdWN0RmlsdGVy'
    'UgZmaWx0ZXISNwoEc29ydBgDIAEoDjIjLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHJvZHVjdF'
    'NvcnRSBHNvcnQSGwoJcGFnZV9zaXplGAQgASgFUghwYWdlU2l6ZRIdCgpwYWdlX3Rva2VuGAUg'
    'ASgJUglwYWdlVG9rZW4=');

@$core.Deprecated('Use searchProductsResponseDescriptor instead')
const SearchProductsResponse$json = {
  '1': 'SearchProductsResponse',
  '2': [
    {
      '1': 'products',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductSummary',
      '10': 'products'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `SearchProductsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchProductsResponseDescriptor = $convert.base64Decode(
    'ChZTZWFyY2hQcm9kdWN0c1Jlc3BvbnNlEkIKCHByb2R1Y3RzGAEgAygLMiYuaGF3a3IuY2F0YW'
    'xvZy52MWFscGhhMS5Qcm9kdWN0U3VtbWFyeVIIcHJvZHVjdHMSJgoPbmV4dF9wYWdlX3Rva2Vu'
    'GAIgASgJUg1uZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use suggestProductNamesRequestDescriptor instead')
const SuggestProductNamesRequest$json = {
  '1': 'SuggestProductNamesRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'query'},
    {'1': 'max_results', '3': 2, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestProductNamesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestProductNamesRequestDescriptor =
    $convert.base64Decode(
        'ChpTdWdnZXN0UHJvZHVjdE5hbWVzUmVxdWVzdBIfCgVxdWVyeRgBIAEoCUIJ4EECkrUYAhBkUg'
        'VxdWVyeRIfCgttYXhfcmVzdWx0cxgCIAEoBVIKbWF4UmVzdWx0cw==');

@$core.Deprecated('Use suggestProductNamesResponseDescriptor instead')
const SuggestProductNamesResponse$json = {
  '1': 'SuggestProductNamesResponse',
  '2': [
    {'1': 'names', '3': 1, '4': 3, '5': 9, '10': 'names'},
  ],
};

/// Descriptor for `SuggestProductNamesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestProductNamesResponseDescriptor =
    $convert.base64Decode(
        'ChtTdWdnZXN0UHJvZHVjdE5hbWVzUmVzcG9uc2USFAoFbmFtZXMYASADKAlSBW5hbWVz');

@$core.Deprecated('Use getProductRequestDescriptor instead')
const GetProductRequest$json = {
  '1': 'GetProductRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `GetProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProductRequestDescriptor = $convert.base64Decode(
    'ChFHZXRQcm9kdWN0UmVxdWVzdBIiCgpwcm9kdWN0X2lkGAEgASgJQgPgQQJSCXByb2R1Y3RJZA'
    '==');

@$core.Deprecated('Use getProductResponseDescriptor instead')
const GetProductResponse$json = {
  '1': 'GetProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Product',
      '10': 'product'
    },
  ],
};

/// Descriptor for `GetProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProductResponseDescriptor = $convert.base64Decode(
    'ChJHZXRQcm9kdWN0UmVzcG9uc2USOQoHcHJvZHVjdBgBIAEoCzIfLmhhd2tyLmNhdGFsb2cudj'
    'FhbHBoYTEuUHJvZHVjdFIHcHJvZHVjdA==');

@$core.Deprecated('Use listCategoriesRequestDescriptor instead')
const ListCategoriesRequest$json = {
  '1': 'ListCategoriesRequest',
};

/// Descriptor for `ListCategoriesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCategoriesRequestDescriptor =
    $convert.base64Decode('ChVMaXN0Q2F0ZWdvcmllc1JlcXVlc3Q=');

@$core.Deprecated('Use listCategoriesResponseDescriptor instead')
const ListCategoriesResponse$json = {
  '1': 'ListCategoriesResponse',
  '2': [
    {
      '1': 'categories',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Category',
      '10': 'categories'
    },
  ],
};

/// Descriptor for `ListCategoriesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCategoriesResponseDescriptor =
    $convert.base64Decode(
        'ChZMaXN0Q2F0ZWdvcmllc1Jlc3BvbnNlEkAKCmNhdGVnb3JpZXMYASADKAsyIC5oYXdrci5jYX'
        'RhbG9nLnYxYWxwaGExLkNhdGVnb3J5UgpjYXRlZ29yaWVz');

@$core.Deprecated('Use categoryDescriptor instead')
const Category$json = {
  '1': 'Category',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {
      '1': 'children',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Category',
      '8': {},
      '10': 'children'
    },
  ],
};

/// Descriptor for `Category`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List categoryDescriptor = $convert.base64Decode(
    'CghDYXRlZ29yeRITCgJpZBgBIAEoCUID4EEIUgJpZBIXCgRuYW1lGAIgASgJQgPgQQNSBG5hbW'
    'USQQoIY2hpbGRyZW4YAyADKAsyIC5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkNhdGVnb3J5QgPg'
    'QQNSCGNoaWxkcmVu');

@$core.Deprecated('Use suggestHashtagsRequestDescriptor instead')
const SuggestHashtagsRequest$json = {
  '1': 'SuggestHashtagsRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'query'},
    {'1': 'contains', '3': 2, '4': 1, '5': 8, '10': 'contains'},
    {'1': 'max_results', '3': 3, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestHashtagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestHashtagsRequestDescriptor = $convert.base64Decode(
    'ChZTdWdnZXN0SGFzaHRhZ3NSZXF1ZXN0EhkKBXF1ZXJ5GAEgASgJQgPgQQJSBXF1ZXJ5EhoKCG'
    'NvbnRhaW5zGAIgASgIUghjb250YWlucxIfCgttYXhfcmVzdWx0cxgDIAEoBVIKbWF4UmVzdWx0'
    'cw==');

@$core.Deprecated('Use suggestHashtagsResponseDescriptor instead')
const SuggestHashtagsResponse$json = {
  '1': 'SuggestHashtagsResponse',
  '2': [
    {
      '1': 'hashtags',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.HashtagCount',
      '10': 'hashtags'
    },
  ],
};

/// Descriptor for `SuggestHashtagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestHashtagsResponseDescriptor =
    $convert.base64Decode(
        'ChdTdWdnZXN0SGFzaHRhZ3NSZXNwb25zZRJACghoYXNodGFncxgBIAMoCzIkLmhhd2tyLmNhdG'
        'Fsb2cudjFhbHBoYTEuSGFzaHRhZ0NvdW50UghoYXNodGFncw==');

@$core.Deprecated('Use suggestTrendingHashtagsRequestDescriptor instead')
const SuggestTrendingHashtagsRequest$json = {
  '1': 'SuggestTrendingHashtagsRequest',
  '2': [
    {'1': 'max_results', '3': 1, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestTrendingHashtagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTrendingHashtagsRequestDescriptor =
    $convert.base64Decode(
        'Ch5TdWdnZXN0VHJlbmRpbmdIYXNodGFnc1JlcXVlc3QSHwoLbWF4X3Jlc3VsdHMYASABKAVSCm'
        '1heFJlc3VsdHM=');

@$core.Deprecated('Use suggestTrendingHashtagsResponseDescriptor instead')
const SuggestTrendingHashtagsResponse$json = {
  '1': 'SuggestTrendingHashtagsResponse',
  '2': [
    {
      '1': 'hashtags',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.HashtagCount',
      '10': 'hashtags'
    },
  ],
};

/// Descriptor for `SuggestTrendingHashtagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTrendingHashtagsResponseDescriptor =
    $convert.base64Decode(
        'Ch9TdWdnZXN0VHJlbmRpbmdIYXNodGFnc1Jlc3BvbnNlEkAKCGhhc2h0YWdzGAEgAygLMiQuaG'
        'F3a3IuY2F0YWxvZy52MWFscGhhMS5IYXNodGFnQ291bnRSCGhhc2h0YWdz');

@$core.Deprecated('Use hashtagCountDescriptor instead')
const HashtagCount$json = {
  '1': 'HashtagCount',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'display', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'display'},
    {
      '1': 'product_count',
      '3': 3,
      '4': 1,
      '5': 3,
      '8': {},
      '10': 'productCount'
    },
  ],
};

/// Descriptor for `HashtagCount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hashtagCountDescriptor = $convert.base64Decode(
    'CgxIYXNodGFnQ291bnQSFwoEbmFtZRgBIAEoCUID4EEDUgRuYW1lEh0KB2Rpc3BsYXkYAiABKA'
    'lCA+BBA1IHZGlzcGxheRIoCg1wcm9kdWN0X2NvdW50GAMgASgDQgPgQQNSDHByb2R1Y3RDb3Vu'
    'dA==');

@$core.Deprecated('Use myProductFilterDescriptor instead')
const MyProductFilter$json = {
  '1': 'MyProductFilter',
  '2': [
    {
      '1': 'status',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductStatus',
      '10': 'status'
    },
    {
      '1': 'stock_status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.StockStatus',
      '10': 'stockStatus'
    },
    {'1': 'query', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'query'},
  ],
};

/// Descriptor for `MyProductFilter`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List myProductFilterDescriptor = $convert.base64Decode(
    'Cg9NeVByb2R1Y3RGaWx0ZXISPQoGc3RhdHVzGAEgASgOMiUuaGF3a3IuY2F0YWxvZy52MWFscG'
    'hhMS5Qcm9kdWN0U3RhdHVzUgZzdGF0dXMSRgoMc3RvY2tfc3RhdHVzGAIgASgOMiMuaGF3a3Iu'
    'Y2F0YWxvZy52MWFscGhhMS5TdG9ja1N0YXR1c1ILc3RvY2tTdGF0dXMSHAoFcXVlcnkYAyABKA'
    'lCBpK1GAIQZFIFcXVlcnk=');

@$core.Deprecated('Use listMyProductsRequestDescriptor instead')
const ListMyProductsRequest$json = {
  '1': 'ListMyProductsRequest',
  '2': [
    {
      '1': 'filter',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.MyProductFilter',
      '10': 'filter'
    },
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListMyProductsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyProductsRequestDescriptor = $convert.base64Decode(
    'ChVMaXN0TXlQcm9kdWN0c1JlcXVlc3QSPwoGZmlsdGVyGAEgASgLMicuaGF3a3IuY2F0YWxvZy'
    '52MWFscGhhMS5NeVByb2R1Y3RGaWx0ZXJSBmZpbHRlchIbCglwYWdlX3NpemUYAiABKAVSCHBh'
    'Z2VTaXplEh0KCnBhZ2VfdG9rZW4YAyABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listMyProductsResponseDescriptor instead')
const ListMyProductsResponse$json = {
  '1': 'ListMyProductsResponse',
  '2': [
    {
      '1': 'products',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProductSummary',
      '10': 'products'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyProductsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyProductsResponseDescriptor = $convert.base64Decode(
    'ChZMaXN0TXlQcm9kdWN0c1Jlc3BvbnNlEkgKCHByb2R1Y3RzGAEgAygLMiwuaGF3a3IuY2F0YW'
    'xvZy52MWFscGhhMS5TZWxsZXJQcm9kdWN0U3VtbWFyeVIIcHJvZHVjdHMSJgoPbmV4dF9wYWdl'
    'X3Rva2VuGAIgASgJUg1uZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use getMyProductRequestDescriptor instead')
const GetMyProductRequest$json = {
  '1': 'GetMyProductRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `GetMyProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyProductRequestDescriptor = $convert.base64Decode(
    'ChNHZXRNeVByb2R1Y3RSZXF1ZXN0EiIKCnByb2R1Y3RfaWQYASABKAlCA+BBAlIJcHJvZHVjdE'
    'lk');

@$core.Deprecated('Use getMyProductResponseDescriptor instead')
const GetMyProductResponse$json = {
  '1': 'GetMyProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
  ],
};

/// Descriptor for `GetMyProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyProductResponseDescriptor = $convert.base64Decode(
    'ChRHZXRNeVByb2R1Y3RSZXNwb25zZRI/Cgdwcm9kdWN0GAEgASgLMiUuaGF3a3IuY2F0YWxvZy'
    '52MWFscGhhMS5TZWxsZXJQcm9kdWN0Ugdwcm9kdWN0');

@$core.Deprecated('Use countMyProductsRequestDescriptor instead')
const CountMyProductsRequest$json = {
  '1': 'CountMyProductsRequest',
  '2': [
    {
      '1': 'filter',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.MyProductFilter',
      '10': 'filter'
    },
  ],
};

/// Descriptor for `CountMyProductsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List countMyProductsRequestDescriptor =
    $convert.base64Decode(
        'ChZDb3VudE15UHJvZHVjdHNSZXF1ZXN0Ej8KBmZpbHRlchgBIAEoCzInLmhhd2tyLmNhdGFsb2'
        'cudjFhbHBoYTEuTXlQcm9kdWN0RmlsdGVyUgZmaWx0ZXI=');

@$core.Deprecated('Use countMyProductsResponseDescriptor instead')
const CountMyProductsResponse$json = {
  '1': 'CountMyProductsResponse',
  '2': [
    {'1': 'count', '3': 1, '4': 1, '5': 5, '8': {}, '10': 'count'},
    {
      '1': 'default_low_stock_threshold',
      '3': 2,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'defaultLowStockThreshold'
    },
  ],
};

/// Descriptor for `CountMyProductsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List countMyProductsResponseDescriptor = $convert.base64Decode(
    'ChdDb3VudE15UHJvZHVjdHNSZXNwb25zZRIZCgVjb3VudBgBIAEoBUID4EEDUgVjb3VudBJCCh'
    'tkZWZhdWx0X2xvd19zdG9ja190aHJlc2hvbGQYAiABKAVCA+BBA1IYZGVmYXVsdExvd1N0b2Nr'
    'VGhyZXNob2xk');

@$core.Deprecated('Use createProductRequestDescriptor instead')
const CreateProductRequest$json = {
  '1': 'CreateProductRequest',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductInput',
      '8': {},
      '10': 'product'
    },
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
};

/// Descriptor for `CreateProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createProductRequestDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVQcm9kdWN0UmVxdWVzdBJDCgdwcm9kdWN0GAEgASgLMiQuaGF3a3IuY2F0YWxvZy'
    '52MWFscGhhMS5Qcm9kdWN0SW5wdXRCA+BBAlIHcHJvZHVjdBIqCgpyZXF1ZXN0X2lkGAIgASgJ'
    'QgvgQQLijM/XCAIIAVIJcmVxdWVzdElk');

@$core.Deprecated('Use createProductResponseDescriptor instead')
const CreateProductResponse$json = {
  '1': 'CreateProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
  ],
};

/// Descriptor for `CreateProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createProductResponseDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVQcm9kdWN0UmVzcG9uc2USPwoHcHJvZHVjdBgBIAEoCzIlLmhhd2tyLmNhdGFsb2'
    'cudjFhbHBoYTEuU2VsbGVyUHJvZHVjdFIHcHJvZHVjdA==');

@$core.Deprecated('Use updateProductRequestDescriptor instead')
const UpdateProductRequest$json = {
  '1': 'UpdateProductRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'product',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductInput',
      '8': {},
      '10': 'product'
    },
  ],
};

/// Descriptor for `UpdateProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateProductRequestDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVQcm9kdWN0UmVxdWVzdBIiCgpwcm9kdWN0X2lkGAEgASgJQgPgQQJSCXByb2R1Y3'
    'RJZBIXCgRldGFnGAIgASgJQgPgQQJSBGV0YWcSQwoHcHJvZHVjdBgDIAEoCzIkLmhhd2tyLmNh'
    'dGFsb2cudjFhbHBoYTEuUHJvZHVjdElucHV0QgPgQQJSB3Byb2R1Y3Q=');

@$core.Deprecated('Use updateProductResponseDescriptor instead')
const UpdateProductResponse$json = {
  '1': 'UpdateProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
  ],
};

/// Descriptor for `UpdateProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateProductResponseDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVQcm9kdWN0UmVzcG9uc2USPwoHcHJvZHVjdBgBIAEoCzIlLmhhd2tyLmNhdGFsb2'
    'cudjFhbHBoYTEuU2VsbGVyUHJvZHVjdFIHcHJvZHVjdA==');

@$core.Deprecated('Use setProductStatusRequestDescriptor instead')
const SetProductStatusRequest$json = {
  '1': 'SetProductStatusRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {
      '1': 'status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductStatus',
      '8': {},
      '10': 'status'
    },
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `SetProductStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setProductStatusRequestDescriptor = $convert.base64Decode(
    'ChdTZXRQcm9kdWN0U3RhdHVzUmVxdWVzdBIiCgpwcm9kdWN0X2lkGAEgASgJQgPgQQJSCXByb2'
    'R1Y3RJZBJCCgZzdGF0dXMYAiABKA4yJS5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlByb2R1Y3RT'
    'dGF0dXNCA+BBAlIGc3RhdHVzEhcKBGV0YWcYAyABKAlCA+BBAlIEZXRhZw==');

@$core.Deprecated('Use setProductStatusResponseDescriptor instead')
const SetProductStatusResponse$json = {
  '1': 'SetProductStatusResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
  ],
};

/// Descriptor for `SetProductStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setProductStatusResponseDescriptor =
    $convert.base64Decode(
        'ChhTZXRQcm9kdWN0U3RhdHVzUmVzcG9uc2USPwoHcHJvZHVjdBgBIAEoCzIlLmhhd2tyLmNhdG'
        'Fsb2cudjFhbHBoYTEuU2VsbGVyUHJvZHVjdFIHcHJvZHVjdA==');

@$core.Deprecated('Use deleteProductRequestDescriptor instead')
const DeleteProductRequest$json = {
  '1': 'DeleteProductRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '10': 'etag'},
  ],
};

/// Descriptor for `DeleteProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteProductRequestDescriptor = $convert.base64Decode(
    'ChREZWxldGVQcm9kdWN0UmVxdWVzdBIiCgpwcm9kdWN0X2lkGAEgASgJQgPgQQJSCXByb2R1Y3'
    'RJZBISCgRldGFnGAIgASgJUgRldGFn');

@$core.Deprecated('Use deleteProductResponseDescriptor instead')
const DeleteProductResponse$json = {
  '1': 'DeleteProductResponse',
};

/// Descriptor for `DeleteProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteProductResponseDescriptor =
    $convert.base64Decode('ChVEZWxldGVQcm9kdWN0UmVzcG9uc2U=');

@$core.Deprecated('Use adjustStockRequestDescriptor instead')
const AdjustStockRequest$json = {
  '1': 'AdjustStockRequest',
  '2': [
    {'1': 'sku_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'skuId'},
    {'1': 'delta', '3': 2, '4': 1, '5': 5, '8': {}, '10': 'delta'},
    {'1': 'request_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
};

/// Descriptor for `AdjustStockRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List adjustStockRequestDescriptor = $convert.base64Decode(
    'ChJBZGp1c3RTdG9ja1JlcXVlc3QSGgoGc2t1X2lkGAEgASgJQgPgQQJSBXNrdUlkEhkKBWRlbH'
    'RhGAIgASgFQgPgQQJSBWRlbHRhEioKCnJlcXVlc3RfaWQYAyABKAlCC+BBAuKMz9cIAggBUgly'
    'ZXF1ZXN0SWQ=');

@$core.Deprecated('Use adjustStockResponseDescriptor instead')
const AdjustStockResponse$json = {
  '1': 'AdjustStockResponse',
  '2': [
    {
      '1': 'sku',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Sku',
      '10': 'sku'
    },
  ],
};

/// Descriptor for `AdjustStockResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List adjustStockResponseDescriptor = $convert.base64Decode(
    'ChNBZGp1c3RTdG9ja1Jlc3BvbnNlEi0KA3NrdRgBIAEoCzIbLmhhd2tyLmNhdGFsb2cudjFhbH'
    'BoYTEuU2t1UgNza3U=');

@$core.Deprecated('Use setStockRequestDescriptor instead')
const SetStockRequest$json = {
  '1': 'SetStockRequest',
  '2': [
    {'1': 'sku_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'skuId'},
    {'1': 'quantity', '3': 2, '4': 1, '5': 5, '10': 'quantity'},
    {
      '1': 'expected_quantity',
      '3': 3,
      '4': 1,
      '5': 5,
      '10': 'expectedQuantity'
    },
    {'1': 'request_id', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
};

/// Descriptor for `SetStockRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setStockRequestDescriptor = $convert.base64Decode(
    'Cg9TZXRTdG9ja1JlcXVlc3QSGgoGc2t1X2lkGAEgASgJQgPgQQJSBXNrdUlkEhoKCHF1YW50aX'
    'R5GAIgASgFUghxdWFudGl0eRIrChFleHBlY3RlZF9xdWFudGl0eRgDIAEoBVIQZXhwZWN0ZWRR'
    'dWFudGl0eRIqCgpyZXF1ZXN0X2lkGAQgASgJQgvgQQLijM/XCAIIAVIJcmVxdWVzdElk');

@$core.Deprecated('Use setStockResponseDescriptor instead')
const SetStockResponse$json = {
  '1': 'SetStockResponse',
  '2': [
    {
      '1': 'sku',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Sku',
      '10': 'sku'
    },
  ],
};

/// Descriptor for `SetStockResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setStockResponseDescriptor = $convert.base64Decode(
    'ChBTZXRTdG9ja1Jlc3BvbnNlEi0KA3NrdRgBIAEoCzIbLmhhd2tyLmNhdGFsb2cudjFhbHBoYT'
    'EuU2t1UgNza3U=');

@$core.Deprecated('Use productImageDescriptor instead')
const ProductImage$json = {
  '1': 'ProductImage',
  '2': [
    {'1': 'asset_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'assetId'},
    {
      '1': 'image',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'image'
    },
    {'1': 'alt', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'alt'},
  ],
};

/// Descriptor for `ProductImage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productImageDescriptor = $convert.base64Decode(
    'CgxQcm9kdWN0SW1hZ2USHgoIYXNzZXRfaWQYASABKAlCA+BBA1IHYXNzZXRJZBI3CgVpbWFnZR'
    'gCIAEoCzIcLmhhd2tyLmNvbW1vbi52MWFscGhhMS5JbWFnZUID4EEDUgVpbWFnZRIVCgNhbHQY'
    'AyABKAlCA+BBA1IDYWx0');

@$core.Deprecated('Use productDescriptionBlockDescriptor instead')
const ProductDescriptionBlock$json = {
  '1': 'ProductDescriptionBlock',
  '2': [
    {'1': 'paragraph', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'paragraph'},
    {
      '1': 'image',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductImage',
      '9': 0,
      '10': 'image'
    },
    {
      '1': 'bullet_list',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.BulletList',
      '9': 0,
      '10': 'bulletList'
    },
    {
      '1': 'table',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Table',
      '9': 0,
      '10': 'table'
    },
  ],
  '8': [
    {'1': 'block'},
  ],
};

/// Descriptor for `ProductDescriptionBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productDescriptionBlockDescriptor = $convert.base64Decode(
    'ChdQcm9kdWN0RGVzY3JpcHRpb25CbG9jaxIeCglwYXJhZ3JhcGgYASABKAlIAFIJcGFyYWdyYX'
    'BoEjwKBWltYWdlGAIgASgLMiQuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9kdWN0SW1hZ2VI'
    'AFIFaW1hZ2USRQoLYnVsbGV0X2xpc3QYAyABKAsyIi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLk'
    'J1bGxldExpc3RIAFIKYnVsbGV0TGlzdBI1CgV0YWJsZRgEIAEoCzIdLmhhd2tyLmNhdGFsb2cu'
    'djFhbHBoYTEuVGFibGVIAFIFdGFibGVCBwoFYmxvY2s=');

@$core.Deprecated('Use optionAxisDescriptor instead')
const OptionAxis$json = {
  '1': 'OptionAxis',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'values', '3': 2, '4': 3, '5': 9, '8': {}, '10': 'values'},
  ],
};

/// Descriptor for `OptionAxis`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List optionAxisDescriptor = $convert.base64Decode(
    'CgpPcHRpb25BeGlzEhcKBG5hbWUYASABKAlCA+BBA1IEbmFtZRIbCgZ2YWx1ZXMYAiADKAlCA+'
    'BBA1IGdmFsdWVz');

@$core.Deprecated('Use skuDescriptor instead')
const Sku$json = {
  '1': 'Sku',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'option1', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'option1'},
    {'1': 'option2', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'option2'},
    {'1': 'price', '3': 4, '4': 1, '5': 3, '8': {}, '10': 'price'},
    {
      '1': 'compare_at_price',
      '3': 5,
      '4': 1,
      '5': 3,
      '8': {},
      '9': 0,
      '10': 'compareAtPrice',
      '17': true
    },
    {'1': 'stock_qty', '3': 6, '4': 1, '5': 5, '8': {}, '10': 'stockQty'},
    {
      '1': 'availability',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductAvailability',
      '8': {},
      '10': 'availability'
    },
  ],
  '8': [
    {'1': '_compare_at_price'},
  ],
};

/// Descriptor for `Sku`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List skuDescriptor = $convert.base64Decode(
    'CgNTa3USEwoCaWQYASABKAlCA+BBCFICaWQSHQoHb3B0aW9uMRgCIAEoCUID4EEDUgdvcHRpb2'
    '4xEh0KB29wdGlvbjIYAyABKAlCA+BBA1IHb3B0aW9uMhIZCgVwcmljZRgEIAEoA0ID4EEDUgVw'
    'cmljZRIyChBjb21wYXJlX2F0X3ByaWNlGAUgASgDQgPgQQNIAFIOY29tcGFyZUF0UHJpY2WIAQ'
    'ESIAoJc3RvY2tfcXR5GAYgASgFQgPgQQNSCHN0b2NrUXR5ElQKDGF2YWlsYWJpbGl0eRgHIAEo'
    'DjIrLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHJvZHVjdEF2YWlsYWJpbGl0eUID4EEDUgxhdm'
    'FpbGFiaWxpdHlCEwoRX2NvbXBhcmVfYXRfcHJpY2U=');

@$core.Deprecated('Use productHashtagDescriptor instead')
const ProductHashtag$json = {
  '1': 'ProductHashtag',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'display', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'display'},
  ],
};

/// Descriptor for `ProductHashtag`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productHashtagDescriptor = $convert.base64Decode(
    'Cg5Qcm9kdWN0SGFzaHRhZxIXCgRuYW1lGAEgASgJQgPgQQNSBG5hbWUSHQoHZGlzcGxheRgCIA'
    'EoCUID4EEDUgdkaXNwbGF5');

@$core.Deprecated('Use productDescriptor instead')
const Product$json = {
  '1': 'Product',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'store',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '8': {},
      '10': 'store'
    },
    {'1': 'category_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'categoryId'},
    {'1': 'title', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'availability',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductAvailability',
      '8': {},
      '10': 'availability'
    },
    {
      '1': 'first_publish_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'firstPublishTime'
    },
    {'1': 'min_price', '3': 7, '4': 1, '5': 3, '8': {}, '10': 'minPrice'},
    {'1': 'max_price', '3': 8, '4': 1, '5': 3, '8': {}, '10': 'maxPrice'},
    {'1': 'sales_count', '3': 9, '4': 1, '5': 5, '8': {}, '10': 'salesCount'},
    {
      '1': 'favorite_count',
      '3': 10,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'favoriteCount'
    },
    {
      '1': 'rating_average',
      '3': 11,
      '4': 1,
      '5': 2,
      '8': {},
      '9': 0,
      '10': 'ratingAverage',
      '17': true
    },
    {
      '1': 'review_count',
      '3': 12,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'reviewCount'
    },
    {
      '1': 'gallery',
      '3': 13,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductImage',
      '8': {},
      '10': 'gallery'
    },
    {
      '1': 'description',
      '3': 14,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductDescriptionBlock',
      '8': {},
      '10': 'description'
    },
    {
      '1': 'axes',
      '3': 15,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.OptionAxis',
      '8': {},
      '10': 'axes'
    },
    {
      '1': 'skus',
      '3': 16,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Sku',
      '8': {},
      '10': 'skus'
    },
    {
      '1': 'hashtags',
      '3': 17,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductHashtag',
      '8': {},
      '10': 'hashtags'
    },
  ],
  '8': [
    {'1': '_rating_average'},
  ],
};

/// Descriptor for `Product`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productDescriptor = $convert.base64Decode(
    'CgdQcm9kdWN0EhMKAmlkGAEgASgJQgPgQQhSAmlkEjsKBXN0b3JlGAIgASgLMiAuaGF3a3IuY2'
    '9tbW9uLnYxYWxwaGExLlN0b3JlQ2FyZEID4EEDUgVzdG9yZRIkCgtjYXRlZ29yeV9pZBgDIAEo'
    'CUID4EEDUgpjYXRlZ29yeUlkEhkKBXRpdGxlGAQgASgJQgPgQQNSBXRpdGxlElQKDGF2YWlsYW'
    'JpbGl0eRgFIAEoDjIrLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHJvZHVjdEF2YWlsYWJpbGl0'
    'eUID4EEDUgxhdmFpbGFiaWxpdHkSTQoSZmlyc3RfcHVibGlzaF90aW1lGAYgASgLMhouZ29vZ2'
    'xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUhBmaXJzdFB1Ymxpc2hUaW1lEiAKCW1pbl9wcmlj'
    'ZRgHIAEoA0ID4EEDUghtaW5QcmljZRIgCgltYXhfcHJpY2UYCCABKANCA+BBA1IIbWF4UHJpY2'
    'USJAoLc2FsZXNfY291bnQYCSABKAVCA+BBA1IKc2FsZXNDb3VudBIqCg5mYXZvcml0ZV9jb3Vu'
    'dBgKIAEoBUID4EEDUg1mYXZvcml0ZUNvdW50Ei8KDnJhdGluZ19hdmVyYWdlGAsgASgCQgPgQQ'
    'NIAFINcmF0aW5nQXZlcmFnZYgBARImCgxyZXZpZXdfY291bnQYDCABKAVCA+BBA1ILcmV2aWV3'
    'Q291bnQSQwoHZ2FsbGVyeRgNIAMoCzIkLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHJvZHVjdE'
    'ltYWdlQgPgQQNSB2dhbGxlcnkSVgoLZGVzY3JpcHRpb24YDiADKAsyLy5oYXdrci5jYXRhbG9n'
    'LnYxYWxwaGExLlByb2R1Y3REZXNjcmlwdGlvbkJsb2NrQgPgQQNSC2Rlc2NyaXB0aW9uEjsKBG'
    'F4ZXMYDyADKAsyIi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLk9wdGlvbkF4aXNCA+BBA1IEYXhl'
    'cxI0CgRza3VzGBAgAygLMhsuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Ta3VCA+BBA1IEc2t1cx'
    'JHCghoYXNodGFncxgRIAMoCzImLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHJvZHVjdEhhc2h0'
    'YWdCA+BBA1IIaGFzaHRhZ3NCEQoPX3JhdGluZ19hdmVyYWdl');

@$core.Deprecated('Use productSummaryDescriptor instead')
const ProductSummary$json = {
  '1': 'ProductSummary',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'store',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '8': {},
      '10': 'store'
    },
    {'1': 'category_id', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'categoryId'},
    {'1': 'title', '3': 4, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'cover_image',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'coverImage'
    },
    {
      '1': 'availability',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductAvailability',
      '8': {},
      '10': 'availability'
    },
    {
      '1': 'first_publish_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'firstPublishTime'
    },
    {'1': 'min_price', '3': 8, '4': 1, '5': 3, '8': {}, '10': 'minPrice'},
    {'1': 'max_price', '3': 9, '4': 1, '5': 3, '8': {}, '10': 'maxPrice'},
    {'1': 'sales_count', '3': 10, '4': 1, '5': 5, '8': {}, '10': 'salesCount'},
    {
      '1': 'favorite_count',
      '3': 11,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'favoriteCount'
    },
    {
      '1': 'rating_average',
      '3': 12,
      '4': 1,
      '5': 2,
      '8': {},
      '9': 0,
      '10': 'ratingAverage',
      '17': true
    },
    {
      '1': 'review_count',
      '3': 13,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'reviewCount'
    },
  ],
  '8': [
    {'1': '_rating_average'},
  ],
};

/// Descriptor for `ProductSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productSummaryDescriptor = $convert.base64Decode(
    'Cg5Qcm9kdWN0U3VtbWFyeRITCgJpZBgBIAEoCUID4EEIUgJpZBI7CgVzdG9yZRgCIAEoCzIgLm'
    'hhd2tyLmNvbW1vbi52MWFscGhhMS5TdG9yZUNhcmRCA+BBA1IFc3RvcmUSJAoLY2F0ZWdvcnlf'
    'aWQYAyABKAlCA+BBA1IKY2F0ZWdvcnlJZBIZCgV0aXRsZRgEIAEoCUID4EEDUgV0aXRsZRJCCg'
    'tjb3Zlcl9pbWFnZRgFIAEoCzIcLmhhd2tyLmNvbW1vbi52MWFscGhhMS5JbWFnZUID4EEDUgpj'
    'b3ZlckltYWdlElQKDGF2YWlsYWJpbGl0eRgGIAEoDjIrLmhhd2tyLmNhdGFsb2cudjFhbHBoYT'
    'EuUHJvZHVjdEF2YWlsYWJpbGl0eUID4EEDUgxhdmFpbGFiaWxpdHkSTQoSZmlyc3RfcHVibGlz'
    'aF90aW1lGAcgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUhBmaXJzdFB1Ym'
    'xpc2hUaW1lEiAKCW1pbl9wcmljZRgIIAEoA0ID4EEDUghtaW5QcmljZRIgCgltYXhfcHJpY2UY'
    'CSABKANCA+BBA1IIbWF4UHJpY2USJAoLc2FsZXNfY291bnQYCiABKAVCA+BBA1IKc2FsZXNDb3'
    'VudBIqCg5mYXZvcml0ZV9jb3VudBgLIAEoBUID4EEDUg1mYXZvcml0ZUNvdW50Ei8KDnJhdGlu'
    'Z19hdmVyYWdlGAwgASgCQgPgQQNIAFINcmF0aW5nQXZlcmFnZYgBARImCgxyZXZpZXdfY291bn'
    'QYDSABKAVCA+BBA1ILcmV2aWV3Q291bnRCEQoPX3JhdGluZ19hdmVyYWdl');

@$core.Deprecated('Use sellerProductDescriptor instead')
const SellerProduct$json = {
  '1': 'SellerProduct',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'category_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'categoryId'},
    {'1': 'title', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'status',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductStatus',
      '8': {},
      '10': 'status'
    },
    {
      '1': 'first_publish_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'firstPublishTime'
    },
    {'1': 'min_price', '3': 6, '4': 1, '5': 3, '8': {}, '10': 'minPrice'},
    {'1': 'max_price', '3': 7, '4': 1, '5': 3, '8': {}, '10': 'maxPrice'},
    {'1': 'sales_count', '3': 8, '4': 1, '5': 5, '8': {}, '10': 'salesCount'},
    {
      '1': 'favorite_count',
      '3': 9,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'favoriteCount'
    },
    {
      '1': 'rating_average',
      '3': 10,
      '4': 1,
      '5': 2,
      '8': {},
      '9': 0,
      '10': 'ratingAverage',
      '17': true
    },
    {
      '1': 'review_count',
      '3': 11,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'reviewCount'
    },
    {
      '1': 'gallery',
      '3': 12,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductImage',
      '8': {},
      '10': 'gallery'
    },
    {
      '1': 'description',
      '3': 13,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductDescriptionBlock',
      '8': {},
      '10': 'description'
    },
    {
      '1': 'axes',
      '3': 14,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.OptionAxis',
      '8': {},
      '10': 'axes'
    },
    {
      '1': 'skus',
      '3': 15,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Sku',
      '8': {},
      '10': 'skus'
    },
    {
      '1': 'hashtags',
      '3': 16,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductHashtag',
      '8': {},
      '10': 'hashtags'
    },
    {
      '1': 'low_stock_threshold',
      '3': 17,
      '4': 1,
      '5': 5,
      '8': {},
      '9': 1,
      '10': 'lowStockThreshold',
      '17': true
    },
    {'1': 'etag', '3': 18, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'create_time',
      '3': 19,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'update_time',
      '3': 20,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
  ],
  '8': [
    {'1': '_rating_average'},
    {'1': '_low_stock_threshold'},
  ],
};

/// Descriptor for `SellerProduct`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerProductDescriptor = $convert.base64Decode(
    'Cg1TZWxsZXJQcm9kdWN0EhMKAmlkGAEgASgJQgPgQQhSAmlkEiQKC2NhdGVnb3J5X2lkGAIgAS'
    'gJQgPgQQNSCmNhdGVnb3J5SWQSGQoFdGl0bGUYAyABKAlCA+BBA1IFdGl0bGUSQgoGc3RhdHVz'
    'GAQgASgOMiUuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9kdWN0U3RhdHVzQgPgQQNSBnN0YX'
    'R1cxJNChJmaXJzdF9wdWJsaXNoX3RpbWUYBSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0'
    'YW1wQgPgQQNSEGZpcnN0UHVibGlzaFRpbWUSIAoJbWluX3ByaWNlGAYgASgDQgPgQQNSCG1pbl'
    'ByaWNlEiAKCW1heF9wcmljZRgHIAEoA0ID4EEDUghtYXhQcmljZRIkCgtzYWxlc19jb3VudBgI'
    'IAEoBUID4EEDUgpzYWxlc0NvdW50EioKDmZhdm9yaXRlX2NvdW50GAkgASgFQgPgQQNSDWZhdm'
    '9yaXRlQ291bnQSLwoOcmF0aW5nX2F2ZXJhZ2UYCiABKAJCA+BBA0gAUg1yYXRpbmdBdmVyYWdl'
    'iAEBEiYKDHJldmlld19jb3VudBgLIAEoBUID4EEDUgtyZXZpZXdDb3VudBJDCgdnYWxsZXJ5GA'
    'wgAygLMiQuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9kdWN0SW1hZ2VCA+BBA1IHZ2FsbGVy'
    'eRJWCgtkZXNjcmlwdGlvbhgNIAMoCzIvLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHJvZHVjdE'
    'Rlc2NyaXB0aW9uQmxvY2tCA+BBA1ILZGVzY3JpcHRpb24SOwoEYXhlcxgOIAMoCzIiLmhhd2ty'
    'LmNhdGFsb2cudjFhbHBoYTEuT3B0aW9uQXhpc0ID4EEDUgRheGVzEjQKBHNrdXMYDyADKAsyGy'
    '5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlNrdUID4EEDUgRza3VzEkcKCGhhc2h0YWdzGBAgAygL'
    'MiYuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9kdWN0SGFzaHRhZ0ID4EEDUghoYXNodGFncx'
    'I4ChNsb3dfc3RvY2tfdGhyZXNob2xkGBEgASgFQgPgQQNIAVIRbG93U3RvY2tUaHJlc2hvbGSI'
    'AQESFwoEZXRhZxgSIAEoCUID4EEDUgRldGFnEkAKC2NyZWF0ZV90aW1lGBMgASgLMhouZ29vZ2'
    'xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpjcmVhdGVUaW1lEkAKC3VwZGF0ZV90aW1lGBQg'
    'ASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgp1cGRhdGVUaW1lQhEKD19yYX'
    'RpbmdfYXZlcmFnZUIWChRfbG93X3N0b2NrX3RocmVzaG9sZA==');

@$core.Deprecated('Use sellerProductSummaryDescriptor instead')
const SellerProductSummary$json = {
  '1': 'SellerProductSummary',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'category_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'categoryId'},
    {'1': 'title', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'cover_image',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'coverImage'
    },
    {
      '1': 'status',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductStatus',
      '8': {},
      '10': 'status'
    },
    {
      '1': 'first_publish_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'firstPublishTime'
    },
    {'1': 'min_price', '3': 7, '4': 1, '5': 3, '8': {}, '10': 'minPrice'},
    {'1': 'max_price', '3': 8, '4': 1, '5': 3, '8': {}, '10': 'maxPrice'},
    {'1': 'sales_count', '3': 9, '4': 1, '5': 5, '8': {}, '10': 'salesCount'},
    {
      '1': 'favorite_count',
      '3': 10,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'favoriteCount'
    },
    {
      '1': 'rating_average',
      '3': 11,
      '4': 1,
      '5': 2,
      '8': {},
      '9': 0,
      '10': 'ratingAverage',
      '17': true
    },
    {
      '1': 'review_count',
      '3': 12,
      '4': 1,
      '5': 5,
      '8': {},
      '10': 'reviewCount'
    },
    {'1': 'total_stock', '3': 13, '4': 1, '5': 5, '8': {}, '10': 'totalStock'},
    {
      '1': 'low_stock_threshold',
      '3': 14,
      '4': 1,
      '5': 5,
      '8': {},
      '9': 1,
      '10': 'lowStockThreshold',
      '17': true
    },
    {'1': 'etag', '3': 15, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'create_time',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
  ],
  '8': [
    {'1': '_rating_average'},
    {'1': '_low_stock_threshold'},
  ],
};

/// Descriptor for `SellerProductSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerProductSummaryDescriptor = $convert.base64Decode(
    'ChRTZWxsZXJQcm9kdWN0U3VtbWFyeRITCgJpZBgBIAEoCUID4EEIUgJpZBIkCgtjYXRlZ29yeV'
    '9pZBgCIAEoCUID4EEDUgpjYXRlZ29yeUlkEhkKBXRpdGxlGAMgASgJQgPgQQNSBXRpdGxlEkIK'
    'C2NvdmVyX2ltYWdlGAQgASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaGExLkltYWdlQgPgQQNSCm'
    'NvdmVySW1hZ2USQgoGc3RhdHVzGAUgASgOMiUuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9k'
    'dWN0U3RhdHVzQgPgQQNSBnN0YXR1cxJNChJmaXJzdF9wdWJsaXNoX3RpbWUYBiABKAsyGi5nb2'
    '9nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSEGZpcnN0UHVibGlzaFRpbWUSIAoJbWluX3By'
    'aWNlGAcgASgDQgPgQQNSCG1pblByaWNlEiAKCW1heF9wcmljZRgIIAEoA0ID4EEDUghtYXhQcm'
    'ljZRIkCgtzYWxlc19jb3VudBgJIAEoBUID4EEDUgpzYWxlc0NvdW50EioKDmZhdm9yaXRlX2Nv'
    'dW50GAogASgFQgPgQQNSDWZhdm9yaXRlQ291bnQSLwoOcmF0aW5nX2F2ZXJhZ2UYCyABKAJCA+'
    'BBA0gAUg1yYXRpbmdBdmVyYWdliAEBEiYKDHJldmlld19jb3VudBgMIAEoBUID4EEDUgtyZXZp'
    'ZXdDb3VudBIkCgt0b3RhbF9zdG9jaxgNIAEoBUID4EEDUgp0b3RhbFN0b2NrEjgKE2xvd19zdG'
    '9ja190aHJlc2hvbGQYDiABKAVCA+BBA0gBUhFsb3dTdG9ja1RocmVzaG9sZIgBARIXCgRldGFn'
    'GA8gASgJQgPgQQNSBGV0YWcSQAoLY3JlYXRlX3RpbWUYECABKAsyGi5nb29nbGUucHJvdG9idW'
    'YuVGltZXN0YW1wQgPgQQNSCmNyZWF0ZVRpbWVCEQoPX3JhdGluZ19hdmVyYWdlQhYKFF9sb3df'
    'c3RvY2tfdGhyZXNob2xk');

@$core.Deprecated('Use productInputDescriptor instead')
const ProductInput$json = {
  '1': 'ProductInput',
  '2': [
    {'1': 'title', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {'1': 'category_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'categoryId'},
    {'1': 'hashtag_inputs', '3': 3, '4': 3, '5': 9, '10': 'hashtagInputs'},
    {
      '1': 'gallery',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ImageInput',
      '8': {},
      '10': 'gallery'
    },
    {
      '1': 'description',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.DescriptionBlock',
      '8': {},
      '10': 'description'
    },
    {
      '1': 'axes',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.OptionAxisInput',
      '8': {},
      '10': 'axes'
    },
    {
      '1': 'skus',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SkuInput',
      '8': {},
      '10': 'skus'
    },
    {
      '1': 'low_stock_threshold',
      '3': 8,
      '4': 1,
      '5': 5,
      '9': 0,
      '10': 'lowStockThreshold',
      '17': true
    },
  ],
  '8': [
    {'1': '_low_stock_threshold'},
  ],
};

/// Descriptor for `ProductInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productInputDescriptor = $convert.base64Decode(
    'CgxQcm9kdWN0SW5wdXQSHwoFdGl0bGUYASABKAlCCeBBApK1GAIQZFIFdGl0bGUSJAoLY2F0ZW'
    'dvcnlfaWQYAiABKAlCA+BBAlIKY2F0ZWdvcnlJZBIlCg5oYXNodGFnX2lucHV0cxgDIAMoCVIN'
    'aGFzaHRhZ0lucHV0cxJECgdnYWxsZXJ5GAQgAygLMiIuaGF3a3IuY2F0YWxvZy52MWFscGhhMS'
    '5JbWFnZUlucHV0QgaStRgCCAlSB2dhbGxlcnkSVwoLZGVzY3JpcHRpb24YBSADKAsyKC5oYXdr'
    'ci5jYXRhbG9nLnYxYWxwaGExLkRlc2NyaXB0aW9uQmxvY2tCC5K1GAcIZBi4FyAUUgtkZXNjcm'
    'lwdGlvbhJDCgRheGVzGAYgAygLMicuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5PcHRpb25BeGlz'
    'SW5wdXRCBpK1GAIIAlIEYXhlcxI/CgRza3VzGAcgAygLMiAuaGF3a3IuY2F0YWxvZy52MWFscG'
    'hhMS5Ta3VJbnB1dEIJ4EECkrUYAghkUgRza3VzEjMKE2xvd19zdG9ja190aHJlc2hvbGQYCCAB'
    'KAVIAFIRbG93U3RvY2tUaHJlc2hvbGSIAQFCFgoUX2xvd19zdG9ja190aHJlc2hvbGQ=');

@$core.Deprecated('Use imageInputDescriptor instead')
const ImageInput$json = {
  '1': 'ImageInput',
  '2': [
    {'1': 'staged_token', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'stagedToken'},
    {'1': 'asset_id', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'assetId'},
    {'1': 'alt', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'alt'},
  ],
  '8': [
    {'1': 'source'},
  ],
};

/// Descriptor for `ImageInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List imageInputDescriptor = $convert.base64Decode(
    'CgpJbWFnZUlucHV0EiMKDHN0YWdlZF90b2tlbhgBIAEoCUgAUgtzdGFnZWRUb2tlbhIbCghhc3'
    'NldF9pZBgCIAEoCUgAUgdhc3NldElkEhkKA2FsdBgDIAEoCUIHkrUYAxDIAVIDYWx0QggKBnNv'
    'dXJjZQ==');

@$core.Deprecated('Use descriptionBlockDescriptor instead')
const DescriptionBlock$json = {
  '1': 'DescriptionBlock',
  '2': [
    {'1': 'paragraph', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'paragraph'},
    {
      '1': 'image',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ImageInput',
      '9': 0,
      '10': 'image'
    },
    {
      '1': 'bullet_list',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.BulletList',
      '9': 0,
      '10': 'bulletList'
    },
    {
      '1': 'table',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.Table',
      '9': 0,
      '10': 'table'
    },
  ],
  '8': [
    {'1': 'block'},
  ],
};

/// Descriptor for `DescriptionBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List descriptionBlockDescriptor = $convert.base64Decode(
    'ChBEZXNjcmlwdGlvbkJsb2NrEh4KCXBhcmFncmFwaBgBIAEoCUgAUglwYXJhZ3JhcGgSOgoFaW'
    '1hZ2UYAiABKAsyIi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkltYWdlSW5wdXRIAFIFaW1hZ2US'
    'RQoLYnVsbGV0X2xpc3QYAyABKAsyIi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkJ1bGxldExpc3'
    'RIAFIKYnVsbGV0TGlzdBI1CgV0YWJsZRgEIAEoCzIdLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEu'
    'VGFibGVIAFIFdGFibGVCBwoFYmxvY2s=');

@$core.Deprecated('Use bulletListDescriptor instead')
const BulletList$json = {
  '1': 'BulletList',
  '2': [
    {'1': 'items', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'items'},
  ],
};

/// Descriptor for `BulletList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bulletListDescriptor = $convert.base64Decode(
    'CgpCdWxsZXRMaXN0Eh8KBWl0ZW1zGAEgAygJQgmStRgFCDIQyAFSBWl0ZW1z');

@$core.Deprecated('Use tableDescriptor instead')
const Table$json = {
  '1': 'Table',
  '2': [
    {
      '1': 'rows',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.TableRow',
      '8': {},
      '10': 'rows'
    },
  ],
};

/// Descriptor for `Table`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tableDescriptor = $convert.base64Decode(
    'CgVUYWJsZRI8CgRyb3dzGAEgAygLMiAuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5UYWJsZVJvd0'
    'IGkrUYAggyUgRyb3dz');

@$core.Deprecated('Use tableRowDescriptor instead')
const TableRow$json = {
  '1': 'TableRow',
  '2': [
    {'1': 'cells', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'cells'},
  ],
};

/// Descriptor for `TableRow`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tableRowDescriptor = $convert.base64Decode(
    'CghUYWJsZVJvdxIfCgVjZWxscxgBIAMoCUIJkrUYBQgKEMgBUgVjZWxscw==');

@$core.Deprecated('Use optionAxisInputDescriptor instead')
const OptionAxisInput$json = {
  '1': 'OptionAxisInput',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'values', '3': 2, '4': 3, '5': 9, '8': {}, '10': 'values'},
  ],
};

/// Descriptor for `OptionAxisInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List optionAxisInputDescriptor = $convert.base64Decode(
    'Cg9PcHRpb25BeGlzSW5wdXQSHQoEbmFtZRgBIAEoCUIJ4EECkrUYAhAUUgRuYW1lEiMKBnZhbH'
    'VlcxgCIAMoCUIL4EECkrUYBAgyEChSBnZhbHVlcw==');

@$core.Deprecated('Use skuInputDescriptor instead')
const SkuInput$json = {
  '1': 'SkuInput',
  '2': [
    {'1': 'option1', '3': 1, '4': 1, '5': 9, '10': 'option1'},
    {'1': 'option2', '3': 2, '4': 1, '5': 9, '10': 'option2'},
    {'1': 'price', '3': 3, '4': 1, '5': 3, '8': {}, '10': 'price'},
    {
      '1': 'compare_at_price',
      '3': 4,
      '4': 1,
      '5': 3,
      '9': 0,
      '10': 'compareAtPrice',
      '17': true
    },
    {'1': 'initial_stock', '3': 5, '4': 1, '5': 5, '10': 'initialStock'},
  ],
  '8': [
    {'1': '_compare_at_price'},
  ],
};

/// Descriptor for `SkuInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List skuInputDescriptor = $convert.base64Decode(
    'CghTa3VJbnB1dBIYCgdvcHRpb24xGAEgASgJUgdvcHRpb24xEhgKB29wdGlvbjIYAiABKAlSB2'
    '9wdGlvbjISGQoFcHJpY2UYAyABKANCA+BBAlIFcHJpY2USLQoQY29tcGFyZV9hdF9wcmljZRgE'
    'IAEoA0gAUg5jb21wYXJlQXRQcmljZYgBARIjCg1pbml0aWFsX3N0b2NrGAUgASgFUgxpbml0aW'
    'FsU3RvY2tCEwoRX2NvbXBhcmVfYXRfcHJpY2U=');

@$core.Deprecated('Use listStoreProductsRequestDescriptor instead')
const ListStoreProductsRequest$json = {
  '1': 'ListStoreProductsRequest',
  '2': [
    {
      '1': 'filter',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.MyProductFilter',
      '10': 'filter'
    },
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListStoreProductsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreProductsRequestDescriptor = $convert.base64Decode(
    'ChhMaXN0U3RvcmVQcm9kdWN0c1JlcXVlc3QSPwoGZmlsdGVyGAEgASgLMicuaGF3a3IuY2F0YW'
    'xvZy52MWFscGhhMS5NeVByb2R1Y3RGaWx0ZXJSBmZpbHRlchIbCglwYWdlX3NpemUYAiABKAVS'
    'CHBhZ2VTaXplEh0KCnBhZ2VfdG9rZW4YAyABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listStoreProductsResponseDescriptor instead')
const ListStoreProductsResponse$json = {
  '1': 'ListStoreProductsResponse',
  '2': [
    {
      '1': 'products',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProductSummary',
      '10': 'products'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListStoreProductsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreProductsResponseDescriptor = $convert.base64Decode(
    'ChlMaXN0U3RvcmVQcm9kdWN0c1Jlc3BvbnNlEkgKCHByb2R1Y3RzGAEgAygLMiwuaGF3a3IuY2'
    'F0YWxvZy52MWFscGhhMS5TZWxsZXJQcm9kdWN0U3VtbWFyeVIIcHJvZHVjdHMSJgoPbmV4dF9w'
    'YWdlX3Rva2VuGAIgASgJUg1uZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use getStoreProductRequestDescriptor instead')
const GetStoreProductRequest$json = {
  '1': 'GetStoreProductRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `GetStoreProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreProductRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRTdG9yZVByb2R1Y3RSZXF1ZXN0EiIKCnByb2R1Y3RfaWQYASABKAlCA+BBAlIJcHJvZH'
        'VjdElk');

@$core.Deprecated('Use getStoreProductResponseDescriptor instead')
const GetStoreProductResponse$json = {
  '1': 'GetStoreProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
  ],
};

/// Descriptor for `GetStoreProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreProductResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRTdG9yZVByb2R1Y3RSZXNwb25zZRI/Cgdwcm9kdWN0GAEgASgLMiUuaGF3a3IuY2F0YW'
        'xvZy52MWFscGhhMS5TZWxsZXJQcm9kdWN0Ugdwcm9kdWN0');

@$core.Deprecated('Use createStoreProductRequestDescriptor instead')
const CreateStoreProductRequest$json = {
  '1': 'CreateStoreProductRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {
      '1': 'product',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductInput',
      '8': {},
      '10': 'product'
    },
  ],
};

/// Descriptor for `CreateStoreProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createStoreProductRequestDescriptor = $convert.base64Decode(
    'ChlDcmVhdGVTdG9yZVByb2R1Y3RSZXF1ZXN0EkwKCW9wZXJhdGlvbhgBIAEoCzIpLmhhd2tyLm'
    'NvbW1vbi52MWFscGhhMS5EZWxlZ2F0ZWRPcGVyYXRpb25CA+BBAlIJb3BlcmF0aW9uEkMKB3By'
    'b2R1Y3QYAiABKAsyJC5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlByb2R1Y3RJbnB1dEID4EECUg'
    'dwcm9kdWN0');

@$core.Deprecated('Use createStoreProductResponseDescriptor instead')
const CreateStoreProductResponse$json = {
  '1': 'CreateStoreProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `CreateStoreProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createStoreProductResponseDescriptor =
    $convert.base64Decode(
        'ChpDcmVhdGVTdG9yZVByb2R1Y3RSZXNwb25zZRI/Cgdwcm9kdWN0GAEgASgLMiUuaGF3a3IuY2'
        'F0YWxvZy52MWFscGhhMS5TZWxsZXJQcm9kdWN0Ugdwcm9kdWN0EiAKDGF1ZGl0X2xvZ19pZBgC'
        'IAEoCVIKYXVkaXRMb2dJZA==');

@$core.Deprecated('Use updateStoreProductRequestDescriptor instead')
const UpdateStoreProductRequest$json = {
  '1': 'UpdateStoreProductRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'product_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'product',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.ProductInput',
      '8': {},
      '10': 'product'
    },
    {
      '1': 'update_mask',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '8': {},
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdateStoreProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateStoreProductRequestDescriptor = $convert.base64Decode(
    'ChlVcGRhdGVTdG9yZVByb2R1Y3RSZXF1ZXN0EkwKCW9wZXJhdGlvbhgBIAEoCzIpLmhhd2tyLm'
    'NvbW1vbi52MWFscGhhMS5EZWxlZ2F0ZWRPcGVyYXRpb25CA+BBAlIJb3BlcmF0aW9uEiIKCnBy'
    'b2R1Y3RfaWQYAiABKAlCA+BBAlIJcHJvZHVjdElkEhcKBGV0YWcYAyABKAlCA+BBAlIEZXRhZx'
    'JDCgdwcm9kdWN0GAQgASgLMiQuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5Qcm9kdWN0SW5wdXRC'
    'A+BBAlIHcHJvZHVjdBJACgt1cGRhdGVfbWFzaxgFIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5GaW'
    'VsZE1hc2tCA+BBAlIKdXBkYXRlTWFzaw==');

@$core.Deprecated('Use updateStoreProductResponseDescriptor instead')
const UpdateStoreProductResponse$json = {
  '1': 'UpdateStoreProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `UpdateStoreProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateStoreProductResponseDescriptor =
    $convert.base64Decode(
        'ChpVcGRhdGVTdG9yZVByb2R1Y3RSZXNwb25zZRI/Cgdwcm9kdWN0GAEgASgLMiUuaGF3a3IuY2'
        'F0YWxvZy52MWFscGhhMS5TZWxsZXJQcm9kdWN0Ugdwcm9kdWN0EiAKDGF1ZGl0X2xvZ19pZBgC'
        'IAEoCVIKYXVkaXRMb2dJZA==');

@$core.Deprecated('Use publishStoreProductRequestDescriptor instead')
const PublishStoreProductRequest$json = {
  '1': 'PublishStoreProductRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'product_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `PublishStoreProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishStoreProductRequestDescriptor = $convert.base64Decode(
    'ChpQdWJsaXNoU3RvcmVQcm9kdWN0UmVxdWVzdBJMCglvcGVyYXRpb24YASABKAsyKS5oYXdrci'
    '5jb21tb24udjFhbHBoYTEuRGVsZWdhdGVkT3BlcmF0aW9uQgPgQQJSCW9wZXJhdGlvbhIiCgpw'
    'cm9kdWN0X2lkGAIgASgJQgPgQQJSCXByb2R1Y3RJZBIXCgRldGFnGAMgASgJQgPgQQJSBGV0YW'
    'c=');

@$core.Deprecated('Use publishStoreProductResponseDescriptor instead')
const PublishStoreProductResponse$json = {
  '1': 'PublishStoreProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `PublishStoreProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishStoreProductResponseDescriptor =
    $convert.base64Decode(
        'ChtQdWJsaXNoU3RvcmVQcm9kdWN0UmVzcG9uc2USPwoHcHJvZHVjdBgBIAEoCzIlLmhhd2tyLm'
        'NhdGFsb2cudjFhbHBoYTEuU2VsbGVyUHJvZHVjdFIHcHJvZHVjdBIgCgxhdWRpdF9sb2dfaWQY'
        'AiABKAlSCmF1ZGl0TG9nSWQ=');

@$core.Deprecated('Use unpublishStoreProductRequestDescriptor instead')
const UnpublishStoreProductRequest$json = {
  '1': 'UnpublishStoreProductRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'product_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `UnpublishStoreProductRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishStoreProductRequestDescriptor = $convert.base64Decode(
    'ChxVbnB1Ymxpc2hTdG9yZVByb2R1Y3RSZXF1ZXN0EkwKCW9wZXJhdGlvbhgBIAEoCzIpLmhhd2'
    'tyLmNvbW1vbi52MWFscGhhMS5EZWxlZ2F0ZWRPcGVyYXRpb25CA+BBAlIJb3BlcmF0aW9uEiIK'
    'CnByb2R1Y3RfaWQYAiABKAlCA+BBAlIJcHJvZHVjdElkEhcKBGV0YWcYAyABKAlCA+BBAlIEZX'
    'RhZw==');

@$core.Deprecated('Use unpublishStoreProductResponseDescriptor instead')
const UnpublishStoreProductResponse$json = {
  '1': 'UnpublishStoreProductResponse',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.SellerProduct',
      '10': 'product'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `UnpublishStoreProductResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishStoreProductResponseDescriptor =
    $convert.base64Decode(
        'Ch1VbnB1Ymxpc2hTdG9yZVByb2R1Y3RSZXNwb25zZRI/Cgdwcm9kdWN0GAEgASgLMiUuaGF3a3'
        'IuY2F0YWxvZy52MWFscGhhMS5TZWxsZXJQcm9kdWN0Ugdwcm9kdWN0EiAKDGF1ZGl0X2xvZ19p'
        'ZBgCIAEoCVIKYXVkaXRMb2dJZA==');

@$core.Deprecated('Use batchUnpublishStoreProductsRequestDescriptor instead')
const BatchUnpublishStoreProductsRequest$json = {
  '1': 'BatchUnpublishStoreProductsRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {
      '1': 'items',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.BatchUnpublishItem',
      '8': {},
      '10': 'items'
    },
  ],
};

/// Descriptor for `BatchUnpublishStoreProductsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchUnpublishStoreProductsRequestDescriptor =
    $convert.base64Decode(
        'CiJCYXRjaFVucHVibGlzaFN0b3JlUHJvZHVjdHNSZXF1ZXN0EkwKCW9wZXJhdGlvbhgBIAEoCz'
        'IpLmhhd2tyLmNvbW1vbi52MWFscGhhMS5EZWxlZ2F0ZWRPcGVyYXRpb25CA+BBAlIJb3BlcmF0'
        'aW9uEksKBWl0ZW1zGAIgAygLMiouaGF3a3IuY2F0YWxvZy52MWFscGhhMS5CYXRjaFVucHVibG'
        'lzaEl0ZW1CCeBBApK1GAIIMlIFaXRlbXM=');

@$core.Deprecated('Use batchUnpublishItemDescriptor instead')
const BatchUnpublishItem$json = {
  '1': 'BatchUnpublishItem',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `BatchUnpublishItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchUnpublishItemDescriptor = $convert.base64Decode(
    'ChJCYXRjaFVucHVibGlzaEl0ZW0SIgoKcHJvZHVjdF9pZBgBIAEoCUID4EECUglwcm9kdWN0SW'
    'QSFwoEZXRhZxgCIAEoCUID4EECUgRldGFn');

@$core.Deprecated('Use batchUnpublishStoreProductsResponseDescriptor instead')
const BatchUnpublishStoreProductsResponse$json = {
  '1': 'BatchUnpublishStoreProductsResponse',
  '2': [
    {
      '1': 'products',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.catalog.v1alpha1.UnpublishedProduct',
      '10': 'products'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `BatchUnpublishStoreProductsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchUnpublishStoreProductsResponseDescriptor =
    $convert.base64Decode(
        'CiNCYXRjaFVucHVibGlzaFN0b3JlUHJvZHVjdHNSZXNwb25zZRJGCghwcm9kdWN0cxgBIAMoCz'
        'IqLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuVW5wdWJsaXNoZWRQcm9kdWN0Ughwcm9kdWN0cxIg'
        'CgxhdWRpdF9sb2dfaWQYAiABKAlSCmF1ZGl0TG9nSWQ=');

@$core.Deprecated('Use unpublishedProductDescriptor instead')
const UnpublishedProduct$json = {
  '1': 'UnpublishedProduct',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.catalog.v1alpha1.ProductStatus',
      '8': {},
      '10': 'status'
    },
  ],
};

/// Descriptor for `UnpublishedProduct`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishedProductDescriptor = $convert.base64Decode(
    'ChJVbnB1Ymxpc2hlZFByb2R1Y3QSIgoKcHJvZHVjdF9pZBgBIAEoCUID4EEDUglwcm9kdWN0SW'
    'QSFwoEZXRhZxgCIAEoCUID4EEDUgRldGFnEkIKBnN0YXR1cxgDIAEoDjIlLmhhd2tyLmNhdGFs'
    'b2cudjFhbHBoYTEuUHJvZHVjdFN0YXR1c0ID4EEDUgZzdGF0dXM=');

const $core.Map<$core.String, $core.dynamic> CatalogPublicServiceBase$json = {
  '1': 'CatalogPublicService',
  '2': [
    {
      '1': 'ListProducts',
      '2': '.hawkr.catalog.v1alpha1.ListProductsRequest',
      '3': '.hawkr.catalog.v1alpha1.ListProductsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SearchProducts',
      '2': '.hawkr.catalog.v1alpha1.SearchProductsRequest',
      '3': '.hawkr.catalog.v1alpha1.SearchProductsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SuggestProductNames',
      '2': '.hawkr.catalog.v1alpha1.SuggestProductNamesRequest',
      '3': '.hawkr.catalog.v1alpha1.SuggestProductNamesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetProduct',
      '2': '.hawkr.catalog.v1alpha1.GetProductRequest',
      '3': '.hawkr.catalog.v1alpha1.GetProductResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListCategories',
      '2': '.hawkr.catalog.v1alpha1.ListCategoriesRequest',
      '3': '.hawkr.catalog.v1alpha1.ListCategoriesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SuggestHashtags',
      '2': '.hawkr.catalog.v1alpha1.SuggestHashtagsRequest',
      '3': '.hawkr.catalog.v1alpha1.SuggestHashtagsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SuggestTrendingHashtags',
      '2': '.hawkr.catalog.v1alpha1.SuggestTrendingHashtagsRequest',
      '3': '.hawkr.catalog.v1alpha1.SuggestTrendingHashtagsResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use catalogPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    CatalogPublicServiceBase$messageJson = {
  '.hawkr.catalog.v1alpha1.ListProductsRequest': ListProductsRequest$json,
  '.hawkr.catalog.v1alpha1.ProductFilter': ProductFilter$json,
  '.hawkr.catalog.v1alpha1.ListProductsResponse': ListProductsResponse$json,
  '.hawkr.catalog.v1alpha1.ProductSummary': ProductSummary$json,
  '.hawkr.common.v1alpha1.StoreCard': $1.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.catalog.v1alpha1.SearchProductsRequest': SearchProductsRequest$json,
  '.hawkr.catalog.v1alpha1.SearchProductsResponse': SearchProductsResponse$json,
  '.hawkr.catalog.v1alpha1.SuggestProductNamesRequest':
      SuggestProductNamesRequest$json,
  '.hawkr.catalog.v1alpha1.SuggestProductNamesResponse':
      SuggestProductNamesResponse$json,
  '.hawkr.catalog.v1alpha1.GetProductRequest': GetProductRequest$json,
  '.hawkr.catalog.v1alpha1.GetProductResponse': GetProductResponse$json,
  '.hawkr.catalog.v1alpha1.Product': Product$json,
  '.hawkr.catalog.v1alpha1.ProductImage': ProductImage$json,
  '.hawkr.catalog.v1alpha1.ProductDescriptionBlock':
      ProductDescriptionBlock$json,
  '.hawkr.catalog.v1alpha1.BulletList': BulletList$json,
  '.hawkr.catalog.v1alpha1.Table': Table$json,
  '.hawkr.catalog.v1alpha1.TableRow': TableRow$json,
  '.hawkr.catalog.v1alpha1.OptionAxis': OptionAxis$json,
  '.hawkr.catalog.v1alpha1.Sku': Sku$json,
  '.hawkr.catalog.v1alpha1.ProductHashtag': ProductHashtag$json,
  '.hawkr.catalog.v1alpha1.ListCategoriesRequest': ListCategoriesRequest$json,
  '.hawkr.catalog.v1alpha1.ListCategoriesResponse': ListCategoriesResponse$json,
  '.hawkr.catalog.v1alpha1.Category': Category$json,
  '.hawkr.catalog.v1alpha1.SuggestHashtagsRequest': SuggestHashtagsRequest$json,
  '.hawkr.catalog.v1alpha1.SuggestHashtagsResponse':
      SuggestHashtagsResponse$json,
  '.hawkr.catalog.v1alpha1.HashtagCount': HashtagCount$json,
  '.hawkr.catalog.v1alpha1.SuggestTrendingHashtagsRequest':
      SuggestTrendingHashtagsRequest$json,
  '.hawkr.catalog.v1alpha1.SuggestTrendingHashtagsResponse':
      SuggestTrendingHashtagsResponse$json,
};

/// Descriptor for `CatalogPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List catalogPublicServiceDescriptor = $convert.base64Decode(
    'ChRDYXRhbG9nUHVibGljU2VydmljZRJuCgxMaXN0UHJvZHVjdHMSKy5oYXdrci5jYXRhbG9nLn'
    'YxYWxwaGExLkxpc3RQcm9kdWN0c1JlcXVlc3QaLC5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkxp'
    'c3RQcm9kdWN0c1Jlc3BvbnNlIgOQAgESdAoOU2VhcmNoUHJvZHVjdHMSLS5oYXdrci5jYXRhbG'
    '9nLnYxYWxwaGExLlNlYXJjaFByb2R1Y3RzUmVxdWVzdBouLmhhd2tyLmNhdGFsb2cudjFhbHBo'
    'YTEuU2VhcmNoUHJvZHVjdHNSZXNwb25zZSIDkAIBEoMBChNTdWdnZXN0UHJvZHVjdE5hbWVzEj'
    'IuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5TdWdnZXN0UHJvZHVjdE5hbWVzUmVxdWVzdBozLmhh'
    'd2tyLmNhdGFsb2cudjFhbHBoYTEuU3VnZ2VzdFByb2R1Y3ROYW1lc1Jlc3BvbnNlIgOQAgESaA'
    'oKR2V0UHJvZHVjdBIpLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuR2V0UHJvZHVjdFJlcXVlc3Qa'
    'Ki5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkdldFByb2R1Y3RSZXNwb25zZSIDkAIBEnQKDkxpc3'
    'RDYXRlZ29yaWVzEi0uaGF3a3IuY2F0YWxvZy52MWFscGhhMS5MaXN0Q2F0ZWdvcmllc1JlcXVl'
    'c3QaLi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkxpc3RDYXRlZ29yaWVzUmVzcG9uc2UiA5ACAR'
    'J3Cg9TdWdnZXN0SGFzaHRhZ3MSLi5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlN1Z2dlc3RIYXNo'
    'dGFnc1JlcXVlc3QaLy5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlN1Z2dlc3RIYXNodGFnc1Jlc3'
    'BvbnNlIgOQAgESjwEKF1N1Z2dlc3RUcmVuZGluZ0hhc2h0YWdzEjYuaGF3a3IuY2F0YWxvZy52'
    'MWFscGhhMS5TdWdnZXN0VHJlbmRpbmdIYXNodGFnc1JlcXVlc3QaNy5oYXdrci5jYXRhbG9nLn'
    'YxYWxwaGExLlN1Z2dlc3RUcmVuZGluZ0hhc2h0YWdzUmVzcG9uc2UiA5ACAQ==');

const $core.Map<$core.String, $core.dynamic> SellerCatalogAppServiceBase$json =
    {
  '1': 'SellerCatalogAppService',
  '2': [
    {
      '1': 'ListMyProducts',
      '2': '.hawkr.catalog.v1alpha1.ListMyProductsRequest',
      '3': '.hawkr.catalog.v1alpha1.ListMyProductsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetMyProduct',
      '2': '.hawkr.catalog.v1alpha1.GetMyProductRequest',
      '3': '.hawkr.catalog.v1alpha1.GetMyProductResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CountMyProducts',
      '2': '.hawkr.catalog.v1alpha1.CountMyProductsRequest',
      '3': '.hawkr.catalog.v1alpha1.CountMyProductsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CreateProduct',
      '2': '.hawkr.catalog.v1alpha1.CreateProductRequest',
      '3': '.hawkr.catalog.v1alpha1.CreateProductResponse'
    },
    {
      '1': 'UpdateProduct',
      '2': '.hawkr.catalog.v1alpha1.UpdateProductRequest',
      '3': '.hawkr.catalog.v1alpha1.UpdateProductResponse',
      '4': {'34': 2},
    },
    {
      '1': 'SetProductStatus',
      '2': '.hawkr.catalog.v1alpha1.SetProductStatusRequest',
      '3': '.hawkr.catalog.v1alpha1.SetProductStatusResponse',
      '4': {'34': 2},
    },
    {
      '1': 'DeleteProduct',
      '2': '.hawkr.catalog.v1alpha1.DeleteProductRequest',
      '3': '.hawkr.catalog.v1alpha1.DeleteProductResponse',
      '4': {'34': 2},
    },
    {
      '1': 'AdjustStock',
      '2': '.hawkr.catalog.v1alpha1.AdjustStockRequest',
      '3': '.hawkr.catalog.v1alpha1.AdjustStockResponse'
    },
    {
      '1': 'SetStock',
      '2': '.hawkr.catalog.v1alpha1.SetStockRequest',
      '3': '.hawkr.catalog.v1alpha1.SetStockResponse'
    },
  ],
};

@$core.Deprecated('Use sellerCatalogAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SellerCatalogAppServiceBase$messageJson = {
  '.hawkr.catalog.v1alpha1.ListMyProductsRequest': ListMyProductsRequest$json,
  '.hawkr.catalog.v1alpha1.MyProductFilter': MyProductFilter$json,
  '.hawkr.catalog.v1alpha1.ListMyProductsResponse': ListMyProductsResponse$json,
  '.hawkr.catalog.v1alpha1.SellerProductSummary': SellerProductSummary$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.catalog.v1alpha1.GetMyProductRequest': GetMyProductRequest$json,
  '.hawkr.catalog.v1alpha1.GetMyProductResponse': GetMyProductResponse$json,
  '.hawkr.catalog.v1alpha1.SellerProduct': SellerProduct$json,
  '.hawkr.catalog.v1alpha1.ProductImage': ProductImage$json,
  '.hawkr.catalog.v1alpha1.ProductDescriptionBlock':
      ProductDescriptionBlock$json,
  '.hawkr.catalog.v1alpha1.BulletList': BulletList$json,
  '.hawkr.catalog.v1alpha1.Table': Table$json,
  '.hawkr.catalog.v1alpha1.TableRow': TableRow$json,
  '.hawkr.catalog.v1alpha1.OptionAxis': OptionAxis$json,
  '.hawkr.catalog.v1alpha1.Sku': Sku$json,
  '.hawkr.catalog.v1alpha1.ProductHashtag': ProductHashtag$json,
  '.hawkr.catalog.v1alpha1.CountMyProductsRequest': CountMyProductsRequest$json,
  '.hawkr.catalog.v1alpha1.CountMyProductsResponse':
      CountMyProductsResponse$json,
  '.hawkr.catalog.v1alpha1.CreateProductRequest': CreateProductRequest$json,
  '.hawkr.catalog.v1alpha1.ProductInput': ProductInput$json,
  '.hawkr.catalog.v1alpha1.ImageInput': ImageInput$json,
  '.hawkr.catalog.v1alpha1.DescriptionBlock': DescriptionBlock$json,
  '.hawkr.catalog.v1alpha1.OptionAxisInput': OptionAxisInput$json,
  '.hawkr.catalog.v1alpha1.SkuInput': SkuInput$json,
  '.hawkr.catalog.v1alpha1.CreateProductResponse': CreateProductResponse$json,
  '.hawkr.catalog.v1alpha1.UpdateProductRequest': UpdateProductRequest$json,
  '.hawkr.catalog.v1alpha1.UpdateProductResponse': UpdateProductResponse$json,
  '.hawkr.catalog.v1alpha1.SetProductStatusRequest':
      SetProductStatusRequest$json,
  '.hawkr.catalog.v1alpha1.SetProductStatusResponse':
      SetProductStatusResponse$json,
  '.hawkr.catalog.v1alpha1.DeleteProductRequest': DeleteProductRequest$json,
  '.hawkr.catalog.v1alpha1.DeleteProductResponse': DeleteProductResponse$json,
  '.hawkr.catalog.v1alpha1.AdjustStockRequest': AdjustStockRequest$json,
  '.hawkr.catalog.v1alpha1.AdjustStockResponse': AdjustStockResponse$json,
  '.hawkr.catalog.v1alpha1.SetStockRequest': SetStockRequest$json,
  '.hawkr.catalog.v1alpha1.SetStockResponse': SetStockResponse$json,
};

/// Descriptor for `SellerCatalogAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List sellerCatalogAppServiceDescriptor = $convert.base64Decode(
    'ChdTZWxsZXJDYXRhbG9nQXBwU2VydmljZRJ0Cg5MaXN0TXlQcm9kdWN0cxItLmhhd2tyLmNhdG'
    'Fsb2cudjFhbHBoYTEuTGlzdE15UHJvZHVjdHNSZXF1ZXN0Gi4uaGF3a3IuY2F0YWxvZy52MWFs'
    'cGhhMS5MaXN0TXlQcm9kdWN0c1Jlc3BvbnNlIgOQAgESbgoMR2V0TXlQcm9kdWN0EisuaGF3a3'
    'IuY2F0YWxvZy52MWFscGhhMS5HZXRNeVByb2R1Y3RSZXF1ZXN0GiwuaGF3a3IuY2F0YWxvZy52'
    'MWFscGhhMS5HZXRNeVByb2R1Y3RSZXNwb25zZSIDkAIBEncKD0NvdW50TXlQcm9kdWN0cxIuLm'
    'hhd2tyLmNhdGFsb2cudjFhbHBoYTEuQ291bnRNeVByb2R1Y3RzUmVxdWVzdBovLmhhd2tyLmNh'
    'dGFsb2cudjFhbHBoYTEuQ291bnRNeVByb2R1Y3RzUmVzcG9uc2UiA5ACARJsCg1DcmVhdGVQcm'
    '9kdWN0EiwuaGF3a3IuY2F0YWxvZy52MWFscGhhMS5DcmVhdGVQcm9kdWN0UmVxdWVzdBotLmhh'
    'd2tyLmNhdGFsb2cudjFhbHBoYTEuQ3JlYXRlUHJvZHVjdFJlc3BvbnNlEnEKDVVwZGF0ZVByb2'
    'R1Y3QSLC5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlVwZGF0ZVByb2R1Y3RSZXF1ZXN0Gi0uaGF3'
    'a3IuY2F0YWxvZy52MWFscGhhMS5VcGRhdGVQcm9kdWN0UmVzcG9uc2UiA5ACAhJ6ChBTZXRQcm'
    '9kdWN0U3RhdHVzEi8uaGF3a3IuY2F0YWxvZy52MWFscGhhMS5TZXRQcm9kdWN0U3RhdHVzUmVx'
    'dWVzdBowLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuU2V0UHJvZHVjdFN0YXR1c1Jlc3BvbnNlIg'
    'OQAgIScQoNRGVsZXRlUHJvZHVjdBIsLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuRGVsZXRlUHJv'
    'ZHVjdFJlcXVlc3QaLS5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkRlbGV0ZVByb2R1Y3RSZXNwb2'
    '5zZSIDkAICEmYKC0FkanVzdFN0b2NrEiouaGF3a3IuY2F0YWxvZy52MWFscGhhMS5BZGp1c3RT'
    'dG9ja1JlcXVlc3QaKy5oYXdrci5jYXRhbG9nLnYxYWxwaGExLkFkanVzdFN0b2NrUmVzcG9uc2'
    'USXQoIU2V0U3RvY2sSJy5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlNldFN0b2NrUmVxdWVzdBoo'
    'Lmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuU2V0U3RvY2tSZXNwb25zZQ==');

const $core.Map<$core.String, $core.dynamic> CatalogDelegatedServiceBase$json =
    {
  '1': 'CatalogDelegatedService',
  '2': [
    {
      '1': 'ListStoreProducts',
      '2': '.hawkr.catalog.v1alpha1.ListStoreProductsRequest',
      '3': '.hawkr.catalog.v1alpha1.ListStoreProductsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetStoreProduct',
      '2': '.hawkr.catalog.v1alpha1.GetStoreProductRequest',
      '3': '.hawkr.catalog.v1alpha1.GetStoreProductResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CreateStoreProduct',
      '2': '.hawkr.catalog.v1alpha1.CreateStoreProductRequest',
      '3': '.hawkr.catalog.v1alpha1.CreateStoreProductResponse'
    },
    {
      '1': 'UpdateStoreProduct',
      '2': '.hawkr.catalog.v1alpha1.UpdateStoreProductRequest',
      '3': '.hawkr.catalog.v1alpha1.UpdateStoreProductResponse',
      '4': {'34': 2},
    },
    {
      '1': 'PublishStoreProduct',
      '2': '.hawkr.catalog.v1alpha1.PublishStoreProductRequest',
      '3': '.hawkr.catalog.v1alpha1.PublishStoreProductResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnpublishStoreProduct',
      '2': '.hawkr.catalog.v1alpha1.UnpublishStoreProductRequest',
      '3': '.hawkr.catalog.v1alpha1.UnpublishStoreProductResponse',
      '4': {'34': 2},
    },
    {
      '1': 'BatchUnpublishStoreProducts',
      '2': '.hawkr.catalog.v1alpha1.BatchUnpublishStoreProductsRequest',
      '3': '.hawkr.catalog.v1alpha1.BatchUnpublishStoreProductsResponse',
      '4': {'34': 2},
    },
  ],
};

@$core.Deprecated('Use catalogDelegatedServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    CatalogDelegatedServiceBase$messageJson = {
  '.hawkr.catalog.v1alpha1.ListStoreProductsRequest':
      ListStoreProductsRequest$json,
  '.hawkr.catalog.v1alpha1.MyProductFilter': MyProductFilter$json,
  '.hawkr.catalog.v1alpha1.ListStoreProductsResponse':
      ListStoreProductsResponse$json,
  '.hawkr.catalog.v1alpha1.SellerProductSummary': SellerProductSummary$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.catalog.v1alpha1.GetStoreProductRequest': GetStoreProductRequest$json,
  '.hawkr.catalog.v1alpha1.GetStoreProductResponse':
      GetStoreProductResponse$json,
  '.hawkr.catalog.v1alpha1.SellerProduct': SellerProduct$json,
  '.hawkr.catalog.v1alpha1.ProductImage': ProductImage$json,
  '.hawkr.catalog.v1alpha1.ProductDescriptionBlock':
      ProductDescriptionBlock$json,
  '.hawkr.catalog.v1alpha1.BulletList': BulletList$json,
  '.hawkr.catalog.v1alpha1.Table': Table$json,
  '.hawkr.catalog.v1alpha1.TableRow': TableRow$json,
  '.hawkr.catalog.v1alpha1.OptionAxis': OptionAxis$json,
  '.hawkr.catalog.v1alpha1.Sku': Sku$json,
  '.hawkr.catalog.v1alpha1.ProductHashtag': ProductHashtag$json,
  '.hawkr.catalog.v1alpha1.CreateStoreProductRequest':
      CreateStoreProductRequest$json,
  '.hawkr.common.v1alpha1.DelegatedOperation': $3.DelegatedOperation$json,
  '.hawkr.catalog.v1alpha1.ProductInput': ProductInput$json,
  '.hawkr.catalog.v1alpha1.ImageInput': ImageInput$json,
  '.hawkr.catalog.v1alpha1.DescriptionBlock': DescriptionBlock$json,
  '.hawkr.catalog.v1alpha1.OptionAxisInput': OptionAxisInput$json,
  '.hawkr.catalog.v1alpha1.SkuInput': SkuInput$json,
  '.hawkr.catalog.v1alpha1.CreateStoreProductResponse':
      CreateStoreProductResponse$json,
  '.hawkr.catalog.v1alpha1.UpdateStoreProductRequest':
      UpdateStoreProductRequest$json,
  '.google.protobuf.FieldMask': $4.FieldMask$json,
  '.hawkr.catalog.v1alpha1.UpdateStoreProductResponse':
      UpdateStoreProductResponse$json,
  '.hawkr.catalog.v1alpha1.PublishStoreProductRequest':
      PublishStoreProductRequest$json,
  '.hawkr.catalog.v1alpha1.PublishStoreProductResponse':
      PublishStoreProductResponse$json,
  '.hawkr.catalog.v1alpha1.UnpublishStoreProductRequest':
      UnpublishStoreProductRequest$json,
  '.hawkr.catalog.v1alpha1.UnpublishStoreProductResponse':
      UnpublishStoreProductResponse$json,
  '.hawkr.catalog.v1alpha1.BatchUnpublishStoreProductsRequest':
      BatchUnpublishStoreProductsRequest$json,
  '.hawkr.catalog.v1alpha1.BatchUnpublishItem': BatchUnpublishItem$json,
  '.hawkr.catalog.v1alpha1.BatchUnpublishStoreProductsResponse':
      BatchUnpublishStoreProductsResponse$json,
  '.hawkr.catalog.v1alpha1.UnpublishedProduct': UnpublishedProduct$json,
};

/// Descriptor for `CatalogDelegatedService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List catalogDelegatedServiceDescriptor = $convert.base64Decode(
    'ChdDYXRhbG9nRGVsZWdhdGVkU2VydmljZRJ9ChFMaXN0U3RvcmVQcm9kdWN0cxIwLmhhd2tyLm'
    'NhdGFsb2cudjFhbHBoYTEuTGlzdFN0b3JlUHJvZHVjdHNSZXF1ZXN0GjEuaGF3a3IuY2F0YWxv'
    'Zy52MWFscGhhMS5MaXN0U3RvcmVQcm9kdWN0c1Jlc3BvbnNlIgOQAgESdwoPR2V0U3RvcmVQcm'
    '9kdWN0Ei4uaGF3a3IuY2F0YWxvZy52MWFscGhhMS5HZXRTdG9yZVByb2R1Y3RSZXF1ZXN0Gi8u'
    'aGF3a3IuY2F0YWxvZy52MWFscGhhMS5HZXRTdG9yZVByb2R1Y3RSZXNwb25zZSIDkAIBEnsKEk'
    'NyZWF0ZVN0b3JlUHJvZHVjdBIxLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuQ3JlYXRlU3RvcmVQ'
    'cm9kdWN0UmVxdWVzdBoyLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuQ3JlYXRlU3RvcmVQcm9kdW'
    'N0UmVzcG9uc2USgAEKElVwZGF0ZVN0b3JlUHJvZHVjdBIxLmhhd2tyLmNhdGFsb2cudjFhbHBo'
    'YTEuVXBkYXRlU3RvcmVQcm9kdWN0UmVxdWVzdBoyLmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuVX'
    'BkYXRlU3RvcmVQcm9kdWN0UmVzcG9uc2UiA5ACAhKDAQoTUHVibGlzaFN0b3JlUHJvZHVjdBIy'
    'Lmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuUHVibGlzaFN0b3JlUHJvZHVjdFJlcXVlc3QaMy5oYX'
    'drci5jYXRhbG9nLnYxYWxwaGExLlB1Ymxpc2hTdG9yZVByb2R1Y3RSZXNwb25zZSIDkAICEokB'
    'ChVVbnB1Ymxpc2hTdG9yZVByb2R1Y3QSNC5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlVucHVibG'
    'lzaFN0b3JlUHJvZHVjdFJlcXVlc3QaNS5oYXdrci5jYXRhbG9nLnYxYWxwaGExLlVucHVibGlz'
    'aFN0b3JlUHJvZHVjdFJlc3BvbnNlIgOQAgISmwEKG0JhdGNoVW5wdWJsaXNoU3RvcmVQcm9kdW'
    'N0cxI6Lmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuQmF0Y2hVbnB1Ymxpc2hTdG9yZVByb2R1Y3Rz'
    'UmVxdWVzdBo7Lmhhd2tyLmNhdGFsb2cudjFhbHBoYTEuQmF0Y2hVbnB1Ymxpc2hTdG9yZVByb2'
    'R1Y3RzUmVzcG9uc2UiA5ACAg==');
