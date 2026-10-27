// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'order_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$OrderItem {

 String get id; String get productId; String get productName; String get skuId; String get skuName; String get quantity;// PayUni #758: 新欄位（成交單價 / 原價快照）
 String get unitPrice; String get listPrice;/// Deprecated: 舊欄位 fallback；新訂單請用 unitPrice。
 String get originalPrice; String get actualPayAmount; String get refundStatus; String get shareDiscount; String get imageUrl;
/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderItemCopyWith<OrderItem> get copyWith => _$OrderItemCopyWithImpl<OrderItem>(this as OrderItem, _$identity);

  /// Serializes this OrderItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OrderItem&&(identical(other.id, id) || other.id == id)&&(identical(other.productId, productId) || other.productId == productId)&&(identical(other.productName, productName) || other.productName == productName)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.originalPrice, originalPrice) || other.originalPrice == originalPrice)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.refundStatus, refundStatus) || other.refundStatus == refundStatus)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,productId,productName,skuId,skuName,quantity,unitPrice,listPrice,originalPrice,actualPayAmount,refundStatus,shareDiscount,imageUrl);

@override
String toString() {
  return 'OrderItem(id: $id, productId: $productId, productName: $productName, skuId: $skuId, skuName: $skuName, quantity: $quantity, unitPrice: $unitPrice, listPrice: $listPrice, originalPrice: $originalPrice, actualPayAmount: $actualPayAmount, refundStatus: $refundStatus, shareDiscount: $shareDiscount, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class $OrderItemCopyWith<$Res>  {
  factory $OrderItemCopyWith(OrderItem value, $Res Function(OrderItem) _then) = _$OrderItemCopyWithImpl;
@useResult
$Res call({
 String id, String productId, String productName, String skuId, String skuName, String quantity, String unitPrice, String listPrice, String originalPrice, String actualPayAmount, String refundStatus, String shareDiscount, String imageUrl
});




}
/// @nodoc
class _$OrderItemCopyWithImpl<$Res>
    implements $OrderItemCopyWith<$Res> {
  _$OrderItemCopyWithImpl(this._self, this._then);

  final OrderItem _self;
  final $Res Function(OrderItem) _then;

/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? productId = null,Object? productName = null,Object? skuId = null,Object? skuName = null,Object? quantity = null,Object? unitPrice = null,Object? listPrice = null,Object? originalPrice = null,Object? actualPayAmount = null,Object? refundStatus = null,Object? shareDiscount = null,Object? imageUrl = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,productId: null == productId ? _self.productId : productId // ignore: cast_nullable_to_non_nullable
as String,productName: null == productName ? _self.productName : productName // ignore: cast_nullable_to_non_nullable
as String,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String,skuName: null == skuName ? _self.skuName : skuName // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unitPrice: null == unitPrice ? _self.unitPrice : unitPrice // ignore: cast_nullable_to_non_nullable
as String,listPrice: null == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as String,originalPrice: null == originalPrice ? _self.originalPrice : originalPrice // ignore: cast_nullable_to_non_nullable
as String,actualPayAmount: null == actualPayAmount ? _self.actualPayAmount : actualPayAmount // ignore: cast_nullable_to_non_nullable
as String,refundStatus: null == refundStatus ? _self.refundStatus : refundStatus // ignore: cast_nullable_to_non_nullable
as String,shareDiscount: null == shareDiscount ? _self.shareDiscount : shareDiscount // ignore: cast_nullable_to_non_nullable
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [OrderItem].
extension OrderItemPatterns on OrderItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _OrderItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _OrderItem value)  $default,){
final _that = this;
switch (_that) {
case _OrderItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _OrderItem value)?  $default,){
final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String productId,  String productName,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String refundStatus,  String shareDiscount,  String imageUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
return $default(_that.id,_that.productId,_that.productName,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.refundStatus,_that.shareDiscount,_that.imageUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String productId,  String productName,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String refundStatus,  String shareDiscount,  String imageUrl)  $default,) {final _that = this;
switch (_that) {
case _OrderItem():
return $default(_that.id,_that.productId,_that.productName,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.refundStatus,_that.shareDiscount,_that.imageUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String productId,  String productName,  String skuId,  String skuName,  String quantity,  String unitPrice,  String listPrice,  String originalPrice,  String actualPayAmount,  String refundStatus,  String shareDiscount,  String imageUrl)?  $default,) {final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
return $default(_that.id,_that.productId,_that.productName,_that.skuId,_that.skuName,_that.quantity,_that.unitPrice,_that.listPrice,_that.originalPrice,_that.actualPayAmount,_that.refundStatus,_that.shareDiscount,_that.imageUrl);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _OrderItem implements OrderItem {
  const _OrderItem({this.id = '', this.productId = '', this.productName = '', this.skuId = '', this.skuName = '', this.quantity = '', this.unitPrice = '', this.listPrice = '', this.originalPrice = '', this.actualPayAmount = '', this.refundStatus = '', this.shareDiscount = '', this.imageUrl = ''});
  factory _OrderItem.fromJson(Map<String, dynamic> json) => _$OrderItemFromJson(json);

@override@JsonKey() final  String id;
@override@JsonKey() final  String productId;
@override@JsonKey() final  String productName;
@override@JsonKey() final  String skuId;
@override@JsonKey() final  String skuName;
@override@JsonKey() final  String quantity;
// PayUni #758: 新欄位（成交單價 / 原價快照）
@override@JsonKey() final  String unitPrice;
@override@JsonKey() final  String listPrice;
/// Deprecated: 舊欄位 fallback；新訂單請用 unitPrice。
@override@JsonKey() final  String originalPrice;
@override@JsonKey() final  String actualPayAmount;
@override@JsonKey() final  String refundStatus;
@override@JsonKey() final  String shareDiscount;
@override@JsonKey() final  String imageUrl;

/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderItemCopyWith<_OrderItem> get copyWith => __$OrderItemCopyWithImpl<_OrderItem>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$OrderItemToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OrderItem&&(identical(other.id, id) || other.id == id)&&(identical(other.productId, productId) || other.productId == productId)&&(identical(other.productName, productName) || other.productName == productName)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.skuName, skuName) || other.skuName == skuName)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.originalPrice, originalPrice) || other.originalPrice == originalPrice)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.refundStatus, refundStatus) || other.refundStatus == refundStatus)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,productId,productName,skuId,skuName,quantity,unitPrice,listPrice,originalPrice,actualPayAmount,refundStatus,shareDiscount,imageUrl);

