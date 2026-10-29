// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'member_card_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$MemberCard {

 String get sellerId; String get sellerName;/// 店面網址的路徑段；店家不是營業中時是空字串。
 String get storeCode;/// 店家暫停營業或已關閉時的標籤（共用文案目錄）；營業中是空字串。
 String get statusLabel;/// 賣家 logo 的 objectKey（49 §6.1 成對契約中的 SSOT 那一半）。
/// 顯示 URL 一律由 OssImage / buildImageUrl 於渲染期組出，不存完整 URL。
 String get sellerLogoKey;/// 賣家封面的 objectKey（同上）。
 String get sellerCoverKey; String get totalSpending;
/// Create a copy of MemberCard
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$MemberCardCopyWith<MemberCard> get copyWith => _$MemberCardCopyWithImpl<MemberCard>(this as MemberCard, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is MemberCard&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.statusLabel, statusLabel) || other.statusLabel == statusLabel)&&(identical(other.sellerLogoKey, sellerLogoKey) || other.sellerLogoKey == sellerLogoKey)&&(identical(other.sellerCoverKey, sellerCoverKey) || other.sellerCoverKey == sellerCoverKey)&&(identical(other.totalSpending, totalSpending) || other.totalSpending == totalSpending));
}


@override
int get hashCode => Object.hash(runtimeType,sellerId,sellerName,storeCode,statusLabel,sellerLogoKey,sellerCoverKey,totalSpending);

@override
String toString() {
  return 'MemberCard(sellerId: $sellerId, sellerName: $sellerName, storeCode: $storeCode, statusLabel: $statusLabel, sellerLogoKey: $sellerLogoKey, sellerCoverKey: $sellerCoverKey, totalSpending: $totalSpending)';
}


}

/// @nodoc
abstract mixin class $MemberCardCopyWith<$Res>  {
  factory $MemberCardCopyWith(MemberCard value, $Res Function(MemberCard) _then) = _$MemberCardCopyWithImpl;
@useResult
$Res call({
 String sellerId, String sellerName, String storeCode, String statusLabel, String sellerLogoKey, String sellerCoverKey, String totalSpending
});




}
/// @nodoc
class _$MemberCardCopyWithImpl<$Res>
    implements $MemberCardCopyWith<$Res> {
  _$MemberCardCopyWithImpl(this._self, this._then);

  final MemberCard _self;
  final $Res Function(MemberCard) _then;

/// Create a copy of MemberCard
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? sellerId = null,Object? sellerName = null,Object? storeCode = null,Object? statusLabel = null,Object? sellerLogoKey = null,Object? sellerCoverKey = null,Object? totalSpending = null,}) {
  return _then(_self.copyWith(
sellerId: null == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as String,sellerName: null == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
as String,statusLabel: null == statusLabel ? _self.statusLabel : statusLabel // ignore: cast_nullable_to_non_nullable
as String,sellerLogoKey: null == sellerLogoKey ? _self.sellerLogoKey : sellerLogoKey // ignore: cast_nullable_to_non_nullable
as String,sellerCoverKey: null == sellerCoverKey ? _self.sellerCoverKey : sellerCoverKey // ignore: cast_nullable_to_non_nullable
as String,totalSpending: null == totalSpending ? _self.totalSpending : totalSpending // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [MemberCard].
extension MemberCardPatterns on MemberCard {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _MemberCard value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _MemberCard() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _MemberCard value)  $default,){
final _that = this;
switch (_that) {
case _MemberCard():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _MemberCard value)?  $default,){
final _that = this;
switch (_that) {
case _MemberCard() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String sellerId,  String sellerName,  String storeCode,  String statusLabel,  String sellerLogoKey,  String sellerCoverKey,  String totalSpending)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _MemberCard() when $default != null:
return $default(_that.sellerId,_that.sellerName,_that.storeCode,_that.statusLabel,_that.sellerLogoKey,_that.sellerCoverKey,_that.totalSpending);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String sellerId,  String sellerName,  String storeCode,  String statusLabel,  String sellerLogoKey,  String sellerCoverKey,  String totalSpending)  $default,) {final _that = this;
switch (_that) {
case _MemberCard():
return $default(_that.sellerId,_that.sellerName,_that.storeCode,_that.statusLabel,_that.sellerLogoKey,_that.sellerCoverKey,_that.totalSpending);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String sellerId,  String sellerName,  String storeCode,  String statusLabel,  String sellerLogoKey,  String sellerCoverKey,  String totalSpending)?  $default,) {final _that = this;
switch (_that) {
case _MemberCard() when $default != null:
return $default(_that.sellerId,_that.sellerName,_that.storeCode,_that.statusLabel,_that.sellerLogoKey,_that.sellerCoverKey,_that.totalSpending);case _:
  return null;

}
}

}

