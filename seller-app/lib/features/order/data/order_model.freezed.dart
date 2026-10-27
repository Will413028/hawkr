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

 String get name; String get price;// PayUni #758: 新欄位（成交單價 / 原價快照 / 折抵 / 實付）
 String get unitPrice; String get listPrice; String get shareDiscount; String get actualPayAmount; int get quantity;/// 商品圖 **objectKey**（49 §6.1 的 SSOT）。顯示 URL 一律交給
/// [OptimizedNetworkImage] / [buildImageUrl] 這個唯一組裝入口，模型層不存 URL。
 String get imageKey; String get skuId; String get spuId;
/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderItemCopyWith<OrderItem> get copyWith => _$OrderItemCopyWithImpl<OrderItem>(this as OrderItem, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OrderItem&&(identical(other.name, name) || other.name == name)&&(identical(other.price, price) || other.price == price)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.imageKey, imageKey) || other.imageKey == imageKey)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.spuId, spuId) || other.spuId == spuId));
}


@override
int get hashCode => Object.hash(runtimeType,name,price,unitPrice,listPrice,shareDiscount,actualPayAmount,quantity,imageKey,skuId,spuId);

@override
String toString() {
  return 'OrderItem(name: $name, price: $price, unitPrice: $unitPrice, listPrice: $listPrice, shareDiscount: $shareDiscount, actualPayAmount: $actualPayAmount, quantity: $quantity, imageKey: $imageKey, skuId: $skuId, spuId: $spuId)';
}


}

/// @nodoc
abstract mixin class $OrderItemCopyWith<$Res>  {
  factory $OrderItemCopyWith(OrderItem value, $Res Function(OrderItem) _then) = _$OrderItemCopyWithImpl;
@useResult
$Res call({
 String name, String price, String unitPrice, String listPrice, String shareDiscount, String actualPayAmount, int quantity, String imageKey, String skuId, String spuId
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
@pragma('vm:prefer-inline') @override $Res call({Object? name = null,Object? price = null,Object? unitPrice = null,Object? listPrice = null,Object? shareDiscount = null,Object? actualPayAmount = null,Object? quantity = null,Object? imageKey = null,Object? skuId = null,Object? spuId = null,}) {
  return _then(_self.copyWith(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,unitPrice: null == unitPrice ? _self.unitPrice : unitPrice // ignore: cast_nullable_to_non_nullable
as String,listPrice: null == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as String,shareDiscount: null == shareDiscount ? _self.shareDiscount : shareDiscount // ignore: cast_nullable_to_non_nullable
as String,actualPayAmount: null == actualPayAmount ? _self.actualPayAmount : actualPayAmount // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as int,imageKey: null == imageKey ? _self.imageKey : imageKey // ignore: cast_nullable_to_non_nullable
as String,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String,spuId: null == spuId ? _self.spuId : spuId // ignore: cast_nullable_to_non_nullable
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String name,  String price,  String unitPrice,  String listPrice,  String shareDiscount,  String actualPayAmount,  int quantity,  String imageKey,  String skuId,  String spuId)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
return $default(_that.name,_that.price,_that.unitPrice,_that.listPrice,_that.shareDiscount,_that.actualPayAmount,_that.quantity,_that.imageKey,_that.skuId,_that.spuId);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String name,  String price,  String unitPrice,  String listPrice,  String shareDiscount,  String actualPayAmount,  int quantity,  String imageKey,  String skuId,  String spuId)  $default,) {final _that = this;
switch (_that) {
case _OrderItem():
return $default(_that.name,_that.price,_that.unitPrice,_that.listPrice,_that.shareDiscount,_that.actualPayAmount,_that.quantity,_that.imageKey,_that.skuId,_that.spuId);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String name,  String price,  String unitPrice,  String listPrice,  String shareDiscount,  String actualPayAmount,  int quantity,  String imageKey,  String skuId,  String spuId)?  $default,) {final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
return $default(_that.name,_that.price,_that.unitPrice,_that.listPrice,_that.shareDiscount,_that.actualPayAmount,_that.quantity,_that.imageKey,_that.skuId,_that.spuId);case _:
  return null;

}
}

}

/// @nodoc


class _OrderItem extends OrderItem {
  const _OrderItem({required this.name, required this.price, this.unitPrice = '', this.listPrice = '', this.shareDiscount = '', this.actualPayAmount = '', this.quantity = 1, this.imageKey = '', this.skuId = '', this.spuId = ''}): super._();
  

@override final  String name;
@override final  String price;
// PayUni #758: 新欄位（成交單價 / 原價快照 / 折抵 / 實付）
@override@JsonKey() final  String unitPrice;
@override@JsonKey() final  String listPrice;
@override@JsonKey() final  String shareDiscount;
@override@JsonKey() final  String actualPayAmount;
@override@JsonKey() final  int quantity;
/// 商品圖 **objectKey**（49 §6.1 的 SSOT）。顯示 URL 一律交給
/// [OptimizedNetworkImage] / [buildImageUrl] 這個唯一組裝入口，模型層不存 URL。
@override@JsonKey() final  String imageKey;
@override@JsonKey() final  String skuId;
@override@JsonKey() final  String spuId;

/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderItemCopyWith<_OrderItem> get copyWith => __$OrderItemCopyWithImpl<_OrderItem>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OrderItem&&(identical(other.name, name) || other.name == name)&&(identical(other.price, price) || other.price == price)&&(identical(other.unitPrice, unitPrice) || other.unitPrice == unitPrice)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.shareDiscount, shareDiscount) || other.shareDiscount == shareDiscount)&&(identical(other.actualPayAmount, actualPayAmount) || other.actualPayAmount == actualPayAmount)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.imageKey, imageKey) || other.imageKey == imageKey)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.spuId, spuId) || other.spuId == spuId));
}


