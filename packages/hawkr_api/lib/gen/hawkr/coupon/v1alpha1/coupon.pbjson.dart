// This is a generated file - do not edit.
//
// Generated from hawkr/coupon/v1alpha1/coupon.proto.

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

import '../../common/v1alpha1/image.pbjson.dart' as $2;
import '../../common/v1alpha1/store_card.pbjson.dart' as $0;

@$core.Deprecated('Use discountTypeDescriptor instead')
const DiscountType$json = {
  '1': 'DiscountType',
  '2': [
    {'1': 'DISCOUNT_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'DISCOUNT_TYPE_CASH', '2': 1},
    {'1': 'DISCOUNT_TYPE_PERCENT', '2': 2},
  ],
};

/// Descriptor for `DiscountType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List discountTypeDescriptor = $convert.base64Decode(
    'CgxEaXNjb3VudFR5cGUSHQoZRElTQ09VTlRfVFlQRV9VTlNQRUNJRklFRBAAEhYKEkRJU0NPVU'
    '5UX1RZUEVfQ0FTSBABEhkKFURJU0NPVU5UX1RZUEVfUEVSQ0VOVBAC');

@$core.Deprecated('Use targetTypeDescriptor instead')
const TargetType$json = {
  '1': 'TargetType',
  '2': [
    {'1': 'TARGET_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'TARGET_TYPE_ALL', '2': 1},
    {'1': 'TARGET_TYPE_MEMBER', '2': 2},
    {'1': 'TARGET_TYPE_DESIGNATED', '2': 3},
    {'1': 'TARGET_TYPE_COMPENSATION', '2': 4},
  ],
};

/// Descriptor for `TargetType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List targetTypeDescriptor = $convert.base64Decode(
    'CgpUYXJnZXRUeXBlEhsKF1RBUkdFVF9UWVBFX1VOU1BFQ0lGSUVEEAASEwoPVEFSR0VUX1RZUE'
    'VfQUxMEAESFgoSVEFSR0VUX1RZUEVfTUVNQkVSEAISGgoWVEFSR0VUX1RZUEVfREVTSUdOQVRF'
    'RBADEhwKGFRBUkdFVF9UWVBFX0NPTVBFTlNBVElPThAE');

@$core.Deprecated('Use campaignStatusDescriptor instead')
const CampaignStatus$json = {
  '1': 'CampaignStatus',
  '2': [
    {'1': 'CAMPAIGN_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'CAMPAIGN_STATUS_DRAFT', '2': 1},
    {'1': 'CAMPAIGN_STATUS_ACTIVE', '2': 2},
    {'1': 'CAMPAIGN_STATUS_ENDED', '2': 3},
  ],
};

/// Descriptor for `CampaignStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List campaignStatusDescriptor = $convert.base64Decode(
    'Cg5DYW1wYWlnblN0YXR1cxIfChtDQU1QQUlHTl9TVEFUVVNfVU5TUEVDSUZJRUQQABIZChVDQU'
    '1QQUlHTl9TVEFUVVNfRFJBRlQQARIaChZDQU1QQUlHTl9TVEFUVVNfQUNUSVZFEAISGQoVQ0FN'
    'UEFJR05fU1RBVFVTX0VOREVEEAM=');

@$core.Deprecated('Use voucherStatusDescriptor instead')
const VoucherStatus$json = {
  '1': 'VoucherStatus',
  '2': [
    {'1': 'VOUCHER_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'VOUCHER_STATUS_AVAILABLE', '2': 1},
    {'1': 'VOUCHER_STATUS_LOCKED', '2': 2},
    {'1': 'VOUCHER_STATUS_REDEEMED', '2': 3},
    {'1': 'VOUCHER_STATUS_VOIDED', '2': 4},
  ],
};

/// Descriptor for `VoucherStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List voucherStatusDescriptor = $convert.base64Decode(
    'Cg1Wb3VjaGVyU3RhdHVzEh4KGlZPVUNIRVJfU1RBVFVTX1VOU1BFQ0lGSUVEEAASHAoYVk9VQ0'
    'hFUl9TVEFUVVNfQVZBSUxBQkxFEAESGQoVVk9VQ0hFUl9TVEFUVVNfTE9DS0VEEAISGwoXVk9V'
    'Q0hFUl9TVEFUVVNfUkVERUVNRUQQAxIZChVWT1VDSEVSX1NUQVRVU19WT0lERUQQBA==');

@$core.Deprecated('Use voucherOriginDescriptor instead')
const VoucherOrigin$json = {
  '1': 'VoucherOrigin',
  '2': [
    {'1': 'VOUCHER_ORIGIN_UNSPECIFIED', '2': 0},
    {'1': 'VOUCHER_ORIGIN_CLAIM', '2': 1},
    {'1': 'VOUCHER_ORIGIN_GRANT', '2': 2},
    {'1': 'VOUCHER_ORIGIN_COMPENSATION', '2': 3},
  ],
};

