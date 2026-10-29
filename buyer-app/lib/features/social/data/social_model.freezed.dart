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

@JsonKey(fromJson: tsidToInt) int get id; String get content;@JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull) int? get merchantId;// 店面網址的路徑段（列表端點由 gateway 從 seller 批次回填；詳情端點沒有）。空字串代表沒有可進的店面。
@JsonKey(name: 'storeCode') String get storeCode;@JsonKey(name: 'sellerName') String? get merchantName;@JsonKey(name: 'logoUrl') String? get merchantAvatar;// 49 §6.1 成對契約：貼文圖只有複數欄。social 的 posts.image 是單一 TEXT 欄，
// 所以現階段恆為 0–1 element；後端無 omitempty，無圖是 `[]` 而非缺欄。
@JsonKey(name: 'imageKeys') List<String> get imageKeys;@JsonKey(name: 'imageUrls') List<String> get imageUrls;@JsonKey(name: 'likeCount') int get likeCount;@JsonKey(name: 'commentCount') int get commentCount;@JsonKey(name: 'shareCount') int get shareCount;@JsonKey(name: 'viewCount') int get viewCount;@JsonKey(name: 'isLiked') bool get liked;@JsonKey(name: 'createdAt') dynamic get createdAt; Map<String, dynamic>? get product;
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCopyWith<Post> get copyWith => _$PostCopyWithImpl<Post>(this as Post, _$identity);

  /// Serializes this Post to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Post&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&const DeepCollectionEquality().equals(other.imageKeys, imageKeys)&&const DeepCollectionEquality().equals(other.imageUrls, imageUrls)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.viewCount, viewCount) || other.viewCount == viewCount)&&(identical(other.liked, liked) || other.liked == liked)&&const DeepCollectionEquality().equals(other.createdAt, createdAt)&&const DeepCollectionEquality().equals(other.product, product));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,content,merchantId,storeCode,merchantName,merchantAvatar,const DeepCollectionEquality().hash(imageKeys),const DeepCollectionEquality().hash(imageUrls),likeCount,commentCount,shareCount,viewCount,liked,const DeepCollectionEquality().hash(createdAt),const DeepCollectionEquality().hash(product));

@override
String toString() {
  return 'Post(id: $id, content: $content, merchantId: $merchantId, storeCode: $storeCode, merchantName: $merchantName, merchantAvatar: $merchantAvatar, imageKeys: $imageKeys, imageUrls: $imageUrls, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, viewCount: $viewCount, liked: $liked, createdAt: $createdAt, product: $product)';
}


}

