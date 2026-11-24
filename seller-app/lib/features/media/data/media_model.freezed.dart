// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'media_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$MediaUploadResult {

/// Staged token（`GetUpload` 為 READY 後才回傳）
 String get stagedToken;/// 上傳的位元組數
 int get size;
/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$MediaUploadResultCopyWith<MediaUploadResult> get copyWith => _$MediaUploadResultCopyWithImpl<MediaUploadResult>(this as MediaUploadResult, _$identity);

  /// Serializes this MediaUploadResult to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is MediaUploadResult&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken)&&(identical(other.size, size) || other.size == size));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,stagedToken,size);

@override
String toString() {
  return 'MediaUploadResult(stagedToken: $stagedToken, size: $size)';
}


}

/// @nodoc
abstract mixin class $MediaUploadResultCopyWith<$Res>  {
  factory $MediaUploadResultCopyWith(MediaUploadResult value, $Res Function(MediaUploadResult) _then) = _$MediaUploadResultCopyWithImpl;
@useResult
$Res call({
 String stagedToken, int size
});




}
/// @nodoc
class _$MediaUploadResultCopyWithImpl<$Res>
    implements $MediaUploadResultCopyWith<$Res> {
  _$MediaUploadResultCopyWithImpl(this._self, this._then);

  final MediaUploadResult _self;
  final $Res Function(MediaUploadResult) _then;

/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? stagedToken = null,Object? size = null,}) {
  return _then(_self.copyWith(
stagedToken: null == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String,size: null == size ? _self.size : size // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [MediaUploadResult].
extension MediaUploadResultPatterns on MediaUploadResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _MediaUploadResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _MediaUploadResult value)  $default,){
final _that = this;
switch (_that) {
case _MediaUploadResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _MediaUploadResult value)?  $default,){
final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String stagedToken,  int size)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
return $default(_that.stagedToken,_that.size);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String stagedToken,  int size)  $default,) {final _that = this;
switch (_that) {
case _MediaUploadResult():
return $default(_that.stagedToken,_that.size);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String stagedToken,  int size)?  $default,) {final _that = this;
switch (_that) {
case _MediaUploadResult() when $default != null:
return $default(_that.stagedToken,_that.size);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _MediaUploadResult implements MediaUploadResult {
  const _MediaUploadResult({required this.stagedToken, required this.size});
  factory _MediaUploadResult.fromJson(Map<String, dynamic> json) => _$MediaUploadResultFromJson(json);

/// Staged token（`GetUpload` 為 READY 後才回傳）
@override final  String stagedToken;
/// 上傳的位元組數
@override final  int size;

/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$MediaUploadResultCopyWith<_MediaUploadResult> get copyWith => __$MediaUploadResultCopyWithImpl<_MediaUploadResult>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$MediaUploadResultToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _MediaUploadResult&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken)&&(identical(other.size, size) || other.size == size));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,stagedToken,size);

@override
String toString() {
  return 'MediaUploadResult(stagedToken: $stagedToken, size: $size)';
}


}

/// @nodoc
abstract mixin class _$MediaUploadResultCopyWith<$Res> implements $MediaUploadResultCopyWith<$Res> {
  factory _$MediaUploadResultCopyWith(_MediaUploadResult value, $Res Function(_MediaUploadResult) _then) = __$MediaUploadResultCopyWithImpl;
@override @useResult
$Res call({
 String stagedToken, int size
});




}
/// @nodoc
class __$MediaUploadResultCopyWithImpl<$Res>
    implements _$MediaUploadResultCopyWith<$Res> {
  __$MediaUploadResultCopyWithImpl(this._self, this._then);

  final _MediaUploadResult _self;
  final $Res Function(_MediaUploadResult) _then;

/// Create a copy of MediaUploadResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? stagedToken = null,Object? size = null,}) {
  return _then(_MediaUploadResult(
stagedToken: null == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String,size: null == size ? _self.size : size // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
