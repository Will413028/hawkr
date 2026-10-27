// This is a generated file - do not edit.
//
// Generated from hawkr/buyer/v1alpha1/buyer.proto.

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
    as $5;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $2;

import '../../../google/type/date.pbjson.dart' as $0;
import '../../common/v1alpha1/image.pbjson.dart' as $1;
import '../../common/v1alpha1/postal_address.pbjson.dart' as $3;
import '../../common/v1alpha1/store_card.pbjson.dart' as $4;

@$core.Deprecated('Use buyerStatusDescriptor instead')
const BuyerStatus$json = {
  '1': 'BuyerStatus',
  '2': [
    {'1': 'BUYER_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'BUYER_STATUS_ACTIVE', '2': 1},
    {'1': 'BUYER_STATUS_PENDING_DELETION', '2': 2},
  ],
};

/// Descriptor for `BuyerStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List buyerStatusDescriptor = $convert.base64Decode(
    'CgtCdXllclN0YXR1cxIcChhCVVlFUl9TVEFUVVNfVU5TUEVDSUZJRUQQABIXChNCVVlFUl9TVE'
    'FUVVNfQUNUSVZFEAESIQodQlVZRVJfU1RBVFVTX1BFTkRJTkdfREVMRVRJT04QAg==');

@$core.Deprecated('Use genderDescriptor instead')
const Gender$json = {
  '1': 'Gender',
  '2': [
    {'1': 'GENDER_UNSPECIFIED', '2': 0},
    {'1': 'GENDER_MALE', '2': 1},
    {'1': 'GENDER_FEMALE', '2': 2},
    {'1': 'GENDER_OTHER', '2': 3},
  ],
};

/// Descriptor for `Gender`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List genderDescriptor = $convert.base64Decode(
    'CgZHZW5kZXISFgoSR0VOREVSX1VOU1BFQ0lGSUVEEAASDwoLR0VOREVSX01BTEUQARIRCg1HRU'
    '5ERVJfRkVNQUxFEAISEAoMR0VOREVSX09USEVSEAM=');

@$core.Deprecated('Use profileDescriptor instead')
const Profile$json = {
  '1': 'Profile',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'display_name', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'displayName'},
    {'1': 'phone', '3': 3, '4': 1, '5': 9, '10': 'phone'},
    {
      '1': 'gender',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.buyer.v1alpha1.Gender',
      '9': 0,
      '10': 'gender',
      '17': true
    },
    {
      '1': 'birth_date',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.type.Date',
      '10': 'birthDate'
    },
    {'1': 'city', '3': 6, '4': 1, '5': 9, '10': 'city'},
    {'1': 'job', '3': 7, '4': 1, '5': 9, '10': 'job'},
    {
      '1': 'avatar',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'avatar'
    },
    {
      '1': 'status',
      '3': 9,
      '4': 1,
      '5': 14,
      '6': '.hawkr.buyer.v1alpha1.BuyerStatus',
      '8': {},
      '10': 'status'
    },
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
      '1': 'update_time',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {'1': 'etag', '3': 12, '4': 1, '5': 9, '10': 'etag'},
    {
      '1': 'avatar_staged_token',
      '3': 13,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'avatarStagedToken'
    },
  ],
  '8': [
    {'1': '_gender'},
  ],
};

/// Descriptor for `Profile`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileDescriptor = $convert.base64Decode(
    'CgdQcm9maWxlEhMKAmlkGAEgASgJQgPgQQhSAmlkEiYKDGRpc3BsYXlfbmFtZRgCIAEoCUID4E'
    'ECUgtkaXNwbGF5TmFtZRIUCgVwaG9uZRgDIAEoCVIFcGhvbmUSOQoGZ2VuZGVyGAQgASgOMhwu'
    'aGF3a3IuYnV5ZXIudjFhbHBoYTEuR2VuZGVySABSBmdlbmRlcogBARIwCgpiaXJ0aF9kYXRlGA'
    'UgASgLMhEuZ29vZ2xlLnR5cGUuRGF0ZVIJYmlydGhEYXRlEhIKBGNpdHkYBiABKAlSBGNpdHkS'
    'EAoDam9iGAcgASgJUgNqb2ISOQoGYXZhdGFyGAggASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaG'
    'ExLkltYWdlQgPgQQNSBmF2YXRhchI+CgZzdGF0dXMYCSABKA4yIS5oYXdrci5idXllci52MWFs'
    'cGhhMS5CdXllclN0YXR1c0ID4EEDUgZzdGF0dXMSQAoLY3JlYXRlX3RpbWUYCiABKAsyGi5nb2'
    '9nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCmNyZWF0ZVRpbWUSQAoLdXBkYXRlX3RpbWUY'
    'CyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCnVwZGF0ZVRpbWUSEgoEZX'
    'RhZxgMIAEoCVIEZXRhZxIzChNhdmF0YXJfc3RhZ2VkX3Rva2VuGA0gASgJQgPgQQRSEWF2YXRh'
    'clN0YWdlZFRva2VuQgkKB19nZW5kZXI=');

@$core.Deprecated('Use addressDescriptor instead')
const Address$json = {
  '1': 'Address',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'receiver_name',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'receiverName'
    },
    {
      '1': 'receiver_phone',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'receiverPhone'
    },
    {
      '1': 'postal_address',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.PostalAddress',
      '8': {},
      '10': 'postalAddress'
    },
    {'1': 'default_address', '3': 8, '4': 1, '5': 8, '10': 'defaultAddress'},
    {
      '1': 'create_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'update_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {'1': 'etag', '3': 11, '4': 1, '5': 9, '10': 'etag'},
  ],
  '9': [
    {'1': 2, '2': 3},
    {'1': 5, '2': 6},
    {'1': 6, '2': 7},
    {'1': 7, '2': 8},
  ],
  '10': ['label', 'city', 'district', 'detail_address'],
};

/// Descriptor for `Address`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addressDescriptor = $convert.base64Decode(
    'CgdBZGRyZXNzEhMKAmlkGAEgASgJQgPgQQhSAmlkEigKDXJlY2VpdmVyX25hbWUYAyABKAlCA+'
    'BBAlIMcmVjZWl2ZXJOYW1lEioKDnJlY2VpdmVyX3Bob25lGAQgASgJQgPgQQJSDXJlY2VpdmVy'
    'UGhvbmUSUAoOcG9zdGFsX2FkZHJlc3MYDCABKAsyJC5oYXdrci5jb21tb24udjFhbHBoYTEuUG'
    '9zdGFsQWRkcmVzc0ID4EECUg1wb3N0YWxBZGRyZXNzEicKD2RlZmF1bHRfYWRkcmVzcxgIIAEo'
    'CFIOZGVmYXVsdEFkZHJlc3MSQAoLY3JlYXRlX3RpbWUYCSABKAsyGi5nb29nbGUucHJvdG9idW'
    'YuVGltZXN0YW1wQgPgQQNSCmNyZWF0ZVRpbWUSQAoLdXBkYXRlX3RpbWUYCiABKAsyGi5nb29n'
    'bGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCnVwZGF0ZVRpbWUSEgoEZXRhZxgLIAEoCVIEZX'
    'RhZ0oECAIQA0oECAUQBkoECAYQB0oECAcQCFIFbGFiZWxSBGNpdHlSCGRpc3RyaWN0Ug5kZXRh'
    'aWxfYWRkcmVzcw==');

