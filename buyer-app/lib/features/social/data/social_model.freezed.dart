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
mixin _$PostProduct {

 String get id; String get name; String get image; int get price;
/// Create a copy of PostProduct
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostProductCopyWith<PostProduct> get copyWith => _$PostProductCopyWithImpl<PostProduct>(this as PostProduct, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is PostProduct&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.image, image) || other.image == image)&&(identical(other.price, price) || other.price == price));
}


@override
int get hashCode => Object.hash(runtimeType,id,name,image,price);

@override
String toString() {
  return 'PostProduct(id: $id, name: $name, image: $image, price: $price)';
}


}

/// @nodoc
abstract mixin class $PostProductCopyWith<$Res>  {
  factory $PostProductCopyWith(PostProduct value, $Res Function(PostProduct) _then) = _$PostProductCopyWithImpl;
@useResult
$Res call({
 String id, String name, String image, int price
});




}
/// @nodoc
class _$PostProductCopyWithImpl<$Res>
    implements $PostProductCopyWith<$Res> {
  _$PostProductCopyWithImpl(this._self, this._then);

  final PostProduct _self;
  final $Res Function(PostProduct) _then;

/// Create a copy of PostProduct
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? name = null,Object? image = null,Object? price = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,image: null == image ? _self.image : image // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [PostProduct].
extension PostProductPatterns on PostProduct {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _PostProduct value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _PostProduct() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _PostProduct value)  $default,){
final _that = this;
switch (_that) {
case _PostProduct():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _PostProduct value)?  $default,){
final _that = this;
switch (_that) {
case _PostProduct() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String name,  String image,  int price)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _PostProduct() when $default != null:
return $default(_that.id,_that.name,_that.image,_that.price);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String name,  String image,  int price)  $default,) {final _that = this;
switch (_that) {
case _PostProduct():
return $default(_that.id,_that.name,_that.image,_that.price);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String name,  String image,  int price)?  $default,) {final _that = this;
switch (_that) {
case _PostProduct() when $default != null:
return $default(_that.id,_that.name,_that.image,_that.price);case _:
  return null;

}
}

}

/// @nodoc


class _PostProduct implements PostProduct {
  const _PostProduct({required this.id, required this.name, required this.image, required this.price});
  

@override final  String id;
@override final  String name;
@override final  String image;
@override final  int price;

/// Create a copy of PostProduct
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostProductCopyWith<_PostProduct> get copyWith => __$PostProductCopyWithImpl<_PostProduct>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _PostProduct&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.image, image) || other.image == image)&&(identical(other.price, price) || other.price == price));
}


@override
int get hashCode => Object.hash(runtimeType,id,name,image,price);

@override
String toString() {
  return 'PostProduct(id: $id, name: $name, image: $image, price: $price)';
}


}

/// @nodoc
abstract mixin class _$PostProductCopyWith<$Res> implements $PostProductCopyWith<$Res> {
  factory _$PostProductCopyWith(_PostProduct value, $Res Function(_PostProduct) _then) = __$PostProductCopyWithImpl;
@override @useResult
$Res call({
 String id, String name, String image, int price
});




}
/// @nodoc
class __$PostProductCopyWithImpl<$Res>
    implements _$PostProductCopyWith<$Res> {
  __$PostProductCopyWithImpl(this._self, this._then);

  final _PostProduct _self;
  final $Res Function(_PostProduct) _then;

/// Create a copy of PostProduct
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? name = null,Object? image = null,Object? price = null,}) {
  return _then(_PostProduct(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,image: null == image ? _self.image : image // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

/// @nodoc
mixin _$Post {

 int get id; String get content;// 發文店家的不透明 ID（封鎖、比對用）；沒有店家時空字串。
 String get storeId;// 店面網址的路徑段；空字串代表沒有可進的店面。
 String get storeCode; String? get merchantName; String? get merchantAvatar; String get imageUrl; int get likeCount; int get commentCount; int get shareCount; int get viewCount; bool get liked;// 第一次發布的時間，epoch 秒；沒有時 null。
 int? get createdAt;// 商品卡片欄位：id、名稱、圖片與最低價。
 PostProduct? get product;// 伺服器判斷觀看者此刻能否留言。
 bool get viewerCanComment;// 留言時的觀看者身分；訪客為 null。
 pb.Actor? get viewerActor;// 分享用的絕對網址；沒有時空字串。
 String get shareUrl;
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCopyWith<Post> get copyWith => _$PostCopyWithImpl<Post>(this as Post, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Post&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.viewCount, viewCount) || other.viewCount == viewCount)&&(identical(other.liked, liked) || other.liked == liked)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.product, product) || other.product == product)&&(identical(other.viewerCanComment, viewerCanComment) || other.viewerCanComment == viewerCanComment)&&(identical(other.viewerActor, viewerActor) || other.viewerActor == viewerActor)&&(identical(other.shareUrl, shareUrl) || other.shareUrl == shareUrl));
}


