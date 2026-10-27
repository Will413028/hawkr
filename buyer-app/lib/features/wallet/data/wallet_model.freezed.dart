// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'wallet_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$Coupon {

/// `instanceId`（已使用券）或 `templateId`（可用券）。
 String get id; String get name; int? get status; String? get createdAt; num? get discountAmount; num? get discountPercent; num? get maxDiscountAmount; num? get thresholdAmount; num? get usageLimitPerAccount; String? get startAt; String? get endAt; String? get sellerName; String? get storeId;/// 商家 logo 來源：`logoKey`（裸 objectKey，SSOT）優先、`logoUrl` 退路。
/// 顯示端一律帶 scene 過 `buildImageUrl`（dual-mode）。
 String? get sellerAvatarUrl;
/// Create a copy of Coupon
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CouponCopyWith<Coupon> get copyWith => _$CouponCopyWithImpl<Coupon>(this as Coupon, _$identity);

  /// Serializes this Coupon to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Coupon&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.status, status) || other.status == status)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.discountPercent, discountPercent) || other.discountPercent == discountPercent)&&(identical(other.maxDiscountAmount, maxDiscountAmount) || other.maxDiscountAmount == maxDiscountAmount)&&(identical(other.thresholdAmount, thresholdAmount) || other.thresholdAmount == thresholdAmount)&&(identical(other.usageLimitPerAccount, usageLimitPerAccount) || other.usageLimitPerAccount == usageLimitPerAccount)&&(identical(other.startAt, startAt) || other.startAt == startAt)&&(identical(other.endAt, endAt) || other.endAt == endAt)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.sellerAvatarUrl, sellerAvatarUrl) || other.sellerAvatarUrl == sellerAvatarUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name,status,createdAt,discountAmount,discountPercent,maxDiscountAmount,thresholdAmount,usageLimitPerAccount,startAt,endAt,sellerName,storeId,sellerAvatarUrl);

@override
String toString() {
  return 'Coupon(id: $id, name: $name, status: $status, createdAt: $createdAt, discountAmount: $discountAmount, discountPercent: $discountPercent, maxDiscountAmount: $maxDiscountAmount, thresholdAmount: $thresholdAmount, usageLimitPerAccount: $usageLimitPerAccount, startAt: $startAt, endAt: $endAt, sellerName: $sellerName, storeId: $storeId, sellerAvatarUrl: $sellerAvatarUrl)';
}


}

