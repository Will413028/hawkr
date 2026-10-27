// This is a generated file - do not edit.
//
// Generated from hawkr/content/v1alpha1/content.proto.

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

@$core.Deprecated('Use appTypeDescriptor instead')
const AppType$json = {
  '1': 'AppType',
  '2': [
    {'1': 'APP_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'APP_TYPE_BUYER', '2': 1},
    {'1': 'APP_TYPE_SELLER', '2': 2},
  ],
};

/// Descriptor for `AppType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List appTypeDescriptor = $convert.base64Decode(
    'CgdBcHBUeXBlEhgKFEFQUF9UWVBFX1VOU1BFQ0lGSUVEEAASEgoOQVBQX1RZUEVfQlVZRVIQAR'
    'ITCg9BUFBfVFlQRV9TRUxMRVIQAg==');

@$core.Deprecated('Use bannerDescriptor instead')
const Banner$json = {
  '1': 'Banner',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
    {'1': 'button_text', '3': 3, '4': 1, '5': 9, '10': 'buttonText'},
    {
      '1': 'preview_image',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'previewImage'
    },
    {
      '1': 'popup_image',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'popupImage'
    },
    {
      '1': 'action',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.content.v1alpha1.BannerAction',
      '10': 'action'
    },
  ],
};

/// Descriptor for `Banner`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bannerDescriptor = $convert.base64Decode(
    'CgZCYW5uZXISDgoCaWQYASABKAlSAmlkEhQKBXRpdGxlGAIgASgJUgV0aXRsZRIfCgtidXR0b2'
    '5fdGV4dBgDIAEoCVIKYnV0dG9uVGV4dBJBCg1wcmV2aWV3X2ltYWdlGAQgASgLMhwuaGF3a3Iu'
    'Y29tbW9uLnYxYWxwaGExLkltYWdlUgxwcmV2aWV3SW1hZ2USPQoLcG9wdXBfaW1hZ2UYBSABKA'
    'syHC5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VSCnBvcHVwSW1hZ2USPAoGYWN0aW9uGAYg'
    'ASgLMiQuaGF3a3IuY29udGVudC52MWFscGhhMS5CYW5uZXJBY3Rpb25SBmFjdGlvbg==');

@$core.Deprecated('Use bannerActionDescriptor instead')
const BannerAction$json = {
  '1': 'BannerAction',
  '2': [
    {
      '1': 'popup',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.content.v1alpha1.BannerAction.Popup',
      '9': 0,
      '10': 'popup'
    },
    {'1': 'url', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'url'},
    {'1': 'product_id', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'productId'},
    {'1': 'post_id', '3': 4, '4': 1, '5': 9, '9': 0, '10': 'postId'},
  ],
  '3': [BannerAction_Popup$json],
  '8': [
    {'1': 'kind'},
  ],
};

@$core.Deprecated('Use bannerActionDescriptor instead')
const BannerAction_Popup$json = {
  '1': 'Popup',
};

/// Descriptor for `BannerAction`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bannerActionDescriptor = $convert.base64Decode(
    'CgxCYW5uZXJBY3Rpb24SQgoFcG9wdXAYASABKAsyKi5oYXdrci5jb250ZW50LnYxYWxwaGExLk'
    'Jhbm5lckFjdGlvbi5Qb3B1cEgAUgVwb3B1cBISCgN1cmwYAiABKAlIAFIDdXJsEh8KCnByb2R1'
    'Y3RfaWQYAyABKAlIAFIJcHJvZHVjdElkEhkKB3Bvc3RfaWQYBCABKAlIAFIGcG9zdElkGgcKBV'
    'BvcHVwQgYKBGtpbmQ=');

