// This is a generated file - do not edit.
//
// Generated from hawkr/notification/v1alpha1/notification.proto.

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

@$core.Deprecated('Use notificationAudienceDescriptor instead')
const NotificationAudience$json = {
  '1': 'NotificationAudience',
  '2': [
    {'1': 'NOTIFICATION_AUDIENCE_UNSPECIFIED', '2': 0},
    {'1': 'NOTIFICATION_AUDIENCE_BUYER', '2': 1},
    {'1': 'NOTIFICATION_AUDIENCE_STORE', '2': 2},
  ],
};

/// Descriptor for `NotificationAudience`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List notificationAudienceDescriptor = $convert.base64Decode(
    'ChROb3RpZmljYXRpb25BdWRpZW5jZRIlCiFOT1RJRklDQVRJT05fQVVESUVOQ0VfVU5TUEVDSU'
    'ZJRUQQABIfChtOT1RJRklDQVRJT05fQVVESUVOQ0VfQlVZRVIQARIfChtOT1RJRklDQVRJT05f'
    'QVVESUVOQ0VfU1RPUkUQAg==');

@$core.Deprecated('Use notificationCategoryDescriptor instead')
const NotificationCategory$json = {
  '1': 'NotificationCategory',
  '2': [
    {'1': 'NOTIFICATION_CATEGORY_UNSPECIFIED', '2': 0},
    {'1': 'NOTIFICATION_CATEGORY_ORDER', '2': 1},
    {'1': 'NOTIFICATION_CATEGORY_SHIPMENT', '2': 2},
    {'1': 'NOTIFICATION_CATEGORY_SHIPMENT_ANOMALY', '2': 3},
    {'1': 'NOTIFICATION_CATEGORY_AFTERSALES', '2': 4},
    {'1': 'NOTIFICATION_CATEGORY_PAYOUT', '2': 5},
    {'1': 'NOTIFICATION_CATEGORY_ACCOUNT', '2': 6},
    {'1': 'NOTIFICATION_CATEGORY_CHAT', '2': 7},
    {'1': 'NOTIFICATION_CATEGORY_REVIEW', '2': 8},
    {'1': 'NOTIFICATION_CATEGORY_COMMENT', '2': 9},
    {'1': 'NOTIFICATION_CATEGORY_LIKE', '2': 10},
    {'1': 'NOTIFICATION_CATEGORY_FOLLOW', '2': 11},
    {'1': 'NOTIFICATION_CATEGORY_FOLLOWED_STORE_POST', '2': 12},
    {'1': 'NOTIFICATION_CATEGORY_COUPON', '2': 13},
    {'1': 'NOTIFICATION_CATEGORY_RESTOCK', '2': 14},
  ],
};

/// Descriptor for `NotificationCategory`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List notificationCategoryDescriptor = $convert.base64Decode(
    'ChROb3RpZmljYXRpb25DYXRlZ29yeRIlCiFOT1RJRklDQVRJT05fQ0FURUdPUllfVU5TUEVDSU'
    'ZJRUQQABIfChtOT1RJRklDQVRJT05fQ0FURUdPUllfT1JERVIQARIiCh5OT1RJRklDQVRJT05f'
    'Q0FURUdPUllfU0hJUE1FTlQQAhIqCiZOT1RJRklDQVRJT05fQ0FURUdPUllfU0hJUE1FTlRfQU'
    '5PTUFMWRADEiQKIE5PVElGSUNBVElPTl9DQVRFR09SWV9BRlRFUlNBTEVTEAQSIAocTk9USUZJ'
    'Q0FUSU9OX0NBVEVHT1JZX1BBWU9VVBAFEiEKHU5PVElGSUNBVElPTl9DQVRFR09SWV9BQ0NPVU'
    '5UEAYSHgoaTk9USUZJQ0FUSU9OX0NBVEVHT1JZX0NIQVQQBxIgChxOT1RJRklDQVRJT05fQ0FU'
    'RUdPUllfUkVWSUVXEAgSIQodTk9USUZJQ0FUSU9OX0NBVEVHT1JZX0NPTU1FTlQQCRIeChpOT1'
    'RJRklDQVRJT05fQ0FURUdPUllfTElLRRAKEiAKHE5PVElGSUNBVElPTl9DQVRFR09SWV9GT0xM'
    'T1cQCxItCilOT1RJRklDQVRJT05fQ0FURUdPUllfRk9MTE9XRURfU1RPUkVfUE9TVBAMEiAKHE'
    '5PVElGSUNBVElPTl9DQVRFR09SWV9DT1VQT04QDRIhCh1OT1RJRklDQVRJT05fQ0FURUdPUllf'
    'UkVTVE9DSxAO');

@$core.Deprecated('Use notificationChannelDescriptor instead')
const NotificationChannel$json = {
  '1': 'NotificationChannel',
  '2': [
    {'1': 'NOTIFICATION_CHANNEL_UNSPECIFIED', '2': 0},
    {'1': 'NOTIFICATION_CHANNEL_INBOX', '2': 1},
    {'1': 'NOTIFICATION_CHANNEL_PUSH', '2': 2},
    {'1': 'NOTIFICATION_CHANNEL_EMAIL', '2': 3},
  ],
};

