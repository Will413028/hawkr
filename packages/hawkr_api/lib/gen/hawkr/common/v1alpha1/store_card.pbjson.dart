// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/store_card.proto.

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

@$core.Deprecated('Use storeCardStatusDescriptor instead')
const StoreCardStatus$json = {
  '1': 'StoreCardStatus',
  '2': [
    {'1': 'STORE_CARD_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'STORE_CARD_STATUS_OPEN', '2': 1},
    {'1': 'STORE_CARD_STATUS_PAUSED', '2': 2},
    {'1': 'STORE_CARD_STATUS_CLOSED', '2': 3},
  ],
};

/// Descriptor for `StoreCardStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List storeCardStatusDescriptor = $convert.base64Decode(
    'Cg9TdG9yZUNhcmRTdGF0dXMSIQodU1RPUkVfQ0FSRF9TVEFUVVNfVU5TUEVDSUZJRUQQABIaCh'
    'ZTVE9SRV9DQVJEX1NUQVRVU19PUEVOEAESHAoYU1RPUkVfQ0FSRF9TVEFUVVNfUEFVU0VEEAIS'
    'HAoYU1RPUkVfQ0FSRF9TVEFUVVNfQ0xPU0VEEAM=');

@$core.Deprecated('Use storeCardDescriptor instead')
const StoreCard$json = {
  '1': 'StoreCard',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'store_code', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'storeCode'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'logo',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'logo'
    },
    {
      '1': 'cover_image',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'coverImage'
    },
    {
      '1': 'status',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.StoreCardStatus',
      '10': 'status'
    },
    {'1': 'town_code', '3': 7, '4': 1, '5': 9, '10': 'townCode'},
    {
      '1': 'review_count',
      '3': 8,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'reviewCount',
      '17': true
    },
    {
      '1': 'rating_average',
      '3': 9,
      '4': 1,
      '5': 2,
      '9': 2,
      '10': 'ratingAverage',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_review_count'},
    {'1': '_rating_average'},
  ],
};

/// Descriptor for `StoreCard`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storeCardDescriptor = $convert.base64Decode(
    'CglTdG9yZUNhcmQSEwoCaWQYASABKAlCA+BBCFICaWQSIgoKc3RvcmVfY29kZRgCIAEoCUID4E'
    'EDUglzdG9yZUNvZGUSFwoEbmFtZRgDIAEoCUgAUgRuYW1liAEBEjAKBGxvZ28YBCABKAsyHC5o'
    'YXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VSBGxvZ28SPQoLY292ZXJfaW1hZ2UYBSABKAsyHC'
    '5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VSCmNvdmVySW1hZ2USPgoGc3RhdHVzGAYgASgO'
    'MiYuaGF3a3IuY29tbW9uLnYxYWxwaGExLlN0b3JlQ2FyZFN0YXR1c1IGc3RhdHVzEhsKCXRvd2'
    '5fY29kZRgHIAEoCVIIdG93bkNvZGUSJgoMcmV2aWV3X2NvdW50GAggASgFSAFSC3Jldmlld0Nv'
    'dW50iAEBEioKDnJhdGluZ19hdmVyYWdlGAkgASgCSAJSDXJhdGluZ0F2ZXJhZ2WIAQFCBwoFX2'
    '5hbWVCDwoNX3Jldmlld19jb3VudEIRCg9fcmF0aW5nX2F2ZXJhZ2U=');