@override
int get hashCode => Object.hash(runtimeType,name,price,unitPrice,listPrice,shareDiscount,actualPayAmount,quantity,imageKey,skuId,spuId);

@override
String toString() {
  return 'OrderItem(name: $name, price: $price, unitPrice: $unitPrice, listPrice: $listPrice, shareDiscount: $shareDiscount, actualPayAmount: $actualPayAmount, quantity: $quantity, imageKey: $imageKey, skuId: $skuId, spuId: $spuId)';
}


}

/// @nodoc
abstract mixin class _$OrderItemCopyWith<$Res> implements $OrderItemCopyWith<$Res> {
  factory _$OrderItemCopyWith(_OrderItem value, $Res Function(_OrderItem) _then) = __$OrderItemCopyWithImpl;
@override @useResult
$Res call({
 String name, String price, String unitPrice, String listPrice, String shareDiscount, String actualPayAmount, int quantity, String imageKey, String skuId, String spuId
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
@override @pragma('vm:prefer-inline') $Res call({Object? name = null,Object? price = null,Object? unitPrice = null,Object? listPrice = null,Object? shareDiscount = null,Object? actualPayAmount = null,Object? quantity = null,Object? imageKey = null,Object? skuId = null,Object? spuId = null,}) {
  return _then(_OrderItem(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,unitPrice: null == unitPrice ? _self.unitPrice : unitPrice // ignore: cast_nullable_to_non_nullable
as String,listPrice: null == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as String,shareDiscount: null == shareDiscount ? _self.shareDiscount : shareDiscount // ignore: cast_nullable_to_non_nullable
as String,actualPayAmount: null == actualPayAmount ? _self.actualPayAmount : actualPayAmount // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as int,imageKey: null == imageKey ? _self.imageKey : imageKey // ignore: cast_nullable_to_non_nullable
as String,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String,spuId: null == spuId ? _self.spuId : spuId // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$OrderSummary {

 String get id; String get orderSn; int? get statusCode; String get title; String get subtitle; String get dateTime; String get productName; String get price; String get sellerName; List<OrderItem> get items; int? get buyerId; String get address; String get buyerName; num get shippingFee; num get discountAmount; num get totalAmount; String get phone; String get recipientName; String get shippingAddress; String get shippingMethod; String get deliveryTemp; String get buyerAvatar; String get pickupStoreName; String get statusSummary; String get shippingStatus; String get labelUrl; String get odno; String get validationNo; String get shipmentStatus; List<String> get shipmentPhotoUrls; List<String> get receiptPhotoUrls; String get labelExpireAt; String get shipStatusDesc; String get autoCancelAt; String get cancelReason; String get cancelRemark; String get cancelByRole;
/// Create a copy of OrderSummary
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderSummaryCopyWith<OrderSummary> get copyWith => _$OrderSummaryCopyWithImpl<OrderSummary>(this as OrderSummary, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OrderSummary&&(identical(other.id, id) || other.id == id)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.statusCode, statusCode) || other.statusCode == statusCode)&&(identical(other.title, title) || other.title == title)&&(identical(other.subtitle, subtitle) || other.subtitle == subtitle)&&(identical(other.dateTime, dateTime) || other.dateTime == dateTime)&&(identical(other.productName, productName) || other.productName == productName)&&(identical(other.price, price) || other.price == price)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&const DeepCollectionEquality().equals(other.items, items)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.address, address) || other.address == address)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.shippingFee, shippingFee) || other.shippingFee == shippingFee)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount)&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.recipientName, recipientName) || other.recipientName == recipientName)&&(identical(other.shippingAddress, shippingAddress) || other.shippingAddress == shippingAddress)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.deliveryTemp, deliveryTemp) || other.deliveryTemp == deliveryTemp)&&(identical(other.buyerAvatar, buyerAvatar) || other.buyerAvatar == buyerAvatar)&&(identical(other.pickupStoreName, pickupStoreName) || other.pickupStoreName == pickupStoreName)&&(identical(other.statusSummary, statusSummary) || other.statusSummary == statusSummary)&&(identical(other.shippingStatus, shippingStatus) || other.shippingStatus == shippingStatus)&&(identical(other.labelUrl, labelUrl) || other.labelUrl == labelUrl)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.shipmentStatus, shipmentStatus) || other.shipmentStatus == shipmentStatus)&&const DeepCollectionEquality().equals(other.shipmentPhotoUrls, shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other.receiptPhotoUrls, receiptPhotoUrls)&&(identical(other.labelExpireAt, labelExpireAt) || other.labelExpireAt == labelExpireAt)&&(identical(other.shipStatusDesc, shipStatusDesc) || other.shipStatusDesc == shipStatusDesc)&&(identical(other.autoCancelAt, autoCancelAt) || other.autoCancelAt == autoCancelAt)&&(identical(other.cancelReason, cancelReason) || other.cancelReason == cancelReason)&&(identical(other.cancelRemark, cancelRemark) || other.cancelRemark == cancelRemark)&&(identical(other.cancelByRole, cancelByRole) || other.cancelByRole == cancelByRole));
}