@$core.Deprecated('Use listLiveBannersRequestDescriptor instead')
const ListLiveBannersRequest$json = {
  '1': 'ListLiveBannersRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListLiveBannersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listLiveBannersRequestDescriptor =
    $convert.base64Decode(
        'ChZMaXN0TGl2ZUJhbm5lcnNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQ'
        'oKcGFnZV90b2tlbhgCIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listLiveBannersResponseDescriptor instead')
const ListLiveBannersResponse$json = {
  '1': 'ListLiveBannersResponse',
  '2': [
    {
      '1': 'banners',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.content.v1alpha1.Banner',
      '10': 'banners'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListLiveBannersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listLiveBannersResponseDescriptor = $convert.base64Decode(
    'ChdMaXN0TGl2ZUJhbm5lcnNSZXNwb25zZRI4CgdiYW5uZXJzGAEgAygLMh4uaGF3a3IuY29udG'
    'VudC52MWFscGhhMS5CYW5uZXJSB2Jhbm5lcnMSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJUg1u'
    'ZXh0UGFnZVRva2Vu');

@$core.Deprecated('Use checkAppVersionRequestDescriptor instead')
const CheckAppVersionRequest$json = {
  '1': 'CheckAppVersionRequest',
  '2': [
    {
      '1': 'app_type',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.content.v1alpha1.AppType',
      '8': {},
      '10': 'appType'
    },
    {
      '1': 'client_version',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientVersion'
    },
  ],
};

/// Descriptor for `CheckAppVersionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkAppVersionRequestDescriptor = $convert.base64Decode(
    'ChZDaGVja0FwcFZlcnNpb25SZXF1ZXN0Ej8KCGFwcF90eXBlGAEgASgOMh8uaGF3a3IuY29udG'
    'VudC52MWFscGhhMS5BcHBUeXBlQgPgQQJSB2FwcFR5cGUSKgoOY2xpZW50X3ZlcnNpb24YAyAB'
    'KAlCA+BBAlINY2xpZW50VmVyc2lvbg==');

@$core.Deprecated('Use checkAppVersionResponseDescriptor instead')
const CheckAppVersionResponse$json = {
  '1': 'CheckAppVersionResponse',
  '2': [
    {'1': 'update_required', '3': 1, '4': 1, '5': 8, '10': 'updateRequired'},
    {'1': 'minimum_version', '3': 2, '4': 1, '5': 9, '10': 'minimumVersion'},
    {'1': 'store_url', '3': 3, '4': 1, '5': 9, '10': 'storeUrl'},
    {'1': 'notice_title', '3': 4, '4': 1, '5': 9, '10': 'noticeTitle'},
    {'1': 'notice_body', '3': 5, '4': 1, '5': 9, '10': 'noticeBody'},
  ],
};

/// Descriptor for `CheckAppVersionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkAppVersionResponseDescriptor = $convert.base64Decode(
    'ChdDaGVja0FwcFZlcnNpb25SZXNwb25zZRInCg91cGRhdGVfcmVxdWlyZWQYASABKAhSDnVwZG'
    'F0ZVJlcXVpcmVkEicKD21pbmltdW1fdmVyc2lvbhgCIAEoCVIObWluaW11bVZlcnNpb24SGwoJ'
    'c3RvcmVfdXJsGAMgASgJUghzdG9yZVVybBIhCgxub3RpY2VfdGl0bGUYBCABKAlSC25vdGljZV'
    'RpdGxlEh8KC25vdGljZV9ib2R5GAUgASgJUgpub3RpY2VCb2R5');

const $core.Map<$core.String, $core.dynamic> ContentPublicServiceBase$json = {
  '1': 'ContentPublicService',
  '2': [
    {
      '1': 'ListLiveBanners',
      '2': '.hawkr.content.v1alpha1.ListLiveBannersRequest',
      '3': '.hawkr.content.v1alpha1.ListLiveBannersResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CheckAppVersion',
      '2': '.hawkr.content.v1alpha1.CheckAppVersionRequest',
      '3': '.hawkr.content.v1alpha1.CheckAppVersionResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use contentPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    ContentPublicServiceBase$messageJson = {
  '.hawkr.content.v1alpha1.ListLiveBannersRequest': ListLiveBannersRequest$json,
  '.hawkr.content.v1alpha1.ListLiveBannersResponse':
      ListLiveBannersResponse$json,
  '.hawkr.content.v1alpha1.Banner': Banner$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.hawkr.content.v1alpha1.BannerAction': BannerAction$json,
  '.hawkr.content.v1alpha1.BannerAction.Popup': BannerAction_Popup$json,
  '.hawkr.content.v1alpha1.CheckAppVersionRequest': CheckAppVersionRequest$json,
  '.hawkr.content.v1alpha1.CheckAppVersionResponse':
      CheckAppVersionResponse$json,
};

/// Descriptor for `ContentPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List contentPublicServiceDescriptor = $convert.base64Decode(
    'ChRDb250ZW50UHVibGljU2VydmljZRJ3Cg9MaXN0TGl2ZUJhbm5lcnMSLi5oYXdrci5jb250ZW'
    '50LnYxYWxwaGExLkxpc3RMaXZlQmFubmVyc1JlcXVlc3QaLy5oYXdrci5jb250ZW50LnYxYWxw'
    'aGExLkxpc3RMaXZlQmFubmVyc1Jlc3BvbnNlIgOQAgESdwoPQ2hlY2tBcHBWZXJzaW9uEi4uaG'
    'F3a3IuY29udGVudC52MWFscGhhMS5DaGVja0FwcFZlcnNpb25SZXF1ZXN0Gi8uaGF3a3IuY29u'
    'dGVudC52MWFscGhhMS5DaGVja0FwcFZlcnNpb25SZXNwb25zZSIDkAIB');
