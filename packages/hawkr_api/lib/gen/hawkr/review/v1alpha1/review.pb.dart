// This is a generated file - do not edit.
//
// Generated from hawkr/review/v1alpha1/review.proto.

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
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../common/v1alpha1/image.pb.dart' as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// ReviewAuthor 是評價的作者，名稱與頭像在讀取時由 buyer 補。
class ReviewAuthor extends $pb.GeneratedMessage {
  factory ReviewAuthor({
    $core.String? name,
    $0.Image? avatar,
  }) {
    final result = ReviewAuthor._();
    if (name != null) result.name = name;
    if (avatar != null) result.avatar = avatar;
    return result;
  }

  ReviewAuthor._();

  factory ReviewAuthor.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReviewAuthor()..mergeFromBuffer(data, registry);
  factory ReviewAuthor.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReviewAuthor()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReviewAuthor',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ReviewAuthor.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOM<$0.Image>(2, _omitFieldNames ? '' : 'avatar',
        subBuilder: $0.Image.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewAuthor clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewAuthor copyWith(void Function(ReviewAuthor) updates) =>
      super.copyWith((message) => updates(message as ReviewAuthor))
          as ReviewAuthor;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReviewAuthor() / ReviewAuthor.new instead')
  static ReviewAuthor create() => ReviewAuthor._();
  static $pb.GeneratedMessage $_createMessage() => ReviewAuthor._();
  @$core.override
  ReviewAuthor createEmptyInstance() => ReviewAuthor._();
  @$core.pragma('dart2js:noInline')
  static ReviewAuthor getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReviewAuthor>(
          ReviewAuthor.$_createMessage);
  static ReviewAuthor? _defaultInstance;

  /// 買家的暱稱；沒有設定時不設。
  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $0.Image get avatar => $_getN(1);
  @$pb.TagNumber(2)
  set avatar($0.Image value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAvatar() => $_has(1);
  @$pb.TagNumber(2)
  void clearAvatar() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Image ensureAvatar() => $_ensure(1);
}

/// ReviewReply 是店家的回覆。
class ReviewReply extends $pb.GeneratedMessage {
  factory ReviewReply({
    $core.String? content,
    $1.Timestamp? replyTime,
  }) {
    final result = ReviewReply._();
    if (content != null) result.content = content;
    if (replyTime != null) result.replyTime = replyTime;
    return result;
  }

  ReviewReply._();