/// @nodoc


class _MemberCard implements MemberCard {
  const _MemberCard({required this.sellerId, required this.sellerName, this.storeCode = '', this.statusLabel = '', this.sellerLogoKey = '', this.sellerCoverKey = '', required this.totalSpending});
  

@override final  String sellerId;
@override final  String sellerName;
/// 店面網址的路徑段；店家不是營業中時是空字串。
@override@JsonKey() final  String storeCode;
/// 店家暫停營業或已關閉時的標籤（共用文案目錄）；營業中是空字串。
@override@JsonKey() final  String statusLabel;
/// 賣家 logo 的 objectKey（49 §6.1 成對契約中的 SSOT 那一半）。
/// 顯示 URL 一律由 OssImage / buildImageUrl 於渲染期組出，不存完整 URL。
@override@JsonKey() final  String sellerLogoKey;
/// 賣家封面的 objectKey（同上）。
@override@JsonKey() final  String sellerCoverKey;
@override final  String totalSpending;

/// Create a copy of MemberCard
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$MemberCardCopyWith<_MemberCard> get copyWith => __$MemberCardCopyWithImpl<_MemberCard>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _MemberCard&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.storeCode, storeCode) || other.storeCode == storeCode)&&(identical(other.statusLabel, statusLabel) || other.statusLabel == statusLabel)&&(identical(other.sellerLogoKey, sellerLogoKey) || other.sellerLogoKey == sellerLogoKey)&&(identical(other.sellerCoverKey, sellerCoverKey) || other.sellerCoverKey == sellerCoverKey)&&(identical(other.totalSpending, totalSpending) || other.totalSpending == totalSpending));
}


@override
int get hashCode => Object.hash(runtimeType,sellerId,sellerName,storeCode,statusLabel,sellerLogoKey,sellerCoverKey,totalSpending);

@override
String toString() {
  return 'MemberCard(sellerId: $sellerId, sellerName: $sellerName, storeCode: $storeCode, statusLabel: $statusLabel, sellerLogoKey: $sellerLogoKey, sellerCoverKey: $sellerCoverKey, totalSpending: $totalSpending)';
}


}

/// @nodoc
abstract mixin class _$MemberCardCopyWith<$Res> implements $MemberCardCopyWith<$Res> {
  factory _$MemberCardCopyWith(_MemberCard value, $Res Function(_MemberCard) _then) = __$MemberCardCopyWithImpl;
@override @useResult
$Res call({
 String sellerId, String sellerName, String storeCode, String statusLabel, String sellerLogoKey, String sellerCoverKey, String totalSpending
});




}
/// @nodoc
class __$MemberCardCopyWithImpl<$Res>
    implements _$MemberCardCopyWith<$Res> {
  __$MemberCardCopyWithImpl(this._self, this._then);

  final _MemberCard _self;
  final $Res Function(_MemberCard) _then;

/// Create a copy of MemberCard
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? sellerId = null,Object? sellerName = null,Object? storeCode = null,Object? statusLabel = null,Object? sellerLogoKey = null,Object? sellerCoverKey = null,Object? totalSpending = null,}) {
  return _then(_MemberCard(
sellerId: null == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as String,sellerName: null == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String,storeCode: null == storeCode ? _self.storeCode : storeCode // ignore: cast_nullable_to_non_nullable
as String,statusLabel: null == statusLabel ? _self.statusLabel : statusLabel // ignore: cast_nullable_to_non_nullable
as String,sellerLogoKey: null == sellerLogoKey ? _self.sellerLogoKey : sellerLogoKey // ignore: cast_nullable_to_non_nullable
as String,sellerCoverKey: null == sellerCoverKey ? _self.sellerCoverKey : sellerCoverKey // ignore: cast_nullable_to_non_nullable
as String,totalSpending: null == totalSpending ? _self.totalSpending : totalSpending // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$SpendingRecord {

 String get orderDate; String get orderSn; String get amount; String get refund; bool get hasCoupon;
/// Create a copy of SpendingRecord
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SpendingRecordCopyWith<SpendingRecord> get copyWith => _$SpendingRecordCopyWithImpl<SpendingRecord>(this as SpendingRecord, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SpendingRecord&&(identical(other.orderDate, orderDate) || other.orderDate == orderDate)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.refund, refund) || other.refund == refund)&&(identical(other.hasCoupon, hasCoupon) || other.hasCoupon == hasCoupon));
}


@override
int get hashCode => Object.hash(runtimeType,orderDate,orderSn,amount,refund,hasCoupon);

@override
String toString() {
  return 'SpendingRecord(orderDate: $orderDate, orderSn: $orderSn, amount: $amount, refund: $refund, hasCoupon: $hasCoupon)';
}


}