@override
int get hashCode => Object.hashAll([runtimeType,id,orderSn,statusCode,title,subtitle,dateTime,productName,price,sellerName,const DeepCollectionEquality().hash(items),buyerId,address,buyerName,shippingFee,discountAmount,totalAmount,phone,recipientName,shippingAddress,shippingMethod,deliveryTemp,buyerAvatar,pickupStoreName,statusSummary,shippingStatus,labelUrl,odno,validationNo,shipmentStatus,const DeepCollectionEquality().hash(shipmentPhotoUrls),const DeepCollectionEquality().hash(receiptPhotoUrls),labelExpireAt,shipStatusDesc,autoCancelAt,cancelReason,cancelRemark,cancelByRole]);

@override
String toString() {
  return 'OrderSummary(id: $id, orderSn: $orderSn, statusCode: $statusCode, title: $title, subtitle: $subtitle, dateTime: $dateTime, productName: $productName, price: $price, sellerName: $sellerName, items: $items, buyerId: $buyerId, address: $address, buyerName: $buyerName, shippingFee: $shippingFee, discountAmount: $discountAmount, totalAmount: $totalAmount, phone: $phone, recipientName: $recipientName, shippingAddress: $shippingAddress, shippingMethod: $shippingMethod, deliveryTemp: $deliveryTemp, buyerAvatar: $buyerAvatar, pickupStoreName: $pickupStoreName, statusSummary: $statusSummary, shippingStatus: $shippingStatus, labelUrl: $labelUrl, odno: $odno, validationNo: $validationNo, shipmentStatus: $shipmentStatus, shipmentPhotoUrls: $shipmentPhotoUrls, receiptPhotoUrls: $receiptPhotoUrls, labelExpireAt: $labelExpireAt, shipStatusDesc: $shipStatusDesc, autoCancelAt: $autoCancelAt, cancelReason: $cancelReason, cancelRemark: $cancelRemark, cancelByRole: $cancelByRole)';
}


}

