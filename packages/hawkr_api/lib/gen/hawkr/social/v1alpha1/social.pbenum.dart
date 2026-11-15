// This is a generated file - do not edit.
//
// Generated from hawkr/social/v1alpha1/social.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ActorKind 是互動者的種類：買家或店家。
class ActorKind extends $pb.ProtobufEnum {
  static const ActorKind ACTOR_KIND_UNSPECIFIED =
      ActorKind._(0, _omitEnumNames ? '' : 'ACTOR_KIND_UNSPECIFIED');
  static const ActorKind ACTOR_KIND_BUYER =
      ActorKind._(1, _omitEnumNames ? '' : 'ACTOR_KIND_BUYER');
  static const ActorKind ACTOR_KIND_STORE =
      ActorKind._(2, _omitEnumNames ? '' : 'ACTOR_KIND_STORE');

  static const $core.List<ActorKind> values = <ActorKind>[
    ACTOR_KIND_UNSPECIFIED,
    ACTOR_KIND_BUYER,
    ACTOR_KIND_STORE,
  ];

  static final $core.List<ActorKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static ActorKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ActorKind._(super.value, super.name);
}

/// CommentState 是留言列表項目的狀態。
class CommentState extends $pb.ProtobufEnum {
  static const CommentState COMMENT_STATE_UNSPECIFIED =
      CommentState._(0, _omitEnumNames ? '' : 'COMMENT_STATE_UNSPECIFIED');

  /// 有內文與作者。
  static const CommentState COMMENT_STATE_VISIBLE =
      CommentState._(1, _omitEnumNames ? '' : 'COMMENT_STATE_VISIBLE');

  /// 作者或店家刪除；只有底下有可見回覆時才以佔位項目出現。
  static const CommentState COMMENT_STATE_DELETED =
      CommentState._(2, _omitEnumNames ? '' : 'COMMENT_STATE_DELETED');

  /// 被平台隱藏或內容下架；只有底下有可見回覆時才以佔位項目出現。
  static const CommentState COMMENT_STATE_HIDDEN =
      CommentState._(3, _omitEnumNames ? '' : 'COMMENT_STATE_HIDDEN');

  static const $core.List<CommentState> values = <CommentState>[
    COMMENT_STATE_UNSPECIFIED,
    COMMENT_STATE_VISIBLE,
    COMMENT_STATE_DELETED,
    COMMENT_STATE_HIDDEN,
  ];

  static final $core.List<CommentState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static CommentState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CommentState._(super.value, super.name);
}

/// PostStatus 是貼文的狀態：DRAFT → PUBLISHED → UNPUBLISHED → PUBLISHED。只有店主讀自己的貼文時看得到。
class PostStatus extends $pb.ProtobufEnum {
  static const PostStatus POST_STATUS_UNSPECIFIED =
      PostStatus._(0, _omitEnumNames ? '' : 'POST_STATUS_UNSPECIFIED');

  /// 草稿，還沒發布過。
  static const PostStatus POST_STATUS_DRAFT =
      PostStatus._(1, _omitEnumNames ? '' : 'POST_STATUS_DRAFT');
  static const PostStatus POST_STATUS_PUBLISHED =
      PostStatus._(2, _omitEnumNames ? '' : 'POST_STATUS_PUBLISHED');

  /// 發布過，被店主下架。
  static const PostStatus POST_STATUS_UNPUBLISHED =
      PostStatus._(3, _omitEnumNames ? '' : 'POST_STATUS_UNPUBLISHED');

  static const $core.List<PostStatus> values = <PostStatus>[
    POST_STATUS_UNSPECIFIED,
    POST_STATUS_DRAFT,
    POST_STATUS_PUBLISHED,
    POST_STATUS_UNPUBLISHED,
  ];

  static final $core.List<PostStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static PostStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PostStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
