// This is a generated file - do not edit.
//
// Generated from hawkr/delegation/v1alpha1/delegation.proto.

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
    as $1;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $2;

import '../../common/v1alpha1/image.pbjson.dart' as $4;
import '../../common/v1alpha1/store_card.pbjson.dart' as $3;
import '../../social/v1alpha1/social.pbjson.dart' as $0;

@$core.Deprecated('Use delegatedOperationDescriptor instead')
const DelegatedOperation$json = {
  '1': 'DelegatedOperation',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'operation_reason',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'operationReason'
    },
    {
      '1': 'notify_seller',
      '3': 3,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'notifySeller',
      '17': true
    },
  ],
  '8': [
    {'1': '_notify_seller'},
  ],
};

/// Descriptor for `DelegatedOperation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List delegatedOperationDescriptor = $convert.base64Decode(
    'ChJEZWxlZ2F0ZWRPcGVyYXRpb24SKgoKcmVxdWVzdF9pZBgBIAEoCUIL4EEC4ozP1wgCCAFSCX'
    'JlcXVlc3RJZBIuChBvcGVyYXRpb25fcmVhc29uGAIgASgJQgPgQQJSD29wZXJhdGlvblJlYXNv'
    'bhIoCg1ub3RpZnlfc2VsbGVyGAMgASgISABSDG5vdGlmeVNlbGxlcogBAUIQCg5fbm90aWZ5X3'
    'NlbGxlcg==');

@$core.Deprecated('Use listPostsRequestDescriptor instead')
const ListPostsRequest$json = {
  '1': 'ListPostsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'status',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.PostStatus',
      '10': 'status'
    },
  ],
};

/// Descriptor for `ListPostsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listPostsRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0UG9zdHNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQoKcGFnZV'
    '90b2tlbhgCIAEoCVIJcGFnZVRva2VuEjkKBnN0YXR1cxgDIAEoDjIhLmhhd2tyLnNvY2lhbC52'
    'MWFscGhhMS5Qb3N0U3RhdHVzUgZzdGF0dXM=');

@$core.Deprecated('Use listPostsResponseDescriptor instead')
const ListPostsResponse$json = {
  '1': 'ListPostsResponse',
  '2': [
    {
      '1': 'posts',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'posts'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListPostsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listPostsResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0UG9zdHNSZXNwb25zZRIxCgVwb3N0cxgBIAMoCzIbLmhhd2tyLnNvY2lhbC52MWFscG'
    'hhMS5Qb3N0UgVwb3N0cxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlSDW5leHRQYWdlVG9rZW4=');

@$core.Deprecated('Use getPostRequestDescriptor instead')
const GetPostRequest$json = {
  '1': 'GetPostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
  ],
};

/// Descriptor for `GetPostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPostRequestDescriptor = $convert.base64Decode(
    'Cg5HZXRQb3N0UmVxdWVzdBIcCgdwb3N0X2lkGAEgASgJQgPgQQJSBnBvc3RJZA==');

@$core.Deprecated('Use getPostResponseDescriptor instead')
const GetPostResponse$json = {
  '1': 'GetPostResponse',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'post'
    },
  ],
};

/// Descriptor for `GetPostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPostResponseDescriptor = $convert.base64Decode(
    'Cg9HZXRQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MWFscGhhMS'
    '5Qb3N0UgRwb3N0');

@$core.Deprecated('Use createPostRequestDescriptor instead')
const CreatePostRequest$json = {
  '1': 'CreatePostRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.delegation.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'body', '3': 2, '4': 1, '5': 9, '10': 'body'},
    {
      '1': 'image_staged_token',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'imageStagedToken'
    },
    {
      '1': 'product_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'productId',
      '17': true
    },
    {'1': 'hashtag_inputs', '3': 5, '4': 3, '5': 9, '10': 'hashtagInputs'},
  ],
  '8': [
    {'1': '_product_id'},
  ],
};

/// Descriptor for `CreatePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPostRequestDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVQb3N0UmVxdWVzdBJQCglvcGVyYXRpb24YASABKAsyLS5oYXdrci5kZWxlZ2F0aW'
    '9uLnYxYWxwaGExLkRlbGVnYXRlZE9wZXJhdGlvbkID4EECUglvcGVyYXRpb24SEgoEYm9keRgC'
    'IAEoCVIEYm9keRIxChJpbWFnZV9zdGFnZWRfdG9rZW4YAyABKAlCA+BBBFIQaW1hZ2VTdGFnZW'
    'RUb2tlbhIiCgpwcm9kdWN0X2lkGAQgASgJSABSCXByb2R1Y3RJZIgBARIlCg5oYXNodGFnX2lu'
    'cHV0cxgFIAMoCVINaGFzaHRhZ0lucHV0c0INCgtfcHJvZHVjdF9pZA==');