/// @nodoc
abstract mixin class $OrderSummaryCopyWith<$Res>  {
  factory $OrderSummaryCopyWith(OrderSummary value, $Res Function(OrderSummary) _then) = _$OrderSummaryCopyWithImpl;
@useResult
$Res call({
 String id, String orderSn, int? statusCode, String title, String subtitle, String dateTime, String productName, String price, String sellerName, List<OrderItem> items, int? buyerId, String address, String buyerName, num shippingFee, num discountAmount, num totalAmount, String phone, String recipientName, String shippingAddress, String shippingMethod, String deliveryTemp, String buyerAvatar, String pickupStoreName, String statusSummary, String shippingStatus, String labelUrl, String odno, String validationNo, String shipmentStatus, List<String> shipmentPhotoUrls, List<String> receiptPhotoUrls, String labelExpireAt, String shipStatusDesc, String autoCancelAt, String cancelReason, String cancelRemark, String cancelByRole
});




}
/// @nodoc
class _$OrderSummaryCopyWithImpl<$Res>
    implements $OrderSummaryCopyWith<$Res> {
  _$OrderSummaryCopyWithImpl(this._self, this._then);

  final OrderSummary _self;
  final $Res Function(OrderSummary) _then;

/// Create a copy of OrderSummary
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? orderSn = null,Object? statusCode = freezed,Object? title = null,Object? subtitle = null,Object? dateTime = null,Object? productName = null,Object? price = null,Object? sellerName = null,Object? items = null,Object? buyerId = freezed,Object? address = null,Object? buyerName = null,Object? shippingFee = null,Object? discountAmount = null,Object? totalAmount = null,Object? phone = null,Object? recipientName = null,Object? shippingAddress = null,Object? shippingMethod = null,Object? deliveryTemp = null,Object? buyerAvatar = null,Object? pickupStoreName = null,Object? statusSummary = null,Object? shippingStatus = null,Object? labelUrl = null,Object? odno = null,Object? validationNo = null,Object? shipmentStatus = null,Object? shipmentPhotoUrls = null,Object? receiptPhotoUrls = null,Object? labelExpireAt = null,Object? shipStatusDesc = null,Object? autoCancelAt = null,Object? cancelReason = null,Object? cancelRemark = null,Object? cancelByRole = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,statusCode: freezed == statusCode ? _self.statusCode : statusCode // ignore: cast_nullable_to_non_nullable
as int?,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,subtitle: null == subtitle ? _self.subtitle : subtitle // ignore: cast_nullable_to_non_nullable
as String,dateTime: null == dateTime ? _self.dateTime : dateTime // ignore: cast_nullable_to_non_nullable
as String,productName: null == productName ? _self.productName : productName // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,sellerName: null == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<OrderItem>,buyerId: freezed == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as int?,address: null == address ? _self.address : address // ignore: cast_nullable_to_non_nullable
as String,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,shippingFee: null == shippingFee ? _self.shippingFee : shippingFee // ignore: cast_nullable_to_non_nullable
as num,discountAmount: null == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as num,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as num,phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,recipientName: null == recipientName ? _self.recipientName : recipientName // ignore: cast_nullable_to_non_nullable
as String,shippingAddress: null == shippingAddress ? _self.shippingAddress : shippingAddress // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,deliveryTemp: null == deliveryTemp ? _self.deliveryTemp : deliveryTemp // ignore: cast_nullable_to_non_nullable
as String,buyerAvatar: null == buyerAvatar ? _self.buyerAvatar : buyerAvatar // ignore: cast_nullable_to_non_nullable
as String,pickupStoreName: null == pickupStoreName ? _self.pickupStoreName : pickupStoreName // ignore: cast_nullable_to_non_nullable
as String,statusSummary: null == statusSummary ? _self.statusSummary : statusSummary // ignore: cast_nullable_to_non_nullable
as String,shippingStatus: null == shippingStatus ? _self.shippingStatus : shippingStatus // ignore: cast_nullable_to_non_nullable
as String,labelUrl: null == labelUrl ? _self.labelUrl : labelUrl // ignore: cast_nullable_to_non_nullable
as String,odno: null == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String,validationNo: null == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String,shipmentStatus: null == shipmentStatus ? _self.shipmentStatus : shipmentStatus // ignore: cast_nullable_to_non_nullable
as String,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self.shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,receiptPhotoUrls: null == receiptPhotoUrls ? _self.receiptPhotoUrls : receiptPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,labelExpireAt: null == labelExpireAt ? _self.labelExpireAt : labelExpireAt // ignore: cast_nullable_to_non_nullable
as String,shipStatusDesc: null == shipStatusDesc ? _self.shipStatusDesc : shipStatusDesc // ignore: cast_nullable_to_non_nullable
as String,autoCancelAt: null == autoCancelAt ? _self.autoCancelAt : autoCancelAt // ignore: cast_nullable_to_non_nullable
as String,cancelReason: null == cancelReason ? _self.cancelReason : cancelReason // ignore: cast_nullable_to_non_nullable
as String,cancelRemark: null == cancelRemark ? _self.cancelRemark : cancelRemark // ignore: cast_nullable_to_non_nullable
as String,cancelByRole: null == cancelByRole ? _self.cancelByRole : cancelByRole // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [OrderSummary].
extension OrderSummaryPatterns on OrderSummary {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _OrderSummary value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _OrderSummary() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _OrderSummary value)  $default,){
final _that = this;
switch (_that) {
case _OrderSummary():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _OrderSummary value)?  $default,){
final _that = this;
switch (_that) {
case _OrderSummary() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String orderSn,  int? statusCode,  String title,  String subtitle,  String dateTime,  String productName,  String price,  String sellerName,  List<OrderItem> items,  int? buyerId,  String address,  String buyerName,  num shippingFee,  num discountAmount,  num totalAmount,  String phone,  String recipientName,  String shippingAddress,  String shippingMethod,  String deliveryTemp,  String buyerAvatar,  String pickupStoreName,  String statusSummary,  String shippingStatus,  String labelUrl,  String odno,  String validationNo,  String shipmentStatus,  List<String> shipmentPhotoUrls,  List<String> receiptPhotoUrls,  String labelExpireAt,  String shipStatusDesc,  String autoCancelAt,  String cancelReason,  String cancelRemark,  String cancelByRole)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OrderSummary() when $default != null:
return $default(_that.id,_that.orderSn,_that.statusCode,_that.title,_that.subtitle,_that.dateTime,_that.productName,_that.price,_that.sellerName,_that.items,_that.buyerId,_that.address,_that.buyerName,_that.shippingFee,_that.discountAmount,_that.totalAmount,_that.phone,_that.recipientName,_that.shippingAddress,_that.shippingMethod,_that.deliveryTemp,_that.buyerAvatar,_that.pickupStoreName,_that.statusSummary,_that.shippingStatus,_that.labelUrl,_that.odno,_that.validationNo,_that.shipmentStatus,_that.shipmentPhotoUrls,_that.receiptPhotoUrls,_that.labelExpireAt,_that.shipStatusDesc,_that.autoCancelAt,_that.cancelReason,_that.cancelRemark,_that.cancelByRole);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String orderSn,  int? statusCode,  String title,  String subtitle,  String dateTime,  String productName,  String price,  String sellerName,  List<OrderItem> items,  int? buyerId,  String address,  String buyerName,  num shippingFee,  num discountAmount,  num totalAmount,  String phone,  String recipientName,  String shippingAddress,  String shippingMethod,  String deliveryTemp,  String buyerAvatar,  String pickupStoreName,  String statusSummary,  String shippingStatus,  String labelUrl,  String odno,  String validationNo,  String shipmentStatus,  List<String> shipmentPhotoUrls,  List<String> receiptPhotoUrls,  String labelExpireAt,  String shipStatusDesc,  String autoCancelAt,  String cancelReason,  String cancelRemark,  String cancelByRole)  $default,) {final _that = this;
switch (_that) {
case _OrderSummary():
return $default(_that.id,_that.orderSn,_that.statusCode,_that.title,_that.subtitle,_that.dateTime,_that.productName,_that.price,_that.sellerName,_that.items,_that.buyerId,_that.address,_that.buyerName,_that.shippingFee,_that.discountAmount,_that.totalAmount,_that.phone,_that.recipientName,_that.shippingAddress,_that.shippingMethod,_that.deliveryTemp,_that.buyerAvatar,_that.pickupStoreName,_that.statusSummary,_that.shippingStatus,_that.labelUrl,_that.odno,_that.validationNo,_that.shipmentStatus,_that.shipmentPhotoUrls,_that.receiptPhotoUrls,_that.labelExpireAt,_that.shipStatusDesc,_that.autoCancelAt,_that.cancelReason,_that.cancelRemark,_that.cancelByRole);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String orderSn,  int? statusCode,  String title,  String subtitle,  String dateTime,  String productName,  String price,  String sellerName,  List<OrderItem> items,  int? buyerId,  String address,  String buyerName,  num shippingFee,  num discountAmount,  num totalAmount,  String phone,  String recipientName,  String shippingAddress,  String shippingMethod,  String deliveryTemp,  String buyerAvatar,  String pickupStoreName,  String statusSummary,  String shippingStatus,  String labelUrl,  String odno,  String validationNo,  String shipmentStatus,  List<String> shipmentPhotoUrls,  List<String> receiptPhotoUrls,  String labelExpireAt,  String shipStatusDesc,  String autoCancelAt,  String cancelReason,  String cancelRemark,  String cancelByRole)?  $default,) {final _that = this;
switch (_that) {
case _OrderSummary() when $default != null:
return $default(_that.id,_that.orderSn,_that.statusCode,_that.title,_that.subtitle,_that.dateTime,_that.productName,_that.price,_that.sellerName,_that.items,_that.buyerId,_that.address,_that.buyerName,_that.shippingFee,_that.discountAmount,_that.totalAmount,_that.phone,_that.recipientName,_that.shippingAddress,_that.shippingMethod,_that.deliveryTemp,_that.buyerAvatar,_that.pickupStoreName,_that.statusSummary,_that.shippingStatus,_that.labelUrl,_that.odno,_that.validationNo,_that.shipmentStatus,_that.shipmentPhotoUrls,_that.receiptPhotoUrls,_that.labelExpireAt,_that.shipStatusDesc,_that.autoCancelAt,_that.cancelReason,_that.cancelRemark,_that.cancelByRole);case _:
  return null;

}
}

}