/// @nodoc
abstract mixin class $PostCopyWith<$Res>  {
  factory $PostCopyWith(Post value, $Res Function(Post) _then) = _$PostCopyWithImpl;
@useResult
$Res call({
@JsonKey(fromJson: tsidToInt) int id, String content,@JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull) int? merchantId,@JsonKey(name: 'storeCode') String storeCode,@JsonKey(name: 'sellerName') String? merchantName,@JsonKey(name: 'logoUrl') String? merchantAvatar,@JsonKey(name: 'imageKeys') List<String> imageKeys,@JsonKey(name: 'imageUrls') List<String> imageUrls,@JsonKey(name: 'likeCount') int likeCount,@JsonKey(name: 'commentCount') int commentCount,@JsonKey(name: 'shareCount') int shareCount,@JsonKey(name: 'viewCount') int viewCount,@JsonKey(name: 'isLiked') bool liked,@JsonKey(name: 'createdAt') dynamic createdAt, Map<String, dynamic>? product
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
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? content = null,Object? merchantId = freezed,Object? storeCode = null,Object? merchantName = freezed,Object? merchantAvatar = freezed,Object? imageKeys = null,Object? imageUrls = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? viewCount = null,Object? liked = null,Object? createdAt = freezed,Object? product = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as int?,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
as String,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,merchantAvatar: freezed == merchantAvatar ? _self.merchantAvatar : merchantAvatar // ignore: cast_nullable_to_non_nullable
as String?,imageKeys: null == imageKeys ? _self.imageKeys : imageKeys // ignore: cast_nullable_to_non_nullable
as List<String>,imageUrls: null == imageUrls ? _self.imageUrls : imageUrls // ignore: cast_nullable_to_non_nullable
as List<String>,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,viewCount: null == viewCount ? _self.viewCount : viewCount // ignore: cast_nullable_to_non_nullable
as int,liked: null == liked ? _self.liked : liked // ignore: cast_nullable_to_non_nullable
as bool,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as dynamic,product: freezed == product ? _self.product : product // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(fromJson: tsidToInt)  int id,  String content, @JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull)  int? merchantId, @JsonKey(name: 'storeCode')  String storeCode, @JsonKey(name: 'sellerName')  String? merchantName, @JsonKey(name: 'logoUrl')  String? merchantAvatar, @JsonKey(name: 'imageKeys')  List<String> imageKeys, @JsonKey(name: 'imageUrls')  List<String> imageUrls, @JsonKey(name: 'likeCount')  int likeCount, @JsonKey(name: 'commentCount')  int commentCount, @JsonKey(name: 'shareCount')  int shareCount, @JsonKey(name: 'viewCount')  int viewCount, @JsonKey(name: 'isLiked')  bool liked, @JsonKey(name: 'createdAt')  dynamic createdAt,  Map<String, dynamic>? product)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.content,_that.merchantId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageKeys,_that.imageUrls,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(fromJson: tsidToInt)  int id,  String content, @JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull)  int? merchantId, @JsonKey(name: 'storeCode')  String storeCode, @JsonKey(name: 'sellerName')  String? merchantName, @JsonKey(name: 'logoUrl')  String? merchantAvatar, @JsonKey(name: 'imageKeys')  List<String> imageKeys, @JsonKey(name: 'imageUrls')  List<String> imageUrls, @JsonKey(name: 'likeCount')  int likeCount, @JsonKey(name: 'commentCount')  int commentCount, @JsonKey(name: 'shareCount')  int shareCount, @JsonKey(name: 'viewCount')  int viewCount, @JsonKey(name: 'isLiked')  bool liked, @JsonKey(name: 'createdAt')  dynamic createdAt,  Map<String, dynamic>? product)  $default,) {final _that = this;
switch (_that) {
case _Post():
return $default(_that.id,_that.content,_that.merchantId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageKeys,_that.imageUrls,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(fromJson: tsidToInt)  int id,  String content, @JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull)  int? merchantId, @JsonKey(name: 'storeCode')  String storeCode, @JsonKey(name: 'sellerName')  String? merchantName, @JsonKey(name: 'logoUrl')  String? merchantAvatar, @JsonKey(name: 'imageKeys')  List<String> imageKeys, @JsonKey(name: 'imageUrls')  List<String> imageUrls, @JsonKey(name: 'likeCount')  int likeCount, @JsonKey(name: 'commentCount')  int commentCount, @JsonKey(name: 'shareCount')  int shareCount, @JsonKey(name: 'viewCount')  int viewCount, @JsonKey(name: 'isLiked')  bool liked, @JsonKey(name: 'createdAt')  dynamic createdAt,  Map<String, dynamic>? product)?  $default,) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.content,_that.merchantId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageKeys,_that.imageUrls,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Post extends Post {
  const _Post({@JsonKey(fromJson: tsidToInt) required this.id, this.content = '', @JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull) this.merchantId, @JsonKey(name: 'storeCode') this.storeCode = '', @JsonKey(name: 'sellerName') this.merchantName, @JsonKey(name: 'logoUrl') this.merchantAvatar, @JsonKey(name: 'imageKeys') final  List<String> imageKeys = const <String>[], @JsonKey(name: 'imageUrls') final  List<String> imageUrls = const <String>[], @JsonKey(name: 'likeCount') this.likeCount = 0, @JsonKey(name: 'commentCount') this.commentCount = 0, @JsonKey(name: 'shareCount') this.shareCount = 0, @JsonKey(name: 'viewCount') this.viewCount = 0, @JsonKey(name: 'isLiked') this.liked = false, @JsonKey(name: 'createdAt') this.createdAt, final  Map<String, dynamic>? product}): _imageKeys = imageKeys,_imageUrls = imageUrls,_product = product,super._();
  factory _Post.fromJson(Map<String, dynamic> json) => _$PostFromJson(json);

@override@JsonKey(fromJson: tsidToInt) final  int id;
@override@JsonKey() final  String content;
@override@JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull) final  int? merchantId;
// 店面網址的路徑段（列表端點由 gateway 從 seller 批次回填；詳情端點沒有）。空字串代表沒有可進的店面。
@override@JsonKey(name: 'storeCode') final  String storeCode;
@override@JsonKey(name: 'sellerName') final  String? merchantName;
@override@JsonKey(name: 'logoUrl') final  String? merchantAvatar;
// 49 §6.1 成對契約：貼文圖只有複數欄。social 的 posts.image 是單一 TEXT 欄，
// 所以現階段恆為 0–1 element；後端無 omitempty，無圖是 `[]` 而非缺欄。
 final  List<String> _imageKeys;
