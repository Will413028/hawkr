// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'review_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$Review {

 int get id;@JsonKey(name: 'skuId') int get skuId;@JsonKey(name: 'orderItemId') int? get orderItemId; int get rating; String get content; List<String> get images;@JsonKey(name: 'userId') int? get userId;@JsonKey(name: 'username') String? get username;// 49 §6.1 成對契約：avatarKey 是裸 objectKey（SSOT），顯示前過 buildImageUrl。
@JsonKey(name: 'avatarKey') String? get avatarKey;@JsonKey(name: 'createdAt') String? get createdAt;
/// Create a copy of Review
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ReviewCopyWith<Review> get copyWith => _$ReviewCopyWithImpl<Review>(this as Review, _$identity);

  /// Serializes this Review to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Review&&(identical(other.id, id) || other.id == id)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.rating, rating) || other.rating == rating)&&(identical(other.content, content) || other.content == content)&&const DeepCollectionEquality().equals(other.images, images)&&(identical(other.userId, userId) || other.userId == userId)&&(identical(other.username, username) || other.username == username)&&(identical(other.avatarKey, avatarKey) || other.avatarKey == avatarKey)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,skuId,orderItemId,rating,content,const DeepCollectionEquality().hash(images),userId,username,avatarKey,createdAt);

@override
String toString() {
  return 'Review(id: $id, skuId: $skuId, orderItemId: $orderItemId, rating: $rating, content: $content, images: $images, userId: $userId, username: $username, avatarKey: $avatarKey, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $ReviewCopyWith<$Res>  {
  factory $ReviewCopyWith(Review value, $Res Function(Review) _then) = _$ReviewCopyWithImpl;
@useResult
$Res call({
 int id,@JsonKey(name: 'skuId') int skuId,@JsonKey(name: 'orderItemId') int? orderItemId, int rating, String content, List<String> images,@JsonKey(name: 'userId') int? userId,@JsonKey(name: 'username') String? username,@JsonKey(name: 'avatarKey') String? avatarKey,@JsonKey(name: 'createdAt') String? createdAt
});




}
/// @nodoc
class _$ReviewCopyWithImpl<$Res>
    implements $ReviewCopyWith<$Res> {
  _$ReviewCopyWithImpl(this._self, this._then);

  final Review _self;
  final $Res Function(Review) _then;

/// Create a copy of Review
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? skuId = null,Object? orderItemId = freezed,Object? rating = null,Object? content = null,Object? images = null,Object? userId = freezed,Object? username = freezed,Object? avatarKey = freezed,Object? createdAt = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as int,orderItemId: freezed == orderItemId ? _self.orderItemId : orderItemId // ignore: cast_nullable_to_non_nullable
as int?,rating: null == rating ? _self.rating : rating // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,images: null == images ? _self.images : images // ignore: cast_nullable_to_non_nullable
as List<String>,userId: freezed == userId ? _self.userId : userId // ignore: cast_nullable_to_non_nullable
as int?,username: freezed == username ? _self.username : username // ignore: cast_nullable_to_non_nullable
as String?,avatarKey: freezed == avatarKey ? _self.avatarKey : avatarKey // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [Review].
extension ReviewPatterns on Review {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Review value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Review() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Review value)  $default,){
final _that = this;
switch (_that) {
case _Review():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Review value)?  $default,){
final _that = this;
switch (_that) {
case _Review() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id, @JsonKey(name: 'skuId')  int skuId, @JsonKey(name: 'orderItemId')  int? orderItemId,  int rating,  String content,  List<String> images, @JsonKey(name: 'userId')  int? userId, @JsonKey(name: 'username')  String? username, @JsonKey(name: 'avatarKey')  String? avatarKey, @JsonKey(name: 'createdAt')  String? createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Review() when $default != null:
return $default(_that.id,_that.skuId,_that.orderItemId,_that.rating,_that.content,_that.images,_that.userId,_that.username,_that.avatarKey,_that.createdAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id, @JsonKey(name: 'skuId')  int skuId, @JsonKey(name: 'orderItemId')  int? orderItemId,  int rating,  String content,  List<String> images, @JsonKey(name: 'userId')  int? userId, @JsonKey(name: 'username')  String? username, @JsonKey(name: 'avatarKey')  String? avatarKey, @JsonKey(name: 'createdAt')  String? createdAt)  $default,) {final _that = this;
switch (_that) {
case _Review():
return $default(_that.id,_that.skuId,_that.orderItemId,_that.rating,_that.content,_that.images,_that.userId,_that.username,_that.avatarKey,_that.createdAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id, @JsonKey(name: 'skuId')  int skuId, @JsonKey(name: 'orderItemId')  int? orderItemId,  int rating,  String content,  List<String> images, @JsonKey(name: 'userId')  int? userId, @JsonKey(name: 'username')  String? username, @JsonKey(name: 'avatarKey')  String? avatarKey, @JsonKey(name: 'createdAt')  String? createdAt)?  $default,) {final _that = this;
switch (_that) {
case _Review() when $default != null:
return $default(_that.id,_that.skuId,_that.orderItemId,_that.rating,_that.content,_that.images,_that.userId,_that.username,_that.avatarKey,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Review implements Review {
  const _Review({required this.id, @JsonKey(name: 'skuId') required this.skuId, @JsonKey(name: 'orderItemId') this.orderItemId, required this.rating, required this.content, final  List<String> images = const [], @JsonKey(name: 'userId') this.userId, @JsonKey(name: 'username') this.username, @JsonKey(name: 'avatarKey') this.avatarKey, @JsonKey(name: 'createdAt') this.createdAt}): _images = images;
  factory _Review.fromJson(Map<String, dynamic> json) => _$ReviewFromJson(json);

@override final  int id;
@override@JsonKey(name: 'skuId') final  int skuId;
@override@JsonKey(name: 'orderItemId') final  int? orderItemId;
@override final  int rating;
@override final  String content;
 final  List<String> _images;
@override@JsonKey() List<String> get images {
  if (_images is EqualUnmodifiableListView) return _images;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_images);
}

@override@JsonKey(name: 'userId') final  int? userId;
@override@JsonKey(name: 'username') final  String? username;
// 49 §6.1 成對契約：avatarKey 是裸 objectKey（SSOT），顯示前過 buildImageUrl。
@override@JsonKey(name: 'avatarKey') final  String? avatarKey;
@override@JsonKey(name: 'createdAt') final  String? createdAt;

/// Create a copy of Review
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ReviewCopyWith<_Review> get copyWith => __$ReviewCopyWithImpl<_Review>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$ReviewToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Review&&(identical(other.id, id) || other.id == id)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.rating, rating) || other.rating == rating)&&(identical(other.content, content) || other.content == content)&&const DeepCollectionEquality().equals(other._images, _images)&&(identical(other.userId, userId) || other.userId == userId)&&(identical(other.username, username) || other.username == username)&&(identical(other.avatarKey, avatarKey) || other.avatarKey == avatarKey)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,skuId,orderItemId,rating,content,const DeepCollectionEquality().hash(_images),userId,username,avatarKey,createdAt);

@override
String toString() {
  return 'Review(id: $id, skuId: $skuId, orderItemId: $orderItemId, rating: $rating, content: $content, images: $images, userId: $userId, username: $username, avatarKey: $avatarKey, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$ReviewCopyWith<$Res> implements $ReviewCopyWith<$Res> {
  factory _$ReviewCopyWith(_Review value, $Res Function(_Review) _then) = __$ReviewCopyWithImpl;
@override @useResult
$Res call({
 int id,@JsonKey(name: 'skuId') int skuId,@JsonKey(name: 'orderItemId') int? orderItemId, int rating, String content, List<String> images,@JsonKey(name: 'userId') int? userId,@JsonKey(name: 'username') String? username,@JsonKey(name: 'avatarKey') String? avatarKey,@JsonKey(name: 'createdAt') String? createdAt
});




}
/// @nodoc
class __$ReviewCopyWithImpl<$Res>
    implements _$ReviewCopyWith<$Res> {
  __$ReviewCopyWithImpl(this._self, this._then);

  final _Review _self;
  final $Res Function(_Review) _then;

/// Create a copy of Review
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? skuId = null,Object? orderItemId = freezed,Object? rating = null,Object? content = null,Object? images = null,Object? userId = freezed,Object? username = freezed,Object? avatarKey = freezed,Object? createdAt = freezed,}) {
  return _then(_Review(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as int,orderItemId: freezed == orderItemId ? _self.orderItemId : orderItemId // ignore: cast_nullable_to_non_nullable
as int?,rating: null == rating ? _self.rating : rating // ignore: cast_nullable_to_non_nullable
as int,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,images: null == images ? _self._images : images // ignore: cast_nullable_to_non_nullable
as List<String>,userId: freezed == userId ? _self.userId : userId // ignore: cast_nullable_to_non_nullable
as int?,username: freezed == username ? _self.username : username // ignore: cast_nullable_to_non_nullable
as String?,avatarKey: freezed == avatarKey ? _self.avatarKey : avatarKey // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
