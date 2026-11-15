// This is a generated file - do not edit.
//
// Generated from hawkr/social/v1alpha1/social.proto.

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
    as $3;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $2;

import '../../common/v1alpha1/image.pbjson.dart' as $1;
import '../../common/v1alpha1/store_card.pbjson.dart' as $0;

@$core.Deprecated('Use actorKindDescriptor instead')
const ActorKind$json = {
  '1': 'ActorKind',
  '2': [
    {'1': 'ACTOR_KIND_UNSPECIFIED', '2': 0},
    {'1': 'ACTOR_KIND_BUYER', '2': 1},
    {'1': 'ACTOR_KIND_STORE', '2': 2},
  ],
};

/// Descriptor for `ActorKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List actorKindDescriptor = $convert.base64Decode(
    'CglBY3RvcktpbmQSGgoWQUNUT1JfS0lORF9VTlNQRUNJRklFRBAAEhQKEEFDVE9SX0tJTkRfQl'
    'VZRVIQARIUChBBQ1RPUl9LSU5EX1NUT1JFEAI=');

@$core.Deprecated('Use commentStateDescriptor instead')
const CommentState$json = {
  '1': 'CommentState',
  '2': [
    {'1': 'COMMENT_STATE_UNSPECIFIED', '2': 0},
    {'1': 'COMMENT_STATE_VISIBLE', '2': 1},
    {'1': 'COMMENT_STATE_DELETED', '2': 2},
    {'1': 'COMMENT_STATE_HIDDEN', '2': 3},
  ],
};

/// Descriptor for `CommentState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List commentStateDescriptor = $convert.base64Decode(
    'CgxDb21tZW50U3RhdGUSHQoZQ09NTUVOVF9TVEFURV9VTlNQRUNJRklFRBAAEhkKFUNPTU1FTl'
    'RfU1RBVEVfVklTSUJMRRABEhkKFUNPTU1FTlRfU1RBVEVfREVMRVRFRBACEhgKFENPTU1FTlRf'
    'U1RBVEVfSElEREVOEAM=');

@$core.Deprecated('Use postStatusDescriptor instead')
const PostStatus$json = {
  '1': 'PostStatus',
  '2': [
    {'1': 'POST_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'POST_STATUS_DRAFT', '2': 1},
    {'1': 'POST_STATUS_PUBLISHED', '2': 2},
    {'1': 'POST_STATUS_UNPUBLISHED', '2': 3},
  ],
};

/// Descriptor for `PostStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List postStatusDescriptor = $convert.base64Decode(
    'CgpQb3N0U3RhdHVzEhsKF1BPU1RfU1RBVFVTX1VOU1BFQ0lGSUVEEAASFQoRUE9TVF9TVEFUVV'
    'NfRFJBRlQQARIZChVQT1NUX1NUQVRVU19QVUJMSVNIRUQQAhIbChdQT1NUX1NUQVRVU19VTlBV'
    'QkxJU0hFRBAD');

@$core.Deprecated('Use sanctionKindDescriptor instead')
const SanctionKind$json = {
  '1': 'SanctionKind',
  '2': [
    {'1': 'SANCTION_KIND_UNSPECIFIED', '2': 0},
    {'1': 'SANCTION_KIND_WARNING', '2': 1},
  ],
};

/// Descriptor for `SanctionKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sanctionKindDescriptor = $convert.base64Decode(
    'CgxTYW5jdGlvbktpbmQSHQoZU0FOQ1RJT05fS0lORF9VTlNQRUNJRklFRBAAEhkKFVNBTkNUSU'
    '9OX0tJTkRfV0FSTklORxAB');

@$core.Deprecated('Use hiddenContentKindDescriptor instead')
const HiddenContentKind$json = {
  '1': 'HiddenContentKind',
  '2': [
    {'1': 'HIDDEN_CONTENT_KIND_UNSPECIFIED', '2': 0},
    {'1': 'HIDDEN_CONTENT_KIND_COMMENT', '2': 1},
    {'1': 'HIDDEN_CONTENT_KIND_FORUM_ENTRY', '2': 2},
  ],
};

/// Descriptor for `HiddenContentKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List hiddenContentKindDescriptor = $convert.base64Decode(
    'ChFIaWRkZW5Db250ZW50S2luZBIjCh9ISURERU5fQ09OVEVOVF9LSU5EX1VOU1BFQ0lGSUVEEA'
    'ASHwobSElEREVOX0NPTlRFTlRfS0lORF9DT01NRU5UEAESIwofSElEREVOX0NPTlRFTlRfS0lO'
    'RF9GT1JVTV9FTlRSWRAC');

@$core.Deprecated('Use hashtagDescriptor instead')
const Hashtag$json = {
  '1': 'Hashtag',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'display', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'display'},
  ],
};

/// Descriptor for `Hashtag`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hashtagDescriptor = $convert.base64Decode(
    'CgdIYXNodGFnEhcKBG5hbWUYASABKAlCA+BBA1IEbmFtZRIdCgdkaXNwbGF5GAIgASgJQgPgQQ'
    'NSB2Rpc3BsYXk=');

@$core.Deprecated('Use postDescriptor instead')
const Post$json = {
  '1': 'Post',
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
    {'1': 'body', '3': 3, '4': 1, '5': 9, '10': 'body'},
    {
      '1': 'image',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'image'
    },
    {
      '1': 'product_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'productId',
      '17': true
    },
    {
      '1': 'hashtags',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Hashtag',
      '8': {},
      '10': 'hashtags'
    },
    {
      '1': 'publish_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'publishTime'
    },
    {
      '1': 'update_time',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {'1': 'like_count', '3': 9, '4': 1, '5': 3, '8': {}, '10': 'likeCount'},
    {
      '1': 'comment_count',
      '3': 10,
      '4': 1,
      '5': 3,
      '8': {},
      '10': 'commentCount'
    },
    {'1': 'view_count', '3': 11, '4': 1, '5': 3, '8': {}, '10': 'viewCount'},
    {'1': 'share_count', '3': 12, '4': 1, '5': 3, '8': {}, '10': 'shareCount'},
    {'1': 'url', '3': 13, '4': 1, '5': 9, '8': {}, '10': 'url'},
    {
      '1': 'viewer_can_comment',
      '3': 14,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanComment'
    },
    {
      '1': 'status',
      '3': 15,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.PostStatus',
      '8': {},
      '10': 'status'
    },
    {'1': 'etag', '3': 16, '4': 1, '5': 9, '10': 'etag'},
    {
      '1': 'create_time',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'image_staged_token',
      '3': 18,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'imageStagedToken'
    },
    {
      '1': 'hashtag_inputs',
      '3': 19,
      '4': 3,
      '5': 9,
      '8': {},
      '10': 'hashtagInputs'
    },
    {
      '1': 'viewer_liked',
      '3': 20,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerLiked'
    },
  ],
  '8': [
    {'1': '_product_id'},
  ],
};

/// Descriptor for `Post`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List postDescriptor = $convert.base64Decode(
    'CgRQb3N0EhMKAmlkGAEgASgJQgPgQQhSAmlkEjsKBXN0b3JlGAIgASgLMiAuaGF3a3IuY29tbW'
    '9uLnYxYWxwaGExLlN0b3JlQ2FyZEID4EEDUgVzdG9yZRISCgRib2R5GAMgASgJUgRib2R5EjcK'
    'BWltYWdlGAQgASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaGExLkltYWdlQgPgQQNSBWltYWdlEi'
    'IKCnByb2R1Y3RfaWQYBSABKAlIAFIJcHJvZHVjdElkiAEBEj8KCGhhc2h0YWdzGAYgAygLMh4u'
    'aGF3a3Iuc29jaWFsLnYxYWxwaGExLkhhc2h0YWdCA+BBA1IIaGFzaHRhZ3MSQgoMcHVibGlzaF'
    '90aW1lGAcgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgtwdWJsaXNoVGlt'
    'ZRJACgt1cGRhdGVfdGltZRgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1'
    'IKdXBkYXRlVGltZRIiCgpsaWtlX2NvdW50GAkgASgDQgPgQQNSCWxpa2VDb3VudBIoCg1jb21t'
    'ZW50X2NvdW50GAogASgDQgPgQQNSDGNvbW1lbnRDb3VudBIiCgp2aWV3X2NvdW50GAsgASgDQg'
    'PgQQNSCXZpZXdDb3VudBIkCgtzaGFyZV9jb3VudBgMIAEoA0ID4EEDUgpzaGFyZUNvdW50EhUK'
    'A3VybBgNIAEoCUID4EEDUgN1cmwSMQoSdmlld2VyX2Nhbl9jb21tZW50GA4gASgIQgPgQQNSEH'
    'ZpZXdlckNhbkNvbW1lbnQSPgoGc3RhdHVzGA8gASgOMiEuaGF3a3Iuc29jaWFsLnYxYWxwaGEx'
    'LlBvc3RTdGF0dXNCA+BBA1IGc3RhdHVzEhIKBGV0YWcYECABKAlSBGV0YWcSQAoLY3JlYXRlX3'
    'RpbWUYESABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCmNyZWF0ZVRpbWUS'
    'MQoSaW1hZ2Vfc3RhZ2VkX3Rva2VuGBIgASgJQgPgQQRSEGltYWdlU3RhZ2VkVG9rZW4SKgoOaG'
    'FzaHRhZ19pbnB1dHMYEyADKAlCA+BBBFINaGFzaHRhZ0lucHV0cxImCgx2aWV3ZXJfbGlrZWQY'
    'FCABKAhCA+BBA1ILdmlld2VyTGlrZWRCDQoLX3Byb2R1Y3RfaWQ=');

@$core.Deprecated('Use actorDescriptor instead')
const Actor$json = {
  '1': 'Actor',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.ActorKind',
      '8': {},
      '10': 'kind'
    },
    {'1': 'id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'name',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'name',
      '17': true
    },
    {
      '1': 'avatar',
      '3': 4,
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

/// Descriptor for `Actor`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List actorDescriptor = $convert.base64Decode(
    'CgVBY3RvchI5CgRraW5kGAEgASgOMiAuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkFjdG9yS2luZE'
    'ID4EEDUgRraW5kEhMKAmlkGAIgASgJQgPgQQNSAmlkEhwKBG5hbWUYAyABKAlCA+BBA0gAUgRu'
    'YW1liAEBEjkKBmF2YXRhchgEIAEoCzIcLmhhd2tyLmNvbW1vbi52MWFscGhhMS5JbWFnZUID4E'
    'EDUgZhdmF0YXJCBwoFX25hbWU=');

