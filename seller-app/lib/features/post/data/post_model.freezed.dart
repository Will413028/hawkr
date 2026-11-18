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

 int get id; String get etag; String get content; String get imageUrl; int get likeCount; int get commentCount; int get shareCount; DateTime? get createdAt; PostLinkedProduct? get linkedProduct; PostStatus get status; String? get takedownMeasureId; DateTime? get scheduledPublishAt; String get url;
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostCopyWith<Post> get copyWith => _$PostCopyWithImpl<Post>(this as Post, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Post&&(identical(other.id, id) || other.id == id)&&(identical(other.etag, etag) || other.etag == etag)&&(identical(other.content, content) || other.content == content)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.linkedProduct, linkedProduct) || other.linkedProduct == linkedProduct)&&(identical(other.status, status) || other.status == status)&&(identical(other.takedownMeasureId, takedownMeasureId) || other.takedownMeasureId == takedownMeasureId)&&(identical(other.scheduledPublishAt, scheduledPublishAt) || other.scheduledPublishAt == scheduledPublishAt)&&(identical(other.url, url) || other.url == url));
}


@override
int get hashCode => Object.hash(runtimeType,id,etag,content,imageUrl,likeCount,commentCount,shareCount,createdAt,linkedProduct,status,takedownMeasureId,scheduledPublishAt,url);

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
 int id, String etag, String content, String imageUrl, int likeCount, int commentCount, int shareCount, DateTime? createdAt, PostLinkedProduct? linkedProduct, PostStatus status, String? takedownMeasureId, DateTime? scheduledPublishAt, String url
});


$PostLinkedProductCopyWith<$Res>? get linkedProduct;

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
as PostLinkedProduct?,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as PostStatus,takedownMeasureId: freezed == takedownMeasureId ? _self.takedownMeasureId : takedownMeasureId // ignore: cast_nullable_to_non_nullable
as String?,scheduledPublishAt: freezed == scheduledPublishAt ? _self.scheduledPublishAt : scheduledPublishAt // ignore: cast_nullable_to_non_nullable
as DateTime?,url: null == url ? _self.url : url // ignore: cast_nullable_to_non_nullable
as String,
  ));
}
/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$PostLinkedProductCopyWith<$Res>? get linkedProduct {
    if (_self.linkedProduct == null) {
    return null;
  }

  return $PostLinkedProductCopyWith<$Res>(_self.linkedProduct!, (value) {
    return _then(_self.copyWith(linkedProduct: value));
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id,  String etag,  String content,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  DateTime? createdAt,  PostLinkedProduct? linkedProduct,  PostStatus status,  String? takedownMeasureId,  DateTime? scheduledPublishAt,  String url)?  $default,{required TResult orElse(),}) {final _that = this;
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id,  String etag,  String content,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  DateTime? createdAt,  PostLinkedProduct? linkedProduct,  PostStatus status,  String? takedownMeasureId,  DateTime? scheduledPublishAt,  String url)  $default,) {final _that = this;
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id,  String etag,  String content,  String imageUrl,  int likeCount,  int commentCount,  int shareCount,  DateTime? createdAt,  PostLinkedProduct? linkedProduct,  PostStatus status,  String? takedownMeasureId,  DateTime? scheduledPublishAt,  String url)?  $default,) {final _that = this;
switch (_that) {
case _Post() when $default != null:
return $default(_that.id,_that.etag,_that.content,_that.imageUrl,_that.likeCount,_that.commentCount,_that.shareCount,_that.createdAt,_that.linkedProduct,_that.status,_that.takedownMeasureId,_that.scheduledPublishAt,_that.url);case _:
  return null;

}
}

}

/// @nodoc


class _Post extends Post {
  const _Post({required this.id, this.etag = '', this.content = '', this.imageUrl = '', this.likeCount = 0, this.commentCount = 0, this.shareCount = 0, this.createdAt, this.linkedProduct, this.status = PostStatus.POST_STATUS_UNSPECIFIED, this.takedownMeasureId, this.scheduledPublishAt, this.url = ''}): super._();
  

@override final  int id;
@override@JsonKey() final  String etag;
@override@JsonKey() final  String content;
@override@JsonKey() final  String imageUrl;
@override@JsonKey() final  int likeCount;
@override@JsonKey() final  int commentCount;
@override@JsonKey() final  int shareCount;
@override final  DateTime? createdAt;
@override final  PostLinkedProduct? linkedProduct;
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
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Post&&(identical(other.id, id) || other.id == id)&&(identical(other.etag, etag) || other.etag == etag)&&(identical(other.content, content) || other.content == content)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.likeCount, likeCount) || other.likeCount == likeCount)&&(identical(other.commentCount, commentCount) || other.commentCount == commentCount)&&(identical(other.shareCount, shareCount) || other.shareCount == shareCount)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.linkedProduct, linkedProduct) || other.linkedProduct == linkedProduct)&&(identical(other.status, status) || other.status == status)&&(identical(other.takedownMeasureId, takedownMeasureId) || other.takedownMeasureId == takedownMeasureId)&&(identical(other.scheduledPublishAt, scheduledPublishAt) || other.scheduledPublishAt == scheduledPublishAt)&&(identical(other.url, url) || other.url == url));
}