@$core.Deprecated('Use productCardDescriptor instead')
const ProductCard$json = {
  '1': 'ProductCard',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'deleted', '3': 2, '4': 1, '5': 8, '10': 'deleted'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {
      '1': 'image',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'image'
    },
    {'1': 'min_price', '3': 5, '4': 1, '5': 3, '10': 'minPrice'},
    {'1': 'max_price', '3': 6, '4': 1, '5': 3, '10': 'maxPrice'},
    {'1': 'rating', '3': 7, '4': 1, '5': 2, '9': 0, '10': 'rating', '17': true},
    {'1': 'available', '3': 8, '4': 1, '5': 8, '10': 'available'},
    {
      '1': 'restock_subscribed',
      '3': 9,
      '4': 1,
      '5': 8,
      '10': 'restockSubscribed'
    },
    {'1': 'store_id', '3': 11, '4': 1, '5': 9, '10': 'storeId'},
    {
      '1': 'store',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'store'
    },
  ],
  '8': [
    {'1': '_rating'},
  ],
};

/// Descriptor for `ProductCard`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List productCardDescriptor = $convert.base64Decode(
    'CgtQcm9kdWN0Q2FyZBIOCgJpZBgBIAEoCVICaWQSGAoHZGVsZXRlZBgCIAEoCFIHZGVsZXRlZB'
    'ISCgRuYW1lGAMgASgJUgRuYW1lEjIKBWltYWdlGAQgASgLMhwuaGF3a3IuY29tbW9uLnYxYWxw'
    'aGExLkltYWdlUgVpbWFnZRIbCgltaW5fcHJpY2UYBSABKANSCG1pblByaWNlEhsKCW1heF9wcm'
    'ljZRgGIAEoA1IIbWF4UHJpY2USGwoGcmF0aW5nGAcgASgCSABSBnJhdGluZ4gBARIcCglhdmFp'
    'bGFibGUYCCABKAhSCWF2YWlsYWJsZRItChJyZXN0b2NrX3N1YnNjcmliZWQYCSABKAhSEXJlc3'
    'RvY2tTdWJzY3JpYmVkEhkKCHN0b3JlX2lkGAsgASgJUgdzdG9yZUlkEjYKBXN0b3JlGAogASgL'
    'MiAuaGF3a3IuY29tbW9uLnYxYWxwaGExLlN0b3JlQ2FyZFIFc3RvcmVCCQoHX3JhdGluZw==');

@$core.Deprecated('Use favoriteDescriptor instead')
const Favorite$json = {
  '1': 'Favorite',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.ProductCard',
      '10': 'product'
    },
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

/// Descriptor for `Favorite`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List favoriteDescriptor = $convert.base64Decode(
    'CghGYXZvcml0ZRI7Cgdwcm9kdWN0GAEgASgLMiEuaGF3a3IuYnV5ZXIudjFhbHBoYTEuUHJvZH'
    'VjdENhcmRSB3Byb2R1Y3QSOwoLY3JlYXRlX3RpbWUYAiABKAsyGi5nb29nbGUucHJvdG9idWYu'
    'VGltZXN0YW1wUgpjcmVhdGVUaW1l');

@$core.Deprecated('Use followDescriptor instead')
const Follow$json = {
  '1': 'Follow',
  '2': [
    {'1': 'store_id', '3': 3, '4': 1, '5': 9, '10': 'storeId'},
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'store'
    },
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

/// Descriptor for `Follow`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List followDescriptor = $convert.base64Decode(
    'CgZGb2xsb3cSGQoIc3RvcmVfaWQYAyABKAlSB3N0b3JlSWQSNgoFc3RvcmUYASABKAsyIC5oYX'
    'drci5jb21tb24udjFhbHBoYTEuU3RvcmVDYXJkUgVzdG9yZRI7CgtjcmVhdGVfdGltZRgCIAEo'
    'CzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCmNyZWF0ZVRpbWU=');

@$core.Deprecated('Use visitDescriptor instead')
const Visit$json = {
  '1': 'Visit',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.ProductCard',
      '10': 'product'
    },
    {
      '1': 'visit_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'visitTime'
    },
  ],
};

/// Descriptor for `Visit`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List visitDescriptor = $convert.base64Decode(
    'CgVWaXNpdBI7Cgdwcm9kdWN0GAEgASgLMiEuaGF3a3IuYnV5ZXIudjFhbHBoYTEuUHJvZHVjdE'
    'NhcmRSB3Byb2R1Y3QSOQoKdmlzaXRfdGltZRgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1l'
    'c3RhbXBSCXZpc2l0VGltZQ==');

@$core.Deprecated('Use restockSubscriptionDescriptor instead')
const RestockSubscription$json = {
  '1': 'RestockSubscription',
  '2': [
    {
      '1': 'product',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.ProductCard',
      '10': 'product'
    },
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

/// Descriptor for `RestockSubscription`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List restockSubscriptionDescriptor = $convert.base64Decode(
    'ChNSZXN0b2NrU3Vic2NyaXB0aW9uEjsKB3Byb2R1Y3QYASABKAsyIS5oYXdrci5idXllci52MW'
    'FscGhhMS5Qcm9kdWN0Q2FyZFIHcHJvZHVjdBI7CgtjcmVhdGVfdGltZRgCIAEoCzIaLmdvb2ds'
    'ZS5wcm90b2J1Zi5UaW1lc3RhbXBSCmNyZWF0ZVRpbWU=');

@$core.Deprecated('Use memberCardDescriptor instead')
const MemberCard$json = {
  '1': 'MemberCard',
  '2': [
    {'1': 'store_id', '3': 4, '4': 1, '5': 9, '10': 'storeId'},
    {
      '1': 'store',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '10': 'store'
    },
    {'1': 'total_spent', '3': 2, '4': 1, '5': 3, '10': 'totalSpent'},
    {'1': 'vip', '3': 3, '4': 1, '5': 8, '10': 'vip'},
  ],
};

/// Descriptor for `MemberCard`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberCardDescriptor = $convert.base64Decode(
    'CgpNZW1iZXJDYXJkEhkKCHN0b3JlX2lkGAQgASgJUgdzdG9yZUlkEjYKBXN0b3JlGAEgASgLMi'
    'AuaGF3a3IuY29tbW9uLnYxYWxwaGExLlN0b3JlQ2FyZFIFc3RvcmUSHwoLdG90YWxfc3BlbnQY'
    'AiABKANSCnRvdGFsU3BlbnQSEAoDdmlwGAMgASgIUgN2aXA=');

@$core.Deprecated('Use spendingRecordDescriptor instead')
const SpendingRecord$json = {
  '1': 'SpendingRecord',
  '2': [
    {'1': 'order_number', '3': 1, '4': 1, '5': 9, '10': 'orderNumber'},
    {'1': 'amount', '3': 2, '4': 1, '5': 3, '10': 'amount'},
    {'1': 'refunded_amount', '3': 3, '4': 1, '5': 3, '10': 'refundedAmount'},
    {'1': 'coupon_applied', '3': 4, '4': 1, '5': 8, '10': 'couponApplied'},
    {
      '1': 'order_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'orderTime'
    },
  ],
};

/// Descriptor for `SpendingRecord`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spendingRecordDescriptor = $convert.base64Decode(
    'Cg5TcGVuZGluZ1JlY29yZBIhCgxvcmRlcl9udW1iZXIYASABKAlSC29yZGVyTnVtYmVyEhYKBm'
    'Ftb3VudBgCIAEoA1IGYW1vdW50EicKD3JlZnVuZGVkX2Ftb3VudBgDIAEoA1IOcmVmdW5kZWRB'
    'bW91bnQSJQoOY291cG9uX2FwcGxpZWQYBCABKAhSDWNvdXBvbkFwcGxpZWQSOQoKb3JkZXJfdG'
    'ltZRgFIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCW9yZGVyVGltZQ==');

@$core.Deprecated('Use getProfileRequestDescriptor instead')
const GetProfileRequest$json = {
  '1': 'GetProfileRequest',
};

/// Descriptor for `GetProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfileRequestDescriptor =
    $convert.base64Decode('ChFHZXRQcm9maWxlUmVxdWVzdA==');