@$core.Deprecated('Use commentDescriptor instead')
const Comment$json = {
  '1': 'Comment',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'post_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {
      '1': 'parent_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'parentId',
      '17': true
    },
    {
      '1': 'reply_to_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 1,
      '10': 'replyToId',
      '17': true
    },
    {
      '1': 'state',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.CommentState',
      '8': {},
      '10': 'state'
    },
    {
      '1': 'author',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Actor',
      '8': {},
      '10': 'author'
    },
    {'1': 'body', '3': 7, '4': 1, '5': 9, '10': 'body'},
    {
      '1': 'create_time',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'update_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {'1': 'like_count', '3': 10, '4': 1, '5': 3, '8': {}, '10': 'likeCount'},
    {'1': 'reply_count', '3': 11, '4': 1, '5': 3, '8': {}, '10': 'replyCount'},
    {
      '1': 'viewer_liked',
      '3': 12,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerLiked'
    },
    {'1': 'is_mine', '3': 13, '4': 1, '5': 8, '8': {}, '10': 'isMine'},
    {
      '1': 'viewer_can_edit',
      '3': 14,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanEdit'
    },
    {
      '1': 'viewer_can_delete',
      '3': 15,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanDelete'
    },
    {
      '1': 'viewer_can_reply',
      '3': 16,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanReply'
    },
    {'1': 'etag', '3': 17, '4': 1, '5': 9, '10': 'etag'},
  ],
  '8': [
    {'1': '_parent_id'},
    {'1': '_reply_to_id'},
  ],
};

/// Descriptor for `Comment`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List commentDescriptor = $convert.base64Decode(
    'CgdDb21tZW50EhMKAmlkGAEgASgJQgPgQQhSAmlkEhwKB3Bvc3RfaWQYAiABKAlCA+BBA1IGcG'
    '9zdElkEiUKCXBhcmVudF9pZBgDIAEoCUID4EEDSABSCHBhcmVudElkiAEBEigKC3JlcGx5X3Rv'
    'X2lkGAQgASgJQgPgQQNIAVIJcmVwbHlUb0lkiAEBEj4KBXN0YXRlGAUgASgOMiMuaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLkNvbW1lbnRTdGF0ZUID4EEDUgVzdGF0ZRI5CgZhdXRob3IYBiABKAsy'
    'HC5oYXdrci5zb2NpYWwudjFhbHBoYTEuQWN0b3JCA+BBA1IGYXV0aG9yEhIKBGJvZHkYByABKA'
    'lSBGJvZHkSQAoLY3JlYXRlX3RpbWUYCCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1w'
    'QgPgQQNSCmNyZWF0ZVRpbWUSQAoLdXBkYXRlX3RpbWUYCSABKAsyGi5nb29nbGUucHJvdG9idW'
    'YuVGltZXN0YW1wQgPgQQNSCnVwZGF0ZVRpbWUSIgoKbGlrZV9jb3VudBgKIAEoA0ID4EEDUgls'
    'aWtlQ291bnQSJAoLcmVwbHlfY291bnQYCyABKANCA+BBA1IKcmVwbHlDb3VudBImCgx2aWV3ZX'
    'JfbGlrZWQYDCABKAhCA+BBA1ILdmlld2VyTGlrZWQSHAoHaXNfbWluZRgNIAEoCEID4EEDUgZp'
    'c01pbmUSKwoPdmlld2VyX2Nhbl9lZGl0GA4gASgIQgPgQQNSDXZpZXdlckNhbkVkaXQSLwoRdm'
    'lld2VyX2Nhbl9kZWxldGUYDyABKAhCA+BBA1IPdmlld2VyQ2FuRGVsZXRlEi0KEHZpZXdlcl9j'
    'YW5fcmVwbHkYECABKAhCA+BBA1IOdmlld2VyQ2FuUmVwbHkSEgoEZXRhZxgRIAEoCVIEZXRhZ0'
    'IMCgpfcGFyZW50X2lkQg4KDF9yZXBseV90b19pZA==');

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

@$core.Deprecated('Use listPostsRequestDescriptor instead')
const ListPostsRequest$json = {
  '1': 'ListPostsRequest',
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
    {
      '1': 'hashtag',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'hashtag',
      '17': true
    },
  ],
  '8': [
    {'1': '_store_id'},
    {'1': '_hashtag'},
  ],
};

/// Descriptor for `ListPostsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listPostsRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0UG9zdHNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQoKcGFnZV'
    '90b2tlbhgCIAEoCVIJcGFnZVRva2VuEh4KCHN0b3JlX2lkGAMgASgJSABSB3N0b3JlSWSIAQES'
    'HQoHaGFzaHRhZxgEIAEoCUgBUgdoYXNodGFniAEBQgsKCV9zdG9yZV9pZEIKCghfaGFzaHRhZw'
    '==');

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

@$core.Deprecated('Use listCommentsRequestDescriptor instead')
const ListCommentsRequest$json = {
  '1': 'ListCommentsRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'parent_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'parentId',
      '17': true
    },
  ],
  '8': [
    {'1': '_parent_id'},
  ],
};

/// Descriptor for `ListCommentsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCommentsRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0Q29tbWVudHNSZXF1ZXN0EhwKB3Bvc3RfaWQYASABKAlCA+BBAlIGcG9zdElkEhsKCX'
    'BhZ2Vfc2l6ZRgCIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgDIAEoCVIJcGFnZVRva2Vu'
    'EiAKCXBhcmVudF9pZBgEIAEoCUgAUghwYXJlbnRJZIgBAUIMCgpfcGFyZW50X2lk');

