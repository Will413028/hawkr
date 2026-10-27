// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'hashtag_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$HashtagItem {

/// canonical 名稱（後端 normalize 後的 lowercase 字串）
 String get name;/// 顯示名稱（保留原大小寫）；auto-complete 場景 server 回空字串
 String get displayName;/// 引用此 hashtag 的商品總數（即時值）
 int get productCount;
/// Create a copy of HashtagItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$HashtagItemCopyWith<HashtagItem> get copyWith => _$HashtagItemCopyWithImpl<HashtagItem>(this as HashtagItem, _$identity);

  /// Serializes this HashtagItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is HashtagItem&&(identical(other.name, name) || other.name == name)&&(identical(other.displayName, displayName) || other.displayName == displayName)&&(identical(other.productCount, productCount) || other.productCount == productCount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,name,displayName,productCount);

@override
String toString() {
  return 'HashtagItem(name: $name, displayName: $displayName, productCount: $productCount)';
}


}

/// @nodoc
abstract mixin class $HashtagItemCopyWith<$Res>  {
  factory $HashtagItemCopyWith(HashtagItem value, $Res Function(HashtagItem) _then) = _$HashtagItemCopyWithImpl;
@useResult
$Res call({
 String name, String displayName, int productCount
});




}
/// @nodoc
class _$HashtagItemCopyWithImpl<$Res>
    implements $HashtagItemCopyWith<$Res> {
  _$HashtagItemCopyWithImpl(this._self, this._then);

  final HashtagItem _self;
  final $Res Function(HashtagItem) _then;

/// Create a copy of HashtagItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? name = null,Object? displayName = null,Object? productCount = null,}) {
  return _then(_self.copyWith(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,displayName: null == displayName ? _self.displayName : displayName // ignore: cast_nullable_to_non_nullable
as String,productCount: null == productCount ? _self.productCount : productCount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [HashtagItem].
extension HashtagItemPatterns on HashtagItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _HashtagItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _HashtagItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _HashtagItem value)  $default,){
final _that = this;
switch (_that) {
case _HashtagItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _HashtagItem value)?  $default,){
final _that = this;
switch (_that) {
case _HashtagItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String name,  String displayName,  int productCount)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _HashtagItem() when $default != null:
return $default(_that.name,_that.displayName,_that.productCount);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String name,  String displayName,  int productCount)  $default,) {final _that = this;
switch (_that) {
case _HashtagItem():
return $default(_that.name,_that.displayName,_that.productCount);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String name,  String displayName,  int productCount)?  $default,) {final _that = this;
switch (_that) {
case _HashtagItem() when $default != null:
return $default(_that.name,_that.displayName,_that.productCount);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _HashtagItem implements HashtagItem {
  const _HashtagItem({required this.name, this.displayName = '', this.productCount = 0});
  factory _HashtagItem.fromJson(Map<String, dynamic> json) => _$HashtagItemFromJson(json);

/// canonical 名稱（後端 normalize 後的 lowercase 字串）
@override final  String name;
/// 顯示名稱（保留原大小寫）；auto-complete 場景 server 回空字串
@override@JsonKey() final  String displayName;
/// 引用此 hashtag 的商品總數（即時值）
@override@JsonKey() final  int productCount;

/// Create a copy of HashtagItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$HashtagItemCopyWith<_HashtagItem> get copyWith => __$HashtagItemCopyWithImpl<_HashtagItem>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$HashtagItemToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _HashtagItem&&(identical(other.name, name) || other.name == name)&&(identical(other.displayName, displayName) || other.displayName == displayName)&&(identical(other.productCount, productCount) || other.productCount == productCount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,name,displayName,productCount);

@override
String toString() {
  return 'HashtagItem(name: $name, displayName: $displayName, productCount: $productCount)';
}


}

/// @nodoc
abstract mixin class _$HashtagItemCopyWith<$Res> implements $HashtagItemCopyWith<$Res> {
  factory _$HashtagItemCopyWith(_HashtagItem value, $Res Function(_HashtagItem) _then) = __$HashtagItemCopyWithImpl;
@override @useResult
$Res call({
 String name, String displayName, int productCount
});




}
/// @nodoc
class __$HashtagItemCopyWithImpl<$Res>
    implements _$HashtagItemCopyWith<$Res> {
  __$HashtagItemCopyWithImpl(this._self, this._then);

  final _HashtagItem _self;
  final $Res Function(_HashtagItem) _then;

/// Create a copy of HashtagItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? name = null,Object? displayName = null,Object? productCount = null,}) {
  return _then(_HashtagItem(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,displayName: null == displayName ? _self.displayName : displayName // ignore: cast_nullable_to_non_nullable
as String,productCount: null == productCount ? _self.productCount : productCount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