/// Descriptor for `NotificationChannel`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List notificationChannelDescriptor = $convert.base64Decode(
    'ChNOb3RpZmljYXRpb25DaGFubmVsEiQKIE5PVElGSUNBVElPTl9DSEFOTkVMX1VOU1BFQ0lGSU'
    'VEEAASHgoaTk9USUZJQ0FUSU9OX0NIQU5ORUxfSU5CT1gQARIdChlOT1RJRklDQVRJT05fQ0hB'
    'Tk5FTF9QVVNIEAISHgoaTk9USUZJQ0FUSU9OX0NIQU5ORUxfRU1BSUwQAw==');

@$core.Deprecated('Use inboxPresentationDescriptor instead')
const InboxPresentation$json = {
  '1': 'InboxPresentation',
  '2': [
    {'1': 'INBOX_PRESENTATION_UNSPECIFIED', '2': 0},
    {'1': 'INBOX_PRESENTATION_ORDER', '2': 1},
    {'1': 'INBOX_PRESENTATION_REFUND', '2': 2},
    {'1': 'INBOX_PRESENTATION_PAYOUT', '2': 3},
    {'1': 'INBOX_PRESENTATION_MESSAGE', '2': 4},
    {'1': 'INBOX_PRESENTATION_SOCIAL', '2': 5},
    {'1': 'INBOX_PRESENTATION_REVIEW', '2': 6},
    {'1': 'INBOX_PRESENTATION_PROMOTION', '2': 7},
    {'1': 'INBOX_PRESENTATION_PROCUREMENT', '2': 8},
    {'1': 'INBOX_PRESENTATION_ACCOUNT', '2': 9},
    {'1': 'INBOX_PRESENTATION_SYSTEM', '2': 10},
  ],
};

/// Descriptor for `InboxPresentation`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List inboxPresentationDescriptor = $convert.base64Decode(
    'ChFJbmJveFByZXNlbnRhdGlvbhIiCh5JTkJPWF9QUkVTRU5UQVRJT05fVU5TUEVDSUZJRUQQAB'
    'IcChhJTkJPWF9QUkVTRU5UQVRJT05fT1JERVIQARIdChlJTkJPWF9QUkVTRU5UQVRJT05fUkVG'
    'VU5EEAISHQoZSU5CT1hfUFJFU0VOVEFUSU9OX1BBWU9VVBADEh4KGklOQk9YX1BSRVNFTlRBVE'
    'lPTl9NRVNTQUdFEAQSHQoZSU5CT1hfUFJFU0VOVEFUSU9OX1NPQ0lBTBAFEh0KGUlOQk9YX1BS'
    'RVNFTlRBVElPTl9SRVZJRVcQBhIgChxJTkJPWF9QUkVTRU5UQVRJT05fUFJPTU9USU9OEAcSIg'
    'oeSU5CT1hfUFJFU0VOVEFUSU9OX1BST0NVUkVNRU5UEAgSHgoaSU5CT1hfUFJFU0VOVEFUSU9O'
    'X0FDQ09VTlQQCRIdChlJTkJPWF9QUkVTRU5UQVRJT05fU1lTVEVNEAo=');

@$core.Deprecated('Use stateRefreshHintDescriptor instead')
const StateRefreshHint$json = {
  '1': 'StateRefreshHint',
  '2': [
    {'1': 'STATE_REFRESH_HINT_UNSPECIFIED', '2': 0},
    {'1': 'STATE_REFRESH_HINT_ORDER', '2': 1},
    {'1': 'STATE_REFRESH_HINT_AFTER_SALE', '2': 2},
    {'1': 'STATE_REFRESH_HINT_SELLER_STATUS', '2': 3},
  ],
};

/// Descriptor for `StateRefreshHint`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List stateRefreshHintDescriptor = $convert.base64Decode(
    'ChBTdGF0ZVJlZnJlc2hIaW50EiIKHlNUQVRFX1JFRlJFU0hfSElOVF9VTlNQRUNJRklFRBAAEh'
    'wKGFNUQVRFX1JFRlJFU0hfSElOVF9PUkRFUhABEiEKHVNUQVRFX1JFRlJFU0hfSElOVF9BRlRF'
    'Ul9TQUxFEAISJAogU1RBVEVfUkVGUkVTSF9ISU5UX1NFTExFUl9TVEFUVVMQAw==');

@$core.Deprecated('Use notificationAppDescriptor instead')
const NotificationApp$json = {
  '1': 'NotificationApp',
  '2': [
    {'1': 'NOTIFICATION_APP_UNSPECIFIED', '2': 0},
    {'1': 'NOTIFICATION_APP_BUYER_APP', '2': 1},
    {'1': 'NOTIFICATION_APP_SELLER_APP', '2': 2},
  ],
};

/// Descriptor for `NotificationApp`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List notificationAppDescriptor = $convert.base64Decode(
    'Cg9Ob3RpZmljYXRpb25BcHASIAocTk9USUZJQ0FUSU9OX0FQUF9VTlNQRUNJRklFRBAAEh4KGk'
    '5PVElGSUNBVElPTl9BUFBfQlVZRVJfQVBQEAESHwobTk9USUZJQ0FUSU9OX0FQUF9TRUxMRVJf'
    'QVBQEAI=');

@$core.Deprecated('Use sellerWorkspaceChangeDescriptor instead')
const SellerWorkspaceChange$json = {
  '1': 'SellerWorkspaceChange',
  '2': [
    {'1': 'SELLER_WORKSPACE_CHANGE_UNSPECIFIED', '2': 0},
    {'1': 'SELLER_WORKSPACE_CHANGE_ORDER', '2': 1},
    {'1': 'SELLER_WORKSPACE_CHANGE_AFTER_SALE', '2': 2},
  ],
};

