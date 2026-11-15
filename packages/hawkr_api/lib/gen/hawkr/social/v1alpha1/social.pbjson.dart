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
  ],
  '8': [
    {'1': '_product_id'},
  ],
};

/// Descriptor for `Post`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List postDescriptor = $convert.base64Decode(
    'CgRQb3N0EhMKAmlkGAEgASgJQgPgQQhSAmlkEjsKBXN0b3JlGAIgASgLMiAuaGF3a3IuY29tbW'
    '9uLnYxYWxwaGExLlN0b3JlQ2FyZEID4EEDUgVzdG9yZRISCgRib2R5GAMgASgJUgRib2R5EjIK'
    'BWltYWdlGAQgASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaGExLkltYWdlUgVpbWFnZRIiCgpwcm'
    '9kdWN0X2lkGAUgASgJSABSCXByb2R1Y3RJZIgBARI/CghoYXNodGFncxgGIAMoCzIeLmhhd2ty'
    'LnNvY2lhbC52MWFscGhhMS5IYXNodGFnQgPgQQNSCGhhc2h0YWdzEkIKDHB1Ymxpc2hfdGltZR'
    'gHIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1ILcHVibGlzaFRpbWUSQAoL'
    'dXBkYXRlX3RpbWUYCCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wQgPgQQNSCnVwZG'
    'F0ZVRpbWUSIgoKbGlrZV9jb3VudBgJIAEoA0ID4EEDUglsaWtlQ291bnQSKAoNY29tbWVudF9j'
    'b3VudBgKIAEoA0ID4EEDUgxjb21tZW50Q291bnQSIgoKdmlld19jb3VudBgLIAEoA0ID4EEDUg'
    'l2aWV3Q291bnQSJAoLc2hhcmVfY291bnQYDCABKANCA+BBA1IKc2hhcmVDb3VudBIVCgN1cmwY'
    'DSABKAlCA+BBA1IDdXJsEjEKEnZpZXdlcl9jYW5fY29tbWVudBgOIAEoCEID4EEDUhB2aWV3ZX'
    'JDYW5Db21tZW50Qg0KC19wcm9kdWN0X2lk');

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
    {'1': 'body', '3': 7, '4': 1, '5': 9, '8': {}, '10': 'body'},
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
    'HC5oYXdrci5zb2NpYWwudjFhbHBoYTEuQWN0b3JCA+BBA1IGYXV0aG9yEhcKBGJvZHkYByABKA'
    'lCA+BBA1IEYm9keRJACgtjcmVhdGVfdGltZRgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1l'
    'c3RhbXBCA+BBA1IKY3JlYXRlVGltZRJACgt1cGRhdGVfdGltZRgJIAEoCzIaLmdvb2dsZS5wcm'
    '90b2J1Zi5UaW1lc3RhbXBCA+BBA1IKdXBkYXRlVGltZRIiCgpsaWtlX2NvdW50GAogASgDQgPg'
    'QQNSCWxpa2VDb3VudBIkCgtyZXBseV9jb3VudBgLIAEoA0ID4EEDUgpyZXBseUNvdW50EiYKDH'
    'ZpZXdlcl9saWtlZBgMIAEoCEID4EEDUgt2aWV3ZXJMaWtlZBIcCgdpc19taW5lGA0gASgIQgPg'
    'QQNSBmlzTWluZRIrCg92aWV3ZXJfY2FuX2VkaXQYDiABKAhCA+BBA1INdmlld2VyQ2FuRWRpdB'
    'IvChF2aWV3ZXJfY2FuX2RlbGV0ZRgPIAEoCEID4EEDUg92aWV3ZXJDYW5EZWxldGUSLQoQdmll'
    'd2VyX2Nhbl9yZXBseRgQIAEoCEID4EEDUg52aWV3ZXJDYW5SZXBseUIMCgpfcGFyZW50X2lkQg'
    '4KDF9yZXBseV90b19pZA==');

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

@$core.Deprecated('Use listTrendingTagsRequestDescriptor instead')
const ListTrendingTagsRequest$json = {
  '1': 'ListTrendingTagsRequest',
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

/// Descriptor for `ListTrendingTagsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTrendingTagsRequestDescriptor = $convert.base64Decode(
    'ChdMaXN0VHJlbmRpbmdUYWdzUmVxdWVzdBI5CgpzdGFydF90aW1lGAEgASgLMhouZ29vZ2xlLn'
    'Byb3RvYnVmLlRpbWVzdGFtcFIJc3RhcnRUaW1lEjUKCGVuZF90aW1lGAIgASgLMhouZ29vZ2xl'
    'LnByb3RvYnVmLlRpbWVzdGFtcFIHZW5kVGltZRIfCgttYXhfcmVzdWx0cxgDIAEoBVIKbWF4Um'
    'VzdWx0cw==');