@$core.Deprecated('Use listCommentsResponseDescriptor instead')
const ListCommentsResponse$json = {
  '1': 'ListCommentsResponse',
  '2': [
    {
      '1': 'comments',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Comment',
      '10': 'comments'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListCommentsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCommentsResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0Q29tbWVudHNSZXNwb25zZRI6Cghjb21tZW50cxgBIAMoCzIeLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5Db21tZW50Ughjb21tZW50cxImCg9uZXh0X3BhZ2VfdG9rZW4YAiABKAlSDW5l'
    'eHRQYWdlVG9rZW4=');

@$core.Deprecated('Use suggestTrendingPostsRequestDescriptor instead')
const SuggestTrendingPostsRequest$json = {
  '1': 'SuggestTrendingPostsRequest',
  '2': [
    {'1': 'max_results', '3': 1, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestTrendingPostsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTrendingPostsRequestDescriptor =
    $convert.base64Decode(
        'ChtTdWdnZXN0VHJlbmRpbmdQb3N0c1JlcXVlc3QSHwoLbWF4X3Jlc3VsdHMYASABKAVSCm1heF'
        'Jlc3VsdHM=');

@$core.Deprecated('Use suggestTrendingPostsResponseDescriptor instead')
const SuggestTrendingPostsResponse$json = {
  '1': 'SuggestTrendingPostsResponse',
  '2': [
    {
      '1': 'posts',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '10': 'posts'
    },
  ],
};

/// Descriptor for `SuggestTrendingPostsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTrendingPostsResponseDescriptor =
    $convert.base64Decode(
        'ChxTdWdnZXN0VHJlbmRpbmdQb3N0c1Jlc3BvbnNlEjEKBXBvc3RzGAEgAygLMhsuaGF3a3Iuc2'
        '9jaWFsLnYxYWxwaGExLlBvc3RSBXBvc3Rz');

@$core.Deprecated('Use suggestTagsRequestDescriptor instead')
const SuggestTagsRequest$json = {
  '1': 'SuggestTagsRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'query'},
    {'1': 'contains', '3': 2, '4': 1, '5': 8, '10': 'contains'},
    {'1': 'max_results', '3': 3, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestTagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTagsRequestDescriptor = $convert.base64Decode(
    'ChJTdWdnZXN0VGFnc1JlcXVlc3QSGQoFcXVlcnkYASABKAlCA+BBAlIFcXVlcnkSGgoIY29udG'
    'FpbnMYAiABKAhSCGNvbnRhaW5zEh8KC21heF9yZXN1bHRzGAMgASgFUgptYXhSZXN1bHRz');

@$core.Deprecated('Use suggestTagsResponseDescriptor instead')
const SuggestTagsResponse$json = {
  '1': 'SuggestTagsResponse',
  '2': [
    {
      '1': 'tags',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.TagCount',
      '10': 'tags'
    },
  ],
};

/// Descriptor for `SuggestTagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTagsResponseDescriptor = $convert.base64Decode(
    'ChNTdWdnZXN0VGFnc1Jlc3BvbnNlEjMKBHRhZ3MYASADKAsyHy5oYXdrci5zb2NpYWwudjFhbH'
    'BoYTEuVGFnQ291bnRSBHRhZ3M=');

@$core.Deprecated('Use suggestTrendingTagsRequestDescriptor instead')
const SuggestTrendingTagsRequest$json = {
  '1': 'SuggestTrendingTagsRequest',
  '2': [
    {
      '1': 'start_time',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'startTime'
    },
    {
      '1': 'end_time',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'endTime'
    },
    {'1': 'max_results', '3': 3, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SuggestTrendingTagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTrendingTagsRequestDescriptor = $convert.base64Decode(
    'ChpTdWdnZXN0VHJlbmRpbmdUYWdzUmVxdWVzdBI5CgpzdGFydF90aW1lGAEgASgLMhouZ29vZ2'
    'xlLnByb3RvYnVmLlRpbWVzdGFtcFIJc3RhcnRUaW1lEjUKCGVuZF90aW1lGAIgASgLMhouZ29v'
    'Z2xlLnByb3RvYnVmLlRpbWVzdGFtcFIHZW5kVGltZRIfCgttYXhfcmVzdWx0cxgDIAEoBVIKbW'
    'F4UmVzdWx0cw==');

@$core.Deprecated('Use suggestTrendingTagsResponseDescriptor instead')
const SuggestTrendingTagsResponse$json = {
  '1': 'SuggestTrendingTagsResponse',
  '2': [
    {
      '1': 'tags',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.TagCount',
      '10': 'tags'
    },
  ],
};

/// Descriptor for `SuggestTrendingTagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List suggestTrendingTagsResponseDescriptor =
    $convert.base64Decode(
        'ChtTdWdnZXN0VHJlbmRpbmdUYWdzUmVzcG9uc2USMwoEdGFncxgBIAMoCzIfLmhhd2tyLnNvY2'
        'lhbC52MWFscGhhMS5UYWdDb3VudFIEdGFncw==');

@$core.Deprecated('Use tagCountDescriptor instead')
const TagCount$json = {
  '1': 'TagCount',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'name'},
    {'1': 'post_count', '3': 2, '4': 1, '5': 3, '8': {}, '10': 'postCount'},
  ],
};

/// Descriptor for `TagCount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tagCountDescriptor = $convert.base64Decode(
    'CghUYWdDb3VudBIXCgRuYW1lGAEgASgJQgPgQQNSBG5hbWUSIgoKcG9zdF9jb3VudBgCIAEoA0'
    'ID4EEDUglwb3N0Q291bnQ=');

@$core.Deprecated('Use forumEntryDescriptor instead')
const ForumEntry$json = {
  '1': 'ForumEntry',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {'1': 'store_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {
      '1': 'parent_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'parentId',
      '17': true
    },
    {
      '1': 'reply_to_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 1,
      '10': 'replyToId',
      '17': true
    },
    {
      '1': 'state',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.CommentState',
      '8': {},
      '10': 'state'
    },
    {
      '1': 'author',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Actor',
      '8': {},
      '10': 'author'
    },
    {'1': 'body', '3': 7, '4': 1, '5': 9, '10': 'body'},
    {
      '1': 'create_time',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
    {
      '1': 'update_time',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'updateTime'
    },
    {'1': 'reply_count', '3': 10, '4': 1, '5': 3, '8': {}, '10': 'replyCount'},
    {'1': 'is_mine', '3': 11, '4': 1, '5': 8, '8': {}, '10': 'isMine'},
    {
      '1': 'viewer_can_edit',
      '3': 12,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanEdit'
    },
    {
      '1': 'viewer_can_delete',
      '3': 13,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanDelete'
    },
    {
      '1': 'viewer_can_reply',
      '3': 14,
      '4': 1,
      '5': 8,
      '8': {},
      '10': 'viewerCanReply'
    },
    {'1': 'etag', '3': 15, '4': 1, '5': 9, '10': 'etag'},
  ],
  '8': [
    {'1': '_parent_id'},
    {'1': '_reply_to_id'},
  ],
};

/// Descriptor for `ForumEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List forumEntryDescriptor = $convert.base64Decode(
    'CgpGb3J1bUVudHJ5EhMKAmlkGAEgASgJQgPgQQhSAmlkEh4KCHN0b3JlX2lkGAIgASgJQgPgQQ'
    'NSB3N0b3JlSWQSJQoJcGFyZW50X2lkGAMgASgJQgPgQQNIAFIIcGFyZW50SWSIAQESKAoLcmVw'
    'bHlfdG9faWQYBCABKAlCA+BBA0gBUglyZXBseVRvSWSIAQESPgoFc3RhdGUYBSABKA4yIy5oYX'
    'drci5zb2NpYWwudjFhbHBoYTEuQ29tbWVudFN0YXRlQgPgQQNSBXN0YXRlEjkKBmF1dGhvchgG'
    'IAEoCzIcLmhhd2tyLnNvY2lhbC52MWFscGhhMS5BY3RvckID4EEDUgZhdXRob3ISEgoEYm9keR'
    'gHIAEoCVIEYm9keRJACgtjcmVhdGVfdGltZRgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1l'
    'c3RhbXBCA+BBA1IKY3JlYXRlVGltZRJACgt1cGRhdGVfdGltZRgJIAEoCzIaLmdvb2dsZS5wcm'
    '90b2J1Zi5UaW1lc3RhbXBCA+BBA1IKdXBkYXRlVGltZRIkCgtyZXBseV9jb3VudBgKIAEoA0ID'
    '4EEDUgpyZXBseUNvdW50EhwKB2lzX21pbmUYCyABKAhCA+BBA1IGaXNNaW5lEisKD3ZpZXdlcl'
    '9jYW5fZWRpdBgMIAEoCEID4EEDUg12aWV3ZXJDYW5FZGl0Ei8KEXZpZXdlcl9jYW5fZGVsZXRl'
    'GA0gASgIQgPgQQNSD3ZpZXdlckNhbkRlbGV0ZRItChB2aWV3ZXJfY2FuX3JlcGx5GA4gASgIQg'
    'PgQQNSDnZpZXdlckNhblJlcGx5EhIKBGV0YWcYDyABKAlSBGV0YWdCDAoKX3BhcmVudF9pZEIO'
    'CgxfcmVwbHlfdG9faWQ=');

@$core.Deprecated('Use listForumEntriesRequestDescriptor instead')
const ListForumEntriesRequest$json = {
  '1': 'ListForumEntriesRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 3, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'parent_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'parentId',
      '17': true
    },
  ],
  '8': [
    {'1': '_parent_id'},
  ],
};

/// Descriptor for `ListForumEntriesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listForumEntriesRequestDescriptor = $convert.base64Decode(
    'ChdMaXN0Rm9ydW1FbnRyaWVzUmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG9yZU'
    'lkEhsKCXBhZ2Vfc2l6ZRgCIAEoBVIIcGFnZVNpemUSHQoKcGFnZV90b2tlbhgDIAEoCVIJcGFn'
    'ZVRva2VuEiAKCXBhcmVudF9pZBgEIAEoCUgAUghwYXJlbnRJZIgBAUIMCgpfcGFyZW50X2lk');

@$core.Deprecated('Use listForumEntriesResponseDescriptor instead')
const ListForumEntriesResponse$json = {
  '1': 'ListForumEntriesResponse',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.ForumEntry',
      '10': 'entries'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListForumEntriesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listForumEntriesResponseDescriptor = $convert.base64Decode(
    'ChhMaXN0Rm9ydW1FbnRyaWVzUmVzcG9uc2USOwoHZW50cmllcxgBIAMoCzIhLmhhd2tyLnNvY2'
    'lhbC52MWFscGhhMS5Gb3J1bUVudHJ5UgdlbnRyaWVzEiYKD25leHRfcGFnZV90b2tlbhgCIAEo'
    'CVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use recordPostViewRequestDescriptor instead')
const RecordPostViewRequest$json = {
  '1': 'RecordPostViewRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
  ],
};

/// Descriptor for `RecordPostViewRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordPostViewRequestDescriptor = $convert.base64Decode(
    'ChVSZWNvcmRQb3N0Vmlld1JlcXVlc3QSHAoHcG9zdF9pZBgBIAEoCUID4EECUgZwb3N0SWQ=');

@$core.Deprecated('Use recordPostViewResponseDescriptor instead')
const RecordPostViewResponse$json = {
  '1': 'RecordPostViewResponse',
};

/// Descriptor for `RecordPostViewResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recordPostViewResponseDescriptor =
    $convert.base64Decode('ChZSZWNvcmRQb3N0Vmlld1Jlc3BvbnNl');

@$core.Deprecated('Use sharePostRequestDescriptor instead')
const SharePostRequest$json = {
  '1': 'SharePostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
  ],
};

/// Descriptor for `SharePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sharePostRequestDescriptor = $convert.base64Decode(
    'ChBTaGFyZVBvc3RSZXF1ZXN0EhwKB3Bvc3RfaWQYASABKAlCA+BBAlIGcG9zdElk');

@$core.Deprecated('Use sharePostResponseDescriptor instead')
const SharePostResponse$json = {
  '1': 'SharePostResponse',
};

/// Descriptor for `SharePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sharePostResponseDescriptor =
    $convert.base64Decode('ChFTaGFyZVBvc3RSZXNwb25zZQ==');

@$core.Deprecated('Use likePostRequestDescriptor instead')
const LikePostRequest$json = {
  '1': 'LikePostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
  ],
};

/// Descriptor for `LikePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List likePostRequestDescriptor = $convert.base64Decode(
    'Cg9MaWtlUG9zdFJlcXVlc3QSHAoHcG9zdF9pZBgBIAEoCUID4EECUgZwb3N0SWQ=');

@$core.Deprecated('Use likePostResponseDescriptor instead')
const LikePostResponse$json = {
  '1': 'LikePostResponse',
};

/// Descriptor for `LikePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List likePostResponseDescriptor =
    $convert.base64Decode('ChBMaWtlUG9zdFJlc3BvbnNl');

@$core.Deprecated('Use unlikePostRequestDescriptor instead')
const UnlikePostRequest$json = {
  '1': 'UnlikePostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
  ],
};

/// Descriptor for `UnlikePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unlikePostRequestDescriptor = $convert.base64Decode(
    'ChFVbmxpa2VQb3N0UmVxdWVzdBIcCgdwb3N0X2lkGAEgASgJQgPgQQJSBnBvc3RJZA==');

@$core.Deprecated('Use unlikePostResponseDescriptor instead')
const UnlikePostResponse$json = {
  '1': 'UnlikePostResponse',
};

/// Descriptor for `UnlikePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unlikePostResponseDescriptor =
    $convert.base64Decode('ChJVbmxpa2VQb3N0UmVzcG9uc2U=');

@$core.Deprecated('Use likeCommentRequestDescriptor instead')
const LikeCommentRequest$json = {
  '1': 'LikeCommentRequest',
  '2': [
    {'1': 'comment_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'commentId'},
  ],
};

/// Descriptor for `LikeCommentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List likeCommentRequestDescriptor = $convert.base64Decode(
    'ChJMaWtlQ29tbWVudFJlcXVlc3QSIgoKY29tbWVudF9pZBgBIAEoCUID4EECUgljb21tZW50SW'
    'Q=');

@$core.Deprecated('Use likeCommentResponseDescriptor instead')
const LikeCommentResponse$json = {
  '1': 'LikeCommentResponse',
};

/// Descriptor for `LikeCommentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List likeCommentResponseDescriptor =
    $convert.base64Decode('ChNMaWtlQ29tbWVudFJlc3BvbnNl');

@$core.Deprecated('Use unlikeCommentRequestDescriptor instead')
const UnlikeCommentRequest$json = {
  '1': 'UnlikeCommentRequest',
  '2': [
    {'1': 'comment_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'commentId'},
  ],
};

/// Descriptor for `UnlikeCommentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unlikeCommentRequestDescriptor = $convert.base64Decode(
    'ChRVbmxpa2VDb21tZW50UmVxdWVzdBIiCgpjb21tZW50X2lkGAEgASgJQgPgQQJSCWNvbW1lbn'
    'RJZA==');

@$core.Deprecated('Use unlikeCommentResponseDescriptor instead')
const UnlikeCommentResponse$json = {
  '1': 'UnlikeCommentResponse',
};