// 49 §6.1 成對契約：貼文圖只有複數欄。social 的 posts.image 是單一 TEXT 欄，
// 所以現階段恆為 0–1 element；後端無 omitempty，無圖是 `[]` 而非缺欄。
@override@JsonKey(name: 'imageKeys') List<String> get imageKeys {
  if (_imageKeys is EqualUnmodifiableListView) return _imageKeys;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_imageKeys);
}

 final  List<String> _imageUrls;
@override@JsonKey(name: 'imageUrls') List<String> get imageUrls {
  if (_imageUrls is EqualUnmodifiableListView) return _imageUrls;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_imageUrls);
}

@override@JsonKey(name: 'likeCount') final  int likeCount;
@override@JsonKey(name: 'commentCount') final  int commentCount;
@override@JsonKey(name: 'shareCount') final  int shareCount;
@override@JsonKey(name: 'viewCount') final  int viewCount;
@override@JsonKey(name: 'isLiked') final  bool liked;
@override@JsonKey(name: 'createdAt') final  dynamic createdAt;
 final  Map<String, dynamic>? _product;
@override Map<String, dynamic>? get product {
  final value = _product;
  if (value == null) return null;
  if (_product is EqualUnmodifiableMapView) return _product;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}


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
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Post&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&const DeepCollectionEquality().equals(other._imageKeys, _imageKeys)&&const DeepCollectionEquality().equals(other._imageUrls, _imageUrls)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.viewCount, viewCount) || other.viewCount == viewCount)&&(identical(other.liked, liked) || other.liked == liked)&&const DeepCollectionEquality().equals(other.createdAt, createdAt)&&const DeepCollectionEquality().equals(other._product, _product));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,content,merchantId,storeCode,merchantName,merchantAvatar,const DeepCollectionEquality().hash(_imageKeys),const DeepCollectionEquality().hash(_imageUrls),likeCount,commentCount,shareCount,viewCount,liked,const DeepCollectionEquality().hash(createdAt),const DeepCollectionEquality().hash(_product));

@override
String toString() {
  return 'Post(id: $id, content: $content, merchantId: $merchantId, storeCode: $storeCode, merchantName: $merchantName, merchantAvatar: $merchantAvatar, imageKeys: $imageKeys, imageUrls: $imageUrls, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, viewCount: $viewCount, liked: $liked, createdAt: $createdAt, product: $product)';
}


}

/// @nodoc
abstract mixin class _$PostCopyWith<$Res> implements $PostCopyWith<$Res> {
  factory _$PostCopyWith(_Post value, $Res Function(_Post) _then) = __$PostCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(fromJson: tsidToInt) int id, String content,@JsonKey(name: 'sellerId', fromJson: tsidToIntOrNull) int? merchantId,@JsonKey(name: 'storeCode') String storeCode,@JsonKey(name: 'sellerName') String? merchantName,@JsonKey(name: 'logoUrl') String? merchantAvatar,@JsonKey(name: 'imageKeys') List<String> imageKeys,@JsonKey(name: 'imageUrls') List<String> imageUrls,@JsonKey(name: 'likeCount') int likeCount,@JsonKey(name: 'commentCount') int commentCount,@JsonKey(name: 'shareCount') int shareCount,@JsonKey(name: 'viewCount') int viewCount,@JsonKey(name: 'isLiked') bool liked,@JsonKey(name: 'createdAt') dynamic createdAt, Map<String, dynamic>? product
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
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? content = null,Object? merchantId = freezed,Object? storeCode = null,Object? merchantName = freezed,Object? merchantAvatar = freezed,Object? imageKeys = null,Object? imageUrls = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? viewCount = null,Object? liked = null,Object? createdAt = freezed,Object? product = freezed,}) {
  return _then(_Post(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as int?,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
as String,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,merchantAvatar: freezed == merchantAvatar ? _self.merchantAvatar : merchantAvatar // ignore: cast_nullable_to_non_nullable
as String?,imageKeys: null == imageKeys ? _self._imageKeys : imageKeys // ignore: cast_nullable_to_non_nullable
as List<String>,imageUrls: null == imageUrls ? _self._imageUrls : imageUrls // ignore: cast_nullable_to_non_nullable
as List<String>,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,commentCount: null == commentCount ? _self.commentCount : commentCount // ignore: cast_nullable_to_non_nullable
as int,shareCount: null == shareCount ? _self.shareCount : shareCount // ignore: cast_nullable_to_non_nullable
as int,viewCount: null == viewCount ? _self.viewCount : viewCount // ignore: cast_nullable_to_non_nullable
as int,liked: null == liked ? _self.liked : liked // ignore: cast_nullable_to_non_nullable
as bool,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as dynamic,product: freezed == product ? _self._product : product // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,
  ));
}


}