/// Descriptor for `VoucherOrigin`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List voucherOriginDescriptor = $convert.base64Decode(
    'Cg1Wb3VjaGVyT3JpZ2luEh4KGlZPVUNIRVJfT1JJR0lOX1VOU1BFQ0lGSUVEEAASGAoUVk9VQ0'
    'hFUl9PUklHSU5fQ0xBSU0QARIYChRWT1VDSEVSX09SSUdJTl9HUkFOVBACEh8KG1ZPVUNIRVJf'
    'T1JJR0lOX0NPTVBFTlNBVElPThAD');

@$core.Deprecated('Use cashDiscountDescriptor instead')
const CashDiscount$json = {
  '1': 'CashDiscount',
  '2': [
    {'1': 'amount', '3': 1, '4': 1, '5': 3, '8': {}, '10': 'amount'},
  ],
};

/// Descriptor for `CashDiscount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cashDiscountDescriptor = $convert.base64Decode(
    'CgxDYXNoRGlzY291bnQSGwoGYW1vdW50GAEgASgDQgPgQQJSBmFtb3VudA==');

@$core.Deprecated('Use percentDiscountDescriptor instead')
const PercentDiscount$json = {
  '1': 'PercentDiscount',
  '2': [
    {'1': 'percent', '3': 1, '4': 1, '5': 5, '8': {}, '10': 'percent'},
    {'1': 'max_amount', '3': 2, '4': 1, '5': 3, '8': {}, '10': 'maxAmount'},
  ],
};

/// Descriptor for `PercentDiscount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List percentDiscountDescriptor = $convert.base64Decode(
    'Cg9QZXJjZW50RGlzY291bnQSHQoHcGVyY2VudBgBIAEoBUID4EECUgdwZXJjZW50EiIKCm1heF'
    '9hbW91bnQYAiABKANCA+BBAlIJbWF4QW1vdW50');

@$core.Deprecated('Use campaignDescriptor instead')
const Campaign$json = {
  '1': 'Campaign',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'cash',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.CashDiscount',
      '8': {},
      '9': 0,
      '10': 'cash'
    },
    {
      '1': 'percent',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.PercentDiscount',
      '8': {},
      '9': 0,
      '10': 'percent'
    },
    {
      '1': 'threshold_amount',
      '3': 5,
      '4': 1,
      '5': 3,
      '8': {},
      '10': 'thresholdAmount'
    },
    {
      '1': 'target_type',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.TargetType',
      '8': {},
      '10': 'targetType'
    },
    {
      '1': 'store',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '8': {},
      '9': 1,
      '10': 'store',
      '17': true
    },
    {
      '1': 'store_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 2,
      '10': 'storeId',
      '17': true
    },
    {
      '1': 'start_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'startTime'
    },
    {
      '1': 'end_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'endTime'
    },
  ],
  '8': [
    {'1': 'discount'},
    {'1': '_store'},
    {'1': '_store_id'},
  ],
};

/// Descriptor for `Campaign`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List campaignDescriptor = $convert.base64Decode(
    'CghDYW1wYWlnbhITCgJpZBgBIAEoCUID4EEIUgJpZBIZCgV0aXRsZRgCIAEoCUID4EEDUgV0aX'
    'RsZRI+CgRjYXNoGAMgASgLMiMuaGF3a3IuY291cG9uLnYxYWxwaGExLkNhc2hEaXNjb3VudEID'
    '4EEDSABSBGNhc2gSRwoHcGVyY2VudBgEIAEoCzImLmhhd2tyLmNvdXBvbi52MWFscGhhMS5QZX'
    'JjZW50RGlzY291bnRCA+BBA0gAUgdwZXJjZW50Ei4KEHRocmVzaG9sZF9hbW91bnQYBSABKANC'
    'A+BBA1IPdGhyZXNob2xkQW1vdW50EkcKC3RhcmdldF90eXBlGAYgASgOMiEuaGF3a3IuY291cG'
    '9uLnYxYWxwaGExLlRhcmdldFR5cGVCA+BBA1IKdGFyZ2V0VHlwZRJACgVzdG9yZRgHIAEoCzIg'
    'Lmhhd2tyLmNvbW1vbi52MWFscGhhMS5TdG9yZUNhcmRCA+BBA0gBUgVzdG9yZYgBARIjCghzdG'
    '9yZV9pZBgIIAEoCUID4EEDSAJSB3N0b3JlSWSIAQESPgoKc3RhcnRfdGltZRgJIAEoCzIaLmdv'
    'b2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1IJc3RhcnRUaW1lEjoKCGVuZF90aW1lGAogAS'
    'gLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgdlbmRUaW1lQgoKCGRpc2NvdW50'
    'QggKBl9zdG9yZUILCglfc3RvcmVfaWQ=');