@override
String toString() {
  return 'OrderItem(id: $id, productId: $productId, productName: $productName, skuId: $skuId, skuName: $skuName, quantity: $quantity, unitPrice: $unitPrice, listPrice: $listPrice, originalPrice: $originalPrice, actualPayAmount: $actualPayAmount, refundStatus: $refundStatus, shareDiscount: $shareDiscount, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class _$OrderItemCopyWith<$Res> implements $OrderItemCopyWith<$Res> {
  factory _$OrderItemCopyWith(_OrderItem value, $Res Function(_OrderItem) _then) = __$OrderItemCopyWithImpl;
@override @useResult
$Res call({
 String id, String productId, String productName, String skuId, String skuName, String quantity, String unitPrice, String listPrice, String originalPrice, String actualPayAmount, String refundStatus, String shareDiscount, String imageUrl
});




}
/// @nodoc
class __$OrderItemCopyWithImpl<$Res>
    implements _$OrderItemCopyWith<$Res> {
  __$OrderItemCopyWithImpl(this._self, this._then);

  final _OrderItem _self;
  final $Res Function(_OrderItem) _then;

/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? productId = null,Object? productName = null,Object? skuId = null,Object? skuName = null,Object? quantity = null,Object? unitPrice = null,Object? listPrice = null,Object? originalPrice = null,Object? actualPayAmount = null,Object? refundStatus = null,Object? shareDiscount = null,Object? imageUrl = null,}) {
  return _then(_OrderItem(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,productId: null == productId ? _self.productId : productId // ignore: cast_nullable_to_non_nullable
as String,productName: null == productName ? _self.productName : productName // ignore: cast_nullable_to_non_nullable
as String,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String,skuName: null == skuName ? _self.skuName : skuName // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as String,unitPrice: null == unitPrice ? _self.unitPrice : unitPrice // ignore: cast_nullable_to_non_nullable
as String,listPrice: null == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as String,originalPrice: null == originalPrice ? _self.originalPrice : originalPrice // ignore: cast_nullable_to_non_nullable
as String,actualPayAmount: null == actualPayAmount ? _self.actualPayAmount : actualPayAmount // ignore: cast_nullable_to_non_nullable
as String,refundStatus: null == refundStatus ? _self.refundStatus : refundStatus // ignore: cast_nullable_to_non_nullable
as String,shareDiscount: null == shareDiscount ? _self.shareDiscount : shareDiscount // ignore: cast_nullable_to_non_nullable
as String,imageUrl: null == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$RefundRecord {

 String get requestNo; String get orderItemId; String get status; String get refundAmount; String get reason; String get createTime;
/// Create a copy of RefundRecord
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$RefundRecordCopyWith<RefundRecord> get copyWith => _$RefundRecordCopyWithImpl<RefundRecord>(this as RefundRecord, _$identity);

  /// Serializes this RefundRecord to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RefundRecord&&(identical(other.requestNo, requestNo) || other.requestNo == requestNo)&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.status, status) || other.status == status)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.createTime, createTime) || other.createTime == createTime));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,requestNo,orderItemId,status,refundAmount,reason,createTime);

@override
String toString() {
  return 'RefundRecord(requestNo: $requestNo, orderItemId: $orderItemId, status: $status, refundAmount: $refundAmount, reason: $reason, createTime: $createTime)';
}


}

/// @nodoc
abstract mixin class $RefundRecordCopyWith<$Res>  {
  factory $RefundRecordCopyWith(RefundRecord value, $Res Function(RefundRecord) _then) = _$RefundRecordCopyWithImpl;
@useResult
$Res call({
 String requestNo, String orderItemId, String status, String refundAmount, String reason, String createTime
});




}
/// @nodoc
class _$RefundRecordCopyWithImpl<$Res>
    implements $RefundRecordCopyWith<$Res> {
  _$RefundRecordCopyWithImpl(this._self, this._then);

  final RefundRecord _self;
  final $Res Function(RefundRecord) _then;

/// Create a copy of RefundRecord
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? requestNo = null,Object? orderItemId = null,Object? status = null,Object? refundAmount = null,Object? reason = null,Object? createTime = null,}) {
  return _then(_self.copyWith(
requestNo: null == requestNo ? _self.requestNo : requestNo // ignore: cast_nullable_to_non_nullable
as String,orderItemId: null == orderItemId ? _self.orderItemId : orderItemId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,refundAmount: null == refundAmount ? _self.refundAmount : refundAmount // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,createTime: null == createTime ? _self.createTime : createTime // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [RefundRecord].
extension RefundRecordPatterns on RefundRecord {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _RefundRecord value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _RefundRecord() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _RefundRecord value)  $default,){
final _that = this;
switch (_that) {
case _RefundRecord():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _RefundRecord value)?  $default,){
final _that = this;
switch (_that) {
case _RefundRecord() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String requestNo,  String orderItemId,  String status,  String refundAmount,  String reason,  String createTime)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _RefundRecord() when $default != null:
return $default(_that.requestNo,_that.orderItemId,_that.status,_that.refundAmount,_that.reason,_that.createTime);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String requestNo,  String orderItemId,  String status,  String refundAmount,  String reason,  String createTime)  $default,) {final _that = this;
switch (_that) {
case _RefundRecord():
return $default(_that.requestNo,_that.orderItemId,_that.status,_that.refundAmount,_that.reason,_that.createTime);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String requestNo,  String orderItemId,  String status,  String refundAmount,  String reason,  String createTime)?  $default,) {final _that = this;
switch (_that) {
case _RefundRecord() when $default != null:
return $default(_that.requestNo,_that.orderItemId,_that.status,_that.refundAmount,_that.reason,_that.createTime);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _RefundRecord implements RefundRecord {
  const _RefundRecord({this.requestNo = '', this.orderItemId = '', this.status = '', this.refundAmount = '', this.reason = '', this.createTime = ''});
  factory _RefundRecord.fromJson(Map<String, dynamic> json) => _$RefundRecordFromJson(json);

@override@JsonKey() final  String requestNo;
@override@JsonKey() final  String orderItemId;
@override@JsonKey() final  String status;
@override@JsonKey() final  String refundAmount;
@override@JsonKey() final  String reason;
@override@JsonKey() final  String createTime;

/// Create a copy of RefundRecord
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$RefundRecordCopyWith<_RefundRecord> get copyWith => __$RefundRecordCopyWithImpl<_RefundRecord>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$RefundRecordToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _RefundRecord&&(identical(other.requestNo, requestNo) || other.requestNo == requestNo)&&(identical(other.orderItemId, orderItemId) || other.orderItemId == orderItemId)&&(identical(other.status, status) || other.status == status)&&(identical(other.refundAmount, refundAmount) || other.refundAmount == refundAmount)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.createTime, createTime) || other.createTime == createTime));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,requestNo,orderItemId,status,refundAmount,reason,createTime);

@override
String toString() {
  return 'RefundRecord(requestNo: $requestNo, orderItemId: $orderItemId, status: $status, refundAmount: $refundAmount, reason: $reason, createTime: $createTime)';
}


}

/// @nodoc
abstract mixin class _$RefundRecordCopyWith<$Res> implements $RefundRecordCopyWith<$Res> {
  factory _$RefundRecordCopyWith(_RefundRecord value, $Res Function(_RefundRecord) _then) = __$RefundRecordCopyWithImpl;
@override @useResult
$Res call({
 String requestNo, String orderItemId, String status, String refundAmount, String reason, String createTime
});




}
/// @nodoc
class __$RefundRecordCopyWithImpl<$Res>
    implements _$RefundRecordCopyWith<$Res> {
  __$RefundRecordCopyWithImpl(this._self, this._then);

  final _RefundRecord _self;
  final $Res Function(_RefundRecord) _then;

/// Create a copy of RefundRecord
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? requestNo = null,Object? orderItemId = null,Object? status = null,Object? refundAmount = null,Object? reason = null,Object? createTime = null,}) {
  return _then(_RefundRecord(
requestNo: null == requestNo ? _self.requestNo : requestNo // ignore: cast_nullable_to_non_nullable
as String,orderItemId: null == orderItemId ? _self.orderItemId : orderItemId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,refundAmount: null == refundAmount ? _self.refundAmount : refundAmount // ignore: cast_nullable_to_non_nullable
as String,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,createTime: null == createTime ? _self.createTime : createTime // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$Order {

 int get id;@JsonKey(name: 'orderNo') String get orderSn; String get status; String get totalOriginalAmount; String get totalActualAmount; String get createTime; String get payTime; String get shipTime; String get shipmentNo; String get transactionId; String get buyerId; String get deliveryTemp; String get shippingMethod; String get shippingFee; String get shippingFeePayer; String get shippingArea; String get couponId; String get discountAmount; String get shippingStatus; String get statusTime; String get statusSummary;/// B-124：送達後自動完成時間（ISO 8601 UTC，status=40 時有值）
/// 政策：送達後 3 天自動完成（從 7 天縮短，2026-04-25 後生效）
 String get autoCompleteAt;/// B-132：買家付款後賣家須於此時間前處理物流，逾時後端自動取消。
 String get processingDeadline; String get labelUrl; String get recipientName; String get recipientPhone; String get shippingAddress; String get shippingNote; String get orderRemark; String get partnerId; String get odno; String get validationNo; String get paymentMethod; String get atmBankCode; String get atmVirtualAccount; String get atmExpireDate;// 付款期限（ISO 8601 UTC，僅 status='0' 時有值；超過後訂單會被系統自動取消，cancelReason 9101）
 String get paymentDeadline; String get itemCount; String get sellerName;@JsonKey(name: 'logoUrl') String get sellerAvatarUrl; String get sellerCity; String get sellerDistrict; String get pickupStoreName; String get storeId; bool get isReviewed;/// B-101：後端回傳目前是否仍可評價。預設 true 以保留舊 API 行為。
 bool get canReview;/// B-101：評價截止時間（ISO 8601 UTC，status=50 且可評價時有值）。
 String get reviewDeadline;/// 買家給此訂單的評分（1~5），未評分為 0。
/// 後端可能放在 `reviewRating` 或 `review.rating`，由 fromJson 正規化。
 int get reviewRating;/// 買家評論內容（reviewContent / review.content），未填為空字串。
 String get reviewContent; String get cancelReason; String get cancelRemark; String get cancelByRole;/// status=80 時若為 true，表示買家需手動填寫 ATM 退款銀行資訊。
/// 觸發 POST /app/orders/{order_no}/refund_page 取得 PayUni WebView URL。
 bool get needRefundPage;/// 賣家出貨上傳的商品照片（status>=20 後才會有值）
 List<String> get shipmentPhotoUrls;/// 買家確認收貨上傳的照片（status>=40 後才會有值）
 List<String> get receiptPhotoUrls;/// 再次付款可用付款方式（依 totalActualAmount 算；僅 pending 時非空）。§3.4.1
/// 每筆 = {paymentType, eligible, ineligibleReason}。
 List<Map<String, dynamic>> get availablePaymentMethods; List<OrderItem> get items; List<RefundRecord> get refundHistory;
/// Create a copy of Order
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderCopyWith<Order> get copyWith => _$OrderCopyWithImpl<Order>(this as Order, _$identity);

  /// Serializes this Order to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Order&&(identical(other.id, id) || other.id == id)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.status, status) || other.status == status)&&(identical(other.totalOriginalAmount, totalOriginalAmount) || other.totalOriginalAmount == totalOriginalAmount)&&(identical(other.totalActualAmount, totalActualAmount) || other.totalActualAmount == totalActualAmount)&&(identical(other.createTime, createTime) || other.createTime == createTime)&&(identical(other.payTime, payTime) || other.payTime == payTime)&&(identical(other.shipTime, shipTime) || other.shipTime == shipTime)&&(identical(other.shipmentNo, shipmentNo) || other.shipmentNo == shipmentNo)&&(identical(other.transactionId, transactionId) || other.transactionId == transactionId)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.deliveryTemp, deliveryTemp) || other.deliveryTemp == deliveryTemp)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.shippingFee, shippingFee) || other.shippingFee == shippingFee)&&(identical(other.shippingFeePayer, shippingFeePayer) || other.shippingFeePayer == shippingFeePayer)&&(identical(other.shippingArea, shippingArea) || other.shippingArea == shippingArea)&&(identical(other.couponId, couponId) || other.couponId == couponId)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.shippingStatus, shippingStatus) || other.shippingStatus == shippingStatus)&&(identical(other.statusTime, statusTime) || other.statusTime == statusTime)&&(identical(other.statusSummary, statusSummary) || other.statusSummary == statusSummary)&&(identical(other.autoCompleteAt, autoCompleteAt) || other.autoCompleteAt == autoCompleteAt)&&(identical(other.processingDeadline, processingDeadline) || other.processingDeadline == processingDeadline)&&(identical(other.labelUrl, labelUrl) || other.labelUrl == labelUrl)&&(identical(other.recipientName, recipientName) || other.recipientName == recipientName)&&(identical(other.recipientPhone, recipientPhone) || other.recipientPhone == recipientPhone)&&(identical(other.shippingAddress, shippingAddress) || other.shippingAddress == shippingAddress)&&(identical(other.shippingNote, shippingNote) || other.shippingNote == shippingNote)&&(identical(other.orderRemark, orderRemark) || other.orderRemark == orderRemark)&&(identical(other.partnerId, partnerId) || other.partnerId == partnerId)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.paymentMethod, paymentMethod) || other.paymentMethod == paymentMethod)&&(identical(other.atmBankCode, atmBankCode) || other.atmBankCode == atmBankCode)&&(identical(other.atmVirtualAccount, atmVirtualAccount) || other.atmVirtualAccount == atmVirtualAccount)&&(identical(other.atmExpireDate, atmExpireDate) || other.atmExpireDate == atmExpireDate)&&(identical(other.paymentDeadline, paymentDeadline) || other.paymentDeadline == paymentDeadline)&&(identical(other.itemCount, itemCount) || other.itemCount == itemCount)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.sellerAvatarUrl, sellerAvatarUrl) || other.sellerAvatarUrl == sellerAvatarUrl)&&(identical(other.sellerCity, sellerCity) || other.sellerCity == sellerCity)&&(identical(other.sellerDistrict, sellerDistrict) || other.sellerDistrict == sellerDistrict)&&(identical(other.pickupStoreName, pickupStoreName) || other.pickupStoreName == pickupStoreName)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.isReviewed, isReviewed) || other.isReviewed == isReviewed)&&(identical(other.canReview, canReview) || other.canReview == canReview)&&(identical(other.reviewDeadline, reviewDeadline) || other.reviewDeadline == reviewDeadline)&&(identical(other.reviewRating, reviewRating) || other.reviewRating == reviewRating)&&(identical(other.reviewContent, reviewContent) || other.reviewContent == reviewContent)&&(identical(other.cancelReason, cancelReason) || other.cancelReason == cancelReason)&&(identical(other.cancelRemark, cancelRemark) || other.cancelRemark == cancelRemark)&&(identical(other.cancelByRole, cancelByRole) || other.cancelByRole == cancelByRole)&&(identical(other.needRefundPage, needRefundPage) || other.needRefundPage == needRefundPage)&&const DeepCollectionEquality().equals(other.shipmentPhotoUrls, shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other.receiptPhotoUrls, receiptPhotoUrls)&&const DeepCollectionEquality().equals(other.availablePaymentMethods, availablePaymentMethods)&&const DeepCollectionEquality().equals(other.items, items)&&const DeepCollectionEquality().equals(other.refundHistory, refundHistory));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,id,orderSn,status,totalOriginalAmount,totalActualAmount,createTime,payTime,shipTime,shipmentNo,transactionId,buyerId,deliveryTemp,shippingMethod,shippingFee,shippingFeePayer,shippingArea,couponId,discountAmount,shippingStatus,statusTime,statusSummary,autoCompleteAt,processingDeadline,labelUrl,recipientName,recipientPhone,shippingAddress,shippingNote,orderRemark,partnerId,odno,validationNo,paymentMethod,atmBankCode,atmVirtualAccount,atmExpireDate,paymentDeadline,itemCount,sellerName,sellerAvatarUrl,sellerCity,sellerDistrict,pickupStoreName,storeId,isReviewed,canReview,reviewDeadline,reviewRating,reviewContent,cancelReason,cancelRemark,cancelByRole,needRefundPage,const DeepCollectionEquality().hash(shipmentPhotoUrls),const DeepCollectionEquality().hash(receiptPhotoUrls),const DeepCollectionEquality().hash(availablePaymentMethods),const DeepCollectionEquality().hash(items),const DeepCollectionEquality().hash(refundHistory)]);

@override
String toString() {
  return 'Order(id: $id, orderSn: $orderSn, status: $status, totalOriginalAmount: $totalOriginalAmount, totalActualAmount: $totalActualAmount, createTime: $createTime, payTime: $payTime, shipTime: $shipTime, shipmentNo: $shipmentNo, transactionId: $transactionId, buyerId: $buyerId, deliveryTemp: $deliveryTemp, shippingMethod: $shippingMethod, shippingFee: $shippingFee, shippingFeePayer: $shippingFeePayer, shippingArea: $shippingArea, couponId: $couponId, discountAmount: $discountAmount, shippingStatus: $shippingStatus, statusTime: $statusTime, statusSummary: $statusSummary, autoCompleteAt: $autoCompleteAt, processingDeadline: $processingDeadline, labelUrl: $labelUrl, recipientName: $recipientName, recipientPhone: $recipientPhone, shippingAddress: $shippingAddress, shippingNote: $shippingNote, orderRemark: $orderRemark, partnerId: $partnerId, odno: $odno, validationNo: $validationNo, paymentMethod: $paymentMethod, atmBankCode: $atmBankCode, atmVirtualAccount: $atmVirtualAccount, atmExpireDate: $atmExpireDate, paymentDeadline: $paymentDeadline, itemCount: $itemCount, sellerName: $sellerName, sellerAvatarUrl: $sellerAvatarUrl, sellerCity: $sellerCity, sellerDistrict: $sellerDistrict, pickupStoreName: $pickupStoreName, storeId: $storeId, isReviewed: $isReviewed, canReview: $canReview, reviewDeadline: $reviewDeadline, reviewRating: $reviewRating, reviewContent: $reviewContent, cancelReason: $cancelReason, cancelRemark: $cancelRemark, cancelByRole: $cancelByRole, needRefundPage: $needRefundPage, shipmentPhotoUrls: $shipmentPhotoUrls, receiptPhotoUrls: $receiptPhotoUrls, availablePaymentMethods: $availablePaymentMethods, items: $items, refundHistory: $refundHistory)';
}


}

/// @nodoc
abstract mixin class $OrderCopyWith<$Res>  {
  factory $OrderCopyWith(Order value, $Res Function(Order) _then) = _$OrderCopyWithImpl;
@useResult
$Res call({
 int id,@JsonKey(name: 'orderNo') String orderSn, String status, String totalOriginalAmount, String totalActualAmount, String createTime, String payTime, String shipTime, String shipmentNo, String transactionId, String buyerId, String deliveryTemp, String shippingMethod, String shippingFee, String shippingFeePayer, String shippingArea, String couponId, String discountAmount, String shippingStatus, String statusTime, String statusSummary, String autoCompleteAt, String processingDeadline, String labelUrl, String recipientName, String recipientPhone, String shippingAddress, String shippingNote, String orderRemark, String partnerId, String odno, String validationNo, String paymentMethod, String atmBankCode, String atmVirtualAccount, String atmExpireDate, String paymentDeadline, String itemCount, String sellerName,@JsonKey(name: 'logoUrl') String sellerAvatarUrl, String sellerCity, String sellerDistrict, String pickupStoreName, String storeId, bool isReviewed, bool canReview, String reviewDeadline, int reviewRating, String reviewContent, String cancelReason, String cancelRemark, String cancelByRole, bool needRefundPage, List<String> shipmentPhotoUrls, List<String> receiptPhotoUrls, List<Map<String, dynamic>> availablePaymentMethods, List<OrderItem> items, List<RefundRecord> refundHistory
});




}
/// @nodoc
class _$OrderCopyWithImpl<$Res>
    implements $OrderCopyWith<$Res> {
  _$OrderCopyWithImpl(this._self, this._then);

  final Order _self;
  final $Res Function(Order) _then;

/// Create a copy of Order
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? orderSn = null,Object? status = null,Object? totalOriginalAmount = null,Object? totalActualAmount = null,Object? createTime = null,Object? payTime = null,Object? shipTime = null,Object? shipmentNo = null,Object? transactionId = null,Object? buyerId = null,Object? deliveryTemp = null,Object? shippingMethod = null,Object? shippingFee = null,Object? shippingFeePayer = null,Object? shippingArea = null,Object? couponId = null,Object? discountAmount = null,Object? shippingStatus = null,Object? statusTime = null,Object? statusSummary = null,Object? autoCompleteAt = null,Object? processingDeadline = null,Object? labelUrl = null,Object? recipientName = null,Object? recipientPhone = null,Object? shippingAddress = null,Object? shippingNote = null,Object? orderRemark = null,Object? partnerId = null,Object? odno = null,Object? validationNo = null,Object? paymentMethod = null,Object? atmBankCode = null,Object? atmVirtualAccount = null,Object? atmExpireDate = null,Object? paymentDeadline = null,Object? itemCount = null,Object? sellerName = null,Object? sellerAvatarUrl = null,Object? sellerCity = null,Object? sellerDistrict = null,Object? pickupStoreName = null,Object? storeId = null,Object? isReviewed = null,Object? canReview = null,Object? reviewDeadline = null,Object? reviewRating = null,Object? reviewContent = null,Object? cancelReason = null,Object? cancelRemark = null,Object? cancelByRole = null,Object? needRefundPage = null,Object? shipmentPhotoUrls = null,Object? receiptPhotoUrls = null,Object? availablePaymentMethods = null,Object? items = null,Object? refundHistory = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,totalOriginalAmount: null == totalOriginalAmount ? _self.totalOriginalAmount : totalOriginalAmount // ignore: cast_nullable_to_non_nullable
as String,totalActualAmount: null == totalActualAmount ? _self.totalActualAmount : totalActualAmount // ignore: cast_nullable_to_non_nullable
as String,createTime: null == createTime ? _self.createTime : createTime // ignore: cast_nullable_to_non_nullable
as String,payTime: null == payTime ? _self.payTime : payTime // ignore: cast_nullable_to_non_nullable
as String,shipTime: null == shipTime ? _self.shipTime : shipTime // ignore: cast_nullable_to_non_nullable
as String,shipmentNo: null == shipmentNo ? _self.shipmentNo : shipmentNo // ignore: cast_nullable_to_non_nullable
as String,transactionId: null == transactionId ? _self.transactionId : transactionId // ignore: cast_nullable_to_non_nullable
as String,buyerId: null == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as String,deliveryTemp: null == deliveryTemp ? _self.deliveryTemp : deliveryTemp // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,shippingFee: null == shippingFee ? _self.shippingFee : shippingFee // ignore: cast_nullable_to_non_nullable
as String,shippingFeePayer: null == shippingFeePayer ? _self.shippingFeePayer : shippingFeePayer // ignore: cast_nullable_to_non_nullable
as String,shippingArea: null == shippingArea ? _self.shippingArea : shippingArea // ignore: cast_nullable_to_non_nullable
as String,couponId: null == couponId ? _self.couponId : couponId // ignore: cast_nullable_to_non_nullable
as String,discountAmount: null == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as String,shippingStatus: null == shippingStatus ? _self.shippingStatus : shippingStatus // ignore: cast_nullable_to_non_nullable
as String,statusTime: null == statusTime ? _self.statusTime : statusTime // ignore: cast_nullable_to_non_nullable
as String,statusSummary: null == statusSummary ? _self.statusSummary : statusSummary // ignore: cast_nullable_to_non_nullable
as String,autoCompleteAt: null == autoCompleteAt ? _self.autoCompleteAt : autoCompleteAt // ignore: cast_nullable_to_non_nullable
as String,processingDeadline: null == processingDeadline ? _self.processingDeadline : processingDeadline // ignore: cast_nullable_to_non_nullable
as String,labelUrl: null == labelUrl ? _self.labelUrl : labelUrl // ignore: cast_nullable_to_non_nullable
as String,recipientName: null == recipientName ? _self.recipientName : recipientName // ignore: cast_nullable_to_non_nullable
as String,recipientPhone: null == recipientPhone ? _self.recipientPhone : recipientPhone // ignore: cast_nullable_to_non_nullable
as String,shippingAddress: null == shippingAddress ? _self.shippingAddress : shippingAddress // ignore: cast_nullable_to_non_nullable
as String,shippingNote: null == shippingNote ? _self.shippingNote : shippingNote // ignore: cast_nullable_to_non_nullable
as String,orderRemark: null == orderRemark ? _self.orderRemark : orderRemark // ignore: cast_nullable_to_non_nullable
as String,partnerId: null == partnerId ? _self.partnerId : partnerId // ignore: cast_nullable_to_non_nullable
as String,odno: null == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String,validationNo: null == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String,paymentMethod: null == paymentMethod ? _self.paymentMethod : paymentMethod // ignore: cast_nullable_to_non_nullable
as String,atmBankCode: null == atmBankCode ? _self.atmBankCode : atmBankCode // ignore: cast_nullable_to_non_nullable
as String,atmVirtualAccount: null == atmVirtualAccount ? _self.atmVirtualAccount : atmVirtualAccount // ignore: cast_nullable_to_non_nullable
as String,atmExpireDate: null == atmExpireDate ? _self.atmExpireDate : atmExpireDate // ignore: cast_nullable_to_non_nullable
as String,paymentDeadline: null == paymentDeadline ? _self.paymentDeadline : paymentDeadline // ignore: cast_nullable_to_non_nullable
as String,itemCount: null == itemCount ? _self.itemCount : itemCount // ignore: cast_nullable_to_non_nullable
as String,sellerName: null == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String,sellerAvatarUrl: null == sellerAvatarUrl ? _self.sellerAvatarUrl : sellerAvatarUrl // ignore: cast_nullable_to_non_nullable
as String,sellerCity: null == sellerCity ? _self.sellerCity : sellerCity // ignore: cast_nullable_to_non_nullable
as String,sellerDistrict: null == sellerDistrict ? _self.sellerDistrict : sellerDistrict // ignore: cast_nullable_to_non_nullable
as String,pickupStoreName: null == pickupStoreName ? _self.pickupStoreName : pickupStoreName // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,isReviewed: null == isReviewed ? _self.isReviewed : isReviewed // ignore: cast_nullable_to_non_nullable
as bool,canReview: null == canReview ? _self.canReview : canReview // ignore: cast_nullable_to_non_nullable
as bool,reviewDeadline: null == reviewDeadline ? _self.reviewDeadline : reviewDeadline // ignore: cast_nullable_to_non_nullable
as String,reviewRating: null == reviewRating ? _self.reviewRating : reviewRating // ignore: cast_nullable_to_non_nullable
as int,reviewContent: null == reviewContent ? _self.reviewContent : reviewContent // ignore: cast_nullable_to_non_nullable
as String,cancelReason: null == cancelReason ? _self.cancelReason : cancelReason // ignore: cast_nullable_to_non_nullable
as String,cancelRemark: null == cancelRemark ? _self.cancelRemark : cancelRemark // ignore: cast_nullable_to_non_nullable
as String,cancelByRole: null == cancelByRole ? _self.cancelByRole : cancelByRole // ignore: cast_nullable_to_non_nullable
as String,needRefundPage: null == needRefundPage ? _self.needRefundPage : needRefundPage // ignore: cast_nullable_to_non_nullable
as bool,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self.shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,receiptPhotoUrls: null == receiptPhotoUrls ? _self.receiptPhotoUrls : receiptPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,availablePaymentMethods: null == availablePaymentMethods ? _self.availablePaymentMethods : availablePaymentMethods // ignore: cast_nullable_to_non_nullable
as List<Map<String, dynamic>>,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<OrderItem>,refundHistory: null == refundHistory ? _self.refundHistory : refundHistory // ignore: cast_nullable_to_non_nullable
as List<RefundRecord>,
  ));
}

}