/// @nodoc
mixin _$PostComment {

@JsonKey(fromJson: tsidToInt) int get id; String get content;// 伺服器依呼叫者身分算好的「這是不是我的留言」；公開輸出沒有留言者 ID。
@JsonKey(name: 'isMine') bool get isMine;// REST 留言列表／寫入回應與 SSE comment.created 收斂後同名 userName。
@JsonKey(name: 'userName') String? get userName;// 49 §6.1 成對契約：留言者頭像顯示 URL（Gateway 組）。
@JsonKey(name: 'avatarUrl') String? get avatarUrl;@JsonKey(name: 'likeCount') int get likeCount;@JsonKey(name: 'isLiked') bool get liked;// REST 回 epoch 秒（int），SSE 回 ISO 8601 字串——兩通道型別不同，維持 dynamic。
@JsonKey(name: 'createdAt') dynamic get createdAt;
/// Create a copy of PostComment
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCommentCopyWith<PostComment> get copyWith => _$PostCommentCopyWithImpl<PostComment>(this as PostComment, _$identity);

  /// Serializes this PostComment to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is PostComment&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.isMine, isMine) || other.isMine == isMine)&&(identical(other.userName, userName) || other.userName == userName)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.liked, liked) || other.liked == liked)&&const DeepCollectionEquality().equals(other.createdAt, createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,content,isMine,userName,avatarUrl,likeCount,liked,const DeepCollectionEquality().hash(createdAt));

@override
String toString() {
  return 'PostComment(id: $id, content: $content, isMine: $isMine, userName: $userName, avatarUrl: $avatarUrl, likeCount: $likeCount, liked: $liked, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $PostCommentCopyWith<$Res>  {
  factory $PostCommentCopyWith(PostComment value, $Res Function(PostComment) _then) = _$PostCommentCopyWithImpl;
@useResult
$Res call({
@JsonKey(fromJson: tsidToInt) int id, String content,@JsonKey(name: 'isMine') bool isMine,@JsonKey(name: 'userName') String? userName,@JsonKey(name: 'avatarUrl') String? avatarUrl,@JsonKey(name: 'likeCount') int likeCount,@JsonKey(name: 'isLiked') bool liked,@JsonKey(name: 'createdAt') dynamic createdAt
});




}
/// @nodoc
class _$PostCommentCopyWithImpl<$Res>
    implements $PostCommentCopyWith<$Res> {
  _$PostCommentCopyWithImpl(this._self, this._then);

  final PostComment _self;
  final $Res Function(PostComment) _then;

/// Create a copy of PostComment
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? content = null,Object? isMine = null,Object? userName = freezed,Object? avatarUrl = freezed,Object? likeCount = null,Object? liked = null,Object? createdAt = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,isMine: null == isMine ? _self.isMine : isMine // ignore: cast_nullable_to_non_nullable
as bool,userName: freezed == userName ? _self.userName : userName // ignore: cast_nullable_to_non_nullable
as String?,avatarUrl: freezed == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String?,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,liked: null == liked ? _self.liked : liked // ignore: cast_nullable_to_non_nullable
as bool,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as dynamic,
  ));
}

}