/// Descriptor for `UnlikeCommentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unlikeCommentResponseDescriptor =
    $convert.base64Decode('ChVVbmxpa2VDb21tZW50UmVzcG9uc2U=');

@$core.Deprecated('Use createCommentRequestDescriptor instead')
const CreateCommentRequest$json = {
  '1': 'CreateCommentRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'body', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'body'},
    {
      '1': 'parent_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'parentId',
      '17': true
    },
    {
      '1': 'reply_to_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'replyToId',
      '17': true
    },
    {'1': 'request_id', '3': 5, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
  '8': [
    {'1': '_parent_id'},
    {'1': '_reply_to_id'},
  ],
};

/// Descriptor for `CreateCommentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCommentRequestDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVDb21tZW50UmVxdWVzdBIcCgdwb3N0X2lkGAEgASgJQgPgQQJSBnBvc3RJZBIXCg'
    'Rib2R5GAIgASgJQgPgQQJSBGJvZHkSIAoJcGFyZW50X2lkGAMgASgJSABSCHBhcmVudElkiAEB'
    'EiMKC3JlcGx5X3RvX2lkGAQgASgJSAFSCXJlcGx5VG9JZIgBARInCgpyZXF1ZXN0X2lkGAUgAS'
    'gJQgjijM/XCAIIAVIJcmVxdWVzdElkQgwKCl9wYXJlbnRfaWRCDgoMX3JlcGx5X3RvX2lk');

@$core.Deprecated('Use createCommentResponseDescriptor instead')
const CreateCommentResponse$json = {
  '1': 'CreateCommentResponse',
  '2': [
    {
      '1': 'comment',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Comment',
      '10': 'comment'
    },
  ],
};

/// Descriptor for `CreateCommentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCommentResponseDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVDb21tZW50UmVzcG9uc2USOAoHY29tbWVudBgBIAEoCzIeLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5Db21tZW50Ugdjb21tZW50');

@$core.Deprecated('Use updateCommentRequestDescriptor instead')
const UpdateCommentRequest$json = {
  '1': 'UpdateCommentRequest',
  '2': [
    {
      '1': 'comment',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Comment',
      '8': {},
      '10': 'comment'
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

/// Descriptor for `UpdateCommentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateCommentRequestDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVDb21tZW50UmVxdWVzdBI9Cgdjb21tZW50GAEgASgLMh4uaGF3a3Iuc29jaWFsLn'
    'YxYWxwaGExLkNvbW1lbnRCA+BBAlIHY29tbWVudBI7Cgt1cGRhdGVfbWFzaxgCIAEoCzIaLmdv'
    'b2dsZS5wcm90b2J1Zi5GaWVsZE1hc2tSCnVwZGF0ZU1hc2s=');

@$core.Deprecated('Use updateCommentResponseDescriptor instead')
const UpdateCommentResponse$json = {
  '1': 'UpdateCommentResponse',
  '2': [
    {
      '1': 'comment',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Comment',
      '10': 'comment'
    },
  ],
};

/// Descriptor for `UpdateCommentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateCommentResponseDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVDb21tZW50UmVzcG9uc2USOAoHY29tbWVudBgBIAEoCzIeLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5Db21tZW50Ugdjb21tZW50');

@$core.Deprecated('Use deleteCommentRequestDescriptor instead')
const DeleteCommentRequest$json = {
  '1': 'DeleteCommentRequest',
  '2': [
    {'1': 'comment_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'commentId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '10': 'etag'},
  ],
};

/// Descriptor for `DeleteCommentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteCommentRequestDescriptor = $convert.base64Decode(
    'ChREZWxldGVDb21tZW50UmVxdWVzdBIiCgpjb21tZW50X2lkGAEgASgJQgPgQQJSCWNvbW1lbn'
    'RJZBISCgRldGFnGAIgASgJUgRldGFn');

@$core.Deprecated('Use deleteCommentResponseDescriptor instead')
const DeleteCommentResponse$json = {
  '1': 'DeleteCommentResponse',
};

/// Descriptor for `DeleteCommentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteCommentResponseDescriptor =
    $convert.base64Decode('ChVEZWxldGVDb21tZW50UmVzcG9uc2U=');

@$core.Deprecated('Use createForumEntryRequestDescriptor instead')
const CreateForumEntryRequest$json = {
  '1': 'CreateForumEntryRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'body', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'body'},
    {
      '1': 'parent_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'parentId',
      '17': true
    },
    {
      '1': 'reply_to_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'replyToId',
      '17': true
    },
    {'1': 'request_id', '3': 5, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
  '8': [
    {'1': '_parent_id'},
    {'1': '_reply_to_id'},
  ],
};

/// Descriptor for `CreateForumEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createForumEntryRequestDescriptor = $convert.base64Decode(
    'ChdDcmVhdGVGb3J1bUVudHJ5UmVxdWVzdBIeCghzdG9yZV9pZBgBIAEoCUID4EECUgdzdG9yZU'
    'lkEhcKBGJvZHkYAiABKAlCA+BBAlIEYm9keRIgCglwYXJlbnRfaWQYAyABKAlIAFIIcGFyZW50'
    'SWSIAQESIwoLcmVwbHlfdG9faWQYBCABKAlIAVIJcmVwbHlUb0lkiAEBEicKCnJlcXVlc3RfaW'
    'QYBSABKAlCCOKMz9cIAggBUglyZXF1ZXN0SWRCDAoKX3BhcmVudF9pZEIOCgxfcmVwbHlfdG9f'
    'aWQ=');

@$core.Deprecated('Use createForumEntryResponseDescriptor instead')
const CreateForumEntryResponse$json = {
  '1': 'CreateForumEntryResponse',
  '2': [
    {
      '1': 'entry',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.ForumEntry',
      '10': 'entry'
    },
  ],
};

/// Descriptor for `CreateForumEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createForumEntryResponseDescriptor =
    $convert.base64Decode(
        'ChhDcmVhdGVGb3J1bUVudHJ5UmVzcG9uc2USNwoFZW50cnkYASABKAsyIS5oYXdrci5zb2NpYW'
        'wudjFhbHBoYTEuRm9ydW1FbnRyeVIFZW50cnk=');

@$core.Deprecated('Use updateForumEntryRequestDescriptor instead')
const UpdateForumEntryRequest$json = {
  '1': 'UpdateForumEntryRequest',
  '2': [
    {
      '1': 'entry',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.ForumEntry',
      '8': {},
      '10': 'entry'
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

/// Descriptor for `UpdateForumEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateForumEntryRequestDescriptor = $convert.base64Decode(
    'ChdVcGRhdGVGb3J1bUVudHJ5UmVxdWVzdBI8CgVlbnRyeRgBIAEoCzIhLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5Gb3J1bUVudHJ5QgPgQQJSBWVudHJ5EjsKC3VwZGF0ZV9tYXNrGAIgASgLMhou'
    'Z29vZ2xlLnByb3RvYnVmLkZpZWxkTWFza1IKdXBkYXRlTWFzaw==');

@$core.Deprecated('Use updateForumEntryResponseDescriptor instead')
const UpdateForumEntryResponse$json = {
  '1': 'UpdateForumEntryResponse',
  '2': [
    {
      '1': 'entry',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.ForumEntry',
      '10': 'entry'
    },
  ],
};

/// Descriptor for `UpdateForumEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateForumEntryResponseDescriptor =
    $convert.base64Decode(
        'ChhVcGRhdGVGb3J1bUVudHJ5UmVzcG9uc2USNwoFZW50cnkYASABKAsyIS5oYXdrci5zb2NpYW'
        'wudjFhbHBoYTEuRm9ydW1FbnRyeVIFZW50cnk=');

@$core.Deprecated('Use deleteForumEntryRequestDescriptor instead')
const DeleteForumEntryRequest$json = {
  '1': 'DeleteForumEntryRequest',
  '2': [
    {'1': 'entry_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'entryId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '10': 'etag'},
  ],
};

/// Descriptor for `DeleteForumEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteForumEntryRequestDescriptor =
    $convert.base64Decode(
        'ChdEZWxldGVGb3J1bUVudHJ5UmVxdWVzdBIeCghlbnRyeV9pZBgBIAEoCUID4EECUgdlbnRyeU'
        'lkEhIKBGV0YWcYAiABKAlSBGV0YWc=');

@$core.Deprecated('Use deleteForumEntryResponseDescriptor instead')
const DeleteForumEntryResponse$json = {
  '1': 'DeleteForumEntryResponse',
};

/// Descriptor for `DeleteForumEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteForumEntryResponseDescriptor =
    $convert.base64Decode('ChhEZWxldGVGb3J1bUVudHJ5UmVzcG9uc2U=');

@$core.Deprecated('Use createPostRequestDescriptor instead')
const CreatePostRequest$json = {
  '1': 'CreatePostRequest',
  '2': [
    {'1': 'body', '3': 1, '4': 1, '5': 9, '10': 'body'},
    {
      '1': 'image_staged_token',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'imageStagedToken'
    },
    {
      '1': 'product_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'productId',
      '17': true
    },
    {'1': 'hashtag_inputs', '3': 4, '4': 3, '5': 9, '10': 'hashtagInputs'},
    {'1': 'publish', '3': 5, '4': 1, '5': 8, '10': 'publish'},
    {'1': 'request_id', '3': 6, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
  ],
  '8': [
    {'1': '_product_id'},
  ],
};

/// Descriptor for `CreatePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPostRequestDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVQb3N0UmVxdWVzdBISCgRib2R5GAEgASgJUgRib2R5EjEKEmltYWdlX3N0YWdlZF'
    '90b2tlbhgCIAEoCUID4EEEUhBpbWFnZVN0YWdlZFRva2VuEiIKCnByb2R1Y3RfaWQYAyABKAlI'
    'AFIJcHJvZHVjdElkiAEBEiUKDmhhc2h0YWdfaW5wdXRzGAQgAygJUg1oYXNodGFnSW5wdXRzEh'
    'gKB3B1Ymxpc2gYBSABKAhSB3B1Ymxpc2gSJwoKcmVxdWVzdF9pZBgGIAEoCUII4ozP1wgCCAFS'
    'CXJlcXVlc3RJZEINCgtfcHJvZHVjdF9pZA==');

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
  ],
};

/// Descriptor for `CreatePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createPostResponseDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MWFscG'
    'hhMS5Qb3N0UgRwb3N0');

@$core.Deprecated('Use getMyPostRequestDescriptor instead')
const GetMyPostRequest$json = {
  '1': 'GetMyPostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
  ],
};