/// Adds pattern-matching-related methods to [Order].
extension OrderPatterns on Order {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Order value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Order() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Order value)  $default,){
final _that = this;
switch (_that) {
case _Order():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Order value)?  $default,){
final _that = this;
switch (_that) {
case _Order() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id, @JsonKey(name: 'orderNo')  String orderSn,  String status,  String totalOriginalAmount,  String totalActualAmount,  String createTime,  String payTime,  String shipTime,  String shipmentNo,  String transactionId,  String buyerId,  String deliveryTemp,  String shippingMethod,  String shippingFee,  String shippingFeePayer,  String shippingArea,  String couponId,  String discountAmount,  String shippingStatus,  String statusTime,  String statusSummary,  String autoCompleteAt,  String processingDeadline,  String labelUrl,  String recipientName,  String recipientPhone,  String shippingAddress,  String shippingNote,  String orderRemark,  String partnerId,  String odno,  String validationNo,  String paymentMethod,  String atmBankCode,  String atmVirtualAccount,  String atmExpireDate,  String paymentDeadline,  String itemCount,  String sellerName, @JsonKey(name: 'logoUrl')  String sellerAvatarUrl,  String sellerCity,  String sellerDistrict,  String pickupStoreName,  String storeId,  bool isReviewed,  bool canReview,  String reviewDeadline,  int reviewRating,  String reviewContent,  String cancelReason,  String cancelRemark,  String cancelByRole,  bool needRefundPage,  List<String> shipmentPhotoUrls,  List<String> receiptPhotoUrls,  List<Map<String, dynamic>> availablePaymentMethods,  List<OrderItem> items,  List<RefundRecord> refundHistory)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Order() when $default != null:
return $default(_that.id,_that.orderSn,_that.status,_that.totalOriginalAmount,_that.totalActualAmount,_that.createTime,_that.payTime,_that.shipTime,_that.shipmentNo,_that.transactionId,_that.buyerId,_that.deliveryTemp,_that.shippingMethod,_that.shippingFee,_that.shippingFeePayer,_that.shippingArea,_that.couponId,_that.discountAmount,_that.shippingStatus,_that.statusTime,_that.statusSummary,_that.autoCompleteAt,_that.processingDeadline,_that.labelUrl,_that.recipientName,_that.recipientPhone,_that.shippingAddress,_that.shippingNote,_that.orderRemark,_that.partnerId,_that.odno,_that.validationNo,_that.paymentMethod,_that.atmBankCode,_that.atmVirtualAccount,_that.atmExpireDate,_that.paymentDeadline,_that.itemCount,_that.sellerName,_that.sellerAvatarUrl,_that.sellerCity,_that.sellerDistrict,_that.pickupStoreName,_that.storeId,_that.isReviewed,_that.canReview,_that.reviewDeadline,_that.reviewRating,_that.reviewContent,_that.cancelReason,_that.cancelRemark,_that.cancelByRole,_that.needRefundPage,_that.shipmentPhotoUrls,_that.receiptPhotoUrls,_that.availablePaymentMethods,_that.items,_that.refundHistory);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id, @JsonKey(name: 'orderNo')  String orderSn,  String status,  String totalOriginalAmount,  String totalActualAmount,  String createTime,  String payTime,  String shipTime,  String shipmentNo,  String transactionId,  String buyerId,  String deliveryTemp,  String shippingMethod,  String shippingFee,  String shippingFeePayer,  String shippingArea,  String couponId,  String discountAmount,  String shippingStatus,  String statusTime,  String statusSummary,  String autoCompleteAt,  String processingDeadline,  String labelUrl,  String recipientName,  String recipientPhone,  String shippingAddress,  String shippingNote,  String orderRemark,  String partnerId,  String odno,  String validationNo,  String paymentMethod,  String atmBankCode,  String atmVirtualAccount,  String atmExpireDate,  String paymentDeadline,  String itemCount,  String sellerName, @JsonKey(name: 'logoUrl')  String sellerAvatarUrl,  String sellerCity,  String sellerDistrict,  String pickupStoreName,  String storeId,  bool isReviewed,  bool canReview,  String reviewDeadline,  int reviewRating,  String reviewContent,  String cancelReason,  String cancelRemark,  String cancelByRole,  bool needRefundPage,  List<String> shipmentPhotoUrls,  List<String> receiptPhotoUrls,  List<Map<String, dynamic>> availablePaymentMethods,  List<OrderItem> items,  List<RefundRecord> refundHistory)  $default,) {final _that = this;
switch (_that) {
case _Order():
return $default(_that.id,_that.orderSn,_that.status,_that.totalOriginalAmount,_that.totalActualAmount,_that.createTime,_that.payTime,_that.shipTime,_that.shipmentNo,_that.transactionId,_that.buyerId,_that.deliveryTemp,_that.shippingMethod,_that.shippingFee,_that.shippingFeePayer,_that.shippingArea,_that.couponId,_that.discountAmount,_that.shippingStatus,_that.statusTime,_that.statusSummary,_that.autoCompleteAt,_that.processingDeadline,_that.labelUrl,_that.recipientName,_that.recipientPhone,_that.shippingAddress,_that.shippingNote,_that.orderRemark,_that.partnerId,_that.odno,_that.validationNo,_that.paymentMethod,_that.atmBankCode,_that.atmVirtualAccount,_that.atmExpireDate,_that.paymentDeadline,_that.itemCount,_that.sellerName,_that.sellerAvatarUrl,_that.sellerCity,_that.sellerDistrict,_that.pickupStoreName,_that.storeId,_that.isReviewed,_that.canReview,_that.reviewDeadline,_that.reviewRating,_that.reviewContent,_that.cancelReason,_that.cancelRemark,_that.cancelByRole,_that.needRefundPage,_that.shipmentPhotoUrls,_that.receiptPhotoUrls,_that.availablePaymentMethods,_that.items,_that.refundHistory);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id, @JsonKey(name: 'orderNo')  String orderSn,  String status,  String totalOriginalAmount,  String totalActualAmount,  String createTime,  String payTime,  String shipTime,  String shipmentNo,  String transactionId,  String buyerId,  String deliveryTemp,  String shippingMethod,  String shippingFee,  String shippingFeePayer,  String shippingArea,  String couponId,  String discountAmount,  String shippingStatus,  String statusTime,  String statusSummary,  String autoCompleteAt,  String processingDeadline,  String labelUrl,  String recipientName,  String recipientPhone,  String shippingAddress,  String shippingNote,  String orderRemark,  String partnerId,  String odno,  String validationNo,  String paymentMethod,  String atmBankCode,  String atmVirtualAccount,  String atmExpireDate,  String paymentDeadline,  String itemCount,  String sellerName, @JsonKey(name: 'logoUrl')  String sellerAvatarUrl,  String sellerCity,  String sellerDistrict,  String pickupStoreName,  String storeId,  bool isReviewed,  bool canReview,  String reviewDeadline,  int reviewRating,  String reviewContent,  String cancelReason,  String cancelRemark,  String cancelByRole,  bool needRefundPage,  List<String> shipmentPhotoUrls,  List<String> receiptPhotoUrls,  List<Map<String, dynamic>> availablePaymentMethods,  List<OrderItem> items,  List<RefundRecord> refundHistory)?  $default,) {final _that = this;
switch (_that) {
case _Order() when $default != null:
return $default(_that.id,_that.orderSn,_that.status,_that.totalOriginalAmount,_that.totalActualAmount,_that.createTime,_that.payTime,_that.shipTime,_that.shipmentNo,_that.transactionId,_that.buyerId,_that.deliveryTemp,_that.shippingMethod,_that.shippingFee,_that.shippingFeePayer,_that.shippingArea,_that.couponId,_that.discountAmount,_that.shippingStatus,_that.statusTime,_that.statusSummary,_that.autoCompleteAt,_that.processingDeadline,_that.labelUrl,_that.recipientName,_that.recipientPhone,_that.shippingAddress,_that.shippingNote,_that.orderRemark,_that.partnerId,_that.odno,_that.validationNo,_that.paymentMethod,_that.atmBankCode,_that.atmVirtualAccount,_that.atmExpireDate,_that.paymentDeadline,_that.itemCount,_that.sellerName,_that.sellerAvatarUrl,_that.sellerCity,_that.sellerDistrict,_that.pickupStoreName,_that.storeId,_that.isReviewed,_that.canReview,_that.reviewDeadline,_that.reviewRating,_that.reviewContent,_that.cancelReason,_that.cancelRemark,_that.cancelByRole,_that.needRefundPage,_that.shipmentPhotoUrls,_that.receiptPhotoUrls,_that.availablePaymentMethods,_that.items,_that.refundHistory);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Order implements Order {
  const _Order({this.id = 0, @JsonKey(name: 'orderNo') this.orderSn = '', this.status = '', this.totalOriginalAmount = '', this.totalActualAmount = '', this.createTime = '', this.payTime = '', this.shipTime = '', this.shipmentNo = '', this.transactionId = '', this.buyerId = '', this.deliveryTemp = '', this.shippingMethod = '', this.shippingFee = '', this.shippingFeePayer = '', this.shippingArea = '', this.couponId = '', this.discountAmount = '', this.shippingStatus = '', this.statusTime = '', this.statusSummary = '', this.autoCompleteAt = '', this.processingDeadline = '', this.labelUrl = '', this.recipientName = '', this.recipientPhone = '', this.shippingAddress = '', this.shippingNote = '', this.orderRemark = '', this.partnerId = '', this.odno = '', this.validationNo = '', this.paymentMethod = '', this.atmBankCode = '', this.atmVirtualAccount = '', this.atmExpireDate = '', this.paymentDeadline = '', this.itemCount = '', this.sellerName = '', @JsonKey(name: 'logoUrl') this.sellerAvatarUrl = '', this.sellerCity = '', this.sellerDistrict = '', this.pickupStoreName = '', this.storeId = '', this.isReviewed = false, this.canReview = true, this.reviewDeadline = '', this.reviewRating = 0, this.reviewContent = '', this.cancelReason = '', this.cancelRemark = '', this.cancelByRole = '', this.needRefundPage = false, final  List<String> shipmentPhotoUrls = const <String>[], final  List<String> receiptPhotoUrls = const <String>[], final  List<Map<String, dynamic>> availablePaymentMethods = const <Map<String, dynamic>>[], final  List<OrderItem> items = const [], final  List<RefundRecord> refundHistory = const []}): _shipmentPhotoUrls = shipmentPhotoUrls,_receiptPhotoUrls = receiptPhotoUrls,_availablePaymentMethods = availablePaymentMethods,_items = items,_refundHistory = refundHistory;
  factory _Order.fromJson(Map<String, dynamic> json) => _$OrderFromJson(json);

@override@JsonKey() final  int id;
@override@JsonKey(name: 'orderNo') final  String orderSn;
@override@JsonKey() final  String status;
@override@JsonKey() final  String totalOriginalAmount;
@override@JsonKey() final  String totalActualAmount;
@override@JsonKey() final  String createTime;
@override@JsonKey() final  String payTime;
@override@JsonKey() final  String shipTime;
@override@JsonKey() final  String shipmentNo;
@override@JsonKey() final  String transactionId;
@override@JsonKey() final  String buyerId;
@override@JsonKey() final  String deliveryTemp;
@override@JsonKey() final  String shippingMethod;
@override@JsonKey() final  String shippingFee;
@override@JsonKey() final  String shippingFeePayer;
@override@JsonKey() final  String shippingArea;
@override@JsonKey() final  String couponId;
@override@JsonKey() final  String discountAmount;
@override@JsonKey() final  String shippingStatus;
@override@JsonKey() final  String statusTime;
@override@JsonKey() final  String statusSummary;
/// B-124：送達後自動完成時間（ISO 8601 UTC，status=40 時有值）
/// 政策：送達後 3 天自動完成（從 7 天縮短，2026-04-25 後生效）
@override@JsonKey() final  String autoCompleteAt;
/// B-132：買家付款後賣家須於此時間前處理物流，逾時後端自動取消。
@override@JsonKey() final  String processingDeadline;
@override@JsonKey() final  String labelUrl;
@override@JsonKey() final  String recipientName;
@override@JsonKey() final  String recipientPhone;
@override@JsonKey() final  String shippingAddress;
@override@JsonKey() final  String shippingNote;
@override@JsonKey() final  String orderRemark;
@override@JsonKey() final  String partnerId;
@override@JsonKey() final  String odno;
@override@JsonKey() final  String validationNo;
@override@JsonKey() final  String paymentMethod;
@override@JsonKey() final  String atmBankCode;
@override@JsonKey() final  String atmVirtualAccount;
@override@JsonKey() final  String atmExpireDate;
// 付款期限（ISO 8601 UTC，僅 status='0' 時有值；超過後訂單會被系統自動取消，cancelReason 9101）
@override@JsonKey() final  String paymentDeadline;
@override@JsonKey() final  String itemCount;
@override@JsonKey() final  String sellerName;
@override@JsonKey(name: 'logoUrl') final  String sellerAvatarUrl;
@override@JsonKey() final  String sellerCity;
@override@JsonKey() final  String sellerDistrict;
@override@JsonKey() final  String pickupStoreName;
@override@JsonKey() final  String storeId;
@override@JsonKey() final  bool isReviewed;
/// B-101：後端回傳目前是否仍可評價。預設 true 以保留舊 API 行為。
@override@JsonKey() final  bool canReview;
/// B-101：評價截止時間（ISO 8601 UTC，status=50 且可評價時有值）。
@override@JsonKey() final  String reviewDeadline;
/// 買家給此訂單的評分（1~5），未評分為 0。
/// 後端可能放在 `reviewRating` 或 `review.rating`，由 fromJson 正規化。
@override@JsonKey() final  int reviewRating;
/// 買家評論內容（reviewContent / review.content），未填為空字串。
@override@JsonKey() final  String reviewContent;
@override@JsonKey() final  String cancelReason;
@override@JsonKey() final  String cancelRemark;
@override@JsonKey() final  String cancelByRole;
/// status=80 時若為 true，表示買家需手動填寫 ATM 退款銀行資訊。
/// 觸發 POST /app/orders/{order_no}/refund_page 取得 PayUni WebView URL。
@override@JsonKey() final  bool needRefundPage;
/// 賣家出貨上傳的商品照片（status>=20 後才會有值）
 final  List<String> _shipmentPhotoUrls;
/// 賣家出貨上傳的商品照片（status>=20 後才會有值）
@override@JsonKey() List<String> get shipmentPhotoUrls {
  if (_shipmentPhotoUrls is EqualUnmodifiableListView) return _shipmentPhotoUrls;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_shipmentPhotoUrls);
}

/// 買家確認收貨上傳的照片（status>=40 後才會有值）
 final  List<String> _receiptPhotoUrls;
/// 買家確認收貨上傳的照片（status>=40 後才會有值）
@override@JsonKey() List<String> get receiptPhotoUrls {
  if (_receiptPhotoUrls is EqualUnmodifiableListView) return _receiptPhotoUrls;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_receiptPhotoUrls);
}

/// 再次付款可用付款方式（依 totalActualAmount 算；僅 pending 時非空）。§3.4.1
/// 每筆 = {paymentType, eligible, ineligibleReason}。
 final  List<Map<String, dynamic>> _availablePaymentMethods;
/// 再次付款可用付款方式（依 totalActualAmount 算；僅 pending 時非空）。§3.4.1
/// 每筆 = {paymentType, eligible, ineligibleReason}。
@override@JsonKey() List<Map<String, dynamic>> get availablePaymentMethods {
  if (_availablePaymentMethods is EqualUnmodifiableListView) return _availablePaymentMethods;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_availablePaymentMethods);
}

 final  List<OrderItem> _items;