/// Adds pattern-matching-related methods to [PostComment].
extension PostCommentPatterns on PostComment {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _PostComment value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _PostComment() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _PostComment value)  $default,){
final _that = this;
switch (_that) {
case _PostComment():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _PostComment value)?  $default,){
final _that = this;
switch (_that) {
case _PostComment() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(fromJson: tsidToInt)  int id,  String content, @JsonKey(name: 'isMine')  bool isMine, @JsonKey(name: 'userName')  String? userName, @JsonKey(name: 'avatarUrl')  String? avatarUrl, @JsonKey(name: 'likeCount')  int likeCount, @JsonKey(name: 'isLiked')  bool liked, @JsonKey(name: 'createdAt')  dynamic createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _PostComment() when $default != null:
return $default(_that.id,_that.content,_that.isMine,_that.userName,_that.avatarUrl,_that.likeCount,_that.liked,_that.createdAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(fromJson: tsidToInt)  int id,  String content, @JsonKey(name: 'isMine')  bool isMine, @JsonKey(name: 'userName')  String? userName, @JsonKey(name: 'avatarUrl')  String? avatarUrl, @JsonKey(name: 'likeCount')  int likeCount, @JsonKey(name: 'isLiked')  bool liked, @JsonKey(name: 'createdAt')  dynamic createdAt)  $default,) {final _that = this;
switch (_that) {
case _PostComment():
return $default(_that.id,_that.content,_that.isMine,_that.userName,_that.avatarUrl,_that.likeCount,_that.liked,_that.createdAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(fromJson: tsidToInt)  int id,  String content, @JsonKey(name: 'isMine')  bool isMine, @JsonKey(name: 'userName')  String? userName, @JsonKey(name: 'avatarUrl')  String? avatarUrl, @JsonKey(name: 'likeCount')  int likeCount, @JsonKey(name: 'isLiked')  bool liked, @JsonKey(name: 'createdAt')  dynamic createdAt)?  $default,) {final _that = this;
switch (_that) {
case _PostComment() when $default != null:
return $default(_that.id,_that.content,_that.isMine,_that.userName,_that.avatarUrl,_that.likeCount,_that.liked,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _PostComment implements PostComment {
  const _PostComment({@JsonKey(fromJson: tsidToInt) required this.id, required this.content, @JsonKey(name: 'isMine') this.isMine = false, @JsonKey(name: 'userName') this.userName, @JsonKey(name: 'avatarUrl') this.avatarUrl, @JsonKey(name: 'likeCount') this.likeCount = 0, @JsonKey(name: 'isLiked') this.liked = false, @JsonKey(name: 'createdAt') this.createdAt});
  factory _PostComment.fromJson(Map<String, dynamic> json) => _$PostCommentFromJson(json);

@override@JsonKey(fromJson: tsidToInt) final  int id;
@override final  String content;
// 伺服器依呼叫者身分算好的「這是不是我的留言」；公開輸出沒有留言者 ID。
@override@JsonKey(name: 'isMine') final  bool isMine;
// REST 留言列表／寫入回應與 SSE comment.created 收斂後同名 userName。
@override@JsonKey(name: 'userName') final  String? userName;
// 49 §6.1 成對契約：留言者頭像顯示 URL（Gateway 組）。
@override@JsonKey(name: 'avatarUrl') final  String? avatarUrl;
@override@JsonKey(name: 'likeCount') final  int likeCount;
@override@JsonKey(name: 'isLiked') final  bool liked;
// REST 回 epoch 秒（int），SSE 回 ISO 8601 字串——兩通道型別不同，維持 dynamic。
@override@JsonKey(name: 'createdAt') final  dynamic createdAt;

/// Create a copy of PostComment
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostCommentCopyWith<_PostComment> get copyWith => __$PostCommentCopyWithImpl<_PostComment>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$PostCommentToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _PostComment&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.isMine, isMine) || other.isMine == isMine)&&(identical(other.userName, userName) || other.userName == userName)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.liked, liked) || other.liked == liked)&&const DeepCollectionEquality().equals(other.createdAt, createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,content,isMine,userName,avatarUrl,likeCount,liked,const DeepCollectionEquality().hash(createdAt));

@override
String toString() {
  return 'PostComment(id: $id, content: $content, isMine: $isMine, userName: $userName, avatarUrl: $avatarUrl, likeCount: $likeCount, liked: $liked, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$PostCommentCopyWith<$Res> implements $PostCommentCopyWith<$Res> {
  factory _$PostCommentCopyWith(_PostComment value, $Res Function(_PostComment) _then) = __$PostCommentCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(fromJson: tsidToInt) int id, String content,@JsonKey(name: 'isMine') bool isMine,@JsonKey(name: 'userName') String? userName,@JsonKey(name: 'avatarUrl') String? avatarUrl,@JsonKey(name: 'likeCount') int likeCount,@JsonKey(name: 'isLiked') bool liked,@JsonKey(name: 'createdAt') dynamic createdAt
});




}
/// @nodoc
class __$PostCommentCopyWithImpl<$Res>
    implements _$PostCommentCopyWith<$Res> {
  __$PostCommentCopyWithImpl(this._self, this._then);

  final _PostComment _self;
  final $Res Function(_PostComment) _then;

/// Create a copy of PostComment
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? content = null,Object? isMine = null,Object? userName = freezed,Object? avatarUrl = freezed,Object? likeCount = null,Object? liked = null,Object? createdAt = freezed,}) {
  return _then(_PostComment(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,isMine: null == isMine ? _self.isMine : isMine // ignore: cast_nullable_to_non_nullable
as bool,userName: freezed == userName ? _self.userName : userName // ignore: cast_nullable_to_non_nullable
as String?,avatarUrl: freezed == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String?,likeCount: null == likeCount ? _self.likeCount : likeCount // ignore: cast_nullable_to_non_nullable
as int,liked: null == liked ? _self.liked : liked // ignore: cast_nullable_to_non_nullable
as bool,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as dynamic,
  ));
}


}

// dart format on