@override
int get hashCode => Object.hash(runtimeType,id,content,storeId,storeCode,merchantName,merchantAvatar,imageUrl,likeCount,commentCount,shareCount,viewCount,liked,createdAt,product,viewerCanComment,viewerActor,shareUrl);

@override
String toString() {
  return 'Post(id: $id, content: $content, storeId: $storeId, storeCode: $storeCode, merchantName: $merchantName, merchantAvatar: $merchantAvatar, imageUrl: $imageUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, viewCount: $viewCount, liked: $liked, createdAt: $createdAt, product: $product, viewerCanComment: $viewerCanComment, viewerActor: $viewerActor, shareUrl: $shareUrl)';
}


}

/// @nodoc
abstract mixin class $PostCopyWith<$Res>  {
  factory $PostCopyWith(Post value, $Res Function(Post) _then) = _$PostCopyWithImpl;
@useResult
$Res call({
 int id, String content, String storeId, String storeCode, String? merchantName, String? merchantAvatar, String imageUrl, int likeCount, int commentCount, int shareCount, int viewCount, bool liked, int? createdAt, PostProduct? product, bool viewerCanComment, pb.Actor? viewerActor, String shareUrl
});


$PostProductCopyWith<$Res>? get product;

}
/// @nodoc
class _$PostCopyWithImpl<$Res>
    implements $PostCopyWith<$Res> {
  _$PostCopyWithImpl(this._self, this._then);

  final Post _self;
  final $Res Function(Post) _then;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? content = null,Object? storeId = null,Object? storeCode = null,Object? merchantName = freezed,Object? merchantAvatar = freezed,Object? imageUrl = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? viewCount = null,Object? liked = null,Object? createdAt = freezed,Object? product = freezed,Object? viewerCanComment = null,Object? viewerActor = freezed,Object? shareUrl = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
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
as PostProduct?,viewerCanComment: null == viewerCanComment ? _self.viewerCanComment : viewerCanComment // ignore: cast_nullable_to_non_nullable
as bool,viewerActor: freezed == viewerActor ? _self.viewerActor : viewerActor // ignore: cast_nullable_to_non_nullable
as pb.Actor?,shareUrl: null == shareUrl ? _self.shareUrl : shareUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$PostProductCopyWith<$Res>? get product {
    if (_self.product == null) {
    return null;
  }

  return $PostProductCopyWith<$Res>(_self.product!, (value) {
    return _then(_self.copyWith(product: value));
  });
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id,  String content,  String storeId,  String storeCode,  String? merchantName,  String? merchantAvatar,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  int viewCount,  bool liked,  int? createdAt,  PostProduct? product,  bool viewerCanComment,  pb.Actor? viewerActor,  String shareUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.content,_that.storeId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product,_that.viewerCanComment,_that.viewerActor,_that.shareUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id,  String content,  String storeId,  String storeCode,  String? merchantName,  String? merchantAvatar,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  int viewCount,  bool liked,  int? createdAt,  PostProduct? product,  bool viewerCanComment,  pb.Actor? viewerActor,  String shareUrl)  $default,) {final _that = this;
switch (_that) {
case _Post():
return $default(_that.id,_that.content,_that.storeId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product,_that.viewerCanComment,_that.viewerActor,_that.shareUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id,  String content,  String storeId,  String storeCode,  String? merchantName,  String? merchantAvatar,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  int viewCount,  bool liked,  int? createdAt,  PostProduct? product,  bool viewerCanComment,  pb.Actor? viewerActor,  String shareUrl)?  $default,) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.content,_that.storeId,_that.storeCode,_that.merchantName,_that.merchantAvatar,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.viewCount,_that.liked,_that.createdAt,_that.product,_that.viewerCanComment,_that.viewerActor,_that.shareUrl);case _:
  return null;

}
}

}

/// @nodoc