@$core.Deprecated('Use listTrendingTagsResponseDescriptor instead')
const ListTrendingTagsResponse$json = {
  '1': 'ListTrendingTagsResponse',
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

/// Descriptor for `ListTrendingTagsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTrendingTagsResponseDescriptor =
    $convert.base64Decode(
        'ChhMaXN0VHJlbmRpbmdUYWdzUmVzcG9uc2USMwoEdGFncxgBIAMoCzIfLmhhd2tyLnNvY2lhbC'
        '52MWFscGhhMS5UYWdDb3VudFIEdGFncw==');

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
    {'1': 'body', '3': 7, '4': 1, '5': 9, '8': {}, '10': 'body'},
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
    'IAEoCzIcLmhhd2tyLnNvY2lhbC52MWFscGhhMS5BY3RvckID4EEDUgZhdXRob3ISFwoEYm9keR'
    'gHIAEoCUID4EEDUgRib2R5EkAKC2NyZWF0ZV90aW1lGAggASgLMhouZ29vZ2xlLnByb3RvYnVm'
    'LlRpbWVzdGFtcEID4EEDUgpjcmVhdGVUaW1lEkAKC3VwZGF0ZV90aW1lGAkgASgLMhouZ29vZ2'
    'xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUgp1cGRhdGVUaW1lEiQKC3JlcGx5X2NvdW50GAog'
    'ASgDQgPgQQNSCnJlcGx5Q291bnQSHAoHaXNfbWluZRgLIAEoCEID4EEDUgZpc01pbmUSKwoPdm'
    'lld2VyX2Nhbl9lZGl0GAwgASgIQgPgQQNSDXZpZXdlckNhbkVkaXQSLwoRdmlld2VyX2Nhbl9k'
    'ZWxldGUYDSABKAhCA+BBA1IPdmlld2VyQ2FuRGVsZXRlEi0KEHZpZXdlcl9jYW5fcmVwbHkYDi'
    'ABKAhCA+BBA1IOdmlld2VyQ2FuUmVwbHlCDAoKX3BhcmVudF9pZEIOCgxfcmVwbHlfdG9faWQ=');

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
      '1': 'ListTrendingTags',
      '2': '.hawkr.social.v1alpha1.ListTrendingTagsRequest',
      '3': '.hawkr.social.v1alpha1.ListTrendingTagsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListForumEntries',
      '2': '.hawkr.social.v1alpha1.ListForumEntriesRequest',
      '3': '.hawkr.social.v1alpha1.ListForumEntriesResponse',
      '4': {'34': 1},
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
  '.hawkr.social.v1alpha1.ListTrendingTagsRequest':
      ListTrendingTagsRequest$json,
  '.hawkr.social.v1alpha1.ListTrendingTagsResponse':
      ListTrendingTagsResponse$json,
  '.hawkr.social.v1alpha1.ListForumEntriesRequest':
      ListForumEntriesRequest$json,
  '.hawkr.social.v1alpha1.ListForumEntriesResponse':
      ListForumEntriesResponse$json,
  '.hawkr.social.v1alpha1.ForumEntry': ForumEntry$json,
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
    'c1Jlc3BvbnNlIgOQAgESeAoQTGlzdFRyZW5kaW5nVGFncxIuLmhhd2tyLnNvY2lhbC52MWFscG'
    'hhMS5MaXN0VHJlbmRpbmdUYWdzUmVxdWVzdBovLmhhd2tyLnNvY2lhbC52MWFscGhhMS5MaXN0'
    'VHJlbmRpbmdUYWdzUmVzcG9uc2UiA5ACARJ4ChBMaXN0Rm9ydW1FbnRyaWVzEi4uaGF3a3Iuc2'
    '9jaWFsLnYxYWxwaGExLkxpc3RGb3J1bUVudHJpZXNSZXF1ZXN0Gi8uaGF3a3Iuc29jaWFsLnYx'
    'YWxwaGExLkxpc3RGb3J1bUVudHJpZXNSZXNwb25zZSIDkAIB');