/// @nodoc


class _OrderSummary extends OrderSummary {
  const _OrderSummary({required this.id, this.orderSn = '', this.statusCode, required this.title, required this.subtitle, required this.dateTime, required this.productName, required this.price, this.sellerName = '', final  List<OrderItem> items = const [], this.buyerId, this.address = '', this.buyerName = '', this.shippingFee = 0, this.discountAmount = 0, this.totalAmount = 0, this.phone = '', this.recipientName = '', this.shippingAddress = '', this.shippingMethod = '', this.deliveryTemp = '', this.buyerAvatar = '', this.pickupStoreName = '', this.statusSummary = '', this.shippingStatus = '', this.labelUrl = '', this.odno = '', this.validationNo = '', this.shipmentStatus = '', final  List<String> shipmentPhotoUrls = const [], final  List<String> receiptPhotoUrls = const [], this.labelExpireAt = '', this.shipStatusDesc = '', this.autoCancelAt = '', this.cancelReason = '', this.cancelRemark = '', this.cancelByRole = ''}): _items = items,_shipmentPhotoUrls = shipmentPhotoUrls,_receiptPhotoUrls = receiptPhotoUrls,super._();
  

@override final  String id;
@override@JsonKey() final  String orderSn;
@override final  int? statusCode;
@override final  String title;
@override final  String subtitle;
@override final  String dateTime;
@override final  String productName;
@override final  String price;
@override@JsonKey() final  String sellerName;
 final  List<OrderItem> _items;
@override@JsonKey() List<OrderItem> get items {
  if (_items is EqualUnmodifiableListView) return _items;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_items);
}

