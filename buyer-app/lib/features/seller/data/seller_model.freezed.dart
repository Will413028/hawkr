// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'seller_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$RatingStats {

// 44 號身分分離：賣家業務鍵是 store surrogate。
@JsonKey(name: 'storeId') String? get storeId;@JsonKey(name: 'rating') double? get average;@JsonKey(name: 'reviewCount') int? get total;// 星級為百分比（omitempty：全 0 時不出現）。
@JsonKey(name: 'fiveStarPct') double? get fiveStarPct;@JsonKey(name: 'fourStarPct') double? get fourStarPct;@JsonKey(name: 'threeStarPct') double? get threeStarPct;@JsonKey(name: 'twoStarPct') double? get twoStarPct;@JsonKey(name: 'oneStarPct') double? get oneStarPct;
/// Create a copy of RatingStats
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$RatingStatsCopyWith<RatingStats> get copyWith => _$RatingStatsCopyWithImpl<RatingStats>(this as RatingStats, _$identity);

  /// Serializes this RatingStats to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RatingStats&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.average, average) || other.average == average)&&(identical(other.total, total) || other.total == total)&&(identical(other.fiveStarPct, fiveStarPct) || other.fiveStarPct == fiveStarPct)&&(identical(other.fourStarPct, fourStarPct) || other.fourStarPct == fourStarPct)&&(identical(other.threeStarPct, threeStarPct) || other.threeStarPct == threeStarPct)&&(identical(other.twoStarPct, twoStarPct) || other.twoStarPct == twoStarPct)&&(identical(other.oneStarPct, oneStarPct) || other.oneStarPct == oneStarPct));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,storeId,average,total,fiveStarPct,fourStarPct,threeStarPct,twoStarPct,oneStarPct);

@override
String toString() {
  return 'RatingStats(storeId: $storeId, average: $average, total: $total, fiveStarPct: $fiveStarPct, fourStarPct: $fourStarPct, threeStarPct: $threeStarPct, twoStarPct: $twoStarPct, oneStarPct: $oneStarPct)';
}


}

/// @nodoc
abstract mixin class $RatingStatsCopyWith<$Res>  {
  factory $RatingStatsCopyWith(RatingStats value, $Res Function(RatingStats) _then) = _$RatingStatsCopyWithImpl;
@useResult
$Res call({
@JsonKey(name: 'storeId') String? storeId,@JsonKey(name: 'rating') double? average,@JsonKey(name: 'reviewCount') int? total,@JsonKey(name: 'fiveStarPct') double? fiveStarPct,@JsonKey(name: 'fourStarPct') double? fourStarPct,@JsonKey(name: 'threeStarPct') double? threeStarPct,@JsonKey(name: 'twoStarPct') double? twoStarPct,@JsonKey(name: 'oneStarPct') double? oneStarPct
});




}
/// @nodoc
class _$RatingStatsCopyWithImpl<$Res>
    implements $RatingStatsCopyWith<$Res> {
  _$RatingStatsCopyWithImpl(this._self, this._then);

  final RatingStats _self;
  final $Res Function(RatingStats) _then;

/// Create a copy of RatingStats
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? storeId = freezed,Object? average = freezed,Object? total = freezed,Object? fiveStarPct = freezed,Object? fourStarPct = freezed,Object? threeStarPct = freezed,Object? twoStarPct = freezed,Object? oneStarPct = freezed,}) {
  return _then(_self.copyWith(
storeId: freezed == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String?,average: freezed == average ? _self.average : average // ignore: cast_nullable_to_non_nullable
as double?,total: freezed == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int?,fiveStarPct: freezed == fiveStarPct ? _self.fiveStarPct : fiveStarPct // ignore: cast_nullable_to_non_nullable
as double?,fourStarPct: freezed == fourStarPct ? _self.fourStarPct : fourStarPct // ignore: cast_nullable_to_non_nullable
as double?,threeStarPct: freezed == threeStarPct ? _self.threeStarPct : threeStarPct // ignore: cast_nullable_to_non_nullable
as double?,twoStarPct: freezed == twoStarPct ? _self.twoStarPct : twoStarPct // ignore: cast_nullable_to_non_nullable
as double?,oneStarPct: freezed == oneStarPct ? _self.oneStarPct : oneStarPct // ignore: cast_nullable_to_non_nullable
as double?,
  ));
}

}


