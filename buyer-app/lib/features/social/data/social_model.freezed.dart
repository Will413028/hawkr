// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'social_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$Post {

 int get id; String get content; int? get merchantId;// 店面網址的路徑段；空字串代表沒有可進的店面。
 String get storeCode; String? get merchantName; String? get merchantAvatar; String get imageUrl; int get likeCount; int get commentCount; int get shareCount; int get viewCount; bool get liked;// 第一次發布的時間，epoch 秒；沒有時 null。
 int? get createdAt;// 商品卡片要的 id、name、image、price（最低價）。
 Map<String, dynamic>? get product;// 觀看者此刻能不能留言（伺服器算的顯示用提示）。
 bool get viewerCanComment;// 觀看者留言時的身分；訪客 null。
 pb.Actor? get viewerActor;
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCopyWith<Post> get copyWith => _$PostCopyWithImpl<Post>(this as Post, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Post&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.viewCount, viewCount) || other.viewCount == viewCount)&&(identical(other.liked, liked) || other.liked == liked)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other.product, product)&&(identical(other.viewerCanComment, viewerCanComment) || other.viewerCanComment == viewerCanComment)&&(identical(other.viewerActor, viewerActor) || other.viewerActor == viewerActor));
}


@override
int get hashCode => Object.hash(runtimeType,id,content,merchantId,storeCode,merchantName,merchantAvatar,imageUrl,likeCount,commentCount,shareCount,viewCount,liked,createdAt,const DeepCollectionEquality().hash(product),viewerCanComment,viewerActor);

@override
String toString() {
  return 'Post(id: $id, content: $content, merchantId: $merchantId, storeCode: $storeCode, merchantName: $merchantName, merchantAvatar: $merchantAvatar, imageUrl: $imageUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, viewCount: $viewCount, liked: $liked, createdAt: $createdAt, product: $product, viewerCanComment: $viewerCanComment, viewerActor: $viewerActor)';
}


}

/// @nodoc
abstract mixin class $PostCopyWith<$Res>  {
  factory $PostCopyWith(Post value, $Res Function(Post) _then) = _$PostCopyWithImpl;
@useResult
$Res call({
 int id, String content, int? merchantId, String storeCode, String? merchantName, String? merchantAvatar, String imageUrl, int likeCount, int commentCount, int shareCount, int viewCount, bool liked, int? createdAt, Map<String, dynamic>? product, bool viewerCanComment, pb.Actor? viewerActor
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
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? content = null,Object? merchantId = freezed,Object? storeCode = null,Object? merchantName = freezed,Object? merchantAvatar = freezed,Object? imageUrl = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? viewCount = null,Object? liked = null,Object? createdAt = freezed,Object? product = freezed,Object? viewerCanComment = null,Object? viewerActor = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as int?,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
as String,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,merchantAvatar: freezed == merchantAvatar ? _self.merchantAvatar : merchantAvatar // ignore: cast_nullable_to_non_nullable
as String?,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,viewCount: null == viewCount ? _self.viewCount : viewCount // ignore: cast_nullable_to_non_nullable
as int,liked: null == liked ? _self.liked : liked // ignore: cast_nullable_to_non_nullable
as bool,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as int?,product: freezed == product ? _self.product : product // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,viewerCanComment: null == viewerCanComment ? _self.viewerCanComment : viewerCanComment // ignore: cast_nullable_to_non_nullable
as bool,viewerActor: freezed == viewerActor ? _self.viewerActor : viewerActor // ignore: cast_nullable_to_non_nullable
as pb.Actor?,
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id,  String content,  int? merchantId,  String storeCode,  String? merchantName,  String? merchantAvatar,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  int viewCount,  bool liked,  int? createdAt,  Map<String, dynamic>? product,  bool viewerCanComment,  pb.Actor? viewerActor)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.content,_that.merchantId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product,_that.viewerCanComment,_that.viewerActor);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id,  String content,  int? merchantId,  String storeCode,  String? merchantName,  String? merchantAvatar,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  int viewCount,  bool liked,  int? createdAt,  Map<String, dynamic>? product,  bool viewerCanComment,  pb.Actor? viewerActor)  $default,) {final _that = this;
switch (_that) {
case _Post():
return $default(_that.id,_that.content,_that.merchantId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product,_that.viewerCanComment,_that.viewerActor);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id,  String content,  int? merchantId,  String storeCode,  String? merchantName,  String? merchantAvatar,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  int viewCount,  bool liked,  int? createdAt,  Map<String, dynamic>? product,  bool viewerCanComment,  pb.Actor? viewerActor)?  $default,) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.content,_that.merchantId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product,_that.viewerCanComment,_that.viewerActor);case _:
  return null;

}
}

}