@override final  int? buyerId;
@override@JsonKey() final  String address;
@override@JsonKey() final  String buyerName;
@override@JsonKey() final  num shippingFee;
@override@JsonKey() final  num discountAmount;
@override@JsonKey() final  num totalAmount;
@override@JsonKey() final  String phone;
@override@JsonKey() final  String recipientName;
@override@JsonKey() final  String shippingAddress;
@override@JsonKey() final  String shippingMethod;
@override@JsonKey() final  String deliveryTemp;
@override@JsonKey() final  String buyerAvatar;
@override@JsonKey() final  String pickupStoreName;
@override@JsonKey() final  String statusSummary;
@override@JsonKey() final  String shippingStatus;
@override@JsonKey() final  String labelUrl;
@override@JsonKey() final  String odno;
@override@JsonKey() final  String validationNo;
@override@JsonKey() final  String shipmentStatus;
 final  List<String> _shipmentPhotoUrls;
@override@JsonKey() List<String> get shipmentPhotoUrls {
  if (_shipmentPhotoUrls is EqualUnmodifiableListView) return _shipmentPhotoUrls;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_shipmentPhotoUrls);
}

 final  List<String> _receiptPhotoUrls;
@override@JsonKey() List<String> get receiptPhotoUrls {
  if (_receiptPhotoUrls is EqualUnmodifiableListView) return _receiptPhotoUrls;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_receiptPhotoUrls);
}

@override@JsonKey() final  String labelExpireAt;
@override@JsonKey() final  String shipStatusDesc;
@override@JsonKey() final  String autoCancelAt;
@override@JsonKey() final  String cancelReason;
@override@JsonKey() final  String cancelRemark;
@override@JsonKey() final  String cancelByRole;

