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
mixin _$UploadResult {

/// 可讀的圖片預覽 URL
 String get previewUrl;/// 暫存 Token，用於後端建立/更新資源時傳遞
 String get stagedToken;
/// Create a copy of UploadResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$UploadResultCopyWith<UploadResult> get copyWith => _$UploadResultCopyWithImpl<UploadResult>(this as UploadResult, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is UploadResult&&(identical(other.previewUrl, previewUrl) || other.previewUrl == previewUrl)&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken));
}


@override
int get hashCode => Object.hash(runtimeType,previewUrl,stagedToken);

@override
String toString() {
  return 'UploadResult(previewUrl: $previewUrl, stagedToken: $stagedToken)';
}


}

/// @nodoc
abstract mixin class $UploadResultCopyWith<$Res>  {
  factory $UploadResultCopyWith(UploadResult value, $Res Function(UploadResult) _then) = _$UploadResultCopyWithImpl;
@useResult
$Res call({
 String previewUrl, String stagedToken
});




}
/// @nodoc
class _$UploadResultCopyWithImpl<$Res>
    implements $UploadResultCopyWith<$Res> {
  _$UploadResultCopyWithImpl(this._self, this._then);

  final UploadResult _self;
  final $Res Function(UploadResult) _then;

/// Create a copy of UploadResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? previewUrl = null,Object? stagedToken = null,}) {
  return _then(_self.copyWith(
previewUrl: null == previewUrl ? _self.previewUrl : previewUrl // ignore: cast_nullable_to_non_nullable
as String,stagedToken: null == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [UploadResult].
extension UploadResultPatterns on UploadResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _UploadResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _UploadResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _UploadResult value)  $default,){
final _that = this;
switch (_that) {
case _UploadResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _UploadResult value)?  $default,){
final _that = this;
switch (_that) {
case _UploadResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String previewUrl,  String stagedToken)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _UploadResult() when $default != null:
return $default(_that.previewUrl,_that.stagedToken);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String previewUrl,  String stagedToken)  $default,) {final _that = this;
switch (_that) {
case _UploadResult():
return $default(_that.previewUrl,_that.stagedToken);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String previewUrl,  String stagedToken)?  $default,) {final _that = this;
switch (_that) {
case _UploadResult() when $default != null:
return $default(_that.previewUrl,_that.stagedToken);case _:
  return null;

}
}

}

/// @nodoc


class _UploadResult implements UploadResult {
  const _UploadResult({required this.previewUrl, required this.stagedToken});
  

/// 可讀的圖片預覽 URL
@override final  String previewUrl;
/// 暫存 Token，用於後端建立/更新資源時傳遞
@override final  String stagedToken;

/// Create a copy of UploadResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$UploadResultCopyWith<_UploadResult> get copyWith => __$UploadResultCopyWithImpl<_UploadResult>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _UploadResult&&(identical(other.previewUrl, previewUrl) || other.previewUrl == previewUrl)&&(identical(other.stagedToken, stagedToken) || other.stagedToken == stagedToken));
}


@override
int get hashCode => Object.hash(runtimeType,previewUrl,stagedToken);

@override
String toString() {
  return 'UploadResult(previewUrl: $previewUrl, stagedToken: $stagedToken)';
}


}

/// @nodoc
abstract mixin class _$UploadResultCopyWith<$Res> implements $UploadResultCopyWith<$Res> {
  factory _$UploadResultCopyWith(_UploadResult value, $Res Function(_UploadResult) _then) = __$UploadResultCopyWithImpl;
@override @useResult
$Res call({
 String previewUrl, String stagedToken
});




}
/// @nodoc
class __$UploadResultCopyWithImpl<$Res>
    implements _$UploadResultCopyWith<$Res> {
  __$UploadResultCopyWithImpl(this._self, this._then);

  final _UploadResult _self;
  final $Res Function(_UploadResult) _then;

/// Create a copy of UploadResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? previewUrl = null,Object? stagedToken = null,}) {
  return _then(_UploadResult(
previewUrl: null == previewUrl ? _self.previewUrl : previewUrl // ignore: cast_nullable_to_non_nullable
as String,stagedToken: null == stagedToken ? _self.stagedToken : stagedToken // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
