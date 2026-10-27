// This is a generated file - do not edit.
//
// Generated from hawkr/discovery/v1alpha1/discovery.proto.

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

import '../../common/v1alpha1/image.pbjson.dart' as $1;
import '../../common/v1alpha1/store_card.pbjson.dart' as $0;

@$core.Deprecated('Use suggestRecommendedSellersRequestDescriptor instead')
const SuggestRecommendedSellersRequest$json = {
  '1': 'SuggestRecommendedSellersRequest',
  '2': [
    {'1': 'max_results', '3': 1, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestRecommendedSellersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestRecommendedSellersRequestDescriptor =
    $convert.base64Decode(
        'CiBTdWdnZXN0UmVjb21tZW5kZWRTZWxsZXJzUmVxdWVzdBIfCgttYXhfcmVzdWx0cxgBIAEoBV'
        'IKbWF4UmVzdWx0cw==');

@$core.Deprecated('Use suggestRecommendedSellersResponseDescriptor instead')
const SuggestRecommendedSellersResponse$json = {
  '1': 'SuggestRecommendedSellersResponse',
  '2': [
    {
      '1': 'sellers',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.discovery.v1alpha1.RecommendedSeller',
      '10': 'sellers'
    },
  ],
};

/// Descriptor for `SuggestRecommendedSellersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestRecommendedSellersResponseDescriptor =
    $convert.base64Decode(
        'CiFTdWdnZXN0UmVjb21tZW5kZWRTZWxsZXJzUmVzcG9uc2USRQoHc2VsbGVycxgBIAMoCzIrLm'
        'hhd2tyLmRpc2NvdmVyeS52MWFscGhhMS5SZWNvbW1lbmRlZFNlbGxlclIHc2VsbGVycw==');

@$core.Deprecated('Use recommendedSellerDescriptor instead')
const RecommendedSeller$json = {
  '1': 'RecommendedSeller',
  '2': [
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'store'
    },
    {
      '1': 'follower_count',
      '3': 4,
      '4': 1,
      '5': 5,
      '9': 0,
      '10': 'followerCount',
      '17': true
    },
  ],
  '8': [
    {'1': '_follower_count'},
  ],
  '9': [
    {'1': 2, '2': 3},
    {'1': 3, '2': 4},
  ],
  '10': ['review_count', 'rating_average'],
};

/// Descriptor for `RecommendedSeller`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recommendedSellerDescriptor = $convert.base64Decode(
    'ChFSZWNvbW1lbmRlZFNlbGxlchI2CgVzdG9yZRgBIAEoCzIgLmhhd2tyLmNvbW1vbi52MWFscG'
    'hhMS5TdG9yZUNhcmRSBXN0b3JlEioKDmZvbGxvd2VyX2NvdW50GAQgASgFSABSDWZvbGxvd2Vy'
    'Q291bnSIAQFCEQoPX2ZvbGxvd2VyX2NvdW50SgQIAhADSgQIAxAEUgxyZXZpZXdfY291bnRSDn'
    'JhdGluZ19hdmVyYWdl');

const $core.Map<$core.String, $core.dynamic> DiscoveryPublicServiceBase$json = {
  '1': 'DiscoveryPublicService',
  '2': [
    {
      '1': 'SuggestRecommendedSellers',
      '2': '.hawkr.discovery.v1alpha1.SuggestRecommendedSellersRequest',
      '3': '.hawkr.discovery.v1alpha1.SuggestRecommendedSellersResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use discoveryPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    DiscoveryPublicServiceBase$messageJson = {
  '.hawkr.discovery.v1alpha1.SuggestRecommendedSellersRequest':
      SuggestRecommendedSellersRequest$json,
  '.hawkr.discovery.v1alpha1.SuggestRecommendedSellersResponse':
      SuggestRecommendedSellersResponse$json,
  '.hawkr.discovery.v1alpha1.RecommendedSeller': RecommendedSeller$json,
  '.hawkr.common.v1alpha1.StoreCard': $0.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $1.Image$json,
};

/// Descriptor for `DiscoveryPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List discoveryPublicServiceDescriptor = $convert.base64Decode(
    'ChZEaXNjb3ZlcnlQdWJsaWNTZXJ2aWNlEpkBChlTdWdnZXN0UmVjb21tZW5kZWRTZWxsZXJzEj'
    'ouaGF3a3IuZGlzY292ZXJ5LnYxYWxwaGExLlN1Z2dlc3RSZWNvbW1lbmRlZFNlbGxlcnNSZXF1'
    'ZXN0GjsuaGF3a3IuZGlzY292ZXJ5LnYxYWxwaGExLlN1Z2dlc3RSZWNvbW1lbmRlZFNlbGxlcn'
    'NSZXNwb25zZSIDkAIB');