/// Descriptor for `SellerWorkspaceChange`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sellerWorkspaceChangeDescriptor = $convert.base64Decode(
    'ChVTZWxsZXJXb3Jrc3BhY2VDaGFuZ2USJwojU0VMTEVSX1dPUktTUEFDRV9DSEFOR0VfVU5TUE'
    'VDSUZJRUQQABIhCh1TRUxMRVJfV09SS1NQQUNFX0NIQU5HRV9PUkRFUhABEiYKIlNFTExFUl9X'
    'T1JLU1BBQ0VfQ0hBTkdFX0FGVEVSX1NBTEUQAg==');

@$core.Deprecated('Use inboxItemDescriptor instead')
const InboxItem$json = {
  '1': 'InboxItem',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'category',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationCategory',
      '10': 'category'
    },
    {
      '1': 'presentation',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.InboxPresentation',
      '10': 'presentation'
    },
    {'1': 'title', '3': 4, '4': 1, '5': 9, '10': 'title'},
    {'1': 'body', '3': 5, '4': 1, '5': 9, '10': 'body'},
    {'1': 'deep_link', '3': 6, '4': 1, '5': 9, '10': 'deepLink'},
    {
      '1': 'image',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '10': 'image'
    },
    {'1': 'read', '3': 8, '4': 1, '5': 8, '10': 'read'},
    {
      '1': 'read_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'readTime',
      '17': true
    },
    {
      '1': 'create_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
  ],
  '8': [
    {'1': '_read_time'},
  ],
};

/// Descriptor for `InboxItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List inboxItemDescriptor = $convert.base64Decode(
    'CglJbmJveEl0ZW0SEwoCaWQYASABKAlCA+BBCFICaWQSTQoIY2F0ZWdvcnkYAiABKA4yMS5oYX'
    'drci5ub3RpZmljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW9uQ2F0ZWdvcnlSCGNhdGVnb3J5'
    'ElIKDHByZXNlbnRhdGlvbhgDIAEoDjIuLmhhd2tyLm5vdGlmaWNhdGlvbi52MWFscGhhMS5Jbm'
    'JveFByZXNlbnRhdGlvblIMcHJlc2VudGF0aW9uEhQKBXRpdGxlGAQgASgJUgV0aXRsZRISCgRi'
    'b2R5GAUgASgJUgRib2R5EhsKCWRlZXBfbGluaxgGIAEoCVIIZGVlcExpbmsSMgoFaW1hZ2UYBy'
    'ABKAsyHC5oYXdrci5jb21tb24udjFhbHBoYTEuSW1hZ2VSBWltYWdlEhIKBHJlYWQYCCABKAhS'
    'BHJlYWQSPAoJcmVhZF90aW1lGAkgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgAUg'
    'hyZWFkVGltZYgBARI7CgtjcmVhdGVfdGltZRgKIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1l'
    'c3RhbXBSCmNyZWF0ZVRpbWVCDAoKX3JlYWRfdGltZQ==');

@$core.Deprecated('Use listInboxItemsRequestDescriptor instead')
const ListInboxItemsRequest$json = {
  '1': 'ListInboxItemsRequest',
  '2': [
    {
      '1': 'audience',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationAudience',
      '8': {},
      '10': 'audience'
    },
    {
      '1': 'category',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationCategory',
      '9': 0,
      '10': 'category',
      '17': true
    },
    {'1': 'page_size', '3': 3, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 4, '4': 1, '5': 9, '10': 'pageToken'},
  ],
  '8': [
    {'1': '_category'},
  ],
};

/// Descriptor for `ListInboxItemsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInboxItemsRequestDescriptor = $convert.base64Decode(
    'ChVMaXN0SW5ib3hJdGVtc1JlcXVlc3QSUgoIYXVkaWVuY2UYASABKA4yMS5oYXdrci5ub3RpZm'
    'ljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW9uQXVkaWVuY2VCA+BBAlIIYXVkaWVuY2USUgoI'
    'Y2F0ZWdvcnkYAiABKA4yMS5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW'
    '9uQ2F0ZWdvcnlIAFIIY2F0ZWdvcnmIAQESGwoJcGFnZV9zaXplGAMgASgFUghwYWdlU2l6ZRId'
    'CgpwYWdlX3Rva2VuGAQgASgJUglwYWdlVG9rZW5CCwoJX2NhdGVnb3J5');

@$core.Deprecated('Use listInboxItemsResponseDescriptor instead')
const ListInboxItemsResponse$json = {
  '1': 'ListInboxItemsResponse',
  '2': [
    {
      '1': 'items',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.notification.v1alpha1.InboxItem',
      '10': 'items'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
    {'1': 'total_size', '3': 3, '4': 1, '5': 5, '10': 'totalSize'},
  ],
};

/// Descriptor for `ListInboxItemsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInboxItemsResponseDescriptor = $convert.base64Decode(
    'ChZMaXN0SW5ib3hJdGVtc1Jlc3BvbnNlEjwKBWl0ZW1zGAEgAygLMiYuaGF3a3Iubm90aWZpY2'
    'F0aW9uLnYxYWxwaGExLkluYm94SXRlbVIFaXRlbXMSJgoPbmV4dF9wYWdlX3Rva2VuGAIgASgJ'
    'Ug1uZXh0UGFnZVRva2VuEh0KCnRvdGFsX3NpemUYAyABKAVSCXRvdGFsU2l6ZQ==');