  factory ReviewReply.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReviewReply()..mergeFromBuffer(data, registry);
  factory ReviewReply.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReviewReply()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReviewReply',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ReviewReply.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'content')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'replyTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewReply clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewReply copyWith(void Function(ReviewReply) updates) =>
      super.copyWith((message) => updates(message as ReviewReply))
          as ReviewReply;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReviewReply() / ReviewReply.new instead')
  static ReviewReply create() => ReviewReply._();
  static $pb.GeneratedMessage $_createMessage() => ReviewReply._();
  @$core.override
  ReviewReply createEmptyInstance() => ReviewReply._();
  @$core.pragma('dart2js:noInline')
  static ReviewReply getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReviewReply>(
          ReviewReply.$_createMessage);
  static ReviewReply? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get content => $_getSZ(0);
  @$pb.TagNumber(1)
  set content($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasContent() => $_has(0);
  @$pb.TagNumber(1)
  void clearContent() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get replyTime => $_getN(1);
  @$pb.TagNumber(2)
  set replyTime($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasReplyTime() => $_has(1);
  @$pb.TagNumber(2)
  void clearReplyTime() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureReplyTime() => $_ensure(1);
}

/// Review 是一則評價。評價建立後不變，只有店家的回覆可以新增或覆蓋。
class Review extends $pb.GeneratedMessage {
  factory Review({
    $core.String? id,
    $core.String? productId,
    $core.String? storeId,
    $core.int? rating,
    $core.String? content,
    $core.Iterable<$0.Image>? images,
    $core.String? skuName,
    $0.Image? itemImage,
    $fixnum.Int64? price,
    $1.Timestamp? createTime,
    ReviewReply? reply,
    ReviewAuthor? author,
    $core.String? orderNo,
    $core.String? orderItemId,
  }) {
    final result = Review._();
    if (id != null) result.id = id;
    if (productId != null) result.productId = productId;
    if (storeId != null) result.storeId = storeId;
    if (rating != null) result.rating = rating;
    if (content != null) result.content = content;
    if (images != null) result.images.addAll(images);
    if (skuName != null) result.skuName = skuName;
    if (itemImage != null) result.itemImage = itemImage;
    if (price != null) result.price = price;
    if (createTime != null) result.createTime = createTime;
    if (reply != null) result.reply = reply;
    if (author != null) result.author = author;
    if (orderNo != null) result.orderNo = orderNo;
    if (orderItemId != null) result.orderItemId = orderItemId;
    return result;
  }

  Review._();

  factory Review.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Review()..mergeFromBuffer(data, registry);
  factory Review.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Review()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Review',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: Review.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'productId')
    ..aOS(3, _omitFieldNames ? '' : 'storeId')
    ..aI(4, _omitFieldNames ? '' : 'rating')
    ..aOS(5, _omitFieldNames ? '' : 'content')
    ..pPM<$0.Image>(6, _omitFieldNames ? '' : 'images',
        subBuilder: $0.Image.$_createMessage)
    ..aOS(7, _omitFieldNames ? '' : 'skuName')
    ..aOM<$0.Image>(8, _omitFieldNames ? '' : 'itemImage',
        subBuilder: $0.Image.$_createMessage)
    ..aInt64(9, _omitFieldNames ? '' : 'price')
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<ReviewReply>(11, _omitFieldNames ? '' : 'reply',
        subBuilder: ReviewReply.$_createMessage)
    ..aOM<ReviewAuthor>(12, _omitFieldNames ? '' : 'author',
        subBuilder: ReviewAuthor.$_createMessage)
    ..aOS(13, _omitFieldNames ? '' : 'orderNo')
    ..aOS(14, _omitFieldNames ? '' : 'orderItemId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Review clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Review copyWith(void Function(Review) updates) =>
      super.copyWith((message) => updates(message as Review)) as Review;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Review() / Review.new instead')
  static Review create() => Review._();
  static $pb.GeneratedMessage $_createMessage() => Review._();
  @$core.override
  Review createEmptyInstance() => Review._();
  @$core.pragma('dart2js:noInline')
  static Review getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Review>(Review.$_createMessage);
  static Review? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get productId => $_getSZ(1);
  @$pb.TagNumber(2)
  set productId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProductId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProductId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get storeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set storeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStoreId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStoreId() => $_clearField(3);

  /// 1 到 5。
  @$pb.TagNumber(4)
  $core.int get rating => $_getIZ(3);
  @$pb.TagNumber(4)
  set rating($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRating() => $_has(3);
  @$pb.TagNumber(4)
  void clearRating() => $_clearField(4);

  /// 評價內文；沒有文字時是空字串。
  @$pb.TagNumber(5)
  $core.String get content => $_getSZ(4);
  @$pb.TagNumber(5)
  set content($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasContent() => $_has(4);
  @$pb.TagNumber(5)
  void clearContent() => $_clearField(5);

  /// 附圖，依上傳順序，最多 4 張。
  @$pb.TagNumber(6)
  $pb.PbList<$0.Image> get images => $_getList(5);

  /// 下單當時的規格名稱。
  @$pb.TagNumber(7)
  $core.String get skuName => $_getSZ(6);
  @$pb.TagNumber(7)
  set skuName($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSkuName() => $_has(6);
  @$pb.TagNumber(7)
  void clearSkuName() => $_clearField(7);

  /// 訂單項目當時的商品圖；沒有時不設。
  @$pb.TagNumber(8)
  $0.Image get itemImage => $_getN(7);
  @$pb.TagNumber(8)
  set itemImage($0.Image value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasItemImage() => $_has(7);
  @$pb.TagNumber(8)
  void clearItemImage() => $_clearField(8);
  @$pb.TagNumber(8)
  $0.Image ensureItemImage() => $_ensure(7);

  /// 下單當時的單價，新台幣元。
  @$pb.TagNumber(9)
  $fixnum.Int64 get price => $_getI64(8);
  @$pb.TagNumber(9)
  set price($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasPrice() => $_has(8);
  @$pb.TagNumber(9)
  void clearPrice() => $_clearField(9);

  @$pb.TagNumber(10)
  $1.Timestamp get createTime => $_getN(9);
  @$pb.TagNumber(10)
  set createTime($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCreateTime() => $_has(9);
  @$pb.TagNumber(10)
  void clearCreateTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureCreateTime() => $_ensure(9);

  /// 店家的回覆；還沒有回覆或回覆被下架時不設。
  @$pb.TagNumber(11)
  ReviewReply get reply => $_getN(10);
  @$pb.TagNumber(11)
  set reply(ReviewReply value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasReply() => $_has(10);
  @$pb.TagNumber(11)
  void clearReply() => $_clearField(11);
  @$pb.TagNumber(11)
  ReviewReply ensureReply() => $_ensure(10);

  @$pb.TagNumber(12)
  ReviewAuthor get author => $_getN(11);
  @$pb.TagNumber(12)
  set author(ReviewAuthor value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasAuthor() => $_has(11);
  @$pb.TagNumber(12)
  void clearAuthor() => $_clearField(12);
  @$pb.TagNumber(12)
  ReviewAuthor ensureAuthor() => $_ensure(11);

  /// 訂單編號。只有作者本人（CreateReview、ListMyReviews）與店家自己讀自己店的評價（ListOwnStoreReviews、ReplyReview）的回應才設，公開讀取不設。
  @$pb.TagNumber(13)
  $core.String get orderNo => $_getSZ(12);
  @$pb.TagNumber(13)
  set orderNo($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasOrderNo() => $_has(12);
  @$pb.TagNumber(13)
  void clearOrderNo() => $_clearField(13);

  /// 訂單項目 ID，設定的條件同 order_no。
  @$pb.TagNumber(14)
  $core.String get orderItemId => $_getSZ(13);
  @$pb.TagNumber(14)
  set orderItemId($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasOrderItemId() => $_has(13);
  @$pb.TagNumber(14)
  void clearOrderItemId() => $_clearField(14);
}

class ListProductReviewsRequest extends $pb.GeneratedMessage {
  factory ListProductReviewsRequest({
    $core.String? productId,
    $core.int? pageSize,
    $core.String? pageToken,
    $core.int? rating,
    $core.bool? withImages,
  }) {
    final result = ListProductReviewsRequest._();
    if (productId != null) result.productId = productId;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (rating != null) result.rating = rating;
    if (withImages != null) result.withImages = withImages;
    return result;
  }

  ListProductReviewsRequest._();

  factory ListProductReviewsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductReviewsRequest()..mergeFromBuffer(data, registry);
  factory ListProductReviewsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductReviewsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListProductReviewsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListProductReviewsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..aI(4, _omitFieldNames ? '' : 'rating')
    ..aOB(5, _omitFieldNames ? '' : 'withImages')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductReviewsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductReviewsRequest copyWith(
          void Function(ListProductReviewsRequest) updates) =>
      super.copyWith((message) => updates(message as ListProductReviewsRequest))
          as ListProductReviewsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListProductReviewsRequest() / ListProductReviewsRequest.new instead')
  static ListProductReviewsRequest create() => ListProductReviewsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListProductReviewsRequest._();
  @$core.override
  ListProductReviewsRequest createEmptyInstance() =>
      ListProductReviewsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListProductReviewsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListProductReviewsRequest>(
          ListProductReviewsRequest.$_createMessage);
  static ListProductReviewsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);

  /// 預設 20，上限 100。
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

  /// 只列這個星等（1 到 5）；不設是不篩選。
  @$pb.TagNumber(4)
  $core.int get rating => $_getIZ(3);
  @$pb.TagNumber(4)
  set rating($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRating() => $_has(3);
  @$pb.TagNumber(4)
  void clearRating() => $_clearField(4);

  /// 為真時只列有附圖的評價。
  @$pb.TagNumber(5)
  $core.bool get withImages => $_getBF(4);
  @$pb.TagNumber(5)
  set withImages($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasWithImages() => $_has(4);
  @$pb.TagNumber(5)
  void clearWithImages() => $_clearField(5);
}

class ListProductReviewsResponse extends $pb.GeneratedMessage {
  factory ListProductReviewsResponse({
    $core.Iterable<Review>? reviews,
    $core.String? nextPageToken,
  }) {
    final result = ListProductReviewsResponse._();
    if (reviews != null) result.reviews.addAll(reviews);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListProductReviewsResponse._();

  factory ListProductReviewsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductReviewsResponse()..mergeFromBuffer(data, registry);
  factory ListProductReviewsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListProductReviewsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListProductReviewsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListProductReviewsResponse.$_createMessage)
    ..pPM<Review>(1, _omitFieldNames ? '' : 'reviews',
        subBuilder: Review.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductReviewsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListProductReviewsResponse copyWith(
          void Function(ListProductReviewsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListProductReviewsResponse))
          as ListProductReviewsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListProductReviewsResponse() / ListProductReviewsResponse.new instead')
  static ListProductReviewsResponse create() => ListProductReviewsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListProductReviewsResponse._();
  @$core.override
  ListProductReviewsResponse createEmptyInstance() =>
      ListProductReviewsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListProductReviewsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListProductReviewsResponse>(
          ListProductReviewsResponse.$_createMessage);
  static ListProductReviewsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Review> get reviews => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListStoreReviewsRequest extends $pb.GeneratedMessage {
  factory ListStoreReviewsRequest({
    $core.String? storeId,
    $core.int? pageSize,
    $core.String? pageToken,
    $core.int? rating,
    $core.bool? withImages,
  }) {
    final result = ListStoreReviewsRequest._();
    if (storeId != null) result.storeId = storeId;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (rating != null) result.rating = rating;
    if (withImages != null) result.withImages = withImages;
    return result;
  }

  ListStoreReviewsRequest._();

  factory ListStoreReviewsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreReviewsRequest()..mergeFromBuffer(data, registry);
  factory ListStoreReviewsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreReviewsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreReviewsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListStoreReviewsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..aI(4, _omitFieldNames ? '' : 'rating')
    ..aOB(5, _omitFieldNames ? '' : 'withImages')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreReviewsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreReviewsRequest copyWith(
          void Function(ListStoreReviewsRequest) updates) =>
      super.copyWith((message) => updates(message as ListStoreReviewsRequest))
          as ListStoreReviewsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreReviewsRequest() / ListStoreReviewsRequest.new instead')
  static ListStoreReviewsRequest create() => ListStoreReviewsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListStoreReviewsRequest._();
  @$core.override
  ListStoreReviewsRequest createEmptyInstance() => ListStoreReviewsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListStoreReviewsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreReviewsRequest>(
          ListStoreReviewsRequest.$_createMessage);
  static ListStoreReviewsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 預設 20，上限 100。
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

  /// 只列這個星等（1 到 5）；不設是不篩選。
  @$pb.TagNumber(4)
  $core.int get rating => $_getIZ(3);
  @$pb.TagNumber(4)
  set rating($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRating() => $_has(3);
  @$pb.TagNumber(4)
  void clearRating() => $_clearField(4);

  /// 為真時只列有附圖的評價。
  @$pb.TagNumber(5)
  $core.bool get withImages => $_getBF(4);
  @$pb.TagNumber(5)
  set withImages($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasWithImages() => $_has(4);
  @$pb.TagNumber(5)
  void clearWithImages() => $_clearField(5);
}

class ListStoreReviewsResponse extends $pb.GeneratedMessage {
  factory ListStoreReviewsResponse({
    $core.Iterable<Review>? reviews,
    $core.String? nextPageToken,
  }) {
    final result = ListStoreReviewsResponse._();
    if (reviews != null) result.reviews.addAll(reviews);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListStoreReviewsResponse._();

  factory ListStoreReviewsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreReviewsResponse()..mergeFromBuffer(data, registry);
  factory ListStoreReviewsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreReviewsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreReviewsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListStoreReviewsResponse.$_createMessage)
    ..pPM<Review>(1, _omitFieldNames ? '' : 'reviews',
        subBuilder: Review.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreReviewsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreReviewsResponse copyWith(
          void Function(ListStoreReviewsResponse) updates) =>
      super.copyWith((message) => updates(message as ListStoreReviewsResponse))
          as ListStoreReviewsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreReviewsResponse() / ListStoreReviewsResponse.new instead')
  static ListStoreReviewsResponse create() => ListStoreReviewsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListStoreReviewsResponse._();
  @$core.override
  ListStoreReviewsResponse createEmptyInstance() =>
      ListStoreReviewsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListStoreReviewsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreReviewsResponse>(
          ListStoreReviewsResponse.$_createMessage);
  static ListStoreReviewsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Review> get reviews => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetStoreReviewStatsRequest extends $pb.GeneratedMessage {
  factory GetStoreReviewStatsRequest({
    $core.String? storeId,
  }) {
    final result = GetStoreReviewStatsRequest._();
    if (storeId != null) result.storeId = storeId;
    return result;
  }

  GetStoreReviewStatsRequest._();

  factory GetStoreReviewStatsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreReviewStatsRequest()..mergeFromBuffer(data, registry);
  factory GetStoreReviewStatsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreReviewStatsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreReviewStatsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: GetStoreReviewStatsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreReviewStatsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreReviewStatsRequest copyWith(
          void Function(GetStoreReviewStatsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetStoreReviewStatsRequest))
          as GetStoreReviewStatsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreReviewStatsRequest() / GetStoreReviewStatsRequest.new instead')
  static GetStoreReviewStatsRequest create() => GetStoreReviewStatsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetStoreReviewStatsRequest._();
  @$core.override
  GetStoreReviewStatsRequest createEmptyInstance() =>
      GetStoreReviewStatsRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStoreReviewStatsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreReviewStatsRequest>(
          GetStoreReviewStatsRequest.$_createMessage);
  static GetStoreReviewStatsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);
}

/// RatingCount 是一個星等的評價數。
class RatingCount extends $pb.GeneratedMessage {
  factory RatingCount({
    $core.int? rating,
    $core.int? count,
  }) {
    final result = RatingCount._();
    if (rating != null) result.rating = rating;
    if (count != null) result.count = count;
    return result;
  }

  RatingCount._();

  factory RatingCount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RatingCount()..mergeFromBuffer(data, registry);
  factory RatingCount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RatingCount()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RatingCount',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: RatingCount.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'rating')
    ..aI(2, _omitFieldNames ? '' : 'count')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RatingCount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RatingCount copyWith(void Function(RatingCount) updates) =>
      super.copyWith((message) => updates(message as RatingCount))
          as RatingCount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RatingCount() / RatingCount.new instead')
  static RatingCount create() => RatingCount._();
  static $pb.GeneratedMessage $_createMessage() => RatingCount._();
  @$core.override
  RatingCount createEmptyInstance() => RatingCount._();
  @$core.pragma('dart2js:noInline')
  static RatingCount getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RatingCount>(
          RatingCount.$_createMessage);
  static RatingCount? _defaultInstance;

  /// 1 到 5。
  @$pb.TagNumber(1)
  $core.int get rating => $_getIZ(0);
  @$pb.TagNumber(1)
  set rating($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRating() => $_has(0);
  @$pb.TagNumber(1)
  void clearRating() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get count => $_getIZ(1);
  @$pb.TagNumber(2)
  set count($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);
}

class GetStoreReviewStatsResponse extends $pb.GeneratedMessage {
  factory GetStoreReviewStatsResponse({
    $core.int? count,
    $core.double? average,
    $core.Iterable<RatingCount>? distribution,
  }) {
    final result = GetStoreReviewStatsResponse._();
    if (count != null) result.count = count;
    if (average != null) result.average = average;
    if (distribution != null) result.distribution.addAll(distribution);
    return result;
  }

  GetStoreReviewStatsResponse._();

  factory GetStoreReviewStatsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreReviewStatsResponse()..mergeFromBuffer(data, registry);
  factory GetStoreReviewStatsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreReviewStatsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreReviewStatsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: GetStoreReviewStatsResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'count')
    ..aD(2, _omitFieldNames ? '' : 'average')
    ..pPM<RatingCount>(3, _omitFieldNames ? '' : 'distribution',
        subBuilder: RatingCount.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreReviewStatsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreReviewStatsResponse copyWith(
          void Function(GetStoreReviewStatsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetStoreReviewStatsResponse))
          as GetStoreReviewStatsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreReviewStatsResponse() / GetStoreReviewStatsResponse.new instead')
  static GetStoreReviewStatsResponse create() =>
      GetStoreReviewStatsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetStoreReviewStatsResponse._();
  @$core.override
  GetStoreReviewStatsResponse createEmptyInstance() =>
      GetStoreReviewStatsResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStoreReviewStatsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreReviewStatsResponse>(
          GetStoreReviewStatsResponse.$_createMessage);
  static GetStoreReviewStatsResponse? _defaultInstance;

  /// 公開可讀的評價數。
  @$pb.TagNumber(1)
  $core.int get count => $_getIZ(0);
  @$pb.TagNumber(1)
  set count($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearCount() => $_clearField(1);

  /// 平均星等，四捨五入到小數第二位；沒有評價是 0。
  @$pb.TagNumber(2)
  $core.double get average => $_getN(1);
  @$pb.TagNumber(2)
  set average($core.double value) => $_setDouble(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAverage() => $_has(1);
  @$pb.TagNumber(2)
  void clearAverage() => $_clearField(2);

  /// 1 到 5 星各一項，沒有評價的星等是 0。
  @$pb.TagNumber(3)
  $pb.PbList<RatingCount> get distribution => $_getList(2);
}

class GetProductReviewStatsRequest extends $pb.GeneratedMessage {
  factory GetProductReviewStatsRequest({
    $core.String? productId,
  }) {
    final result = GetProductReviewStatsRequest._();
    if (productId != null) result.productId = productId;
    return result;
  }

  GetProductReviewStatsRequest._();

  factory GetProductReviewStatsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductReviewStatsRequest()..mergeFromBuffer(data, registry);
  factory GetProductReviewStatsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductReviewStatsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProductReviewStatsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: GetProductReviewStatsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'productId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductReviewStatsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductReviewStatsRequest copyWith(
          void Function(GetProductReviewStatsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetProductReviewStatsRequest))
          as GetProductReviewStatsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetProductReviewStatsRequest() / GetProductReviewStatsRequest.new instead')
  static GetProductReviewStatsRequest create() =>
      GetProductReviewStatsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetProductReviewStatsRequest._();
  @$core.override
  GetProductReviewStatsRequest createEmptyInstance() =>
      GetProductReviewStatsRequest._();
  @$core.pragma('dart2js:noInline')
  static GetProductReviewStatsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProductReviewStatsRequest>(
          GetProductReviewStatsRequest.$_createMessage);
  static GetProductReviewStatsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get productId => $_getSZ(0);
  @$pb.TagNumber(1)
  set productId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProductId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProductId() => $_clearField(1);
}

class GetProductReviewStatsResponse extends $pb.GeneratedMessage {
  factory GetProductReviewStatsResponse({
    $core.int? count,
    $core.double? average,
    $core.Iterable<RatingCount>? distribution,
    $core.int? withImagesCount,
  }) {
    final result = GetProductReviewStatsResponse._();
    if (count != null) result.count = count;
    if (average != null) result.average = average;
    if (distribution != null) result.distribution.addAll(distribution);
    if (withImagesCount != null) result.withImagesCount = withImagesCount;
    return result;
  }

  GetProductReviewStatsResponse._();

  factory GetProductReviewStatsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductReviewStatsResponse()..mergeFromBuffer(data, registry);
  factory GetProductReviewStatsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetProductReviewStatsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProductReviewStatsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: GetProductReviewStatsResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'count')
    ..aD(2, _omitFieldNames ? '' : 'average')
    ..pPM<RatingCount>(3, _omitFieldNames ? '' : 'distribution',
        subBuilder: RatingCount.$_createMessage)
    ..aI(4, _omitFieldNames ? '' : 'withImagesCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductReviewStatsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProductReviewStatsResponse copyWith(
          void Function(GetProductReviewStatsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetProductReviewStatsResponse))
          as GetProductReviewStatsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetProductReviewStatsResponse() / GetProductReviewStatsResponse.new instead')
  static GetProductReviewStatsResponse create() =>
      GetProductReviewStatsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetProductReviewStatsResponse._();
  @$core.override
  GetProductReviewStatsResponse createEmptyInstance() =>
      GetProductReviewStatsResponse._();
  @$core.pragma('dart2js:noInline')
  static GetProductReviewStatsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProductReviewStatsResponse>(
          GetProductReviewStatsResponse.$_createMessage);
  static GetProductReviewStatsResponse? _defaultInstance;

  /// 公開可讀的評價數。
  @$pb.TagNumber(1)
  $core.int get count => $_getIZ(0);
  @$pb.TagNumber(1)
  set count($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearCount() => $_clearField(1);

  /// 平均星等，四捨五入到小數第二位；沒有評價是 0。
  @$pb.TagNumber(2)
  $core.double get average => $_getN(1);
  @$pb.TagNumber(2)
  set average($core.double value) => $_setDouble(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAverage() => $_has(1);
  @$pb.TagNumber(2)
  void clearAverage() => $_clearField(2);

  /// 1 到 5 星各一項，沒有評價的星等是 0；與店家統計同形。
  @$pb.TagNumber(3)
  $pb.PbList<RatingCount> get distribution => $_getList(2);

  /// 公開可讀且帶圖的評價數（與 ListProductReviews 的 with_images 篩選同一個條件）。
  @$pb.TagNumber(4)
  $core.int get withImagesCount => $_getIZ(3);
  @$pb.TagNumber(4)
  set withImagesCount($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWithImagesCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearWithImagesCount() => $_clearField(4);
}

class GetReviewRequest extends $pb.GeneratedMessage {
  factory GetReviewRequest({
    $core.String? reviewId,
  }) {
    final result = GetReviewRequest._();
    if (reviewId != null) result.reviewId = reviewId;
    return result;
  }

  GetReviewRequest._();

  factory GetReviewRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetReviewRequest()..mergeFromBuffer(data, registry);
  factory GetReviewRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetReviewRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetReviewRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: GetReviewRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reviewId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReviewRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReviewRequest copyWith(void Function(GetReviewRequest) updates) =>
      super.copyWith((message) => updates(message as GetReviewRequest))
          as GetReviewRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetReviewRequest() / GetReviewRequest.new instead')
  static GetReviewRequest create() => GetReviewRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetReviewRequest._();
  @$core.override
  GetReviewRequest createEmptyInstance() => GetReviewRequest._();
  @$core.pragma('dart2js:noInline')
  static GetReviewRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetReviewRequest>(
          GetReviewRequest.$_createMessage);
  static GetReviewRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reviewId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reviewId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReviewId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReviewId() => $_clearField(1);
}

class GetReviewResponse extends $pb.GeneratedMessage {
  factory GetReviewResponse({
    Review? review,
  }) {
    final result = GetReviewResponse._();
    if (review != null) result.review = review;
    return result;
  }

  GetReviewResponse._();

  factory GetReviewResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetReviewResponse()..mergeFromBuffer(data, registry);
  factory GetReviewResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetReviewResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetReviewResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: GetReviewResponse.$_createMessage)
    ..aOM<Review>(1, _omitFieldNames ? '' : 'review',
        subBuilder: Review.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReviewResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReviewResponse copyWith(void Function(GetReviewResponse) updates) =>
      super.copyWith((message) => updates(message as GetReviewResponse))
          as GetReviewResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetReviewResponse() / GetReviewResponse.new instead')
  static GetReviewResponse create() => GetReviewResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetReviewResponse._();
  @$core.override
  GetReviewResponse createEmptyInstance() => GetReviewResponse._();
  @$core.pragma('dart2js:noInline')
  static GetReviewResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetReviewResponse>(
          GetReviewResponse.$_createMessage);
  static GetReviewResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Review get review => $_getN(0);
  @$pb.TagNumber(1)
  set review(Review value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReview() => $_has(0);
  @$pb.TagNumber(1)
  void clearReview() => $_clearField(1);
  @$pb.TagNumber(1)
  Review ensureReview() => $_ensure(0);
}

class ReplyReviewRequest extends $pb.GeneratedMessage {
  factory ReplyReviewRequest({
    $core.String? reviewId,
    $core.String? content,
  }) {
    final result = ReplyReviewRequest._();
    if (reviewId != null) result.reviewId = reviewId;
    if (content != null) result.content = content;
    return result;
  }

  ReplyReviewRequest._();

  factory ReplyReviewRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReplyReviewRequest()..mergeFromBuffer(data, registry);
  factory ReplyReviewRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReplyReviewRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReplyReviewRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ReplyReviewRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reviewId')
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyReviewRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyReviewRequest copyWith(void Function(ReplyReviewRequest) updates) =>
      super.copyWith((message) => updates(message as ReplyReviewRequest))
          as ReplyReviewRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReplyReviewRequest() / ReplyReviewRequest.new instead')
  static ReplyReviewRequest create() => ReplyReviewRequest._();
  static $pb.GeneratedMessage $_createMessage() => ReplyReviewRequest._();
  @$core.override
  ReplyReviewRequest createEmptyInstance() => ReplyReviewRequest._();
  @$core.pragma('dart2js:noInline')
  static ReplyReviewRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReplyReviewRequest>(
          ReplyReviewRequest.$_createMessage);
  static ReplyReviewRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reviewId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reviewId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReviewId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReviewId() => $_clearField(1);

  /// 回覆內文，去除首尾空白後 1 到 1000 個字元（以 Unicode 字元計）。
  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);
}

class ReplyReviewResponse extends $pb.GeneratedMessage {
  factory ReplyReviewResponse({
    Review? review,
  }) {
    final result = ReplyReviewResponse._();
    if (review != null) result.review = review;
    return result;
  }

  ReplyReviewResponse._();

  factory ReplyReviewResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReplyReviewResponse()..mergeFromBuffer(data, registry);
  factory ReplyReviewResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReplyReviewResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReplyReviewResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ReplyReviewResponse.$_createMessage)
    ..aOM<Review>(1, _omitFieldNames ? '' : 'review',
        subBuilder: Review.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyReviewResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyReviewResponse copyWith(void Function(ReplyReviewResponse) updates) =>
      super.copyWith((message) => updates(message as ReplyReviewResponse))
          as ReplyReviewResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ReplyReviewResponse() / ReplyReviewResponse.new instead')
  static ReplyReviewResponse create() => ReplyReviewResponse._();
  static $pb.GeneratedMessage $_createMessage() => ReplyReviewResponse._();
  @$core.override
  ReplyReviewResponse createEmptyInstance() => ReplyReviewResponse._();
  @$core.pragma('dart2js:noInline')
  static ReplyReviewResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReplyReviewResponse>(
          ReplyReviewResponse.$_createMessage);
  static ReplyReviewResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Review get review => $_getN(0);
  @$pb.TagNumber(1)
  set review(Review value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReview() => $_has(0);
  @$pb.TagNumber(1)
  void clearReview() => $_clearField(1);
  @$pb.TagNumber(1)
  Review ensureReview() => $_ensure(0);
}

class CreateReviewRequest extends $pb.GeneratedMessage {
  factory CreateReviewRequest({
    $core.String? orderNo,
    $core.String? orderItemId,
    $core.int? rating,
    $core.String? content,
    $core.Iterable<$core.String>? imageStagedTokens,
  }) {
    final result = CreateReviewRequest._();
    if (orderNo != null) result.orderNo = orderNo;
    if (orderItemId != null) result.orderItemId = orderItemId;
    if (rating != null) result.rating = rating;
    if (content != null) result.content = content;
    if (imageStagedTokens != null)
      result.imageStagedTokens.addAll(imageStagedTokens);
    return result;
  }

  CreateReviewRequest._();

  factory CreateReviewRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReviewRequest()..mergeFromBuffer(data, registry);
  factory CreateReviewRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReviewRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateReviewRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: CreateReviewRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'orderNo')
    ..aOS(2, _omitFieldNames ? '' : 'orderItemId')
    ..aI(3, _omitFieldNames ? '' : 'rating')
    ..aOS(4, _omitFieldNames ? '' : 'content')
    ..pPS(5, _omitFieldNames ? '' : 'imageStagedTokens')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReviewRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReviewRequest copyWith(void Function(CreateReviewRequest) updates) =>
      super.copyWith((message) => updates(message as CreateReviewRequest))
          as CreateReviewRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use CreateReviewRequest() / CreateReviewRequest.new instead')
  static CreateReviewRequest create() => CreateReviewRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateReviewRequest._();
  @$core.override
  CreateReviewRequest createEmptyInstance() => CreateReviewRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateReviewRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateReviewRequest>(
          CreateReviewRequest.$_createMessage);
  static CreateReviewRequest? _defaultInstance;

  /// 訂單編號。
  @$pb.TagNumber(1)
  $core.String get orderNo => $_getSZ(0);
  @$pb.TagNumber(1)
  set orderNo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOrderNo() => $_has(0);
  @$pb.TagNumber(1)
  void clearOrderNo() => $_clearField(1);

  /// 要評價的訂單項目 ID，必須屬於這張訂單。
  @$pb.TagNumber(2)
  $core.String get orderItemId => $_getSZ(1);
  @$pb.TagNumber(2)
  set orderItemId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOrderItemId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOrderItemId() => $_clearField(2);

  /// 1 到 5。
  @$pb.TagNumber(3)
  $core.int get rating => $_getIZ(2);
  @$pb.TagNumber(3)
  set rating($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRating() => $_has(2);
  @$pb.TagNumber(3)
  void clearRating() => $_clearField(3);

  /// 選填的評價內文，去除首尾空白後最多 1000 個字元（以 Unicode 字元計）。
  @$pb.TagNumber(4)
  $core.String get content => $_getSZ(3);
  @$pb.TagNumber(4)
  set content($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasContent() => $_has(3);
  @$pb.TagNumber(4)
  void clearContent() => $_clearField(4);

  /// 附圖的 staged token（media 上傳完成後拿到的），最多 4 個，不能重複。
  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get imageStagedTokens => $_getList(4);
}

class CreateReviewResponse extends $pb.GeneratedMessage {
  factory CreateReviewResponse({
    Review? review,
  }) {
    final result = CreateReviewResponse._();
    if (review != null) result.review = review;
    return result;
  }

  CreateReviewResponse._();

  factory CreateReviewResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReviewResponse()..mergeFromBuffer(data, registry);
  factory CreateReviewResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateReviewResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateReviewResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: CreateReviewResponse.$_createMessage)
    ..aOM<Review>(1, _omitFieldNames ? '' : 'review',
        subBuilder: Review.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReviewResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReviewResponse copyWith(void Function(CreateReviewResponse) updates) =>
      super.copyWith((message) => updates(message as CreateReviewResponse))
          as CreateReviewResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateReviewResponse() / CreateReviewResponse.new instead')
  static CreateReviewResponse create() => CreateReviewResponse._();
  static $pb.GeneratedMessage $_createMessage() => CreateReviewResponse._();
  @$core.override
  CreateReviewResponse createEmptyInstance() => CreateReviewResponse._();
  @$core.pragma('dart2js:noInline')
  static CreateReviewResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateReviewResponse>(
          CreateReviewResponse.$_createMessage);
  static CreateReviewResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Review get review => $_getN(0);
  @$pb.TagNumber(1)
  set review(Review value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReview() => $_has(0);
  @$pb.TagNumber(1)
  void clearReview() => $_clearField(1);
  @$pb.TagNumber(1)
  Review ensureReview() => $_ensure(0);
}

class ListMyReviewsRequest extends $pb.GeneratedMessage {
  factory ListMyReviewsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $core.String? orderItemId,
  }) {
    final result = ListMyReviewsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (orderItemId != null) result.orderItemId = orderItemId;
    return result;
  }

  ListMyReviewsRequest._();

  factory ListMyReviewsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReviewsRequest()..mergeFromBuffer(data, registry);
  factory ListMyReviewsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReviewsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyReviewsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListMyReviewsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aOS(3, _omitFieldNames ? '' : 'orderItemId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReviewsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReviewsRequest copyWith(void Function(ListMyReviewsRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyReviewsRequest))
          as ListMyReviewsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyReviewsRequest() / ListMyReviewsRequest.new instead')
  static ListMyReviewsRequest create() => ListMyReviewsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMyReviewsRequest._();
  @$core.override
  ListMyReviewsRequest createEmptyInstance() => ListMyReviewsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMyReviewsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyReviewsRequest>(
          ListMyReviewsRequest.$_createMessage);
  static ListMyReviewsRequest? _defaultInstance;

  /// 預設 20，上限 100。
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

  /// 只列這個訂單項目的評價；不設是全部。
  @$pb.TagNumber(3)
  $core.String get orderItemId => $_getSZ(2);
  @$pb.TagNumber(3)
  set orderItemId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOrderItemId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOrderItemId() => $_clearField(3);
}

class ListMyReviewsResponse extends $pb.GeneratedMessage {
  factory ListMyReviewsResponse({
    $core.Iterable<Review>? reviews,
    $core.String? nextPageToken,
  }) {
    final result = ListMyReviewsResponse._();
    if (reviews != null) result.reviews.addAll(reviews);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListMyReviewsResponse._();

  factory ListMyReviewsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReviewsResponse()..mergeFromBuffer(data, registry);
  factory ListMyReviewsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMyReviewsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyReviewsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListMyReviewsResponse.$_createMessage)
    ..pPM<Review>(1, _omitFieldNames ? '' : 'reviews',
        subBuilder: Review.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReviewsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyReviewsResponse copyWith(
          void Function(ListMyReviewsResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyReviewsResponse))
          as ListMyReviewsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMyReviewsResponse() / ListMyReviewsResponse.new instead')
  static ListMyReviewsResponse create() => ListMyReviewsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMyReviewsResponse._();
  @$core.override
  ListMyReviewsResponse createEmptyInstance() => ListMyReviewsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMyReviewsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyReviewsResponse>(
          ListMyReviewsResponse.$_createMessage);
  static ListMyReviewsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Review> get reviews => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListOwnStoreReviewsRequest extends $pb.GeneratedMessage {
  factory ListOwnStoreReviewsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
    $core.int? rating,
    $core.bool? withImages,
  }) {
    final result = ListOwnStoreReviewsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (rating != null) result.rating = rating;
    if (withImages != null) result.withImages = withImages;
    return result;
  }

  ListOwnStoreReviewsRequest._();

  factory ListOwnStoreReviewsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListOwnStoreReviewsRequest()..mergeFromBuffer(data, registry);
  factory ListOwnStoreReviewsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListOwnStoreReviewsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListOwnStoreReviewsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListOwnStoreReviewsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..aI(3, _omitFieldNames ? '' : 'rating')
    ..aOB(4, _omitFieldNames ? '' : 'withImages')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListOwnStoreReviewsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListOwnStoreReviewsRequest copyWith(
          void Function(ListOwnStoreReviewsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListOwnStoreReviewsRequest))
          as ListOwnStoreReviewsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListOwnStoreReviewsRequest() / ListOwnStoreReviewsRequest.new instead')
  static ListOwnStoreReviewsRequest create() => ListOwnStoreReviewsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListOwnStoreReviewsRequest._();
  @$core.override
  ListOwnStoreReviewsRequest createEmptyInstance() =>
      ListOwnStoreReviewsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListOwnStoreReviewsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListOwnStoreReviewsRequest>(
          ListOwnStoreReviewsRequest.$_createMessage);
  static ListOwnStoreReviewsRequest? _defaultInstance;

  /// 預設 20，上限 100。
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

  /// 只列這個星等（1 到 5）；不設是不篩選。
  @$pb.TagNumber(3)
  $core.int get rating => $_getIZ(2);
  @$pb.TagNumber(3)
  set rating($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRating() => $_has(2);
  @$pb.TagNumber(3)
  void clearRating() => $_clearField(3);

  /// 為真時只列有附圖的評價。
  @$pb.TagNumber(4)
  $core.bool get withImages => $_getBF(3);
  @$pb.TagNumber(4)
  set withImages($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWithImages() => $_has(3);
  @$pb.TagNumber(4)
  void clearWithImages() => $_clearField(4);
}

class ListOwnStoreReviewsResponse extends $pb.GeneratedMessage {
  factory ListOwnStoreReviewsResponse({
    $core.Iterable<Review>? reviews,
    $core.String? nextPageToken,
  }) {
    final result = ListOwnStoreReviewsResponse._();
    if (reviews != null) result.reviews.addAll(reviews);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListOwnStoreReviewsResponse._();

  factory ListOwnStoreReviewsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListOwnStoreReviewsResponse()..mergeFromBuffer(data, registry);
  factory ListOwnStoreReviewsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListOwnStoreReviewsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListOwnStoreReviewsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'hawkr.review.v1alpha1'),
      createEmptyInstance: ListOwnStoreReviewsResponse.$_createMessage)
    ..pPM<Review>(1, _omitFieldNames ? '' : 'reviews',
        subBuilder: Review.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListOwnStoreReviewsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListOwnStoreReviewsResponse copyWith(
          void Function(ListOwnStoreReviewsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListOwnStoreReviewsResponse))
          as ListOwnStoreReviewsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListOwnStoreReviewsResponse() / ListOwnStoreReviewsResponse.new instead')
  static ListOwnStoreReviewsResponse create() =>
      ListOwnStoreReviewsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListOwnStoreReviewsResponse._();
  @$core.override
  ListOwnStoreReviewsResponse createEmptyInstance() =>
      ListOwnStoreReviewsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListOwnStoreReviewsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListOwnStoreReviewsResponse>(
          ListOwnStoreReviewsResponse.$_createMessage);
  static ListOwnStoreReviewsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Review> get reviews => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

/// ReviewPublicService 是任何人都能讀的評價與統計，不需要登入。
class ReviewPublicServiceApi {
  final $pb.RpcClient _client;

  ReviewPublicServiceApi(this._client);

  /// ListProductReviews 列出商品的公開評價，新到舊（建立時間，相同時以 ID 排序）。公開可讀：沒有未還原的平台隱藏、作者仍在營運中。
  /// 帶登入憑證時套用觀看者的封鎖，過濾在分頁之前，所以一頁可以少於 page_size，結束只看 next_page_token。可以用 rating 與 with_images 篩選；
  /// page_token 只能用在同一組 product_id 與篩選。商品沒有評價是空的一頁。
  $async.Future<ListProductReviewsResponse> listProductReviews(
          $pb.ClientContext? ctx, ListProductReviewsRequest request) =>
      _client.invoke<ListProductReviewsResponse>(ctx, 'ReviewPublicService',
          'ListProductReviews', request, ListProductReviewsResponse());

  /// ListStoreReviews 列出店家的公開評價，規則同 ListProductReviews。店家自己讀自己店的評價用 ReviewAppService.ListOwnStoreReviews（不套觀看者）。
  $async.Future<ListStoreReviewsResponse> listStoreReviews(
          $pb.ClientContext? ctx, ListStoreReviewsRequest request) =>
      _client.invoke<ListStoreReviewsResponse>(ctx, 'ReviewPublicService',
          'ListStoreReviews', request, ListStoreReviewsResponse());

  /// GetStoreReviewStats 是店家的評價統計：公開可讀的評價數、平均與各星等的數量。統計不依觀看者過濾，公開與店家自己看到的數字相同。
  $async.Future<GetStoreReviewStatsResponse> getStoreReviewStats(
          $pb.ClientContext? ctx, GetStoreReviewStatsRequest request) =>
      _client.invoke<GetStoreReviewStatsResponse>(ctx, 'ReviewPublicService',
          'GetStoreReviewStats', request, GetStoreReviewStatsResponse());

  /// GetProductReviewStats 是商品的評價統計：公開可讀的評價數與平均（與 ListProductReviews 同一個公開述詞：沒有未還原的平台隱藏、作者仍在營運中）。
  /// 統計不依觀看者過濾，不套封鎖。商品沒有評價（或商品不存在）是 count 0、average 0，不是 not_found。
  $async.Future<GetProductReviewStatsResponse> getProductReviewStats(
          $pb.ClientContext? ctx, GetProductReviewStatsRequest request) =>
      _client.invoke<GetProductReviewStatsResponse>(ctx, 'ReviewPublicService',
          'GetProductReviewStats', request, GetProductReviewStatsResponse());
}

/// ReviewAppService 是登入者的評價。評價是買家與店家共用的資源（買家寫、店家回）：CreateReview 與 ListMyReviews 是買家；
/// ListOwnStoreReviews 的店家身分由伺服器從 session 推導（呼叫者此刻的店），ReplyReview 的店家由評價所屬的店推導，請求都不帶 store_id。
class ReviewAppServiceApi {
  final $pb.RpcClient _client;

  ReviewAppServiceApi(this._client);

  /// GetReview 讀一則公開可讀的評價，套用觀看者的封鎖；其他一律回 not_found，不區分原因。
  $async.Future<GetReviewResponse> getReview(
          $pb.ClientContext? ctx, GetReviewRequest request) =>
      _client.invoke<GetReviewResponse>(
          ctx, 'ReviewAppService', 'GetReview', request, GetReviewResponse());

  /// ReplyReview 回覆一則評價，已有回覆時覆蓋；店家由評價所屬的店推導，請求不帶 store_id。需要賣家能力 CAP_MARKETING。評價不存在回 not_found。呼叫者不是那間店的店主（包含沒有店）時：評價公開可讀回 permission_denied，
  /// 評價不公開（被隱藏、作者已離開）回 not_found，不洩漏存在性。店主回覆此刻不能回覆的評價（被隱藏、作者已離開、回覆被下架）回 failed_precondition 與 REVIEW_NOT_REPLYABLE。
  $async.Future<ReplyReviewResponse> replyReview(
          $pb.ClientContext? ctx, ReplyReviewRequest request) =>
      _client.invoke<ReplyReviewResponse>(ctx, 'ReviewAppService',
          'ReplyReview', request, ReplyReviewResponse());

  /// CreateReview 以呼叫者的買家身分對一個已完成的訂單項目寫評價。商品、規格、價格與商品圖取自訂單項目。訂單不是呼叫者的或不存在回 not_found；
  /// 同一個訂單項目已有評價（可能已被平台隱藏而不在 ListMyReviews 的結果中）回 already_exists 與 REVIEW_ALREADY_EXISTS，client 顯示「已評價」；訂單還不能評價回 failed_precondition 與 ORDER_NOT_REVIEWABLE；
  /// 超過評價期限回 failed_precondition 與 REVIEW_WINDOW_EXPIRED。
  $async.Future<CreateReviewResponse> createReview(
          $pb.ClientContext? ctx, CreateReviewRequest request) =>
      _client.invoke<CreateReviewResponse>(ctx, 'ReviewAppService',
          'CreateReview', request, CreateReviewResponse());

  /// ListMyReviews 列出呼叫者自己寫的評價，新到舊；被平台隱藏的不列（作者看不到被隱藏的評價）。order_item_id 只列那個訂單項目的可見評價（至多一則）；沒有結果不代表沒寫過，被隱藏的評價仍占著該訂單項目（CreateReview 會回 REVIEW_ALREADY_EXISTS）。
  /// page_token 只能用在同一個 order_item_id。
  $async.Future<ListMyReviewsResponse> listMyReviews(
          $pb.ClientContext? ctx, ListMyReviewsRequest request) =>
      _client.invoke<ListMyReviewsResponse>(ctx, 'ReviewAppService',
          'ListMyReviews', request, ListMyReviewsResponse());

  /// ListOwnStoreReviews 列出呼叫者此刻的店的公開可讀評價，新到舊，不套觀看者的封鎖（店家看得到被它封鎖的買家的評價）。呼叫者沒有店回 not_found。
  /// 不檢查賣家能力：停權的店家仍可讀自己店的評價。
  $async.Future<ListOwnStoreReviewsResponse> listOwnStoreReviews(
          $pb.ClientContext? ctx, ListOwnStoreReviewsRequest request) =>
      _client.invoke<ListOwnStoreReviewsResponse>(ctx, 'ReviewAppService',
          'ListOwnStoreReviews', request, ListOwnStoreReviewsResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