@$core.Deprecated('Use voucherDescriptor instead')
const Voucher$json = {
  '1': 'Voucher',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'campaign',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.Campaign',
      '8': {},
      '10': 'campaign'
    },
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.VoucherStatus',
      '8': {},
      '10': 'status'
    },
    {
      '1': 'origin',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.VoucherOrigin',
      '8': {},
      '10': 'origin'
    },
    {'1': 'expired', '3': 5, '4': 1, '5': 8, '8': {}, '10': 'expired'},
    {
      '1': 'create_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'estimated_discount',
      '3': 7,
      '4': 1,
      '5': 3,
      '8': {},
      '9': 0,
      '10': 'estimatedDiscount',
      '17': true
    },
  ],
  '8': [
    {'1': '_estimated_discount'},
  ],
};

/// Descriptor for `Voucher`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voucherDescriptor = $convert.base64Decode(
    'CgdWb3VjaGVyEhMKAmlkGAEgASgJQgPgQQhSAmlkEkAKCGNhbXBhaWduGAIgASgLMh8uaGF3a3'
    'IuY291cG9uLnYxYWxwaGExLkNhbXBhaWduQgPgQQNSCGNhbXBhaWduEkEKBnN0YXR1cxgDIAEo'
    'DjIkLmhhd2tyLmNvdXBvbi52MWFscGhhMS5Wb3VjaGVyU3RhdHVzQgPgQQNSBnN0YXR1cxJBCg'
    'ZvcmlnaW4YBCABKA4yJC5oYXdrci5jb3Vwb24udjFhbHBoYTEuVm91Y2hlck9yaWdpbkID4EED'
    'UgZvcmlnaW4SHQoHZXhwaXJlZBgFIAEoCEID4EEDUgdleHBpcmVkEkAKC2NyZWF0ZV90aW1lGA'
    'YgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgpjcmVhdGVUaW1lEjcKEmVz'
    'dGltYXRlZF9kaXNjb3VudBgHIAEoA0ID4EEDSABSEWVzdGltYXRlZERpc2NvdW50iAEBQhUKE1'
    '9lc3RpbWF0ZWRfZGlzY291bnQ=');

@$core.Deprecated('Use checkoutDescriptor instead')
const Checkout$json = {
  '1': 'Checkout',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'order_amount', '3': 2, '4': 1, '5': 3, '8': {}, '10': 'orderAmount'},
  ],
};

/// Descriptor for `Checkout`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkoutDescriptor = $convert.base64Decode(
    'CghDaGVja291dBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG9yZUlkEiYKDG9yZGVyX2Ftb3'
    'VudBgCIAEoA0ID4EECUgtvcmRlckFtb3VudA==');

@$core.Deprecated('Use listMyVouchersRequestDescriptor instead')
const ListMyVouchersRequest$json = {
  '1': 'ListMyVouchersRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'statuses',
      '3': 3,
      '4': 3,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.VoucherStatus',
      '10': 'statuses'
    },
    {
      '1': 'checkout',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.Checkout',
      '9': 0,
      '10': 'checkout',
      '17': true
    },
  ],
  '8': [
    {'1': '_checkout'},
  ],
};

/// Descriptor for `ListMyVouchersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyVouchersRequestDescriptor = $convert.base64Decode(
    'ChVMaXN0TXlWb3VjaGVyc1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZRIdCg'
    'pwYWdlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4SQAoIc3RhdHVzZXMYAyADKA4yJC5oYXdrci5j'
    'b3Vwb24udjFhbHBoYTEuVm91Y2hlclN0YXR1c1IIc3RhdHVzZXMSQAoIY2hlY2tvdXQYBCABKA'
    'syHy5oYXdrci5jb3Vwb24udjFhbHBoYTEuQ2hlY2tvdXRIAFIIY2hlY2tvdXSIAQFCCwoJX2No'
    'ZWNrb3V0');