@$core.Deprecated('Use getUnreadCountRequestDescriptor instead')
const GetUnreadCountRequest$json = {
  '1': 'GetUnreadCountRequest',
  '2': [
    {
      '1': 'audience',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationAudience',
      '8': {},
      '10': 'audience'
    },
  ],
};

/// Descriptor for `GetUnreadCountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUnreadCountRequestDescriptor = $convert.base64Decode(
    'ChVHZXRVbnJlYWRDb3VudFJlcXVlc3QSUgoIYXVkaWVuY2UYASABKA4yMS5oYXdrci5ub3RpZm'
    'ljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW9uQXVkaWVuY2VCA+BBAlIIYXVkaWVuY2U=');

@$core.Deprecated('Use getUnreadCountResponseDescriptor instead')
const GetUnreadCountResponse$json = {
  '1': 'GetUnreadCountResponse',
  '2': [
    {'1': 'unread_count', '3': 1, '4': 1, '5': 5, '10': 'unreadCount'},
  ],
};

/// Descriptor for `GetUnreadCountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUnreadCountResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRVbnJlYWRDb3VudFJlc3BvbnNlEiEKDHVucmVhZF9jb3VudBgBIAEoBVILdW5yZWFkQ2'
        '91bnQ=');

@$core.Deprecated('Use markInboxItemReadRequestDescriptor instead')
const MarkInboxItemReadRequest$json = {
  '1': 'MarkInboxItemReadRequest',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
  ],
};

/// Descriptor for `MarkInboxItemReadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markInboxItemReadRequestDescriptor =
    $convert.base64Decode(
        'ChhNYXJrSW5ib3hJdGVtUmVhZFJlcXVlc3QSEwoCaWQYASABKAlCA+BBAlICaWQ=');

@$core.Deprecated('Use markInboxItemReadResponseDescriptor instead')
const MarkInboxItemReadResponse$json = {
  '1': 'MarkInboxItemReadResponse',
};

/// Descriptor for `MarkInboxItemReadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markInboxItemReadResponseDescriptor =
    $convert.base64Decode('ChlNYXJrSW5ib3hJdGVtUmVhZFJlc3BvbnNl');

@$core.Deprecated('Use markAllInboxItemsReadRequestDescriptor instead')
const MarkAllInboxItemsReadRequest$json = {
  '1': 'MarkAllInboxItemsReadRequest',
  '2': [
    {
      '1': 'audience',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationAudience',
      '8': {},
      '10': 'audience'
    },
  ],
};

/// Descriptor for `MarkAllInboxItemsReadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markAllInboxItemsReadRequestDescriptor =
    $convert.base64Decode(
        'ChxNYXJrQWxsSW5ib3hJdGVtc1JlYWRSZXF1ZXN0ElIKCGF1ZGllbmNlGAEgASgOMjEuaGF3a3'
        'Iubm90aWZpY2F0aW9uLnYxYWxwaGExLk5vdGlmaWNhdGlvbkF1ZGllbmNlQgPgQQJSCGF1ZGll'
        'bmNl');

@$core.Deprecated('Use markAllInboxItemsReadResponseDescriptor instead')
const MarkAllInboxItemsReadResponse$json = {
  '1': 'MarkAllInboxItemsReadResponse',
  '2': [
    {'1': 'updated_count', '3': 1, '4': 1, '5': 5, '10': 'updatedCount'},
  ],
};

/// Descriptor for `MarkAllInboxItemsReadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markAllInboxItemsReadResponseDescriptor =
    $convert.base64Decode(
        'Ch1NYXJrQWxsSW5ib3hJdGVtc1JlYWRSZXNwb25zZRIjCg11cGRhdGVkX2NvdW50GAEgASgFUg'
        'x1cGRhdGVkQ291bnQ=');

@$core.Deprecated('Use deleteInboxItemsRequestDescriptor instead')
const DeleteInboxItemsRequest$json = {
  '1': 'DeleteInboxItemsRequest',
  '2': [
    {'1': 'ids', '3': 1, '4': 3, '5': 9, '8': {}, '10': 'ids'},
  ],
};

/// Descriptor for `DeleteInboxItemsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteInboxItemsRequestDescriptor =
    $convert.base64Decode(
        'ChdEZWxldGVJbmJveEl0ZW1zUmVxdWVzdBIVCgNpZHMYASADKAlCA+BBAlIDaWRz');

@$core.Deprecated('Use deleteInboxItemsResponseDescriptor instead')
const DeleteInboxItemsResponse$json = {
  '1': 'DeleteInboxItemsResponse',
  '2': [
    {'1': 'deleted_count', '3': 1, '4': 1, '5': 5, '10': 'deletedCount'},
  ],
};

/// Descriptor for `DeleteInboxItemsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteInboxItemsResponseDescriptor =
    $convert.base64Decode(
        'ChhEZWxldGVJbmJveEl0ZW1zUmVzcG9uc2USIwoNZGVsZXRlZF9jb3VudBgBIAEoBVIMZGVsZX'
        'RlZENvdW50');