@$core.Deprecated('Use createPostResponseDescriptor instead')
const CreatePostResponse$json = {
  '1': 'CreatePostResponse',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'post'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `CreatePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPostResponseDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MWFscG'
    'hhMS5Qb3N0UgRwb3N0EiAKDGF1ZGl0X2xvZ19pZBgCIAEoCVIKYXVkaXRMb2dJZA==');

@$core.Deprecated('Use updatePostRequestDescriptor instead')
const UpdatePostRequest$json = {
  '1': 'UpdatePostRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.delegation.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {
      '1': 'post',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '8': {},
      '10': 'post'
    },
    {
      '1': 'update_mask',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.FieldMask',
      '10': 'updateMask'
    },
  ],
};

/// Descriptor for `UpdatePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePostRequestDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVQb3N0UmVxdWVzdBJQCglvcGVyYXRpb24YASABKAsyLS5oYXdrci5kZWxlZ2F0aW'
    '9uLnYxYWxwaGExLkRlbGVnYXRlZE9wZXJhdGlvbkID4EECUglvcGVyYXRpb24SNAoEcG9zdBgC'
    'IAEoCzIbLmhhd2tyLnNvY2lhbC52MWFscGhhMS5Qb3N0QgPgQQJSBHBvc3QSOwoLdXBkYXRlX2'
    '1hc2sYAyABKAsyGi5nb29nbGUucHJvdG9idWYuRmllbGRNYXNrUgp1cGRhdGVNYXNr');

@$core.Deprecated('Use updatePostResponseDescriptor instead')
const UpdatePostResponse$json = {
  '1': 'UpdatePostResponse',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'post'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `UpdatePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePostResponseDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MWFscG'
    'hhMS5Qb3N0UgRwb3N0EiAKDGF1ZGl0X2xvZ19pZBgCIAEoCVIKYXVkaXRMb2dJZA==');

@$core.Deprecated('Use publishPostRequestDescriptor instead')
const PublishPostRequest$json = {
  '1': 'PublishPostRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.delegation.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'post_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `PublishPostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishPostRequestDescriptor = $convert.base64Decode(
    'ChJQdWJsaXNoUG9zdFJlcXVlc3QSUAoJb3BlcmF0aW9uGAEgASgLMi0uaGF3a3IuZGVsZWdhdG'
    'lvbi52MWFscGhhMS5EZWxlZ2F0ZWRPcGVyYXRpb25CA+BBAlIJb3BlcmF0aW9uEhwKB3Bvc3Rf'
    'aWQYAiABKAlCA+BBAlIGcG9zdElkEhcKBGV0YWcYAyABKAlCA+BBAlIEZXRhZw==');

@$core.Deprecated('Use publishPostResponseDescriptor instead')
const PublishPostResponse$json = {
  '1': 'PublishPostResponse',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'post'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `PublishPostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishPostResponseDescriptor = $convert.base64Decode(
    'ChNQdWJsaXNoUG9zdFJlc3BvbnNlEi8KBHBvc3QYASABKAsyGy5oYXdrci5zb2NpYWwudjFhbH'
    'BoYTEuUG9zdFIEcG9zdBIgCgxhdWRpdF9sb2dfaWQYAiABKAlSCmF1ZGl0TG9nSWQ=');

@$core.Deprecated('Use unpublishPostRequestDescriptor instead')
const UnpublishPostRequest$json = {
  '1': 'UnpublishPostRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.delegation.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'post_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `UnpublishPostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishPostRequestDescriptor = $convert.base64Decode(
    'ChRVbnB1Ymxpc2hQb3N0UmVxdWVzdBJQCglvcGVyYXRpb24YASABKAsyLS5oYXdrci5kZWxlZ2'
    'F0aW9uLnYxYWxwaGExLkRlbGVnYXRlZE9wZXJhdGlvbkID4EECUglvcGVyYXRpb24SHAoHcG9z'
    'dF9pZBgCIAEoCUID4EECUgZwb3N0SWQSFwoEZXRhZxgDIAEoCUID4EECUgRldGFn');

@$core.Deprecated('Use unpublishPostResponseDescriptor instead')
const UnpublishPostResponse$json = {
  '1': 'UnpublishPostResponse',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'post'
    },
    {'1': 'audit_log_id', '3': 2, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `UnpublishPostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishPostResponseDescriptor = $convert.base64Decode(
    'ChVVbnB1Ymxpc2hQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MW'
    'FscGhhMS5Qb3N0UgRwb3N0EiAKDGF1ZGl0X2xvZ19pZBgCIAEoCVIKYXVkaXRMb2dJZA==');

@$core.Deprecated('Use schedulePostRequestDescriptor instead')
const SchedulePostRequest$json = {
  '1': 'SchedulePostRequest',
  '2': [
    {
      '1': 'operation',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.delegation.v1alpha1.DelegatedOperation',
      '8': {},
      '10': 'operation'
    },
    {'1': 'post_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'publish_time',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'publishTime'
    },
  ],
};

/// Descriptor for `SchedulePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List schedulePostRequestDescriptor = $convert.base64Decode(
    'ChNTY2hlZHVsZVBvc3RSZXF1ZXN0ElAKCW9wZXJhdGlvbhgBIAEoCzItLmhhd2tyLmRlbGVnYX'
    'Rpb24udjFhbHBoYTEuRGVsZWdhdGVkT3BlcmF0aW9uQgPgQQJSCW9wZXJhdGlvbhIcCgdwb3N0'
    'X2lkGAIgASgJQgPgQQJSBnBvc3RJZBIXCgRldGFnGAMgASgJQgPgQQJSBGV0YWcSQgoMcHVibG'
    'lzaF90aW1lGAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EECUgtwdWJsaXNo'
    'VGltZQ==');

@$core.Deprecated('Use schedulePostResponseDescriptor instead')
const SchedulePostResponse$json = {
  '1': 'SchedulePostResponse',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'post'
    },
    {
      '1': 'publish_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'publishTime'
    },
    {'1': 'audit_log_id', '3': 3, '4': 1, '5': 9, '10': 'auditLogId'},
  ],
};

/// Descriptor for `SchedulePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List schedulePostResponseDescriptor = $convert.base64Decode(
    'ChRTY2hlZHVsZVBvc3RSZXNwb25zZRIvCgRwb3N0GAEgASgLMhsuaGF3a3Iuc29jaWFsLnYxYW'
    'xwaGExLlBvc3RSBHBvc3QSPQoMcHVibGlzaF90aW1lGAIgASgLMhouZ29vZ2xlLnByb3RvYnVm'
    'LlRpbWVzdGFtcFILcHVibGlzaFRpbWUSIAoMYXVkaXRfbG9nX2lkGAMgASgJUgphdWRpdExvZ0'
    'lk');