/// @nodoc
abstract mixin class $SpendingRecordCopyWith<$Res>  {
  factory $SpendingRecordCopyWith(SpendingRecord value, $Res Function(SpendingRecord) _then) = _$SpendingRecordCopyWithImpl;
@useResult
$Res call({
 String orderDate, String orderSn, String amount, String refund, bool hasCoupon
});




}
/// @nodoc
class _$SpendingRecordCopyWithImpl<$Res>
    implements $SpendingRecordCopyWith<$Res> {
  _$SpendingRecordCopyWithImpl(this._self, this._then);

  final SpendingRecord _self;
  final $Res Function(SpendingRecord) _then;

/// Create a copy of SpendingRecord
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? orderDate = null,Object? orderSn = null,Object? amount = null,Object? refund = null,Object? hasCoupon = null,}) {
  return _then(_self.copyWith(
orderDate: null == orderDate ? _self.orderDate : orderDate // ignore: cast_nullable_to_non_nullable
as String,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,amount: null == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as String,refund: null == refund ? _self.refund : refund // ignore: cast_nullable_to_non_nullable
as String,hasCoupon: null == hasCoupon ? _self.hasCoupon : hasCoupon // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [SpendingRecord].
extension SpendingRecordPatterns on SpendingRecord {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SpendingRecord value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SpendingRecord() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SpendingRecord value)  $default,){
final _that = this;
switch (_that) {
case _SpendingRecord():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SpendingRecord value)?  $default,){
final _that = this;
switch (_that) {
case _SpendingRecord() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String orderDate,  String orderSn,  String amount,  String refund,  bool hasCoupon)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SpendingRecord() when $default != null:
return $default(_that.orderDate,_that.orderSn,_that.amount,_that.refund,_that.hasCoupon);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String orderDate,  String orderSn,  String amount,  String refund,  bool hasCoupon)  $default,) {final _that = this;
switch (_that) {
case _SpendingRecord():
return $default(_that.orderDate,_that.orderSn,_that.amount,_that.refund,_that.hasCoupon);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String orderDate,  String orderSn,  String amount,  String refund,  bool hasCoupon)?  $default,) {final _that = this;
switch (_that) {
case _SpendingRecord() when $default != null:
return $default(_that.orderDate,_that.orderSn,_that.amount,_that.refund,_that.hasCoupon);case _:
  return null;

}
}

}

/// @nodoc


class _SpendingRecord implements SpendingRecord {
  const _SpendingRecord({required this.orderDate, required this.orderSn, required this.amount, this.refund = '0', this.hasCoupon = false});
  

@override final  String orderDate;
@override final  String orderSn;
@override final  String amount;
@override@JsonKey() final  String refund;
@override@JsonKey() final  bool hasCoupon;

/// Create a copy of SpendingRecord
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SpendingRecordCopyWith<_SpendingRecord> get copyWith => __$SpendingRecordCopyWithImpl<_SpendingRecord>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SpendingRecord&&(identical(other.orderDate, orderDate) || other.orderDate == orderDate)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.refund, refund) || other.refund == refund)&&(identical(other.hasCoupon, hasCoupon) || other.hasCoupon == hasCoupon));
}


@override
int get hashCode => Object.hash(runtimeType,orderDate,orderSn,amount,refund,hasCoupon);

@override
String toString() {
  return 'SpendingRecord(orderDate: $orderDate, orderSn: $orderSn, amount: $amount, refund: $refund, hasCoupon: $hasCoupon)';
}


}

/// @nodoc
abstract mixin class _$SpendingRecordCopyWith<$Res> implements $SpendingRecordCopyWith<$Res> {
  factory _$SpendingRecordCopyWith(_SpendingRecord value, $Res Function(_SpendingRecord) _then) = __$SpendingRecordCopyWithImpl;
@override @useResult
$Res call({
 String orderDate, String orderSn, String amount, String refund, bool hasCoupon
});




}
/// @nodoc
class __$SpendingRecordCopyWithImpl<$Res>
    implements _$SpendingRecordCopyWith<$Res> {
  __$SpendingRecordCopyWithImpl(this._self, this._then);

  final _SpendingRecord _self;
  final $Res Function(_SpendingRecord) _then;

/// Create a copy of SpendingRecord
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? orderDate = null,Object? orderSn = null,Object? amount = null,Object? refund = null,Object? hasCoupon = null,}) {
  return _then(_SpendingRecord(
orderDate: null == orderDate ? _self.orderDate : orderDate // ignore: cast_nullable_to_non_nullable
as String,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,amount: null == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as String,refund: null == refund ? _self.refund : refund // ignore: cast_nullable_to_non_nullable
as String,hasCoupon: null == hasCoupon ? _self.hasCoupon : hasCoupon // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}

// dart format on