@$core.Deprecated('Use preferenceEntryDescriptor instead')
const PreferenceEntry$json = {
  '1': 'PreferenceEntry',
  '2': [
    {
      '1': 'category',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationCategory',
      '10': 'category'
    },
    {
      '1': 'channel',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationChannel',
      '10': 'channel'
    },
    {'1': 'enabled', '3': 3, '4': 1, '5': 8, '10': 'enabled'},
    {'1': 'can_disable', '3': 4, '4': 1, '5': 8, '10': 'canDisable'},
  ],
};

/// Descriptor for `PreferenceEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List preferenceEntryDescriptor = $convert.base64Decode(
    'Cg9QcmVmZXJlbmNlRW50cnkSTQoIY2F0ZWdvcnkYASABKA4yMS5oYXdrci5ub3RpZmljYXRpb2'
    '4udjFhbHBoYTEuTm90aWZpY2F0aW9uQ2F0ZWdvcnlSCGNhdGVnb3J5EkoKB2NoYW5uZWwYAiAB'
    'KA4yMC5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW9uQ2hhbm5lbFIHY2'
    'hhbm5lbBIYCgdlbmFibGVkGAMgASgIUgdlbmFibGVkEh8KC2Nhbl9kaXNhYmxlGAQgASgIUgpj'
    'YW5EaXNhYmxl');

@$core.Deprecated('Use getNotificationPreferencesRequestDescriptor instead')
const GetNotificationPreferencesRequest$json = {
  '1': 'GetNotificationPreferencesRequest',
  '2': [
    {
      '1': 'audience',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationAudience',
      '8': {},
      '10': 'audience'
    },
  ],
};

/// Descriptor for `GetNotificationPreferencesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getNotificationPreferencesRequestDescriptor =
    $convert.base64Decode(
        'CiFHZXROb3RpZmljYXRpb25QcmVmZXJlbmNlc1JlcXVlc3QSUgoIYXVkaWVuY2UYASABKA4yMS'
        '5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW9uQXVkaWVuY2VCA+BBAlII'
        'YXVkaWVuY2U=');

@$core.Deprecated('Use getNotificationPreferencesResponseDescriptor instead')
const GetNotificationPreferencesResponse$json = {
  '1': 'GetNotificationPreferencesResponse',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.notification.v1alpha1.PreferenceEntry',
      '10': 'entries'
    },
  ],
};

/// Descriptor for `GetNotificationPreferencesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getNotificationPreferencesResponseDescriptor =
    $convert.base64Decode(
        'CiJHZXROb3RpZmljYXRpb25QcmVmZXJlbmNlc1Jlc3BvbnNlEkYKB2VudHJpZXMYASADKAsyLC'
        '5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuUHJlZmVyZW5jZUVudHJ5UgdlbnRyaWVz');

@$core.Deprecated('Use updateNotificationPreferenceRequestDescriptor instead')
const UpdateNotificationPreferenceRequest$json = {
  '1': 'UpdateNotificationPreferenceRequest',
  '2': [
    {
      '1': 'audience',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationAudience',
      '8': {},
      '10': 'audience'
    },
    {
      '1': 'category',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationCategory',
      '8': {},
      '10': 'category'
    },
    {
      '1': 'channel',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationChannel',
      '8': {},
      '10': 'channel'
    },
    {'1': 'enabled', '3': 4, '4': 1, '5': 8, '10': 'enabled'},
  ],
};

/// Descriptor for `UpdateNotificationPreferenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateNotificationPreferenceRequestDescriptor = $convert.base64Decode(
    'CiNVcGRhdGVOb3RpZmljYXRpb25QcmVmZXJlbmNlUmVxdWVzdBJSCghhdWRpZW5jZRgBIAEoDj'
    'IxLmhhd2tyLm5vdGlmaWNhdGlvbi52MWFscGhhMS5Ob3RpZmljYXRpb25BdWRpZW5jZUID4EEC'
    'UghhdWRpZW5jZRJSCghjYXRlZ29yeRgCIAEoDjIxLmhhd2tyLm5vdGlmaWNhdGlvbi52MWFscG'
    'hhMS5Ob3RpZmljYXRpb25DYXRlZ29yeUID4EECUghjYXRlZ29yeRJPCgdjaGFubmVsGAMgASgO'
    'MjAuaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLk5vdGlmaWNhdGlvbkNoYW5uZWxCA+BBAl'
    'IHY2hhbm5lbBIYCgdlbmFibGVkGAQgASgIUgdlbmFibGVk');

@$core.Deprecated('Use updateNotificationPreferenceResponseDescriptor instead')
const UpdateNotificationPreferenceResponse$json = {
  '1': 'UpdateNotificationPreferenceResponse',
  '2': [
    {
      '1': 'entry',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.notification.v1alpha1.PreferenceEntry',
      '10': 'entry'
    },
  ],
};

/// Descriptor for `UpdateNotificationPreferenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateNotificationPreferenceResponseDescriptor =
    $convert.base64Decode(
        'CiRVcGRhdGVOb3RpZmljYXRpb25QcmVmZXJlbmNlUmVzcG9uc2USQgoFZW50cnkYASABKAsyLC'
        '5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuUHJlZmVyZW5jZUVudHJ5UgVlbnRyeQ==');