@$core.Deprecated('Use getProfileResponseDescriptor instead')
const GetProfileResponse$json = {
  '1': 'GetProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `GetProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfileResponseDescriptor = $convert.base64Decode(
    'ChJHZXRQcm9maWxlUmVzcG9uc2USNwoHcHJvZmlsZRgBIAEoCzIdLmhhd2tyLmJ1eWVyLnYxYW'
    'xwaGExLlByb2ZpbGVSB3Byb2ZpbGU=');

@$core.Deprecated('Use updateProfileRequestDescriptor instead')
const UpdateProfileRequest$json = {
  '1': 'UpdateProfileRequest',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Profile',
      '8': {},
      '10': 'profile'
    },
    {
      '1': 'update_mask',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdateProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateProfileRequestDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVQcm9maWxlUmVxdWVzdBI8Cgdwcm9maWxlGAEgASgLMh0uaGF3a3IuYnV5ZXIudj'
    'FhbHBoYTEuUHJvZmlsZUID4EECUgdwcm9maWxlEjsKC3VwZGF0ZV9tYXNrGAIgASgLMhouZ29v'
    'Z2xlLnByb3RvYnVmLkZpZWxkTWFza1IKdXBkYXRlTWFzaw==');

@$core.Deprecated('Use updateProfileResponseDescriptor instead')
const UpdateProfileResponse$json = {
  '1': 'UpdateProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `UpdateProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateProfileResponseDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVQcm9maWxlUmVzcG9uc2USNwoHcHJvZmlsZRgBIAEoCzIdLmhhd2tyLmJ1eWVyLn'
    'YxYWxwaGExLlByb2ZpbGVSB3Byb2ZpbGU=');

@$core.Deprecated('Use listAddressesRequestDescriptor instead')
const ListAddressesRequest$json = {
  '1': 'ListAddressesRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListAddressesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAddressesRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0QWRkcmVzc2VzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCn'
    'BhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listAddressesResponseDescriptor instead')
const ListAddressesResponse$json = {
  '1': 'ListAddressesResponse',
  '2': [
    {
      '1': 'addresses',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Address',
      '10': 'addresses'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListAddressesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAddressesResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0QWRkcmVzc2VzUmVzcG9uc2USOwoJYWRkcmVzc2VzGAEgAygLMh0uaGF3a3IuYnV5ZX'
    'IudjFhbHBoYTEuQWRkcmVzc1IJYWRkcmVzc2VzEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVIN'
    'bmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use getAddressRequestDescriptor instead')
const GetAddressRequest$json = {
  '1': 'GetAddressRequest',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
  ],
};

/// Descriptor for `GetAddressRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAddressRequestDescriptor = $convert
    .base64Decode('ChFHZXRBZGRyZXNzUmVxdWVzdBITCgJpZBgBIAEoCUID4EECUgJpZA==');

@$core.Deprecated('Use getAddressResponseDescriptor instead')
const GetAddressResponse$json = {
  '1': 'GetAddressResponse',
  '2': [
    {
      '1': 'address',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Address',
      '10': 'address'
    },
  ],
};

/// Descriptor for `GetAddressResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAddressResponseDescriptor = $convert.base64Decode(
    'ChJHZXRBZGRyZXNzUmVzcG9uc2USNwoHYWRkcmVzcxgBIAEoCzIdLmhhd2tyLmJ1eWVyLnYxYW'
    'xwaGExLkFkZHJlc3NSB2FkZHJlc3M=');

@$core.Deprecated('Use createAddressRequestDescriptor instead')
const CreateAddressRequest$json = {
  '1': 'CreateAddressRequest',
  '2': [
    {
      '1': 'address',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Address',
      '8': {},
      '10': 'address'
    },
  ],
};

/// Descriptor for `CreateAddressRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createAddressRequestDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVBZGRyZXNzUmVxdWVzdBI8CgdhZGRyZXNzGAEgASgLMh0uaGF3a3IuYnV5ZXIudj'
    'FhbHBoYTEuQWRkcmVzc0ID4EECUgdhZGRyZXNz');

@$core.Deprecated('Use createAddressResponseDescriptor instead')
const CreateAddressResponse$json = {
  '1': 'CreateAddressResponse',
  '2': [
    {
      '1': 'address',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Address',
      '10': 'address'
    },
  ],
};

/// Descriptor for `CreateAddressResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createAddressResponseDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVBZGRyZXNzUmVzcG9uc2USNwoHYWRkcmVzcxgBIAEoCzIdLmhhd2tyLmJ1eWVyLn'
    'YxYWxwaGExLkFkZHJlc3NSB2FkZHJlc3M=');

@$core.Deprecated('Use updateAddressRequestDescriptor instead')
const UpdateAddressRequest$json = {
  '1': 'UpdateAddressRequest',
  '2': [
    {
      '1': 'address',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Address',
      '8': {},
      '10': 'address'
    },
    {
      '1': 'update_mask',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdateAddressRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateAddressRequestDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVBZGRyZXNzUmVxdWVzdBI8CgdhZGRyZXNzGAEgASgLMh0uaGF3a3IuYnV5ZXIudj'
    'FhbHBoYTEuQWRkcmVzc0ID4EECUgdhZGRyZXNzEjsKC3VwZGF0ZV9tYXNrGAIgASgLMhouZ29v'
    'Z2xlLnByb3RvYnVmLkZpZWxkTWFza1IKdXBkYXRlTWFzaw==');

@$core.Deprecated('Use updateAddressResponseDescriptor instead')
const UpdateAddressResponse$json = {
  '1': 'UpdateAddressResponse',
  '2': [
    {
      '1': 'address',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Address',
      '10': 'address'
    },
  ],
};

/// Descriptor for `UpdateAddressResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateAddressResponseDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVBZGRyZXNzUmVzcG9uc2USNwoHYWRkcmVzcxgBIAEoCzIdLmhhd2tyLmJ1eWVyLn'
    'YxYWxwaGExLkFkZHJlc3NSB2FkZHJlc3M=');

@$core.Deprecated('Use deleteAddressRequestDescriptor instead')
const DeleteAddressRequest$json = {
  '1': 'DeleteAddressRequest',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '10': 'etag'},
  ],
};

/// Descriptor for `DeleteAddressRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAddressRequestDescriptor = $convert.base64Decode(
    'ChREZWxldGVBZGRyZXNzUmVxdWVzdBITCgJpZBgBIAEoCUID4EECUgJpZBISCgRldGFnGAIgAS'
    'gJUgRldGFn');

@$core.Deprecated('Use deleteAddressResponseDescriptor instead')
const DeleteAddressResponse$json = {
  '1': 'DeleteAddressResponse',
};

/// Descriptor for `DeleteAddressResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAddressResponseDescriptor =
    $convert.base64Decode('ChVEZWxldGVBZGRyZXNzUmVzcG9uc2U=');

@$core.Deprecated('Use listFavoritesRequestDescriptor instead')
const ListFavoritesRequest$json = {
  '1': 'ListFavoritesRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListFavoritesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFavoritesRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0RmF2b3JpdGVzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCn'
    'BhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listFavoritesResponseDescriptor instead')
const ListFavoritesResponse$json = {
  '1': 'ListFavoritesResponse',
  '2': [
    {
      '1': 'favorites',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Favorite',
      '10': 'favorites'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
    {'1': 'total_size', '3': 3, '4': 1, '5': 5, '10': 'totalSize'},
  ],
};

/// Descriptor for `ListFavoritesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFavoritesResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0RmF2b3JpdGVzUmVzcG9uc2USPAoJZmF2b3JpdGVzGAEgAygLMh4uaGF3a3IuYnV5ZX'
    'IudjFhbHBoYTEuRmF2b3JpdGVSCWZhdm9yaXRlcxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlS'
    'DW5leHRQYWdlVG9rZW4SHQoKdG90YWxfc2l6ZRgDIAEoBVIJdG90YWxTaXpl');

