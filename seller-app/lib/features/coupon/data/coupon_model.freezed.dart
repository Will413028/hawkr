// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'coupon_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$Coupon {

/// templateId from API (used as primary key)
 String get templateId;/// instanceId for customer-specific coupon instances
 String get instanceId; String get title;/// CASH | PERCENT
 String get type; String get status; String? get discountAmount; String? get discountPercent;/// 門檻金額
 String? get thresholdAmount; String? get maxDiscountAmount; String? get startAt; String? get endAt; String? get issuedCount; String? get totalIssueLimit; String? get usageLimitPerAccount; String? get issuerAccountId; String? get issuerStoreId; String? get issuerType; String? get applicableStoreId;/// 賣家名稱 (from customer coupons API)
 String? get sellerName; String? get createdAt;
/// Create a copy of Coupon
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CouponCopyWith<Coupon> get copyWith => _$CouponCopyWithImpl<Coupon>(this as Coupon, _$identity);

  /// Serializes this Coupon to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Coupon&&(identical(other.templateId, templateId) || other.templateId == templateId)&&(identical(other.instanceId, instanceId) || other.instanceId == instanceId)&&(identical(other.title, title) || other.title == title)&&(identical(other.type, type) || other.type == type)&&(identical(other.status, status) || other.status == status)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.discountPercent, discountPercent) || other.discountPercent == discountPercent)&&(identical(other.thresholdAmount, thresholdAmount) || other.thresholdAmount == thresholdAmount)&&(identical(other.maxDiscountAmount, maxDiscountAmount) || other.maxDiscountAmount == maxDiscountAmount)&&(identical(other.startAt, startAt) || other.startAt == startAt)&&(identical(other.endAt, endAt) || other.endAt == endAt)&&(identical(other.issuedCount, issuedCount) || other.issuedCount == issuedCount)&&(identical(other.totalIssueLimit, totalIssueLimit) || other.totalIssueLimit == totalIssueLimit)&&(identical(other.usageLimitPerAccount, usageLimitPerAccount) || other.usageLimitPerAccount == usageLimitPerAccount)&&(identical(other.issuerAccountId, issuerAccountId) || other.issuerAccountId == issuerAccountId)&&(identical(other.issuerStoreId, issuerStoreId) || other.issuerStoreId == issuerStoreId)&&(identical(other.issuerType, issuerType) || other.issuerType == issuerType)&&(identical(other.applicableStoreId, applicableStoreId) || other.applicableStoreId == applicableStoreId)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,templateId,instanceId,title,type,status,discountAmount,discountPercent,thresholdAmount,maxDiscountAmount,startAt,endAt,issuedCount,totalIssueLimit,usageLimitPerAccount,issuerAccountId,issuerStoreId,issuerType,applicableStoreId,sellerName,createdAt]);