@$core.Deprecated('Use registerDeviceTokenRequestDescriptor instead')
const RegisterDeviceTokenRequest$json = {
  '1': 'RegisterDeviceTokenRequest',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'token'},
    {
      '1': 'app',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationApp',
      '8': {},
      '10': 'app'
    },
  ],
};

/// Descriptor for `RegisterDeviceTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerDeviceTokenRequestDescriptor =
    $convert.base64Decode(
        'ChpSZWdpc3RlckRldmljZVRva2VuUmVxdWVzdBIZCgV0b2tlbhgBIAEoCUID4EECUgV0b2tlbh'
        'JDCgNhcHAYAiABKA4yLC5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuTm90aWZpY2F0aW9u'
        'QXBwQgPgQQJSA2FwcA==');

@$core.Deprecated('Use registerDeviceTokenResponseDescriptor instead')
const RegisterDeviceTokenResponse$json = {
  '1': 'RegisterDeviceTokenResponse',
};

/// Descriptor for `RegisterDeviceTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerDeviceTokenResponseDescriptor =
    $convert.base64Decode('ChtSZWdpc3RlckRldmljZVRva2VuUmVzcG9uc2U=');

@$core.Deprecated('Use deleteDeviceTokenRequestDescriptor instead')
const DeleteDeviceTokenRequest$json = {
  '1': 'DeleteDeviceTokenRequest',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'token'},
  ],
};

/// Descriptor for `DeleteDeviceTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteDeviceTokenRequestDescriptor =
    $convert.base64Decode(
        'ChhEZWxldGVEZXZpY2VUb2tlblJlcXVlc3QSGQoFdG9rZW4YASABKAlCA+BBAlIFdG9rZW4=');

@$core.Deprecated('Use deleteDeviceTokenResponseDescriptor instead')
const DeleteDeviceTokenResponse$json = {
  '1': 'DeleteDeviceTokenResponse',
};

/// Descriptor for `DeleteDeviceTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteDeviceTokenResponseDescriptor =
    $convert.base64Decode('ChlEZWxldGVEZXZpY2VUb2tlblJlc3BvbnNl');

@$core.Deprecated('Use watchNotificationsRequestDescriptor instead')
const WatchNotificationsRequest$json = {
  '1': 'WatchNotificationsRequest',
  '2': [
    {
      '1': 'audience',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.NotificationAudience',
      '8': {},
      '10': 'audience'
    },
  ],
};

/// Descriptor for `WatchNotificationsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchNotificationsRequestDescriptor = $convert.base64Decode(
    'ChlXYXRjaE5vdGlmaWNhdGlvbnNSZXF1ZXN0ElIKCGF1ZGllbmNlGAEgASgOMjEuaGF3a3Iubm'
    '90aWZpY2F0aW9uLnYxYWxwaGExLk5vdGlmaWNhdGlvbkF1ZGllbmNlQgPgQQJSCGF1ZGllbmNl');

@$core.Deprecated('Use watchNotificationsResponseDescriptor instead')
const WatchNotificationsResponse$json = {
  '1': 'WatchNotificationsResponse',
  '2': [
    {
      '1': 'inbox',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.notification.v1alpha1.InboxSignal',
      '9': 0,
      '10': 'inbox'
    },
    {
      '1': 'seller_workspace',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.notification.v1alpha1.SellerWorkspaceSignal',
      '9': 0,
      '10': 'sellerWorkspace'
    },
  ],
  '8': [
    {'1': 'event'},
  ],
};

/// Descriptor for `WatchNotificationsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchNotificationsResponseDescriptor = $convert.base64Decode(
    'ChpXYXRjaE5vdGlmaWNhdGlvbnNSZXNwb25zZRJACgVpbmJveBgBIAEoCzIoLmhhd2tyLm5vdG'
    'lmaWNhdGlvbi52MWFscGhhMS5JbmJveFNpZ25hbEgAUgVpbmJveBJfChBzZWxsZXJfd29ya3Nw'
    'YWNlGAIgASgLMjIuaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLlNlbGxlcldvcmtzcGFjZV'
    'NpZ25hbEgAUg9zZWxsZXJXb3Jrc3BhY2VCBwoFZXZlbnQ=');

@$core.Deprecated('Use inboxSignalDescriptor instead')
const InboxSignal$json = {
  '1': 'InboxSignal',
  '2': [
    {'1': 'unread_count', '3': 1, '4': 1, '5': 5, '10': 'unreadCount'},
    {
      '1': 'latest_inbox_item_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'latestInboxItemId',
      '17': true
    },
    {
      '1': 'state_refresh_hint',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.StateRefreshHint',
      '10': 'stateRefreshHint'
    },
  ],
  '8': [
    {'1': '_latest_inbox_item_id'},
  ],
};

/// Descriptor for `InboxSignal`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List inboxSignalDescriptor = $convert.base64Decode(
    'CgtJbmJveFNpZ25hbBIhCgx1bnJlYWRfY291bnQYASABKAVSC3VucmVhZENvdW50EjQKFGxhdG'
    'VzdF9pbmJveF9pdGVtX2lkGAIgASgJSABSEWxhdGVzdEluYm94SXRlbUlkiAEBElsKEnN0YXRl'
    'X3JlZnJlc2hfaGludBgDIAEoDjItLmhhd2tyLm5vdGlmaWNhdGlvbi52MWFscGhhMS5TdGF0ZV'
    'JlZnJlc2hIaW50UhBzdGF0ZVJlZnJlc2hIaW50QhcKFV9sYXRlc3RfaW5ib3hfaXRlbV9pZA==');