@$core.Deprecated('Use checkFavoriteRequestDescriptor instead')
const CheckFavoriteRequest$json = {
  '1': 'CheckFavoriteRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `CheckFavoriteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkFavoriteRequestDescriptor = $convert.base64Decode(
    'ChRDaGVja0Zhdm9yaXRlUmVxdWVzdBIiCgpwcm9kdWN0X2lkGAEgASgJQgPgQQJSCXByb2R1Y3'
    'RJZA==');

@$core.Deprecated('Use checkFavoriteResponseDescriptor instead')
const CheckFavoriteResponse$json = {
  '1': 'CheckFavoriteResponse',
  '2': [
    {'1': 'favorited', '3': 1, '4': 1, '5': 8, '10': 'favorited'},
  ],
};

/// Descriptor for `CheckFavoriteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkFavoriteResponseDescriptor = $convert.base64Decode(
    'ChVDaGVja0Zhdm9yaXRlUmVzcG9uc2USHAoJZmF2b3JpdGVkGAEgASgIUglmYXZvcml0ZWQ=');

@$core.Deprecated('Use batchCheckFavoritesRequestDescriptor instead')
const BatchCheckFavoritesRequest$json = {
  '1': 'BatchCheckFavoritesRequest',
  '2': [
    {'1': 'product_ids', '3': 1, '4': 3, '5': 9, '10': 'productIds'},
  ],
};

/// Descriptor for `BatchCheckFavoritesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchCheckFavoritesRequestDescriptor =
    $convert.base64Decode(
        'ChpCYXRjaENoZWNrRmF2b3JpdGVzUmVxdWVzdBIfCgtwcm9kdWN0X2lkcxgBIAMoCVIKcHJvZH'
        'VjdElkcw==');

@$core.Deprecated('Use batchCheckFavoritesResponseDescriptor instead')
const BatchCheckFavoritesResponse$json = {
  '1': 'BatchCheckFavoritesResponse',
  '2': [
    {
      '1': 'favorited_product_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '10': 'favoritedProductIds'
    },
  ],
};

/// Descriptor for `BatchCheckFavoritesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchCheckFavoritesResponseDescriptor =
    $convert.base64Decode(
        'ChtCYXRjaENoZWNrRmF2b3JpdGVzUmVzcG9uc2USMgoVZmF2b3JpdGVkX3Byb2R1Y3RfaWRzGA'
        'EgAygJUhNmYXZvcml0ZWRQcm9kdWN0SWRz');

@$core.Deprecated('Use addFavoriteRequestDescriptor instead')
const AddFavoriteRequest$json = {
  '1': 'AddFavoriteRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `AddFavoriteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addFavoriteRequestDescriptor = $convert.base64Decode(
    'ChJBZGRGYXZvcml0ZVJlcXVlc3QSIgoKcHJvZHVjdF9pZBgBIAEoCUID4EECUglwcm9kdWN0SW'
    'Q=');

@$core.Deprecated('Use addFavoriteResponseDescriptor instead')
const AddFavoriteResponse$json = {
  '1': 'AddFavoriteResponse',
};

/// Descriptor for `AddFavoriteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addFavoriteResponseDescriptor =
    $convert.base64Decode('ChNBZGRGYXZvcml0ZVJlc3BvbnNl');

@$core.Deprecated('Use removeFavoriteRequestDescriptor instead')
const RemoveFavoriteRequest$json = {
  '1': 'RemoveFavoriteRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `RemoveFavoriteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeFavoriteRequestDescriptor = $convert.base64Decode(
    'ChVSZW1vdmVGYXZvcml0ZVJlcXVlc3QSIgoKcHJvZHVjdF9pZBgBIAEoCUID4EECUglwcm9kdW'
    'N0SWQ=');

@$core.Deprecated('Use removeFavoriteResponseDescriptor instead')
const RemoveFavoriteResponse$json = {
  '1': 'RemoveFavoriteResponse',
};

/// Descriptor for `RemoveFavoriteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeFavoriteResponseDescriptor =
    $convert.base64Decode('ChZSZW1vdmVGYXZvcml0ZVJlc3BvbnNl');

@$core.Deprecated('Use listFollowsRequestDescriptor instead')
const ListFollowsRequest$json = {
  '1': 'ListFollowsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListFollowsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFollowsRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0Rm9sbG93c1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZRIdCgpwYW'
    'dlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4=');

@$core.Deprecated('Use listFollowsResponseDescriptor instead')
const ListFollowsResponse$json = {
  '1': 'ListFollowsResponse',
  '2': [
    {
      '1': 'follows',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Follow',
      '10': 'follows'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
    {'1': 'total_size', '3': 3, '4': 1, '5': 5, '10': 'totalSize'},
  ],
};

/// Descriptor for `ListFollowsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFollowsResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0Rm9sbG93c1Jlc3BvbnNlEjYKB2ZvbGxvd3MYASADKAsyHC5oYXdrci5idXllci52MW'
    'FscGhhMS5Gb2xsb3dSB2ZvbGxvd3MSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJUg1uZXh0UGFn'
    'ZVRva2VuEh0KCnRvdGFsX3NpemUYAyABKAVSCXRvdGFsU2l6ZQ==');

@$core.Deprecated('Use checkFollowRequestDescriptor instead')
const CheckFollowRequest$json = {
  '1': 'CheckFollowRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
  ],
};

/// Descriptor for `CheckFollowRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkFollowRequestDescriptor = $convert.base64Decode(
    'ChJDaGVja0ZvbGxvd1JlcXVlc3QSHgoIc3RvcmVfaWQYASABKAlCA+BBAlIHc3RvcmVJZA==');

@$core.Deprecated('Use checkFollowResponseDescriptor instead')
const CheckFollowResponse$json = {
  '1': 'CheckFollowResponse',
  '2': [
    {'1': 'following', '3': 1, '4': 1, '5': 8, '10': 'following'},
  ],
};

/// Descriptor for `CheckFollowResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkFollowResponseDescriptor =
    $convert.base64Decode(
        'ChNDaGVja0ZvbGxvd1Jlc3BvbnNlEhwKCWZvbGxvd2luZxgBIAEoCFIJZm9sbG93aW5n');

@$core.Deprecated('Use batchCheckFollowsRequestDescriptor instead')
const BatchCheckFollowsRequest$json = {
  '1': 'BatchCheckFollowsRequest',
  '2': [
    {'1': 'store_ids', '3': 1, '4': 3, '5': 9, '10': 'storeIds'},
  ],
};

/// Descriptor for `BatchCheckFollowsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchCheckFollowsRequestDescriptor =
    $convert.base64Decode(
        'ChhCYXRjaENoZWNrRm9sbG93c1JlcXVlc3QSGwoJc3RvcmVfaWRzGAEgAygJUghzdG9yZUlkcw'
        '==');

@$core.Deprecated('Use batchCheckFollowsResponseDescriptor instead')
const BatchCheckFollowsResponse$json = {
  '1': 'BatchCheckFollowsResponse',
  '2': [
    {
      '1': 'followed_store_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '10': 'followedStoreIds'
    },
  ],
};

/// Descriptor for `BatchCheckFollowsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List batchCheckFollowsResponseDescriptor =
    $convert.base64Decode(
        'ChlCYXRjaENoZWNrRm9sbG93c1Jlc3BvbnNlEiwKEmZvbGxvd2VkX3N0b3JlX2lkcxgBIAMoCV'
        'IQZm9sbG93ZWRTdG9yZUlkcw==');

@$core.Deprecated('Use followStoreRequestDescriptor instead')
const FollowStoreRequest$json = {
  '1': 'FollowStoreRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
  ],
};

/// Descriptor for `FollowStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List followStoreRequestDescriptor = $convert.base64Decode(
    'ChJGb2xsb3dTdG9yZVJlcXVlc3QSHgoIc3RvcmVfaWQYASABKAlCA+BBAlIHc3RvcmVJZA==');

