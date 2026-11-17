// This is a generated file - do not edit.
//
// Generated from hawkr/social/v1alpha1/social.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/field_mask.pb.dart'
    as $3;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

import '../../common/v1alpha1/image.pb.dart' as $2;
import '../../common/v1alpha1/policy_category.pbenum.dart' as $4;
import '../../common/v1alpha1/store_card.pb.dart' as $1;
import 'social.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'social.pbenum.dart';

/// PostSchedule 是一篇貼文待發布的排程。
class PostSchedule extends $pb.GeneratedMessage {
  factory PostSchedule({
    $core.String? id,
    $core.String? postId,
    $0.Timestamp? publishTime,
    ScheduleKind? scheduledBy,
    $0.Timestamp? createTime,
  }) {
    final result = PostSchedule._();
    if (id != null) result.id = id;
    if (postId != null) result.postId = postId;
    if (publishTime != null) result.publishTime = publishTime;
    if (scheduledBy != null) result.scheduledBy = scheduledBy;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  PostSchedule._();

  factory PostSchedule.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PostSchedule()..mergeFromBuffer(data, registry);
  factory PostSchedule.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PostSchedule()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PostSchedule',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: PostSchedule.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOM<$0.Timestamp>(3, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aE<ScheduleKind>(4, _omitFieldNames ? '' : 'scheduledBy',
        enumValues: ScheduleKind.values)
    ..aOM<$0.Timestamp>(5, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PostSchedule clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PostSchedule copyWith(void Function(PostSchedule) updates) =>
      super.copyWith((message) => updates(message as PostSchedule))
          as PostSchedule;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PostSchedule() / PostSchedule.new instead')
  static PostSchedule create() => PostSchedule._();
  static $pb.GeneratedMessage $_createMessage() => PostSchedule._();
  @$core.override
  PostSchedule createEmptyInstance() => PostSchedule._();
  @$core.pragma('dart2js:noInline')
  static PostSchedule getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PostSchedule>(
          PostSchedule.$_createMessage);
  static PostSchedule? _defaultInstance;

  /// 排程的 ID。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 要發布的貼文。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 發布的時間點。
  @$pb.TagNumber(3)
  $0.Timestamp get publishTime => $_getN(2);
  @$pb.TagNumber(3)
  set publishTime($0.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPublishTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearPublishTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.Timestamp ensurePublishTime() => $_ensure(2);

  @$pb.TagNumber(4)
  ScheduleKind get scheduledBy => $_getN(3);
  @$pb.TagNumber(4)
  set scheduledBy(ScheduleKind value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasScheduledBy() => $_has(3);
  @$pb.TagNumber(4)
  void clearScheduledBy() => $_clearField(4);

  @$pb.TagNumber(5)
  $0.Timestamp get createTime => $_getN(4);
  @$pb.TagNumber(5)
  set createTime($0.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCreateTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearCreateTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Timestamp ensureCreateTime() => $_ensure(4);
}

/// Hashtag 是貼文的一個標籤。
class Hashtag extends $pb.GeneratedMessage {
  factory Hashtag({
    $core.String? name,
    $core.String? display,
  }) {
    final result = Hashtag._();
    if (name != null) result.name = name;
    if (display != null) result.display = display;
    return result;
  }

  Hashtag._();

  factory Hashtag.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Hashtag()..mergeFromBuffer(data, registry);
  factory Hashtag.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Hashtag()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Hashtag',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: Hashtag.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'display')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Hashtag clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Hashtag copyWith(void Function(Hashtag) updates) =>
      super.copyWith((message) => updates(message as Hashtag)) as Hashtag;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Hashtag() / Hashtag.new instead')
  static Hashtag create() => Hashtag._();
  static $pb.GeneratedMessage $_createMessage() => Hashtag._();
  @$core.override
  Hashtag createEmptyInstance() => Hashtag._();
  @$core.pragma('dart2js:noInline')
  static Hashtag getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Hashtag>(Hashtag.$_createMessage);
  static Hashtag? _defaultInstance;

  /// 正規化後的名稱，用來比對與連結。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  /// 使用者輸入的樣子，用來顯示。
  @$pb.TagNumber(2)
  $core.String get display => $_getSZ(1);
  @$pb.TagNumber(2)
  set display($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplay() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplay() => $_clearField(2);
}

/// Post 是一篇公開的貼文。
class Post extends $pb.GeneratedMessage {
  factory Post({
    $core.String? id,
    $1.StoreCard? store,
    $core.String? body,
    $2.Image? image,
    $core.String? productId,
    $core.Iterable<Hashtag>? hashtags,
    $0.Timestamp? publishTime,
    $0.Timestamp? updateTime,
    $fixnum.Int64? likeCount,
    $fixnum.Int64? commentCount,
    $fixnum.Int64? viewCount,
    $fixnum.Int64? shareCount,
    $core.String? url,
    $core.bool? viewerCanComment,
    PostStatus? status,
    $core.String? etag,
    $0.Timestamp? createTime,
    $core.String? imageStagedToken,
    $core.Iterable<$core.String>? hashtagInputs,
    $core.bool? viewerLiked,
    $core.String? takedownMeasureId,
    Actor? viewerActor,
  }) {
    final result = Post._();
    if (id != null) result.id = id;
    if (store != null) result.store = store;
    if (body != null) result.body = body;
    if (image != null) result.image = image;
    if (productId != null) result.productId = productId;
    if (hashtags != null) result.hashtags.addAll(hashtags);
    if (publishTime != null) result.publishTime = publishTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (likeCount != null) result.likeCount = likeCount;
    if (commentCount != null) result.commentCount = commentCount;
    if (viewCount != null) result.viewCount = viewCount;
    if (shareCount != null) result.shareCount = shareCount;
    if (url != null) result.url = url;
    if (viewerCanComment != null) result.viewerCanComment = viewerCanComment;
    if (status != null) result.status = status;
    if (etag != null) result.etag = etag;
    if (createTime != null) result.createTime = createTime;
    if (imageStagedToken != null) result.imageStagedToken = imageStagedToken;
    if (hashtagInputs != null) result.hashtagInputs.addAll(hashtagInputs);
    if (viewerLiked != null) result.viewerLiked = viewerLiked;
    if (takedownMeasureId != null) result.takedownMeasureId = takedownMeasureId;
    if (viewerActor != null) result.viewerActor = viewerActor;
    return result;
  }

  Post._();

  factory Post.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Post()..mergeFromBuffer(data, registry);
  factory Post.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Post()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Post',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: Post.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<$1.StoreCard>(2, _omitFieldNames ? '' : 'store',
        subBuilder: $1.StoreCard.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'body')
    ..aOM<$2.Image>(4, _omitFieldNames ? '' : 'image',
        subBuilder: $2.Image.$_createMessage)
    ..aOS(5, _omitFieldNames ? '' : 'productId')
    ..pPM<Hashtag>(6, _omitFieldNames ? '' : 'hashtags',
        subBuilder: Hashtag.$_createMessage)
    ..aOM<$0.Timestamp>(7, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(8, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aInt64(9, _omitFieldNames ? '' : 'likeCount')
    ..aInt64(10, _omitFieldNames ? '' : 'commentCount')
    ..aInt64(11, _omitFieldNames ? '' : 'viewCount')
    ..aInt64(12, _omitFieldNames ? '' : 'shareCount')
    ..aOS(13, _omitFieldNames ? '' : 'url')
    ..aOB(14, _omitFieldNames ? '' : 'viewerCanComment')
    ..aE<PostStatus>(15, _omitFieldNames ? '' : 'status',
        enumValues: PostStatus.values)
    ..aOS(16, _omitFieldNames ? '' : 'etag')
    ..aOM<$0.Timestamp>(17, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOS(18, _omitFieldNames ? '' : 'imageStagedToken')
    ..pPS(19, _omitFieldNames ? '' : 'hashtagInputs')
    ..aOB(20, _omitFieldNames ? '' : 'viewerLiked')
    ..aOS(21, _omitFieldNames ? '' : 'takedownMeasureId')
    ..aOM<Actor>(22, _omitFieldNames ? '' : 'viewerActor',
        subBuilder: Actor.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Post clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Post copyWith(void Function(Post) updates) =>
      super.copyWith((message) => updates(message as Post)) as Post;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Post() / Post.new instead')
  static Post create() => Post._();
  static $pb.GeneratedMessage $_createMessage() => Post._();
  @$core.override
  Post createEmptyInstance() => Post._();
  @$core.pragma('dart2js:noInline')
  static Post getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Post>(Post.$_createMessage);
  static Post? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 發文的店家。店家卡片讀不到時不設。
  @$pb.TagNumber(2)
  $1.StoreCard get store => $_getN(1);
  @$pb.TagNumber(2)
  set store($1.StoreCard value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStore() => $_has(1);
  @$pb.TagNumber(2)
  void clearStore() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.StoreCard ensureStore() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get body => $_getSZ(2);
  @$pb.TagNumber(3)
  set body($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBody() => $_has(2);
  @$pb.TagNumber(3)
  void clearBody() => $_clearField(3);

  /// 貼文的圖片，沒有圖時不設。
  @$pb.TagNumber(4)
  $2.Image get image => $_getN(3);
  @$pb.TagNumber(4)
  set image($2.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasImage() => $_has(3);
  @$pb.TagNumber(4)
  void clearImage() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.Image ensureImage() => $_ensure(3);

  /// 貼文帶的商品 ID，沒有時不設。
  @$pb.TagNumber(5)
  $core.String get productId => $_getSZ(4);
  @$pb.TagNumber(5)
  set productId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasProductId() => $_has(4);
  @$pb.TagNumber(5)
  void clearProductId() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<Hashtag> get hashtags => $_getList(5);

  /// 第一次發布的時間；下架後再發布沿用。
  @$pb.TagNumber(7)
  $0.Timestamp get publishTime => $_getN(6);
  @$pb.TagNumber(7)
  set publishTime($0.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasPublishTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearPublishTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Timestamp ensurePublishTime() => $_ensure(6);

  @$pb.TagNumber(8)
  $0.Timestamp get updateTime => $_getN(7);
  @$pb.TagNumber(8)
  set updateTime($0.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasUpdateTime() => $_has(7);
  @$pb.TagNumber(8)
  void clearUpdateTime() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Timestamp ensureUpdateTime() => $_ensure(7);

  @$pb.TagNumber(9)
  $fixnum.Int64 get likeCount => $_getI64(8);
  @$pb.TagNumber(9)
  set likeCount($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasLikeCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearLikeCount() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get commentCount => $_getI64(9);
  @$pb.TagNumber(10)
  set commentCount($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasCommentCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearCommentCount() => $_clearField(10);

  @$pb.TagNumber(11)
  $fixnum.Int64 get viewCount => $_getI64(10);
  @$pb.TagNumber(11)
  set viewCount($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasViewCount() => $_has(10);
  @$pb.TagNumber(11)
  void clearViewCount() => $_clearField(11);

  @$pb.TagNumber(12)
  $fixnum.Int64 get shareCount => $_getI64(11);
  @$pb.TagNumber(12)
  set shareCount($fixnum.Int64 value) => $_setInt64(11, value);
  @$pb.TagNumber(12)
  $core.bool hasShareCount() => $_has(11);
  @$pb.TagNumber(12)
  void clearShareCount() => $_clearField(12);

  /// 這篇貼文的單篇網址（絕對網址，由伺服器組，分享用）；client 不自己拼。
  @$pb.TagNumber(13)
  $core.String get url => $_getSZ(12);
  @$pb.TagNumber(13)
  set url($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasUrl() => $_has(12);
  @$pb.TagNumber(13)
  void clearUrl() => $_clearField(13);

  /// 觀看者此刻能不能在這篇貼文留言：訪客是 false；買家在這家店沒有被封鎖時是 true；店主讀自己店的貼文是 true。
  /// 這是顯示用的提示，寫入時伺服器仍會檢查。
  @$pb.TagNumber(14)
  $core.bool get viewerCanComment => $_getBF(13);
  @$pb.TagNumber(14)
  set viewerCanComment($core.bool value) => $_setBool(13, value);
  @$pb.TagNumber(14)
  $core.bool hasViewerCanComment() => $_has(13);
  @$pb.TagNumber(14)
  void clearViewerCanComment() => $_clearField(14);

  /// 貼文的狀態；只有店主讀自己的貼文（GetMyPost、ListMyPosts 與店主寫入的回應）才設，公開讀取不設。
  @$pb.TagNumber(15)
  PostStatus get status => $_getN(14);
  @$pb.TagNumber(15)
  set status(PostStatus value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasStatus() => $_has(14);
  @$pb.TagNumber(15)
  void clearStatus() => $_clearField(15);

  /// 修改、發布、下架、排程與刪除時帶回，client 視為不透明字串；只有店主讀自己的貼文才設。
  @$pb.TagNumber(16)
  $core.String get etag => $_getSZ(15);
  @$pb.TagNumber(16)
  set etag($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasEtag() => $_has(15);
  @$pb.TagNumber(16)
  void clearEtag() => $_clearField(16);

  /// 建立時間；只有店主讀自己的貼文才設（店主清單依它排序）。
  @$pb.TagNumber(17)
  $0.Timestamp get createTime => $_getN(16);
  @$pb.TagNumber(17)
  set createTime($0.Timestamp value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasCreateTime() => $_has(16);
  @$pb.TagNumber(17)
  void clearCreateTime() => $_clearField(17);
  @$pb.TagNumber(17)
  $0.Timestamp ensureCreateTime() => $_ensure(16);

  /// 換圖：把 media 上傳完成後拿到的 staged token 設在這裡並列進 update_mask；列進 update_mask 而 token 是空字串是清掉圖（已發布或有待發布排程的貼文不能清）。
  @$pb.TagNumber(18)
  $core.String get imageStagedToken => $_getSZ(17);
  @$pb.TagNumber(18)
  set imageStagedToken($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasImageStagedToken() => $_has(17);
  @$pb.TagNumber(18)
  void clearImageStagedToken() => $_clearField(18);

  /// 標籤的輸入（使用者輸入的樣子，可帶開頭的 #）；列進 update_mask 是整批取代（空清單是清掉所有標籤），沒有列進去保留原本的標籤。
  @$pb.TagNumber(19)
  $pb.PbList<$core.String> get hashtagInputs => $_getList(18);

  /// 觀看者是否讚過；訪客是 false。店主讀自己店的貼文（GetMyPost、ListMyPosts 與店主寫入的回應）時不是買家身分，一律 false。
  @$pb.TagNumber(20)
  $core.bool get viewerLiked => $_getBF(19);
  @$pb.TagNumber(20)
  set viewerLiked($core.bool value) => $_setBool(19, value);
  @$pb.TagNumber(20)
  $core.bool hasViewerLiked() => $_has(19);
  @$pb.TagNumber(20)
  void clearViewerLiked() => $_clearField(20);

  /// 下架這篇貼文的內容下架措施 ID（seller 的 `Measure.id`）；貼文沒有被內容下架時不設。只有店主讀自己的貼文才設（後台列表在 AdminPost 另給），公開讀取不設。
  /// 下架理由不在這裡：用這個 ID 呼叫賣家的 `GetMyMeasure`。同時被多個措施下架時是最早建立的那一個。
  @$pb.TagNumber(21)
  $core.String get takedownMeasureId => $_getSZ(20);
  @$pb.TagNumber(21)
  set takedownMeasureId($core.String value) => $_setString(20, value);
  @$pb.TagNumber(21)
  $core.bool hasTakedownMeasureId() => $_has(20);
  @$pb.TagNumber(21)
  void clearTakedownMeasureId() => $_clearField(21);

  /// 觀看者在這篇貼文留言時的身分，與 viewer_can_comment 用同一套觀看者判斷：觀看者此刻的店擁有這篇貼文時是店家（店名、店 logo），
  /// 否則有買家身分時是買家（顯示名稱、頭像）；訪客不設。被這家店封鎖的買家仍設（身分不變，只是 viewer_can_comment 是 false）。
  /// 名稱與頭像讀不到時（店家卡片讀不到、買家已刪除）只帶 kind 與 id。
  @$pb.TagNumber(22)
  Actor get viewerActor => $_getN(21);
  @$pb.TagNumber(22)
  set viewerActor(Actor value) => $_setField(22, value);
  @$pb.TagNumber(22)
  $core.bool hasViewerActor() => $_has(21);
  @$pb.TagNumber(22)
  void clearViewerActor() => $_clearField(22);
  @$pb.TagNumber(22)
  Actor ensureViewerActor() => $_ensure(21);
}

/// Actor 是留言的作者，名稱與頭像在讀取時由 buyer、seller 補。
class Actor extends $pb.GeneratedMessage {
  factory Actor({
    ActorKind? kind,
    $core.String? id,
    $core.String? name,
    $2.Image? avatar,
  }) {
    final result = Actor._();
    if (kind != null) result.kind = kind;
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (avatar != null) result.avatar = avatar;
    return result;
  }

  Actor._();

  factory Actor.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Actor()..mergeFromBuffer(data, registry);
  factory Actor.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Actor()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Actor',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: Actor.$_createMessage)
    ..aE<ActorKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: ActorKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'id')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOM<$2.Image>(4, _omitFieldNames ? '' : 'avatar',
        subBuilder: $2.Image.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Actor clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Actor copyWith(void Function(Actor) updates) =>
      super.copyWith((message) => updates(message as Actor)) as Actor;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Actor() / Actor.new instead')
  static Actor create() => Actor._();
  static $pb.GeneratedMessage $_createMessage() => Actor._();
  @$core.override
  Actor createEmptyInstance() => Actor._();
  @$core.pragma('dart2js:noInline')
  static Actor getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Actor>(Actor.$_createMessage);
  static Actor? _defaultInstance;

  @$pb.TagNumber(1)
  ActorKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(ActorKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  /// 買家或店家的 ID（依 kind）；已刪除的買家仍有 ID，但沒有名稱與頭像。
  @$pb.TagNumber(2)
  $core.String get id => $_getSZ(1);
  @$pb.TagNumber(2)
  set id($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasId() => $_has(1);
  @$pb.TagNumber(2)
  void clearId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $2.Image get avatar => $_getN(3);
  @$pb.TagNumber(4)
  set avatar($2.Image value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasAvatar() => $_has(3);
  @$pb.TagNumber(4)
  void clearAvatar() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.Image ensureAvatar() => $_ensure(3);
}

/// Comment 是留言列表的一個項目（頂層留言或回覆）。
class Comment extends $pb.GeneratedMessage {
  factory Comment({
    $core.String? id,
    $core.String? postId,
    $core.String? parentId,
    $core.String? replyToId,
    CommentState? state,
    Actor? author,
    $core.String? body,
    $0.Timestamp? createTime,
    $0.Timestamp? updateTime,
    $fixnum.Int64? likeCount,
    $fixnum.Int64? replyCount,
    $core.bool? viewerLiked,
    $core.bool? isMine,
    $core.bool? viewerCanEdit,
    $core.bool? viewerCanDelete,
    $core.bool? viewerCanReply,
    $core.String? etag,
  }) {
    final result = Comment._();
    if (id != null) result.id = id;
    if (postId != null) result.postId = postId;
    if (parentId != null) result.parentId = parentId;
    if (replyToId != null) result.replyToId = replyToId;
    if (state != null) result.state = state;
    if (author != null) result.author = author;
    if (body != null) result.body = body;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (likeCount != null) result.likeCount = likeCount;
    if (replyCount != null) result.replyCount = replyCount;
    if (viewerLiked != null) result.viewerLiked = viewerLiked;
    if (isMine != null) result.isMine = isMine;
    if (viewerCanEdit != null) result.viewerCanEdit = viewerCanEdit;
    if (viewerCanDelete != null) result.viewerCanDelete = viewerCanDelete;
    if (viewerCanReply != null) result.viewerCanReply = viewerCanReply;
    if (etag != null) result.etag = etag;
    return result;
  }

  Comment._();

  factory Comment.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Comment()..mergeFromBuffer(data, registry);
  factory Comment.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Comment()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Comment',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: Comment.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'parentId')
    ..aOS(4, _omitFieldNames ? '' : 'replyToId')
    ..aE<CommentState>(5, _omitFieldNames ? '' : 'state',
        enumValues: CommentState.values)
    ..aOM<Actor>(6, _omitFieldNames ? '' : 'author',
        subBuilder: Actor.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'body')
    ..aOM<$0.Timestamp>(8, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(9, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aInt64(10, _omitFieldNames ? '' : 'likeCount')
    ..aInt64(11, _omitFieldNames ? '' : 'replyCount')
    ..aOB(12, _omitFieldNames ? '' : 'viewerLiked')
    ..aOB(13, _omitFieldNames ? '' : 'isMine')
    ..aOB(14, _omitFieldNames ? '' : 'viewerCanEdit')
    ..aOB(15, _omitFieldNames ? '' : 'viewerCanDelete')
    ..aOB(16, _omitFieldNames ? '' : 'viewerCanReply')
    ..aOS(17, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Comment clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Comment copyWith(void Function(Comment) updates) =>
      super.copyWith((message) => updates(message as Comment)) as Comment;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Comment() / Comment.new instead')
  static Comment create() => Comment._();
  static $pb.GeneratedMessage $_createMessage() => Comment._();
  @$core.override
  Comment createEmptyInstance() => Comment._();
  @$core.pragma('dart2js:noInline')
  static Comment getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Comment>(Comment.$_createMessage);
  static Comment? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 回覆所屬的頂層留言；頂層留言不設。
  @$pb.TagNumber(3)
  $core.String get parentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set parentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasParentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearParentId() => $_clearField(3);

  /// 回覆針對的那一則留言；沒有時不設。
  @$pb.TagNumber(4)
  $core.String get replyToId => $_getSZ(3);
  @$pb.TagNumber(4)
  set replyToId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReplyToId() => $_has(3);
  @$pb.TagNumber(4)
  void clearReplyToId() => $_clearField(4);

  @$pb.TagNumber(5)
  CommentState get state => $_getN(4);
  @$pb.TagNumber(5)
  set state(CommentState value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasState() => $_has(4);
  @$pb.TagNumber(5)
  void clearState() => $_clearField(5);

  /// 只有 VISIBLE 的項目有作者與內文。
  @$pb.TagNumber(6)
  Actor get author => $_getN(5);
  @$pb.TagNumber(6)
  set author(Actor value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasAuthor() => $_has(5);
  @$pb.TagNumber(6)
  void clearAuthor() => $_clearField(6);
  @$pb.TagNumber(6)
  Actor ensureAuthor() => $_ensure(5);

  /// 留言唯一可以修改的欄位（UpdateComment 的 update_mask 列 body）。
  @$pb.TagNumber(7)
  $core.String get body => $_getSZ(6);
  @$pb.TagNumber(7)
  set body($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasBody() => $_has(6);
  @$pb.TagNumber(7)
  void clearBody() => $_clearField(7);

  @$pb.TagNumber(8)
  $0.Timestamp get createTime => $_getN(7);
  @$pb.TagNumber(8)
  set createTime($0.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCreateTime() => $_has(7);
  @$pb.TagNumber(8)
  void clearCreateTime() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Timestamp ensureCreateTime() => $_ensure(7);

  @$pb.TagNumber(9)
  $0.Timestamp get updateTime => $_getN(8);
  @$pb.TagNumber(9)
  set updateTime($0.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasUpdateTime() => $_has(8);
  @$pb.TagNumber(9)
  void clearUpdateTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $0.Timestamp ensureUpdateTime() => $_ensure(8);

  @$pb.TagNumber(10)
  $fixnum.Int64 get likeCount => $_getI64(9);
  @$pb.TagNumber(10)
  set likeCount($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasLikeCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearLikeCount() => $_clearField(10);

  @$pb.TagNumber(11)
  $fixnum.Int64 get replyCount => $_getI64(10);
  @$pb.TagNumber(11)
  set replyCount($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasReplyCount() => $_has(10);
  @$pb.TagNumber(11)
  void clearReplyCount() => $_clearField(11);

  /// 觀看者是否讚過；訪客是 false。
  @$pb.TagNumber(12)
  $core.bool get viewerLiked => $_getBF(11);
  @$pb.TagNumber(12)
  set viewerLiked($core.bool value) => $_setBool(11, value);
  @$pb.TagNumber(12)
  $core.bool hasViewerLiked() => $_has(11);
  @$pb.TagNumber(12)
  void clearViewerLiked() => $_clearField(12);

  /// 這則留言是不是觀看者自己的某個身分（買家或此刻的店）寫的。
  @$pb.TagNumber(13)
  $core.bool get isMine => $_getBF(12);
  @$pb.TagNumber(13)
  set isMine($core.bool value) => $_setBool(12, value);
  @$pb.TagNumber(13)
  $core.bool hasIsMine() => $_has(12);
  @$pb.TagNumber(13)
  void clearIsMine() => $_clearField(13);

  /// 觀看者能不能編輯：只有 VISIBLE 而且是自己的。
  @$pb.TagNumber(14)
  $core.bool get viewerCanEdit => $_getBF(13);
  @$pb.TagNumber(14)
  set viewerCanEdit($core.bool value) => $_setBool(13, value);
  @$pb.TagNumber(14)
  $core.bool hasViewerCanEdit() => $_has(13);
  @$pb.TagNumber(14)
  void clearViewerCanEdit() => $_clearField(14);

  /// 觀看者能不能刪除：VISIBLE 的自己的留言，或這篇貼文是自己的店的。
  @$pb.TagNumber(15)
  $core.bool get viewerCanDelete => $_getBF(14);
  @$pb.TagNumber(15)
  set viewerCanDelete($core.bool value) => $_setBool(14, value);
  @$pb.TagNumber(15)
  $core.bool hasViewerCanDelete() => $_has(14);
  @$pb.TagNumber(15)
  void clearViewerCanDelete() => $_clearField(15);

  /// 觀看者能不能回覆：VISIBLE，而且這篇貼文是觀看者此刻的店的（貼文留言的回覆只限貼文所屬的店，買家一律 false）。
  @$pb.TagNumber(16)
  $core.bool get viewerCanReply => $_getBF(15);
  @$pb.TagNumber(16)
  set viewerCanReply($core.bool value) => $_setBool(15, value);
  @$pb.TagNumber(16)
  $core.bool hasViewerCanReply() => $_has(15);
  @$pb.TagNumber(16)
  void clearViewerCanReply() => $_clearField(16);

  /// 只有 VISIBLE 的項目有；修改與刪除時帶回，client 視為不透明字串。
  @$pb.TagNumber(17)
  $core.String get etag => $_getSZ(16);
  @$pb.TagNumber(17)
  set etag($core.String value) => $_setString(16, value);
  @$pb.TagNumber(17)
  $core.bool hasEtag() => $_has(16);
  @$pb.TagNumber(17)
  void clearEtag() => $_clearField(17);
}

class GetPostRequest extends $pb.GeneratedMessage {
  factory GetPostRequest({
    $core.String? postId,
  }) {
    final result = GetPostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  GetPostRequest._();

  factory GetPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostRequest()..mergeFromBuffer(data, registry);
  factory GetPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetPostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostRequest copyWith(void Function(GetPostRequest) updates) =>
      super.copyWith((message) => updates(message as GetPostRequest))
          as GetPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPostRequest() / GetPostRequest.new instead')
  static GetPostRequest create() => GetPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetPostRequest._();
  @$core.override
  GetPostRequest createEmptyInstance() => GetPostRequest._();
  @$core.pragma('dart2js:noInline')
  static GetPostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetPostRequest>(
          GetPostRequest.$_createMessage);
  static GetPostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class GetPostResponse extends $pb.GeneratedMessage {
  factory GetPostResponse({
    Post? post,
  }) {
    final result = GetPostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  GetPostResponse._();

  factory GetPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostResponse()..mergeFromBuffer(data, registry);
  factory GetPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetPostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPostResponse copyWith(void Function(GetPostResponse) updates) =>
      super.copyWith((message) => updates(message as GetPostResponse))
          as GetPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPostResponse() / GetPostResponse.new instead')
  static GetPostResponse create() => GetPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetPostResponse._();
  @$core.override
  GetPostResponse createEmptyInstance() => GetPostResponse._();
  @$core.pragma('dart2js:noInline')
  static GetPostResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetPostResponse>(
          GetPostResponse.$_createMessage);
  static GetPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class ListPostsRequest extends $pb.GeneratedMessage {
  factory ListPostsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $core.String? storeId,
    $core.String? hashtag,
  }) {
    final result = ListPostsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (storeId != null) result.storeId = storeId;
    if (hashtag != null) result.hashtag = hashtag;
    return result;
  }

  ListPostsRequest._();

  factory ListPostsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsRequest()..mergeFromBuffer(data, registry);
  factory ListPostsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListPostsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListPostsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aOS(3, _omitFieldNames ? '' : 'storeId')
    ..aOS(4, _omitFieldNames ? '' : 'hashtag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsRequest copyWith(void Function(ListPostsRequest) updates) =>
      super.copyWith((message) => updates(message as ListPostsRequest))
          as ListPostsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListPostsRequest() / ListPostsRequest.new instead')
  static ListPostsRequest create() => ListPostsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListPostsRequest._();
  @$core.override
  ListPostsRequest createEmptyInstance() => ListPostsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListPostsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListPostsRequest>(
          ListPostsRequest.$_createMessage);
  static ListPostsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);

  /// 只列這家店的貼文；不設是全站動態牆。page_token 只能用在同一個 store_id。
  @$pb.TagNumber(3)
  $core.String get storeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreId() => $_clearField(3);

  /// 只列帶這個標籤的貼文；名稱不分大小寫、可以帶開頭的 #，不設是不篩選。不能與 store_id 同時設。page_token 只能用在同一個 hashtag。
  @$pb.TagNumber(4)
  $core.String get hashtag => $_getSZ(3);
  @$pb.TagNumber(4)
  set hashtag($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHashtag() => $_has(3);
  @$pb.TagNumber(4)
  void clearHashtag() => $_clearField(4);
}

class ListPostsResponse extends $pb.GeneratedMessage {
  factory ListPostsResponse({
    $core.Iterable<Post>? posts,
    $core.String? nextPageToken,
  }) {
    final result = ListPostsResponse._();
    if (posts != null) result.posts.addAll(posts);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListPostsResponse._();

  factory ListPostsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsResponse()..mergeFromBuffer(data, registry);
  factory ListPostsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPostsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListPostsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListPostsResponse.$_createMessage)
    ..pPM<Post>(1, _omitFieldNames ? '' : 'posts',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPostsResponse copyWith(void Function(ListPostsResponse) updates) =>
      super.copyWith((message) => updates(message as ListPostsResponse))
          as ListPostsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListPostsResponse() / ListPostsResponse.new instead')
  static ListPostsResponse create() => ListPostsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListPostsResponse._();
  @$core.override
  ListPostsResponse createEmptyInstance() => ListPostsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListPostsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListPostsResponse>(
          ListPostsResponse.$_createMessage);
  static ListPostsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Post> get posts => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListCommentsRequest extends $pb.GeneratedMessage {
  factory ListCommentsRequest({
    $core.String? postId,
    $core.int? pageSize,
    $core.String? pageToken,
    $core.String? parentId,
  }) {
    final result = ListCommentsRequest._();
    if (postId != null) result.postId = postId;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (parentId != null) result.parentId = parentId;
    return result;
  }

  ListCommentsRequest._();

  factory ListCommentsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCommentsRequest()..mergeFromBuffer(data, registry);
  factory ListCommentsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCommentsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCommentsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListCommentsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..aOS(4, _omitFieldNames ? '' : 'parentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCommentsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCommentsRequest copyWith(void Function(ListCommentsRequest) updates) =>
      super.copyWith((message) => updates(message as ListCommentsRequest))
          as ListCommentsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListCommentsRequest() / ListCommentsRequest.new instead')
  static ListCommentsRequest create() => ListCommentsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListCommentsRequest._();
  @$core.override
  ListCommentsRequest createEmptyInstance() => ListCommentsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCommentsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCommentsRequest>(
          ListCommentsRequest.$_createMessage);
  static ListCommentsRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 預設 20，上限 50。
  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get pageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set pageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageToken() => $_clearField(3);

  /// 設了是該頂層留言的回覆；不設是貼文的頂層留言。page_token 只能用在同一組 post_id 與 parent_id。
  @$pb.TagNumber(4)
  $core.String get parentId => $_getSZ(3);
  @$pb.TagNumber(4)
  set parentId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasParentId() => $_has(3);
  @$pb.TagNumber(4)
  void clearParentId() => $_clearField(4);
}

class ListCommentsResponse extends $pb.GeneratedMessage {
  factory ListCommentsResponse({
    $core.Iterable<Comment>? comments,
    $core.String? nextPageToken,
  }) {
    final result = ListCommentsResponse._();
    if (comments != null) result.comments.addAll(comments);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListCommentsResponse._();

  factory ListCommentsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCommentsResponse()..mergeFromBuffer(data, registry);
  factory ListCommentsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCommentsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCommentsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListCommentsResponse.$_createMessage)
    ..pPM<Comment>(1, _omitFieldNames ? '' : 'comments',
        subBuilder: Comment.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCommentsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCommentsResponse copyWith(void Function(ListCommentsResponse) updates) =>
      super.copyWith((message) => updates(message as ListCommentsResponse))
          as ListCommentsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCommentsResponse() / ListCommentsResponse.new instead')
  static ListCommentsResponse create() => ListCommentsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListCommentsResponse._();
  @$core.override
  ListCommentsResponse createEmptyInstance() => ListCommentsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCommentsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCommentsResponse>(
          ListCommentsResponse.$_createMessage);
  static ListCommentsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Comment> get comments => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class SuggestTrendingPostsRequest extends $pb.GeneratedMessage {
  factory SuggestTrendingPostsRequest({
    $core.int? maxResults,
  }) {
    final result = SuggestTrendingPostsRequest._();
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestTrendingPostsRequest._();

  factory SuggestTrendingPostsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingPostsRequest()..mergeFromBuffer(data, registry);
  factory SuggestTrendingPostsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingPostsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTrendingPostsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SuggestTrendingPostsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingPostsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingPostsRequest copyWith(
          void Function(SuggestTrendingPostsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestTrendingPostsRequest))
          as SuggestTrendingPostsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestTrendingPostsRequest() / SuggestTrendingPostsRequest.new instead')
  static SuggestTrendingPostsRequest create() =>
      SuggestTrendingPostsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestTrendingPostsRequest._();
  @$core.override
  SuggestTrendingPostsRequest createEmptyInstance() =>
      SuggestTrendingPostsRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestTrendingPostsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTrendingPostsRequest>(
          SuggestTrendingPostsRequest.$_createMessage);
  static SuggestTrendingPostsRequest? _defaultInstance;

  /// 最多回傳幾篇，1 到 30；0 表示預設值 10，超過 30 以 30 計，負數回 invalid_argument。
  @$pb.TagNumber(1)
  $core.int get maxResults => $_getIZ(0);
  @$pb.TagNumber(1)
  set maxResults($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMaxResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearMaxResults() => $_clearField(1);
}

class SuggestTrendingPostsResponse extends $pb.GeneratedMessage {
  factory SuggestTrendingPostsResponse({
    $core.Iterable<Post>? posts,
  }) {
    final result = SuggestTrendingPostsResponse._();
    if (posts != null) result.posts.addAll(posts);
    return result;
  }

  SuggestTrendingPostsResponse._();

  factory SuggestTrendingPostsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingPostsResponse()..mergeFromBuffer(data, registry);
  factory SuggestTrendingPostsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingPostsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTrendingPostsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SuggestTrendingPostsResponse.$_createMessage)
    ..pPM<Post>(1, _omitFieldNames ? '' : 'posts',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingPostsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingPostsResponse copyWith(
          void Function(SuggestTrendingPostsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestTrendingPostsResponse))
          as SuggestTrendingPostsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestTrendingPostsResponse() / SuggestTrendingPostsResponse.new instead')
  static SuggestTrendingPostsResponse create() =>
      SuggestTrendingPostsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestTrendingPostsResponse._();
  @$core.override
  SuggestTrendingPostsResponse createEmptyInstance() =>
      SuggestTrendingPostsResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestTrendingPostsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTrendingPostsResponse>(
          SuggestTrendingPostsResponse.$_createMessage);
  static SuggestTrendingPostsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Post> get posts => $_getList(0);
}

class SuggestTagsRequest extends $pb.GeneratedMessage {
  factory SuggestTagsRequest({
    $core.String? query,
    $core.bool? contains,
    $core.int? maxResults,
  }) {
    final result = SuggestTagsRequest._();
    if (query != null) result.query = query;
    if (contains != null) result.contains = contains;
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestTagsRequest._();

  factory SuggestTagsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTagsRequest()..mergeFromBuffer(data, registry);
  factory SuggestTagsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTagsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTagsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SuggestTagsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOB(2, _omitFieldNames ? '' : 'contains')
    ..aI(3, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTagsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTagsRequest copyWith(void Function(SuggestTagsRequest) updates) =>
      super.copyWith((message) => updates(message as SuggestTagsRequest))
          as SuggestTagsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SuggestTagsRequest() / SuggestTagsRequest.new instead')
  static SuggestTagsRequest create() => SuggestTagsRequest._();
  static $pb.GeneratedMessage $_createMessage() => SuggestTagsRequest._();
  @$core.override
  SuggestTagsRequest createEmptyInstance() => SuggestTagsRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestTagsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTagsRequest>(
          SuggestTagsRequest.$_createMessage);
  static SuggestTagsRequest? _defaultInstance;

  /// 搜尋字串，可以帶開頭的 #；正規化後是空的回空清單。
  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);

  /// true 是名稱包含 query，false 是以 query 開頭。
  @$pb.TagNumber(2)
  $core.bool get contains => $_getBF(1);
  @$pb.TagNumber(2)
  set contains($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContains() => $_has(1);
  @$pb.TagNumber(2)
  void clearContains() => $_clearField(2);

  /// 最多回傳幾筆，1 到 20；0 表示預設值 10，超過 20 以 20 計，負數回 invalid_argument。
  @$pb.TagNumber(3)
  $core.int get maxResults => $_getIZ(2);
  @$pb.TagNumber(3)
  set maxResults($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMaxResults() => $_has(2);
  @$pb.TagNumber(3)
  void clearMaxResults() => $_clearField(3);
}

class SuggestTagsResponse extends $pb.GeneratedMessage {
  factory SuggestTagsResponse({
    $core.Iterable<TagCount>? tags,
  }) {
    final result = SuggestTagsResponse._();
    if (tags != null) result.tags.addAll(tags);
    return result;
  }

  SuggestTagsResponse._();

  factory SuggestTagsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTagsResponse()..mergeFromBuffer(data, registry);
  factory SuggestTagsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTagsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTagsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SuggestTagsResponse.$_createMessage)
    ..pPM<TagCount>(1, _omitFieldNames ? '' : 'tags',
        subBuilder: TagCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTagsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTagsResponse copyWith(void Function(SuggestTagsResponse) updates) =>
      super.copyWith((message) => updates(message as SuggestTagsResponse))
          as SuggestTagsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SuggestTagsResponse() / SuggestTagsResponse.new instead')
  static SuggestTagsResponse create() => SuggestTagsResponse._();
  static $pb.GeneratedMessage $_createMessage() => SuggestTagsResponse._();
  @$core.override
  SuggestTagsResponse createEmptyInstance() => SuggestTagsResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestTagsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTagsResponse>(
          SuggestTagsResponse.$_createMessage);
  static SuggestTagsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<TagCount> get tags => $_getList(0);
}

class SuggestTrendingTagsRequest extends $pb.GeneratedMessage {
  factory SuggestTrendingTagsRequest({
    $0.Timestamp? startTime,
    $0.Timestamp? endTime,
    $core.int? maxResults,
  }) {
    final result = SuggestTrendingTagsRequest._();
    if (startTime != null) result.startTime = startTime;
    if (endTime != null) result.endTime = endTime;
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SuggestTrendingTagsRequest._();

  factory SuggestTrendingTagsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingTagsRequest()..mergeFromBuffer(data, registry);
  factory SuggestTrendingTagsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingTagsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTrendingTagsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SuggestTrendingTagsRequest.$_createMessage)
    ..aOM<$0.Timestamp>(1, _omitFieldNames ? '' : 'startTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'endTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aI(3, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingTagsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingTagsRequest copyWith(
          void Function(SuggestTrendingTagsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestTrendingTagsRequest))
          as SuggestTrendingTagsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestTrendingTagsRequest() / SuggestTrendingTagsRequest.new instead')
  static SuggestTrendingTagsRequest create() => SuggestTrendingTagsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestTrendingTagsRequest._();
  @$core.override
  SuggestTrendingTagsRequest createEmptyInstance() =>
      SuggestTrendingTagsRequest._();
  @$core.pragma('dart2js:noInline')
  static SuggestTrendingTagsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTrendingTagsRequest>(
          SuggestTrendingTagsRequest.$_createMessage);
  static SuggestTrendingTagsRequest? _defaultInstance;

  /// 時間窗的起點（含）；不設是 end_time 往前 30 天。
  @$pb.TagNumber(1)
  $0.Timestamp get startTime => $_getN(0);
  @$pb.TagNumber(1)
  set startTime($0.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStartTime() => $_has(0);
  @$pb.TagNumber(1)
  void clearStartTime() => $_clearField(1);
  @$pb.TagNumber(1)
  $0.Timestamp ensureStartTime() => $_ensure(0);

  /// 時間窗的終點（不含）；不設是現在。
  @$pb.TagNumber(2)
  $0.Timestamp get endTime => $_getN(1);
  @$pb.TagNumber(2)
  set endTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasEndTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearEndTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureEndTime() => $_ensure(1);

  /// 最多回傳幾名，1 到 50；0 表示預設值 20，超過 50 以 50 計，負數回 invalid_argument。
  @$pb.TagNumber(3)
  $core.int get maxResults => $_getIZ(2);
  @$pb.TagNumber(3)
  set maxResults($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMaxResults() => $_has(2);
  @$pb.TagNumber(3)
  void clearMaxResults() => $_clearField(3);
}

class SuggestTrendingTagsResponse extends $pb.GeneratedMessage {
  factory SuggestTrendingTagsResponse({
    $core.Iterable<TagCount>? tags,
  }) {
    final result = SuggestTrendingTagsResponse._();
    if (tags != null) result.tags.addAll(tags);
    return result;
  }

  SuggestTrendingTagsResponse._();

  factory SuggestTrendingTagsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingTagsResponse()..mergeFromBuffer(data, registry);
  factory SuggestTrendingTagsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SuggestTrendingTagsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SuggestTrendingTagsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SuggestTrendingTagsResponse.$_createMessage)
    ..pPM<TagCount>(1, _omitFieldNames ? '' : 'tags',
        subBuilder: TagCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingTagsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SuggestTrendingTagsResponse copyWith(
          void Function(SuggestTrendingTagsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SuggestTrendingTagsResponse))
          as SuggestTrendingTagsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SuggestTrendingTagsResponse() / SuggestTrendingTagsResponse.new instead')
  static SuggestTrendingTagsResponse create() =>
      SuggestTrendingTagsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SuggestTrendingTagsResponse._();
  @$core.override
  SuggestTrendingTagsResponse createEmptyInstance() =>
      SuggestTrendingTagsResponse._();
  @$core.pragma('dart2js:noInline')
  static SuggestTrendingTagsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SuggestTrendingTagsResponse>(
          SuggestTrendingTagsResponse.$_createMessage);
  static SuggestTrendingTagsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<TagCount> get tags => $_getList(0);
}

/// TagCount 是一個標籤與它在範圍內的公開貼文數。
class TagCount extends $pb.GeneratedMessage {
  factory TagCount({
    $core.String? name,
    $fixnum.Int64? postCount,
  }) {
    final result = TagCount._();
    if (name != null) result.name = name;
    if (postCount != null) result.postCount = postCount;
    return result;
  }

  TagCount._();

  factory TagCount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TagCount()..mergeFromBuffer(data, registry);
  factory TagCount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TagCount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TagCount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: TagCount.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aInt64(2, _omitFieldNames ? '' : 'postCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TagCount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TagCount copyWith(void Function(TagCount) updates) =>
      super.copyWith((message) => updates(message as TagCount)) as TagCount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TagCount() / TagCount.new instead')
  static TagCount create() => TagCount._();
  static $pb.GeneratedMessage $_createMessage() => TagCount._();
  @$core.override
  TagCount createEmptyInstance() => TagCount._();
  @$core.pragma('dart2js:noInline')
  static TagCount getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TagCount>(TagCount.$_createMessage);
  static TagCount? _defaultInstance;

  /// 正規化後的名稱，用來比對與連結。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get postCount => $_getI64(1);
  @$pb.TagNumber(2)
  set postCount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostCount() => $_clearField(2);
}

/// ForumEntry 是店家討論區的一個項目（頂層留言或回覆）。
class ForumEntry extends $pb.GeneratedMessage {
  factory ForumEntry({
    $core.String? id,
    $core.String? storeId,
    $core.String? parentId,
    $core.String? replyToId,
    CommentState? state,
    Actor? author,
    $core.String? body,
    $0.Timestamp? createTime,
    $0.Timestamp? updateTime,
    $fixnum.Int64? replyCount,
    $core.bool? isMine,
    $core.bool? viewerCanEdit,
    $core.bool? viewerCanDelete,
    $core.bool? viewerCanReply,
    $core.String? etag,
  }) {
    final result = ForumEntry._();
    if (id != null) result.id = id;
    if (storeId != null) result.storeId = storeId;
    if (parentId != null) result.parentId = parentId;
    if (replyToId != null) result.replyToId = replyToId;
    if (state != null) result.state = state;
    if (author != null) result.author = author;
    if (body != null) result.body = body;
    if (createTime != null) result.createTime = createTime;
    if (updateTime != null) result.updateTime = updateTime;
    if (replyCount != null) result.replyCount = replyCount;
    if (isMine != null) result.isMine = isMine;
    if (viewerCanEdit != null) result.viewerCanEdit = viewerCanEdit;
    if (viewerCanDelete != null) result.viewerCanDelete = viewerCanDelete;
    if (viewerCanReply != null) result.viewerCanReply = viewerCanReply;
    if (etag != null) result.etag = etag;
    return result;
  }

  ForumEntry._();

  factory ForumEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ForumEntry()..mergeFromBuffer(data, registry);
  factory ForumEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ForumEntry()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ForumEntry',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ForumEntry.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'storeId')
    ..aOS(3, _omitFieldNames ? '' : 'parentId')
    ..aOS(4, _omitFieldNames ? '' : 'replyToId')
    ..aE<CommentState>(5, _omitFieldNames ? '' : 'state',
        enumValues: CommentState.values)
    ..aOM<Actor>(6, _omitFieldNames ? '' : 'author',
        subBuilder: Actor.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'body')
    ..aOM<$0.Timestamp>(8, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOM<$0.Timestamp>(9, _omitFieldNames ? '' : 'updateTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aInt64(10, _omitFieldNames ? '' : 'replyCount')
    ..aOB(11, _omitFieldNames ? '' : 'isMine')
    ..aOB(12, _omitFieldNames ? '' : 'viewerCanEdit')
    ..aOB(13, _omitFieldNames ? '' : 'viewerCanDelete')
    ..aOB(14, _omitFieldNames ? '' : 'viewerCanReply')
    ..aOS(15, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForumEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForumEntry copyWith(void Function(ForumEntry) updates) =>
      super.copyWith((message) => updates(message as ForumEntry)) as ForumEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ForumEntry() / ForumEntry.new instead')
  static ForumEntry create() => ForumEntry._();
  static $pb.GeneratedMessage $_createMessage() => ForumEntry._();
  @$core.override
  ForumEntry createEmptyInstance() => ForumEntry._();
  @$core.pragma('dart2js:noInline')
  static ForumEntry getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ForumEntry>(ForumEntry.$_createMessage);
  static ForumEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get storeId => $_getSZ(1);
  @$pb.TagNumber(2)
  set storeId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoreId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoreId() => $_clearField(2);

  /// 回覆所屬的頂層留言；頂層留言不設。
  @$pb.TagNumber(3)
  $core.String get parentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set parentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasParentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearParentId() => $_clearField(3);

  /// 回覆針對的那一則留言；沒有時不設。
  @$pb.TagNumber(4)
  $core.String get replyToId => $_getSZ(3);
  @$pb.TagNumber(4)
  set replyToId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReplyToId() => $_has(3);
  @$pb.TagNumber(4)
  void clearReplyToId() => $_clearField(4);

  @$pb.TagNumber(5)
  CommentState get state => $_getN(4);
  @$pb.TagNumber(5)
  set state(CommentState value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasState() => $_has(4);
  @$pb.TagNumber(5)
  void clearState() => $_clearField(5);

  /// 只有 VISIBLE 的項目有作者與內文。
  @$pb.TagNumber(6)
  Actor get author => $_getN(5);
  @$pb.TagNumber(6)
  set author(Actor value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasAuthor() => $_has(5);
  @$pb.TagNumber(6)
  void clearAuthor() => $_clearField(6);
  @$pb.TagNumber(6)
  Actor ensureAuthor() => $_ensure(5);

  /// 項目唯一可以修改的欄位（UpdateForumEntry 的 update_mask 列 body）。
  @$pb.TagNumber(7)
  $core.String get body => $_getSZ(6);
  @$pb.TagNumber(7)
  set body($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasBody() => $_has(6);
  @$pb.TagNumber(7)
  void clearBody() => $_clearField(7);

  @$pb.TagNumber(8)
  $0.Timestamp get createTime => $_getN(7);
  @$pb.TagNumber(8)
  set createTime($0.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCreateTime() => $_has(7);
  @$pb.TagNumber(8)
  void clearCreateTime() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Timestamp ensureCreateTime() => $_ensure(7);

  @$pb.TagNumber(9)
  $0.Timestamp get updateTime => $_getN(8);
  @$pb.TagNumber(9)
  set updateTime($0.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasUpdateTime() => $_has(8);
  @$pb.TagNumber(9)
  void clearUpdateTime() => $_clearField(9);
  @$pb.TagNumber(9)
  $0.Timestamp ensureUpdateTime() => $_ensure(8);

  @$pb.TagNumber(10)
  $fixnum.Int64 get replyCount => $_getI64(9);
  @$pb.TagNumber(10)
  set replyCount($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasReplyCount() => $_has(9);
  @$pb.TagNumber(10)
  void clearReplyCount() => $_clearField(10);

  /// 這則是不是觀看者自己的某個身分（買家或此刻的店）寫的。
  @$pb.TagNumber(11)
  $core.bool get isMine => $_getBF(10);
  @$pb.TagNumber(11)
  set isMine($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasIsMine() => $_has(10);
  @$pb.TagNumber(11)
  void clearIsMine() => $_clearField(11);

  /// 觀看者能不能編輯：只有 VISIBLE 而且是自己的。
  @$pb.TagNumber(12)
  $core.bool get viewerCanEdit => $_getBF(11);
  @$pb.TagNumber(12)
  set viewerCanEdit($core.bool value) => $_setBool(11, value);
  @$pb.TagNumber(12)
  $core.bool hasViewerCanEdit() => $_has(11);
  @$pb.TagNumber(12)
  void clearViewerCanEdit() => $_clearField(12);

  /// 觀看者能不能刪除：VISIBLE 的自己的留言，或這是自己的店的討論區。
  @$pb.TagNumber(13)
  $core.bool get viewerCanDelete => $_getBF(12);
  @$pb.TagNumber(13)
  set viewerCanDelete($core.bool value) => $_setBool(12, value);
  @$pb.TagNumber(13)
  $core.bool hasViewerCanDelete() => $_has(12);
  @$pb.TagNumber(13)
  void clearViewerCanDelete() => $_clearField(13);

  /// 觀看者能不能回覆：VISIBLE，而且觀看者在這家店的討論區沒有被封鎖（訪客是 false；店主讀自己的討論區是 true）。顯示用的提示，寫入時伺服器仍會檢查。
  @$pb.TagNumber(14)
  $core.bool get viewerCanReply => $_getBF(13);
  @$pb.TagNumber(14)
  set viewerCanReply($core.bool value) => $_setBool(13, value);
  @$pb.TagNumber(14)
  $core.bool hasViewerCanReply() => $_has(13);
  @$pb.TagNumber(14)
  void clearViewerCanReply() => $_clearField(14);

  /// 只有 VISIBLE 的項目有；修改與刪除時帶回，client 視為不透明字串。
  @$pb.TagNumber(15)
  $core.String get etag => $_getSZ(14);
  @$pb.TagNumber(15)
  set etag($core.String value) => $_setString(14, value);
  @$pb.TagNumber(15)
  $core.bool hasEtag() => $_has(14);
  @$pb.TagNumber(15)
  void clearEtag() => $_clearField(15);
}

class ListForumEntriesRequest extends $pb.GeneratedMessage {
  factory ListForumEntriesRequest({
    $core.String? storeId,
    $core.int? pageSize,
    $core.String? pageToken,
    $core.String? parentId,
  }) {
    final result = ListForumEntriesRequest._();
    if (storeId != null) result.storeId = storeId;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (parentId != null) result.parentId = parentId;
    return result;
  }

  ListForumEntriesRequest._();

  factory ListForumEntriesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListForumEntriesRequest()..mergeFromBuffer(data, registry);
  factory ListForumEntriesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListForumEntriesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListForumEntriesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListForumEntriesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..aOS(4, _omitFieldNames ? '' : 'parentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListForumEntriesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListForumEntriesRequest copyWith(
          void Function(ListForumEntriesRequest) updates) =>
      super.copyWith((message) => updates(message as ListForumEntriesRequest))
          as ListForumEntriesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListForumEntriesRequest() / ListForumEntriesRequest.new instead')
  static ListForumEntriesRequest create() => ListForumEntriesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListForumEntriesRequest._();
  @$core.override
  ListForumEntriesRequest createEmptyInstance() => ListForumEntriesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListForumEntriesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListForumEntriesRequest>(
          ListForumEntriesRequest.$_createMessage);
  static ListForumEntriesRequest? _defaultInstance;

  /// 店家 ID。
  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 預設 20，上限 50。
  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get pageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set pageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageToken() => $_clearField(3);

  /// 設了是該頂層留言的回覆；不設是討論區的頂層留言。page_token 只能用在同一組 store_id 與 parent_id。
  @$pb.TagNumber(4)
  $core.String get parentId => $_getSZ(3);
  @$pb.TagNumber(4)
  set parentId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasParentId() => $_has(3);
  @$pb.TagNumber(4)
  void clearParentId() => $_clearField(4);
}

class ListForumEntriesResponse extends $pb.GeneratedMessage {
  factory ListForumEntriesResponse({
    $core.Iterable<ForumEntry>? entries,
    $core.String? nextPageToken,
    Actor? viewerActor,
    $core.bool? viewerCanComment,
  }) {
    final result = ListForumEntriesResponse._();
    if (entries != null) result.entries.addAll(entries);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    if (viewerActor != null) result.viewerActor = viewerActor;
    if (viewerCanComment != null) result.viewerCanComment = viewerCanComment;
    return result;
  }

  ListForumEntriesResponse._();

  factory ListForumEntriesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListForumEntriesResponse()..mergeFromBuffer(data, registry);
  factory ListForumEntriesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListForumEntriesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListForumEntriesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListForumEntriesResponse.$_createMessage)
    ..pPM<ForumEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: ForumEntry.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..aOM<Actor>(3, _omitFieldNames ? '' : 'viewerActor',
        subBuilder: Actor.$_createMessage)
    ..aOB(4, _omitFieldNames ? '' : 'viewerCanComment')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListForumEntriesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListForumEntriesResponse copyWith(
          void Function(ListForumEntriesResponse) updates) =>
      super.copyWith((message) => updates(message as ListForumEntriesResponse))
          as ListForumEntriesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListForumEntriesResponse() / ListForumEntriesResponse.new instead')
  static ListForumEntriesResponse create() => ListForumEntriesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListForumEntriesResponse._();
  @$core.override
  ListForumEntriesResponse createEmptyInstance() =>
      ListForumEntriesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListForumEntriesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListForumEntriesResponse>(
          ListForumEntriesResponse.$_createMessage);
  static ListForumEntriesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ForumEntry> get entries => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);

  /// 觀看者在這家店的討論區留言時的身分：觀看者此刻的店就是這家店時是店家（店名、店 logo），否則有買家身分時是買家（顯示名稱、頭像）；訪客不設。
  /// 被這家店封鎖的買家仍設（身分不變，只是 viewer_can_comment 是 false）。名稱與頭像讀不到時只帶 kind 與 id。與 ForumEntry.viewer_can_reply 用同一套觀看者判斷。
  @$pb.TagNumber(3)
  Actor get viewerActor => $_getN(2);
  @$pb.TagNumber(3)
  set viewerActor(Actor value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasViewerActor() => $_has(2);
  @$pb.TagNumber(3)
  void clearViewerActor() => $_clearField(3);
  @$pb.TagNumber(3)
  Actor ensureViewerActor() => $_ensure(2);

  /// 觀看者能不能在這家店的討論區新增頂層留言：買家沒有被這家店封鎖，或這就是觀看者此刻的店（訪客是 false）。顯示用的提示，寫入時伺服器仍會檢查（店主還要有行銷權限）。
  /// 意思是「能不能新增頂層討論區留言」，列回覆（設了 parent_id）時也照樣設、不隨 parent_id 變；單一留言能不能回覆看 ForumEntry.viewer_can_reply。
  @$pb.TagNumber(4)
  $core.bool get viewerCanComment => $_getBF(3);
  @$pb.TagNumber(4)
  set viewerCanComment($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasViewerCanComment() => $_has(3);
  @$pb.TagNumber(4)
  void clearViewerCanComment() => $_clearField(4);
}

class RecordPostViewRequest extends $pb.GeneratedMessage {
  factory RecordPostViewRequest({
    $core.String? postId,
  }) {
    final result = RecordPostViewRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  RecordPostViewRequest._();

  factory RecordPostViewRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordPostViewRequest()..mergeFromBuffer(data, registry);
  factory RecordPostViewRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordPostViewRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordPostViewRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: RecordPostViewRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordPostViewRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordPostViewRequest copyWith(
          void Function(RecordPostViewRequest) updates) =>
      super.copyWith((message) => updates(message as RecordPostViewRequest))
          as RecordPostViewRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RecordPostViewRequest() / RecordPostViewRequest.new instead')
  static RecordPostViewRequest create() => RecordPostViewRequest._();
  static $pb.GeneratedMessage $_createMessage() => RecordPostViewRequest._();
  @$core.override
  RecordPostViewRequest createEmptyInstance() => RecordPostViewRequest._();
  @$core.pragma('dart2js:noInline')
  static RecordPostViewRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecordPostViewRequest>(
          RecordPostViewRequest.$_createMessage);
  static RecordPostViewRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class RecordPostViewResponse extends $pb.GeneratedMessage {
  factory RecordPostViewResponse() => RecordPostViewResponse._();

  RecordPostViewResponse._();

  factory RecordPostViewResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordPostViewResponse()..mergeFromBuffer(data, registry);
  factory RecordPostViewResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RecordPostViewResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RecordPostViewResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: RecordPostViewResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordPostViewResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RecordPostViewResponse copyWith(
          void Function(RecordPostViewResponse) updates) =>
      super.copyWith((message) => updates(message as RecordPostViewResponse))
          as RecordPostViewResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RecordPostViewResponse() / RecordPostViewResponse.new instead')
  static RecordPostViewResponse create() => RecordPostViewResponse._();
  static $pb.GeneratedMessage $_createMessage() => RecordPostViewResponse._();
  @$core.override
  RecordPostViewResponse createEmptyInstance() => RecordPostViewResponse._();
  @$core.pragma('dart2js:noInline')
  static RecordPostViewResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RecordPostViewResponse>(
          RecordPostViewResponse.$_createMessage);
  static RecordPostViewResponse? _defaultInstance;
}

class SharePostRequest extends $pb.GeneratedMessage {
  factory SharePostRequest({
    $core.String? postId,
  }) {
    final result = SharePostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  SharePostRequest._();

  factory SharePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SharePostRequest()..mergeFromBuffer(data, registry);
  factory SharePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SharePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SharePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SharePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharePostRequest copyWith(void Function(SharePostRequest) updates) =>
      super.copyWith((message) => updates(message as SharePostRequest))
          as SharePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SharePostRequest() / SharePostRequest.new instead')
  static SharePostRequest create() => SharePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => SharePostRequest._();
  @$core.override
  SharePostRequest createEmptyInstance() => SharePostRequest._();
  @$core.pragma('dart2js:noInline')
  static SharePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SharePostRequest>(
          SharePostRequest.$_createMessage);
  static SharePostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class SharePostResponse extends $pb.GeneratedMessage {
  factory SharePostResponse() => SharePostResponse._();

  SharePostResponse._();

  factory SharePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SharePostResponse()..mergeFromBuffer(data, registry);
  factory SharePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SharePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SharePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SharePostResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SharePostResponse copyWith(void Function(SharePostResponse) updates) =>
      super.copyWith((message) => updates(message as SharePostResponse))
          as SharePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SharePostResponse() / SharePostResponse.new instead')
  static SharePostResponse create() => SharePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => SharePostResponse._();
  @$core.override
  SharePostResponse createEmptyInstance() => SharePostResponse._();
  @$core.pragma('dart2js:noInline')
  static SharePostResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SharePostResponse>(
          SharePostResponse.$_createMessage);
  static SharePostResponse? _defaultInstance;
}

class LikePostRequest extends $pb.GeneratedMessage {
  factory LikePostRequest({
    $core.String? postId,
  }) {
    final result = LikePostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  LikePostRequest._();

  factory LikePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikePostRequest()..mergeFromBuffer(data, registry);
  factory LikePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LikePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: LikePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikePostRequest copyWith(void Function(LikePostRequest) updates) =>
      super.copyWith((message) => updates(message as LikePostRequest))
          as LikePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use LikePostRequest() / LikePostRequest.new instead')
  static LikePostRequest create() => LikePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => LikePostRequest._();
  @$core.override
  LikePostRequest createEmptyInstance() => LikePostRequest._();
  @$core.pragma('dart2js:noInline')
  static LikePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LikePostRequest>(
          LikePostRequest.$_createMessage);
  static LikePostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class LikePostResponse extends $pb.GeneratedMessage {
  factory LikePostResponse() => LikePostResponse._();

  LikePostResponse._();

  factory LikePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikePostResponse()..mergeFromBuffer(data, registry);
  factory LikePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LikePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: LikePostResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikePostResponse copyWith(void Function(LikePostResponse) updates) =>
      super.copyWith((message) => updates(message as LikePostResponse))
          as LikePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use LikePostResponse() / LikePostResponse.new instead')
  static LikePostResponse create() => LikePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => LikePostResponse._();
  @$core.override
  LikePostResponse createEmptyInstance() => LikePostResponse._();
  @$core.pragma('dart2js:noInline')
  static LikePostResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<LikePostResponse>(
          LikePostResponse.$_createMessage);
  static LikePostResponse? _defaultInstance;
}

class UnlikePostRequest extends $pb.GeneratedMessage {
  factory UnlikePostRequest({
    $core.String? postId,
  }) {
    final result = UnlikePostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  UnlikePostRequest._();

  factory UnlikePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikePostRequest()..mergeFromBuffer(data, registry);
  factory UnlikePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnlikePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnlikePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikePostRequest copyWith(void Function(UnlikePostRequest) updates) =>
      super.copyWith((message) => updates(message as UnlikePostRequest))
          as UnlikePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UnlikePostRequest() / UnlikePostRequest.new instead')
  static UnlikePostRequest create() => UnlikePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => UnlikePostRequest._();
  @$core.override
  UnlikePostRequest createEmptyInstance() => UnlikePostRequest._();
  @$core.pragma('dart2js:noInline')
  static UnlikePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UnlikePostRequest>(
          UnlikePostRequest.$_createMessage);
  static UnlikePostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class UnlikePostResponse extends $pb.GeneratedMessage {
  factory UnlikePostResponse() => UnlikePostResponse._();

  UnlikePostResponse._();

  factory UnlikePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikePostResponse()..mergeFromBuffer(data, registry);
  factory UnlikePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnlikePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnlikePostResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikePostResponse copyWith(void Function(UnlikePostResponse) updates) =>
      super.copyWith((message) => updates(message as UnlikePostResponse))
          as UnlikePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UnlikePostResponse() / UnlikePostResponse.new instead')
  static UnlikePostResponse create() => UnlikePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => UnlikePostResponse._();
  @$core.override
  UnlikePostResponse createEmptyInstance() => UnlikePostResponse._();
  @$core.pragma('dart2js:noInline')
  static UnlikePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnlikePostResponse>(
          UnlikePostResponse.$_createMessage);
  static UnlikePostResponse? _defaultInstance;
}

class LikeCommentRequest extends $pb.GeneratedMessage {
  factory LikeCommentRequest({
    $core.String? commentId,
  }) {
    final result = LikeCommentRequest._();
    if (commentId != null) result.commentId = commentId;
    return result;
  }

  LikeCommentRequest._();

  factory LikeCommentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikeCommentRequest()..mergeFromBuffer(data, registry);
  factory LikeCommentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikeCommentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LikeCommentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: LikeCommentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'commentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikeCommentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikeCommentRequest copyWith(void Function(LikeCommentRequest) updates) =>
      super.copyWith((message) => updates(message as LikeCommentRequest))
          as LikeCommentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use LikeCommentRequest() / LikeCommentRequest.new instead')
  static LikeCommentRequest create() => LikeCommentRequest._();
  static $pb.GeneratedMessage $_createMessage() => LikeCommentRequest._();
  @$core.override
  LikeCommentRequest createEmptyInstance() => LikeCommentRequest._();
  @$core.pragma('dart2js:noInline')
  static LikeCommentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LikeCommentRequest>(
          LikeCommentRequest.$_createMessage);
  static LikeCommentRequest? _defaultInstance;

  /// 留言 ID。
  @$pb.TagNumber(1)
  $core.String get commentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set commentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCommentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCommentId() => $_clearField(1);
}

class LikeCommentResponse extends $pb.GeneratedMessage {
  factory LikeCommentResponse() => LikeCommentResponse._();

  LikeCommentResponse._();

  factory LikeCommentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikeCommentResponse()..mergeFromBuffer(data, registry);
  factory LikeCommentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      LikeCommentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LikeCommentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: LikeCommentResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikeCommentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LikeCommentResponse copyWith(void Function(LikeCommentResponse) updates) =>
      super.copyWith((message) => updates(message as LikeCommentResponse))
          as LikeCommentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use LikeCommentResponse() / LikeCommentResponse.new instead')
  static LikeCommentResponse create() => LikeCommentResponse._();
  static $pb.GeneratedMessage $_createMessage() => LikeCommentResponse._();
  @$core.override
  LikeCommentResponse createEmptyInstance() => LikeCommentResponse._();
  @$core.pragma('dart2js:noInline')
  static LikeCommentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LikeCommentResponse>(
          LikeCommentResponse.$_createMessage);
  static LikeCommentResponse? _defaultInstance;
}

class UnlikeCommentRequest extends $pb.GeneratedMessage {
  factory UnlikeCommentRequest({
    $core.String? commentId,
  }) {
    final result = UnlikeCommentRequest._();
    if (commentId != null) result.commentId = commentId;
    return result;
  }

  UnlikeCommentRequest._();

  factory UnlikeCommentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikeCommentRequest()..mergeFromBuffer(data, registry);
  factory UnlikeCommentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikeCommentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnlikeCommentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnlikeCommentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'commentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikeCommentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikeCommentRequest copyWith(void Function(UnlikeCommentRequest) updates) =>
      super.copyWith((message) => updates(message as UnlikeCommentRequest))
          as UnlikeCommentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnlikeCommentRequest() / UnlikeCommentRequest.new instead')
  static UnlikeCommentRequest create() => UnlikeCommentRequest._();
  static $pb.GeneratedMessage $_createMessage() => UnlikeCommentRequest._();
  @$core.override
  UnlikeCommentRequest createEmptyInstance() => UnlikeCommentRequest._();
  @$core.pragma('dart2js:noInline')
  static UnlikeCommentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnlikeCommentRequest>(
          UnlikeCommentRequest.$_createMessage);
  static UnlikeCommentRequest? _defaultInstance;

  /// 留言 ID。
  @$pb.TagNumber(1)
  $core.String get commentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set commentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCommentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCommentId() => $_clearField(1);
}

class UnlikeCommentResponse extends $pb.GeneratedMessage {
  factory UnlikeCommentResponse() => UnlikeCommentResponse._();

  UnlikeCommentResponse._();

  factory UnlikeCommentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikeCommentResponse()..mergeFromBuffer(data, registry);
  factory UnlikeCommentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnlikeCommentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnlikeCommentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnlikeCommentResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikeCommentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnlikeCommentResponse copyWith(
          void Function(UnlikeCommentResponse) updates) =>
      super.copyWith((message) => updates(message as UnlikeCommentResponse))
          as UnlikeCommentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnlikeCommentResponse() / UnlikeCommentResponse.new instead')
  static UnlikeCommentResponse create() => UnlikeCommentResponse._();
  static $pb.GeneratedMessage $_createMessage() => UnlikeCommentResponse._();
  @$core.override
  UnlikeCommentResponse createEmptyInstance() => UnlikeCommentResponse._();
  @$core.pragma('dart2js:noInline')
  static UnlikeCommentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnlikeCommentResponse>(
          UnlikeCommentResponse.$_createMessage);
  static UnlikeCommentResponse? _defaultInstance;
}

class CreateCommentRequest extends $pb.GeneratedMessage {
  factory CreateCommentRequest({
    $core.String? postId,
    $core.String? body,
    $core.String? parentId,
    $core.String? replyToId,
    $core.String? requestId,
  }) {
    final result = CreateCommentRequest._();
    if (postId != null) result.postId = postId;
    if (body != null) result.body = body;
    if (parentId != null) result.parentId = parentId;
    if (replyToId != null) result.replyToId = replyToId;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  CreateCommentRequest._();

  factory CreateCommentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCommentRequest()..mergeFromBuffer(data, registry);
  factory CreateCommentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCommentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCommentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreateCommentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aOS(2, _omitFieldNames ? '' : 'body')
    ..aOS(3, _omitFieldNames ? '' : 'parentId')
    ..aOS(4, _omitFieldNames ? '' : 'replyToId')
    ..aOS(5, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCommentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCommentRequest copyWith(void Function(CreateCommentRequest) updates) =>
      super.copyWith((message) => updates(message as CreateCommentRequest))
          as CreateCommentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateCommentRequest() / CreateCommentRequest.new instead')
  static CreateCommentRequest create() => CreateCommentRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateCommentRequest._();
  @$core.override
  CreateCommentRequest createEmptyInstance() => CreateCommentRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateCommentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCommentRequest>(
          CreateCommentRequest.$_createMessage);
  static CreateCommentRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 內文，去除首尾空白後 1 到 2200 個字元（以 Unicode 字元計）。
  @$pb.TagNumber(2)
  $core.String get body => $_getSZ(1);
  @$pb.TagNumber(2)
  set body($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBody() => $_has(1);
  @$pb.TagNumber(2)
  void clearBody() => $_clearField(2);

  /// 設了是回覆；父留言不是頂層時，回覆掛在它所屬的頂層留言底下。不設是頂層留言。
  @$pb.TagNumber(3)
  $core.String get parentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set parentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasParentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearParentId() => $_clearField(3);

  /// 回覆針對的那一則留言，必須與 parent_id 同一串；只能在設了 parent_id 時設，不設是父留言。
  @$pb.TagNumber(4)
  $core.String get replyToId => $_getSZ(3);
  @$pb.TagNumber(4)
  set replyToId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReplyToId() => $_has(3);
  @$pb.TagNumber(4)
  void clearReplyToId() => $_clearField(4);

  /// 選填，UUIDv4（AIP-155）：同一位互動者以同一個 request_id 重送回這則留言目前的樣子；內容不同回 invalid_argument。
  @$pb.TagNumber(5)
  $core.String get requestId => $_getSZ(4);
  @$pb.TagNumber(5)
  set requestId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRequestId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRequestId() => $_clearField(5);
}

class CreateCommentResponse extends $pb.GeneratedMessage {
  factory CreateCommentResponse({
    Comment? comment,
  }) {
    final result = CreateCommentResponse._();
    if (comment != null) result.comment = comment;
    return result;
  }

  CreateCommentResponse._();

  factory CreateCommentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCommentResponse()..mergeFromBuffer(data, registry);
  factory CreateCommentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateCommentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCommentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreateCommentResponse.$_createMessage)
    ..aOM<Comment>(1, _omitFieldNames ? '' : 'comment',
        subBuilder: Comment.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCommentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCommentResponse copyWith(
          void Function(CreateCommentResponse) updates) =>
      super.copyWith((message) => updates(message as CreateCommentResponse))
          as CreateCommentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateCommentResponse() / CreateCommentResponse.new instead')
  static CreateCommentResponse create() => CreateCommentResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateCommentResponse._();
  @$core.override
  CreateCommentResponse createEmptyInstance() => CreateCommentResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateCommentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCommentResponse>(
          CreateCommentResponse.$_createMessage);
  static CreateCommentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Comment get comment => $_getN(0);
  @$pb.TagNumber(1)
  set comment(Comment value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasComment() => $_has(0);
  @$pb.TagNumber(1)
  void clearComment() => $_clearField(1);
  @$pb.TagNumber(1)
  Comment ensureComment() => $_ensure(0);
}

class UpdateCommentRequest extends $pb.GeneratedMessage {
  factory UpdateCommentRequest({
    Comment? comment,
    $3.FieldMask? updateMask,
  }) {
    final result = UpdateCommentRequest._();
    if (comment != null) result.comment = comment;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateCommentRequest._();

  factory UpdateCommentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCommentRequest()..mergeFromBuffer(data, registry);
  factory UpdateCommentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCommentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateCommentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdateCommentRequest.$_createMessage)
    ..aOM<Comment>(1, _omitFieldNames ? '' : 'comment',
        subBuilder: Comment.$_createMessage)
    ..aOM<$3.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $3.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCommentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCommentRequest copyWith(void Function(UpdateCommentRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateCommentRequest))
          as UpdateCommentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateCommentRequest() / UpdateCommentRequest.new instead')
  static UpdateCommentRequest create() => UpdateCommentRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateCommentRequest._();
  @$core.override
  UpdateCommentRequest createEmptyInstance() => UpdateCommentRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateCommentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateCommentRequest>(
          UpdateCommentRequest.$_createMessage);
  static UpdateCommentRequest? _defaultInstance;

  /// id 與 etag 必填；body 的規則同 CreateCommentRequest.body。留言只有內文可以修改，update_mask 要列 body。
  @$pb.TagNumber(1)
  Comment get comment => $_getN(0);
  @$pb.TagNumber(1)
  set comment(Comment value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasComment() => $_has(0);
  @$pb.TagNumber(1)
  void clearComment() => $_clearField(1);
  @$pb.TagNumber(1)
  Comment ensureComment() => $_ensure(0);

  @$pb.TagNumber(2)
  $3.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($3.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.FieldMask ensureUpdateMask() => $_ensure(1);
}

/// UpdateCommentResponse 的留言是修改後的完整資源（含讚數、回覆數與新的 etag）。
class UpdateCommentResponse extends $pb.GeneratedMessage {
  factory UpdateCommentResponse({
    Comment? comment,
  }) {
    final result = UpdateCommentResponse._();
    if (comment != null) result.comment = comment;
    return result;
  }

  UpdateCommentResponse._();

  factory UpdateCommentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCommentResponse()..mergeFromBuffer(data, registry);
  factory UpdateCommentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateCommentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateCommentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdateCommentResponse.$_createMessage)
    ..aOM<Comment>(1, _omitFieldNames ? '' : 'comment',
        subBuilder: Comment.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCommentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCommentResponse copyWith(
          void Function(UpdateCommentResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateCommentResponse))
          as UpdateCommentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateCommentResponse() / UpdateCommentResponse.new instead')
  static UpdateCommentResponse create() => UpdateCommentResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateCommentResponse._();
  @$core.override
  UpdateCommentResponse createEmptyInstance() => UpdateCommentResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateCommentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateCommentResponse>(
          UpdateCommentResponse.$_createMessage);
  static UpdateCommentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Comment get comment => $_getN(0);
  @$pb.TagNumber(1)
  set comment(Comment value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasComment() => $_has(0);
  @$pb.TagNumber(1)
  void clearComment() => $_clearField(1);
  @$pb.TagNumber(1)
  Comment ensureComment() => $_ensure(0);
}

class DeleteCommentRequest extends $pb.GeneratedMessage {
  factory DeleteCommentRequest({
    $core.String? commentId,
    $core.String? etag,
  }) {
    final result = DeleteCommentRequest._();
    if (commentId != null) result.commentId = commentId;
    if (etag != null) result.etag = etag;
    return result;
  }

  DeleteCommentRequest._();

  factory DeleteCommentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCommentRequest()..mergeFromBuffer(data, registry);
  factory DeleteCommentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCommentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteCommentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DeleteCommentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'commentId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCommentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCommentRequest copyWith(void Function(DeleteCommentRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteCommentRequest))
          as DeleteCommentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteCommentRequest() / DeleteCommentRequest.new instead')
  static DeleteCommentRequest create() => DeleteCommentRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteCommentRequest._();
  @$core.override
  DeleteCommentRequest createEmptyInstance() => DeleteCommentRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteCommentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteCommentRequest>(
          DeleteCommentRequest.$_createMessage);
  static DeleteCommentRequest? _defaultInstance;

  /// 留言 ID。
  @$pb.TagNumber(1)
  $core.String get commentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set commentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCommentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCommentId() => $_clearField(1);

  /// 帶了就檢查版本；已經刪除的留言不檢查。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class DeleteCommentResponse extends $pb.GeneratedMessage {
  factory DeleteCommentResponse() => DeleteCommentResponse._();

  DeleteCommentResponse._();

  factory DeleteCommentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCommentResponse()..mergeFromBuffer(data, registry);
  factory DeleteCommentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCommentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteCommentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DeleteCommentResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCommentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCommentResponse copyWith(
          void Function(DeleteCommentResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteCommentResponse))
          as DeleteCommentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteCommentResponse() / DeleteCommentResponse.new instead')
  static DeleteCommentResponse create() => DeleteCommentResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteCommentResponse._();
  @$core.override
  DeleteCommentResponse createEmptyInstance() => DeleteCommentResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteCommentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteCommentResponse>(
          DeleteCommentResponse.$_createMessage);
  static DeleteCommentResponse? _defaultInstance;
}

class CreateForumEntryRequest extends $pb.GeneratedMessage {
  factory CreateForumEntryRequest({
    $core.String? storeId,
    $core.String? body,
    $core.String? parentId,
    $core.String? replyToId,
    $core.String? requestId,
  }) {
    final result = CreateForumEntryRequest._();
    if (storeId != null) result.storeId = storeId;
    if (body != null) result.body = body;
    if (parentId != null) result.parentId = parentId;
    if (replyToId != null) result.replyToId = replyToId;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  CreateForumEntryRequest._();

  factory CreateForumEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateForumEntryRequest()..mergeFromBuffer(data, registry);
  factory CreateForumEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateForumEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateForumEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreateForumEntryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'body')
    ..aOS(3, _omitFieldNames ? '' : 'parentId')
    ..aOS(4, _omitFieldNames ? '' : 'replyToId')
    ..aOS(5, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateForumEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateForumEntryRequest copyWith(
          void Function(CreateForumEntryRequest) updates) =>
      super.copyWith((message) => updates(message as CreateForumEntryRequest))
          as CreateForumEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateForumEntryRequest() / CreateForumEntryRequest.new instead')
  static CreateForumEntryRequest create() => CreateForumEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateForumEntryRequest._();
  @$core.override
  CreateForumEntryRequest createEmptyInstance() => CreateForumEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateForumEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateForumEntryRequest>(
          CreateForumEntryRequest.$_createMessage);
  static CreateForumEntryRequest? _defaultInstance;

  /// 店家 ID。
  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 內文，規則同 CreateCommentRequest.body。
  @$pb.TagNumber(2)
  $core.String get body => $_getSZ(1);
  @$pb.TagNumber(2)
  set body($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBody() => $_has(1);
  @$pb.TagNumber(2)
  void clearBody() => $_clearField(2);

  /// 設了是回覆；父項不是頂層時，回覆掛在它所屬的頂層項目底下。不設是頂層發言。
  @$pb.TagNumber(3)
  $core.String get parentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set parentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasParentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearParentId() => $_clearField(3);

  /// 回覆針對的那一則，必須與 parent_id 同一串；只能在設了 parent_id 時設，不設是父項。
  @$pb.TagNumber(4)
  $core.String get replyToId => $_getSZ(3);
  @$pb.TagNumber(4)
  set replyToId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReplyToId() => $_has(3);
  @$pb.TagNumber(4)
  void clearReplyToId() => $_clearField(4);

  /// 選填，UUIDv4（AIP-155）：同一位互動者以同一個 request_id 重送回這個項目目前的樣子；內容不同回 invalid_argument。
  @$pb.TagNumber(5)
  $core.String get requestId => $_getSZ(4);
  @$pb.TagNumber(5)
  set requestId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRequestId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRequestId() => $_clearField(5);
}

class CreateForumEntryResponse extends $pb.GeneratedMessage {
  factory CreateForumEntryResponse({
    ForumEntry? entry,
  }) {
    final result = CreateForumEntryResponse._();
    if (entry != null) result.entry = entry;
    return result;
  }

  CreateForumEntryResponse._();

  factory CreateForumEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateForumEntryResponse()..mergeFromBuffer(data, registry);
  factory CreateForumEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateForumEntryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateForumEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreateForumEntryResponse.$_createMessage)
    ..aOM<ForumEntry>(1, _omitFieldNames ? '' : 'entry',
        subBuilder: ForumEntry.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateForumEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateForumEntryResponse copyWith(
          void Function(CreateForumEntryResponse) updates) =>
      super.copyWith((message) => updates(message as CreateForumEntryResponse))
          as CreateForumEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateForumEntryResponse() / CreateForumEntryResponse.new instead')
  static CreateForumEntryResponse create() => CreateForumEntryResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateForumEntryResponse._();
  @$core.override
  CreateForumEntryResponse createEmptyInstance() =>
      CreateForumEntryResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateForumEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateForumEntryResponse>(
          CreateForumEntryResponse.$_createMessage);
  static CreateForumEntryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ForumEntry get entry => $_getN(0);
  @$pb.TagNumber(1)
  set entry(ForumEntry value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEntry() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntry() => $_clearField(1);
  @$pb.TagNumber(1)
  ForumEntry ensureEntry() => $_ensure(0);
}

class UpdateForumEntryRequest extends $pb.GeneratedMessage {
  factory UpdateForumEntryRequest({
    ForumEntry? entry,
    $3.FieldMask? updateMask,
  }) {
    final result = UpdateForumEntryRequest._();
    if (entry != null) result.entry = entry;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateForumEntryRequest._();

  factory UpdateForumEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateForumEntryRequest()..mergeFromBuffer(data, registry);
  factory UpdateForumEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateForumEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateForumEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdateForumEntryRequest.$_createMessage)
    ..aOM<ForumEntry>(1, _omitFieldNames ? '' : 'entry',
        subBuilder: ForumEntry.$_createMessage)
    ..aOM<$3.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $3.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateForumEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateForumEntryRequest copyWith(
          void Function(UpdateForumEntryRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateForumEntryRequest))
          as UpdateForumEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateForumEntryRequest() / UpdateForumEntryRequest.new instead')
  static UpdateForumEntryRequest create() => UpdateForumEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateForumEntryRequest._();
  @$core.override
  UpdateForumEntryRequest createEmptyInstance() => UpdateForumEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateForumEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateForumEntryRequest>(
          UpdateForumEntryRequest.$_createMessage);
  static UpdateForumEntryRequest? _defaultInstance;

  /// id 與 etag 必填；body 的規則同 CreateCommentRequest.body。討論區項目只有內文可以修改，update_mask 要列 body。
  @$pb.TagNumber(1)
  ForumEntry get entry => $_getN(0);
  @$pb.TagNumber(1)
  set entry(ForumEntry value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEntry() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntry() => $_clearField(1);
  @$pb.TagNumber(1)
  ForumEntry ensureEntry() => $_ensure(0);

  @$pb.TagNumber(2)
  $3.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($3.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.FieldMask ensureUpdateMask() => $_ensure(1);
}

/// UpdateForumEntryResponse 的項目是修改後的完整資源（含回覆數與新的 etag）。
class UpdateForumEntryResponse extends $pb.GeneratedMessage {
  factory UpdateForumEntryResponse({
    ForumEntry? entry,
  }) {
    final result = UpdateForumEntryResponse._();
    if (entry != null) result.entry = entry;
    return result;
  }

  UpdateForumEntryResponse._();

  factory UpdateForumEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateForumEntryResponse()..mergeFromBuffer(data, registry);
  factory UpdateForumEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateForumEntryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateForumEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdateForumEntryResponse.$_createMessage)
    ..aOM<ForumEntry>(1, _omitFieldNames ? '' : 'entry',
        subBuilder: ForumEntry.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateForumEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateForumEntryResponse copyWith(
          void Function(UpdateForumEntryResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateForumEntryResponse))
          as UpdateForumEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateForumEntryResponse() / UpdateForumEntryResponse.new instead')
  static UpdateForumEntryResponse create() => UpdateForumEntryResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateForumEntryResponse._();
  @$core.override
  UpdateForumEntryResponse createEmptyInstance() =>
      UpdateForumEntryResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateForumEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateForumEntryResponse>(
          UpdateForumEntryResponse.$_createMessage);
  static UpdateForumEntryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ForumEntry get entry => $_getN(0);
  @$pb.TagNumber(1)
  set entry(ForumEntry value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEntry() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntry() => $_clearField(1);
  @$pb.TagNumber(1)
  ForumEntry ensureEntry() => $_ensure(0);
}

class DeleteForumEntryRequest extends $pb.GeneratedMessage {
  factory DeleteForumEntryRequest({
    $core.String? entryId,
    $core.String? etag,
  }) {
    final result = DeleteForumEntryRequest._();
    if (entryId != null) result.entryId = entryId;
    if (etag != null) result.etag = etag;
    return result;
  }

  DeleteForumEntryRequest._();

  factory DeleteForumEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteForumEntryRequest()..mergeFromBuffer(data, registry);
  factory DeleteForumEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteForumEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteForumEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DeleteForumEntryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'entryId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteForumEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteForumEntryRequest copyWith(
          void Function(DeleteForumEntryRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteForumEntryRequest))
          as DeleteForumEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteForumEntryRequest() / DeleteForumEntryRequest.new instead')
  static DeleteForumEntryRequest create() => DeleteForumEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteForumEntryRequest._();
  @$core.override
  DeleteForumEntryRequest createEmptyInstance() => DeleteForumEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteForumEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteForumEntryRequest>(
          DeleteForumEntryRequest.$_createMessage);
  static DeleteForumEntryRequest? _defaultInstance;

  /// 討論區項目 ID。
  @$pb.TagNumber(1)
  $core.String get entryId => $_getSZ(0);
  @$pb.TagNumber(1)
  set entryId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEntryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntryId() => $_clearField(1);

  /// 帶了就檢查版本；已經刪除的項目不檢查。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class DeleteForumEntryResponse extends $pb.GeneratedMessage {
  factory DeleteForumEntryResponse() => DeleteForumEntryResponse._();

  DeleteForumEntryResponse._();

  factory DeleteForumEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteForumEntryResponse()..mergeFromBuffer(data, registry);
  factory DeleteForumEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteForumEntryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteForumEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DeleteForumEntryResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteForumEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteForumEntryResponse copyWith(
          void Function(DeleteForumEntryResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteForumEntryResponse))
          as DeleteForumEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteForumEntryResponse() / DeleteForumEntryResponse.new instead')
  static DeleteForumEntryResponse create() => DeleteForumEntryResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteForumEntryResponse._();
  @$core.override
  DeleteForumEntryResponse createEmptyInstance() =>
      DeleteForumEntryResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteForumEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteForumEntryResponse>(
          DeleteForumEntryResponse.$_createMessage);
  static DeleteForumEntryResponse? _defaultInstance;
}

class CreatePostRequest extends $pb.GeneratedMessage {
  factory CreatePostRequest({
    $core.String? body,
    $core.String? imageStagedToken,
    $core.String? productId,
    $core.Iterable<$core.String>? hashtagInputs,
    $core.bool? publish,
    $core.String? requestId,
  }) {
    final result = CreatePostRequest._();
    if (body != null) result.body = body;
    if (imageStagedToken != null) result.imageStagedToken = imageStagedToken;
    if (productId != null) result.productId = productId;
    if (hashtagInputs != null) result.hashtagInputs.addAll(hashtagInputs);
    if (publish != null) result.publish = publish;
    if (requestId != null) result.requestId = requestId;
    return result;
  }

  CreatePostRequest._();

  factory CreatePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostRequest()..mergeFromBuffer(data, registry);
  factory CreatePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreatePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreatePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'body')
    ..aOS(2, _omitFieldNames ? '' : 'imageStagedToken')
    ..aOS(3, _omitFieldNames ? '' : 'productId')
    ..pPS(4, _omitFieldNames ? '' : 'hashtagInputs')
    ..aOB(5, _omitFieldNames ? '' : 'publish')
    ..aOS(6, _omitFieldNames ? '' : 'requestId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostRequest copyWith(void Function(CreatePostRequest) updates) =>
      super.copyWith((message) => updates(message as CreatePostRequest))
          as CreatePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CreatePostRequest() / CreatePostRequest.new instead')
  static CreatePostRequest create() => CreatePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreatePostRequest._();
  @$core.override
  CreatePostRequest createEmptyInstance() => CreatePostRequest._();
  @$core.pragma('dart2js:noInline')
  static CreatePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CreatePostRequest>(
          CreatePostRequest.$_createMessage);
  static CreatePostRequest? _defaultInstance;

  /// 內文，去除首尾空白後最多 2200 個字元（以 Unicode 字元計）；草稿可以是空的，發布時必填。
  @$pb.TagNumber(1)
  $core.String get body => $_getSZ(0);
  @$pb.TagNumber(1)
  set body($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBody() => $_has(0);
  @$pb.TagNumber(1)
  void clearBody() => $_clearField(1);

  /// 圖片的 staged token（media 上傳完成後拿到的）；發布時必須有。
  @$pb.TagNumber(2)
  $core.String get imageStagedToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set imageStagedToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasImageStagedToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearImageStagedToken() => $_clearField(2);

  /// 貼文帶的商品 ID；商品要存在、上架中而且屬於這家店。
  @$pb.TagNumber(3)
  $core.String get productId => $_getSZ(2);
  @$pb.TagNumber(3)
  set productId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProductId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProductId() => $_clearField(3);

  /// 標籤（使用者輸入的樣子，可帶開頭的 #）。
  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get hashtagInputs => $_getList(3);

  /// 為真時建立後立刻發布。
  @$pb.TagNumber(5)
  $core.bool get publish => $_getBF(4);
  @$pb.TagNumber(5)
  set publish($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPublish() => $_has(4);
  @$pb.TagNumber(5)
  void clearPublish() => $_clearField(5);

  /// 選填，UUIDv4（AIP-155）：同一家店以同一個 request_id 重送回這篇貼文目前的樣子；內容不同回 invalid_argument。
  @$pb.TagNumber(6)
  $core.String get requestId => $_getSZ(5);
  @$pb.TagNumber(6)
  set requestId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRequestId() => $_has(5);
  @$pb.TagNumber(6)
  void clearRequestId() => $_clearField(6);
}

class CreatePostResponse extends $pb.GeneratedMessage {
  factory CreatePostResponse({
    Post? post,
  }) {
    final result = CreatePostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  CreatePostResponse._();

  factory CreatePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostResponse()..mergeFromBuffer(data, registry);
  factory CreatePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreatePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreatePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreatePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreatePostResponse copyWith(void Function(CreatePostResponse) updates) =>
      super.copyWith((message) => updates(message as CreatePostResponse))
          as CreatePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CreatePostResponse() / CreatePostResponse.new instead')
  static CreatePostResponse create() => CreatePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreatePostResponse._();
  @$core.override
  CreatePostResponse createEmptyInstance() => CreatePostResponse._();
  @$core.pragma('dart2js:noInline')
  static CreatePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreatePostResponse>(
          CreatePostResponse.$_createMessage);
  static CreatePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class GetMyPostRequest extends $pb.GeneratedMessage {
  factory GetMyPostRequest({
    $core.String? postId,
  }) {
    final result = GetMyPostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  GetMyPostRequest._();

  factory GetMyPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyPostRequest()..mergeFromBuffer(data, registry);
  factory GetMyPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetMyPostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPostRequest copyWith(void Function(GetMyPostRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyPostRequest))
          as GetMyPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyPostRequest() / GetMyPostRequest.new instead')
  static GetMyPostRequest create() => GetMyPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetMyPostRequest._();
  @$core.override
  GetMyPostRequest createEmptyInstance() => GetMyPostRequest._();
  @$core.pragma('dart2js:noInline')
  static GetMyPostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetMyPostRequest>(
          GetMyPostRequest.$_createMessage);
  static GetMyPostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class GetMyPostResponse extends $pb.GeneratedMessage {
  factory GetMyPostResponse({
    Post? post,
  }) {
    final result = GetMyPostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  GetMyPostResponse._();

  factory GetMyPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyPostResponse()..mergeFromBuffer(data, registry);
  factory GetMyPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetMyPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetMyPostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPostResponse copyWith(void Function(GetMyPostResponse) updates) =>
      super.copyWith((message) => updates(message as GetMyPostResponse))
          as GetMyPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetMyPostResponse() / GetMyPostResponse.new instead')
  static GetMyPostResponse create() => GetMyPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetMyPostResponse._();
  @$core.override
  GetMyPostResponse createEmptyInstance() => GetMyPostResponse._();
  @$core.pragma('dart2js:noInline')
  static GetMyPostResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetMyPostResponse>(
          GetMyPostResponse.$_createMessage);
  static GetMyPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class ListMyPostsRequest extends $pb.GeneratedMessage {
  factory ListMyPostsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    PostStatus? status,
  }) {
    final result = ListMyPostsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (status != null) result.status = status;
    return result;
  }

  ListMyPostsRequest._();

  factory ListMyPostsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyPostsRequest()..mergeFromBuffer(data, registry);
  factory ListMyPostsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyPostsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyPostsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListMyPostsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aE<PostStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: PostStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyPostsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyPostsRequest copyWith(void Function(ListMyPostsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyPostsRequest))
          as ListMyPostsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListMyPostsRequest() / ListMyPostsRequest.new instead')
  static ListMyPostsRequest create() => ListMyPostsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyPostsRequest._();
  @$core.override
  ListMyPostsRequest createEmptyInstance() => ListMyPostsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyPostsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyPostsRequest>(
          ListMyPostsRequest.$_createMessage);
  static ListMyPostsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);

  /// 只列這個狀態的貼文；不設（UNSPECIFIED）是全部。page_token 只能用在同一個 status。
  @$pb.TagNumber(3)
  PostStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(PostStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);
}

class ListMyPostsResponse extends $pb.GeneratedMessage {
  factory ListMyPostsResponse({
    $core.Iterable<Post>? posts,
    $core.String? nextPageToken,
  }) {
    final result = ListMyPostsResponse._();
    if (posts != null) result.posts.addAll(posts);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyPostsResponse._();

  factory ListMyPostsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyPostsResponse()..mergeFromBuffer(data, registry);
  factory ListMyPostsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyPostsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyPostsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListMyPostsResponse.$_createMessage)
    ..pPM<Post>(1, _omitFieldNames ? '' : 'posts',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyPostsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyPostsResponse copyWith(void Function(ListMyPostsResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyPostsResponse))
          as ListMyPostsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListMyPostsResponse() / ListMyPostsResponse.new instead')
  static ListMyPostsResponse create() => ListMyPostsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyPostsResponse._();
  @$core.override
  ListMyPostsResponse createEmptyInstance() => ListMyPostsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyPostsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyPostsResponse>(
          ListMyPostsResponse.$_createMessage);
  static ListMyPostsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Post> get posts => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class UpdatePostRequest extends $pb.GeneratedMessage {
  factory UpdatePostRequest({
    Post? post,
    $3.FieldMask? updateMask,
  }) {
    final result = UpdatePostRequest._();
    if (post != null) result.post = post;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdatePostRequest._();

  factory UpdatePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostRequest()..mergeFromBuffer(data, registry);
  factory UpdatePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdatePostRequest.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOM<$3.FieldMask>(2, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $3.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostRequest copyWith(void Function(UpdatePostRequest) updates) =>
      super.copyWith((message) => updates(message as UpdatePostRequest))
          as UpdatePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UpdatePostRequest() / UpdatePostRequest.new instead')
  static UpdatePostRequest create() => UpdatePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdatePostRequest._();
  @$core.override
  UpdatePostRequest createEmptyInstance() => UpdatePostRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdatePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UpdatePostRequest>(
          UpdatePostRequest.$_createMessage);
  static UpdatePostRequest? _defaultInstance;

  /// id 與 etag 必填。可以修改的欄位是 body、image_staged_token、product_id、hashtag_inputs，update_mask 要列至少一個；沒列的欄位保留目前的值
  /// （product_id 列進 update_mask 而沒設是清掉商品）。
  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $3.FieldMask get updateMask => $_getN(1);
  @$pb.TagNumber(2)
  set updateMask($3.FieldMask value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUpdateMask() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpdateMask() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.FieldMask ensureUpdateMask() => $_ensure(1);
}

/// UpdatePostResponse 的貼文是修改後的完整資源（含新的 etag）。
class UpdatePostResponse extends $pb.GeneratedMessage {
  factory UpdatePostResponse({
    Post? post,
  }) {
    final result = UpdatePostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  UpdatePostResponse._();

  factory UpdatePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostResponse()..mergeFromBuffer(data, registry);
  factory UpdatePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdatePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdatePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePostResponse copyWith(void Function(UpdatePostResponse) updates) =>
      super.copyWith((message) => updates(message as UpdatePostResponse))
          as UpdatePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UpdatePostResponse() / UpdatePostResponse.new instead')
  static UpdatePostResponse create() => UpdatePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdatePostResponse._();
  @$core.override
  UpdatePostResponse createEmptyInstance() => UpdatePostResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdatePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdatePostResponse>(
          UpdatePostResponse.$_createMessage);
  static UpdatePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class PublishPostRequest extends $pb.GeneratedMessage {
  factory PublishPostRequest({
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = PublishPostRequest._();
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  PublishPostRequest._();

  factory PublishPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostRequest()..mergeFromBuffer(data, registry);
  factory PublishPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: PublishPostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostRequest copyWith(void Function(PublishPostRequest) updates) =>
      super.copyWith((message) => updates(message as PublishPostRequest))
          as PublishPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PublishPostRequest() / PublishPostRequest.new instead')
  static PublishPostRequest create() => PublishPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => PublishPostRequest._();
  @$core.override
  PublishPostRequest createEmptyInstance() => PublishPostRequest._();
  @$core.pragma('dart2js:noInline')
  static PublishPostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishPostRequest>(
          PublishPostRequest.$_createMessage);
  static PublishPostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class PublishPostResponse extends $pb.GeneratedMessage {
  factory PublishPostResponse({
    Post? post,
  }) {
    final result = PublishPostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  PublishPostResponse._();

  factory PublishPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostResponse()..mergeFromBuffer(data, registry);
  factory PublishPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: PublishPostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishPostResponse copyWith(void Function(PublishPostResponse) updates) =>
      super.copyWith((message) => updates(message as PublishPostResponse))
          as PublishPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use PublishPostResponse() / PublishPostResponse.new instead')
  static PublishPostResponse create() => PublishPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => PublishPostResponse._();
  @$core.override
  PublishPostResponse createEmptyInstance() => PublishPostResponse._();
  @$core.pragma('dart2js:noInline')
  static PublishPostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishPostResponse>(
          PublishPostResponse.$_createMessage);
  static PublishPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class UnpublishPostRequest extends $pb.GeneratedMessage {
  factory UnpublishPostRequest({
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = UnpublishPostRequest._();
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  UnpublishPostRequest._();

  factory UnpublishPostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostRequest()..mergeFromBuffer(data, registry);
  factory UnpublishPostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishPostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnpublishPostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostRequest copyWith(void Function(UnpublishPostRequest) updates) =>
      super.copyWith((message) => updates(message as UnpublishPostRequest))
          as UnpublishPostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishPostRequest() / UnpublishPostRequest.new instead')
  static UnpublishPostRequest create() => UnpublishPostRequest._();
  static $pb.GeneratedMessage $_createMessage() => UnpublishPostRequest._();
  @$core.override
  UnpublishPostRequest createEmptyInstance() => UnpublishPostRequest._();
  @$core.pragma('dart2js:noInline')
  static UnpublishPostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishPostRequest>(
          UnpublishPostRequest.$_createMessage);
  static UnpublishPostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class UnpublishPostResponse extends $pb.GeneratedMessage {
  factory UnpublishPostResponse({
    Post? post,
  }) {
    final result = UnpublishPostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  UnpublishPostResponse._();

  factory UnpublishPostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostResponse()..mergeFromBuffer(data, registry);
  factory UnpublishPostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishPostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishPostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnpublishPostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishPostResponse copyWith(
          void Function(UnpublishPostResponse) updates) =>
      super.copyWith((message) => updates(message as UnpublishPostResponse))
          as UnpublishPostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishPostResponse() / UnpublishPostResponse.new instead')
  static UnpublishPostResponse create() => UnpublishPostResponse._();
  static $pb.GeneratedMessage $_createMessage() => UnpublishPostResponse._();
  @$core.override
  UnpublishPostResponse createEmptyInstance() => UnpublishPostResponse._();
  @$core.pragma('dart2js:noInline')
  static UnpublishPostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishPostResponse>(
          UnpublishPostResponse.$_createMessage);
  static UnpublishPostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class DeletePostRequest extends $pb.GeneratedMessage {
  factory DeletePostRequest({
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = DeletePostRequest._();
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  DeletePostRequest._();

  factory DeletePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeletePostRequest()..mergeFromBuffer(data, registry);
  factory DeletePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeletePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeletePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DeletePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePostRequest copyWith(void Function(DeletePostRequest) updates) =>
      super.copyWith((message) => updates(message as DeletePostRequest))
          as DeletePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DeletePostRequest() / DeletePostRequest.new instead')
  static DeletePostRequest create() => DeletePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeletePostRequest._();
  @$core.override
  DeletePostRequest createEmptyInstance() => DeletePostRequest._();
  @$core.pragma('dart2js:noInline')
  static DeletePostRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DeletePostRequest>(
          DeletePostRequest.$_createMessage);
  static DeletePostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 帶了就檢查版本；已經刪除的貼文不檢查。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class DeletePostResponse extends $pb.GeneratedMessage {
  factory DeletePostResponse() => DeletePostResponse._();

  DeletePostResponse._();

  factory DeletePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeletePostResponse()..mergeFromBuffer(data, registry);
  factory DeletePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeletePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeletePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DeletePostResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePostResponse copyWith(void Function(DeletePostResponse) updates) =>
      super.copyWith((message) => updates(message as DeletePostResponse))
          as DeletePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DeletePostResponse() / DeletePostResponse.new instead')
  static DeletePostResponse create() => DeletePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeletePostResponse._();
  @$core.override
  DeletePostResponse createEmptyInstance() => DeletePostResponse._();
  @$core.pragma('dart2js:noInline')
  static DeletePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeletePostResponse>(
          DeletePostResponse.$_createMessage);
  static DeletePostResponse? _defaultInstance;
}

class SchedulePostRequest extends $pb.GeneratedMessage {
  factory SchedulePostRequest({
    $core.String? postId,
    $core.String? etag,
    $0.Timestamp? publishTime,
  }) {
    final result = SchedulePostRequest._();
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    if (publishTime != null) result.publishTime = publishTime;
    return result;
  }

  SchedulePostRequest._();

  factory SchedulePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostRequest()..mergeFromBuffer(data, registry);
  factory SchedulePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SchedulePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SchedulePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..aOM<$0.Timestamp>(3, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostRequest copyWith(void Function(SchedulePostRequest) updates) =>
      super.copyWith((message) => updates(message as SchedulePostRequest))
          as SchedulePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SchedulePostRequest() / SchedulePostRequest.new instead')
  static SchedulePostRequest create() => SchedulePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => SchedulePostRequest._();
  @$core.override
  SchedulePostRequest createEmptyInstance() => SchedulePostRequest._();
  @$core.pragma('dart2js:noInline')
  static SchedulePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SchedulePostRequest>(
          SchedulePostRequest.$_createMessage);
  static SchedulePostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);

  /// 發布的時間點，距現在 5 分鐘到 90 天。
  @$pb.TagNumber(3)
  $0.Timestamp get publishTime => $_getN(2);
  @$pb.TagNumber(3)
  set publishTime($0.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPublishTime() => $_has(2);
  @$pb.TagNumber(3)
  void clearPublishTime() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.Timestamp ensurePublishTime() => $_ensure(2);
}

class SchedulePostResponse extends $pb.GeneratedMessage {
  factory SchedulePostResponse({
    Post? post,
    $0.Timestamp? publishTime,
  }) {
    final result = SchedulePostResponse._();
    if (post != null) result.post = post;
    if (publishTime != null) result.publishTime = publishTime;
    return result;
  }

  SchedulePostResponse._();

  factory SchedulePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostResponse()..mergeFromBuffer(data, registry);
  factory SchedulePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SchedulePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SchedulePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: SchedulePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SchedulePostResponse copyWith(void Function(SchedulePostResponse) updates) =>
      super.copyWith((message) => updates(message as SchedulePostResponse))
          as SchedulePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SchedulePostResponse() / SchedulePostResponse.new instead')
  static SchedulePostResponse create() => SchedulePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => SchedulePostResponse._();
  @$core.override
  SchedulePostResponse createEmptyInstance() => SchedulePostResponse._();
  @$core.pragma('dart2js:noInline')
  static SchedulePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SchedulePostResponse>(
          SchedulePostResponse.$_createMessage);
  static SchedulePostResponse? _defaultInstance;

  /// 排程後的貼文（含新的 etag）。
  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  /// 排定的發布時間。
  @$pb.TagNumber(2)
  $0.Timestamp get publishTime => $_getN(1);
  @$pb.TagNumber(2)
  set publishTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPublishTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublishTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensurePublishTime() => $_ensure(1);
}

class ListPendingSchedulesRequest extends $pb.GeneratedMessage {
  factory ListPendingSchedulesRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListPendingSchedulesRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListPendingSchedulesRequest._();

  factory ListPendingSchedulesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPendingSchedulesRequest()..mergeFromBuffer(data, registry);
  factory ListPendingSchedulesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPendingSchedulesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListPendingSchedulesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListPendingSchedulesRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPendingSchedulesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPendingSchedulesRequest copyWith(
          void Function(ListPendingSchedulesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListPendingSchedulesRequest))
          as ListPendingSchedulesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListPendingSchedulesRequest() / ListPendingSchedulesRequest.new instead')
  static ListPendingSchedulesRequest create() =>
      ListPendingSchedulesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListPendingSchedulesRequest._();
  @$core.override
  ListPendingSchedulesRequest createEmptyInstance() =>
      ListPendingSchedulesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListPendingSchedulesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListPendingSchedulesRequest>(
          ListPendingSchedulesRequest.$_createMessage);
  static ListPendingSchedulesRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListPendingSchedulesResponse extends $pb.GeneratedMessage {
  factory ListPendingSchedulesResponse({
    $core.Iterable<PostSchedule>? schedules,
    $core.String? nextPageToken,
  }) {
    final result = ListPendingSchedulesResponse._();
    if (schedules != null) result.schedules.addAll(schedules);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListPendingSchedulesResponse._();

  factory ListPendingSchedulesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPendingSchedulesResponse()..mergeFromBuffer(data, registry);
  factory ListPendingSchedulesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListPendingSchedulesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListPendingSchedulesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListPendingSchedulesResponse.$_createMessage)
    ..pPM<PostSchedule>(1, _omitFieldNames ? '' : 'schedules',
        subBuilder: PostSchedule.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPendingSchedulesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListPendingSchedulesResponse copyWith(
          void Function(ListPendingSchedulesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListPendingSchedulesResponse))
          as ListPendingSchedulesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListPendingSchedulesResponse() / ListPendingSchedulesResponse.new instead')
  static ListPendingSchedulesResponse create() =>
      ListPendingSchedulesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListPendingSchedulesResponse._();
  @$core.override
  ListPendingSchedulesResponse createEmptyInstance() =>
      ListPendingSchedulesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListPendingSchedulesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListPendingSchedulesResponse>(
          ListPendingSchedulesResponse.$_createMessage);
  static ListPendingSchedulesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PostSchedule> get schedules => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class CancelPostScheduleRequest extends $pb.GeneratedMessage {
  factory CancelPostScheduleRequest({
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = CancelPostScheduleRequest._();
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  CancelPostScheduleRequest._();

  factory CancelPostScheduleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelPostScheduleRequest()..mergeFromBuffer(data, registry);
  factory CancelPostScheduleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelPostScheduleRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelPostScheduleRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CancelPostScheduleRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..aOS(2, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelPostScheduleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelPostScheduleRequest copyWith(
          void Function(CancelPostScheduleRequest) updates) =>
      super.copyWith((message) => updates(message as CancelPostScheduleRequest))
          as CancelPostScheduleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CancelPostScheduleRequest() / CancelPostScheduleRequest.new instead')
  static CancelPostScheduleRequest create() => CancelPostScheduleRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      CancelPostScheduleRequest._();
  @$core.override
  CancelPostScheduleRequest createEmptyInstance() =>
      CancelPostScheduleRequest._();
  @$core.pragma('dart2js:noInline')
  static CancelPostScheduleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelPostScheduleRequest>(
          CancelPostScheduleRequest.$_createMessage);
  static CancelPostScheduleRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(2)
  $core.String get etag => $_getSZ(1);
  @$pb.TagNumber(2)
  set etag($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEtag() => $_has(1);
  @$pb.TagNumber(2)
  void clearEtag() => $_clearField(2);
}

class CancelPostScheduleResponse extends $pb.GeneratedMessage {
  factory CancelPostScheduleResponse({
    $core.String? etag,
  }) {
    final result = CancelPostScheduleResponse._();
    if (etag != null) result.etag = etag;
    return result;
  }

  CancelPostScheduleResponse._();

  factory CancelPostScheduleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelPostScheduleResponse()..mergeFromBuffer(data, registry);
  factory CancelPostScheduleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelPostScheduleResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelPostScheduleResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CancelPostScheduleResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelPostScheduleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelPostScheduleResponse copyWith(
          void Function(CancelPostScheduleResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CancelPostScheduleResponse))
          as CancelPostScheduleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CancelPostScheduleResponse() / CancelPostScheduleResponse.new instead')
  static CancelPostScheduleResponse create() => CancelPostScheduleResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      CancelPostScheduleResponse._();
  @$core.override
  CancelPostScheduleResponse createEmptyInstance() =>
      CancelPostScheduleResponse._();
  @$core.pragma('dart2js:noInline')
  static CancelPostScheduleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelPostScheduleResponse>(
          CancelPostScheduleResponse.$_createMessage);
  static CancelPostScheduleResponse? _defaultInstance;

  /// 取消後貼文的 etag。
  @$pb.TagNumber(1)
  $core.String get etag => $_getSZ(0);
  @$pb.TagNumber(1)
  set etag($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEtag() => $_has(0);
  @$pb.TagNumber(1)
  void clearEtag() => $_clearField(1);
}

class ListStoreCommentsRequest extends $pb.GeneratedMessage {
  factory ListStoreCommentsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $core.String? postId,
    $core.bool? unreplied,
  }) {
    final result = ListStoreCommentsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (postId != null) result.postId = postId;
    if (unreplied != null) result.unreplied = unreplied;
    return result;
  }

  ListStoreCommentsRequest._();

  factory ListStoreCommentsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreCommentsRequest()..mergeFromBuffer(data, registry);
  factory ListStoreCommentsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreCommentsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreCommentsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListStoreCommentsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aOS(3, _omitFieldNames ? '' : 'postId')
    ..aOB(4, _omitFieldNames ? '' : 'unreplied')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreCommentsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreCommentsRequest copyWith(
          void Function(ListStoreCommentsRequest) updates) =>
      super.copyWith((message) => updates(message as ListStoreCommentsRequest))
          as ListStoreCommentsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreCommentsRequest() / ListStoreCommentsRequest.new instead')
  static ListStoreCommentsRequest create() => ListStoreCommentsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListStoreCommentsRequest._();
  @$core.override
  ListStoreCommentsRequest createEmptyInstance() =>
      ListStoreCommentsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListStoreCommentsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreCommentsRequest>(
          ListStoreCommentsRequest.$_createMessage);
  static ListStoreCommentsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);

  /// 設了只列這一篇（必須是本店的貼文，否則 not_found）；不設是本店所有貼文。
  @$pb.TagNumber(3)
  $core.String get postId => $_getSZ(2);
  @$pb.TagNumber(3)
  set postId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPostId() => $_has(2);
  @$pb.TagNumber(3)
  void clearPostId() => $_clearField(3);

  /// 為真只列未回覆的留言（定義見 ListStoreComments）。
  @$pb.TagNumber(4)
  $core.bool get unreplied => $_getBF(3);
  @$pb.TagNumber(4)
  set unreplied($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUnreplied() => $_has(3);
  @$pb.TagNumber(4)
  void clearUnreplied() => $_clearField(4);
}

class ListStoreCommentsResponse extends $pb.GeneratedMessage {
  factory ListStoreCommentsResponse({
    $core.Iterable<Comment>? comments,
    $core.String? nextPageToken,
  }) {
    final result = ListStoreCommentsResponse._();
    if (comments != null) result.comments.addAll(comments);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListStoreCommentsResponse._();

  factory ListStoreCommentsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreCommentsResponse()..mergeFromBuffer(data, registry);
  factory ListStoreCommentsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreCommentsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreCommentsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListStoreCommentsResponse.$_createMessage)
    ..pPM<Comment>(1, _omitFieldNames ? '' : 'comments',
        subBuilder: Comment.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreCommentsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreCommentsResponse copyWith(
          void Function(ListStoreCommentsResponse) updates) =>
      super.copyWith((message) => updates(message as ListStoreCommentsResponse))
          as ListStoreCommentsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreCommentsResponse() / ListStoreCommentsResponse.new instead')
  static ListStoreCommentsResponse create() => ListStoreCommentsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListStoreCommentsResponse._();
  @$core.override
  ListStoreCommentsResponse createEmptyInstance() =>
      ListStoreCommentsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListStoreCommentsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreCommentsResponse>(
          ListStoreCommentsResponse.$_createMessage);
  static ListStoreCommentsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Comment> get comments => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetStoreSocialSummaryRequest extends $pb.GeneratedMessage {
  factory GetStoreSocialSummaryRequest() => GetStoreSocialSummaryRequest._();

  GetStoreSocialSummaryRequest._();

  factory GetStoreSocialSummaryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreSocialSummaryRequest()..mergeFromBuffer(data, registry);
  factory GetStoreSocialSummaryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreSocialSummaryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreSocialSummaryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetStoreSocialSummaryRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreSocialSummaryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreSocialSummaryRequest copyWith(
          void Function(GetStoreSocialSummaryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetStoreSocialSummaryRequest))
          as GetStoreSocialSummaryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreSocialSummaryRequest() / GetStoreSocialSummaryRequest.new instead')
  static GetStoreSocialSummaryRequest create() =>
      GetStoreSocialSummaryRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetStoreSocialSummaryRequest._();
  @$core.override
  GetStoreSocialSummaryRequest createEmptyInstance() =>
      GetStoreSocialSummaryRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStoreSocialSummaryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreSocialSummaryRequest>(
          GetStoreSocialSummaryRequest.$_createMessage);
  static GetStoreSocialSummaryRequest? _defaultInstance;
}

class GetStoreSocialSummaryResponse extends $pb.GeneratedMessage {
  factory GetStoreSocialSummaryResponse({
    $core.Iterable<PostStatusCount>? postCounts,
    $fixnum.Int64? scheduledPostCount,
  }) {
    final result = GetStoreSocialSummaryResponse._();
    if (postCounts != null) result.postCounts.addAll(postCounts);
    if (scheduledPostCount != null)
      result.scheduledPostCount = scheduledPostCount;
    return result;
  }

  GetStoreSocialSummaryResponse._();

  factory GetStoreSocialSummaryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreSocialSummaryResponse()..mergeFromBuffer(data, registry);
  factory GetStoreSocialSummaryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreSocialSummaryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreSocialSummaryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetStoreSocialSummaryResponse.$_createMessage)
    ..pPM<PostStatusCount>(1, _omitFieldNames ? '' : 'postCounts',
        subBuilder: PostStatusCount.$_createMessage)
    ..aInt64(2, _omitFieldNames ? '' : 'scheduledPostCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreSocialSummaryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreSocialSummaryResponse copyWith(
          void Function(GetStoreSocialSummaryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetStoreSocialSummaryResponse))
          as GetStoreSocialSummaryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreSocialSummaryResponse() / GetStoreSocialSummaryResponse.new instead')
  static GetStoreSocialSummaryResponse create() =>
      GetStoreSocialSummaryResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetStoreSocialSummaryResponse._();
  @$core.override
  GetStoreSocialSummaryResponse createEmptyInstance() =>
      GetStoreSocialSummaryResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStoreSocialSummaryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreSocialSummaryResponse>(
          GetStoreSocialSummaryResponse.$_createMessage);
  static GetStoreSocialSummaryResponse? _defaultInstance;

  /// 每個貼文狀態一項（DRAFT、PUBLISHED、UNPUBLISHED 都會有，沒有貼文時是 0），不含已刪除的貼文。
  @$pb.TagNumber(1)
  $pb.PbList<PostStatusCount> get postCounts => $_getList(0);

  /// 有待發布排程的貼文篇數；它們的狀態是 DRAFT 或 UNPUBLISHED，所以也算在 post_counts 裡。
  @$pb.TagNumber(2)
  $fixnum.Int64 get scheduledPostCount => $_getI64(1);
  @$pb.TagNumber(2)
  set scheduledPostCount($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasScheduledPostCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearScheduledPostCount() => $_clearField(2);
}

/// PostStatusCount 是一個貼文狀態與店家在這個狀態的貼文篇數。
class PostStatusCount extends $pb.GeneratedMessage {
  factory PostStatusCount({
    PostStatus? status,
    $fixnum.Int64? count,
  }) {
    final result = PostStatusCount._();
    if (status != null) result.status = status;
    if (count != null) result.count = count;
    return result;
  }

  PostStatusCount._();

  factory PostStatusCount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PostStatusCount()..mergeFromBuffer(data, registry);
  factory PostStatusCount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PostStatusCount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PostStatusCount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: PostStatusCount.$_createMessage)
    ..aE<PostStatus>(1, _omitFieldNames ? '' : 'status',
        enumValues: PostStatus.values)
    ..aInt64(2, _omitFieldNames ? '' : 'count')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PostStatusCount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PostStatusCount copyWith(void Function(PostStatusCount) updates) =>
      super.copyWith((message) => updates(message as PostStatusCount))
          as PostStatusCount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PostStatusCount() / PostStatusCount.new instead')
  static PostStatusCount create() => PostStatusCount._();
  static $pb.GeneratedMessage $_createMessage() => PostStatusCount._();
  @$core.override
  PostStatusCount createEmptyInstance() => PostStatusCount._();
  @$core.pragma('dart2js:noInline')
  static PostStatusCount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PostStatusCount>(
          PostStatusCount.$_createMessage);
  static PostStatusCount? _defaultInstance;

  @$pb.TagNumber(1)
  PostStatus get status => $_getN(0);
  @$pb.TagNumber(1)
  set status(PostStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get count => $_getI64(1);
  @$pb.TagNumber(2)
  set count($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);
}

/// DelegatedOperation 是每個代行寫入都要帶的稽核資料。
class DelegatedOperation extends $pb.GeneratedMessage {
  factory DelegatedOperation({
    $core.String? requestId,
    $core.String? operationReason,
    $core.bool? notifySeller,
  }) {
    final result = DelegatedOperation._();
    if (requestId != null) result.requestId = requestId;
    if (operationReason != null) result.operationReason = operationReason;
    if (notifySeller != null) result.notifySeller = notifySeller;
    return result;
  }

  DelegatedOperation._();

  factory DelegatedOperation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegatedOperation()..mergeFromBuffer(data, registry);
  factory DelegatedOperation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegatedOperation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DelegatedOperation',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: DelegatedOperation.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'operationReason')
    ..aOB(3, _omitFieldNames ? '' : 'notifySeller')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegatedOperation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegatedOperation copyWith(void Function(DelegatedOperation) updates) =>
      super.copyWith((message) => updates(message as DelegatedOperation))
          as DelegatedOperation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DelegatedOperation() / DelegatedOperation.new instead')
  static DelegatedOperation create() => DelegatedOperation._();
  static $pb.GeneratedMessage $_createMessage() => DelegatedOperation._();
  @$core.override
  DelegatedOperation createEmptyInstance() => DelegatedOperation._();
  @$core.pragma('dart2js:noInline')
  static DelegatedOperation getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DelegatedOperation>(
          DelegatedOperation.$_createMessage);
  static DelegatedOperation? _defaultInstance;

  /// UUIDv4（AIP-155）：同一個代行 session 內以同一個 request_id 重送，回第一次的結果，不重做寫入也不再寫稽核；同一個 request_id 用在不同的請求內容回 failed_precondition。必填。
  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 操作原因，去除首尾空白後 1 到 100 個字元，不得含控制字元；寫進稽核，必填。
  @$pb.TagNumber(2)
  $core.String get operationReason => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationReason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationReason() => $_clearField(2);

  /// 是否在完成後通知店主；不設是通知。
  @$pb.TagNumber(3)
  $core.bool get notifySeller => $_getBF(2);
  @$pb.TagNumber(3)
  set notifySeller($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNotifySeller() => $_has(2);
  @$pb.TagNumber(3)
  void clearNotifySeller() => $_clearField(3);
}

class ListStorePostsRequest extends $pb.GeneratedMessage {
  factory ListStorePostsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    PostStatus? status,
  }) {
    final result = ListStorePostsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (status != null) result.status = status;
    return result;
  }

  ListStorePostsRequest._();

  factory ListStorePostsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStorePostsRequest()..mergeFromBuffer(data, registry);
  factory ListStorePostsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStorePostsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStorePostsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListStorePostsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aE<PostStatus>(3, _omitFieldNames ? '' : 'status',
        enumValues: PostStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStorePostsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStorePostsRequest copyWith(
          void Function(ListStorePostsRequest) updates) =>
      super.copyWith((message) => updates(message as ListStorePostsRequest))
          as ListStorePostsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStorePostsRequest() / ListStorePostsRequest.new instead')
  static ListStorePostsRequest create() => ListStorePostsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListStorePostsRequest._();
  @$core.override
  ListStorePostsRequest createEmptyInstance() => ListStorePostsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListStorePostsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStorePostsRequest>(
          ListStorePostsRequest.$_createMessage);
  static ListStorePostsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);

  /// 只列這個狀態的貼文；不設（UNSPECIFIED）是全部。page_token 只能用在同一個 status。
  @$pb.TagNumber(3)
  PostStatus get status => $_getN(2);
  @$pb.TagNumber(3)
  set status(PostStatus value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);
}

class ListStorePostsResponse extends $pb.GeneratedMessage {
  factory ListStorePostsResponse({
    $core.Iterable<Post>? posts,
    $core.String? nextPageToken,
  }) {
    final result = ListStorePostsResponse._();
    if (posts != null) result.posts.addAll(posts);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListStorePostsResponse._();

  factory ListStorePostsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStorePostsResponse()..mergeFromBuffer(data, registry);
  factory ListStorePostsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStorePostsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStorePostsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListStorePostsResponse.$_createMessage)
    ..pPM<Post>(1, _omitFieldNames ? '' : 'posts',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStorePostsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStorePostsResponse copyWith(
          void Function(ListStorePostsResponse) updates) =>
      super.copyWith((message) => updates(message as ListStorePostsResponse))
          as ListStorePostsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStorePostsResponse() / ListStorePostsResponse.new instead')
  static ListStorePostsResponse create() => ListStorePostsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListStorePostsResponse._();
  @$core.override
  ListStorePostsResponse createEmptyInstance() => ListStorePostsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListStorePostsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStorePostsResponse>(
          ListStorePostsResponse.$_createMessage);
  static ListStorePostsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Post> get posts => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetStorePostRequest extends $pb.GeneratedMessage {
  factory GetStorePostRequest({
    $core.String? postId,
  }) {
    final result = GetStorePostRequest._();
    if (postId != null) result.postId = postId;
    return result;
  }

  GetStorePostRequest._();

  factory GetStorePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePostRequest()..mergeFromBuffer(data, registry);
  factory GetStorePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStorePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetStorePostRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'postId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePostRequest copyWith(void Function(GetStorePostRequest) updates) =>
      super.copyWith((message) => updates(message as GetStorePostRequest))
          as GetStorePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetStorePostRequest() / GetStorePostRequest.new instead')
  static GetStorePostRequest create() => GetStorePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetStorePostRequest._();
  @$core.override
  GetStorePostRequest createEmptyInstance() => GetStorePostRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStorePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStorePostRequest>(
          GetStorePostRequest.$_createMessage);
  static GetStorePostRequest? _defaultInstance;

  /// 貼文 ID。
  @$pb.TagNumber(1)
  $core.String get postId => $_getSZ(0);
  @$pb.TagNumber(1)
  set postId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearPostId() => $_clearField(1);
}

class GetStorePostResponse extends $pb.GeneratedMessage {
  factory GetStorePostResponse({
    Post? post,
  }) {
    final result = GetStorePostResponse._();
    if (post != null) result.post = post;
    return result;
  }

  GetStorePostResponse._();

  factory GetStorePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePostResponse()..mergeFromBuffer(data, registry);
  factory GetStorePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStorePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: GetStorePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePostResponse copyWith(void Function(GetStorePostResponse) updates) =>
      super.copyWith((message) => updates(message as GetStorePostResponse))
          as GetStorePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStorePostResponse() / GetStorePostResponse.new instead')
  static GetStorePostResponse create() => GetStorePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetStorePostResponse._();
  @$core.override
  GetStorePostResponse createEmptyInstance() => GetStorePostResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStorePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStorePostResponse>(
          GetStorePostResponse.$_createMessage);
  static GetStorePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);
}

class CreateStorePostRequest extends $pb.GeneratedMessage {
  factory CreateStorePostRequest({
    DelegatedOperation? operation,
    $core.String? body,
    $core.String? imageStagedToken,
    $core.String? productId,
    $core.Iterable<$core.String>? hashtagInputs,
  }) {
    final result = CreateStorePostRequest._();
    if (operation != null) result.operation = operation;
    if (body != null) result.body = body;
    if (imageStagedToken != null) result.imageStagedToken = imageStagedToken;
    if (productId != null) result.productId = productId;
    if (hashtagInputs != null) result.hashtagInputs.addAll(hashtagInputs);
    return result;
  }

  CreateStorePostRequest._();

  factory CreateStorePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStorePostRequest()..mergeFromBuffer(data, registry);
  factory CreateStorePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStorePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateStorePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreateStorePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'body')
    ..aOS(3, _omitFieldNames ? '' : 'imageStagedToken')
    ..aOS(4, _omitFieldNames ? '' : 'productId')
    ..pPS(5, _omitFieldNames ? '' : 'hashtagInputs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStorePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStorePostRequest copyWith(
          void Function(CreateStorePostRequest) updates) =>
      super.copyWith((message) => updates(message as CreateStorePostRequest))
          as CreateStorePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateStorePostRequest() / CreateStorePostRequest.new instead')
  static CreateStorePostRequest create() => CreateStorePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateStorePostRequest._();
  @$core.override
  CreateStorePostRequest createEmptyInstance() => CreateStorePostRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateStorePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateStorePostRequest>(
          CreateStorePostRequest.$_createMessage);
  static CreateStorePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 內文，去除首尾空白後最多 2200 個字元；草稿可以是空的。
  @$pb.TagNumber(2)
  $core.String get body => $_getSZ(1);
  @$pb.TagNumber(2)
  set body($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBody() => $_has(1);
  @$pb.TagNumber(2)
  void clearBody() => $_clearField(2);

  /// 圖片的 staged token（media 上傳完成後拿到的）。
  @$pb.TagNumber(3)
  $core.String get imageStagedToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set imageStagedToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasImageStagedToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearImageStagedToken() => $_clearField(3);

  /// 貼文帶的商品 ID；商品要存在、上架中而且屬於這家店。
  @$pb.TagNumber(4)
  $core.String get productId => $_getSZ(3);
  @$pb.TagNumber(4)
  set productId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProductId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProductId() => $_clearField(4);

  /// 標籤（使用者輸入的樣子，可帶開頭的 #）。
  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get hashtagInputs => $_getList(4);
}

class CreateStorePostResponse extends $pb.GeneratedMessage {
  factory CreateStorePostResponse({
    Post? post,
    $core.String? auditLogId,
  }) {
    final result = CreateStorePostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  CreateStorePostResponse._();

  factory CreateStorePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStorePostResponse()..mergeFromBuffer(data, registry);
  factory CreateStorePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateStorePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateStorePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: CreateStorePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStorePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStorePostResponse copyWith(
          void Function(CreateStorePostResponse) updates) =>
      super.copyWith((message) => updates(message as CreateStorePostResponse))
          as CreateStorePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateStorePostResponse() / CreateStorePostResponse.new instead')
  static CreateStorePostResponse create() => CreateStorePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateStorePostResponse._();
  @$core.override
  CreateStorePostResponse createEmptyInstance() => CreateStorePostResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateStorePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateStorePostResponse>(
          CreateStorePostResponse.$_createMessage);
  static CreateStorePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  /// 這次寫入的稽核列 ID（重送時是第一次的）。
  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class UpdateStorePostRequest extends $pb.GeneratedMessage {
  factory UpdateStorePostRequest({
    DelegatedOperation? operation,
    Post? post,
    $3.FieldMask? updateMask,
  }) {
    final result = UpdateStorePostRequest._();
    if (operation != null) result.operation = operation;
    if (post != null) result.post = post;
    if (updateMask != null) result.updateMask = updateMask;
    return result;
  }

  UpdateStorePostRequest._();

  factory UpdateStorePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStorePostRequest()..mergeFromBuffer(data, registry);
  factory UpdateStorePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStorePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateStorePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdateStorePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOM<Post>(2, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOM<$3.FieldMask>(3, _omitFieldNames ? '' : 'updateMask',
        subBuilder: $3.FieldMask.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStorePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStorePostRequest copyWith(
          void Function(UpdateStorePostRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateStorePostRequest))
          as UpdateStorePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateStorePostRequest() / UpdateStorePostRequest.new instead')
  static UpdateStorePostRequest create() => UpdateStorePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => UpdateStorePostRequest._();
  @$core.override
  UpdateStorePostRequest createEmptyInstance() => UpdateStorePostRequest._();
  @$core.pragma('dart2js:noInline')
  static UpdateStorePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateStorePostRequest>(
          UpdateStorePostRequest.$_createMessage);
  static UpdateStorePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// id 與 etag 必填。可以修改的欄位是 body、image_staged_token、product_id、hashtag_inputs，update_mask 要列至少一個；沒列的欄位保留目前的值。
  @$pb.TagNumber(2)
  Post get post => $_getN(1);
  @$pb.TagNumber(2)
  set post(Post value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPost() => $_has(1);
  @$pb.TagNumber(2)
  void clearPost() => $_clearField(2);
  @$pb.TagNumber(2)
  Post ensurePost() => $_ensure(1);

  @$pb.TagNumber(3)
  $3.FieldMask get updateMask => $_getN(2);
  @$pb.TagNumber(3)
  set updateMask($3.FieldMask value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasUpdateMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearUpdateMask() => $_clearField(3);
  @$pb.TagNumber(3)
  $3.FieldMask ensureUpdateMask() => $_ensure(2);
}

class UpdateStorePostResponse extends $pb.GeneratedMessage {
  factory UpdateStorePostResponse({
    Post? post,
    $core.String? auditLogId,
  }) {
    final result = UpdateStorePostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  UpdateStorePostResponse._();

  factory UpdateStorePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStorePostResponse()..mergeFromBuffer(data, registry);
  factory UpdateStorePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UpdateStorePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateStorePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UpdateStorePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStorePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateStorePostResponse copyWith(
          void Function(UpdateStorePostResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateStorePostResponse))
          as UpdateStorePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UpdateStorePostResponse() / UpdateStorePostResponse.new instead')
  static UpdateStorePostResponse create() => UpdateStorePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => UpdateStorePostResponse._();
  @$core.override
  UpdateStorePostResponse createEmptyInstance() => UpdateStorePostResponse._();
  @$core.pragma('dart2js:noInline')
  static UpdateStorePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateStorePostResponse>(
          UpdateStorePostResponse.$_createMessage);
  static UpdateStorePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class PublishStorePostRequest extends $pb.GeneratedMessage {
  factory PublishStorePostRequest({
    DelegatedOperation? operation,
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = PublishStorePostRequest._();
    if (operation != null) result.operation = operation;
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  PublishStorePostRequest._();

  factory PublishStorePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStorePostRequest()..mergeFromBuffer(data, registry);
  factory PublishStorePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStorePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishStorePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: PublishStorePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStorePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStorePostRequest copyWith(
          void Function(PublishStorePostRequest) updates) =>
      super.copyWith((message) => updates(message as PublishStorePostRequest))
          as PublishStorePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PublishStorePostRequest() / PublishStorePostRequest.new instead')
  static PublishStorePostRequest create() => PublishStorePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => PublishStorePostRequest._();
  @$core.override
  PublishStorePostRequest createEmptyInstance() => PublishStorePostRequest._();
  @$core.pragma('dart2js:noInline')
  static PublishStorePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishStorePostRequest>(
          PublishStorePostRequest.$_createMessage);
  static PublishStorePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 貼文 ID。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class PublishStorePostResponse extends $pb.GeneratedMessage {
  factory PublishStorePostResponse({
    Post? post,
    $core.String? auditLogId,
  }) {
    final result = PublishStorePostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  PublishStorePostResponse._();

  factory PublishStorePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStorePostResponse()..mergeFromBuffer(data, registry);
  factory PublishStorePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PublishStorePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PublishStorePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: PublishStorePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStorePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PublishStorePostResponse copyWith(
          void Function(PublishStorePostResponse) updates) =>
      super.copyWith((message) => updates(message as PublishStorePostResponse))
          as PublishStorePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use PublishStorePostResponse() / PublishStorePostResponse.new instead')
  static PublishStorePostResponse create() => PublishStorePostResponse._();
  static $pb.GeneratedMessage $_createMessage() => PublishStorePostResponse._();
  @$core.override
  PublishStorePostResponse createEmptyInstance() =>
      PublishStorePostResponse._();
  @$core.pragma('dart2js:noInline')
  static PublishStorePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PublishStorePostResponse>(
          PublishStorePostResponse.$_createMessage);
  static PublishStorePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class UnpublishStorePostRequest extends $pb.GeneratedMessage {
  factory UnpublishStorePostRequest({
    DelegatedOperation? operation,
    $core.String? postId,
    $core.String? etag,
  }) {
    final result = UnpublishStorePostRequest._();
    if (operation != null) result.operation = operation;
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    return result;
  }

  UnpublishStorePostRequest._();

  factory UnpublishStorePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStorePostRequest()..mergeFromBuffer(data, registry);
  factory UnpublishStorePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStorePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishStorePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnpublishStorePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStorePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStorePostRequest copyWith(
          void Function(UnpublishStorePostRequest) updates) =>
      super.copyWith((message) => updates(message as UnpublishStorePostRequest))
          as UnpublishStorePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishStorePostRequest() / UnpublishStorePostRequest.new instead')
  static UnpublishStorePostRequest create() => UnpublishStorePostRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnpublishStorePostRequest._();
  @$core.override
  UnpublishStorePostRequest createEmptyInstance() =>
      UnpublishStorePostRequest._();
  @$core.pragma('dart2js:noInline')
  static UnpublishStorePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishStorePostRequest>(
          UnpublishStorePostRequest.$_createMessage);
  static UnpublishStorePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 貼文 ID。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);
}

class UnpublishStorePostResponse extends $pb.GeneratedMessage {
  factory UnpublishStorePostResponse({
    Post? post,
    $core.String? auditLogId,
  }) {
    final result = UnpublishStorePostResponse._();
    if (post != null) result.post = post;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  UnpublishStorePostResponse._();

  factory UnpublishStorePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStorePostResponse()..mergeFromBuffer(data, registry);
  factory UnpublishStorePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnpublishStorePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnpublishStorePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: UnpublishStorePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStorePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnpublishStorePostResponse copyWith(
          void Function(UnpublishStorePostResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UnpublishStorePostResponse))
          as UnpublishStorePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnpublishStorePostResponse() / UnpublishStorePostResponse.new instead')
  static UnpublishStorePostResponse create() => UnpublishStorePostResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnpublishStorePostResponse._();
  @$core.override
  UnpublishStorePostResponse createEmptyInstance() =>
      UnpublishStorePostResponse._();
  @$core.pragma('dart2js:noInline')
  static UnpublishStorePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnpublishStorePostResponse>(
          UnpublishStorePostResponse.$_createMessage);
  static UnpublishStorePostResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get auditLogId => $_getSZ(1);
  @$pb.TagNumber(2)
  set auditLogId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuditLogId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuditLogId() => $_clearField(2);
}

class ScheduleStorePostRequest extends $pb.GeneratedMessage {
  factory ScheduleStorePostRequest({
    DelegatedOperation? operation,
    $core.String? postId,
    $core.String? etag,
    $0.Timestamp? publishTime,
  }) {
    final result = ScheduleStorePostRequest._();
    if (operation != null) result.operation = operation;
    if (postId != null) result.postId = postId;
    if (etag != null) result.etag = etag;
    if (publishTime != null) result.publishTime = publishTime;
    return result;
  }

  ScheduleStorePostRequest._();

  factory ScheduleStorePostRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ScheduleStorePostRequest()..mergeFromBuffer(data, registry);
  factory ScheduleStorePostRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ScheduleStorePostRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ScheduleStorePostRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ScheduleStorePostRequest.$_createMessage)
    ..aOM<DelegatedOperation>(1, _omitFieldNames ? '' : 'operation',
        subBuilder: DelegatedOperation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'postId')
    ..aOS(3, _omitFieldNames ? '' : 'etag')
    ..aOM<$0.Timestamp>(4, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScheduleStorePostRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScheduleStorePostRequest copyWith(
          void Function(ScheduleStorePostRequest) updates) =>
      super.copyWith((message) => updates(message as ScheduleStorePostRequest))
          as ScheduleStorePostRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ScheduleStorePostRequest() / ScheduleStorePostRequest.new instead')
  static ScheduleStorePostRequest create() => ScheduleStorePostRequest._();
  static $pb.GeneratedMessage $_createMessage() => ScheduleStorePostRequest._();
  @$core.override
  ScheduleStorePostRequest createEmptyInstance() =>
      ScheduleStorePostRequest._();
  @$core.pragma('dart2js:noInline')
  static ScheduleStorePostRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ScheduleStorePostRequest>(
          ScheduleStorePostRequest.$_createMessage);
  static ScheduleStorePostRequest? _defaultInstance;

  @$pb.TagNumber(1)
  DelegatedOperation get operation => $_getN(0);
  @$pb.TagNumber(1)
  set operation(DelegatedOperation value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOperation() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperation() => $_clearField(1);
  @$pb.TagNumber(1)
  DelegatedOperation ensureOperation() => $_ensure(0);

  /// 貼文 ID。
  @$pb.TagNumber(2)
  $core.String get postId => $_getSZ(1);
  @$pb.TagNumber(2)
  set postId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearPostId() => $_clearField(2);

  /// 讀取時拿到的 etag，必填。
  @$pb.TagNumber(3)
  $core.String get etag => $_getSZ(2);
  @$pb.TagNumber(3)
  set etag($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEtag() => $_has(2);
  @$pb.TagNumber(3)
  void clearEtag() => $_clearField(3);

  /// 發布的時間點，距現在 5 分鐘到 90 天。
  @$pb.TagNumber(4)
  $0.Timestamp get publishTime => $_getN(3);
  @$pb.TagNumber(4)
  set publishTime($0.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasPublishTime() => $_has(3);
  @$pb.TagNumber(4)
  void clearPublishTime() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.Timestamp ensurePublishTime() => $_ensure(3);
}

class ScheduleStorePostResponse extends $pb.GeneratedMessage {
  factory ScheduleStorePostResponse({
    Post? post,
    $0.Timestamp? publishTime,
    $core.String? auditLogId,
  }) {
    final result = ScheduleStorePostResponse._();
    if (post != null) result.post = post;
    if (publishTime != null) result.publishTime = publishTime;
    if (auditLogId != null) result.auditLogId = auditLogId;
    return result;
  }

  ScheduleStorePostResponse._();

  factory ScheduleStorePostResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ScheduleStorePostResponse()..mergeFromBuffer(data, registry);
  factory ScheduleStorePostResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ScheduleStorePostResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ScheduleStorePostResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ScheduleStorePostResponse.$_createMessage)
    ..aOM<Post>(1, _omitFieldNames ? '' : 'post',
        subBuilder: Post.$_createMessage)
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'publishTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'auditLogId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScheduleStorePostResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScheduleStorePostResponse copyWith(
          void Function(ScheduleStorePostResponse) updates) =>
      super.copyWith((message) => updates(message as ScheduleStorePostResponse))
          as ScheduleStorePostResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ScheduleStorePostResponse() / ScheduleStorePostResponse.new instead')
  static ScheduleStorePostResponse create() => ScheduleStorePostResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ScheduleStorePostResponse._();
  @$core.override
  ScheduleStorePostResponse createEmptyInstance() =>
      ScheduleStorePostResponse._();
  @$core.pragma('dart2js:noInline')
  static ScheduleStorePostResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ScheduleStorePostResponse>(
          ScheduleStorePostResponse.$_createMessage);
  static ScheduleStorePostResponse? _defaultInstance;

  /// 排程後的貼文（含新的 etag）。
  @$pb.TagNumber(1)
  Post get post => $_getN(0);
  @$pb.TagNumber(1)
  set post(Post value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPost() => $_has(0);
  @$pb.TagNumber(1)
  void clearPost() => $_clearField(1);
  @$pb.TagNumber(1)
  Post ensurePost() => $_ensure(0);

  /// 排定的發布時間。
  @$pb.TagNumber(2)
  $0.Timestamp get publishTime => $_getN(1);
  @$pb.TagNumber(2)
  set publishTime($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPublishTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublishTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensurePublishTime() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get auditLogId => $_getSZ(2);
  @$pb.TagNumber(3)
  set auditLogId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAuditLogId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAuditLogId() => $_clearField(3);
}

class ListMySanctionsRequest extends $pb.GeneratedMessage {
  factory ListMySanctionsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMySanctionsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMySanctionsRequest._();

  factory ListMySanctionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMySanctionsRequest()..mergeFromBuffer(data, registry);
  factory ListMySanctionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMySanctionsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMySanctionsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListMySanctionsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySanctionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySanctionsRequest copyWith(
          void Function(ListMySanctionsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMySanctionsRequest))
          as ListMySanctionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMySanctionsRequest() / ListMySanctionsRequest.new instead')
  static ListMySanctionsRequest create() => ListMySanctionsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMySanctionsRequest._();
  @$core.override
  ListMySanctionsRequest createEmptyInstance() => ListMySanctionsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMySanctionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMySanctionsRequest>(
          ListMySanctionsRequest.$_createMessage);
  static ListMySanctionsRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListMySanctionsResponse extends $pb.GeneratedMessage {
  factory ListMySanctionsResponse({
    $core.Iterable<Sanction>? sanctions,
    $core.String? nextPageToken,
  }) {
    final result = ListMySanctionsResponse._();
    if (sanctions != null) result.sanctions.addAll(sanctions);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMySanctionsResponse._();

  factory ListMySanctionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMySanctionsResponse()..mergeFromBuffer(data, registry);
  factory ListMySanctionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMySanctionsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMySanctionsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListMySanctionsResponse.$_createMessage)
    ..pPM<Sanction>(1, _omitFieldNames ? '' : 'sanctions',
        subBuilder: Sanction.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySanctionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySanctionsResponse copyWith(
          void Function(ListMySanctionsResponse) updates) =>
      super.copyWith((message) => updates(message as ListMySanctionsResponse))
          as ListMySanctionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMySanctionsResponse() / ListMySanctionsResponse.new instead')
  static ListMySanctionsResponse create() => ListMySanctionsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMySanctionsResponse._();
  @$core.override
  ListMySanctionsResponse createEmptyInstance() => ListMySanctionsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMySanctionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMySanctionsResponse>(
          ListMySanctionsResponse.$_createMessage);
  static ListMySanctionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Sanction> get sanctions => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

/// Sanction 是買家收到的一筆社群處分。
class Sanction extends $pb.GeneratedMessage {
  factory Sanction({
    $core.String? id,
    SanctionKind? kind,
    $4.PolicyCategory? category,
    $core.String? reason,
    $0.Timestamp? createTime,
  }) {
    final result = Sanction._();
    if (id != null) result.id = id;
    if (kind != null) result.kind = kind;
    if (category != null) result.category = category;
    if (reason != null) result.reason = reason;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  Sanction._();

  factory Sanction.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Sanction()..mergeFromBuffer(data, registry);
  factory Sanction.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Sanction()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Sanction',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: Sanction.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<SanctionKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: SanctionKind.values)
    ..aE<$4.PolicyCategory>(3, _omitFieldNames ? '' : 'category',
        enumValues: $4.PolicyCategory.values)
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..aOM<$0.Timestamp>(5, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sanction clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sanction copyWith(void Function(Sanction) updates) =>
      super.copyWith((message) => updates(message as Sanction)) as Sanction;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Sanction() / Sanction.new instead')
  static Sanction create() => Sanction._();
  static $pb.GeneratedMessage $_createMessage() => Sanction._();
  @$core.override
  Sanction createEmptyInstance() => Sanction._();
  @$core.pragma('dart2js:noInline')
  static Sanction getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Sanction>(Sanction.$_createMessage);
  static Sanction? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  SanctionKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(SanctionKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  /// 政策分類；不設表示沒有對應的分類。
  @$pb.TagNumber(3)
  $4.PolicyCategory get category => $_getN(2);
  @$pb.TagNumber(3)
  set category($4.PolicyCategory value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasCategory() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategory() => $_clearField(3);

  /// 給作者看的理由；沒有補充說明時是空字串。
  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(3);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(3);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);

  @$pb.TagNumber(5)
  $0.Timestamp get createTime => $_getN(4);
  @$pb.TagNumber(5)
  set createTime($0.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCreateTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearCreateTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.Timestamp ensureCreateTime() => $_ensure(4);
}

class ListMyHiddenContentRequest extends $pb.GeneratedMessage {
  factory ListMyHiddenContentRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListMyHiddenContentRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListMyHiddenContentRequest._();

  factory ListMyHiddenContentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyHiddenContentRequest()..mergeFromBuffer(data, registry);
  factory ListMyHiddenContentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyHiddenContentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyHiddenContentRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListMyHiddenContentRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyHiddenContentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyHiddenContentRequest copyWith(
          void Function(ListMyHiddenContentRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListMyHiddenContentRequest))
          as ListMyHiddenContentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyHiddenContentRequest() / ListMyHiddenContentRequest.new instead')
  static ListMyHiddenContentRequest create() => ListMyHiddenContentRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListMyHiddenContentRequest._();
  @$core.override
  ListMyHiddenContentRequest createEmptyInstance() =>
      ListMyHiddenContentRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyHiddenContentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyHiddenContentRequest>(
          ListMyHiddenContentRequest.$_createMessage);
  static ListMyHiddenContentRequest? _defaultInstance;

  /// 預設 20，上限 50。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListMyHiddenContentResponse extends $pb.GeneratedMessage {
  factory ListMyHiddenContentResponse({
    $core.Iterable<HiddenContent>? hiddenContents,
    $core.String? nextPageToken,
  }) {
    final result = ListMyHiddenContentResponse._();
    if (hiddenContents != null) result.hiddenContents.addAll(hiddenContents);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyHiddenContentResponse._();

  factory ListMyHiddenContentResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyHiddenContentResponse()..mergeFromBuffer(data, registry);
  factory ListMyHiddenContentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyHiddenContentResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyHiddenContentResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: ListMyHiddenContentResponse.$_createMessage)
    ..pPM<HiddenContent>(1, _omitFieldNames ? '' : 'hiddenContents',
        subBuilder: HiddenContent.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyHiddenContentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyHiddenContentResponse copyWith(
          void Function(ListMyHiddenContentResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListMyHiddenContentResponse))
          as ListMyHiddenContentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyHiddenContentResponse() / ListMyHiddenContentResponse.new instead')
  static ListMyHiddenContentResponse create() =>
      ListMyHiddenContentResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListMyHiddenContentResponse._();
  @$core.override
  ListMyHiddenContentResponse createEmptyInstance() =>
      ListMyHiddenContentResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyHiddenContentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyHiddenContentResponse>(
          ListMyHiddenContentResponse.$_createMessage);
  static ListMyHiddenContentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<HiddenContent> get hiddenContents => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

/// HiddenContent 是買家被隱藏的一則內容與隱藏的理由。
class HiddenContent extends $pb.GeneratedMessage {
  factory HiddenContent({
    $core.String? id,
    HiddenContentKind? kind,
    $core.String? contentId,
    $core.String? postId,
    $core.String? storeId,
    $core.String? body,
    $0.Timestamp? createTime,
    $4.PolicyCategory? category,
    $core.String? reason,
    $0.Timestamp? hideTime,
  }) {
    final result = HiddenContent._();
    if (id != null) result.id = id;
    if (kind != null) result.kind = kind;
    if (contentId != null) result.contentId = contentId;
    if (postId != null) result.postId = postId;
    if (storeId != null) result.storeId = storeId;
    if (body != null) result.body = body;
    if (createTime != null) result.createTime = createTime;
    if (category != null) result.category = category;
    if (reason != null) result.reason = reason;
    if (hideTime != null) result.hideTime = hideTime;
    return result;
  }

  HiddenContent._();

  factory HiddenContent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HiddenContent()..mergeFromBuffer(data, registry);
  factory HiddenContent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HiddenContent()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HiddenContent',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.social.v1alpha1'),
      createEmptyInstance: HiddenContent.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aE<HiddenContentKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: HiddenContentKind.values)
    ..aOS(3, _omitFieldNames ? '' : 'contentId')
    ..aOS(4, _omitFieldNames ? '' : 'postId')
    ..aOS(5, _omitFieldNames ? '' : 'storeId')
    ..aOS(6, _omitFieldNames ? '' : 'body')
    ..aOM<$0.Timestamp>(7, _omitFieldNames ? '' : 'createTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..aE<$4.PolicyCategory>(8, _omitFieldNames ? '' : 'category',
        enumValues: $4.PolicyCategory.values)
    ..aOS(9, _omitFieldNames ? '' : 'reason')
    ..aOM<$0.Timestamp>(10, _omitFieldNames ? '' : 'hideTime',
        subBuilder: $0.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HiddenContent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HiddenContent copyWith(void Function(HiddenContent) updates) =>
      super.copyWith((message) => updates(message as HiddenContent))
          as HiddenContent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HiddenContent() / HiddenContent.new instead')
  static HiddenContent create() => HiddenContent._();
  static $pb.GeneratedMessage $_createMessage() => HiddenContent._();
  @$core.override
  HiddenContent createEmptyInstance() => HiddenContent._();
  @$core.pragma('dart2js:noInline')
  static HiddenContent getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<HiddenContent>(
          HiddenContent.$_createMessage);
  static HiddenContent? _defaultInstance;

  /// 隱藏紀錄的 ID。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  HiddenContentKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(HiddenContentKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  /// 留言或討論區留言的 ID。
  @$pb.TagNumber(3)
  $core.String get contentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set contentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasContentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearContentId() => $_clearField(3);

  /// 留言所屬的貼文；討論區留言沒有。
  @$pb.TagNumber(4)
  $core.String get postId => $_getSZ(3);
  @$pb.TagNumber(4)
  set postId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPostId() => $_has(3);
  @$pb.TagNumber(4)
  void clearPostId() => $_clearField(4);

  /// 內容所屬的店家 ID（過渡期的十進位整數字串）；留言沒有時不設。
  @$pb.TagNumber(5)
  $core.String get storeId => $_getSZ(4);
  @$pb.TagNumber(5)
  set storeId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStoreId() => $_has(4);
  @$pb.TagNumber(5)
  void clearStoreId() => $_clearField(5);

  /// 被隱藏的本文，只有作者讀得到。
  @$pb.TagNumber(6)
  $core.String get body => $_getSZ(5);
  @$pb.TagNumber(6)
  set body($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasBody() => $_has(5);
  @$pb.TagNumber(6)
  void clearBody() => $_clearField(6);

  @$pb.TagNumber(7)
  $0.Timestamp get createTime => $_getN(6);
  @$pb.TagNumber(7)
  set createTime($0.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasCreateTime() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreateTime() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Timestamp ensureCreateTime() => $_ensure(6);

  @$pb.TagNumber(8)
  $4.PolicyCategory get category => $_getN(7);
  @$pb.TagNumber(8)
  set category($4.PolicyCategory value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCategory() => $_has(7);
  @$pb.TagNumber(8)
  void clearCategory() => $_clearField(8);

  /// 給作者看的理由；沒有補充說明時是空字串。
  @$pb.TagNumber(9)
  $core.String get reason => $_getSZ(8);
  @$pb.TagNumber(9)
  set reason($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasReason() => $_has(8);
  @$pb.TagNumber(9)
  void clearReason() => $_clearField(9);

  @$pb.TagNumber(10)
  $0.Timestamp get hideTime => $_getN(9);
  @$pb.TagNumber(10)
  set hideTime($0.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasHideTime() => $_has(9);
  @$pb.TagNumber(10)
  void clearHideTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $0.Timestamp ensureHideTime() => $_ensure(9);
}

/// SocialPublicService 是任何人都能讀的貼文與留言，不需要登入。
class SocialPublicServiceApi {
  final $pb.RpcClient _client;

  SocialPublicServiceApi(this._client);

  /// GetStorePost 讀一篇觀看者可讀的貼文：已發布、沒有被下架或平台隱藏，而且店家不是被觀看者個人封鎖的。其他一律回 not_found，
  /// 不區分原因。店主讀自己店的貼文不套個人封鎖。這個方法不計瀏覽數，瀏覽由 RecordPostView 另外記錄。
  $async.Future<GetPostResponse> getPost(
          $pb.ClientContext? ctx, GetPostRequest request) =>
      _client.invoke<GetPostResponse>(
          ctx, 'SocialPublicService', 'GetPost', request, GetPostResponse());

  /// ListStorePosts 列出觀看者可讀的貼文，依第一次發布的時間新到舊（同一時間以 ID 排序）。不帶篩選是全站動態牆，帶 store_id 是該店的貼文頁，
  /// 帶 hashtag 是該標籤的貼文頁；兩個篩選不能同時帶（invalid_argument）。
  /// 觀看者個人封鎖的店不出現在動態牆，過濾在分頁之前，所以一頁可以少於 page_size，結束只看 next_page_token。
  $async.Future<ListPostsResponse> listPosts(
          $pb.ClientContext? ctx, ListPostsRequest request) =>
      _client.invoke<ListPostsResponse>(ctx, 'SocialPublicService', 'ListPosts',
          request, ListPostsResponse());

  /// ListComments 列出貼文的頂層留言，或某則頂層留言的回覆，依建立時間舊到新。刪除或隱藏而底下有可見回覆的頂層留言是佔位項目
  /// （只有 id、post_id、create_time、reply_count 與 state）；與觀看者任一方向有個人封鎖的買家的留言連同回覆整則不出現。
  /// 貼文讀不到時回 not_found。
  $async.Future<ListCommentsResponse> listComments(
          $pb.ClientContext? ctx, ListCommentsRequest request) =>
      _client.invoke<ListCommentsResponse>(ctx, 'SocialPublicService',
          'ListComments', request, ListCommentsResponse());

  /// SuggestTrendingPosts 是「本週主題」：最近發布的 30 篇觀看者可讀的貼文中，瀏覽數最高的前 max_results 篇（同分以發布時間、ID 排序）。
  /// 沒有分頁，時間窗不是日期而是「最近 30 篇」，所以回傳的篇數不超過 30，也可能更少。
  $async.Future<SuggestTrendingPostsResponse> suggestTrendingPosts(
          $pb.ClientContext? ctx, SuggestTrendingPostsRequest request) =>
      _client.invoke<SuggestTrendingPostsResponse>(ctx, 'SocialPublicService',
          'SuggestTrendingPosts', request, SuggestTrendingPostsResponse());

  /// SuggestTags 是標籤輸入時的建議：公開貼文上名稱符合的標籤與貼文數，貼文數多的在前。全域彙總，不依觀看者。沒有分頁，最多 20 筆。
  $async.Future<SuggestTagsResponse> suggestTags(
          $pb.ClientContext? ctx, SuggestTagsRequest request) =>
      _client.invoke<SuggestTagsResponse>(ctx, 'SocialPublicService',
          'SuggestTags', request, SuggestTagsResponse());

  /// SuggestTrendingTags 是熱門標籤：發布時間落在時間窗內的公開貼文上，各標籤的貼文數，多的在前。全域彙總，不依觀看者。
  /// 這是只取前 max_results 名的排行榜（最多 50 名），不分頁；時間窗最長 30 天。
  $async.Future<SuggestTrendingTagsResponse> suggestTrendingTags(
          $pb.ClientContext? ctx, SuggestTrendingTagsRequest request) =>
      _client.invoke<SuggestTrendingTagsResponse>(ctx, 'SocialPublicService',
          'SuggestTrendingTags', request, SuggestTrendingTagsResponse());

  /// ListForumEntries 列出店家討論區的頂層留言，或某則頂層留言的回覆，依建立時間舊到新。刪除或隱藏而底下有可見回覆的頂層留言是佔位項目
  /// （只有 id、store_id、create_time、reply_count 與 state）；觀看者封鎖了這家店、或與作者任一方向有個人封鎖時，對應的內容不出現，
  /// 該頁因此可以少於 page_size。店家讀不到時是空的一頁；指定的 parent_id 不屬於這家店回 not_found。
  $async.Future<ListForumEntriesResponse> listForumEntries(
          $pb.ClientContext? ctx, ListForumEntriesRequest request) =>
      _client.invoke<ListForumEntriesResponse>(ctx, 'SocialPublicService',
          'ListForumEntries', request, ListForumEntriesResponse());

  /// RecordPostView 記一次貼文瀏覽，匿名也能呼叫。只計觀看者讀得到的貼文，同一篇、同一個台北日、同一個觀看者只算一次；
  /// 讀不到的貼文與沒有計入的呼叫都回成功，不區分原因。登入者以帳號去重，訪客以來源 IP 與 User-Agent 去重；停權的帳號與無效的憑證以訪客計入。
  /// 店主看自己店的貼文不計。每個來源 IP 有額度，超過回 resource_exhausted 與 RetryInfo。瀏覽是寫入，所以不標 NO_SIDE_EFFECTS，client 以 POST 呼叫；
  /// 讀取貼文的方法（GetPost）不計瀏覽。
  $async.Future<RecordPostViewResponse> recordPostView(
          $pb.ClientContext? ctx, RecordPostViewRequest request) =>
      _client.invoke<RecordPostViewResponse>(ctx, 'SocialPublicService',
          'RecordPostView', request, RecordPostViewResponse());
}

/// SocialAppService 是登入者在貼文、留言與店家討論區上的互動寫入。呼叫者的身分由伺服器從 session 推導：呼叫者此刻的店擁有該貼文或討論區時，
/// 互動者是店家，否則是買家。被店家封鎖的買家在這家店的留言、發言、編輯與按讚被拒（permission_denied，不另給原因，與讀取回應的
/// viewer_can_comment 一致）。
class SocialAppServiceApi {
  final $pb.RpcClient _client;

  SocialAppServiceApi(this._client);

  /// SharePost 記一次分享：同一位買家對同一篇只算一次，重送安全。貼文讀不到回 not_found。
  $async.Future<SharePostResponse> sharePost(
          $pb.ClientContext? ctx, SharePostRequest request) =>
      _client.invoke<SharePostResponse>(
          ctx, 'SocialAppService', 'SharePost', request, SharePostResponse());

  /// LikePost 對貼文按讚，已經讚過也回成功（設定而不是切換）。只有買家能按讚；貼文讀不到回 not_found。
  $async.Future<LikePostResponse> likePost(
          $pb.ClientContext? ctx, LikePostRequest request) =>
      _client.invoke<LikePostResponse>(
          ctx, 'SocialAppService', 'LikePost', request, LikePostResponse());

  /// UnlikePost 取消貼文的讚，沒有讚過也回成功。
  $async.Future<UnlikePostResponse> unlikePost(
          $pb.ClientContext? ctx, UnlikePostRequest request) =>
      _client.invoke<UnlikePostResponse>(
          ctx, 'SocialAppService', 'UnlikePost', request, UnlikePostResponse());

  /// LikeComment 對留言按讚，已經讚過也回成功。留言讀不到回 not_found。
  $async.Future<LikeCommentResponse> likeComment(
          $pb.ClientContext? ctx, LikeCommentRequest request) =>
      _client.invoke<LikeCommentResponse>(ctx, 'SocialAppService',
          'LikeComment', request, LikeCommentResponse());

  /// UnlikeComment 取消留言的讚，沒有讚過也回成功。
  $async.Future<UnlikeCommentResponse> unlikeComment(
          $pb.ClientContext? ctx, UnlikeCommentRequest request) =>
      _client.invoke<UnlikeCommentResponse>(ctx, 'SocialAppService',
          'UnlikeComment', request, UnlikeCommentResponse());

  /// CreateComment 在貼文下留言，帶 parent_id 是回覆（回覆只限貼文所屬的店家）。帶 request_id 重送回目前的留言（狀態與計數是此刻的），不重複建立。
  $async.Future<CreateCommentResponse> createComment(
          $pb.ClientContext? ctx, CreateCommentRequest request) =>
      _client.invoke<CreateCommentResponse>(ctx, 'SocialAppService',
          'CreateComment', request, CreateCommentResponse());

  /// UpdateComment 修改自己的留言內文（update_mask 列 body）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。被隱藏的留言不能修改（failed_precondition），已刪除的回 not_found。
  $async.Future<UpdateCommentResponse> updateComment(
          $pb.ClientContext? ctx, UpdateCommentRequest request) =>
      _client.invoke<UpdateCommentResponse>(ctx, 'SocialAppService',
          'UpdateComment', request, UpdateCommentResponse());

  /// DeleteComment 刪除留言：作者，或貼文所屬的店家。已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。
  $async.Future<DeleteCommentResponse> deleteComment(
          $pb.ClientContext? ctx, DeleteCommentRequest request) =>
      _client.invoke<DeleteCommentResponse>(ctx, 'SocialAppService',
          'DeleteComment', request, DeleteCommentResponse());

  /// CreateForumEntry 在店家的討論區發言，帶 parent_id 是回覆（討論區的回覆不限身分，買家也能回覆）。帶 request_id 重送回目前的項目，不重複建立。
  $async.Future<CreateForumEntryResponse> createForumEntry(
          $pb.ClientContext? ctx, CreateForumEntryRequest request) =>
      _client.invoke<CreateForumEntryResponse>(ctx, 'SocialAppService',
          'CreateForumEntry', request, CreateForumEntryResponse());

  /// UpdateForumEntry 修改自己的討論區發言內文（update_mask 列 body）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。被隱藏的不能修改（failed_precondition），已刪除的回 not_found。
  $async.Future<UpdateForumEntryResponse> updateForumEntry(
          $pb.ClientContext? ctx, UpdateForumEntryRequest request) =>
      _client.invoke<UpdateForumEntryResponse>(ctx, 'SocialAppService',
          'UpdateForumEntry', request, UpdateForumEntryResponse());

  /// DeleteForumEntry 刪除討論區發言：作者，或該店家。已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。
  $async.Future<DeleteForumEntryResponse> deleteForumEntry(
          $pb.ClientContext? ctx, DeleteForumEntryRequest request) =>
      _client.invoke<DeleteForumEntryResponse>(ctx, 'SocialAppService',
          'DeleteForumEntry', request, DeleteForumEntryResponse());

  /// CreateStorePost 以呼叫者此刻的店建立貼文（店家由伺服器推導，請求不帶 store_id）。publish 為真時同一個動作發布，公開要件（恰好一張圖、內文必填）在轉為公開時驗證；
  /// 草稿可以不完整。帶 request_id 重送回第一次建立的貼文目前的樣子（狀態與計數是此刻的），不重複建立；內容不同回 invalid_argument。需要賣家能力 CAP_MARKETING。
  $async.Future<CreatePostResponse> createPost(
          $pb.ClientContext? ctx, CreatePostRequest request) =>
      _client.invoke<CreatePostResponse>(
          ctx, 'SocialAppService', 'CreatePost', request, CreatePostResponse());

  /// GetMyPost 讀呼叫者此刻的店自己的貼文：任何狀態（草稿、已發布、已下架），不含已刪除；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。
  /// 賣家能力 CAP_CONTENT_CONTRACT（停權時仍可讀，才能下架與刪除）。
  $async.Future<GetMyPostResponse> getMyPost(
          $pb.ClientContext? ctx, GetMyPostRequest request) =>
      _client.invoke<GetMyPostResponse>(
          ctx, 'SocialAppService', 'GetMyPost', request, GetMyPostResponse());

  /// ListMyPosts 列出呼叫者此刻的店自己的貼文（含草稿與下架），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。賣家能力 CAP_CONTENT_CONTRACT。
  $async.Future<ListMyPostsResponse> listMyPosts(
          $pb.ClientContext? ctx, ListMyPostsRequest request) =>
      _client.invoke<ListMyPostsResponse>(ctx, 'SocialAppService',
          'ListMyPosts', request, ListMyPostsResponse());

  /// UpdateStorePost 修改自己的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。
  /// 已發布或有待發布排程的貼文被修改後仍要滿足公開要件。需要賣家能力 CAP_MARKETING。
  $async.Future<UpdatePostResponse> updatePost(
          $pb.ClientContext? ctx, UpdatePostRequest request) =>
      _client.invoke<UpdatePostResponse>(
          ctx, 'SocialAppService', 'UpdatePost', request, UpdatePostResponse());

  /// PublishPost 發布草稿或下架的貼文；etag 必填。被內容下架的貼文一律回 failed_precondition，已經是已發布也一樣（授權、etag 必填與貼文歸屬的檢查先於下架檢查，下架檢查先於重複發布的冪等與 etag 比對，所以被下架的貼文帶過期的 etag 也是 failed_precondition）；沒被下架而已經是已發布時是冪等的 no-op（不檢查 etag、不改版本）。貼文沒有圖片或內文（不滿足公開要件）回 failed_precondition。需要賣家能力 CAP_MARKETING。
  $async.Future<PublishPostResponse> publishPost(
          $pb.ClientContext? ctx, PublishPostRequest request) =>
      _client.invoke<PublishPostResponse>(ctx, 'SocialAppService',
          'PublishPost', request, PublishPostResponse());

  /// UnpublishStorePost 下架已發布的貼文；etag 必填。已經下架的是冪等的 no-op；草稿沒有發布過，回 failed_precondition。收縮類動作，店家停權時仍可執行（CAP_CONTENT_CONTRACT）；帳號停權照動作性質擋寫入（ACCOUNT_SUSPENDED）。
  $async.Future<UnpublishPostResponse> unpublishPost(
          $pb.ClientContext? ctx, UnpublishPostRequest request) =>
      _client.invoke<UnpublishPostResponse>(ctx, 'SocialAppService',
          'UnpublishPost', request, UnpublishPostResponse());

  /// DeletePost 刪除自己的貼文；已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。收縮類動作，店家停權時仍可執行（CAP_CONTENT_CONTRACT）；帳號停權照動作性質擋寫入（ACCOUNT_SUSPENDED）。
  $async.Future<DeletePostResponse> deletePost(
          $pb.ClientContext? ctx, DeletePostRequest request) =>
      _client.invoke<DeletePostResponse>(
          ctx, 'SocialAppService', 'DeletePost', request, DeletePostResponse());

  /// ScheduleStorePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天，而且貼文要滿足公開要件（沒有圖片或內文回 failed_precondition）；貼文已有待發布的排程時，舊的被取代。etag 必填。
  /// 需要賣家能力 CAP_MARKETING。排程的讀取與取消見 ListPendingSchedules、CancelPostSchedule。
  $async.Future<SchedulePostResponse> schedulePost(
          $pb.ClientContext? ctx, SchedulePostRequest request) =>
      _client.invoke<SchedulePostResponse>(ctx, 'SocialAppService',
          'SchedulePost', request, SchedulePostResponse());

  /// ListPendingSchedules 列出呼叫者此刻的店待發布的排程（含店主自己排的與代行營運排的），發布時間早的在前（同一時間以 ID 排序）。店主看得到代行排的貼文並能取消（CancelPostSchedule）。
  /// 只讀自己店的排程，店家由伺服器推導。
  $async.Future<ListPendingSchedulesResponse> listPendingSchedules(
          $pb.ClientContext? ctx, ListPendingSchedulesRequest request) =>
      _client.invoke<ListPendingSchedulesResponse>(ctx, 'SocialAppService',
          'ListPendingSchedules', request, ListPendingSchedulesResponse());

  /// CancelPostSchedule 店主取消貼文待發布的排程（不分發起者；取消原因記為店主取消）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT。貼文沒有待發布的排程時什麼都不做（不檢查 etag），回目前的 etag。
  /// 貼文不是這家店的回 not_found。需要賣家能力 CAP_MARKETING。
  $async.Future<CancelPostScheduleResponse> cancelPostSchedule(
          $pb.ClientContext? ctx, CancelPostScheduleRequest request) =>
      _client.invoke<CancelPostScheduleResponse>(ctx, 'SocialAppService',
          'CancelPostSchedule', request, CancelPostScheduleResponse());

  /// ListStoreComments 列出呼叫者此刻的店所有貼文（含草稿與下架的貼文）的頂層留言，依建立時間舊到新；post_id 選填，設了只列那一篇。
  /// 刪除或隱藏而底下有可見回覆的頂層留言是佔位項目。unreplied 為真時只列「未回覆」的留言：本店貼文上、作者不是本店的頂層留言，沒有刪除、沒有被隱藏，
  /// 而且底下沒有任何一則仍可見、作者是本店的回覆（已刪除或被隱藏的店家回覆不算回覆）；此時沒有佔位項目。賣家能力 CAP_CONTENT_CONTRACT。
  $async.Future<ListStoreCommentsResponse> listStoreComments(
          $pb.ClientContext? ctx, ListStoreCommentsRequest request) =>
      _client.invoke<ListStoreCommentsResponse>(ctx, 'SocialAppService',
          'ListStoreComments', request, ListStoreCommentsResponse());

  /// GetStoreSocialSummary 是賣家中心的動態數：呼叫者此刻的店的貼文依狀態的篇數（已刪除的不算），以及待發布排程的篇數。數字由伺服器以資料庫計數，
  /// 不是 client 以列表長度推算。賣家能力 CAP_CONTENT_CONTRACT。
  $async.Future<GetStoreSocialSummaryResponse> getStoreSocialSummary(
          $pb.ClientContext? ctx, GetStoreSocialSummaryRequest request) =>
      _client.invoke<GetStoreSocialSummaryResponse>(ctx, 'SocialAppService',
          'GetStoreSocialSummary', request, GetStoreSocialSummaryResponse());

  /// ListMySanctions 列出呼叫者（買家）收到的社群處分，新到舊：種類、政策分類與理由，作者有權讀自己的處分。停權的帳號也可以呼叫。
  $async.Future<ListMySanctionsResponse> listMySanctions(
          $pb.ClientContext? ctx, ListMySanctionsRequest request) =>
      _client.invoke<ListMySanctionsResponse>(ctx, 'SocialAppService',
          'ListMySanctions', request, ListMySanctionsResponse());

  /// ListMyHiddenContent 列出呼叫者（買家）被平台隱藏的留言與討論區留言，新隱藏到舊：連同本文與隱藏的政策分類與理由（別人看不到被隱藏的內容）。
  /// 已還原的隱藏不在名單裡。停權的帳號也可以呼叫。
  $async.Future<ListMyHiddenContentResponse> listMyHiddenContent(
          $pb.ClientContext? ctx, ListMyHiddenContentRequest request) =>
      _client.invoke<ListMyHiddenContentResponse>(ctx, 'SocialAppService',
          'ListMyHiddenContent', request, ListMyHiddenContentResponse());
}

/// SocialDelegatedService 是代行營運中的平台管理員讀寫代行店家的貼文：憑證是 X-HK-Operation-Session 標頭裡的代行 session（由 website 的 BFF 轉送，不放在請求訊息），
/// 不讀顧客或管理員的身分；目標店家是 session 建立當下綁定的那一家，請求不帶 store_id。商品與訂單由各自模組的 `<模組>DelegatedService` 在 A10 提供。
/// 回應一律不可快取（private, no-store）。
///
/// 能力（posts.read、posts.write、posts.publish）依方法宣告：憑證缺少或無效回 unauthenticated；session 已結束、到期或撤銷回 failed_precondition；
/// session 沒有授予對應的能力回 permission_denied；店家此刻不可代行（不存在、已關閉、等待刪除、停權）回 failed_precondition，只拒絕、不撤銷 session。
///
/// 稽核：成功的寫入在同一個 transaction 內留下一列 SUCCEEDED（寫入失敗時一併 rollback）；被拒絕的請求（含讀取）與寫入的業務失敗另開 transaction 留下一列 FAILED；
/// 成功的讀取不稽核。
class SocialDelegatedServiceApi {
  final $pb.RpcClient _client;

  SocialDelegatedServiceApi(this._client);

  /// ListStorePosts 列出代行店家自己的貼文（含草稿與下架，不含已刪除），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。需要能力 posts.read。
  $async.Future<ListStorePostsResponse> listStorePosts(
          $pb.ClientContext? ctx, ListStorePostsRequest request) =>
      _client.invoke<ListStorePostsResponse>(ctx, 'SocialDelegatedService',
          'ListStorePosts', request, ListStorePostsResponse());

  /// GetStorePost 讀代行店家的一篇貼文（任何狀態，不含已刪除）；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。需要能力 posts.read。
  $async.Future<GetStorePostResponse> getStorePost(
          $pb.ClientContext? ctx, GetStorePostRequest request) =>
      _client.invoke<GetStorePostResponse>(ctx, 'SocialDelegatedService',
          'GetStorePost', request, GetStorePostResponse());

  /// CreateStorePost 以代行店家建立草稿貼文（建立一律是草稿；要發布再呼叫 PublishStorePost）。同一個 session 以同一個 request_id 重送回第一次建立的貼文目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 posts.write。
  $async.Future<CreateStorePostResponse> createStorePost(
          $pb.ClientContext? ctx, CreateStorePostRequest request) =>
      _client.invoke<CreateStorePostResponse>(ctx, 'SocialDelegatedService',
          'CreateStorePost', request, CreateStorePostResponse());

  /// UpdateStorePost 修改代行店家的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留稽核。重送規則同 CreateStorePost。需要能力 posts.write。
  $async.Future<UpdateStorePostResponse> updateStorePost(
          $pb.ClientContext? ctx, UpdateStorePostRequest request) =>
      _client.invoke<UpdateStorePostResponse>(ctx, 'SocialDelegatedService',
          'UpdateStorePost', request, UpdateStorePostResponse());

  /// PublishStorePost 立即發布草稿或下架的貼文；etag 必填。重送規則同 CreateStorePost。需要能力 posts.publish。
  $async.Future<PublishStorePostResponse> publishStorePost(
          $pb.ClientContext? ctx, PublishStorePostRequest request) =>
      _client.invoke<PublishStorePostResponse>(ctx, 'SocialDelegatedService',
          'PublishStorePost', request, PublishStorePostResponse());

  /// UnpublishStorePost 下架已發布的貼文；etag 必填。重送規則同 CreateStorePost。需要能力 posts.publish。
  $async.Future<UnpublishStorePostResponse> unpublishStorePost(
          $pb.ClientContext? ctx, UnpublishStorePostRequest request) =>
      _client.invoke<UnpublishStorePostResponse>(ctx, 'SocialDelegatedService',
          'UnpublishStorePost', request, UnpublishStorePostResponse());

  /// ScheduleStorePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天；貼文已有待發布的排程時，舊的被取代。排程屬於店家，session 結束後仍會
  /// 在到點時重驗授權並發布（授權已撤回則取消）。稽核寫 POST_SCHEDULE，到點發布時另寫一列 POST_PUBLISH。重送規則同 CreateStorePost。需要能力 posts.publish。
  $async.Future<ScheduleStorePostResponse> scheduleStorePost(
          $pb.ClientContext? ctx, ScheduleStorePostRequest request) =>
      _client.invoke<ScheduleStorePostResponse>(ctx, 'SocialDelegatedService',
          'ScheduleStorePost', request, ScheduleStorePostResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