@$core.Deprecated('Use listMyVouchersResponseDescriptor instead')
const ListMyVouchersResponse$json = {
  '1': 'ListMyVouchersResponse',
  '2': [
    {
      '1': 'vouchers',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.Voucher',
      '10': 'vouchers'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyVouchersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyVouchersResponseDescriptor = $convert.base64Decode(
    'ChZMaXN0TXlWb3VjaGVyc1Jlc3BvbnNlEjoKCHZvdWNoZXJzGAEgAygLMh4uaGF3a3IuY291cG'
    '9uLnYxYWxwaGExLlZvdWNoZXJSCHZvdWNoZXJzEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVIN'
    'bmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use listClaimableCampaignsRequestDescriptor instead')
const ListClaimableCampaignsRequest$json = {
  '1': 'ListClaimableCampaignsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'store_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'storeId',
      '17': true
    },
  ],
  '8': [
    {'1': '_store_id'},
  ],
};

/// Descriptor for `ListClaimableCampaignsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listClaimableCampaignsRequestDescriptor =
    $convert.base64Decode(
        'Ch1MaXN0Q2xhaW1hYmxlQ2FtcGFpZ25zUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2'
        'VTaXplEh0KCnBhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbhIeCghzdG9yZV9pZBgDIAEoCUgA'
        'UgdzdG9yZUlkiAEBQgsKCV9zdG9yZV9pZA==');

@$core.Deprecated('Use listClaimableCampaignsResponseDescriptor instead')
const ListClaimableCampaignsResponse$json = {
  '1': 'ListClaimableCampaignsResponse',
  '2': [
    {
      '1': 'campaigns',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.Campaign',
      '10': 'campaigns'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListClaimableCampaignsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listClaimableCampaignsResponseDescriptor =
    $convert.base64Decode(
        'Ch5MaXN0Q2xhaW1hYmxlQ2FtcGFpZ25zUmVzcG9uc2USPQoJY2FtcGFpZ25zGAEgAygLMh8uaG'
        'F3a3IuY291cG9uLnYxYWxwaGExLkNhbXBhaWduUgljYW1wYWlnbnMSJgoPbmV4dF9wYWdlX3Rv'
        'a2VuGAIgASgJUg1uZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use claimCouponRequestDescriptor instead')
const ClaimCouponRequest$json = {
  '1': 'ClaimCouponRequest',
  '2': [
    {'1': 'campaign_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'campaignId'},
  ],
};

/// Descriptor for `ClaimCouponRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List claimCouponRequestDescriptor = $convert.base64Decode(
    'ChJDbGFpbUNvdXBvblJlcXVlc3QSJAoLY2FtcGFpZ25faWQYASABKAlCA+BBAlIKY2FtcGFpZ2'
    '5JZA==');

@$core.Deprecated('Use claimCouponResponseDescriptor instead')
const ClaimCouponResponse$json = {
  '1': 'ClaimCouponResponse',
  '2': [
    {
      '1': 'voucher',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.Voucher',
      '10': 'voucher'
    },
  ],
};

/// Descriptor for `ClaimCouponResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List claimCouponResponseDescriptor = $convert.base64Decode(
    'ChNDbGFpbUNvdXBvblJlc3BvbnNlEjgKB3ZvdWNoZXIYASABKAsyHi5oYXdrci5jb3Vwb24udj'
    'FhbHBoYTEuVm91Y2hlclIHdm91Y2hlcg==');

@$core.Deprecated('Use sellerCampaignDescriptor instead')
const SellerCampaign$json = {
  '1': 'SellerCampaign',
  '2': [
    {
      '1': 'campaign',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.Campaign',
      '8': {},
      '10': 'campaign'
    },
    {
      '1': 'status',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.CampaignStatus',
      '8': {},
      '10': 'status'
    },
    {
      '1': 'total_issue_limit',
      '3': 3,
      '4': 1,
      '5': 5,
      '8': {},
      '9': 0,
      '10': 'totalIssueLimit',
      '17': true
    },
    {'1': 'issued_count', '3': 4, '4': 1, '5': 5, '8': {}, '10': 'issuedCount'},
    {'1': 'expired', '3': 5, '4': 1, '5': 8, '8': {}, '10': 'expired'},
    {
      '1': 'create_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'ended_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '9': 1,
      '10': 'endedTime',
      '17': true
    },
  ],
  '8': [
    {'1': '_total_issue_limit'},
    {'1': '_ended_time'},
  ],
};

/// Descriptor for `SellerCampaign`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerCampaignDescriptor = $convert.base64Decode(
    'Cg5TZWxsZXJDYW1wYWlnbhJACghjYW1wYWlnbhgBIAEoCzIfLmhhd2tyLmNvdXBvbi52MWFscG'
    'hhMS5DYW1wYWlnbkID4EEDUghjYW1wYWlnbhJCCgZzdGF0dXMYAiABKA4yJS5oYXdrci5jb3Vw'
    'b24udjFhbHBoYTEuQ2FtcGFpZ25TdGF0dXNCA+BBA1IGc3RhdHVzEjQKEXRvdGFsX2lzc3VlX2'
    'xpbWl0GAMgASgFQgPgQQNIAFIPdG90YWxJc3N1ZUxpbWl0iAEBEiYKDGlzc3VlZF9jb3VudBgE'
    'IAEoBUID4EEDUgtpc3N1ZWRDb3VudBIdCgdleHBpcmVkGAUgASgIQgPgQQNSB2V4cGlyZWQSQA'
    'oLY3JlYXRlX3RpbWUYBiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCmNy'
    'ZWF0ZVRpbWUSQwoKZW5kZWRfdGltZRgHIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbX'
    'BCA+BBA0gBUgllbmRlZFRpbWWIAQFCFAoSX3RvdGFsX2lzc3VlX2xpbWl0Qg0KC19lbmRlZF90'
    'aW1l');

@$core.Deprecated('Use campaignInputDescriptor instead')
const CampaignInput$json = {
  '1': 'CampaignInput',
  '2': [
    {'1': 'title', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'title'},
    {
      '1': 'cash',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.CashDiscount',
      '9': 0,
      '10': 'cash'
    },
    {
      '1': 'percent',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.PercentDiscount',
      '9': 0,
      '10': 'percent'
    },
    {'1': 'threshold_amount', '3': 4, '4': 1, '5': 3, '10': 'thresholdAmount'},
    {
      '1': 'target_type',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.TargetType',
      '8': {},
      '10': 'targetType'
    },
    {
      '1': 'start_time',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'startTime'
    },
    {
      '1': 'end_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'endTime'
    },
    {
      '1': 'total_issue_limit',
      '3': 8,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'totalIssueLimit',
      '17': true
    },
    {
      '1': 'designated_buyer_ids',
      '3': 9,
      '4': 3,
      '5': 9,
      '10': 'designatedBuyerIds'
    },
  ],
  '8': [
    {'1': 'discount'},
    {'1': '_total_issue_limit'},
  ],
};

/// Descriptor for `CampaignInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List campaignInputDescriptor = $convert.base64Decode(
    'Cg1DYW1wYWlnbklucHV0EhkKBXRpdGxlGAEgASgJQgPgQQJSBXRpdGxlEjkKBGNhc2gYAiABKA'
    'syIy5oYXdrci5jb3Vwb24udjFhbHBoYTEuQ2FzaERpc2NvdW50SABSBGNhc2gSQgoHcGVyY2Vu'
    'dBgDIAEoCzImLmhhd2tyLmNvdXBvbi52MWFscGhhMS5QZXJjZW50RGlzY291bnRIAFIHcGVyY2'
    'VudBIpChB0aHJlc2hvbGRfYW1vdW50GAQgASgDUg90aHJlc2hvbGRBbW91bnQSRwoLdGFyZ2V0'
    'X3R5cGUYBSABKA4yIS5oYXdrci5jb3Vwb24udjFhbHBoYTEuVGFyZ2V0VHlwZUID4EECUgp0YX'
    'JnZXRUeXBlEj4KCnN0YXJ0X3RpbWUYBiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1w'
    'QgPgQQJSCXN0YXJ0VGltZRI6CghlbmRfdGltZRgHIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW'
    '1lc3RhbXBCA+BBAlIHZW5kVGltZRIvChF0b3RhbF9pc3N1ZV9saW1pdBgIIAEoBUgBUg90b3Rh'
    'bElzc3VlTGltaXSIAQESMAoUZGVzaWduYXRlZF9idXllcl9pZHMYCSADKAlSEmRlc2lnbmF0ZW'
    'RCdXllcklkc0IKCghkaXNjb3VudEIUChJfdG90YWxfaXNzdWVfbGltaXQ=');

@$core.Deprecated('Use createCampaignRequestDescriptor instead')
const CreateCampaignRequest$json = {
  '1': 'CreateCampaignRequest',
  '2': [
    {
      '1': 'campaign',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.CampaignInput',
      '8': {},
      '10': 'campaign'
    },
  ],
};

/// Descriptor for `CreateCampaignRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCampaignRequestDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVDYW1wYWlnblJlcXVlc3QSRQoIY2FtcGFpZ24YASABKAsyJC5oYXdrci5jb3Vwb2'
    '4udjFhbHBoYTEuQ2FtcGFpZ25JbnB1dEID4EECUghjYW1wYWlnbg==');