@$core.Deprecated('Use followStoreResponseDescriptor instead')
const FollowStoreResponse$json = {
  '1': 'FollowStoreResponse',
};

/// Descriptor for `FollowStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List followStoreResponseDescriptor =
    $convert.base64Decode('ChNGb2xsb3dTdG9yZVJlc3BvbnNl');

@$core.Deprecated('Use unfollowStoreRequestDescriptor instead')
const UnfollowStoreRequest$json = {
  '1': 'UnfollowStoreRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
  ],
};

/// Descriptor for `UnfollowStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unfollowStoreRequestDescriptor = $convert.base64Decode(
    'ChRVbmZvbGxvd1N0b3JlUmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG9yZUlk');

@$core.Deprecated('Use unfollowStoreResponseDescriptor instead')
const UnfollowStoreResponse$json = {
  '1': 'UnfollowStoreResponse',
};

/// Descriptor for `UnfollowStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unfollowStoreResponseDescriptor =
    $convert.base64Decode('ChVVbmZvbGxvd1N0b3JlUmVzcG9uc2U=');

@$core.Deprecated('Use listMemberCardsRequestDescriptor instead')
const ListMemberCardsRequest$json = {
  '1': 'ListMemberCardsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListMemberCardsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMemberCardsRequestDescriptor =
    $convert.base64Decode(
        'ChZMaXN0TWVtYmVyQ2FyZHNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQ'
        'oKcGFnZV90b2tlbhgCIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listMemberCardsResponseDescriptor instead')
const ListMemberCardsResponse$json = {
  '1': 'ListMemberCardsResponse',
  '2': [
    {
      '1': 'member_cards',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.MemberCard',
      '10': 'memberCards'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMemberCardsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMemberCardsResponseDescriptor = $convert.base64Decode(
    'ChdMaXN0TWVtYmVyQ2FyZHNSZXNwb25zZRJDCgxtZW1iZXJfY2FyZHMYASADKAsyIC5oYXdrci'
    '5idXllci52MWFscGhhMS5NZW1iZXJDYXJkUgttZW1iZXJDYXJkcxImCg9uZXh0X3BhZ2VfdG9r'
    'ZW4YAiABKAlSDW5leHRQYWdlVG9rZW4=');

@$core.Deprecated('Use listSpendingRecordsRequestDescriptor instead')
const ListSpendingRecordsRequest$json = {
  '1': 'ListSpendingRecordsRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListSpendingRecordsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSpendingRecordsRequestDescriptor =
    $convert.base64Decode(
        'ChpMaXN0U3BlbmRpbmdSZWNvcmRzUmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG'
        '9yZUlkEhsKCXBhZ2Vfc2l6ZRgCIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgDIAEoCVIJ'
        'cGFnZVRva2Vu');

@$core.Deprecated('Use listSpendingRecordsResponseDescriptor instead')
const ListSpendingRecordsResponse$json = {
  '1': 'ListSpendingRecordsResponse',
  '2': [
    {
      '1': 'records',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.SpendingRecord',
      '10': 'records'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
    {'1': 'total_spent', '3': 3, '4': 1, '5': 3, '10': 'totalSpent'},
    {'1': 'total_refunded', '3': 4, '4': 1, '5': 3, '10': 'totalRefunded'},
  ],
};

/// Descriptor for `ListSpendingRecordsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSpendingRecordsResponseDescriptor = $convert.base64Decode(
    'ChtMaXN0U3BlbmRpbmdSZWNvcmRzUmVzcG9uc2USPgoHcmVjb3JkcxgBIAMoCzIkLmhhd2tyLm'
    'J1eWVyLnYxYWxwaGExLlNwZW5kaW5nUmVjb3JkUgdyZWNvcmRzEiYKD25leHRfcGFnZV90b2tl'
    'bhgCIAEoCVINbmV4dFBhZ2VUb2tlbhIfCgt0b3RhbF9zcGVudBgDIAEoA1IKdG90YWxTcGVudB'
    'IlCg50b3RhbF9yZWZ1bmRlZBgEIAEoA1INdG90YWxSZWZ1bmRlZA==');

@$core.Deprecated('Use listVisitsRequestDescriptor instead')
const ListVisitsRequest$json = {
  '1': 'ListVisitsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListVisitsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listVisitsRequestDescriptor = $convert.base64Decode(
    'ChFMaXN0VmlzaXRzUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCnBhZ2'
    'VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listVisitsResponseDescriptor instead')
const ListVisitsResponse$json = {
  '1': 'ListVisitsResponse',
  '2': [
    {
      '1': 'visits',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.Visit',
      '10': 'visits'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListVisitsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listVisitsResponseDescriptor = $convert.base64Decode(
    'ChJMaXN0VmlzaXRzUmVzcG9uc2USMwoGdmlzaXRzGAEgAygLMhsuaGF3a3IuYnV5ZXIudjFhbH'
    'BoYTEuVmlzaXRSBnZpc2l0cxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlSDW5leHRQYWdlVG9r'
    'ZW4=');

@$core.Deprecated('Use recordVisitRequestDescriptor instead')
const RecordVisitRequest$json = {
  '1': 'RecordVisitRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `RecordVisitRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordVisitRequestDescriptor = $convert.base64Decode(
    'ChJSZWNvcmRWaXNpdFJlcXVlc3QSIgoKcHJvZHVjdF9pZBgBIAEoCUID4EECUglwcm9kdWN0SW'
    'Q=');

@$core.Deprecated('Use recordVisitResponseDescriptor instead')
const RecordVisitResponse$json = {
  '1': 'RecordVisitResponse',
};

/// Descriptor for `RecordVisitResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordVisitResponseDescriptor =
    $convert.base64Decode('ChNSZWNvcmRWaXNpdFJlc3BvbnNl');

@$core.Deprecated('Use clearVisitsRequestDescriptor instead')
const ClearVisitsRequest$json = {
  '1': 'ClearVisitsRequest',
};

/// Descriptor for `ClearVisitsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearVisitsRequestDescriptor =
    $convert.base64Decode('ChJDbGVhclZpc2l0c1JlcXVlc3Q=');

@$core.Deprecated('Use clearVisitsResponseDescriptor instead')
const ClearVisitsResponse$json = {
  '1': 'ClearVisitsResponse',
};

/// Descriptor for `ClearVisitsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearVisitsResponseDescriptor =
    $convert.base64Decode('ChNDbGVhclZpc2l0c1Jlc3BvbnNl');

@$core.Deprecated('Use listRestockSubscriptionsRequestDescriptor instead')
const ListRestockSubscriptionsRequest$json = {
  '1': 'ListRestockSubscriptionsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListRestockSubscriptionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRestockSubscriptionsRequestDescriptor =
    $convert.base64Decode(
        'Ch9MaXN0UmVzdG9ja1N1YnNjcmlwdGlvbnNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcG'
        'FnZVNpemUSHQoKcGFnZV90b2tlbhgCIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listRestockSubscriptionsResponseDescriptor instead')
const ListRestockSubscriptionsResponse$json = {
  '1': 'ListRestockSubscriptionsResponse',
  '2': [
    {
      '1': 'restock_subscriptions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.buyer.v1alpha1.RestockSubscription',
      '10': 'restockSubscriptions'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListRestockSubscriptionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRestockSubscriptionsResponseDescriptor =
    $convert.base64Decode(
        'CiBMaXN0UmVzdG9ja1N1YnNjcmlwdGlvbnNSZXNwb25zZRJeChVyZXN0b2NrX3N1YnNjcmlwdG'
        'lvbnMYASADKAsyKS5oYXdrci5idXllci52MWFscGhhMS5SZXN0b2NrU3Vic2NyaXB0aW9uUhRy'
        'ZXN0b2NrU3Vic2NyaXB0aW9ucxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlSDW5leHRQYWdlVG'
        '9rZW4=');

@$core.Deprecated('Use subscribeRestockRequestDescriptor instead')
const SubscribeRestockRequest$json = {
  '1': 'SubscribeRestockRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `SubscribeRestockRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscribeRestockRequestDescriptor =
    $convert.base64Decode(
        'ChdTdWJzY3JpYmVSZXN0b2NrUmVxdWVzdBIiCgpwcm9kdWN0X2lkGAEgASgJQgPgQQJSCXByb2'
        'R1Y3RJZA==');

@$core.Deprecated('Use subscribeRestockResponseDescriptor instead')
const SubscribeRestockResponse$json = {
  '1': 'SubscribeRestockResponse',
};

/// Descriptor for `SubscribeRestockResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscribeRestockResponseDescriptor =
    $convert.base64Decode('ChhTdWJzY3JpYmVSZXN0b2NrUmVzcG9uc2U=');

@$core.Deprecated('Use unsubscribeRestockRequestDescriptor instead')
const UnsubscribeRestockRequest$json = {
  '1': 'UnsubscribeRestockRequest',
  '2': [
    {'1': 'product_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'productId'},
  ],
};

/// Descriptor for `UnsubscribeRestockRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unsubscribeRestockRequestDescriptor =
    $convert.base64Decode(
        'ChlVbnN1YnNjcmliZVJlc3RvY2tSZXF1ZXN0EiIKCnByb2R1Y3RfaWQYASABKAlCA+BBAlIJcH'
        'JvZHVjdElk');

@$core.Deprecated('Use unsubscribeRestockResponseDescriptor instead')
const UnsubscribeRestockResponse$json = {
  '1': 'UnsubscribeRestockResponse',
};

/// Descriptor for `UnsubscribeRestockResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unsubscribeRestockResponseDescriptor =
    $convert.base64Decode('ChpVbnN1YnNjcmliZVJlc3RvY2tSZXNwb25zZQ==');

const $core.Map<$core.String, $core.dynamic> BuyerAppServiceBase$json = {
  '1': 'BuyerAppService',
  '2': [
    {
      '1': 'GetProfile',
      '2': '.hawkr.buyer.v1alpha1.GetProfileRequest',
      '3': '.hawkr.buyer.v1alpha1.GetProfileResponse',
      '4': {'34': 1},
    },
    {
      '1': 'UpdateProfile',
      '2': '.hawkr.buyer.v1alpha1.UpdateProfileRequest',
      '3': '.hawkr.buyer.v1alpha1.UpdateProfileResponse'
    },
    {
      '1': 'ListAddresses',
      '2': '.hawkr.buyer.v1alpha1.ListAddressesRequest',
      '3': '.hawkr.buyer.v1alpha1.ListAddressesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetAddress',
      '2': '.hawkr.buyer.v1alpha1.GetAddressRequest',
      '3': '.hawkr.buyer.v1alpha1.GetAddressResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CreateAddress',
      '2': '.hawkr.buyer.v1alpha1.CreateAddressRequest',
      '3': '.hawkr.buyer.v1alpha1.CreateAddressResponse'
    },
    {
      '1': 'UpdateAddress',
      '2': '.hawkr.buyer.v1alpha1.UpdateAddressRequest',
      '3': '.hawkr.buyer.v1alpha1.UpdateAddressResponse'
    },
    {
      '1': 'DeleteAddress',
      '2': '.hawkr.buyer.v1alpha1.DeleteAddressRequest',
      '3': '.hawkr.buyer.v1alpha1.DeleteAddressResponse'
    },
    {
      '1': 'ListFavorites',
      '2': '.hawkr.buyer.v1alpha1.ListFavoritesRequest',
      '3': '.hawkr.buyer.v1alpha1.ListFavoritesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CheckFavorite',
      '2': '.hawkr.buyer.v1alpha1.CheckFavoriteRequest',
      '3': '.hawkr.buyer.v1alpha1.CheckFavoriteResponse',
      '4': {'34': 1},
    },
    {
      '1': 'BatchCheckFavorites',
      '2': '.hawkr.buyer.v1alpha1.BatchCheckFavoritesRequest',
      '3': '.hawkr.buyer.v1alpha1.BatchCheckFavoritesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'AddFavorite',
      '2': '.hawkr.buyer.v1alpha1.AddFavoriteRequest',
      '3': '.hawkr.buyer.v1alpha1.AddFavoriteResponse',
      '4': {'34': 2},
    },
    {
      '1': 'RemoveFavorite',
      '2': '.hawkr.buyer.v1alpha1.RemoveFavoriteRequest',
      '3': '.hawkr.buyer.v1alpha1.RemoveFavoriteResponse',
      '4': {'34': 2},
    },
    {
      '1': 'ListFollows',
      '2': '.hawkr.buyer.v1alpha1.ListFollowsRequest',
      '3': '.hawkr.buyer.v1alpha1.ListFollowsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CheckFollow',
      '2': '.hawkr.buyer.v1alpha1.CheckFollowRequest',
      '3': '.hawkr.buyer.v1alpha1.CheckFollowResponse',
      '4': {'34': 1},
    },
    {
      '1': 'BatchCheckFollows',
      '2': '.hawkr.buyer.v1alpha1.BatchCheckFollowsRequest',
      '3': '.hawkr.buyer.v1alpha1.BatchCheckFollowsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'FollowStore',
      '2': '.hawkr.buyer.v1alpha1.FollowStoreRequest',
      '3': '.hawkr.buyer.v1alpha1.FollowStoreResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnfollowStore',
      '2': '.hawkr.buyer.v1alpha1.UnfollowStoreRequest',
      '3': '.hawkr.buyer.v1alpha1.UnfollowStoreResponse',
      '4': {'34': 2},
    },
    {
      '1': 'ListMemberCards',
      '2': '.hawkr.buyer.v1alpha1.ListMemberCardsRequest',
      '3': '.hawkr.buyer.v1alpha1.ListMemberCardsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListSpendingRecords',
      '2': '.hawkr.buyer.v1alpha1.ListSpendingRecordsRequest',
      '3': '.hawkr.buyer.v1alpha1.ListSpendingRecordsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListVisits',
      '2': '.hawkr.buyer.v1alpha1.ListVisitsRequest',
      '3': '.hawkr.buyer.v1alpha1.ListVisitsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'RecordVisit',
      '2': '.hawkr.buyer.v1alpha1.RecordVisitRequest',
      '3': '.hawkr.buyer.v1alpha1.RecordVisitResponse'
    },
    {
      '1': 'ClearVisits',
      '2': '.hawkr.buyer.v1alpha1.ClearVisitsRequest',
      '3': '.hawkr.buyer.v1alpha1.ClearVisitsResponse',
      '4': {'34': 2},
    },
    {
      '1': 'ListRestockSubscriptions',
      '2': '.hawkr.buyer.v1alpha1.ListRestockSubscriptionsRequest',
      '3': '.hawkr.buyer.v1alpha1.ListRestockSubscriptionsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SubscribeRestock',
      '2': '.hawkr.buyer.v1alpha1.SubscribeRestockRequest',
      '3': '.hawkr.buyer.v1alpha1.SubscribeRestockResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnsubscribeRestock',
      '2': '.hawkr.buyer.v1alpha1.UnsubscribeRestockRequest',
      '3': '.hawkr.buyer.v1alpha1.UnsubscribeRestockResponse',
      '4': {'34': 2},
    },
  ],
};

@$core.Deprecated('Use buyerAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    BuyerAppServiceBase$messageJson = {
  '.hawkr.buyer.v1alpha1.GetProfileRequest': GetProfileRequest$json,
  '.hawkr.buyer.v1alpha1.GetProfileResponse': GetProfileResponse$json,
  '.hawkr.buyer.v1alpha1.Profile': Profile$json,
  '.google.type.Date': $0.Date$json,
  '.hawkr.common.v1alpha1.Image': $1.Image$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.buyer.v1alpha1.UpdateProfileRequest': UpdateProfileRequest$json,
  '.google.protobuf.FieldMask': $5.FieldMask$json,
  '.hawkr.buyer.v1alpha1.UpdateProfileResponse': UpdateProfileResponse$json,
  '.hawkr.buyer.v1alpha1.ListAddressesRequest': ListAddressesRequest$json,
  '.hawkr.buyer.v1alpha1.ListAddressesResponse': ListAddressesResponse$json,
  '.hawkr.buyer.v1alpha1.Address': Address$json,
  '.hawkr.common.v1alpha1.PostalAddress': $3.PostalAddress$json,
  '.hawkr.buyer.v1alpha1.GetAddressRequest': GetAddressRequest$json,
  '.hawkr.buyer.v1alpha1.GetAddressResponse': GetAddressResponse$json,
  '.hawkr.buyer.v1alpha1.CreateAddressRequest': CreateAddressRequest$json,
  '.hawkr.buyer.v1alpha1.CreateAddressResponse': CreateAddressResponse$json,
  '.hawkr.buyer.v1alpha1.UpdateAddressRequest': UpdateAddressRequest$json,
  '.hawkr.buyer.v1alpha1.UpdateAddressResponse': UpdateAddressResponse$json,
  '.hawkr.buyer.v1alpha1.DeleteAddressRequest': DeleteAddressRequest$json,
  '.hawkr.buyer.v1alpha1.DeleteAddressResponse': DeleteAddressResponse$json,
  '.hawkr.buyer.v1alpha1.ListFavoritesRequest': ListFavoritesRequest$json,
  '.hawkr.buyer.v1alpha1.ListFavoritesResponse': ListFavoritesResponse$json,
  '.hawkr.buyer.v1alpha1.Favorite': Favorite$json,
  '.hawkr.buyer.v1alpha1.ProductCard': ProductCard$json,
  '.hawkr.common.v1alpha1.StoreCard': $4.StoreCard$json,
  '.hawkr.buyer.v1alpha1.CheckFavoriteRequest': CheckFavoriteRequest$json,
  '.hawkr.buyer.v1alpha1.CheckFavoriteResponse': CheckFavoriteResponse$json,
  '.hawkr.buyer.v1alpha1.BatchCheckFavoritesRequest':
      BatchCheckFavoritesRequest$json,
  '.hawkr.buyer.v1alpha1.BatchCheckFavoritesResponse':
      BatchCheckFavoritesResponse$json,
  '.hawkr.buyer.v1alpha1.AddFavoriteRequest': AddFavoriteRequest$json,
  '.hawkr.buyer.v1alpha1.AddFavoriteResponse': AddFavoriteResponse$json,
  '.hawkr.buyer.v1alpha1.RemoveFavoriteRequest': RemoveFavoriteRequest$json,
  '.hawkr.buyer.v1alpha1.RemoveFavoriteResponse': RemoveFavoriteResponse$json,
  '.hawkr.buyer.v1alpha1.ListFollowsRequest': ListFollowsRequest$json,
  '.hawkr.buyer.v1alpha1.ListFollowsResponse': ListFollowsResponse$json,
  '.hawkr.buyer.v1alpha1.Follow': Follow$json,
  '.hawkr.buyer.v1alpha1.CheckFollowRequest': CheckFollowRequest$json,
  '.hawkr.buyer.v1alpha1.CheckFollowResponse': CheckFollowResponse$json,
  '.hawkr.buyer.v1alpha1.BatchCheckFollowsRequest':
      BatchCheckFollowsRequest$json,
  '.hawkr.buyer.v1alpha1.BatchCheckFollowsResponse':
      BatchCheckFollowsResponse$json,
  '.hawkr.buyer.v1alpha1.FollowStoreRequest': FollowStoreRequest$json,
  '.hawkr.buyer.v1alpha1.FollowStoreResponse': FollowStoreResponse$json,
  '.hawkr.buyer.v1alpha1.UnfollowStoreRequest': UnfollowStoreRequest$json,
  '.hawkr.buyer.v1alpha1.UnfollowStoreResponse': UnfollowStoreResponse$json,
  '.hawkr.buyer.v1alpha1.ListMemberCardsRequest': ListMemberCardsRequest$json,
  '.hawkr.buyer.v1alpha1.ListMemberCardsResponse': ListMemberCardsResponse$json,
  '.hawkr.buyer.v1alpha1.MemberCard': MemberCard$json,
  '.hawkr.buyer.v1alpha1.ListSpendingRecordsRequest':
      ListSpendingRecordsRequest$json,
  '.hawkr.buyer.v1alpha1.ListSpendingRecordsResponse':
      ListSpendingRecordsResponse$json,
  '.hawkr.buyer.v1alpha1.SpendingRecord': SpendingRecord$json,
  '.hawkr.buyer.v1alpha1.ListVisitsRequest': ListVisitsRequest$json,
  '.hawkr.buyer.v1alpha1.ListVisitsResponse': ListVisitsResponse$json,
  '.hawkr.buyer.v1alpha1.Visit': Visit$json,
  '.hawkr.buyer.v1alpha1.RecordVisitRequest': RecordVisitRequest$json,
  '.hawkr.buyer.v1alpha1.RecordVisitResponse': RecordVisitResponse$json,
  '.hawkr.buyer.v1alpha1.ClearVisitsRequest': ClearVisitsRequest$json,
  '.hawkr.buyer.v1alpha1.ClearVisitsResponse': ClearVisitsResponse$json,
  '.hawkr.buyer.v1alpha1.ListRestockSubscriptionsRequest':
      ListRestockSubscriptionsRequest$json,
  '.hawkr.buyer.v1alpha1.ListRestockSubscriptionsResponse':
      ListRestockSubscriptionsResponse$json,
  '.hawkr.buyer.v1alpha1.RestockSubscription': RestockSubscription$json,
  '.hawkr.buyer.v1alpha1.SubscribeRestockRequest': SubscribeRestockRequest$json,
  '.hawkr.buyer.v1alpha1.SubscribeRestockResponse':
      SubscribeRestockResponse$json,
  '.hawkr.buyer.v1alpha1.UnsubscribeRestockRequest':
      UnsubscribeRestockRequest$json,
  '.hawkr.buyer.v1alpha1.UnsubscribeRestockResponse':
      UnsubscribeRestockResponse$json,
};

/// Descriptor for `BuyerAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List buyerAppServiceDescriptor = $convert.base64Decode(
    'Cg9CdXllckFwcFNlcnZpY2USZAoKR2V0UHJvZmlsZRInLmhhd2tyLmJ1eWVyLnYxYWxwaGExLk'
    'dldFByb2ZpbGVSZXF1ZXN0GiguaGF3a3IuYnV5ZXIudjFhbHBoYTEuR2V0UHJvZmlsZVJlc3Bv'
    'bnNlIgOQAgESaAoNVXBkYXRlUHJvZmlsZRIqLmhhd2tyLmJ1eWVyLnYxYWxwaGExLlVwZGF0ZV'
    'Byb2ZpbGVSZXF1ZXN0GisuaGF3a3IuYnV5ZXIudjFhbHBoYTEuVXBkYXRlUHJvZmlsZVJlc3Bv'
    'bnNlEm0KDUxpc3RBZGRyZXNzZXMSKi5oYXdrci5idXllci52MWFscGhhMS5MaXN0QWRkcmVzc2'
    'VzUmVxdWVzdBorLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkxpc3RBZGRyZXNzZXNSZXNwb25zZSID'
    'kAIBEmQKCkdldEFkZHJlc3MSJy5oYXdrci5idXllci52MWFscGhhMS5HZXRBZGRyZXNzUmVxdW'
    'VzdBooLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkdldEFkZHJlc3NSZXNwb25zZSIDkAIBEmgKDUNy'
    'ZWF0ZUFkZHJlc3MSKi5oYXdrci5idXllci52MWFscGhhMS5DcmVhdGVBZGRyZXNzUmVxdWVzdB'
    'orLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkNyZWF0ZUFkZHJlc3NSZXNwb25zZRJoCg1VcGRhdGVB'
    'ZGRyZXNzEiouaGF3a3IuYnV5ZXIudjFhbHBoYTEuVXBkYXRlQWRkcmVzc1JlcXVlc3QaKy5oYX'
    'drci5idXllci52MWFscGhhMS5VcGRhdGVBZGRyZXNzUmVzcG9uc2USaAoNRGVsZXRlQWRkcmVz'
    'cxIqLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkRlbGV0ZUFkZHJlc3NSZXF1ZXN0GisuaGF3a3IuYn'
    'V5ZXIudjFhbHBoYTEuRGVsZXRlQWRkcmVzc1Jlc3BvbnNlEm0KDUxpc3RGYXZvcml0ZXMSKi5o'
    'YXdrci5idXllci52MWFscGhhMS5MaXN0RmF2b3JpdGVzUmVxdWVzdBorLmhhd2tyLmJ1eWVyLn'
    'YxYWxwaGExLkxpc3RGYXZvcml0ZXNSZXNwb25zZSIDkAIBEm0KDUNoZWNrRmF2b3JpdGUSKi5o'
    'YXdrci5idXllci52MWFscGhhMS5DaGVja0Zhdm9yaXRlUmVxdWVzdBorLmhhd2tyLmJ1eWVyLn'
    'YxYWxwaGExLkNoZWNrRmF2b3JpdGVSZXNwb25zZSIDkAIBEn8KE0JhdGNoQ2hlY2tGYXZvcml0'
    'ZXMSMC5oYXdrci5idXllci52MWFscGhhMS5CYXRjaENoZWNrRmF2b3JpdGVzUmVxdWVzdBoxLm'
    'hhd2tyLmJ1eWVyLnYxYWxwaGExLkJhdGNoQ2hlY2tGYXZvcml0ZXNSZXNwb25zZSIDkAIBEmcK'
    'C0FkZEZhdm9yaXRlEiguaGF3a3IuYnV5ZXIudjFhbHBoYTEuQWRkRmF2b3JpdGVSZXF1ZXN0Gi'
    'kuaGF3a3IuYnV5ZXIudjFhbHBoYTEuQWRkRmF2b3JpdGVSZXNwb25zZSIDkAICEnAKDlJlbW92'
    'ZUZhdm9yaXRlEisuaGF3a3IuYnV5ZXIudjFhbHBoYTEuUmVtb3ZlRmF2b3JpdGVSZXF1ZXN0Gi'
    'wuaGF3a3IuYnV5ZXIudjFhbHBoYTEuUmVtb3ZlRmF2b3JpdGVSZXNwb25zZSIDkAICEmcKC0xp'
    'c3RGb2xsb3dzEiguaGF3a3IuYnV5ZXIudjFhbHBoYTEuTGlzdEZvbGxvd3NSZXF1ZXN0GikuaG'
    'F3a3IuYnV5ZXIudjFhbHBoYTEuTGlzdEZvbGxvd3NSZXNwb25zZSIDkAIBEmcKC0NoZWNrRm9s'
    'bG93EiguaGF3a3IuYnV5ZXIudjFhbHBoYTEuQ2hlY2tGb2xsb3dSZXF1ZXN0GikuaGF3a3IuYn'
    'V5ZXIudjFhbHBoYTEuQ2hlY2tGb2xsb3dSZXNwb25zZSIDkAIBEnkKEUJhdGNoQ2hlY2tGb2xs'
    'b3dzEi4uaGF3a3IuYnV5ZXIudjFhbHBoYTEuQmF0Y2hDaGVja0ZvbGxvd3NSZXF1ZXN0Gi8uaG'
    'F3a3IuYnV5ZXIudjFhbHBoYTEuQmF0Y2hDaGVja0ZvbGxvd3NSZXNwb25zZSIDkAIBEmcKC0Zv'
    'bGxvd1N0b3JlEiguaGF3a3IuYnV5ZXIudjFhbHBoYTEuRm9sbG93U3RvcmVSZXF1ZXN0GikuaG'
    'F3a3IuYnV5ZXIudjFhbHBoYTEuRm9sbG93U3RvcmVSZXNwb25zZSIDkAICEm0KDVVuZm9sbG93'
    'U3RvcmUSKi5oYXdrci5idXllci52MWFscGhhMS5VbmZvbGxvd1N0b3JlUmVxdWVzdBorLmhhd2'
    'tyLmJ1eWVyLnYxYWxwaGExLlVuZm9sbG93U3RvcmVSZXNwb25zZSIDkAICEnMKD0xpc3RNZW1i'
    'ZXJDYXJkcxIsLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkxpc3RNZW1iZXJDYXJkc1JlcXVlc3QaLS'
    '5oYXdrci5idXllci52MWFscGhhMS5MaXN0TWVtYmVyQ2FyZHNSZXNwb25zZSIDkAIBEn8KE0xp'
    'c3RTcGVuZGluZ1JlY29yZHMSMC5oYXdrci5idXllci52MWFscGhhMS5MaXN0U3BlbmRpbmdSZW'
    'NvcmRzUmVxdWVzdBoxLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkxpc3RTcGVuZGluZ1JlY29yZHNS'
    'ZXNwb25zZSIDkAIBEmQKCkxpc3RWaXNpdHMSJy5oYXdrci5idXllci52MWFscGhhMS5MaXN0Vm'
    'lzaXRzUmVxdWVzdBooLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkxpc3RWaXNpdHNSZXNwb25zZSID'
    'kAIBEmIKC1JlY29yZFZpc2l0EiguaGF3a3IuYnV5ZXIudjFhbHBoYTEuUmVjb3JkVmlzaXRSZX'
    'F1ZXN0GikuaGF3a3IuYnV5ZXIudjFhbHBoYTEuUmVjb3JkVmlzaXRSZXNwb25zZRJnCgtDbGVh'
    'clZpc2l0cxIoLmhhd2tyLmJ1eWVyLnYxYWxwaGExLkNsZWFyVmlzaXRzUmVxdWVzdBopLmhhd2'
    'tyLmJ1eWVyLnYxYWxwaGExLkNsZWFyVmlzaXRzUmVzcG9uc2UiA5ACAhKOAQoYTGlzdFJlc3Rv'
    'Y2tTdWJzY3JpcHRpb25zEjUuaGF3a3IuYnV5ZXIudjFhbHBoYTEuTGlzdFJlc3RvY2tTdWJzY3'
    'JpcHRpb25zUmVxdWVzdBo2Lmhhd2tyLmJ1eWVyLnYxYWxwaGExLkxpc3RSZXN0b2NrU3Vic2Ny'
    'aXB0aW9uc1Jlc3BvbnNlIgOQAgESdgoQU3Vic2NyaWJlUmVzdG9jaxItLmhhd2tyLmJ1eWVyLn'
    'YxYWxwaGExLlN1YnNjcmliZVJlc3RvY2tSZXF1ZXN0Gi4uaGF3a3IuYnV5ZXIudjFhbHBoYTEu'
    'U3Vic2NyaWJlUmVzdG9ja1Jlc3BvbnNlIgOQAgISfAoSVW5zdWJzY3JpYmVSZXN0b2NrEi8uaG'
    'F3a3IuYnV5ZXIudjFhbHBoYTEuVW5zdWJzY3JpYmVSZXN0b2NrUmVxdWVzdBowLmhhd2tyLmJ1'
    'eWVyLnYxYWxwaGExLlVuc3Vic2NyaWJlUmVzdG9ja1Jlc3BvbnNlIgOQAgI=');
