// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'address_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$Address {

 int get id; String get etag; String get receiverName; String get receiverPhone;// 內政部 8 碼鄉鎮市區代碼與其後的明細（postal_address）；縣市、郵遞區號、名稱由 hawkr_twaddr 依代碼取得。
 String get townCode; String get addressDetail; bool get isDefault;
/// Create a copy of Address
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AddressCopyWith<Address> get copyWith => _$AddressCopyWithImpl<Address>(this as Address, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Address&&(identical(other.id, id) || other.id == id)&&(identical(other.etag, etag) || other.etag == etag)&&(identical(other.receiverName, receiverName) || other.receiverName == receiverName)&&(identical(other.receiverPhone, receiverPhone) || other.receiverPhone == receiverPhone)&&(identical(other.townCode, townCode) || other.townCode == townCode)&&(identical(other.addressDetail, addressDetail) || other.addressDetail == addressDetail)&&(identical(other.isDefault, isDefault) || other.isDefault == isDefault));
}


@override
int get hashCode => Object.hash(runtimeType,id,etag,receiverName,receiverPhone,townCode,addressDetail,isDefault);

@override
String toString() {
  return 'Address(id: $id, etag: $etag, receiverName: $receiverName, receiverPhone: $receiverPhone, townCode: $townCode, addressDetail: $addressDetail, isDefault: $isDefault)';
}


}

/// @nodoc
abstract mixin class $AddressCopyWith<$Res>  {
  factory $AddressCopyWith(Address value, $Res Function(Address) _then) = _$AddressCopyWithImpl;
@useResult
$Res call({
 int id, String etag, String receiverName, String receiverPhone, String townCode, String addressDetail, bool isDefault
});




}
/// @nodoc
class _$AddressCopyWithImpl<$Res>
    implements $AddressCopyWith<$Res> {
  _$AddressCopyWithImpl(this._self, this._then);

  final Address _self;
  final $Res Function(Address) _then;

/// Create a copy of Address
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? etag = null,Object? receiverName = null,Object? receiverPhone = null,Object? townCode = null,Object? addressDetail = null,Object? isDefault = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,etag: null == etag ? _self.etag : etag // ignore: cast_nullable_to_non_nullable
as String,receiverName: null == receiverName ? _self.receiverName : receiverName // ignore: cast_nullable_to_non_nullable
as String,receiverPhone: null == receiverPhone ? _self.receiverPhone : receiverPhone // ignore: cast_nullable_to_non_nullable
as String,townCode: null == townCode ? _self.townCode : townCode // ignore: cast_nullable_to_non_nullable
as String,addressDetail: null == addressDetail ? _self.addressDetail : addressDetail // ignore: cast_nullable_to_non_nullable
as String,isDefault: null == isDefault ? _self.isDefault : isDefault // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [Address].
extension AddressPatterns on Address {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Address value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Address() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Address value)  $default,){
final _that = this;
switch (_that) {
case _Address():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Address value)?  $default,){
final _that = this;
switch (_that) {
case _Address() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id,  String etag,  String receiverName,  String receiverPhone,  String townCode,  String addressDetail,  bool isDefault)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Address() when $default != null:
return $default(_that.id,_that.etag,_that.receiverName,_that.receiverPhone,_that.townCode,_that.addressDetail,_that.isDefault);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id,  String etag,  String receiverName,  String receiverPhone,  String townCode,  String addressDetail,  bool isDefault)  $default,) {final _that = this;
switch (_that) {
case _Address():
return $default(_that.id,_that.etag,_that.receiverName,_that.receiverPhone,_that.townCode,_that.addressDetail,_that.isDefault);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id,  String etag,  String receiverName,  String receiverPhone,  String townCode,  String addressDetail,  bool isDefault)?  $default,) {final _that = this;
switch (_that) {
case _Address() when $default != null:
return $default(_that.id,_that.etag,_that.receiverName,_that.receiverPhone,_that.townCode,_that.addressDetail,_that.isDefault);case _:
  return null;

}
}

}

/// @nodoc


class _Address implements Address {
  const _Address({required this.id, required this.etag, required this.receiverName, required this.receiverPhone, this.townCode = '', this.addressDetail = '', this.isDefault = false});
  

@override final  int id;
@override final  String etag;
@override final  String receiverName;
@override final  String receiverPhone;
// 內政部 8 碼鄉鎮市區代碼與其後的明細（postal_address）；縣市、郵遞區號、名稱由 hawkr_twaddr 依代碼取得。
@override@JsonKey() final  String townCode;
@override@JsonKey() final  String addressDetail;
@override@JsonKey() final  bool isDefault;

/// Create a copy of Address
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AddressCopyWith<_Address> get copyWith => __$AddressCopyWithImpl<_Address>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Address&&(identical(other.id, id) || other.id == id)&&(identical(other.etag, etag) || other.etag == etag)&&(identical(other.receiverName, receiverName) || other.receiverName == receiverName)&&(identical(other.receiverPhone, receiverPhone) || other.receiverPhone == receiverPhone)&&(identical(other.townCode, townCode) || other.townCode == townCode)&&(identical(other.addressDetail, addressDetail) || other.addressDetail == addressDetail)&&(identical(other.isDefault, isDefault) || other.isDefault == isDefault));
}


@override
int get hashCode => Object.hash(runtimeType,id,etag,receiverName,receiverPhone,townCode,addressDetail,isDefault);

@override
String toString() {
  return 'Address(id: $id, etag: $etag, receiverName: $receiverName, receiverPhone: $receiverPhone, townCode: $townCode, addressDetail: $addressDetail, isDefault: $isDefault)';
}


}

/// @nodoc
abstract mixin class _$AddressCopyWith<$Res> implements $AddressCopyWith<$Res> {
  factory _$AddressCopyWith(_Address value, $Res Function(_Address) _then) = __$AddressCopyWithImpl;
@override @useResult
$Res call({
 int id, String etag, String receiverName, String receiverPhone, String townCode, String addressDetail, bool isDefault
});




}
/// @nodoc
class __$AddressCopyWithImpl<$Res>
    implements _$AddressCopyWith<$Res> {
  __$AddressCopyWithImpl(this._self, this._then);

  final _Address _self;
  final $Res Function(_Address) _then;

/// Create a copy of Address
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? etag = null,Object? receiverName = null,Object? receiverPhone = null,Object? townCode = null,Object? addressDetail = null,Object? isDefault = null,}) {
  return _then(_Address(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,etag: null == etag ? _self.etag : etag // ignore: cast_nullable_to_non_nullable
as String,receiverName: null == receiverName ? _self.receiverName : receiverName // ignore: cast_nullable_to_non_nullable
as String,receiverPhone: null == receiverPhone ? _self.receiverPhone : receiverPhone // ignore: cast_nullable_to_non_nullable
as String,townCode: null == townCode ? _self.townCode : townCode // ignore: cast_nullable_to_non_nullable
as String,addressDetail: null == addressDetail ? _self.addressDetail : addressDetail // ignore: cast_nullable_to_non_nullable
as String,isDefault: null == isDefault ? _self.isDefault : isDefault // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}

// dart format on
