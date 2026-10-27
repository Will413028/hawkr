// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'post_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$Post {

// ignore: invalid_annotation_target
@JsonKey(readValue: _readId) int get id;// ignore: invalid_annotation_target
@JsonKey(readValue: _readSellerId) int get sellerId; String get title; String get content; List<String> get tags; String get contentType;// 'TEXT', 'IMAGE', 'VIDEO'
/// 貼文圖片來源清單（objectKey 優先、URL 次之，見 [_readImageKeys]）。
/// 顯示前必須過 `buildImageUrl`；「這則貼文有沒有圖」一律以本清單是否為空
/// 判斷，不要另外從原始 JSON 猜欄位名。
// ignore: invalid_annotation_target
@JsonKey(readValue: _readImageKeys) List<String> get imageKeys;// ignore: invalid_annotation_target
@JsonKey(readValue: _readVideoUrl) String? get videoUrl;// ignore: invalid_annotation_target
@JsonKey(readValue: _readLikeCount) int get likeCount;// ignore: invalid_annotation_target
@JsonKey(readValue: _readCommentCount) int get commentCount;// ignore: invalid_annotation_target
@JsonKey(readValue: _readShareCount) int get shareCount; bool get isUnlocked; int? get unlockPrice;// ignore: invalid_annotation_target
@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) DateTime? get createdAt;// ignore: invalid_annotation_target
@JsonKey(readValue: _readLinkedProduct) Map<String, dynamic>? get linkedProduct;// ignore: invalid_annotation_target
@JsonKey(readValue: _readPublishStatus) String? get publishStatus;// ignore: invalid_annotation_target
@JsonKey(readValue: _readHiddenAt) int? get hiddenAt;// ignore: invalid_annotation_target
@JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp) DateTime? get scheduledPublishAt;
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCopyWith<Post> get copyWith => _$PostCopyWithImpl<Post>(this as Post, _$identity);

  /// Serializes this Post to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Post&&(identical(other.id, id) || other.id == id)&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.title, title) || other.title == title)&&(identical(other.content, content) || other.content == content)&&const DeepCollectionEquality().equals(other.tags, tags)&&(identical(other.contentType, contentType) || other.contentType == contentType)&&const DeepCollectionEquality().equals(other.imageKeys, imageKeys)&&(identical(other.videoUrl, videoUrl) || other.videoUrl == videoUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.isUnlocked, isUnlocked) || other.isUnlocked == isUnlocked)&&(identical(other.unlockPrice, unlockPrice) || other.unlockPrice == unlockPrice)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other.linkedProduct, linkedProduct)&&(identical(other.publishStatus, publishStatus) || other.publishStatus == publishStatus)&&(identical(other.hiddenAt, hiddenAt) || other.hiddenAt == hiddenAt)&&(identical(other.scheduledPublishAt, scheduledPublishAt) || other.scheduledPublishAt == scheduledPublishAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sellerId,title,content,const DeepCollectionEquality().hash(tags),contentType,const DeepCollectionEquality().hash(imageKeys),videoUrl,likeCount,commentCount,shareCount,isUnlocked,unlockPrice,createdAt,const DeepCollectionEquality().hash(linkedProduct),publishStatus,hiddenAt,scheduledPublishAt);

@override
String toString() {
  return 'Post(id: $id, sellerId: $sellerId, title: $title, content: $content, tags: $tags, contentType: $contentType, imageKeys: $imageKeys, videoUrl: $videoUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, isUnlocked: $isUnlocked, unlockPrice: $unlockPrice, createdAt: $createdAt, linkedProduct: $linkedProduct, publishStatus: $publishStatus, hiddenAt: $hiddenAt, scheduledPublishAt: $scheduledPublishAt)';
}


}