/// Descriptor for `GetMyPostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyPostRequestDescriptor = $convert.base64Decode(
    'ChBHZXRNeVBvc3RSZXF1ZXN0EhwKB3Bvc3RfaWQYASABKAlCA+BBAlIGcG9zdElk');

@$core.Deprecated('Use getMyPostResponseDescriptor instead')
const GetMyPostResponse$json = {
  '1': 'GetMyPostResponse',
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

/// Descriptor for `GetMyPostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyPostResponseDescriptor = $convert.base64Decode(
    'ChFHZXRNeVBvc3RSZXNwb25zZRIvCgRwb3N0GAEgASgLMhsuaGF3a3Iuc29jaWFsLnYxYWxwaG'
    'ExLlBvc3RSBHBvc3Q=');

@$core.Deprecated('Use listMyPostsRequestDescriptor instead')
const ListMyPostsRequest$json = {
  '1': 'ListMyPostsRequest',
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

/// Descriptor for `ListMyPostsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyPostsRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0TXlQb3N0c1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZRIdCgpwYW'
    'dlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4SOQoGc3RhdHVzGAMgASgOMiEuaGF3a3Iuc29jaWFs'
    'LnYxYWxwaGExLlBvc3RTdGF0dXNSBnN0YXR1cw==');

@$core.Deprecated('Use listMyPostsResponseDescriptor instead')
const ListMyPostsResponse$json = {
  '1': 'ListMyPostsResponse',
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

/// Descriptor for `ListMyPostsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyPostsResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0TXlQb3N0c1Jlc3BvbnNlEjEKBXBvc3RzGAEgAygLMhsuaGF3a3Iuc29jaWFsLnYxYW'
    'xwaGExLlBvc3RSBXBvc3RzEiYKD25leHRfcGFnZV90b2tlbhgCIAEoCVINbmV4dFBhZ2VUb2tl'
    'bg==');

@$core.Deprecated('Use updatePostRequestDescriptor instead')
const UpdatePostRequest$json = {
  '1': 'UpdatePostRequest',
  '2': [
    {
      '1': 'post',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Post',
      '8': {},
      '10': 'post'
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

/// Descriptor for `UpdatePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePostRequestDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVQb3N0UmVxdWVzdBI0CgRwb3N0GAEgASgLMhsuaGF3a3Iuc29jaWFsLnYxYWxwaG'
    'ExLlBvc3RCA+BBAlIEcG9zdBI7Cgt1cGRhdGVfbWFzaxgCIAEoCzIaLmdvb2dsZS5wcm90b2J1'
    'Zi5GaWVsZE1hc2tSCnVwZGF0ZU1hc2s=');

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
  ],
};

/// Descriptor for `UpdatePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePostResponseDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MWFscG'
    'hhMS5Qb3N0UgRwb3N0');

@$core.Deprecated('Use publishPostRequestDescriptor instead')
const PublishPostRequest$json = {
  '1': 'PublishPostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `PublishPostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishPostRequestDescriptor = $convert.base64Decode(
    'ChJQdWJsaXNoUG9zdFJlcXVlc3QSHAoHcG9zdF9pZBgBIAEoCUID4EECUgZwb3N0SWQSFwoEZX'
    'RhZxgCIAEoCUID4EECUgRldGFn');

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
  ],
};

/// Descriptor for `PublishPostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishPostResponseDescriptor = $convert.base64Decode(
    'ChNQdWJsaXNoUG9zdFJlc3BvbnNlEi8KBHBvc3QYASABKAsyGy5oYXdrci5zb2NpYWwudjFhbH'
    'BoYTEuUG9zdFIEcG9zdA==');

@$core.Deprecated('Use unpublishPostRequestDescriptor instead')
const UnpublishPostRequest$json = {
  '1': 'UnpublishPostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'etag'},
  ],
};

/// Descriptor for `UnpublishPostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishPostRequestDescriptor = $convert.base64Decode(
    'ChRVbnB1Ymxpc2hQb3N0UmVxdWVzdBIcCgdwb3N0X2lkGAEgASgJQgPgQQJSBnBvc3RJZBIXCg'
    'RldGFnGAIgASgJQgPgQQJSBGV0YWc=');

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
  ],
};

/// Descriptor for `UnpublishPostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpublishPostResponseDescriptor = $convert.base64Decode(
    'ChVVbnB1Ymxpc2hQb3N0UmVzcG9uc2USLwoEcG9zdBgBIAEoCzIbLmhhd2tyLnNvY2lhbC52MW'
    'FscGhhMS5Qb3N0UgRwb3N0');

@$core.Deprecated('Use deletePostRequestDescriptor instead')
const DeletePostRequest$json = {
  '1': 'DeletePostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '10': 'etag'},
  ],
};

/// Descriptor for `DeletePostRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deletePostRequestDescriptor = $convert.base64Decode(
    'ChFEZWxldGVQb3N0UmVxdWVzdBIcCgdwb3N0X2lkGAEgASgJQgPgQQJSBnBvc3RJZBISCgRldG'
    'FnGAIgASgJUgRldGFn');

@$core.Deprecated('Use deletePostResponseDescriptor instead')
const DeletePostResponse$json = {
  '1': 'DeletePostResponse',
};

/// Descriptor for `DeletePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deletePostResponseDescriptor =
    $convert.base64Decode('ChJEZWxldGVQb3N0UmVzcG9uc2U=');

@$core.Deprecated('Use schedulePostRequestDescriptor instead')
const SchedulePostRequest$json = {
  '1': 'SchedulePostRequest',
  '2': [
    {'1': 'post_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'postId'},
    {'1': 'etag', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'etag'},
    {
      '1': 'publish_time',
      '3': 3,
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
    'ChNTY2hlZHVsZVBvc3RSZXF1ZXN0EhwKB3Bvc3RfaWQYASABKAlCA+BBAlIGcG9zdElkEhcKBG'
    'V0YWcYAiABKAlCA+BBAlIEZXRhZxJCCgxwdWJsaXNoX3RpbWUYAyABKAsyGi5nb29nbGUucHJv'
    'dG9idWYuVGltZXN0YW1wQgPgQQJSC3B1Ymxpc2hUaW1l');

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
  ],
};

/// Descriptor for `SchedulePostResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List schedulePostResponseDescriptor = $convert.base64Decode(
    'ChRTY2hlZHVsZVBvc3RSZXNwb25zZRIvCgRwb3N0GAEgASgLMhsuaGF3a3Iuc29jaWFsLnYxYW'
    'xwaGExLlBvc3RSBHBvc3QSPQoMcHVibGlzaF90aW1lGAIgASgLMhouZ29vZ2xlLnByb3RvYnVm'
    'LlRpbWVzdGFtcFILcHVibGlzaFRpbWU=');

@$core.Deprecated('Use listStoreCommentsRequestDescriptor instead')
const ListStoreCommentsRequest$json = {
  '1': 'ListStoreCommentsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
    {
      '1': 'post_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'postId',
      '17': true
    },
    {'1': 'unreplied', '3': 4, '4': 1, '5': 8, '10': 'unreplied'},
  ],
  '8': [
    {'1': '_post_id'},
  ],
};

/// Descriptor for `ListStoreCommentsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreCommentsRequestDescriptor = $convert.base64Decode(
    'ChhMaXN0U3RvcmVDb21tZW50c1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZR'
    'IdCgpwYWdlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4SHAoHcG9zdF9pZBgDIAEoCUgAUgZwb3N0'
    'SWSIAQESHAoJdW5yZXBsaWVkGAQgASgIUgl1bnJlcGxpZWRCCgoIX3Bvc3RfaWQ=');

