// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'customer_controller.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$CustomerFilter {

/// 'normal' / 'vip' / 'blocklist' / null (全部)
 String? get customerType; String get sortBy; String? get keyword; int? get tagId;
/// Create a copy of CustomerFilter
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CustomerFilterCopyWith<CustomerFilter> get copyWith => _$CustomerFilterCopyWithImpl<CustomerFilter>(this as CustomerFilter, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CustomerFilter&&(identical(other.customerType, customerType) || other.customerType == customerType)&&(identical(other.sortBy, sortBy) || other.sortBy == sortBy)&&(identical(other.keyword, keyword) || other.keyword == keyword)&&(identical(other.tagId, tagId) || other.tagId == tagId));
}


@override
int get hashCode => Object.hash(runtimeType,customerType,sortBy,keyword,tagId);

@override
String toString() {
  return 'CustomerFilter(customerType: $customerType, sortBy: $sortBy, keyword: $keyword, tagId: $tagId)';
}


}

/// @nodoc
abstract mixin class $CustomerFilterCopyWith<$Res>  {
  factory $CustomerFilterCopyWith(CustomerFilter value, $Res Function(CustomerFilter) _then) = _$CustomerFilterCopyWithImpl;
@useResult
$Res call({
 String? customerType, String sortBy, String? keyword, int? tagId
});




}
/// @nodoc
class _$CustomerFilterCopyWithImpl<$Res>
    implements $CustomerFilterCopyWith<$Res> {
  _$CustomerFilterCopyWithImpl(this._self, this._then);

  final CustomerFilter _self;
  final $Res Function(CustomerFilter) _then;

/// Create a copy of CustomerFilter
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? customerType = freezed,Object? sortBy = null,Object? keyword = freezed,Object? tagId = freezed,}) {
  return _then(_self.copyWith(
customerType: freezed == customerType ? _self.customerType : customerType // ignore: cast_nullable_to_non_nullable
as String?,sortBy: null == sortBy ? _self.sortBy : sortBy // ignore: cast_nullable_to_non_nullable
as String,keyword: freezed == keyword ? _self.keyword : keyword // ignore: cast_nullable_to_non_nullable
as String?,tagId: freezed == tagId ? _self.tagId : tagId // ignore: cast_nullable_to_non_nullable
as int?,
  ));
}

}


/// Adds pattern-matching-related methods to [CustomerFilter].
extension CustomerFilterPatterns on CustomerFilter {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CustomerFilter value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CustomerFilter() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CustomerFilter value)  $default,){
final _that = this;
switch (_that) {
case _CustomerFilter():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CustomerFilter value)?  $default,){
final _that = this;
switch (_that) {
case _CustomerFilter() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String? customerType,  String sortBy,  String? keyword,  int? tagId)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CustomerFilter() when $default != null:
return $default(_that.customerType,_that.sortBy,_that.keyword,_that.tagId);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String? customerType,  String sortBy,  String? keyword,  int? tagId)  $default,) {final _that = this;
switch (_that) {
case _CustomerFilter():
return $default(_that.customerType,_that.sortBy,_that.keyword,_that.tagId);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String? customerType,  String sortBy,  String? keyword,  int? tagId)?  $default,) {final _that = this;
switch (_that) {
case _CustomerFilter() when $default != null:
return $default(_that.customerType,_that.sortBy,_that.keyword,_that.tagId);case _:
  return null;

}
}

}

/// @nodoc


class _CustomerFilter implements CustomerFilter {
  const _CustomerFilter({this.customerType, this.sortBy = 'visitCount', this.keyword, this.tagId});
  

/// 'normal' / 'vip' / 'blocklist' / null (全部)
@override final  String? customerType;
@override@JsonKey() final  String sortBy;
@override final  String? keyword;
@override final  int? tagId;

/// Create a copy of CustomerFilter
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CustomerFilterCopyWith<_CustomerFilter> get copyWith => __$CustomerFilterCopyWithImpl<_CustomerFilter>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CustomerFilter&&(identical(other.customerType, customerType) || other.customerType == customerType)&&(identical(other.sortBy, sortBy) || other.sortBy == sortBy)&&(identical(other.keyword, keyword) || other.keyword == keyword)&&(identical(other.tagId, tagId) || other.tagId == tagId));
}


@override
int get hashCode => Object.hash(runtimeType,customerType,sortBy,keyword,tagId);

@override
String toString() {
  return 'CustomerFilter(customerType: $customerType, sortBy: $sortBy, keyword: $keyword, tagId: $tagId)';
}


}

/// @nodoc
abstract mixin class _$CustomerFilterCopyWith<$Res> implements $CustomerFilterCopyWith<$Res> {
  factory _$CustomerFilterCopyWith(_CustomerFilter value, $Res Function(_CustomerFilter) _then) = __$CustomerFilterCopyWithImpl;
@override @useResult
$Res call({
 String? customerType, String sortBy, String? keyword, int? tagId
});




}
/// @nodoc
class __$CustomerFilterCopyWithImpl<$Res>
    implements _$CustomerFilterCopyWith<$Res> {
  __$CustomerFilterCopyWithImpl(this._self, this._then);

  final _CustomerFilter _self;
  final $Res Function(_CustomerFilter) _then;

/// Create a copy of CustomerFilter
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? customerType = freezed,Object? sortBy = null,Object? keyword = freezed,Object? tagId = freezed,}) {
  return _then(_CustomerFilter(
customerType: freezed == customerType ? _self.customerType : customerType // ignore: cast_nullable_to_non_nullable
as String?,sortBy: null == sortBy ? _self.sortBy : sortBy // ignore: cast_nullable_to_non_nullable
as String,keyword: freezed == keyword ? _self.keyword : keyword // ignore: cast_nullable_to_non_nullable
as String?,tagId: freezed == tagId ? _self.tagId : tagId // ignore: cast_nullable_to_non_nullable
as int?,
  ));
}


}

// dart format on