class _Post implements Post {
  const _Post({required this.id, this.content = '', this.storeId = '', this.storeCode = '', this.merchantName, this.merchantAvatar, this.imageUrl = '', this.likeCount = 0, this.commentCount = 0, this.shareCount = 0, this.viewCount = 0, this.liked = false, this.createdAt, this.product, this.viewerCanComment = false, this.viewerActor, this.shareUrl = ''});
  

@override final  int id;
@override@JsonKey() final  String content;
// 發文店家的不透明 ID（封鎖、比對用）；沒有店家時空字串。
@override@JsonKey() final  String storeId;
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
// 商品卡片欄位：id、名稱、圖片與最低價。
@override final  PostProduct? product;
// 伺服器判斷觀看者此刻能否留言。
@override@JsonKey() final  bool viewerCanComment;
// 留言時的觀看者身分；訪客為 null。
@override final  pb.Actor? viewerActor;
// 分享用的絕對網址；沒有時空字串。
@override@JsonKey() final  String shareUrl;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostCopyWith<_Post> get copyWith => __$PostCopyWithImpl<_Post>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Post&&(identical(other.id, id) || other.id == id)&&(identical(other.content, content) || other.content == content)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.viewCount, viewCount) || other.viewCount == viewCount)&&(identical(other.liked, liked) || other.liked == liked)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.product, product) || other.product == product)&&(identical(other.viewerCanComment, viewerCanComment) || other.viewerCanComment == viewerCanComment)&&(identical(other.viewerActor, viewerActor) || other.viewerActor == viewerActor)&&(identical(other.shareUrl, shareUrl) || other.shareUrl == shareUrl));
}


@override
int get hashCode => Object.hash(runtimeType,id,content,storeId,storeCode,merchantName,merchantAvatar,imageUrl,likeCount,commentCount,shareCount,viewCount,liked,createdAt,product,viewerCanComment,viewerActor,shareUrl);

@override
String toString() {
  return 'Post(id: $id, content: $content, storeId: $storeId, storeCode: $storeCode, merchantName: $merchantName, merchantAvatar: $merchantAvatar, imageUrl: $imageUrl, likeCount: $likeCount, commentCount: $commentCount, shareCount: $shareCount, viewCount: $viewCount, liked: $liked, createdAt: $createdAt, product: $product, viewerCanComment: $viewerCanComment, viewerActor: $viewerActor, shareUrl: $shareUrl)';
}


}

/// @nodoc
abstract mixin class _$PostCopyWith<$Res> implements $PostCopyWith<$Res> {
  factory _$PostCopyWith(_Post value, $Res Function(_Post) _then) = __$PostCopyWithImpl;
@override @useResult
$Res call({
 int id, String content, String storeId, String storeCode, String? merchantName, String? merchantAvatar, String imageUrl, int likeCount, int commentCount, int shareCount, int viewCount, bool liked, int? createdAt, PostProduct? product, bool viewerCanComment, pb.Actor? viewerActor, String shareUrl
});


@override $PostProductCopyWith<$Res>? get product;

}
/// @nodoc
class __$PostCopyWithImpl<$Res>
    implements _$PostCopyWith<$Res> {
  __$PostCopyWithImpl(this._self, this._then);

  final _Post _self;
  final $Res Function(_Post) _then;

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? content = null,Object? storeId = null,Object? storeCode = null,Object? merchantName = freezed,Object? merchantAvatar = freezed,Object? imageUrl = null,Object? likeCount = null,Object? commentCount = null,Object? shareCount = null,Object? viewCount = null,Object? liked = null,Object? createdAt = freezed,Object? product = freezed,Object? viewerCanComment = null,Object? viewerActor = freezed,Object? shareUrl = null,}) {
  return _then(_Post(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
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
as PostProduct?,viewerCanComment: null == viewerCanComment ? _self.viewerCanComment : viewerCanComment // ignore: cast_nullable_to_non_nullable
as bool,viewerActor: freezed == viewerActor ? _self.viewerActor : viewerActor // ignore: cast_nullable_to_non_nullable
as pb.Actor?,shareUrl: null == shareUrl ? _self.shareUrl : shareUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$PostProductCopyWith<$Res>? get product {
    if (_self.product == null) {
    return null;
  }

  return $PostProductCopyWith<$Res>(_self.product!, (value) {
    return _then(_self.copyWith(product: value));
  });
}
}

// dart format on