/// Create a copy of OrderSummary
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderSummaryCopyWith<_OrderSummary> get copyWith => __$OrderSummaryCopyWithImpl<_OrderSummary>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OrderSummary&&(identical(other.id, id) || other.id == id)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.statusCode, statusCode) || other.statusCode == statusCode)&&(identical(other.title, title) || other.title == title)&&(identical(other.subtitle, subtitle) || other.subtitle == subtitle)&&(identical(other.dateTime, dateTime) || other.dateTime == dateTime)&&(identical(other.productName, productName) || other.productName == productName)&&(identical(other.price, price) || other.price == price)&&(identical(other.sellerName, sellerName) || other.sellerName == sellerName)&&const DeepCollectionEquality().equals(other._items, _items)&&(identical(other.buyerId, buyerId) || other.buyerId == buyerId)&&(identical(other.address, address) || other.address == address)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.shippingFee, shippingFee) || other.shippingFee == shippingFee)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount)&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.recipientName, recipientName) || other.recipientName == recipientName)&&(identical(other.shippingAddress, shippingAddress) || other.shippingAddress == shippingAddress)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.deliveryTemp, deliveryTemp) || other.deliveryTemp == deliveryTemp)&&(identical(other.buyerAvatar, buyerAvatar) || other.buyerAvatar == buyerAvatar)&&(identical(other.pickupStoreName, pickupStoreName) || other.pickupStoreName == pickupStoreName)&&(identical(other.statusSummary, statusSummary) || other.statusSummary == statusSummary)&&(identical(other.shippingStatus, shippingStatus) || other.shippingStatus == shippingStatus)&&(identical(other.labelUrl, labelUrl) || other.labelUrl == labelUrl)&&(identical(other.odno, odno) || other.odno == odno)&&(identical(other.validationNo, validationNo) || other.validationNo == validationNo)&&(identical(other.shipmentStatus, shipmentStatus) || other.shipmentStatus == shipmentStatus)&&const DeepCollectionEquality().equals(other._shipmentPhotoUrls, _shipmentPhotoUrls)&&const DeepCollectionEquality().equals(other._receiptPhotoUrls, _receiptPhotoUrls)&&(identical(other.labelExpireAt, labelExpireAt) || other.labelExpireAt == labelExpireAt)&&(identical(other.shipStatusDesc, shipStatusDesc) || other.shipStatusDesc == shipStatusDesc)&&(identical(other.autoCancelAt, autoCancelAt) || other.autoCancelAt == autoCancelAt)&&(identical(other.cancelReason, cancelReason) || other.cancelReason == cancelReason)&&(identical(other.cancelRemark, cancelRemark) || other.cancelRemark == cancelRemark)&&(identical(other.cancelByRole, cancelByRole) || other.cancelByRole == cancelByRole));
}


@override
int get hashCode => Object.hashAll([runtimeType,id,orderSn,statusCode,title,subtitle,dateTime,productName,price,sellerName,const DeepCollectionEquality().hash(_items),buyerId,address,buyerName,shippingFee,discountAmount,totalAmount,phone,recipientName,shippingAddress,shippingMethod,deliveryTemp,buyerAvatar,pickupStoreName,statusSummary,shippingStatus,labelUrl,odno,validationNo,shipmentStatus,const DeepCollectionEquality().hash(_shipmentPhotoUrls),const DeepCollectionEquality().hash(_receiptPhotoUrls),labelExpireAt,shipStatusDesc,autoCancelAt,cancelReason,cancelRemark,cancelByRole]);

@override
String toString() {
  return 'OrderSummary(id: $id, orderSn: $orderSn, statusCode: $statusCode, title: $title, subtitle: $subtitle, dateTime: $dateTime, productName: $productName, price: $price, sellerName: $sellerName, items: $items, buyerId: $buyerId, address: $address, buyerName: $buyerName, shippingFee: $shippingFee, discountAmount: $discountAmount, totalAmount: $totalAmount, phone: $phone, recipientName: $recipientName, shippingAddress: $shippingAddress, shippingMethod: $shippingMethod, deliveryTemp: $deliveryTemp, buyerAvatar: $buyerAvatar, pickupStoreName: $pickupStoreName, statusSummary: $statusSummary, shippingStatus: $shippingStatus, labelUrl: $labelUrl, odno: $odno, validationNo: $validationNo, shipmentStatus: $shipmentStatus, shipmentPhotoUrls: $shipmentPhotoUrls, receiptPhotoUrls: $receiptPhotoUrls, labelExpireAt: $labelExpireAt, shipStatusDesc: $shipStatusDesc, autoCancelAt: $autoCancelAt, cancelReason: $cancelReason, cancelRemark: $cancelRemark, cancelByRole: $cancelByRole)';
}


}

