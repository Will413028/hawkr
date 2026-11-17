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

 int get id; String get etag; String get content; String get imageUrl; int get likeCount; int get commentCount; int get shareCount; DateTime? get createdAt; Map<String, dynamic>? get linkedProduct; PostStatus get status; String? get takedownMeasureId; DateTime? get scheduledPublishAt; String get url;
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCopyWith<Post> get copyWith => _$PostCopyWithImpl<Post>(this as Post, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Post&&(identical(other.id, id) || other.id == id)&&(identical(other.etag, etag) || other.etag == etag)&&(identical(other.content, content) || other.content == content)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other.linkedProduct, linkedProduct)&&(identical(other.status, status) || other.status == status)&&(identical(other.takedownMeasureId, takedownMeasureId) || other.takedownMeasureId == takedownMeasureId)&&(identical(other.scheduledPublishAt, scheduledPublishAt) || other.scheduledPublishAt == scheduledPublishAt)&&(identical(other.url, url) || other.url == url));
}


@override
int get hashCode => Object.hash(runtimeType,id,etag,content,imageUrl,likeCount,commentCount,shareCount,createdAt,const DeepCollectionEquality().hash(linkedProduct),status,takedownMeasureId,scheduledPublishAt,url);

@override
String toString() {
  return 'Post(id: $id, etag: $etag, content: $content, imageUrl: $imageUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, createdAt: $createdAt, linkedProduct: $linkedProduct, status: $status, takedownMeasureId: $takedownMeasureId, scheduledPublishAt: $scheduledPublishAt, url: $url)';
}


}

/// @nodoc
abstract mixin class $PostCopyWith<$Res>  {
  factory $PostCopyWith(Post value, $Res Function(Post) _then) = _$PostCopyWithImpl;
@useResult
$Res call({
 int id, String etag, String content, String imageUrl, int likeCount, int commentCount, int shareCount, DateTime? createdAt, Map<String, dynamic>? linkedProduct, PostStatus status, String? takedownMeasureId, DateTime? scheduledPublishAt, String url
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
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? etag = null,Object? content = null,Object? imageUrl = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? createdAt = freezed,Object? linkedProduct = freezed,Object? status = null,Object? takedownMeasureId = freezed,Object? scheduledPublishAt = freezed,Object? url = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,etag: null == etag ? _self.etag : etag // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,linkedProduct: freezed == linkedProduct ? _self.linkedProduct : linkedProduct // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as PostStatus,takedownMeasureId: freezed == takedownMeasureId ? _self.takedownMeasureId : takedownMeasureId // ignore: cast_nullable_to_non_nullable
as String?,scheduledPublishAt: freezed == scheduledPublishAt ? _self.scheduledPublishAt : scheduledPublishAt // ignore: cast_nullable_to_non_nullable
as DateTime?,url: null == url ? _self.url : url // ignore: cast_nullable_to_non_nullable
as String,
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id,  String etag,  String content,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  DateTime? createdAt,  Map<String, dynamic>? linkedProduct,  PostStatus status,  String? takedownMeasureId,  DateTime? scheduledPublishAt,  String url)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.etag,_that.content,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.createdAt,_that.linkedProduct,_that.status,_that.takedownMeasureId,_that.scheduledPublishAt,_that.url);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id,  String etag,  String content,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  DateTime? createdAt,  Map<String, dynamic>? linkedProduct,  PostStatus status,  String? takedownMeasureId,  DateTime? scheduledPublishAt,  String url)  $default,) {final _that = this;
switch (_that) {
case _Post():
return $default(_that.id,_that.etag,_that.content,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.createdAt,_that.linkedProduct,_that.status,_that.takedownMeasureId,_that.scheduledPublishAt,_that.url);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id,  String etag,  String content,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  DateTime? createdAt,  Map<String, dynamic>? linkedProduct,  PostStatus status,  String? takedownMeasureId,  DateTime? scheduledPublishAt,  String url)?  $default,) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.etag,_that.content,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.createdAt,_that.linkedProduct,_that.status,_that.takedownMeasureId,_that.scheduledPublishAt,_that.url);case _:
  return null;

}
}

}