@override@JsonKey() List<OrderItem> get items {
  if (_items is EqualUnmodifiableListView) return _items;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_items);
}

 final  List<RefundRecord> _refundHistory;
@override@JsonKey() List<RefundRecord> get refundHistory {
  if (_refundHistory is EqualUnmodifiableListView) return _refundHistory;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_refundHistory);
}


/// Create a copy of Order
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderCopyWith<_Order> get copyWith => __$OrderCopyWithImpl<_Order>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$OrderToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Order&&(identical(other.id, id) || other.id == id)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.status, status) || other.status == status)&&(identical(other.totalOriginalAmount, totalOriginalAmount) || other.totalOriginalAmount == totalOriginalAmount)&&(identical(other.totalActualAmount, totalActualAmount) || other.totalActualAmount == totalActualAmount)&&(identical(other.createTime, createTime) || other.createTime == createTime)&&(identical(other.payTime, payTime) || other.payTime == payTime)&&(identical(other.shipTime, shipTime) || other.shipTime == shipTime)&&(identical(other.shipmentNo, shipmentNo) || other.shipmentNo == shipmentNo)&&(identical(other.transactionId, transactionId) || other.transactionId == transactionId)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.deliveryTemp, deliveryTemp) || other.deliveryTemp == deliveryTemp)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.shippingFee, shippingFee) || other.shippingFee == shippingFee)&&(identical(other.shippingFeePayer, shippingFeePayer) || other.shippingFeePayer == shippingFeePayer)&&(identical(other.shippingArea, shippingArea) || other.shippingArea == shippingArea)&&(identical(other.couponId, couponId) || other.couponId == couponId)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.shippingStatus, shippingStatus) || other.shippingStatus == shippingStatus)&&(identical(other.statusTime, statusTime) || other.statusTime == statusTime)&&(identical(other.statusSummary, statusSummary) || other.statusSummary == statusSummary)&&(identical(other.autoCompleteAt, autoCompleteAt) || other.autoCompleteAt == autoCompleteAt)&&(identical(other.processingDeadline, processingDeadline) || other.processingDeadline == processingDeadline)&&(identical(other.labelUrl, labelUrl) || other.labelUrl == labelUrl)&&(identical(other.recipientName, recipientName) || other.recipientName == recipientName)&&(identical(other.recipientPhone, recipientPhone) || other.recipientPhone == recipientPhone)&&(identical(other.shippingAddress, shippingAddress) || other.shippingAddress == shippingAddress)&&(identical(other.shippingNote, shippingNote) || other.shippingNote == shippingNote)&&(identical(other.orderRemark, orderRemark) || other.orderRemark == orderRemark)&&(identical(other.partnerId, partnerId) || other.partnerId == partnerId)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.paymentMethod, paymentMethod) || other.paymentMethod == paymentMethod)&&(identical(other.atmBankCode, atmBankCode) || other.atmBankCode == atmBankCode)&&(identical(other.atmVirtualAccount, atmVirtualAccount) || other.atmVirtualAccount == atmVirtualAccount)&&(identical(other.atmExpireDate, atmExpireDate) || other.atmExpireDate == atmExpireDate)&&(identical(other.paymentDeadline, paymentDeadline) || other.paymentDeadline == paymentDeadline)&&(identical(other.itemCount, itemCount) || other.itemCount == itemCount)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&(identical(other.sellerAvatarUrl, sellerAvatarUrl) || other.sellerAvatarUrl == sellerAvatarUrl)&&(identical(other.sellerCity, sellerCity) || other.sellerCity == sellerCity)&&(identical(other.sellerDistrict, sellerDistrict) || other.sellerDistrict == sellerDistrict)&&(identical(other.pickupStoreName, pickupStoreName) || other.pickupStoreName == pickupStoreName)&&(identical(other.storeId, storeId) || other.storeId == storeId)&&(identical(other.isReviewed, isReviewed) || other.isReviewed == isReviewed)&&(identical(other.canReview, canReview) || other.canReview == canReview)&&(identical(other.reviewDeadline, reviewDeadline) || other.reviewDeadline == reviewDeadline)&&(identical(other.reviewRating, reviewRating) || other.reviewRating == reviewRating)&&(identical(other.reviewContent, reviewContent) || other.reviewContent == reviewContent)&&(identical(other.cancelReason, cancelReason) || other.cancelReason == cancelReason)&&(identical(other.cancelRemark, cancelRemark) || other.cancelRemark == cancelRemark)&&(identical(other.cancelByRole, cancelByRole) || other.cancelByRole == cancelByRole)&&(identical(other.needRefundPage, needRefundPage) || other.needRefundPage == needRefundPage)&&const DeepCollectionEquality().equals(other._shipmentPhotoUrls, _shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other._receiptPhotoUrls, _receiptPhotoUrls)&&const DeepCollectionEquality().equals(other._availablePaymentMethods, _availablePaymentMethods)&&const DeepCollectionEquality().equals(other._items, _items)&&const DeepCollectionEquality().equals(other._refundHistory, _refundHistory));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,id,orderSn,status,totalOriginalAmount,totalActualAmount,createTime,payTime,shipTime,shipmentNo,transactionId,buyerId,deliveryTemp,shippingMethod,shippingFee,shippingFeePayer,shippingArea,couponId,discountAmount,shippingStatus,statusTime,statusSummary,autoCompleteAt,processingDeadline,labelUrl,recipientName,recipientPhone,shippingAddress,shippingNote,orderRemark,partnerId,odno,validationNo,paymentMethod,atmBankCode,atmVirtualAccount,atmExpireDate,paymentDeadline,itemCount,sellerName,sellerAvatarUrl,sellerCity,sellerDistrict,pickupStoreName,storeId,isReviewed,canReview,reviewDeadline,reviewRating,reviewContent,cancelReason,cancelRemark,cancelByRole,needRefundPage,const DeepCollectionEquality().hash(_shipmentPhotoUrls),const DeepCollectionEquality().hash(_receiptPhotoUrls),const DeepCollectionEquality().hash(_availablePaymentMethods),const DeepCollectionEquality().hash(_items),const DeepCollectionEquality().hash(_refundHistory)]);