@override
String toString() {
  return 'Coupon(templateId: $templateId, instanceId: $instanceId, title: $title, type: $type, status: $status, discountAmount: $discountAmount, discountPercent: $discountPercent, thresholdAmount: $thresholdAmount, maxDiscountAmount: $maxDiscountAmount, startAt: $startAt, endAt: $endAt, issuedCount: $issuedCount, totalIssueLimit: $totalIssueLimit, usageLimitPerAccount: $usageLimitPerAccount, issuerAccountId: $issuerAccountId, issuerStoreId: $issuerStoreId, issuerType: $issuerType, applicableStoreId: $applicableStoreId, sellerName: $sellerName, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $CouponCopyWith<$Res>  {
  factory $CouponCopyWith(Coupon value, $Res Function(Coupon) _then) = _$CouponCopyWithImpl;
@useResult
$Res call({
 String templateId, String instanceId, String title, String type, String status, String? discountAmount, String? discountPercent, String? thresholdAmount, String? maxDiscountAmount, String? startAt, String? endAt, String? issuedCount, String? totalIssueLimit, String? usageLimitPerAccount, String? issuerAccountId, String? issuerStoreId, String? issuerType, String? applicableStoreId, String? sellerName, String? createdAt
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
@pragma('vm:prefer-inline') @override $Res call({Object? templateId = null,Object? instanceId = null,Object? title = null,Object? type = null,Object? status = null,Object? discountAmount = freezed,Object? discountPercent = freezed,Object? thresholdAmount = freezed,Object? maxDiscountAmount = freezed,Object? startAt = freezed,Object? endAt = freezed,Object? issuedCount = freezed,Object? totalIssueLimit = freezed,Object? usageLimitPerAccount = freezed,Object? issuerAccountId = freezed,Object? issuerStoreId = freezed,Object? issuerType = freezed,Object? applicableStoreId = freezed,Object? sellerName = freezed,Object? createdAt = freezed,}) {
  return _then(_self.copyWith(
templateId: null == templateId ? _self.templateId : templateId // ignore: cast_nullable_to_non_nullable
as String,instanceId: null == instanceId ? _self.instanceId : instanceId // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,discountAmount: freezed == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as String?,discountPercent: freezed == discountPercent ? _self.discountPercent : discountPercent // ignore: cast_nullable_to_non_nullable
as String?,thresholdAmount: freezed == thresholdAmount ? _self.thresholdAmount : thresholdAmount // ignore: cast_nullable_to_non_nullable
as String?,maxDiscountAmount: freezed == maxDiscountAmount ? _self.maxDiscountAmount : maxDiscountAmount // ignore: cast_nullable_to_non_nullable
as String?,startAt: freezed == startAt ? _self.startAt : startAt // ignore: cast_nullable_to_non_nullable
as String?,endAt: freezed == endAt ? _self.endAt : endAt // ignore: cast_nullable_to_non_nullable
as String?,issuedCount: freezed == issuedCount ? _self.issuedCount : issuedCount // ignore: cast_nullable_to_non_nullable
as String?,totalIssueLimit: freezed == totalIssueLimit ? _self.totalIssueLimit : totalIssueLimit // ignore: cast_nullable_to_non_nullable
as String?,usageLimitPerAccount: freezed == usageLimitPerAccount ? _self.usageLimitPerAccount : usageLimitPerAccount // ignore: cast_nullable_to_non_nullable
as String?,issuerAccountId: freezed == issuerAccountId ? _self.issuerAccountId : issuerAccountId // ignore: cast_nullable_to_non_nullable
as String?,issuerStoreId: freezed == issuerStoreId ? _self.issuerStoreId : issuerStoreId // ignore: cast_nullable_to_non_nullable
as String?,issuerType: freezed == issuerType ? _self.issuerType : issuerType // ignore: cast_nullable_to_non_nullable
as String?,applicableStoreId: freezed == applicableStoreId ? _self.applicableStoreId : applicableStoreId // ignore: cast_nullable_to_non_nullable
as String?,sellerName: freezed == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String templateId,  String instanceId,  String title,  String type,  String status,  String? discountAmount,  String? discountPercent,  String? thresholdAmount,  String? maxDiscountAmount,  String? startAt,  String? endAt,  String? issuedCount,  String? totalIssueLimit,  String? usageLimitPerAccount,  String? issuerAccountId,  String? issuerStoreId,  String? issuerType,  String? applicableStoreId,  String? sellerName,  String? createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Coupon() when $default != null:
return $default(_that.templateId,_that.instanceId,_that.title,_that.type,_that.status,_that.discountAmount,_that.discountPercent,_that.thresholdAmount,_that.maxDiscountAmount,_that.startAt,_that.endAt,_that.issuedCount,_that.totalIssueLimit,_that.usageLimitPerAccount,_that.issuerAccountId,_that.issuerStoreId,_that.issuerType,_that.applicableStoreId,_that.sellerName,_that.createdAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String templateId,  String instanceId,  String title,  String type,  String status,  String? discountAmount,  String? discountPercent,  String? thresholdAmount,  String? maxDiscountAmount,  String? startAt,  String? endAt,  String? issuedCount,  String? totalIssueLimit,  String? usageLimitPerAccount,  String? issuerAccountId,  String? issuerStoreId,  String? issuerType,  String? applicableStoreId,  String? sellerName,  String? createdAt)  $default,) {final _that = this;
switch (_that) {
case _Coupon():
return $default(_that.templateId,_that.instanceId,_that.title,_that.type,_that.status,_that.discountAmount,_that.discountPercent,_that.thresholdAmount,_that.maxDiscountAmount,_that.startAt,_that.endAt,_that.issuedCount,_that.totalIssueLimit,_that.usageLimitPerAccount,_that.issuerAccountId,_that.issuerStoreId,_that.issuerType,_that.applicableStoreId,_that.sellerName,_that.createdAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String templateId,  String instanceId,  String title,  String type,  String status,  String? discountAmount,  String? discountPercent,  String? thresholdAmount,  String? maxDiscountAmount,  String? startAt,  String? endAt,  String? issuedCount,  String? totalIssueLimit,  String? usageLimitPerAccount,  String? issuerAccountId,  String? issuerStoreId,  String? issuerType,  String? applicableStoreId,  String? sellerName,  String? createdAt)?  $default,) {final _that = this;
switch (_that) {
case _Coupon() when $default != null:
return $default(_that.templateId,_that.instanceId,_that.title,_that.type,_that.status,_that.discountAmount,_that.discountPercent,_that.thresholdAmount,_that.maxDiscountAmount,_that.startAt,_that.endAt,_that.issuedCount,_that.totalIssueLimit,_that.usageLimitPerAccount,_that.issuerAccountId,_that.issuerStoreId,_that.issuerType,_that.applicableStoreId,_that.sellerName,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Coupon implements Coupon {
  const _Coupon({required this.templateId, this.instanceId = '', this.title = '', this.type = 'CASH', this.status = '', this.discountAmount, this.discountPercent, this.thresholdAmount, this.maxDiscountAmount, this.startAt, this.endAt, this.issuedCount, this.totalIssueLimit, this.usageLimitPerAccount, this.issuerAccountId, this.issuerStoreId, this.issuerType, this.applicableStoreId, this.sellerName, this.createdAt});
  factory _Coupon.fromJson(Map<String, dynamic> json) => _$CouponFromJson(json);

/// templateId from API (used as primary key)
@override final  String templateId;
/// instanceId for customer-specific coupon instances
@override@JsonKey() final  String instanceId;
@override@JsonKey() final  String title;
/// CASH | PERCENT
@override@JsonKey() final  String type;
@override@JsonKey() final  String status;
@override final  String? discountAmount;
@override final  String? discountPercent;
/// 門檻金額
@override final  String? thresholdAmount;
@override final  String? maxDiscountAmount;
@override final  String? startAt;
@override final  String? endAt;
@override final  String? issuedCount;
@override final  String? totalIssueLimit;
@override final  String? usageLimitPerAccount;
@override final  String? issuerAccountId;
@override final  String? issuerStoreId;
@override final  String? issuerType;
@override final  String? applicableStoreId;
/// 賣家名稱 (from customer coupons API)
@override final  String? sellerName;
@override final  String? createdAt;

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
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Coupon&&(identical(other.templateId, templateId) || other.templateId == templateId)&&(identical(other.instanceId, instanceId) || other.instanceId == instanceId)&&(identical(other.title, title) || other.title == title)&&(identical(other.type, type) || other.type == type)&&(identical(other.status, status) || other.status == status)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.discountPercent, discountPercent) || other.discountPercent == discountPercent)&&(identical(other.thresholdAmount, thresholdAmount) || other.thresholdAmount == thresholdAmount)&&(identical(other.maxDiscountAmount, maxDiscountAmount) || other.maxDiscountAmount == maxDiscountAmount)&&(identical(other.startAt, startAt) || other.startAt == startAt)&&(identical(other.endAt, endAt) || other.endAt == endAt)&&(identical(other.issuedCount, issuedCount) || other.issuedCount == issuedCount)&&(identical(other.totalIssueLimit, totalIssueLimit) || other.totalIssueLimit == totalIssueLimit)&&(identical(other.usageLimitPerAccount, usageLimitPerAccount) || other.usageLimitPerAccount == usageLimitPerAccount)&&(identical(other.issuerAccountId, issuerAccountId) || other.issuerAccountId == issuerAccountId)&&(identical(other.issuerStoreId, issuerStoreId) || other.issuerStoreId == issuerStoreId)&&(identical(other.issuerType, issuerType) || other.issuerType == issuerType)&&(identical(other.applicableStoreId, applicableStoreId) || other.applicableStoreId == applicableStoreId)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,templateId,instanceId,title,type,status,discountAmount,discountPercent,thresholdAmount,maxDiscountAmount,startAt,endAt,issuedCount,totalIssueLimit,usageLimitPerAccount,issuerAccountId,issuerStoreId,issuerType,applicableStoreId,sellerName,createdAt]);

@override
String toString() {
  return 'Coupon(templateId: $templateId, instanceId: $instanceId, title: $title, type: $type, status: $status, discountAmount: $discountAmount, discountPercent: $discountPercent, thresholdAmount: $thresholdAmount, maxDiscountAmount: $maxDiscountAmount, startAt: $startAt, endAt: $endAt, issuedCount: $issuedCount, totalIssueLimit: $totalIssueLimit, usageLimitPerAccount: $usageLimitPerAccount, issuerAccountId: $issuerAccountId, issuerStoreId: $issuerStoreId, issuerType: $issuerType, applicableStoreId: $applicableStoreId, sellerName: $sellerName, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$CouponCopyWith<$Res> implements $CouponCopyWith<$Res> {
  factory _$CouponCopyWith(_Coupon value, $Res Function(_Coupon) _then) = __$CouponCopyWithImpl;
@override @useResult
$Res call({
 String templateId, String instanceId, String title, String type, String status, String? discountAmount, String? discountPercent, String? thresholdAmount, String? maxDiscountAmount, String? startAt, String? endAt, String? issuedCount, String? totalIssueLimit, String? usageLimitPerAccount, String? issuerAccountId, String? issuerStoreId, String? issuerType, String? applicableStoreId, String? sellerName, String? createdAt
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
@override @pragma('vm:prefer-inline') $Res call({Object? templateId = null,Object? instanceId = null,Object? title = null,Object? type = null,Object? status = null,Object? discountAmount = freezed,Object? discountPercent = freezed,Object? thresholdAmount = freezed,Object? maxDiscountAmount = freezed,Object? startAt = freezed,Object? endAt = freezed,Object? issuedCount = freezed,Object? totalIssueLimit = freezed,Object? usageLimitPerAccount = freezed,Object? issuerAccountId = freezed,Object? issuerStoreId = freezed,Object? issuerType = freezed,Object? applicableStoreId = freezed,Object? sellerName = freezed,Object? createdAt = freezed,}) {
  return _then(_Coupon(
templateId: null == templateId ? _self.templateId : templateId // ignore: cast_nullable_to_non_nullable
as String,instanceId: null == instanceId ? _self.instanceId : instanceId // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,discountAmount: freezed == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as String?,discountPercent: freezed == discountPercent ? _self.discountPercent : discountPercent // ignore: cast_nullable_to_non_nullable
as String?,thresholdAmount: freezed == thresholdAmount ? _self.thresholdAmount : thresholdAmount // ignore: cast_nullable_to_non_nullable
as String?,maxDiscountAmount: freezed == maxDiscountAmount ? _self.maxDiscountAmount : maxDiscountAmount // ignore: cast_nullable_to_non_nullable
as String?,startAt: freezed == startAt ? _self.startAt : startAt // ignore: cast_nullable_to_non_nullable
as String?,endAt: freezed == endAt ? _self.endAt : endAt // ignore: cast_nullable_to_non_nullable
as String?,issuedCount: freezed == issuedCount ? _self.issuedCount : issuedCount // ignore: cast_nullable_to_non_nullable
as String?,totalIssueLimit: freezed == totalIssueLimit ? _self.totalIssueLimit : totalIssueLimit // ignore: cast_nullable_to_non_nullable
as String?,usageLimitPerAccount: freezed == usageLimitPerAccount ? _self.usageLimitPerAccount : usageLimitPerAccount // ignore: cast_nullable_to_non_nullable
as String?,issuerAccountId: freezed == issuerAccountId ? _self.issuerAccountId : issuerAccountId // ignore: cast_nullable_to_non_nullable
as String?,issuerStoreId: freezed == issuerStoreId ? _self.issuerStoreId : issuerStoreId // ignore: cast_nullable_to_non_nullable
as String?,issuerType: freezed == issuerType ? _self.issuerType : issuerType // ignore: cast_nullable_to_non_nullable
as String?,applicableStoreId: freezed == applicableStoreId ? _self.applicableStoreId : applicableStoreId // ignore: cast_nullable_to_non_nullable
as String?,sellerName: freezed == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}


/// @nodoc
mixin _$CustomerLookupResult {

 String get uid; String get buyerName; String get phone; String? get avatarUrl;
/// Create a copy of CustomerLookupResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CustomerLookupResultCopyWith<CustomerLookupResult> get copyWith => _$CustomerLookupResultCopyWithImpl<CustomerLookupResult>(this as CustomerLookupResult, _$identity);

  /// Serializes this CustomerLookupResult to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CustomerLookupResult&&(identical(other.uid, uid) || other.uid == uid)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,uid,buyerName,phone,avatarUrl);

@override
String toString() {
  return 'CustomerLookupResult(uid: $uid, buyerName: $buyerName, phone: $phone, avatarUrl: $avatarUrl)';
}


}

/// @nodoc
abstract mixin class $CustomerLookupResultCopyWith<$Res>  {
  factory $CustomerLookupResultCopyWith(CustomerLookupResult value, $Res Function(CustomerLookupResult) _then) = _$CustomerLookupResultCopyWithImpl;
@useResult
$Res call({
 String uid, String buyerName, String phone, String? avatarUrl
});




}
/// @nodoc
class _$CustomerLookupResultCopyWithImpl<$Res>
    implements $CustomerLookupResultCopyWith<$Res> {
  _$CustomerLookupResultCopyWithImpl(this._self, this._then);

  final CustomerLookupResult _self;
  final $Res Function(CustomerLookupResult) _then;

/// Create a copy of CustomerLookupResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? uid = null,Object? buyerName = null,Object? phone = null,Object? avatarUrl = freezed,}) {
  return _then(_self.copyWith(
uid: null == uid ? _self.uid : uid // ignore: cast_nullable_to_non_nullable
as String,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,avatarUrl: freezed == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [CustomerLookupResult].
extension CustomerLookupResultPatterns on CustomerLookupResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CustomerLookupResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CustomerLookupResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CustomerLookupResult value)  $default,){
final _that = this;
switch (_that) {
case _CustomerLookupResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CustomerLookupResult value)?  $default,){
final _that = this;
switch (_that) {
case _CustomerLookupResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String uid,  String buyerName,  String phone,  String? avatarUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CustomerLookupResult() when $default != null:
return $default(_that.uid,_that.buyerName,_that.phone,_that.avatarUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String uid,  String buyerName,  String phone,  String? avatarUrl)  $default,) {final _that = this;
switch (_that) {
case _CustomerLookupResult():
return $default(_that.uid,_that.buyerName,_that.phone,_that.avatarUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String uid,  String buyerName,  String phone,  String? avatarUrl)?  $default,) {final _that = this;
switch (_that) {
case _CustomerLookupResult() when $default != null:
return $default(_that.uid,_that.buyerName,_that.phone,_that.avatarUrl);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _CustomerLookupResult implements CustomerLookupResult {
  const _CustomerLookupResult({required this.uid, this.buyerName = '', this.phone = '', this.avatarUrl});
  factory _CustomerLookupResult.fromJson(Map<String, dynamic> json) => _$CustomerLookupResultFromJson(json);

@override final  String uid;
@override@JsonKey() final  String buyerName;
@override@JsonKey() final  String phone;
@override final  String? avatarUrl;

/// Create a copy of CustomerLookupResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CustomerLookupResultCopyWith<_CustomerLookupResult> get copyWith => __$CustomerLookupResultCopyWithImpl<_CustomerLookupResult>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CustomerLookupResultToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CustomerLookupResult&&(identical(other.uid, uid) || other.uid == uid)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,uid,buyerName,phone,avatarUrl);

@override
String toString() {
  return 'CustomerLookupResult(uid: $uid, buyerName: $buyerName, phone: $phone, avatarUrl: $avatarUrl)';
}


}

/// @nodoc
abstract mixin class _$CustomerLookupResultCopyWith<$Res> implements $CustomerLookupResultCopyWith<$Res> {
  factory _$CustomerLookupResultCopyWith(_CustomerLookupResult value, $Res Function(_CustomerLookupResult) _then) = __$CustomerLookupResultCopyWithImpl;
@override @useResult
$Res call({
 String uid, String buyerName, String phone, String? avatarUrl
});




}
/// @nodoc
class __$CustomerLookupResultCopyWithImpl<$Res>
    implements _$CustomerLookupResultCopyWith<$Res> {
  __$CustomerLookupResultCopyWithImpl(this._self, this._then);

  final _CustomerLookupResult _self;
  final $Res Function(_CustomerLookupResult) _then;

/// Create a copy of CustomerLookupResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? uid = null,Object? buyerName = null,Object? phone = null,Object? avatarUrl = freezed,}) {
  return _then(_CustomerLookupResult(
uid: null == uid ? _self.uid : uid // ignore: cast_nullable_to_non_nullable
as String,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,avatarUrl: freezed == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

/// @nodoc
mixin _$CouponListState {

 List<Coupon> get coupons; bool get isLoading; String? get error;
/// Create a copy of CouponListState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CouponListStateCopyWith<CouponListState> get copyWith => _$CouponListStateCopyWithImpl<CouponListState>(this as CouponListState, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CouponListState&&const DeepCollectionEquality().equals(other.coupons, coupons)&&(identical(other.isLoading, isLoading) || other.isLoading == isLoading)&&(identical(other.error, error) || other.error == error));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(coupons),isLoading,error);

@override
String toString() {
  return 'CouponListState(coupons: $coupons, isLoading: $isLoading, error: $error)';
}


}

/// @nodoc
abstract mixin class $CouponListStateCopyWith<$Res>  {
  factory $CouponListStateCopyWith(CouponListState value, $Res Function(CouponListState) _then) = _$CouponListStateCopyWithImpl;
@useResult
$Res call({
 List<Coupon> coupons, bool isLoading, String? error
});




}
/// @nodoc
class _$CouponListStateCopyWithImpl<$Res>
    implements $CouponListStateCopyWith<$Res> {
  _$CouponListStateCopyWithImpl(this._self, this._then);

  final CouponListState _self;
  final $Res Function(CouponListState) _then;

/// Create a copy of CouponListState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? coupons = null,Object? isLoading = null,Object? error = freezed,}) {
  return _then(_self.copyWith(
coupons: null == coupons ? _self.coupons : coupons // ignore: cast_nullable_to_non_nullable
as List<Coupon>,isLoading: null == isLoading ? _self.isLoading : isLoading // ignore: cast_nullable_to_non_nullable
as bool,error: freezed == error ? _self.error : error // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [CouponListState].
extension CouponListStatePatterns on CouponListState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CouponListState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CouponListState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CouponListState value)  $default,){
final _that = this;
switch (_that) {
case _CouponListState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CouponListState value)?  $default,){
final _that = this;
switch (_that) {
case _CouponListState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<Coupon> coupons,  bool isLoading,  String? error)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CouponListState() when $default != null:
return $default(_that.coupons,_that.isLoading,_that.error);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<Coupon> coupons,  bool isLoading,  String? error)  $default,) {final _that = this;
switch (_that) {
case _CouponListState():
return $default(_that.coupons,_that.isLoading,_that.error);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<Coupon> coupons,  bool isLoading,  String? error)?  $default,) {final _that = this;
switch (_that) {
case _CouponListState() when $default != null:
return $default(_that.coupons,_that.isLoading,_that.error);case _:
  return null;

}
}

}

/// @nodoc


class _CouponListState implements CouponListState {
  const _CouponListState({final  List<Coupon> coupons = const [], this.isLoading = false, this.error}): _coupons = coupons;
  

 final  List<Coupon> _coupons;
@override@JsonKey() List<Coupon> get coupons {
  if (_coupons is EqualUnmodifiableListView) return _coupons;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_coupons);
}

@override@JsonKey() final  bool isLoading;
@override final  String? error;

/// Create a copy of CouponListState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CouponListStateCopyWith<_CouponListState> get copyWith => __$CouponListStateCopyWithImpl<_CouponListState>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CouponListState&&const DeepCollectionEquality().equals(other._coupons, _coupons)&&(identical(other.isLoading, isLoading) || other.isLoading == isLoading)&&(identical(other.error, error) || other.error == error));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_coupons),isLoading,error);

@override
String toString() {
  return 'CouponListState(coupons: $coupons, isLoading: $isLoading, error: $error)';
}


}

/// @nodoc
abstract mixin class _$CouponListStateCopyWith<$Res> implements $CouponListStateCopyWith<$Res> {
  factory _$CouponListStateCopyWith(_CouponListState value, $Res Function(_CouponListState) _then) = __$CouponListStateCopyWithImpl;
@override @useResult
$Res call({
 List<Coupon> coupons, bool isLoading, String? error
});




}
/// @nodoc
class __$CouponListStateCopyWithImpl<$Res>
    implements _$CouponListStateCopyWith<$Res> {
  __$CouponListStateCopyWithImpl(this._self, this._then);

  final _CouponListState _self;
  final $Res Function(_CouponListState) _then;

/// Create a copy of CouponListState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? coupons = null,Object? isLoading = null,Object? error = freezed,}) {
  return _then(_CouponListState(
coupons: null == coupons ? _self._coupons : coupons // ignore: cast_nullable_to_non_nullable
as List<Coupon>,isLoading: null == isLoading ? _self.isLoading : isLoading // ignore: cast_nullable_to_non_nullable
as bool,error: freezed == error ? _self.error : error // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