/// @nodoc
abstract mixin class $PostCopyWith<$Res>  {
  factory $PostCopyWith(Post value, $Res Function(Post) _then) = _$PostCopyWithImpl;
@useResult
$Res call({
@JsonKey(readValue: _readId) int id,@JsonKey(readValue: _readSellerId) int sellerId, String title, String content, List<String> tags, String contentType,@JsonKey(readValue: _readImageKeys) List<String> imageKeys,@JsonKey(readValue: _readVideoUrl) String? videoUrl,@JsonKey(readValue: _readLikeCount) int likeCount,@JsonKey(readValue: _readCommentCount) int commentCount,@JsonKey(readValue: _readShareCount) int shareCount, bool isUnlocked, int? unlockPrice,@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) DateTime? createdAt,@JsonKey(readValue: _readLinkedProduct) Map<String, dynamic>? linkedProduct,@JsonKey(readValue: _readPublishStatus) String? publishStatus,@JsonKey(readValue: _readHiddenAt) int? hiddenAt,@JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp) DateTime? scheduledPublishAt
});




}
/// @nodoc
class _$PostCopyWithImpl<$Res>
    implements $PostCopyWith<$Res> {
  _$PostCopyWithImpl(this._self, this._then);

  final Post _self;
  final $Res Function(Post) _then;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? sellerId = null,Object? title = null,Object? content = null,Object? tags = null,Object? contentType = null,Object? imageKeys = null,Object? videoUrl = freezed,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? isUnlocked = null,Object? unlockPrice = freezed,Object? createdAt = freezed,Object? linkedProduct = freezed,Object? publishStatus = freezed,Object? hiddenAt = freezed,Object? scheduledPublishAt = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,sellerId: null == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as int,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,tags: null == tags ? _self.tags : tags // ignore: cast_nullable_to_non_nullable
as List<String>,contentType: null == contentType ? _self.contentType : contentType // ignore: cast_nullable_to_non_nullable
as String,imageKeys: null == imageKeys ? _self.imageKeys : imageKeys // ignore: cast_nullable_to_non_nullable
as List<String>,videoUrl: freezed == videoUrl ? _self.videoUrl : videoUrl // ignore: cast_nullable_to_non_nullable
as String?,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,isUnlocked: null == isUnlocked ? _self.isUnlocked : isUnlocked // ignore: cast_nullable_to_non_nullable
as bool,unlockPrice: freezed == unlockPrice ? _self.unlockPrice : unlockPrice // ignore: cast_nullable_to_non_nullable
as int?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,linkedProduct: freezed == linkedProduct ? _self.linkedProduct : linkedProduct // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,publishStatus: freezed == publishStatus ? _self.publishStatus : publishStatus // ignore: cast_nullable_to_non_nullable
as String?,hiddenAt: freezed == hiddenAt ? _self.hiddenAt : hiddenAt // ignore: cast_nullable_to_non_nullable
as int?,scheduledPublishAt: freezed == scheduledPublishAt ? _self.scheduledPublishAt : scheduledPublishAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [Post].
extension PostPatterns on Post {
/// A variant of `map` that fallback to returning `orElse`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Post value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// Callbacks receives the raw object, upcasted.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case final Subclass2 value:
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Post value)  $default,){
final _that = this;
switch (_that) {
case _Post():
return $default(_that);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `map` that fallback to returning `null`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Post value)?  $default,){
final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that);case _:
  return null;

}
}
/// A variant of `when` that fallback to an `orElse` callback.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(readValue: _readId)  int id, @JsonKey(readValue: _readSellerId)  int sellerId,  String title,  String content,  List<String> tags,  String contentType, @JsonKey(readValue: _readImageKeys)  List<String> imageKeys, @JsonKey(readValue: _readVideoUrl)  String? videoUrl, @JsonKey(readValue: _readLikeCount)  int likeCount, @JsonKey(readValue: _readCommentCount)  int commentCount, @JsonKey(readValue: _readShareCount)  int shareCount,  bool isUnlocked,  int? unlockPrice, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)  DateTime? createdAt, @JsonKey(readValue: _readLinkedProduct)  Map<String, dynamic>? linkedProduct, @JsonKey(readValue: _readPublishStatus)  String? publishStatus, @JsonKey(readValue: _readHiddenAt)  int? hiddenAt, @JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp)  DateTime? scheduledPublishAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.sellerId,_that.title,_that.content,_that.tags,_that.contentType,_that.imageKeys,_that.videoUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.isUnlocked,_that.unlockPrice,_that.createdAt,_that.linkedProduct,_that.publishStatus,_that.hiddenAt,_that.scheduledPublishAt);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// As opposed to `map`, this offers destructuring.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case Subclass2(:final field2):
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(readValue: _readId)  int id, @JsonKey(readValue: _readSellerId)  int sellerId,  String title,  String content,  List<String> tags,  String contentType, @JsonKey(readValue: _readImageKeys)  List<String> imageKeys, @JsonKey(readValue: _readVideoUrl)  String? videoUrl, @JsonKey(readValue: _readLikeCount)  int likeCount, @JsonKey(readValue: _readCommentCount)  int commentCount, @JsonKey(readValue: _readShareCount)  int shareCount,  bool isUnlocked,  int? unlockPrice, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)  DateTime? createdAt, @JsonKey(readValue: _readLinkedProduct)  Map<String, dynamic>? linkedProduct, @JsonKey(readValue: _readPublishStatus)  String? publishStatus, @JsonKey(readValue: _readHiddenAt)  int? hiddenAt, @JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp)  DateTime? scheduledPublishAt)  $default,) {final _that = this;
switch (_that) {
case _Post():
return $default(_that.id,_that.sellerId,_that.title,_that.content,_that.tags,_that.contentType,_that.imageKeys,_that.videoUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.isUnlocked,_that.unlockPrice,_that.createdAt,_that.linkedProduct,_that.publishStatus,_that.hiddenAt,_that.scheduledPublishAt);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `when` that fallback to returning `null`
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(readValue: _readId)  int id, @JsonKey(readValue: _readSellerId)  int sellerId,  String title,  String content,  List<String> tags,  String contentType, @JsonKey(readValue: _readImageKeys)  List<String> imageKeys, @JsonKey(readValue: _readVideoUrl)  String? videoUrl, @JsonKey(readValue: _readLikeCount)  int likeCount, @JsonKey(readValue: _readCommentCount)  int commentCount, @JsonKey(readValue: _readShareCount)  int shareCount,  bool isUnlocked,  int? unlockPrice, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)  DateTime? createdAt, @JsonKey(readValue: _readLinkedProduct)  Map<String, dynamic>? linkedProduct, @JsonKey(readValue: _readPublishStatus)  String? publishStatus, @JsonKey(readValue: _readHiddenAt)  int? hiddenAt, @JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp)  DateTime? scheduledPublishAt)?  $default,) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.sellerId,_that.title,_that.content,_that.tags,_that.contentType,_that.imageKeys,_that.videoUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.isUnlocked,_that.unlockPrice,_that.createdAt,_that.linkedProduct,_that.publishStatus,_that.hiddenAt,_that.scheduledPublishAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Post implements Post {
  const _Post({@JsonKey(readValue: _readId) required this.id, @JsonKey(readValue: _readSellerId) required this.sellerId, this.title = '', this.content = '', final  List<String> tags = const <String>[], this.contentType = 'TEXT', @JsonKey(readValue: _readImageKeys) final  List<String> imageKeys = const <String>[], @JsonKey(readValue: _readVideoUrl) this.videoUrl, @JsonKey(readValue: _readLikeCount) this.likeCount = 0, @JsonKey(readValue: _readCommentCount) this.commentCount = 0, @JsonKey(readValue: _readShareCount) this.shareCount = 0, this.isUnlocked = false, this.unlockPrice, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) this.createdAt, @JsonKey(readValue: _readLinkedProduct) final  Map<String, dynamic>? linkedProduct, @JsonKey(readValue: _readPublishStatus) this.publishStatus, @JsonKey(readValue: _readHiddenAt) this.hiddenAt, @JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp) this.scheduledPublishAt}): _tags = tags,_imageKeys = imageKeys,_linkedProduct = linkedProduct;
  factory _Post.fromJson(Map<String, dynamic> json) => _$PostFromJson(json);

// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readId) final  int id;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readSellerId) final  int sellerId;
@override@JsonKey() final  String title;
@override@JsonKey() final  String content;
 final  List<String> _tags;
@override@JsonKey() List<String> get tags {
  if (_tags is EqualUnmodifiableListView) return _tags;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_tags);
}

@override@JsonKey() final  String contentType;
// 'TEXT', 'IMAGE', 'VIDEO'
/// 貼文圖片來源清單（objectKey 優先、URL 次之，見 [_readImageKeys]）。
/// 顯示前必須過 `buildImageUrl`；「這則貼文有沒有圖」一律以本清單是否為空
/// 判斷，不要另外從原始 JSON 猜欄位名。
// ignore: invalid_annotation_target
 final  List<String> _imageKeys;
// 'TEXT', 'IMAGE', 'VIDEO'
/// 貼文圖片來源清單（objectKey 優先、URL 次之，見 [_readImageKeys]）。
/// 顯示前必須過 `buildImageUrl`；「這則貼文有沒有圖」一律以本清單是否為空
/// 判斷，不要另外從原始 JSON 猜欄位名。
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readImageKeys) List<String> get imageKeys {
  if (_imageKeys is EqualUnmodifiableListView) return _imageKeys;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_imageKeys);
}

// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readVideoUrl) final  String? videoUrl;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readLikeCount) final  int likeCount;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readCommentCount) final  int commentCount;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readShareCount) final  int shareCount;
@override@JsonKey() final  bool isUnlocked;
@override final  int? unlockPrice;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) final  DateTime? createdAt;
// ignore: invalid_annotation_target
 final  Map<String, dynamic>? _linkedProduct;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readLinkedProduct) Map<String, dynamic>? get linkedProduct {
  final value = _linkedProduct;
  if (value == null) return null;
  if (_linkedProduct is EqualUnmodifiableMapView) return _linkedProduct;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}

// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readPublishStatus) final  String? publishStatus;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readHiddenAt) final  int? hiddenAt;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp) final  DateTime? scheduledPublishAt;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostCopyWith<_Post> get copyWith => __$PostCopyWithImpl<_Post>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$PostToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Post&&(identical(other.id, id) || other.id == id)&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.title, title) || other.title == title)&&(identical(other.content, content) || other.content == content)&&const DeepCollectionEquality().equals(other._tags, _tags)&&(identical(other.contentType, contentType) || other.contentType == contentType)&&const DeepCollectionEquality().equals(other._imageKeys, _imageKeys)&&(identical(other.videoUrl, videoUrl) || other.videoUrl == videoUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.isUnlocked, isUnlocked) || other.isUnlocked == isUnlocked)&&(identical(other.unlockPrice, unlockPrice) || other.unlockPrice == unlockPrice)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other._linkedProduct, _linkedProduct)&&(identical(other.publishStatus, publishStatus) || other.publishStatus == publishStatus)&&(identical(other.hiddenAt, hiddenAt) || other.hiddenAt == hiddenAt)&&(identical(other.scheduledPublishAt, scheduledPublishAt) || other.scheduledPublishAt == scheduledPublishAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sellerId,title,content,const DeepCollectionEquality().hash(_tags),contentType,const DeepCollectionEquality().hash(_imageKeys),videoUrl,likeCount,commentCount,shareCount,isUnlocked,unlockPrice,createdAt,const DeepCollectionEquality().hash(_linkedProduct),publishStatus,hiddenAt,scheduledPublishAt);