/// @nodoc
abstract mixin class _$OrderSummaryCopyWith<$Res> implements $OrderSummaryCopyWith<$Res> {
  factory _$OrderSummaryCopyWith(_OrderSummary value, $Res Function(_OrderSummary) _then) = __$OrderSummaryCopyWithImpl;
@override @useResult
$Res call({
 String id, String orderSn, int? statusCode, String title, String subtitle, String dateTime, String productName, String price, String sellerName, List<OrderItem> items, int? buyerId, String address, String buyerName, num shippingFee, num discountAmount, num totalAmount, String phone, String recipientName, String shippingAddress, String shippingMethod, String deliveryTemp, String buyerAvatar, String pickupStoreName, String statusSummary, String shippingStatus, String labelUrl, String odno, String validationNo, String shipmentStatus, List<String> shipmentPhotoUrls, List<String> receiptPhotoUrls, String labelExpireAt, String shipStatusDesc, String autoCancelAt, String cancelReason, String cancelRemark, String cancelByRole
});




}
/// @nodoc
class __$OrderSummaryCopyWithImpl<$Res>
    implements _$OrderSummaryCopyWith<$Res> {
  __$OrderSummaryCopyWithImpl(this._self, this._then);

  final _OrderSummary _self;
  final $Res Function(_OrderSummary) _then;

/// Create a copy of OrderSummary
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? orderSn = null,Object? statusCode = freezed,Object? title = null,Object? subtitle = null,Object? dateTime = null,Object? productName = null,Object? price = null,Object? sellerName = null,Object? items = null,Object? buyerId = freezed,Object? address = null,Object? buyerName = null,Object? shippingFee = null,Object? discountAmount = null,Object? totalAmount = null,Object? phone = null,Object? recipientName = null,Object? shippingAddress = null,Object? shippingMethod = null,Object? deliveryTemp = null,Object? buyerAvatar = null,Object? pickupStoreName = null,Object? statusSummary = null,Object? shippingStatus = null,Object? labelUrl = null,Object? odno = null,Object? validationNo = null,Object? shipmentStatus = null,Object? shipmentPhotoUrls = null,Object? receiptPhotoUrls = null,Object? labelExpireAt = null,Object? shipStatusDesc = null,Object? autoCancelAt = null,Object? cancelReason = null,Object? cancelRemark = null,Object? cancelByRole = null,}) {
  return _then(_OrderSummary(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,statusCode: freezed == statusCode ? _self.statusCode : statusCode // ignore: cast_nullable_to_non_nullable
as int?,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,subtitle: null == subtitle ? _self.subtitle : subtitle // ignore: cast_nullable_to_non_nullable
as String,dateTime: null == dateTime ? _self.dateTime : dateTime // ignore: cast_nullable_to_non_nullable
as String,productName: null == productName ? _self.productName : productName // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,sellerName: null == sellerName ? _self.sellerName : sellerName // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<OrderItem>,buyerId: freezed == buyerId ? _self.buyerId : buyerId // ignore: cast_nullable_to_non_nullable
as int?,address: null == address ? _self.address : address // ignore: cast_nullable_to_non_nullable
as String,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,shippingFee: null == shippingFee ? _self.shippingFee : shippingFee // ignore: cast_nullable_to_non_nullable
as num,discountAmount: null == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as num,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as num,phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,recipientName: null == recipientName ? _self.recipientName : recipientName // ignore: cast_nullable_to_non_nullable
as String,shippingAddress: null == shippingAddress ? _self.shippingAddress : shippingAddress // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,deliveryTemp: null == deliveryTemp ? _self.deliveryTemp : deliveryTemp // ignore: cast_nullable_to_non_nullable
as String,buyerAvatar: null == buyerAvatar ? _self.buyerAvatar : buyerAvatar // ignore: cast_nullable_to_non_nullable
as String,pickupStoreName: null == pickupStoreName ? _self.pickupStoreName : pickupStoreName // ignore: cast_nullable_to_non_nullable
as String,statusSummary: null == statusSummary ? _self.statusSummary : statusSummary // ignore: cast_nullable_to_non_nullable
as String,shippingStatus: null == shippingStatus ? _self.shippingStatus : shippingStatus // ignore: cast_nullable_to_non_nullable
as String,labelUrl: null == labelUrl ? _self.labelUrl : labelUrl // ignore: cast_nullable_to_non_nullable
as String,odno: null == odno ? _self.odno : odno // ignore: cast_nullable_to_non_nullable
as String,validationNo: null == validationNo ? _self.validationNo : validationNo // ignore: cast_nullable_to_non_nullable
as String,shipmentStatus: null == shipmentStatus ? _self.shipmentStatus : shipmentStatus // ignore: cast_nullable_to_non_nullable
as String,shipmentPhotoUrls: null == shipmentPhotoUrls ? _self._shipmentPhotoUrls : shipmentPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,receiptPhotoUrls: null == receiptPhotoUrls ? _self._receiptPhotoUrls : receiptPhotoUrls // ignore: cast_nullable_to_non_nullable
as List<String>,labelExpireAt: null == labelExpireAt ? _self.labelExpireAt : labelExpireAt // ignore: cast_nullable_to_non_nullable
as String,shipStatusDesc: null == shipStatusDesc ? _self.shipStatusDesc : shipStatusDesc // ignore: cast_nullable_to_non_nullable
as String,autoCancelAt: null == autoCancelAt ? _self.autoCancelAt : autoCancelAt // ignore: cast_nullable_to_non_nullable
as String,cancelReason: null == cancelReason ? _self.cancelReason : cancelReason // ignore: cast_nullable_to_non_nullable
as String,cancelRemark: null == cancelRemark ? _self.cancelRemark : cancelRemark // ignore: cast_nullable_to_non_nullable
as String,cancelByRole: null == cancelByRole ? _self.cancelByRole : cancelByRole // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