@$core.Deprecated('Use createCampaignResponseDescriptor instead')
const CreateCampaignResponse$json = {
  '1': 'CreateCampaignResponse',
  '2': [
    {
      '1': 'campaign',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.SellerCampaign',
      '10': 'campaign'
    },
  ],
};

/// Descriptor for `CreateCampaignResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCampaignResponseDescriptor =
    $convert.base64Decode(
        'ChZDcmVhdGVDYW1wYWlnblJlc3BvbnNlEkEKCGNhbXBhaWduGAEgASgLMiUuaGF3a3IuY291cG'
        '9uLnYxYWxwaGExLlNlbGxlckNhbXBhaWduUghjYW1wYWlnbg==');

@$core.Deprecated('Use listCampaignsRequestDescriptor instead')
const ListCampaignsRequest$json = {
  '1': 'ListCampaignsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.coupon.v1alpha1.CampaignStatus',
      '9': 0,
      '10': 'status',
      '17': true
    },
  ],
  '8': [
    {'1': '_status'},
  ],
};

/// Descriptor for `ListCampaignsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCampaignsRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0Q2FtcGFpZ25zUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaXplEh0KCn'
    'BhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbhJCCgZzdGF0dXMYAyABKA4yJS5oYXdrci5jb3Vw'
    'b24udjFhbHBoYTEuQ2FtcGFpZ25TdGF0dXNIAFIGc3RhdHVziAEBQgkKB19zdGF0dXM=');