/// @nodoc
abstract mixin class $CouponCopyWith<$Res>  {
  factory $CouponCopyWith(Coupon value, $Res Function(Coupon) _then) = _$CouponCopyWithImpl;
@useResult
$Res call({
 String id, String name, int? status, String? createdAt, num? discountAmount, num? discountPercent, num? maxDiscountAmount, num? thresholdAmount, num? usageLimitPerAccount, String? startAt, String? endAt, String? sellerName, String? storeId, String? sellerAvatarUrl
});




}
/// @nodoc
class _$CouponCopyWithImpl<$Res>
    implements $CouponCopyWith<$Res> {
  _$CouponCopyWithImpl(this._self, this._then);

  final Coupon _self;
  final $Res Function(Coupon) _then;

/// Create a copy of Coupon
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? name = null,Object? status = freezed,Object? createdAt = freezed,Object? discountAmount = freezed,Object? discountPercent = freezed,Object? maxDiscountAmount = freezed,Object? thresholdAmount = freezed,Object? usageLimitPerAccount = freezed,Object? startAt = freezed,Object? endAt = freezed,Object? sellerName = freezed,Object? storeId = freezed,Object? sellerAvatarUrl = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,status: freezed == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as int?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String?,discountAmount: freezed == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as num?,discountPercent: freezed == discountPercent ? _self.discountPercent : discountPercent // ignore: cast_nullable_to_non_nullable
as num?,maxDiscountAmount: freezed == maxDiscountAmount ? _self.maxDiscountAmount : maxDiscountAmount // ignore: cast_nullable_to_non_nullable
as num?,thresholdAmount: freezed == thresholdAmount ? _self.thresholdAmount : thresholdAmount // ignore: cast_nullable_to_non_nullable
as num?,usageLimitPerAccount: freezed == usageLimitPerAccount ? _self.usageLimitPerAccount : usageLimitPerAccount // ignore: cast_nullable_to_non_nullable
as num?,startAt: freezed == startAt ? _self.startAt : startAt // ignore: cast_nullable_to_non_nullable
as String?,endAt: freezed == endAt ? _self.endAt : endAt // ignore: cast_nullable_to_non_nullable
as String?,sellerName: freezed == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String?,storeId: freezed == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String?,sellerAvatarUrl: freezed == sellerAvatarUrl ? _self.sellerAvatarUrl : sellerAvatarUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [Coupon].
extension CouponPatterns on Coupon {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Coupon value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Coupon() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Coupon value)  $default,){
final _that = this;
switch (_that) {
case _Coupon():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Coupon value)?  $default,){
final _that = this;
switch (_that) {
case _Coupon() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String name,  int? status,  String? createdAt,  num? discountAmount,  num? discountPercent,  num? maxDiscountAmount,  num? thresholdAmount,  num? usageLimitPerAccount,  String? startAt,  String? endAt,  String? sellerName,  String? storeId,  String? sellerAvatarUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Coupon() when $default != null:
return $default(_that.id,_that.name,_that.status,_that.createdAt,_that.discountAmount,_that.discountPercent,_that.maxDiscountAmount,_that.thresholdAmount,_that.usageLimitPerAccount,_that.startAt,_that.endAt,_that.sellerName,_that.storeId,_that.sellerAvatarUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String name,  int? status,  String? createdAt,  num? discountAmount,  num? discountPercent,  num? maxDiscountAmount,  num? thresholdAmount,  num? usageLimitPerAccount,  String? startAt,  String? endAt,  String? sellerName,  String? storeId,  String? sellerAvatarUrl)  $default,) {final _that = this;
switch (_that) {
case _Coupon():
return $default(_that.id,_that.name,_that.status,_that.createdAt,_that.discountAmount,_that.discountPercent,_that.maxDiscountAmount,_that.thresholdAmount,_that.usageLimitPerAccount,_that.startAt,_that.endAt,_that.sellerName,_that.storeId,_that.sellerAvatarUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String name,  int? status,  String? createdAt,  num? discountAmount,  num? discountPercent,  num? maxDiscountAmount,  num? thresholdAmount,  num? usageLimitPerAccount,  String? startAt,  String? endAt,  String? sellerName,  String? storeId,  String? sellerAvatarUrl)?  $default,) {final _that = this;
switch (_that) {
case _Coupon() when $default != null:
return $default(_that.id,_that.name,_that.status,_that.createdAt,_that.discountAmount,_that.discountPercent,_that.maxDiscountAmount,_that.thresholdAmount,_that.usageLimitPerAccount,_that.startAt,_that.endAt,_that.sellerName,_that.storeId,_that.sellerAvatarUrl);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Coupon implements Coupon {
  const _Coupon({required this.id, required this.name, this.status, this.createdAt, this.discountAmount, this.discountPercent, this.maxDiscountAmount, this.thresholdAmount, this.usageLimitPerAccount, this.startAt, this.endAt, this.sellerName, this.storeId, this.sellerAvatarUrl});
  factory _Coupon.fromJson(Map<String, dynamic> json) => _$CouponFromJson(json);

/// `instanceId`（已使用券）或 `templateId`（可用券）。
@override final  String id;
@override final  String name;
@override final  int? status;
@override final  String? createdAt;
@override final  num? discountAmount;
@override final  num? discountPercent;
@override final  num? maxDiscountAmount;
@override final  num? thresholdAmount;
@override final  num? usageLimitPerAccount;
@override final  String? startAt;
@override final  String? endAt;
@override final  String? sellerName;
@override final  String? storeId;
/// 商家 logo 來源：`logoKey`（裸 objectKey，SSOT）優先、`logoUrl` 退路。
/// 顯示端一律帶 scene 過 `buildImageUrl`（dual-mode）。
@override final  String? sellerAvatarUrl;

/// Create a copy of Coupon
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CouponCopyWith<_Coupon> get copyWith => __$CouponCopyWithImpl<_Coupon>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CouponToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Coupon&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.status, status) || other.status == status)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.discountPercent, discountPercent) || other.discountPercent == discountPercent)&&(identical(other.maxDiscountAmount, maxDiscountAmount) || other.maxDiscountAmount == maxDiscountAmount)&&(identical(other.thresholdAmount, thresholdAmount) || other.thresholdAmount == thresholdAmount)&&(identical(other.usageLimitPerAccount, usageLimitPerAccount) || other.usageLimitPerAccount == usageLimitPerAccount)&&(identical(other.startAt, startAt) || other.startAt == startAt)&&(identical(other.endAt, endAt) || other.endAt == endAt)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.sellerAvatarUrl, sellerAvatarUrl) || other.sellerAvatarUrl == sellerAvatarUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name,status,createdAt,discountAmount,discountPercent,maxDiscountAmount,thresholdAmount,usageLimitPerAccount,startAt,endAt,sellerName,storeId,sellerAvatarUrl);