/// Adds pattern-matching-related methods to [RatingStats].
extension RatingStatsPatterns on RatingStats {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _RatingStats value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _RatingStats() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _RatingStats value)  $default,){
final _that = this;
switch (_that) {
case _RatingStats():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _RatingStats value)?  $default,){
final _that = this;
switch (_that) {
case _RatingStats() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(name: 'storeId')  String? storeId, @JsonKey(name: 'rating')  double? average, @JsonKey(name: 'reviewCount')  int? total, @JsonKey(name: 'fiveStarPct')  double? fiveStarPct, @JsonKey(name: 'fourStarPct')  double? fourStarPct, @JsonKey(name: 'threeStarPct')  double? threeStarPct, @JsonKey(name: 'twoStarPct')  double? twoStarPct, @JsonKey(name: 'oneStarPct')  double? oneStarPct)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _RatingStats() when $default != null:
return $default(_that.storeId,_that.average,_that.total,_that.fiveStarPct,_that.fourStarPct,_that.threeStarPct,_that.twoStarPct,_that.oneStarPct);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(name: 'storeId')  String? storeId, @JsonKey(name: 'rating')  double? average, @JsonKey(name: 'reviewCount')  int? total, @JsonKey(name: 'fiveStarPct')  double? fiveStarPct, @JsonKey(name: 'fourStarPct')  double? fourStarPct, @JsonKey(name: 'threeStarPct')  double? threeStarPct, @JsonKey(name: 'twoStarPct')  double? twoStarPct, @JsonKey(name: 'oneStarPct')  double? oneStarPct)  $default,) {final _that = this;
switch (_that) {
case _RatingStats():
return $default(_that.storeId,_that.average,_that.total,_that.fiveStarPct,_that.fourStarPct,_that.threeStarPct,_that.twoStarPct,_that.oneStarPct);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(name: 'storeId')  String? storeId, @JsonKey(name: 'rating')  double? average, @JsonKey(name: 'reviewCount')  int? total, @JsonKey(name: 'fiveStarPct')  double? fiveStarPct, @JsonKey(name: 'fourStarPct')  double? fourStarPct, @JsonKey(name: 'threeStarPct')  double? threeStarPct, @JsonKey(name: 'twoStarPct')  double? twoStarPct, @JsonKey(name: 'oneStarPct')  double? oneStarPct)?  $default,) {final _that = this;
switch (_that) {
case _RatingStats() when $default != null:
return $default(_that.storeId,_that.average,_that.total,_that.fiveStarPct,_that.fourStarPct,_that.threeStarPct,_that.twoStarPct,_that.oneStarPct);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _RatingStats implements RatingStats {
  const _RatingStats({@JsonKey(name: 'storeId') this.storeId, @JsonKey(name: 'rating') this.average, @JsonKey(name: 'reviewCount') this.total, @JsonKey(name: 'fiveStarPct') this.fiveStarPct, @JsonKey(name: 'fourStarPct') this.fourStarPct, @JsonKey(name: 'threeStarPct') this.threeStarPct, @JsonKey(name: 'twoStarPct') this.twoStarPct, @JsonKey(name: 'oneStarPct') this.oneStarPct});
  factory _RatingStats.fromJson(Map<String, dynamic> json) => _$RatingStatsFromJson(json);

// 44 號身分分離：賣家業務鍵是 store surrogate。
@override@JsonKey(name: 'storeId') final  String? storeId;
@override@JsonKey(name: 'rating') final  double? average;
@override@JsonKey(name: 'reviewCount') final  int? total;
// 星級為百分比（omitempty：全 0 時不出現）。
@override@JsonKey(name: 'fiveStarPct') final  double? fiveStarPct;
@override@JsonKey(name: 'fourStarPct') final  double? fourStarPct;
@override@JsonKey(name: 'threeStarPct') final  double? threeStarPct;
@override@JsonKey(name: 'twoStarPct') final  double? twoStarPct;
@override@JsonKey(name: 'oneStarPct') final  double? oneStarPct;

/// Create a copy of RatingStats
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$RatingStatsCopyWith<_RatingStats> get copyWith => __$RatingStatsCopyWithImpl<_RatingStats>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$RatingStatsToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _RatingStats&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.average, average) || other.average == average)&&(identical(other.total, total) || other.total == total)&&(identical(other.fiveStarPct, fiveStarPct) || other.fiveStarPct == fiveStarPct)&&(identical(other.fourStarPct, fourStarPct) || other.fourStarPct == fourStarPct)&&(identical(other.threeStarPct, threeStarPct) || other.threeStarPct == threeStarPct)&&(identical(other.twoStarPct, twoStarPct) || other.twoStarPct == twoStarPct)&&(identical(other.oneStarPct, oneStarPct) || other.oneStarPct == oneStarPct));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,storeId,average,total,fiveStarPct,fourStarPct,threeStarPct,twoStarPct,oneStarPct);