const $core.Map<$core.String, $core.dynamic>
    DelegationDelegatedServiceBase$json = {
  '1': 'DelegationDelegatedService',
  '2': [
    {
      '1': 'ListPosts',
      '2': '.hawkr.delegation.v1alpha1.ListPostsRequest',
      '3': '.hawkr.delegation.v1alpha1.ListPostsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetPost',
      '2': '.hawkr.delegation.v1alpha1.GetPostRequest',
      '3': '.hawkr.delegation.v1alpha1.GetPostResponse',
      '4': {'34': 1},
    },
    {
      '1': 'CreatePost',
      '2': '.hawkr.delegation.v1alpha1.CreatePostRequest',
      '3': '.hawkr.delegation.v1alpha1.CreatePostResponse'
    },
    {
      '1': 'UpdatePost',
      '2': '.hawkr.delegation.v1alpha1.UpdatePostRequest',
      '3': '.hawkr.delegation.v1alpha1.UpdatePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'PublishPost',
      '2': '.hawkr.delegation.v1alpha1.PublishPostRequest',
      '3': '.hawkr.delegation.v1alpha1.PublishPostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnpublishPost',
      '2': '.hawkr.delegation.v1alpha1.UnpublishPostRequest',
      '3': '.hawkr.delegation.v1alpha1.UnpublishPostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'SchedulePost',
      '2': '.hawkr.delegation.v1alpha1.SchedulePostRequest',
      '3': '.hawkr.delegation.v1alpha1.SchedulePostResponse',
      '4': {'34': 2},
    },
  ],
};