@override
String toString() {
  return 'Order(id: $id, orderSn: $orderSn, status: $status, totalOriginalAmount: $totalOriginalAmount, totalActualAmount: $totalActualAmount, createTime: $createTime, payTime: $payTime, shipTime: $shipTime, shipmentNo: $shipmentNo, transactionId: $transactionId, buyerId: $buyerId, deliveryTemp: $deliveryTemp, shippingMethod: $shippingMethod, shippingFee: $shippingFee, shippingFeePayer: $shippingFeePayer, shippingArea: $shippingArea, couponId: $couponId, discountAmount: $discountAmount, shippingStatus: $shippingStatus, statusTime: $statusTime, statusSummary: $statusSummary, autoCompleteAt: $autoCompleteAt, processingDeadline: $processingDeadline, labelUrl: $labelUrl, recipientName: $recipientName, recipientPhone: $recipientPhone, shippingAddress: $shippingAddress, shippingNote: $shippingNote, orderRemark: $orderRemark, partnerId: $partnerId, odno: $odno, validationNo: $validationNo, paymentMethod: $paymentMethod, atmBankCode: $atmBankCode, atmVirtualAccount: $atmVirtualAccount, atmExpireDate: $atmExpireDate, paymentDeadline: $paymentDeadline, itemCount: $itemCount, sellerName: $sellerName, sellerAvatarUrl: $sellerAvatarUrl, sellerCity: $sellerCity, sellerDistrict: $sellerDistrict, pickupStoreName: $pickupStoreName, storeId: $storeId, isReviewed: $isReviewed, canReview: $canReview, reviewDeadline: $reviewDeadline, reviewRating: $reviewRating, reviewContent: $reviewContent, cancelReason: $cancelReason, cancelRemark: $cancelRemark, cancelByRole: $cancelByRole, needRefundPage: $needRefundPage, shipmentPhotoUrls: $shipmentPhotoUrls, receiptPhotoUrls: $receiptPhotoUrls, availablePaymentMethods: $availablePaymentMethods, items: $items, refundHistory: $refundHistory)';
}


}