@$core.Deprecated('Use listCampaignsResponseDescriptor instead')
const ListCampaignsResponse$json = {
  '1': 'ListCampaignsResponse',
  '2': [
    {
      '1': 'campaigns',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.SellerCampaign',
      '10': 'campaigns'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
    {
      '1': 'max_active_campaigns',
      '3': 3,
      '4': 1,
      '5': 5,
      '10': 'maxActiveCampaigns'
    },
    {'1': 'active_campaigns', '3': 4, '4': 1, '5': 5, '10': 'activeCampaigns'},
  ],
};

/// Descriptor for `ListCampaignsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCampaignsResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0Q2FtcGFpZ25zUmVzcG9uc2USQwoJY2FtcGFpZ25zGAEgAygLMiUuaGF3a3IuY291cG'
    '9uLnYxYWxwaGExLlNlbGxlckNhbXBhaWduUgljYW1wYWlnbnMSJgoPbmV4dF9wYWdlX3Rva2Vu'
    'GAIgASgJUg1uZXh0UGFnZVRva2VuEjAKFG1heF9hY3RpdmVfY2FtcGFpZ25zGAMgASgFUhJtYX'
    'hBY3RpdmVDYW1wYWlnbnMSKQoQYWN0aXZlX2NhbXBhaWducxgEIAEoBVIPYWN0aXZlQ2FtcGFp'
    'Z25z');

@$core.Deprecated('Use getCampaignRequestDescriptor instead')
const GetCampaignRequest$json = {
  '1': 'GetCampaignRequest',
  '2': [
    {'1': 'campaign_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'campaignId'},
  ],
};

/// Descriptor for `GetCampaignRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCampaignRequestDescriptor = $convert.base64Decode(
    'ChJHZXRDYW1wYWlnblJlcXVlc3QSJAoLY2FtcGFpZ25faWQYASABKAlCA+BBAlIKY2FtcGFpZ2'
    '5JZA==');

@$core.Deprecated('Use designatedBuyerDescriptor instead')
const DesignatedBuyer$json = {
  '1': 'DesignatedBuyer',
  '2': [
    {'1': 'buyer_id', '3': 1, '4': 1, '5': 9, '10': 'buyerId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
  ],
  '8': [
    {'1': '_name'},
  ],
};

/// Descriptor for `DesignatedBuyer`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List designatedBuyerDescriptor = $convert.base64Decode(
    'Cg9EZXNpZ25hdGVkQnV5ZXISGQoIYnV5ZXJfaWQYASABKAlSB2J1eWVySWQSFwoEbmFtZRgCIA'
    'EoCUgAUgRuYW1liAEBQgcKBV9uYW1l');

@$core.Deprecated('Use getCampaignResponseDescriptor instead')
const GetCampaignResponse$json = {
  '1': 'GetCampaignResponse',
  '2': [
    {
      '1': 'campaign',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.SellerCampaign',
      '10': 'campaign'
    },
    {
      '1': 'designated_buyers',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.DesignatedBuyer',
      '10': 'designatedBuyers'
    },
    {'1': 'available_count', '3': 3, '4': 1, '5': 5, '10': 'availableCount'},
    {'1': 'locked_count', '3': 4, '4': 1, '5': 5, '10': 'lockedCount'},
    {'1': 'redeemed_count', '3': 5, '4': 1, '5': 5, '10': 'redeemedCount'},
    {'1': 'voided_count', '3': 6, '4': 1, '5': 5, '10': 'voidedCount'},
  ],
};