@$core.Deprecated('Use listStoreCommentsResponseDescriptor instead')
const ListStoreCommentsResponse$json = {
  '1': 'ListStoreCommentsResponse',
  '2': [
    {
      '1': 'comments',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Comment',
      '10': 'comments'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListStoreCommentsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreCommentsResponseDescriptor = $convert.base64Decode(
    'ChlMaXN0U3RvcmVDb21tZW50c1Jlc3BvbnNlEjoKCGNvbW1lbnRzGAEgAygLMh4uaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLkNvbW1lbnRSCGNvbW1lbnRzEiYKD25leHRfcGFnZV90b2tlbhgCIAEo'
    'CVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use getStoreSocialSummaryRequestDescriptor instead')
const GetStoreSocialSummaryRequest$json = {
  '1': 'GetStoreSocialSummaryRequest',
};

/// Descriptor for `GetStoreSocialSummaryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreSocialSummaryRequestDescriptor =
    $convert.base64Decode('ChxHZXRTdG9yZVNvY2lhbFN1bW1hcnlSZXF1ZXN0');

@$core.Deprecated('Use getStoreSocialSummaryResponseDescriptor instead')
const GetStoreSocialSummaryResponse$json = {
  '1': 'GetStoreSocialSummaryResponse',
  '2': [
    {
      '1': 'post_counts',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.PostStatusCount',
      '10': 'postCounts'
    },
    {
      '1': 'scheduled_post_count',
      '3': 2,
      '4': 1,
      '5': 3,
      '10': 'scheduledPostCount'
    },
  ],
};

/// Descriptor for `GetStoreSocialSummaryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreSocialSummaryResponseDescriptor =
    $convert.base64Decode(
        'Ch1HZXRTdG9yZVNvY2lhbFN1bW1hcnlSZXNwb25zZRJHCgtwb3N0X2NvdW50cxgBIAMoCzImLm'
        'hhd2tyLnNvY2lhbC52MWFscGhhMS5Qb3N0U3RhdHVzQ291bnRSCnBvc3RDb3VudHMSMAoUc2No'
        'ZWR1bGVkX3Bvc3RfY291bnQYAiABKANSEnNjaGVkdWxlZFBvc3RDb3VudA==');

@$core.Deprecated('Use postStatusCountDescriptor instead')
const PostStatusCount$json = {
  '1': 'PostStatusCount',
  '2': [
    {
      '1': 'status',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.PostStatus',
      '10': 'status'
    },
    {'1': 'count', '3': 2, '4': 1, '5': 3, '10': 'count'},
  ],
};

/// Descriptor for `PostStatusCount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List postStatusCountDescriptor = $convert.base64Decode(
    'Cg9Qb3N0U3RhdHVzQ291bnQSOQoGc3RhdHVzGAEgASgOMiEuaGF3a3Iuc29jaWFsLnYxYWxwaG'
    'ExLlBvc3RTdGF0dXNSBnN0YXR1cxIUCgVjb3VudBgCIAEoA1IFY291bnQ=');

@$core.Deprecated('Use listMySanctionsRequestDescriptor instead')
const ListMySanctionsRequest$json = {
  '1': 'ListMySanctionsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListMySanctionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMySanctionsRequestDescriptor =
    $convert.base64Decode(
        'ChZMaXN0TXlTYW5jdGlvbnNSZXF1ZXN0EhsKCXBhZ2Vfc2l6ZRgBIAEoBVIIcGFnZVNpemUSHQ'
        'oKcGFnZV90b2tlbhgCIAEoCVIJcGFnZVRva2Vu');

@$core.Deprecated('Use listMySanctionsResponseDescriptor instead')
const ListMySanctionsResponse$json = {
  '1': 'ListMySanctionsResponse',
  '2': [
    {
      '1': 'sanctions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.Sanction',
      '10': 'sanctions'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMySanctionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMySanctionsResponseDescriptor = $convert.base64Decode(
    'ChdMaXN0TXlTYW5jdGlvbnNSZXNwb25zZRI9CglzYW5jdGlvbnMYASADKAsyHy5oYXdrci5zb2'
    'NpYWwudjFhbHBoYTEuU2FuY3Rpb25SCXNhbmN0aW9ucxImCg9uZXh0X3BhZ2VfdG9rZW4YAiAB'
    'KAlSDW5leHRQYWdlVG9rZW4=');

@$core.Deprecated('Use sanctionDescriptor instead')
const Sanction$json = {
  '1': 'Sanction',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.SanctionKind',
      '10': 'kind'
    },
    {
      '1': 'category',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '10': 'category'
    },
    {'1': 'reason', '3': 4, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'create_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
  ],
};

/// Descriptor for `Sanction`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sanctionDescriptor = $convert.base64Decode(
    'CghTYW5jdGlvbhIOCgJpZBgBIAEoCVICaWQSNwoEa2luZBgCIAEoDjIjLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5TYW5jdGlvbktpbmRSBGtpbmQSQQoIY2F0ZWdvcnkYAyABKA4yJS5oYXdrci5j'
    'b21tb24udjFhbHBoYTEuUG9saWN5Q2F0ZWdvcnlSCGNhdGVnb3J5EhYKBnJlYXNvbhgEIAEoCV'
    'IGcmVhc29uEjsKC2NyZWF0ZV90aW1lGAUgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFt'
    'cFIKY3JlYXRlVGltZQ==');

@$core.Deprecated('Use listMyHiddenContentRequestDescriptor instead')
const ListMyHiddenContentRequest$json = {
  '1': 'ListMyHiddenContentRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListMyHiddenContentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyHiddenContentRequestDescriptor =
    $convert.base64Decode(
        'ChpMaXN0TXlIaWRkZW5Db250ZW50UmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2VTaX'
        'plEh0KCnBhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listMyHiddenContentResponseDescriptor instead')
const ListMyHiddenContentResponse$json = {
  '1': 'ListMyHiddenContentResponse',
  '2': [
    {
      '1': 'hidden_contents',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.social.v1alpha1.HiddenContent',
      '10': 'hiddenContents'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListMyHiddenContentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyHiddenContentResponseDescriptor =
    $convert.base64Decode(
        'ChtMaXN0TXlIaWRkZW5Db250ZW50UmVzcG9uc2USTQoPaGlkZGVuX2NvbnRlbnRzGAEgAygLMi'
        'QuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkhpZGRlbkNvbnRlbnRSDmhpZGRlbkNvbnRlbnRzEiYK'
        'D25leHRfcGFnZV90b2tlbhgCIAEoCVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use hiddenContentDescriptor instead')
const HiddenContent$json = {
  '1': 'HiddenContent',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.hawkr.social.v1alpha1.HiddenContentKind',
      '10': 'kind'
    },
    {'1': 'content_id', '3': 3, '4': 1, '5': 9, '10': 'contentId'},
    {
      '1': 'post_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'postId',
      '17': true
    },
    {
      '1': 'store_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'storeId',
      '17': true
    },
    {'1': 'body', '3': 6, '4': 1, '5': 9, '10': 'body'},
    {
      '1': 'create_time',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createTime'
    },
    {
      '1': 'category',
      '3': 8,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.PolicyCategory',
      '10': 'category'
    },
    {'1': 'reason', '3': 9, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'hide_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'hideTime'
    },
  ],
  '8': [
    {'1': '_post_id'},
    {'1': '_store_id'},
  ],
};

/// Descriptor for `HiddenContent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hiddenContentDescriptor = $convert.base64Decode(
    'Cg1IaWRkZW5Db250ZW50Eg4KAmlkGAEgASgJUgJpZBI8CgRraW5kGAIgASgOMiguaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLkhpZGRlbkNvbnRlbnRLaW5kUgRraW5kEh0KCmNvbnRlbnRfaWQYAyAB'
    'KAlSCWNvbnRlbnRJZBIcCgdwb3N0X2lkGAQgASgJSABSBnBvc3RJZIgBARIeCghzdG9yZV9pZB'
    'gFIAEoCUgBUgdzdG9yZUlkiAEBEhIKBGJvZHkYBiABKAlSBGJvZHkSOwoLY3JlYXRlX3RpbWUY'
    'ByABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpjcmVhdGVUaW1lEkEKCGNhdGVnb3'
    'J5GAggASgOMiUuaGF3a3IuY29tbW9uLnYxYWxwaGExLlBvbGljeUNhdGVnb3J5UghjYXRlZ29y'
    'eRIWCgZyZWFzb24YCSABKAlSBnJlYXNvbhI3CgloaWRlX3RpbWUYCiABKAsyGi5nb29nbGUucH'
    'JvdG9idWYuVGltZXN0YW1wUghoaWRlVGltZUIKCghfcG9zdF9pZEILCglfc3RvcmVfaWQ=');

const $core.Map<$core.String, $core.dynamic> SocialPublicServiceBase$json = {
  '1': 'SocialPublicService',
  '2': [
    {
      '1': 'GetPost',
      '2': '.hawkr.social.v1alpha1.GetPostRequest',
      '3': '.hawkr.social.v1alpha1.GetPostResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListPosts',
      '2': '.hawkr.social.v1alpha1.ListPostsRequest',
      '3': '.hawkr.social.v1alpha1.ListPostsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListComments',
      '2': '.hawkr.social.v1alpha1.ListCommentsRequest',
      '3': '.hawkr.social.v1alpha1.ListCommentsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SuggestTrendingPosts',
      '2': '.hawkr.social.v1alpha1.SuggestTrendingPostsRequest',
      '3': '.hawkr.social.v1alpha1.SuggestTrendingPostsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SuggestTags',
      '2': '.hawkr.social.v1alpha1.SuggestTagsRequest',
      '3': '.hawkr.social.v1alpha1.SuggestTagsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SuggestTrendingTags',
      '2': '.hawkr.social.v1alpha1.SuggestTrendingTagsRequest',
      '3': '.hawkr.social.v1alpha1.SuggestTrendingTagsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListForumEntries',
      '2': '.hawkr.social.v1alpha1.ListForumEntriesRequest',
      '3': '.hawkr.social.v1alpha1.ListForumEntriesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'RecordPostView',
      '2': '.hawkr.social.v1alpha1.RecordPostViewRequest',
      '3': '.hawkr.social.v1alpha1.RecordPostViewResponse'
    },
  ],
};

@$core.Deprecated('Use socialPublicServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SocialPublicServiceBase$messageJson = {
  '.hawkr.social.v1alpha1.GetPostRequest': GetPostRequest$json,
  '.hawkr.social.v1alpha1.GetPostResponse': GetPostResponse$json,
  '.hawkr.social.v1alpha1.Post': Post$json,
  '.hawkr.common.v1alpha1.StoreCard': $0.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $1.Image$json,
  '.hawkr.social.v1alpha1.Hashtag': Hashtag$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.social.v1alpha1.ListPostsRequest': ListPostsRequest$json,
  '.hawkr.social.v1alpha1.ListPostsResponse': ListPostsResponse$json,
  '.hawkr.social.v1alpha1.ListCommentsRequest': ListCommentsRequest$json,
  '.hawkr.social.v1alpha1.ListCommentsResponse': ListCommentsResponse$json,
  '.hawkr.social.v1alpha1.Comment': Comment$json,
  '.hawkr.social.v1alpha1.Actor': Actor$json,
  '.hawkr.social.v1alpha1.SuggestTrendingPostsRequest':
      SuggestTrendingPostsRequest$json,
  '.hawkr.social.v1alpha1.SuggestTrendingPostsResponse':
      SuggestTrendingPostsResponse$json,
  '.hawkr.social.v1alpha1.SuggestTagsRequest': SuggestTagsRequest$json,
  '.hawkr.social.v1alpha1.SuggestTagsResponse': SuggestTagsResponse$json,
  '.hawkr.social.v1alpha1.TagCount': TagCount$json,
  '.hawkr.social.v1alpha1.SuggestTrendingTagsRequest':
      SuggestTrendingTagsRequest$json,
  '.hawkr.social.v1alpha1.SuggestTrendingTagsResponse':
      SuggestTrendingTagsResponse$json,
  '.hawkr.social.v1alpha1.ListForumEntriesRequest':
      ListForumEntriesRequest$json,
  '.hawkr.social.v1alpha1.ListForumEntriesResponse':
      ListForumEntriesResponse$json,
  '.hawkr.social.v1alpha1.ForumEntry': ForumEntry$json,
  '.hawkr.social.v1alpha1.RecordPostViewRequest': RecordPostViewRequest$json,
  '.hawkr.social.v1alpha1.RecordPostViewResponse': RecordPostViewResponse$json,
};

/// Descriptor for `SocialPublicService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List socialPublicServiceDescriptor = $convert.base64Decode(
    'ChNTb2NpYWxQdWJsaWNTZXJ2aWNlEl0KB0dldFBvc3QSJS5oYXdrci5zb2NpYWwudjFhbHBoYT'
    'EuR2V0UG9zdFJlcXVlc3QaJi5oYXdrci5zb2NpYWwudjFhbHBoYTEuR2V0UG9zdFJlc3BvbnNl'
    'IgOQAgESYwoJTGlzdFBvc3RzEicuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkxpc3RQb3N0c1JlcX'
    'Vlc3QaKC5oYXdrci5zb2NpYWwudjFhbHBoYTEuTGlzdFBvc3RzUmVzcG9uc2UiA5ACARJsCgxM'
    'aXN0Q29tbWVudHMSKi5oYXdrci5zb2NpYWwudjFhbHBoYTEuTGlzdENvbW1lbnRzUmVxdWVzdB'
    'orLmhhd2tyLnNvY2lhbC52MWFscGhhMS5MaXN0Q29tbWVudHNSZXNwb25zZSIDkAIBEoQBChRT'
    'dWdnZXN0VHJlbmRpbmdQb3N0cxIyLmhhd2tyLnNvY2lhbC52MWFscGhhMS5TdWdnZXN0VHJlbm'
    'RpbmdQb3N0c1JlcXVlc3QaMy5oYXdrci5zb2NpYWwudjFhbHBoYTEuU3VnZ2VzdFRyZW5kaW5n'
    'UG9zdHNSZXNwb25zZSIDkAIBEmkKC1N1Z2dlc3RUYWdzEikuaGF3a3Iuc29jaWFsLnYxYWxwaG'
    'ExLlN1Z2dlc3RUYWdzUmVxdWVzdBoqLmhhd2tyLnNvY2lhbC52MWFscGhhMS5TdWdnZXN0VGFn'
    'c1Jlc3BvbnNlIgOQAgESgQEKE1N1Z2dlc3RUcmVuZGluZ1RhZ3MSMS5oYXdrci5zb2NpYWwudj'
    'FhbHBoYTEuU3VnZ2VzdFRyZW5kaW5nVGFnc1JlcXVlc3QaMi5oYXdrci5zb2NpYWwudjFhbHBo'
    'YTEuU3VnZ2VzdFRyZW5kaW5nVGFnc1Jlc3BvbnNlIgOQAgESeAoQTGlzdEZvcnVtRW50cmllcx'
    'IuLmhhd2tyLnNvY2lhbC52MWFscGhhMS5MaXN0Rm9ydW1FbnRyaWVzUmVxdWVzdBovLmhhd2ty'
    'LnNvY2lhbC52MWFscGhhMS5MaXN0Rm9ydW1FbnRyaWVzUmVzcG9uc2UiA5ACARJtCg5SZWNvcm'
    'RQb3N0VmlldxIsLmhhd2tyLnNvY2lhbC52MWFscGhhMS5SZWNvcmRQb3N0Vmlld1JlcXVlc3Qa'
    'LS5oYXdrci5zb2NpYWwudjFhbHBoYTEuUmVjb3JkUG9zdFZpZXdSZXNwb25zZQ==');

const $core.Map<$core.String, $core.dynamic> SocialAppServiceBase$json = {
  '1': 'SocialAppService',
  '2': [
    {
      '1': 'SharePost',
      '2': '.hawkr.social.v1alpha1.SharePostRequest',
      '3': '.hawkr.social.v1alpha1.SharePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'LikePost',
      '2': '.hawkr.social.v1alpha1.LikePostRequest',
      '3': '.hawkr.social.v1alpha1.LikePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnlikePost',
      '2': '.hawkr.social.v1alpha1.UnlikePostRequest',
      '3': '.hawkr.social.v1alpha1.UnlikePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'LikeComment',
      '2': '.hawkr.social.v1alpha1.LikeCommentRequest',
      '3': '.hawkr.social.v1alpha1.LikeCommentResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnlikeComment',
      '2': '.hawkr.social.v1alpha1.UnlikeCommentRequest',
      '3': '.hawkr.social.v1alpha1.UnlikeCommentResponse',
      '4': {'34': 2},
    },
    {
      '1': 'CreateComment',
      '2': '.hawkr.social.v1alpha1.CreateCommentRequest',
      '3': '.hawkr.social.v1alpha1.CreateCommentResponse'
    },
    {
      '1': 'UpdateComment',
      '2': '.hawkr.social.v1alpha1.UpdateCommentRequest',
      '3': '.hawkr.social.v1alpha1.UpdateCommentResponse',
      '4': {'34': 2},
    },
    {
      '1': 'DeleteComment',
      '2': '.hawkr.social.v1alpha1.DeleteCommentRequest',
      '3': '.hawkr.social.v1alpha1.DeleteCommentResponse',
      '4': {'34': 2},
    },
    {
      '1': 'CreateForumEntry',
      '2': '.hawkr.social.v1alpha1.CreateForumEntryRequest',
      '3': '.hawkr.social.v1alpha1.CreateForumEntryResponse'
    },
    {
      '1': 'UpdateForumEntry',
      '2': '.hawkr.social.v1alpha1.UpdateForumEntryRequest',
      '3': '.hawkr.social.v1alpha1.UpdateForumEntryResponse',
      '4': {'34': 2},
    },
    {
      '1': 'DeleteForumEntry',
      '2': '.hawkr.social.v1alpha1.DeleteForumEntryRequest',
      '3': '.hawkr.social.v1alpha1.DeleteForumEntryResponse',
      '4': {'34': 2},
    },
    {
      '1': 'CreatePost',
      '2': '.hawkr.social.v1alpha1.CreatePostRequest',
      '3': '.hawkr.social.v1alpha1.CreatePostResponse'
    },
    {
      '1': 'GetMyPost',
      '2': '.hawkr.social.v1alpha1.GetMyPostRequest',
      '3': '.hawkr.social.v1alpha1.GetMyPostResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListMyPosts',
      '2': '.hawkr.social.v1alpha1.ListMyPostsRequest',
      '3': '.hawkr.social.v1alpha1.ListMyPostsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'UpdatePost',
      '2': '.hawkr.social.v1alpha1.UpdatePostRequest',
      '3': '.hawkr.social.v1alpha1.UpdatePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'PublishPost',
      '2': '.hawkr.social.v1alpha1.PublishPostRequest',
      '3': '.hawkr.social.v1alpha1.PublishPostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'UnpublishPost',
      '2': '.hawkr.social.v1alpha1.UnpublishPostRequest',
      '3': '.hawkr.social.v1alpha1.UnpublishPostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'DeletePost',
      '2': '.hawkr.social.v1alpha1.DeletePostRequest',
      '3': '.hawkr.social.v1alpha1.DeletePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'SchedulePost',
      '2': '.hawkr.social.v1alpha1.SchedulePostRequest',
      '3': '.hawkr.social.v1alpha1.SchedulePostResponse',
      '4': {'34': 2},
    },
    {
      '1': 'ListStoreComments',
      '2': '.hawkr.social.v1alpha1.ListStoreCommentsRequest',
      '3': '.hawkr.social.v1alpha1.ListStoreCommentsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetStoreSocialSummary',
      '2': '.hawkr.social.v1alpha1.GetStoreSocialSummaryRequest',
      '3': '.hawkr.social.v1alpha1.GetStoreSocialSummaryResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListMySanctions',
      '2': '.hawkr.social.v1alpha1.ListMySanctionsRequest',
      '3': '.hawkr.social.v1alpha1.ListMySanctionsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListMyHiddenContent',
      '2': '.hawkr.social.v1alpha1.ListMyHiddenContentRequest',
      '3': '.hawkr.social.v1alpha1.ListMyHiddenContentResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use socialAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    SocialAppServiceBase$messageJson = {
  '.hawkr.social.v1alpha1.SharePostRequest': SharePostRequest$json,
  '.hawkr.social.v1alpha1.SharePostResponse': SharePostResponse$json,
  '.hawkr.social.v1alpha1.LikePostRequest': LikePostRequest$json,
  '.hawkr.social.v1alpha1.LikePostResponse': LikePostResponse$json,
  '.hawkr.social.v1alpha1.UnlikePostRequest': UnlikePostRequest$json,
  '.hawkr.social.v1alpha1.UnlikePostResponse': UnlikePostResponse$json,
  '.hawkr.social.v1alpha1.LikeCommentRequest': LikeCommentRequest$json,
  '.hawkr.social.v1alpha1.LikeCommentResponse': LikeCommentResponse$json,
  '.hawkr.social.v1alpha1.UnlikeCommentRequest': UnlikeCommentRequest$json,
  '.hawkr.social.v1alpha1.UnlikeCommentResponse': UnlikeCommentResponse$json,
  '.hawkr.social.v1alpha1.CreateCommentRequest': CreateCommentRequest$json,
  '.hawkr.social.v1alpha1.CreateCommentResponse': CreateCommentResponse$json,
  '.hawkr.social.v1alpha1.Comment': Comment$json,
  '.hawkr.social.v1alpha1.Actor': Actor$json,
  '.hawkr.common.v1alpha1.Image': $1.Image$json,
  '.google.protobuf.Timestamp': $2.Timestamp$json,
  '.hawkr.social.v1alpha1.UpdateCommentRequest': UpdateCommentRequest$json,
  '.google.protobuf.FieldMask': $3.FieldMask$json,
  '.hawkr.social.v1alpha1.UpdateCommentResponse': UpdateCommentResponse$json,
  '.hawkr.social.v1alpha1.DeleteCommentRequest': DeleteCommentRequest$json,
  '.hawkr.social.v1alpha1.DeleteCommentResponse': DeleteCommentResponse$json,
  '.hawkr.social.v1alpha1.CreateForumEntryRequest':
      CreateForumEntryRequest$json,
  '.hawkr.social.v1alpha1.CreateForumEntryResponse':
      CreateForumEntryResponse$json,
  '.hawkr.social.v1alpha1.ForumEntry': ForumEntry$json,
  '.hawkr.social.v1alpha1.UpdateForumEntryRequest':
      UpdateForumEntryRequest$json,
  '.hawkr.social.v1alpha1.UpdateForumEntryResponse':
      UpdateForumEntryResponse$json,
  '.hawkr.social.v1alpha1.DeleteForumEntryRequest':
      DeleteForumEntryRequest$json,
  '.hawkr.social.v1alpha1.DeleteForumEntryResponse':
      DeleteForumEntryResponse$json,
  '.hawkr.social.v1alpha1.CreatePostRequest': CreatePostRequest$json,
  '.hawkr.social.v1alpha1.CreatePostResponse': CreatePostResponse$json,
  '.hawkr.social.v1alpha1.Post': Post$json,
  '.hawkr.common.v1alpha1.StoreCard': $0.StoreCard$json,
  '.hawkr.social.v1alpha1.Hashtag': Hashtag$json,
  '.hawkr.social.v1alpha1.GetMyPostRequest': GetMyPostRequest$json,
  '.hawkr.social.v1alpha1.GetMyPostResponse': GetMyPostResponse$json,
  '.hawkr.social.v1alpha1.ListMyPostsRequest': ListMyPostsRequest$json,
  '.hawkr.social.v1alpha1.ListMyPostsResponse': ListMyPostsResponse$json,
  '.hawkr.social.v1alpha1.UpdatePostRequest': UpdatePostRequest$json,
  '.hawkr.social.v1alpha1.UpdatePostResponse': UpdatePostResponse$json,
  '.hawkr.social.v1alpha1.PublishPostRequest': PublishPostRequest$json,
  '.hawkr.social.v1alpha1.PublishPostResponse': PublishPostResponse$json,
  '.hawkr.social.v1alpha1.UnpublishPostRequest': UnpublishPostRequest$json,
  '.hawkr.social.v1alpha1.UnpublishPostResponse': UnpublishPostResponse$json,
  '.hawkr.social.v1alpha1.DeletePostRequest': DeletePostRequest$json,
  '.hawkr.social.v1alpha1.DeletePostResponse': DeletePostResponse$json,
  '.hawkr.social.v1alpha1.SchedulePostRequest': SchedulePostRequest$json,
  '.hawkr.social.v1alpha1.SchedulePostResponse': SchedulePostResponse$json,
  '.hawkr.social.v1alpha1.ListStoreCommentsRequest':
      ListStoreCommentsRequest$json,
  '.hawkr.social.v1alpha1.ListStoreCommentsResponse':
      ListStoreCommentsResponse$json,
  '.hawkr.social.v1alpha1.GetStoreSocialSummaryRequest':
      GetStoreSocialSummaryRequest$json,
  '.hawkr.social.v1alpha1.GetStoreSocialSummaryResponse':
      GetStoreSocialSummaryResponse$json,
  '.hawkr.social.v1alpha1.PostStatusCount': PostStatusCount$json,
  '.hawkr.social.v1alpha1.ListMySanctionsRequest': ListMySanctionsRequest$json,
  '.hawkr.social.v1alpha1.ListMySanctionsResponse':
      ListMySanctionsResponse$json,
  '.hawkr.social.v1alpha1.Sanction': Sanction$json,
  '.hawkr.social.v1alpha1.ListMyHiddenContentRequest':
      ListMyHiddenContentRequest$json,
  '.hawkr.social.v1alpha1.ListMyHiddenContentResponse':
      ListMyHiddenContentResponse$json,
  '.hawkr.social.v1alpha1.HiddenContent': HiddenContent$json,
};

/// Descriptor for `SocialAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List socialAppServiceDescriptor = $convert.base64Decode(
    'ChBTb2NpYWxBcHBTZXJ2aWNlEmMKCVNoYXJlUG9zdBInLmhhd2tyLnNvY2lhbC52MWFscGhhMS'
    '5TaGFyZVBvc3RSZXF1ZXN0GiguaGF3a3Iuc29jaWFsLnYxYWxwaGExLlNoYXJlUG9zdFJlc3Bv'
    'bnNlIgOQAgISYAoITGlrZVBvc3QSJi5oYXdrci5zb2NpYWwudjFhbHBoYTEuTGlrZVBvc3RSZX'
    'F1ZXN0GicuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkxpa2VQb3N0UmVzcG9uc2UiA5ACAhJmCgpV'
    'bmxpa2VQb3N0EiguaGF3a3Iuc29jaWFsLnYxYWxwaGExLlVubGlrZVBvc3RSZXF1ZXN0GikuaG'
    'F3a3Iuc29jaWFsLnYxYWxwaGExLlVubGlrZVBvc3RSZXNwb25zZSIDkAICEmkKC0xpa2VDb21t'
    'ZW50EikuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkxpa2VDb21tZW50UmVxdWVzdBoqLmhhd2tyLn'
    'NvY2lhbC52MWFscGhhMS5MaWtlQ29tbWVudFJlc3BvbnNlIgOQAgISbwoNVW5saWtlQ29tbWVu'
    'dBIrLmhhd2tyLnNvY2lhbC52MWFscGhhMS5Vbmxpa2VDb21tZW50UmVxdWVzdBosLmhhd2tyLn'
    'NvY2lhbC52MWFscGhhMS5Vbmxpa2VDb21tZW50UmVzcG9uc2UiA5ACAhJqCg1DcmVhdGVDb21t'
    'ZW50EisuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkNyZWF0ZUNvbW1lbnRSZXF1ZXN0GiwuaGF3a3'
    'Iuc29jaWFsLnYxYWxwaGExLkNyZWF0ZUNvbW1lbnRSZXNwb25zZRJvCg1VcGRhdGVDb21tZW50'
    'EisuaGF3a3Iuc29jaWFsLnYxYWxwaGExLlVwZGF0ZUNvbW1lbnRSZXF1ZXN0GiwuaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLlVwZGF0ZUNvbW1lbnRSZXNwb25zZSIDkAICEm8KDURlbGV0ZUNvbW1l'
    'bnQSKy5oYXdrci5zb2NpYWwudjFhbHBoYTEuRGVsZXRlQ29tbWVudFJlcXVlc3QaLC5oYXdrci'
    '5zb2NpYWwudjFhbHBoYTEuRGVsZXRlQ29tbWVudFJlc3BvbnNlIgOQAgIScwoQQ3JlYXRlRm9y'
    'dW1FbnRyeRIuLmhhd2tyLnNvY2lhbC52MWFscGhhMS5DcmVhdGVGb3J1bUVudHJ5UmVxdWVzdB'
    'ovLmhhd2tyLnNvY2lhbC52MWFscGhhMS5DcmVhdGVGb3J1bUVudHJ5UmVzcG9uc2USeAoQVXBk'
    'YXRlRm9ydW1FbnRyeRIuLmhhd2tyLnNvY2lhbC52MWFscGhhMS5VcGRhdGVGb3J1bUVudHJ5Um'
    'VxdWVzdBovLmhhd2tyLnNvY2lhbC52MWFscGhhMS5VcGRhdGVGb3J1bUVudHJ5UmVzcG9uc2Ui'
    'A5ACAhJ4ChBEZWxldGVGb3J1bUVudHJ5Ei4uaGF3a3Iuc29jaWFsLnYxYWxwaGExLkRlbGV0ZU'
    'ZvcnVtRW50cnlSZXF1ZXN0Gi8uaGF3a3Iuc29jaWFsLnYxYWxwaGExLkRlbGV0ZUZvcnVtRW50'
    'cnlSZXNwb25zZSIDkAICEmEKCkNyZWF0ZVBvc3QSKC5oYXdrci5zb2NpYWwudjFhbHBoYTEuQ3'
    'JlYXRlUG9zdFJlcXVlc3QaKS5oYXdrci5zb2NpYWwudjFhbHBoYTEuQ3JlYXRlUG9zdFJlc3Bv'
    'bnNlEmMKCUdldE15UG9zdBInLmhhd2tyLnNvY2lhbC52MWFscGhhMS5HZXRNeVBvc3RSZXF1ZX'
    'N0GiguaGF3a3Iuc29jaWFsLnYxYWxwaGExLkdldE15UG9zdFJlc3BvbnNlIgOQAgESaQoLTGlz'
    'dE15UG9zdHMSKS5oYXdrci5zb2NpYWwudjFhbHBoYTEuTGlzdE15UG9zdHNSZXF1ZXN0GiouaG'
    'F3a3Iuc29jaWFsLnYxYWxwaGExLkxpc3RNeVBvc3RzUmVzcG9uc2UiA5ACARJmCgpVcGRhdGVQ'
    'b3N0EiguaGF3a3Iuc29jaWFsLnYxYWxwaGExLlVwZGF0ZVBvc3RSZXF1ZXN0GikuaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLlVwZGF0ZVBvc3RSZXNwb25zZSIDkAICEmkKC1B1Ymxpc2hQb3N0Eiku'
    'aGF3a3Iuc29jaWFsLnYxYWxwaGExLlB1Ymxpc2hQb3N0UmVxdWVzdBoqLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5QdWJsaXNoUG9zdFJlc3BvbnNlIgOQAgISbwoNVW5wdWJsaXNoUG9zdBIrLmhh'
    'd2tyLnNvY2lhbC52MWFscGhhMS5VbnB1Ymxpc2hQb3N0UmVxdWVzdBosLmhhd2tyLnNvY2lhbC'
    '52MWFscGhhMS5VbnB1Ymxpc2hQb3N0UmVzcG9uc2UiA5ACAhJmCgpEZWxldGVQb3N0EiguaGF3'
    'a3Iuc29jaWFsLnYxYWxwaGExLkRlbGV0ZVBvc3RSZXF1ZXN0GikuaGF3a3Iuc29jaWFsLnYxYW'
    'xwaGExLkRlbGV0ZVBvc3RSZXNwb25zZSIDkAICEmwKDFNjaGVkdWxlUG9zdBIqLmhhd2tyLnNv'
    'Y2lhbC52MWFscGhhMS5TY2hlZHVsZVBvc3RSZXF1ZXN0GisuaGF3a3Iuc29jaWFsLnYxYWxwaG'
    'ExLlNjaGVkdWxlUG9zdFJlc3BvbnNlIgOQAgISewoRTGlzdFN0b3JlQ29tbWVudHMSLy5oYXdr'
    'ci5zb2NpYWwudjFhbHBoYTEuTGlzdFN0b3JlQ29tbWVudHNSZXF1ZXN0GjAuaGF3a3Iuc29jaW'
    'FsLnYxYWxwaGExLkxpc3RTdG9yZUNvbW1lbnRzUmVzcG9uc2UiA5ACARKHAQoVR2V0U3RvcmVT'
    'b2NpYWxTdW1tYXJ5EjMuaGF3a3Iuc29jaWFsLnYxYWxwaGExLkdldFN0b3JlU29jaWFsU3VtbW'
    'FyeVJlcXVlc3QaNC5oYXdrci5zb2NpYWwudjFhbHBoYTEuR2V0U3RvcmVTb2NpYWxTdW1tYXJ5'
    'UmVzcG9uc2UiA5ACARJ1Cg9MaXN0TXlTYW5jdGlvbnMSLS5oYXdrci5zb2NpYWwudjFhbHBoYT'
    'EuTGlzdE15U2FuY3Rpb25zUmVxdWVzdBouLmhhd2tyLnNvY2lhbC52MWFscGhhMS5MaXN0TXlT'
    'YW5jdGlvbnNSZXNwb25zZSIDkAIBEoEBChNMaXN0TXlIaWRkZW5Db250ZW50EjEuaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLkxpc3RNeUhpZGRlbkNvbnRlbnRSZXF1ZXN0GjIuaGF3a3Iuc29jaWFs'
    'LnYxYWxwaGExLkxpc3RNeUhpZGRlbkNvbnRlbnRSZXNwb25zZSIDkAIB');