/// @nodoc
abstract mixin class _$OrderCopyWith<$Res> implements $OrderCopyWith<$Res> {
  factory _$OrderCopyWith(_Order value, $Res Function(_Order) _then) = __$OrderCopyWithImpl;
@override @useResult
$Res call({
 int id,@JsonKey(name: 'orderNo') String orderSn, String status, String totalOriginalAmount, String totalActualAmount, String createTime, String payTime, String shipTime, String shipmentNo, String transactionId, String buyerId, String deliveryTemp, String shippingMethod, String shippingFee, String shippingFeePayer, String shippingArea, String couponId, String discountAmount, String shippingStatus, String statusTime, String statusSummary, String autoCompleteAt, String processingDeadline, String labelUrl, String recipientName, String recipientPhone, String shippingAddress, String shippingNote, String orderRemark, String partnerId, String odno, String validationNo, String paymentMethod, String atmBankCode, String atmVirtualAccount, String atmExpireDate, String paymentDeadline, String itemCount, String sellerName,@JsonKey(name: 'logoUrl') String sellerAvatarUrl, String sellerCity, String sellerDistrict, String pickupStoreName, String storeId, bool isReviewed, bool canReview, String reviewDeadline, int reviewRating, String reviewContent, String cancelReason, String cancelRemark, String cancelByRole, bool needRefundPage, List<String> shipmentPhotoUrls, List<String> receiptPhotoUrls, List<Map<String, dynamic>> availablePaymentMethods, List<OrderItem> items, List<RefundRecord> refundHistory
});




}
/// @nodoc
class __$OrderCopyWithImpl<$Res>
    implements _$OrderCopyWith<$Res> {
  __$OrderCopyWithImpl(this._self, this._then);

  final _Order _self;
  final $Res Function(_Order) _then;

/// Create a copy of Order
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? orderSn = null,Object? status = null,Object? totalOriginalAmount = null,Object? totalActualAmount = null,Object? createTime = null,Object? payTime = null,Object? shipTime = null,Object? shipmentNo = null,Object? transactionId = null,Object? buyerId = null,Object? deliveryTemp = null,Object? shippingMethod = null,Object? shippingFee = null,Object? shippingFeePayer = null,Object? shippingArea = null,Object? couponId = null,Object? discountAmount = null,Object? shippingStatus = null,Object? statusTime = null,Object? statusSummary = null,Object? autoCompleteAt = null,Object? processingDeadline = null,Object? labelUrl = null,Object? recipientName = null,Object? recipientPhone = null,Object? shippingAddress = null,Object? shippingNote = null,Object? orderRemark = null,Object? partnerId = null,Object? odno = null,Object? validationNo = null,Object? paymentMethod = null,Object? atmBankCode = null,Object? atmVirtualAccount = null,Object? atmExpireDate = null,Object? paymentDeadline = null,Object? itemCount = null,Object? sellerName = null,Object? sellerAvatarUrl = null,Object? sellerCity = null,Object? sellerDistrict = null,Object? pickupStoreName = null,Object? storeId = null,Object? isReviewed = null,Object? canReview = null,Object? reviewDeadline = null,Object? reviewRating = null,Object? reviewContent = null,Object? cancelReason = null,Object? cancelRemark = null,Object? cancelByRole = null,Object? needRefundPage = null,Object? shipmentPhotoUrls = null,Object? receiptPhotoUrls = null,Object? availablePaymentMethods = null,Object? items = null,Object? refundHistory = null,}) {
  return _then(_Order(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,totalOriginalAmount: null == totalOriginalAmount ? _self.totalOriginalAmount : totalOriginalAmount // ignore: cast_nullable_to_non_nullable
as String,totalActualAmount: null == totalActualAmount ? _self.totalActualAmount : totalActualAmount // ignore: cast_nullable_to_non_nullable
as String,createTime: null == createTime ? _self.createTime : createTime // ignore: cast_nullable_to_non_nullable
as String,payTime: null == payTime ? _self.payTime : payTime // ignore: cast_nullable_to_non_nullable
as String,shipTime: null == shipTime ? _self.shipTime : shipTime // ignore: cast_nullable_to_non_nullable
as String,shipmentNo: null == shipmentNo ? _self.shipmentNo : shipmentNo // ignore: cast_nullable_to_non_nullable
as String,transactionId: null == transactionId ? _self.transactionId : transactionId // ignore: cast_nullable_to_non_nullable
as String,buyerId: null == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as String,deliveryTemp: null == deliveryTemp ? _self.deliveryTemp : deliveryTemp // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,shippingFee: null == shippingFee ? _self.shippingFee : shippingFee // ignore: cast_nullable_to_non_nullable
as String,shippingFeePayer: null == shippingFeePayer ? _self.shippingFeePayer : shippingFeePayer // ignore: cast_nullable_to_non_nullable
as String,shippingArea: null == shippingArea ? _self.shippingArea : shippingArea // ignore: cast_nullable_to_non_nullable
as String,couponId: null == couponId ? _self.couponId : couponId // ignore: cast_nullable_to_non_nullable
as String,discountAmount: null == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as String,shippingStatus: null == shippingStatus ? _self.shippingStatus : shippingStatus // ignore: cast_nullable_to_non_nullable
as String,statusTime: null == statusTime ? _self.statusTime : statusTime // ignore: cast_nullable_to_non_nullable
as String,statusSummary: null == statusSummary ? _self.statusSummary : statusSummary // ignore: cast_nullable_to_non_nullable
as String,autoCompleteAt: null == autoCompleteAt ? _self.autoCompleteAt : autoCompleteAt // ignore: cast_nullable_to_non_nullable
as String,processingDeadline: null == processingDeadline ? _self.processingDeadline : processingDeadline // ignore: cast_nullable_to_non_nullable
as String,labelUrl: null == labelUrl ? _self.labelUrl : labelUrl // ignore: cast_nullable_to_non_nullable
as String,recipientName: null == recipientName ? _self.recipientName : recipientName // ignore: cast_nullable_to_non_nullable
as String,recipientPhone: null == recipientPhone ? _self.recipientPhone : recipientPhone // ignore: cast_nullable_to_non_nullable
as String,shippingAddress: null == shippingAddress ? _self.shippingAddress : shippingAddress // ignore: cast_nullable_to_non_nullable
as String,shippingNote: null == shippingNote ? _self.shippingNote : shippingNote // ignore: cast_nullable_to_non_nullable
as String,orderRemark: null == orderRemark ? _self.orderRemark : orderRemark // ignore: cast_nullable_to_non_nullable
as String,partnerId: null == partnerId ? _self.partnerId : partnerId // ignore: cast_nullable_to_non_nullable
as String,odno: null == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String,validationNo: null == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String,paymentMethod: null == paymentMethod ? _self.paymentMethod : paymentMethod // ignore: cast_nullable_to_non_nullable
as String,atmBankCode: null == atmBankCode ? _self.atmBankCode : atmBankCode // ignore: cast_nullable_to_non_nullable
as String,atmVirtualAccount: null == atmVirtualAccount ? _self.atmVirtualAccount : atmVirtualAccount // ignore: cast_nullable_to_non_nullable
as String,atmExpireDate: null == atmExpireDate ? _self.atmExpireDate : atmExpireDate // ignore: cast_nullable_to_non_nullable
as String,paymentDeadline: null == paymentDeadline ? _self.paymentDeadline : paymentDeadline // ignore: cast_nullable_to_non_nullable
as String,itemCount: null == itemCount ? _self.itemCount : itemCount // ignore: cast_nullable_to_non_nullable
as String,sellerName: null == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String,sellerAvatarUrl: null == sellerAvatarUrl ? _self.sellerAvatarUrl : sellerAvatarUrl // ignore: cast_nullable_to_non_nullable
as String,sellerCity: null == sellerCity ? _self.sellerCity : sellerCity // ignore: cast_nullable_to_non_nullable
as String,sellerDistrict: null == sellerDistrict ? _self.sellerDistrict : sellerDistrict // ignore: cast_nullable_to_non_nullable
as String,pickupStoreName: null == pickupStoreName ? _self.pickupStoreName : pickupStoreName // ignore: cast_nullable_to_non_nullable
as String,storeId: null == storeId ? _self.storeId : storeId // ignore: cast_nullable_to_non_nullable
as String,isReviewed: null == isReviewed ? _self.isReviewed : isReviewed // ignore: cast_nullable_to_non_nullable
as bool,canReview: null == canReview ? _self.canReview : canReview // ignore: cast_nullable_to_non_nullable
as bool,reviewDeadline: null == reviewDeadline ? _self.reviewDeadline : reviewDeadline // ignore: cast_nullable_to_non_nullable
as String,reviewRating: null == reviewRating ? _self.reviewRating : reviewRating // ignore: cast_nullable_to_non_nullable
as int,reviewContent: null == reviewContent ? _self.reviewContent : reviewContent // ignore: cast_nullable_to_non_nullable
as String,cancelReason: null == cancelReason ? _self.cancelReason : cancelReason // ignore: cast_nullable_to_non_nullable
as String,cancelRemark: null == cancelRemark ? _self.cancelRemark : cancelRemark // ignore: cast_nullable_to_non_nullable
as String,cancelByRole: null == cancelByRole ? _self.cancelByRole : cancelByRole // ignore: cast_nullable_to_non_nullable
as String,needRefundPage: null == needRefundPage ? _self.needRefundPage : needRefundPage // ignore: cast_nullable_to_non_nullable
as bool,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self._shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,receiptPhotoUrls: null == receiptPhotoUrls ? _self._receiptPhotoUrls : receiptPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,availablePaymentMethods: null == availablePaymentMethods ? _self._availablePaymentMethods : availablePaymentMethods // ignore: cast_nullable_to_non_nullable
as List<Map<String, dynamic>>,items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<OrderItem>,refundHistory: null == refundHistory ? _self._refundHistory : refundHistory // ignore: cast_nullable_to_non_nullable
as List<RefundRecord>,
  ));
}


}

// dart format on