@override
int get hashCode => Object.hash(runtimeType,id,etag,content,imageUrl,likeCount,commentCount,shareCount,createdAt,linkedProduct,status,takedownMeasureId,scheduledPublishAt,url);

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
 int id, String etag, String content, String imageUrl, int likeCount, int commentCount, int shareCount, DateTime? createdAt, PostLinkedProduct? linkedProduct, PostStatus status, String? takedownMeasureId, DateTime? scheduledPublishAt, String url
});


@override $PostLinkedProductCopyWith<$Res>? get linkedProduct;

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
as DateTime?,linkedProduct: freezed == linkedProduct ? _self.linkedProduct : linkedProduct // ignore: cast_nullable_to_non_nullable
as PostLinkedProduct?,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as PostStatus,takedownMeasureId: freezed == takedownMeasureId ? _self.takedownMeasureId : takedownMeasureId // ignore: cast_nullable_to_non_nullable
as String?,scheduledPublishAt: freezed == scheduledPublishAt ? _self.scheduledPublishAt : scheduledPublishAt // ignore: cast_nullable_to_non_nullable
as DateTime?,url: null == url ? _self.url : url // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

/// Create a copy of Post
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$PostLinkedProductCopyWith<$Res>? get linkedProduct {
    if (_self.linkedProduct == null) {
    return null;
  }

  return $PostLinkedProductCopyWith<$Res>(_self.linkedProduct!, (value) {
    return _then(_self.copyWith(linkedProduct: value));
  });
}
}

/// @nodoc
mixin _$PostLinkedProduct {

 String get id; String get name; int get price; String get imageUrl;
/// Create a copy of PostLinkedProduct
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$PostLinkedProductCopyWith<PostLinkedProduct> get copyWith => _$PostLinkedProductCopyWithImpl<PostLinkedProduct>(this as PostLinkedProduct, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is PostLinkedProduct&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.price, price) || other.price == price)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}


@override
int get hashCode => Object.hash(runtimeType,id,name,price,imageUrl);

@override
String toString() {
  return 'PostLinkedProduct(id: $id, name: $name, price: $price, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class $PostLinkedProductCopyWith<$Res>  {
  factory $PostLinkedProductCopyWith(PostLinkedProduct value, $Res Function(PostLinkedProduct) _then) = _$PostLinkedProductCopyWithImpl;
@useResult
$Res call({
 String id, String name, int price, String imageUrl
});




}
/// @nodoc
class _$PostLinkedProductCopyWithImpl<$Res>
    implements $PostLinkedProductCopyWith<$Res> {
  _$PostLinkedProductCopyWithImpl(this._self, this._then);

  final PostLinkedProduct _self;
  final $Res Function(PostLinkedProduct) _then;

/// Create a copy of PostLinkedProduct
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? name = null,Object? price = null,Object? imageUrl = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [PostLinkedProduct].
extension PostLinkedProductPatterns on PostLinkedProduct {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _PostLinkedProduct value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _PostLinkedProduct() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _PostLinkedProduct value)  $default,){
final _that = this;
switch (_that) {
case _PostLinkedProduct():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _PostLinkedProduct value)?  $default,){
final _that = this;
switch (_that) {
case _PostLinkedProduct() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String name,  int price,  String imageUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _PostLinkedProduct() when $default != null:
return $default(_that.id,_that.name,_that.price,_that.imageUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String name,  int price,  String imageUrl)  $default,) {final _that = this;
switch (_that) {
case _PostLinkedProduct():
return $default(_that.id,_that.name,_that.price,_that.imageUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String name,  int price,  String imageUrl)?  $default,) {final _that = this;
switch (_that) {
case _PostLinkedProduct() when $default != null:
return $default(_that.id,_that.name,_that.price,_that.imageUrl);case _:
  return null;

}
}

}

/// @nodoc


class _PostLinkedProduct extends PostLinkedProduct {
  const _PostLinkedProduct({required this.id, this.name = '', this.price = 0, this.imageUrl = ''}): super._();
  

@override final  String id;
@override@JsonKey() final  String name;
@override@JsonKey() final  int price;
@override@JsonKey() final  String imageUrl;

/// Create a copy of PostLinkedProduct
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$PostLinkedProductCopyWith<_PostLinkedProduct> get copyWith => __$PostLinkedProductCopyWithImpl<_PostLinkedProduct>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _PostLinkedProduct&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.price, price) || other.price == price)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}


@override
int get hashCode => Object.hash(runtimeType,id,name,price,imageUrl);

@override
String toString() {
  return 'PostLinkedProduct(id: $id, name: $name, price: $price, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class _$PostLinkedProductCopyWith<$Res> implements $PostLinkedProductCopyWith<$Res> {
  factory _$PostLinkedProductCopyWith(_PostLinkedProduct value, $Res Function(_PostLinkedProduct) _then) = __$PostLinkedProductCopyWithImpl;
@override @useResult
$Res call({
 String id, String name, int price, String imageUrl
});




}
/// @nodoc
class __$PostLinkedProductCopyWithImpl<$Res>
    implements _$PostLinkedProductCopyWith<$Res> {
  __$PostLinkedProductCopyWithImpl(this._self, this._then);

  final _PostLinkedProduct _self;
  final $Res Function(_PostLinkedProduct) _then;

/// Create a copy of PostLinkedProduct
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? name = null,Object? price = null,Object? imageUrl = null,}) {
  return _then(_PostLinkedProduct(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