@override
String toString() {
  return 'Coupon(id: $id, name: $name, status: $status, createdAt: $createdAt, discountAmount: $discountAmount, discountPercent: $discountPercent, maxDiscountAmount: $maxDiscountAmount, thresholdAmount: $thresholdAmount, usageLimitPerAccount: $usageLimitPerAccount, startAt: $startAt, endAt: $endAt, sellerName: $sellerName, storeId: $storeId, sellerAvatarUrl: $sellerAvatarUrl)';
}


}

/// @nodoc
abstract mixin class _$CouponCopyWith<$Res> implements $CouponCopyWith<$Res> {
  factory _$CouponCopyWith(_Coupon value, $Res Function(_Coupon) _then) = __$CouponCopyWithImpl;
@override @useResult
$Res call({
 String id, String name, int? status, String? createdAt, num? discountAmount, num? discountPercent, num? maxDiscountAmount, num? thresholdAmount, num? usageLimitPerAccount, String? startAt, String? endAt, String? sellerName, String? storeId, String? sellerAvatarUrl
});




}
/// @nodoc
class __$CouponCopyWithImpl<$Res>
    implements _$CouponCopyWith<$Res> {
  __$CouponCopyWithImpl(this._self, this._then);

  final _Coupon _self;
  final $Res Function(_Coupon) _then;

/// Create a copy of Coupon
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? name = null,Object? status = freezed,Object? createdAt = freezed,Object? discountAmount = freezed,Object? discountPercent = freezed,Object? maxDiscountAmount = freezed,Object? thresholdAmount = freezed,Object? usageLimitPerAccount = freezed,Object? startAt = freezed,Object? endAt = freezed,Object? sellerName = freezed,Object? storeId = freezed,Object? sellerAvatarUrl = freezed,}) {
  return _then(_Coupon(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,status: freezed == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as int?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String?,discountAmount: freezed == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as num?,discountPercent: freezed == discountPercent ? _self.discountPercent : discountPercent // ignore: cast_nullable_to_non_nullable
as num?,maxDiscountAmount: freezed == maxDiscountAmount ? _self.maxDiscountAmount : maxDiscountAmount // ignore: cast_nullable_to_non_nullable
as num?,thresholdAmount: freezed == thresholdAmount ? _self.thresholdAmount : thresholdAmount // ignore: cast_nullable_to_non_nullable
as num?,usageLimitPerAccount: freezed == usageLimitPerAccount ? _self.usageLimitPerAccount : usageLimitPerAccount // ignore: cast_nullable_to_non_nullable
as num?,startAt: freezed == startAt ? _self.startAt : startAt // ignore: cast_nullable_to_non_nullable
as String?,endAt: freezed == endAt ? _self.endAt : endAt // ignore: cast_nullable_to_non_nullable
as String?,sellerName: freezed == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String?,storeId: freezed == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String?,sellerAvatarUrl: freezed == sellerAvatarUrl ? _self.sellerAvatarUrl : sellerAvatarUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