/// Descriptor for `GetCampaignResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCampaignResponseDescriptor = $convert.base64Decode(
    'ChNHZXRDYW1wYWlnblJlc3BvbnNlEkEKCGNhbXBhaWduGAEgASgLMiUuaGF3a3IuY291cG9uLn'
    'YxYWxwaGExLlNlbGxlckNhbXBhaWduUghjYW1wYWlnbhJTChFkZXNpZ25hdGVkX2J1eWVycxgC'
    'IAMoCzImLmhhd2tyLmNvdXBvbi52MWFscGhhMS5EZXNpZ25hdGVkQnV5ZXJSEGRlc2lnbmF0ZW'
    'RCdXllcnMSJwoPYXZhaWxhYmxlX2NvdW50GAMgASgFUg5hdmFpbGFibGVDb3VudBIhCgxsb2Nr'
    'ZWRfY291bnQYBCABKAVSC2xvY2tlZENvdW50EiUKDnJlZGVlbWVkX2NvdW50GAUgASgFUg1yZW'
    'RlZW1lZENvdW50EiEKDHZvaWRlZF9jb3VudBgGIAEoBVILdm9pZGVkQ291bnQ=');

@$core.Deprecated('Use endCampaignRequestDescriptor instead')
const EndCampaignRequest$json = {
  '1': 'EndCampaignRequest',
  '2': [
    {'1': 'campaign_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'campaignId'},
  ],
};

/// Descriptor for `EndCampaignRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List endCampaignRequestDescriptor = $convert.base64Decode(
    'ChJFbmRDYW1wYWlnblJlcXVlc3QSJAoLY2FtcGFpZ25faWQYASABKAlCA+BBAlIKY2FtcGFpZ2'
    '5JZA==');

@$core.Deprecated('Use endCampaignResponseDescriptor instead')
const EndCampaignResponse$json = {
  '1': 'EndCampaignResponse',
  '2': [
    {
      '1': 'campaign',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.coupon.v1alpha1.SellerCampaign',
      '10': 'campaign'
    },
    {'1': 'voided_vouchers', '3': 2, '4': 1, '5': 5, '10': 'voidedVouchers'},
  ],
};

/// Descriptor for `EndCampaignResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List endCampaignResponseDescriptor = $convert.base64Decode(
    'ChNFbmRDYW1wYWlnblJlc3BvbnNlEkEKCGNhbXBhaWduGAEgASgLMiUuaGF3a3IuY291cG9uLn'
    'YxYWxwaGExLlNlbGxlckNhbXBhaWduUghjYW1wYWlnbhInCg92b2lkZWRfdm91Y2hlcnMYAiAB'
    'KAVSDnZvaWRlZFZvdWNoZXJz');

const $core.Map<$core.String, $core.dynamic> CouponAppServiceBase$json = {
  '1': 'CouponAppService',
  '2': [
    {
      '1': 'ListMyVouchers',
      '2': '.hawkr.coupon.v1alpha1.ListMyVouchersRequest',
      '3': '.hawkr.coupon.v1alpha1.ListMyVouchersResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListClaimableCampaigns',
      '2': '.hawkr.coupon.v1alpha1.ListClaimableCampaignsRequest',
      '3': '.hawkr.coupon.v1alpha1.ListClaimableCampaignsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ClaimCoupon',
      '2': '.hawkr.coupon.v1alpha1.ClaimCouponRequest',
      '3': '.hawkr.coupon.v1alpha1.ClaimCouponResponse',
      '4': {'34': 2},
    },
    {
      '1': 'CreateCampaign',
      '2': '.hawkr.coupon.v1alpha1.CreateCampaignRequest',
      '3': '.hawkr.coupon.v1alpha1.CreateCampaignResponse'
    },
    {
      '1': 'ListCampaigns',
      '2': '.hawkr.coupon.v1alpha1.ListCampaignsRequest',
      '3': '.hawkr.coupon.v1alpha1.ListCampaignsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetCampaign',
      '2': '.hawkr.coupon.v1alpha1.GetCampaignRequest',
      '3': '.hawkr.coupon.v1alpha1.GetCampaignResponse',
      '4': {'34': 1},
    },
    {
      '1': 'EndCampaign',
      '2': '.hawkr.coupon.v1alpha1.EndCampaignRequest',
      '3': '.hawkr.coupon.v1alpha1.EndCampaignResponse',
      '4': {'34': 2},
    },
  ],
};