/// @nodoc


class _Post extends Post {
  const _Post({required this.id, this.etag = '', this.content = '', this.imageUrl = '', this.likeCount = 0, this.commentCount = 0, this.shareCount = 0, this.createdAt, final  Map<String, dynamic>? linkedProduct, this.status = PostStatus.POST_STATUS_UNSPECIFIED, this.takedownMeasureId, this.scheduledPublishAt, this.url = ''}): _linkedProduct = linkedProduct,super._();
  

@override final  int id;
@override@JsonKey() final  String etag;
@override@JsonKey() final  String content;
@override@JsonKey() final  String imageUrl;
@override@JsonKey() final  int likeCount;
@override@JsonKey() final  int commentCount;
@override@JsonKey() final  int shareCount;
@override final  DateTime? createdAt;
 final  Map<String, dynamic>? _linkedProduct;
@override Map<String, dynamic>? get linkedProduct {
  final value = _linkedProduct;
  if (value == null) return null;
  if (_linkedProduct is EqualUnmodifiableMapView) return _linkedProduct;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}

@override@JsonKey() final  PostStatus status;
@override final  String? takedownMeasureId;
@override final  DateTime? scheduledPublishAt;
@override@JsonKey() final  String url;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostCopyWith<_Post> get copyWith => __$PostCopyWithImpl<_Post>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Post&&(identical(other.id, id) || other.id == id)&&(identical(other.etag, etag) || other.etag == etag)&&(identical(other.content, content) || other.content == content)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other._linkedProduct, _linkedProduct)&&(identical(other.status, status) || other.status == status)&&(identical(other.takedownMeasureId, takedownMeasureId) || other.takedownMeasureId == takedownMeasureId)&&(identical(other.scheduledPublishAt, scheduledPublishAt) || other.scheduledPublishAt == scheduledPublishAt)&&(identical(other.url, url) || other.url == url));
}


@override
int get hashCode => Object.hash(runtimeType,id,etag,content,imageUrl,likeCount,commentCount,shareCount,createdAt,const DeepCollectionEquality().hash(_linkedProduct),status,takedownMeasureId,scheduledPublishAt,url);

@override
String toString() {
  return 'Post(id: $id, etag: $etag, content: $content, imageUrl: $imageUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, createdAt: $createdAt, linkedProduct: $linkedProduct, status: $status, takedownMeasureId: $takedownMeasureId, scheduledPublishAt: $scheduledPublishAt, url: $url)';
}


}

/// @nodoc
abstract mixin class _$PostCopyWith<$Res> implements $PostCopyWith<$Res> {
  factory _$PostCopyWith(_Post value, $Res Function(_Post) _then) = __$PostCopyWithImpl;
@override @useResult
$Res call({
 int id, String etag, String content, String imageUrl, int likeCount, int commentCount, int shareCount, DateTime? createdAt, Map<String, dynamic>? linkedProduct, PostStatus status, String? takedownMeasureId, DateTime? scheduledPublishAt, String url
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
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? etag = null,Object? content = null,Object? imageUrl = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? createdAt = freezed,Object? linkedProduct = freezed,Object? status = null,Object? takedownMeasureId = freezed,Object? scheduledPublishAt = freezed,Object? url = null,}) {
  return _then(_Post(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,etag: null == etag ? _self.etag : etag // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,linkedProduct: freezed == linkedProduct ? _self._linkedProduct : linkedProduct // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as PostStatus,takedownMeasureId: freezed == takedownMeasureId ? _self.takedownMeasureId : takedownMeasureId // ignore: cast_nullable_to_non_nullable
as String?,scheduledPublishAt: freezed == scheduledPublishAt ? _self.scheduledPublishAt : scheduledPublishAt // ignore: cast_nullable_to_non_nullable
as DateTime?,url: null == url ? _self.url : url // ignore: cast_nullable_to_non_nullable
as String,
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