@$core.Deprecated('Use sellerWorkspaceSignalDescriptor instead')
const SellerWorkspaceSignal$json = {
  '1': 'SellerWorkspaceSignal',
  '2': [
    {
      '1': 'change',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.notification.v1alpha1.SellerWorkspaceChange',
      '10': 'change'
    },
  ],
};

/// Descriptor for `SellerWorkspaceSignal`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sellerWorkspaceSignalDescriptor = $convert.base64Decode(
    'ChVTZWxsZXJXb3Jrc3BhY2VTaWduYWwSSgoGY2hhbmdlGAEgASgOMjIuaGF3a3Iubm90aWZpY2'
    'F0aW9uLnYxYWxwaGExLlNlbGxlcldvcmtzcGFjZUNoYW5nZVIGY2hhbmdl');

const $core.Map<$core.String, $core.dynamic> NotificationAppServiceBase$json = {
  '1': 'NotificationAppService',
  '2': [
    {
      '1': 'ListInboxItems',
      '2': '.hawkr.notification.v1alpha1.ListInboxItemsRequest',
      '3': '.hawkr.notification.v1alpha1.ListInboxItemsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetUnreadCount',
      '2': '.hawkr.notification.v1alpha1.GetUnreadCountRequest',
      '3': '.hawkr.notification.v1alpha1.GetUnreadCountResponse',
      '4': {'34': 1},
    },
    {
      '1': 'MarkInboxItemRead',
      '2': '.hawkr.notification.v1alpha1.MarkInboxItemReadRequest',
      '3': '.hawkr.notification.v1alpha1.MarkInboxItemReadResponse'
    },
    {
      '1': 'MarkAllInboxItemsRead',
      '2': '.hawkr.notification.v1alpha1.MarkAllInboxItemsReadRequest',
      '3': '.hawkr.notification.v1alpha1.MarkAllInboxItemsReadResponse'
    },
    {
      '1': 'DeleteInboxItems',
      '2': '.hawkr.notification.v1alpha1.DeleteInboxItemsRequest',
      '3': '.hawkr.notification.v1alpha1.DeleteInboxItemsResponse'
    },
    {
      '1': 'GetNotificationPreferences',
      '2': '.hawkr.notification.v1alpha1.GetNotificationPreferencesRequest',
      '3': '.hawkr.notification.v1alpha1.GetNotificationPreferencesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'UpdateNotificationPreference',
      '2': '.hawkr.notification.v1alpha1.UpdateNotificationPreferenceRequest',
      '3': '.hawkr.notification.v1alpha1.UpdateNotificationPreferenceResponse'
    },
    {
      '1': 'RegisterDeviceToken',
      '2': '.hawkr.notification.v1alpha1.RegisterDeviceTokenRequest',
      '3': '.hawkr.notification.v1alpha1.RegisterDeviceTokenResponse'
    },
    {
      '1': 'DeleteDeviceToken',
      '2': '.hawkr.notification.v1alpha1.DeleteDeviceTokenRequest',
      '3': '.hawkr.notification.v1alpha1.DeleteDeviceTokenResponse'
    },
    {
      '1': 'WatchNotifications',
      '2': '.hawkr.notification.v1alpha1.WatchNotificationsRequest',
      '3': '.hawkr.notification.v1alpha1.WatchNotificationsResponse',
      '6': true
    },
  ],
};

@$core.Deprecated('Use notificationAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    NotificationAppServiceBase$messageJson = {
  '.hawkr.notification.v1alpha1.ListInboxItemsRequest':
      ListInboxItemsRequest$json,
  '.hawkr.notification.v1alpha1.ListInboxItemsResponse':
      ListInboxItemsResponse$json,
  '.hawkr.notification.v1alpha1.InboxItem': InboxItem$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.notification.v1alpha1.GetUnreadCountRequest':
      GetUnreadCountRequest$json,
  '.hawkr.notification.v1alpha1.GetUnreadCountResponse':
      GetUnreadCountResponse$json,
  '.hawkr.notification.v1alpha1.MarkInboxItemReadRequest':
      MarkInboxItemReadRequest$json,
  '.hawkr.notification.v1alpha1.MarkInboxItemReadResponse':
      MarkInboxItemReadResponse$json,
  '.hawkr.notification.v1alpha1.MarkAllInboxItemsReadRequest':
      MarkAllInboxItemsReadRequest$json,
  '.hawkr.notification.v1alpha1.MarkAllInboxItemsReadResponse':
      MarkAllInboxItemsReadResponse$json,
  '.hawkr.notification.v1alpha1.DeleteInboxItemsRequest':
      DeleteInboxItemsRequest$json,
  '.hawkr.notification.v1alpha1.DeleteInboxItemsResponse':
      DeleteInboxItemsResponse$json,
  '.hawkr.notification.v1alpha1.GetNotificationPreferencesRequest':
      GetNotificationPreferencesRequest$json,
  '.hawkr.notification.v1alpha1.GetNotificationPreferencesResponse':
      GetNotificationPreferencesResponse$json,
  '.hawkr.notification.v1alpha1.PreferenceEntry': PreferenceEntry$json,
  '.hawkr.notification.v1alpha1.UpdateNotificationPreferenceRequest':
      UpdateNotificationPreferenceRequest$json,
  '.hawkr.notification.v1alpha1.UpdateNotificationPreferenceResponse':
      UpdateNotificationPreferenceResponse$json,
  '.hawkr.notification.v1alpha1.RegisterDeviceTokenRequest':
      RegisterDeviceTokenRequest$json,
  '.hawkr.notification.v1alpha1.RegisterDeviceTokenResponse':
      RegisterDeviceTokenResponse$json,
  '.hawkr.notification.v1alpha1.DeleteDeviceTokenRequest':
      DeleteDeviceTokenRequest$json,
  '.hawkr.notification.v1alpha1.DeleteDeviceTokenResponse':
      DeleteDeviceTokenResponse$json,
  '.hawkr.notification.v1alpha1.WatchNotificationsRequest':
      WatchNotificationsRequest$json,
  '.hawkr.notification.v1alpha1.WatchNotificationsResponse':
      WatchNotificationsResponse$json,
  '.hawkr.notification.v1alpha1.InboxSignal': InboxSignal$json,
  '.hawkr.notification.v1alpha1.SellerWorkspaceSignal':
      SellerWorkspaceSignal$json,
};