@$core.Deprecated('Use couponAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    CouponAppServiceBase$messageJson = {
  '.hawkr.coupon.v1alpha1.ListMyVouchersRequest': ListMyVouchersRequest$json,
  '.hawkr.coupon.v1alpha1.Checkout': Checkout$json,
  '.hawkr.coupon.v1alpha1.ListMyVouchersResponse': ListMyVouchersResponse$json,
  '.hawkr.coupon.v1alpha1.Voucher': Voucher$json,
  '.hawkr.coupon.v1alpha1.Campaign': Campaign$json,
  '.hawkr.coupon.v1alpha1.CashDiscount': CashDiscount$json,
  '.hawkr.coupon.v1alpha1.PercentDiscount': PercentDiscount$json,
  '.hawkr.common.v1alpha1.StoreCard': $0.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $2.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.coupon.v1alpha1.ListClaimableCampaignsRequest':
      ListClaimableCampaignsRequest$json,
  '.hawkr.coupon.v1alpha1.ListClaimableCampaignsResponse':
      ListClaimableCampaignsResponse$json,
  '.hawkr.coupon.v1alpha1.ClaimCouponRequest': ClaimCouponRequest$json,
  '.hawkr.coupon.v1alpha1.ClaimCouponResponse': ClaimCouponResponse$json,
  '.hawkr.coupon.v1alpha1.CreateCampaignRequest': CreateCampaignRequest$json,
  '.hawkr.coupon.v1alpha1.CampaignInput': CampaignInput$json,
  '.hawkr.coupon.v1alpha1.CreateCampaignResponse': CreateCampaignResponse$json,
  '.hawkr.coupon.v1alpha1.SellerCampaign': SellerCampaign$json,
  '.hawkr.coupon.v1alpha1.ListCampaignsRequest': ListCampaignsRequest$json,
  '.hawkr.coupon.v1alpha1.ListCampaignsResponse': ListCampaignsResponse$json,
  '.hawkr.coupon.v1alpha1.GetCampaignRequest': GetCampaignRequest$json,
  '.hawkr.coupon.v1alpha1.GetCampaignResponse': GetCampaignResponse$json,
  '.hawkr.coupon.v1alpha1.DesignatedBuyer': DesignatedBuyer$json,
  '.hawkr.coupon.v1alpha1.EndCampaignRequest': EndCampaignRequest$json,
  '.hawkr.coupon.v1alpha1.EndCampaignResponse': EndCampaignResponse$json,
};

/// Descriptor for `CouponAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List couponAppServiceDescriptor = $convert.base64Decode(
    'ChBDb3Vwb25BcHBTZXJ2aWNlEnIKDkxpc3RNeVZvdWNoZXJzEiwuaGF3a3IuY291cG9uLnYxYW'
    'xwaGExLkxpc3RNeVZvdWNoZXJzUmVxdWVzdBotLmhhd2tyLmNvdXBvbi52MWFscGhhMS5MaXN0'
    'TXlWb3VjaGVyc1Jlc3BvbnNlIgOQAgESigEKFkxpc3RDbGFpbWFibGVDYW1wYWlnbnMSNC5oYX'
    'drci5jb3Vwb24udjFhbHBoYTEuTGlzdENsYWltYWJsZUNhbXBhaWduc1JlcXVlc3QaNS5oYXdr'
    'ci5jb3Vwb24udjFhbHBoYTEuTGlzdENsYWltYWJsZUNhbXBhaWduc1Jlc3BvbnNlIgOQAgESaQ'
    'oLQ2xhaW1Db3Vwb24SKS5oYXdrci5jb3Vwb24udjFhbHBoYTEuQ2xhaW1Db3Vwb25SZXF1ZXN0'
    'GiouaGF3a3IuY291cG9uLnYxYWxwaGExLkNsYWltQ291cG9uUmVzcG9uc2UiA5ACAhJtCg5Dcm'
    'VhdGVDYW1wYWlnbhIsLmhhd2tyLmNvdXBvbi52MWFscGhhMS5DcmVhdGVDYW1wYWlnblJlcXVl'
    'c3QaLS5oYXdrci5jb3Vwb24udjFhbHBoYTEuQ3JlYXRlQ2FtcGFpZ25SZXNwb25zZRJvCg1MaX'
    'N0Q2FtcGFpZ25zEisuaGF3a3IuY291cG9uLnYxYWxwaGExLkxpc3RDYW1wYWlnbnNSZXF1ZXN0'
    'GiwuaGF3a3IuY291cG9uLnYxYWxwaGExLkxpc3RDYW1wYWlnbnNSZXNwb25zZSIDkAIBEmkKC0'
    'dldENhbXBhaWduEikuaGF3a3IuY291cG9uLnYxYWxwaGExLkdldENhbXBhaWduUmVxdWVzdBoq'
    'Lmhhd2tyLmNvdXBvbi52MWFscGhhMS5HZXRDYW1wYWlnblJlc3BvbnNlIgOQAgESaQoLRW5kQ2'
    'FtcGFpZ24SKS5oYXdrci5jb3Vwb24udjFhbHBoYTEuRW5kQ2FtcGFpZ25SZXF1ZXN0GiouaGF3'
    'a3IuY291cG9uLnYxYWxwaGExLkVuZENhbXBhaWduUmVzcG9uc2UiA5ACAg==');