@override
String toString() {
  return 'RatingStats(storeId: $storeId, average: $average, total: $total, fiveStarPct: $fiveStarPct, fourStarPct: $fourStarPct, threeStarPct: $threeStarPct, twoStarPct: $twoStarPct, oneStarPct: $oneStarPct)';
}


}

/// @nodoc
abstract mixin class _$RatingStatsCopyWith<$Res> implements $RatingStatsCopyWith<$Res> {
  factory _$RatingStatsCopyWith(_RatingStats value, $Res Function(_RatingStats) _then) = __$RatingStatsCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(name: 'storeId') String? storeId,@JsonKey(name: 'rating') double? average,@JsonKey(name: 'reviewCount') int? total,@JsonKey(name: 'fiveStarPct') double? fiveStarPct,@JsonKey(name: 'fourStarPct') double? fourStarPct,@JsonKey(name: 'threeStarPct') double? threeStarPct,@JsonKey(name: 'twoStarPct') double? twoStarPct,@JsonKey(name: 'oneStarPct') double? oneStarPct
});




}
/// @nodoc
class __$RatingStatsCopyWithImpl<$Res>
    implements _$RatingStatsCopyWith<$Res> {
  __$RatingStatsCopyWithImpl(this._self, this._then);

  final _RatingStats _self;
  final $Res Function(_RatingStats) _then;

/// Create a copy of RatingStats
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? storeId = freezed,Object? average = freezed,Object? total = freezed,Object? fiveStarPct = freezed,Object? fourStarPct = freezed,Object? threeStarPct = freezed,Object? twoStarPct = freezed,Object? oneStarPct = freezed,}) {
  return _then(_RatingStats(
storeId: freezed == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String?,average: freezed == average ? _self.average : average // ignore: cast_nullable_to_non_nullable
as double?,total: freezed == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int?,fiveStarPct: freezed == fiveStarPct ? _self.fiveStarPct : fiveStarPct // ignore: cast_nullable_to_non_nullable
as double?,fourStarPct: freezed == fourStarPct ? _self.fourStarPct : fourStarPct // ignore: cast_nullable_to_non_nullable
as double?,threeStarPct: freezed == threeStarPct ? _self.threeStarPct : threeStarPct // ignore: cast_nullable_to_non_nullable
as double?,twoStarPct: freezed == twoStarPct ? _self.twoStarPct : twoStarPct // ignore: cast_nullable_to_non_nullable
as double?,oneStarPct: freezed == oneStarPct ? _self.oneStarPct : oneStarPct // ignore: cast_nullable_to_non_nullable
as double?,
  ));
}


}

// dart format on