@override
String toString() {
  return 'Post(id: $id, sellerId: $sellerId, title: $title, content: $content, tags: $tags, contentType: $contentType, imageKeys: $imageKeys, videoUrl: $videoUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, isUnlocked: $isUnlocked, unlockPrice: $unlockPrice, createdAt: $createdAt, linkedProduct: $linkedProduct, publishStatus: $publishStatus, hiddenAt: $hiddenAt, scheduledPublishAt: $scheduledPublishAt)';
}


}

/// @nodoc
abstract mixin class _$PostCopyWith<$Res> implements $PostCopyWith<$Res> {
  factory _$PostCopyWith(_Post value, $Res Function(_Post) _then) = __$PostCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(readValue: _readId) int id,@JsonKey(readValue: _readSellerId) int sellerId, String title, String content, List<String> tags, String contentType,@JsonKey(readValue: _readImageKeys) List<String> imageKeys,@JsonKey(readValue: _readVideoUrl) String? videoUrl,@JsonKey(readValue: _readLikeCount) int likeCount,@JsonKey(readValue: _readCommentCount) int commentCount,@JsonKey(readValue: _readShareCount) int shareCount, bool isUnlocked, int? unlockPrice,@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) DateTime? createdAt,@JsonKey(readValue: _readLinkedProduct) Map<String, dynamic>? linkedProduct,@JsonKey(readValue: _readPublishStatus) String? publishStatus,@JsonKey(readValue: _readHiddenAt) int? hiddenAt,@JsonKey(readValue: _readScheduledPublishAt, fromJson: _parseTimestamp) DateTime? scheduledPublishAt
});




}
/// @nodoc
class __$PostCopyWithImpl<$Res>
    implements _$PostCopyWith<$Res> {
  __$PostCopyWithImpl(this._self, this._then);

  final _Post _self;
  final $Res Function(_Post) _then;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? sellerId = null,Object? title = null,Object? content = null,Object? tags = null,Object? contentType = null,Object? imageKeys = null,Object? videoUrl = freezed,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? isUnlocked = null,Object? unlockPrice = freezed,Object? createdAt = freezed,Object? linkedProduct = freezed,Object? publishStatus = freezed,Object? hiddenAt = freezed,Object? scheduledPublishAt = freezed,}) {
  return _then(_Post(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,sellerId: null == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as int,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,tags: null == tags ? _self._tags : tags // ignore: cast_nullable_to_non_nullable
as List<String>,contentType: null == contentType ? _self.contentType : contentType // ignore: cast_nullable_to_non_nullable
as String,imageKeys: null == imageKeys ? _self._imageKeys : imageKeys // ignore: cast_nullable_to_non_nullable
as List<String>,videoUrl: freezed == videoUrl ? _self.videoUrl : videoUrl // ignore: cast_nullable_to_non_nullable
as String?,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,isUnlocked: null == isUnlocked ? _self.isUnlocked : isUnlocked // ignore: cast_nullable_to_non_nullable
as bool,unlockPrice: freezed == unlockPrice ? _self.unlockPrice : unlockPrice // ignore: cast_nullable_to_non_nullable
as int?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,linkedProduct: freezed == linkedProduct ? _self._linkedProduct : linkedProduct // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,publishStatus: freezed == publishStatus ? _self.publishStatus : publishStatus // ignore: cast_nullable_to_non_nullable
as String?,hiddenAt: freezed == hiddenAt ? _self.hiddenAt : hiddenAt // ignore: cast_nullable_to_non_nullable
as int?,scheduledPublishAt: freezed == scheduledPublishAt ? _self.scheduledPublishAt : scheduledPublishAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}


/// @nodoc
mixin _$Comment {

// ignore: invalid_annotation_target
@JsonKey(readValue: _readCommentId) int get id;// ignore: invalid_annotation_target
@JsonKey(readValue: _readCommentPostId) int get postId; bool get isMine;// ignore: invalid_annotation_target
@JsonKey(readValue: _readCommentAuthor) String get author;/// objectKey 優先、URL 次之（見 [_readCommentAvatar]）；顯示前必須過
/// `buildImageUrl(..., ImagePreset.avatar)`，不要直接當 URL 用。
// ignore: invalid_annotation_target
@JsonKey(readValue: _readCommentAvatar) String get avatarKey; String get content;// ignore: invalid_annotation_target
@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) DateTime? get createdAt;
/// Create a copy of Comment
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CommentCopyWith<Comment> get copyWith => _$CommentCopyWithImpl<Comment>(this as Comment, _$identity);

  /// Serializes this Comment to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Comment&&(identical(other.id, id) || other.id == id)&&(identical(other.postId, postId) || other.postId == postId)&&(identical(other.isMine, isMine) || other.isMine == isMine)&&(identical(other.author, author) || other.author == author)&&(identical(other.avatarKey, avatarKey) || other.avatarKey == avatarKey)&&(identical(other.content, content) || other.content == content)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,postId,isMine,author,avatarKey,content,createdAt);

@override
String toString() {
  return 'Comment(id: $id, postId: $postId, isMine: $isMine, author: $author, avatarKey: $avatarKey, content: $content, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $CommentCopyWith<$Res>  {
  factory $CommentCopyWith(Comment value, $Res Function(Comment) _then) = _$CommentCopyWithImpl;
@useResult
$Res call({
@JsonKey(readValue: _readCommentId) int id,@JsonKey(readValue: _readCommentPostId) int postId, bool isMine,@JsonKey(readValue: _readCommentAuthor) String author,@JsonKey(readValue: _readCommentAvatar) String avatarKey, String content,@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) DateTime? createdAt
});




}
/// @nodoc
class _$CommentCopyWithImpl<$Res>
    implements $CommentCopyWith<$Res> {
  _$CommentCopyWithImpl(this._self, this._then);

  final Comment _self;
  final $Res Function(Comment) _then;

/// Create a copy of Comment
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? postId = null,Object? isMine = null,Object? author = null,Object? avatarKey = null,Object? content = null,Object? createdAt = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,postId: null == postId ? _self.postId : postId // ignore: cast_nullable_to_non_nullable
as int,isMine: null == isMine ? _self.isMine : isMine // ignore: cast_nullable_to_non_nullable
as bool,author: null == author ? _self.author : author // ignore: cast_nullable_to_non_nullable
as String,avatarKey: null == avatarKey ? _self.avatarKey : avatarKey // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [Comment].
extension CommentPatterns on Comment {
/// A variant of `map` that fallback to returning `orElse`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Comment value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Comment() when $default != null:
return $default(_that);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// Callbacks receives the raw object, upcasted.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case final Subclass2 value:
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Comment value)  $default,){
final _that = this;
switch (_that) {
case _Comment():
return $default(_that);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `map` that fallback to returning `null`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Comment value)?  $default,){
final _that = this;
switch (_that) {
case _Comment() when $default != null:
return $default(_that);case _:
  return null;

}
}
/// A variant of `when` that fallback to an `orElse` callback.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(readValue: _readCommentId)  int id, @JsonKey(readValue: _readCommentPostId)  int postId,  bool isMine, @JsonKey(readValue: _readCommentAuthor)  String author, @JsonKey(readValue: _readCommentAvatar)  String avatarKey,  String content, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)  DateTime? createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Comment() when $default != null:
return $default(_that.id,_that.postId,_that.isMine,_that.author,_that.avatarKey,_that.content,_that.createdAt);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// As opposed to `map`, this offers destructuring.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case Subclass2(:final field2):
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(readValue: _readCommentId)  int id, @JsonKey(readValue: _readCommentPostId)  int postId,  bool isMine, @JsonKey(readValue: _readCommentAuthor)  String author, @JsonKey(readValue: _readCommentAvatar)  String avatarKey,  String content, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)  DateTime? createdAt)  $default,) {final _that = this;
switch (_that) {
case _Comment():
return $default(_that.id,_that.postId,_that.isMine,_that.author,_that.avatarKey,_that.content,_that.createdAt);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `when` that fallback to returning `null`
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(readValue: _readCommentId)  int id, @JsonKey(readValue: _readCommentPostId)  int postId,  bool isMine, @JsonKey(readValue: _readCommentAuthor)  String author, @JsonKey(readValue: _readCommentAvatar)  String avatarKey,  String content, @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp)  DateTime? createdAt)?  $default,) {final _that = this;
switch (_that) {
case _Comment() when $default != null:
return $default(_that.id,_that.postId,_that.isMine,_that.author,_that.avatarKey,_that.content,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Comment implements Comment {
  const _Comment({@JsonKey(readValue: _readCommentId) required this.id, @JsonKey(readValue: _readCommentPostId) this.postId = 0, this.isMine = false, @JsonKey(readValue: _readCommentAuthor) this.author = '用戶', @JsonKey(readValue: _readCommentAvatar) this.avatarKey = '', this.content = '', @JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) this.createdAt});
  factory _Comment.fromJson(Map<String, dynamic> json) => _$CommentFromJson(json);

// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readCommentId) final  int id;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readCommentPostId) final  int postId;
@override@JsonKey() final  bool isMine;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readCommentAuthor) final  String author;
/// objectKey 優先、URL 次之（見 [_readCommentAvatar]）；顯示前必須過
/// `buildImageUrl(..., ImagePreset.avatar)`，不要直接當 URL 用。
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readCommentAvatar) final  String avatarKey;
@override@JsonKey() final  String content;
// ignore: invalid_annotation_target
@override@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) final  DateTime? createdAt;