/// @nodoc


class _Post implements Post {
  const _Post({required this.id, this.content = '', this.merchantId, this.storeCode = '', this.merchantName, this.merchantAvatar, this.imageUrl = '', this.likeCount = 0, this.commentCount = 0, this.shareCount = 0, this.viewCount = 0, this.liked = false, this.createdAt, final  Map<String, dynamic>? product, this.viewerCanComment = false, this.viewerActor}): _product = product;
  

@override final  int id;
@override@JsonKey() final  String content;
@override final  int? merchantId;
// 店面網址的路徑段；空字串代表沒有可進的店面。
@override@JsonKey() final  String storeCode;
@override final  String? merchantName;
@override final  String? merchantAvatar;
@override@JsonKey() final  String imageUrl;
@override@JsonKey() final  int likeCount;
@override@JsonKey() final  int commentCount;
@override@JsonKey() final  int shareCount;
@override@JsonKey() final  int viewCount;
@override@JsonKey() final  bool liked;
// 第一次發布的時間，epoch 秒；沒有時 null。
@override final  int? createdAt;
// 商品卡片要的 id、name、image、price（最低價）。
 final  Map<String, dynamic>? _product;
// 商品卡片要的 id、name、image、price（最低價）。
@override Map<String, dynamic>? get product {
  final value = _product;
  if (value == null) return null;
  if (_product is EqualUnmodifiableMapView) return _product;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}

// 觀看者此刻能不能留言（伺服器算的顯示用提示）。
@override@JsonKey() final  bool viewerCanComment;
// 觀看者留言時的身分；訪客 null。
@override final  pb.Actor? viewerActor;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostCopyWith<_Post> get copyWith => __$PostCopyWithImpl<_Post>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Post&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.viewCount, viewCount) || other.viewCount == viewCount)&&(identical(other.liked, liked) || other.liked == liked)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other._product, _product)&&(identical(other.viewerCanComment, viewerCanComment) || other.viewerCanComment == viewerCanComment)&&(identical(other.viewerActor, viewerActor) || other.viewerActor == viewerActor));
}


@override
int get hashCode => Object.hash(runtimeType,id,content,merchantId,storeCode,merchantName,merchantAvatar,imageUrl,likeCount,commentCount,shareCount,viewCount,liked,createdAt,const DeepCollectionEquality().hash(_product),viewerCanComment,viewerActor);

@override
String toString() {
  return 'Post(id: $id, content: $content, merchantId: $merchantId, storeCode: $storeCode, merchantName: $merchantName, merchantAvatar: $merchantAvatar, imageUrl: $imageUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, viewCount: $viewCount, liked: $liked, createdAt: $createdAt, product: $product, viewerCanComment: $viewerCanComment, viewerActor: $viewerActor)';
}


}

/// @nodoc
abstract mixin class _$PostCopyWith<$Res> implements $PostCopyWith<$Res> {
  factory _$PostCopyWith(_Post value, $Res Function(_Post) _then) = __$PostCopyWithImpl;
@override @useResult
$Res call({
 int id, String content, int? merchantId, String storeCode, String? merchantName, String? merchantAvatar, String imageUrl, int likeCount, int commentCount, int shareCount, int viewCount, bool liked, int? createdAt, Map<String, dynamic>? product, bool viewerCanComment, pb.Actor? viewerActor
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
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? content = null,Object? merchantId = freezed,Object? storeCode = null,Object? merchantName = freezed,Object? merchantAvatar = freezed,Object? imageUrl = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? viewCount = null,Object? liked = null,Object? createdAt = freezed,Object? product = freezed,Object? viewerCanComment = null,Object? viewerActor = freezed,}) {
  return _then(_Post(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as int?,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
as String,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,merchantAvatar: freezed == merchantAvatar ? _self.merchantAvatar : merchantAvatar // ignore: cast_nullable_to_non_nullable
as String?,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,viewCount: null == viewCount ? _self.viewCount : viewCount // ignore: cast_nullable_to_non_nullable
as int,liked: null == liked ? _self.liked : liked // ignore: cast_nullable_to_non_nullable
as bool,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as int?,product: freezed == product ? _self._product : product // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,viewerCanComment: null == viewerCanComment ? _self.viewerCanComment : viewerCanComment // ignore: cast_nullable_to_non_nullable
as bool,viewerActor: freezed == viewerActor ? _self.viewerActor : viewerActor // ignore: cast_nullable_to_non_nullable
as pb.Actor?,
  ));
}


}

// dart format on