@$core.Deprecated('Use delegationDelegatedServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    DelegationDelegatedServiceBase$messageJson = {
  '.hawkr.delegation.v1alpha1.ListPostsRequest': ListPostsRequest$json,
  '.hawkr.delegation.v1alpha1.ListPostsResponse': ListPostsResponse$json,
  '.hawkr.social.v1alpha1.Post': $0.Post$json,
  '.hawkr.common.v1alpha1.StoreCard': $3.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $4.Image$json,
  '.hawkr.social.v1alpha1.Hashtag': $0.Hashtag$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.delegation.v1alpha1.GetPostRequest': GetPostRequest$json,
  '.hawkr.delegation.v1alpha1.GetPostResponse': GetPostResponse$json,
  '.hawkr.delegation.v1alpha1.CreatePostRequest': CreatePostRequest$json,
  '.hawkr.delegation.v1alpha1.DelegatedOperation': DelegatedOperation$json,
  '.hawkr.delegation.v1alpha1.CreatePostResponse': CreatePostResponse$json,
  '.hawkr.delegation.v1alpha1.UpdatePostRequest': UpdatePostRequest$json,
  '.google.protobuf.FieldMask': $1.FieldMask$json,
  '.hawkr.delegation.v1alpha1.UpdatePostResponse': UpdatePostResponse$json,
  '.hawkr.delegation.v1alpha1.PublishPostRequest': PublishPostRequest$json,
  '.hawkr.delegation.v1alpha1.PublishPostResponse': PublishPostResponse$json,
  '.hawkr.delegation.v1alpha1.UnpublishPostRequest': UnpublishPostRequest$json,
  '.hawkr.delegation.v1alpha1.UnpublishPostResponse':
      UnpublishPostResponse$json,
  '.hawkr.delegation.v1alpha1.SchedulePostRequest': SchedulePostRequest$json,
  '.hawkr.delegation.v1alpha1.SchedulePostResponse': SchedulePostResponse$json,
};

/// Descriptor for `DelegationDelegatedService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List delegationDelegatedServiceDescriptor = $convert.base64Decode(
    'ChpEZWxlZ2F0aW9uRGVsZWdhdGVkU2VydmljZRJrCglMaXN0UG9zdHMSKy5oYXdrci5kZWxlZ2'
    'F0aW9uLnYxYWxwaGExLkxpc3RQb3N0c1JlcXVlc3QaLC5oYXdrci5kZWxlZ2F0aW9uLnYxYWxw'
    'aGExLkxpc3RQb3N0c1Jlc3BvbnNlIgOQAgESZQoHR2V0UG9zdBIpLmhhd2tyLmRlbGVnYXRpb2'
    '4udjFhbHBoYTEuR2V0UG9zdFJlcXVlc3QaKi5oYXdrci5kZWxlZ2F0aW9uLnYxYWxwaGExLkdl'
    'dFBvc3RSZXNwb25zZSIDkAIBEmkKCkNyZWF0ZVBvc3QSLC5oYXdrci5kZWxlZ2F0aW9uLnYxYW'
    'xwaGExLkNyZWF0ZVBvc3RSZXF1ZXN0Gi0uaGF3a3IuZGVsZWdhdGlvbi52MWFscGhhMS5DcmVh'
    'dGVQb3N0UmVzcG9uc2USbgoKVXBkYXRlUG9zdBIsLmhhd2tyLmRlbGVnYXRpb24udjFhbHBoYT'
    'EuVXBkYXRlUG9zdFJlcXVlc3QaLS5oYXdrci5kZWxlZ2F0aW9uLnYxYWxwaGExLlVwZGF0ZVBv'
    'c3RSZXNwb25zZSIDkAICEnEKC1B1Ymxpc2hQb3N0Ei0uaGF3a3IuZGVsZWdhdGlvbi52MWFscG'
    'hhMS5QdWJsaXNoUG9zdFJlcXVlc3QaLi5oYXdrci5kZWxlZ2F0aW9uLnYxYWxwaGExLlB1Ymxp'
    'c2hQb3N0UmVzcG9uc2UiA5ACAhJ3Cg1VbnB1Ymxpc2hQb3N0Ei8uaGF3a3IuZGVsZWdhdGlvbi'
    '52MWFscGhhMS5VbnB1Ymxpc2hQb3N0UmVxdWVzdBowLmhhd2tyLmRlbGVnYXRpb24udjFhbHBo'
    'YTEuVW5wdWJsaXNoUG9zdFJlc3BvbnNlIgOQAgISdAoMU2NoZWR1bGVQb3N0Ei4uaGF3a3IuZG'
    'VsZWdhdGlvbi52MWFscGhhMS5TY2hlZHVsZVBvc3RSZXF1ZXN0Gi8uaGF3a3IuZGVsZWdhdGlv'
    'bi52MWFscGhhMS5TY2hlZHVsZVBvc3RSZXNwb25zZSIDkAIC');
