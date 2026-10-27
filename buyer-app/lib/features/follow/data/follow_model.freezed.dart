// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'follow_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$FansCount {

@JsonKey(name: 'merchantId') int? get merchantId; int get count;
/// Create a copy of FansCount
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$FansCountCopyWith<FansCount> get copyWith => _$FansCountCopyWithImpl<FansCount>(this as FansCount, _$identity);

  /// Serializes this FansCount to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is FansCount&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.count, count) || other.count == count));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,merchantId,count);

@override
String toString() {
  return 'FansCount(merchantId: $merchantId, count: $count)';
}


}

/// @nodoc
abstract mixin class $FansCountCopyWith<$Res>  {
  factory $FansCountCopyWith(FansCount value, $Res Function(FansCount) _then) = _$FansCountCopyWithImpl;
@useResult
$Res call({
@JsonKey(name: 'merchantId') int? merchantId, int count
});




}
/// @nodoc
class _$FansCountCopyWithImpl<$Res>
    implements $FansCountCopyWith<$Res> {
  _$FansCountCopyWithImpl(this._self, this._then);

  final FansCount _self;
  final $Res Function(FansCount) _then;

/// Create a copy of FansCount
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? merchantId = freezed,Object? count = null,}) {
  return _then(_self.copyWith(
merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as int?,count: null == count ? _self.count : count // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [FansCount].
extension FansCountPatterns on FansCount {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _FansCount value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _FansCount() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _FansCount value)  $default,){
final _that = this;
switch (_that) {
case _FansCount():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _FansCount value)?  $default,){
final _that = this;
switch (_that) {
case _FansCount() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(name: 'merchantId')  int? merchantId,  int count)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _FansCount() when $default != null:
return $default(_that.merchantId,_that.count);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(name: 'merchantId')  int? merchantId,  int count)  $default,) {final _that = this;
switch (_that) {
case _FansCount():
return $default(_that.merchantId,_that.count);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(name: 'merchantId')  int? merchantId,  int count)?  $default,) {final _that = this;
switch (_that) {
case _FansCount() when $default != null:
return $default(_that.merchantId,_that.count);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _FansCount implements FansCount {
  const _FansCount({@JsonKey(name: 'merchantId') this.merchantId, this.count = 0});
  factory _FansCount.fromJson(Map<String, dynamic> json) => _$FansCountFromJson(json);

@override@JsonKey(name: 'merchantId') final  int? merchantId;
@override@JsonKey() final  int count;

/// Create a copy of FansCount
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$FansCountCopyWith<_FansCount> get copyWith => __$FansCountCopyWithImpl<_FansCount>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$FansCountToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _FansCount&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.count, count) || other.count == count));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,merchantId,count);

@override
String toString() {
  return 'FansCount(merchantId: $merchantId, count: $count)';
}


}

/// @nodoc
abstract mixin class _$FansCountCopyWith<$Res> implements $FansCountCopyWith<$Res> {
  factory _$FansCountCopyWith(_FansCount value, $Res Function(_FansCount) _then) = __$FansCountCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(name: 'merchantId') int? merchantId, int count
});




}
/// @nodoc
class __$FansCountCopyWithImpl<$Res>
    implements _$FansCountCopyWith<$Res> {
  __$FansCountCopyWithImpl(this._self, this._then);

  final _FansCount _self;
  final $Res Function(_FansCount) _then;

/// Create a copy of FansCount
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? merchantId = freezed,Object? count = null,}) {
  return _then(_FansCount(
merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as int?,count: null == count ? _self.count : count // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