/// Create a copy of Comment
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CommentCopyWith<_Comment> get copyWith => __$CommentCopyWithImpl<_Comment>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CommentToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Comment&&(identical(other.id, id) || other.id == id)&&(identical(other.postId, postId) || other.postId == postId)&&(identical(other.isMine, isMine) || other.isMine == isMine)&&(identical(other.author, author) || other.author == author)&&(identical(other.avatarKey, avatarKey) || other.avatarKey == avatarKey)&&(identical(other.content, content) || other.content == content)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,postId,isMine,author,avatarKey,content,createdAt);

@override
String toString() {
  return 'Comment(id: $id, postId: $postId, isMine: $isMine, author: $author, avatarKey: $avatarKey, content: $content, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$CommentCopyWith<$Res> implements $CommentCopyWith<$Res> {
  factory _$CommentCopyWith(_Comment value, $Res Function(_Comment) _then) = __$CommentCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(readValue: _readCommentId) int id,@JsonKey(readValue: _readCommentPostId) int postId, bool isMine,@JsonKey(readValue: _readCommentAuthor) String author,@JsonKey(readValue: _readCommentAvatar) String avatarKey, String content,@JsonKey(readValue: _readTimestamp, fromJson: _parseTimestamp) DateTime? createdAt
});




}
/// @nodoc
class __$CommentCopyWithImpl<$Res>
    implements _$CommentCopyWith<$Res> {
  __$CommentCopyWithImpl(this._self, this._then);

  final _Comment _self;
  final $Res Function(_Comment) _then;

/// Create a copy of Comment
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? postId = null,Object? isMine = null,Object? author = null,Object? avatarKey = null,Object? content = null,Object? createdAt = freezed,}) {
  return _then(_Comment(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,postId: null == postId ? _self.postId : postId // ignore: cast_nullable_to_non_nullable
as int,isMine: null == isMine ? _self.isMine : isMine // ignore: cast_nullable_to_non_nullable
as bool,author: null == author ? _self.author : author // ignore: cast_nullable_to_non_nullable
as String,avatarKey: null == avatarKey ? _self.avatarKey : avatarKey // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