/// Descriptor for `NotificationAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List notificationAppServiceDescriptor = $convert.base64Decode(
    'ChZOb3RpZmljYXRpb25BcHBTZXJ2aWNlEn4KDkxpc3RJbmJveEl0ZW1zEjIuaGF3a3Iubm90aW'
    'ZpY2F0aW9uLnYxYWxwaGExLkxpc3RJbmJveEl0ZW1zUmVxdWVzdBozLmhhd2tyLm5vdGlmaWNh'
    'dGlvbi52MWFscGhhMS5MaXN0SW5ib3hJdGVtc1Jlc3BvbnNlIgOQAgESfgoOR2V0VW5yZWFkQ2'
    '91bnQSMi5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuR2V0VW5yZWFkQ291bnRSZXF1ZXN0'
    'GjMuaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLkdldFVucmVhZENvdW50UmVzcG9uc2UiA5'
    'ACARKCAQoRTWFya0luYm94SXRlbVJlYWQSNS5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEu'
    'TWFya0luYm94SXRlbVJlYWRSZXF1ZXN0GjYuaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLk'
    '1hcmtJbmJveEl0ZW1SZWFkUmVzcG9uc2USjgEKFU1hcmtBbGxJbmJveEl0ZW1zUmVhZBI5Lmhh'
    'd2tyLm5vdGlmaWNhdGlvbi52MWFscGhhMS5NYXJrQWxsSW5ib3hJdGVtc1JlYWRSZXF1ZXN0Gj'
    'ouaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLk1hcmtBbGxJbmJveEl0ZW1zUmVhZFJlc3Bv'
    'bnNlEn8KEERlbGV0ZUluYm94SXRlbXMSNC5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuRG'
    'VsZXRlSW5ib3hJdGVtc1JlcXVlc3QaNS5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuRGVs'
    'ZXRlSW5ib3hJdGVtc1Jlc3BvbnNlEqIBChpHZXROb3RpZmljYXRpb25QcmVmZXJlbmNlcxI+Lm'
    'hhd2tyLm5vdGlmaWNhdGlvbi52MWFscGhhMS5HZXROb3RpZmljYXRpb25QcmVmZXJlbmNlc1Jl'
    'cXVlc3QaPy5oYXdrci5ub3RpZmljYXRpb24udjFhbHBoYTEuR2V0Tm90aWZpY2F0aW9uUHJlZm'
    'VyZW5jZXNSZXNwb25zZSIDkAIBEqMBChxVcGRhdGVOb3RpZmljYXRpb25QcmVmZXJlbmNlEkAu'
    'aGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLlVwZGF0ZU5vdGlmaWNhdGlvblByZWZlcmVuY2'
    'VSZXF1ZXN0GkEuaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLlVwZGF0ZU5vdGlmaWNhdGlv'
    'blByZWZlcmVuY2VSZXNwb25zZRKIAQoTUmVnaXN0ZXJEZXZpY2VUb2tlbhI3Lmhhd2tyLm5vdG'
    'lmaWNhdGlvbi52MWFscGhhMS5SZWdpc3RlckRldmljZVRva2VuUmVxdWVzdBo4Lmhhd2tyLm5v'
    'dGlmaWNhdGlvbi52MWFscGhhMS5SZWdpc3RlckRldmljZVRva2VuUmVzcG9uc2USggEKEURlbG'
    'V0ZURldmljZVRva2VuEjUuaGF3a3Iubm90aWZpY2F0aW9uLnYxYWxwaGExLkRlbGV0ZURldmlj'
    'ZVRva2VuUmVxdWVzdBo2Lmhhd2tyLm5vdGlmaWNhdGlvbi52MWFscGhhMS5EZWxldGVEZXZpY2'
    'VUb2tlblJlc3BvbnNlEocBChJXYXRjaE5vdGlmaWNhdGlvbnMSNi5oYXdrci5ub3RpZmljYXRp'
    'b24udjFhbHBoYTEuV2F0Y2hOb3RpZmljYXRpb25zUmVxdWVzdBo3Lmhhd2tyLm5vdGlmaWNhdG'
    'lvbi52MWFscGhhMS5XYXRjaE5vdGlmaWNhdGlvbnNSZXNwb25zZTAB');
