// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'order_summary.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$OrderSummary {

 String get id; String get orderSn; String get statusCode; String get title; String get subtitle; String get dateTime; String get productName; String get price; String? get imageUrl; String? get mainSkuId; List<OrderItem> get items; String get merchantId; String get merchantName; String get merchantAvatar; String get receiverName; String get receiverPhone; String get receiverAddress; double get shippingFee; double get discount; double get totalAmount; String get paymentMethod; String get paymentTime; String get paymentStatus; String get atmBankCode; String get atmVirtualAccount; String get atmExpireDate; String get totalActualAmount; String get shippingMethod; String get deliveryTemp; String get pickupStoreName; String get shippingStatus; String get statusTime; String get statusSummary;
/// Create a copy of OrderSummary
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderSummaryCopyWith<OrderSummary> get copyWith => _$OrderSummaryCopyWithImpl<OrderSummary>(this as OrderSummary, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OrderSummary&&(identical(other.id, id) || other.id == id)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.statusCode, statusCode) || other.statusCode == statusCode)&&(identical(other.title, title) || other.title == title)&&(identical(other.subtitle, subtitle) || other.subtitle == subtitle)&&(identical(other.dateTime, dateTime) || other.dateTime == dateTime)&&(identical(other.productName, productName) || other.productName == productName)&&(identical(other.price, price) || other.price == price)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.mainSkuId, mainSkuId) || other.mainSkuId == mainSkuId)&&const DeepCollectionEquality().equals(other.items, items)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&(identical(other.receiverName, receiverName) || other.receiverName == receiverName)&&(identical(other.receiverPhone, receiverPhone) || other.receiverPhone == receiverPhone)&&(identical(other.receiverAddress, receiverAddress) || other.receiverAddress == receiverAddress)&&(identical(other.shippingFee, shippingFee) || other.shippingFee == shippingFee)&&(identical(other.discount, discount) || other.discount == discount)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount)&&(identical(other.paymentMethod, paymentMethod) || other.paymentMethod == paymentMethod)&&(identical(other.paymentTime, paymentTime) || other.paymentTime == paymentTime)&&(identical(other.paymentStatus, paymentStatus) || other.paymentStatus == paymentStatus)&&(identical(other.atmBankCode, atmBankCode) || other.atmBankCode == atmBankCode)&&(identical(other.atmVirtualAccount, atmVirtualAccount) || other.atmVirtualAccount == atmVirtualAccount)&&(identical(other.atmExpireDate, atmExpireDate) || other.atmExpireDate == atmExpireDate)&&(identical(other.totalActualAmount, totalActualAmount) || other.totalActualAmount == totalActualAmount)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.deliveryTemp, deliveryTemp) || other.deliveryTemp == deliveryTemp)&&(identical(other.pickupStoreName, pickupStoreName) || other.pickupStoreName == pickupStoreName)&&(identical(other.shippingStatus, shippingStatus) || other.shippingStatus == shippingStatus)&&(identical(other.statusTime, statusTime) || other.statusTime == statusTime)&&(identical(other.statusSummary, statusSummary) || other.statusSummary == statusSummary));
}


@override
int get hashCode => Object.hashAll([runtimeType,id,orderSn,statusCode,title,subtitle,dateTime,productName,price,imageUrl,mainSkuId,const DeepCollectionEquality().hash(items),merchantId,merchantName,merchantAvatar,receiverName,receiverPhone,receiverAddress,shippingFee,discount,totalAmount,paymentMethod,paymentTime,paymentStatus,atmBankCode,atmVirtualAccount,atmExpireDate,totalActualAmount,shippingMethod,deliveryTemp,pickupStoreName,shippingStatus,statusTime,statusSummary]);

@override
String toString() {
  return 'OrderSummary(id: $id, orderSn: $orderSn, statusCode: $statusCode, title: $title, subtitle: $subtitle, dateTime: $dateTime, productName: $productName, price: $price, imageUrl: $imageUrl, mainSkuId: $mainSkuId, items: $items, merchantId: $merchantId, merchantName: $merchantName, merchantAvatar: $merchantAvatar, receiverName: $receiverName, receiverPhone: $receiverPhone, receiverAddress: $receiverAddress, shippingFee: $shippingFee, discount: $discount, totalAmount: $totalAmount, paymentMethod: $paymentMethod, paymentTime: $paymentTime, paymentStatus: $paymentStatus, atmBankCode: $atmBankCode, atmVirtualAccount: $atmVirtualAccount, atmExpireDate: $atmExpireDate, totalActualAmount: $totalActualAmount, shippingMethod: $shippingMethod, deliveryTemp: $deliveryTemp, pickupStoreName: $pickupStoreName, shippingStatus: $shippingStatus, statusTime: $statusTime, statusSummary: $statusSummary)';
}


}

/// @nodoc
abstract mixin class $OrderSummaryCopyWith<$Res>  {
  factory $OrderSummaryCopyWith(OrderSummary value, $Res Function(OrderSummary) _then) = _$OrderSummaryCopyWithImpl;
@useResult
$Res call({
 String id, String orderSn, String statusCode, String title, String subtitle, String dateTime, String productName, String price, String? imageUrl, String? mainSkuId, List<OrderItem> items, String merchantId, String merchantName, String merchantAvatar, String receiverName, String receiverPhone, String receiverAddress, double shippingFee, double discount, double totalAmount, String paymentMethod, String paymentTime, String paymentStatus, String atmBankCode, String atmVirtualAccount, String atmExpireDate, String totalActualAmount, String shippingMethod, String deliveryTemp, String pickupStoreName, String shippingStatus, String statusTime, String statusSummary
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
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? orderSn = null,Object? statusCode = null,Object? title = null,Object? subtitle = null,Object? dateTime = null,Object? productName = null,Object? price = null,Object? imageUrl = freezed,Object? mainSkuId = freezed,Object? items = null,Object? merchantId = null,Object? merchantName = null,Object? merchantAvatar = null,Object? receiverName = null,Object? receiverPhone = null,Object? receiverAddress = null,Object? shippingFee = null,Object? discount = null,Object? totalAmount = null,Object? paymentMethod = null,Object? paymentTime = null,Object? paymentStatus = null,Object? atmBankCode = null,Object? atmVirtualAccount = null,Object? atmExpireDate = null,Object? totalActualAmount = null,Object? shippingMethod = null,Object? deliveryTemp = null,Object? pickupStoreName = null,Object? shippingStatus = null,Object? statusTime = null,Object? statusSummary = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,statusCode: null == statusCode ? _self.statusCode : statusCode // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,subtitle: null == subtitle ? _self.subtitle : subtitle // ignore: cast_nullable_to_non_nullable
as String,dateTime: null == dateTime ? _self.dateTime : dateTime // ignore: cast_nullable_to_non_nullable
as String,productName: null == productName ? _self.productName : productName // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,mainSkuId: freezed == mainSkuId ? _self.mainSkuId : mainSkuId // ignore: cast_nullable_to_non_nullable
as String?,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<OrderItem>,merchantId: null == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as String,merchantName: null == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String,merchantAvatar: null == merchantAvatar ? _self.merchantAvatar : merchantAvatar // ignore: cast_nullable_to_non_nullable
as String,receiverName: null == receiverName ? _self.receiverName : receiverName // ignore: cast_nullable_to_non_nullable
as String,receiverPhone: null == receiverPhone ? _self.receiverPhone : receiverPhone // ignore: cast_nullable_to_non_nullable
as String,receiverAddress: null == receiverAddress ? _self.receiverAddress : receiverAddress // ignore: cast_nullable_to_non_nullable
as String,shippingFee: null == shippingFee ? _self.shippingFee : shippingFee // ignore: cast_nullable_to_non_nullable
as double,discount: null == discount ? _self.discount : discount // ignore: cast_nullable_to_non_nullable
as double,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as double,paymentMethod: null == paymentMethod ? _self.paymentMethod : paymentMethod // ignore: cast_nullable_to_non_nullable
as String,paymentTime: null == paymentTime ? _self.paymentTime : paymentTime // ignore: cast_nullable_to_non_nullable
as String,paymentStatus: null == paymentStatus ? _self.paymentStatus : paymentStatus // ignore: cast_nullable_to_non_nullable
as String,atmBankCode: null == atmBankCode ? _self.atmBankCode : atmBankCode // ignore: cast_nullable_to_non_nullable
as String,atmVirtualAccount: null == atmVirtualAccount ? _self.atmVirtualAccount : atmVirtualAccount // ignore: cast_nullable_to_non_nullable
as String,atmExpireDate: null == atmExpireDate ? _self.atmExpireDate : atmExpireDate // ignore: cast_nullable_to_non_nullable
as String,totalActualAmount: null == totalActualAmount ? _self.totalActualAmount : totalActualAmount // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,deliveryTemp: null == deliveryTemp ? _self.deliveryTemp : deliveryTemp // ignore: cast_nullable_to_non_nullable
as String,pickupStoreName: null == pickupStoreName ? _self.pickupStoreName : pickupStoreName // ignore: cast_nullable_to_non_nullable
as String,shippingStatus: null == shippingStatus ? _self.shippingStatus : shippingStatus // ignore: cast_nullable_to_non_nullable
as String,statusTime: null == statusTime ? _self.statusTime : statusTime // ignore: cast_nullable_to_non_nullable
as String,statusSummary: null == statusSummary ? _self.statusSummary : statusSummary // ignore: cast_nullable_to_non_nullable
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String orderSn,  String statusCode,  String title,  String subtitle,  String dateTime,  String productName,  String price,  String? imageUrl,  String? mainSkuId,  List<OrderItem> items,  String merchantId,  String merchantName,  String merchantAvatar,  String receiverName,  String receiverPhone,  String receiverAddress,  double shippingFee,  double discount,  double totalAmount,  String paymentMethod,  String paymentTime,  String paymentStatus,  String atmBankCode,  String atmVirtualAccount,  String atmExpireDate,  String totalActualAmount,  String shippingMethod,  String deliveryTemp,  String pickupStoreName,  String shippingStatus,  String statusTime,  String statusSummary)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OrderSummary() when $default != null:
return $default(_that.id,_that.orderSn,_that.statusCode,_that.title,_that.subtitle,_that.dateTime,_that.productName,_that.price,_that.imageUrl,_that.mainSkuId,_that.items,_that.merchantId,_that.merchantName,_that.merchantAvatar,_that.receiverName,_that.receiverPhone,_that.receiverAddress,_that.shippingFee,_that.discount,_that.totalAmount,_that.paymentMethod,_that.paymentTime,_that.paymentStatus,_that.atmBankCode,_that.atmVirtualAccount,_that.atmExpireDate,_that.totalActualAmount,_that.shippingMethod,_that.deliveryTemp,_that.pickupStoreName,_that.shippingStatus,_that.statusTime,_that.statusSummary);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String orderSn,  String statusCode,  String title,  String subtitle,  String dateTime,  String productName,  String price,  String? imageUrl,  String? mainSkuId,  List<OrderItem> items,  String merchantId,  String merchantName,  String merchantAvatar,  String receiverName,  String receiverPhone,  String receiverAddress,  double shippingFee,  double discount,  double totalAmount,  String paymentMethod,  String paymentTime,  String paymentStatus,  String atmBankCode,  String atmVirtualAccount,  String atmExpireDate,  String totalActualAmount,  String shippingMethod,  String deliveryTemp,  String pickupStoreName,  String shippingStatus,  String statusTime,  String statusSummary)  $default,) {final _that = this;
switch (_that) {
case _OrderSummary():
return $default(_that.id,_that.orderSn,_that.statusCode,_that.title,_that.subtitle,_that.dateTime,_that.productName,_that.price,_that.imageUrl,_that.mainSkuId,_that.items,_that.merchantId,_that.merchantName,_that.merchantAvatar,_that.receiverName,_that.receiverPhone,_that.receiverAddress,_that.shippingFee,_that.discount,_that.totalAmount,_that.paymentMethod,_that.paymentTime,_that.paymentStatus,_that.atmBankCode,_that.atmVirtualAccount,_that.atmExpireDate,_that.totalActualAmount,_that.shippingMethod,_that.deliveryTemp,_that.pickupStoreName,_that.shippingStatus,_that.statusTime,_that.statusSummary);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String orderSn,  String statusCode,  String title,  String subtitle,  String dateTime,  String productName,  String price,  String? imageUrl,  String? mainSkuId,  List<OrderItem> items,  String merchantId,  String merchantName,  String merchantAvatar,  String receiverName,  String receiverPhone,  String receiverAddress,  double shippingFee,  double discount,  double totalAmount,  String paymentMethod,  String paymentTime,  String paymentStatus,  String atmBankCode,  String atmVirtualAccount,  String atmExpireDate,  String totalActualAmount,  String shippingMethod,  String deliveryTemp,  String pickupStoreName,  String shippingStatus,  String statusTime,  String statusSummary)?  $default,) {final _that = this;
switch (_that) {
case _OrderSummary() when $default != null:
return $default(_that.id,_that.orderSn,_that.statusCode,_that.title,_that.subtitle,_that.dateTime,_that.productName,_that.price,_that.imageUrl,_that.mainSkuId,_that.items,_that.merchantId,_that.merchantName,_that.merchantAvatar,_that.receiverName,_that.receiverPhone,_that.receiverAddress,_that.shippingFee,_that.discount,_that.totalAmount,_that.paymentMethod,_that.paymentTime,_that.paymentStatus,_that.atmBankCode,_that.atmVirtualAccount,_that.atmExpireDate,_that.totalActualAmount,_that.shippingMethod,_that.deliveryTemp,_that.pickupStoreName,_that.shippingStatus,_that.statusTime,_that.statusSummary);case _:
  return null;

}
}

}

/// @nodoc


class _OrderSummary implements OrderSummary {
  const _OrderSummary({required this.id, this.orderSn = '', this.statusCode = '', this.title = '', this.subtitle = '', this.dateTime = '', this.productName = '', this.price = '', this.imageUrl, this.mainSkuId, final  List<OrderItem> items = const [], this.merchantId = '', this.merchantName = '', this.merchantAvatar = '', this.receiverName = '', this.receiverPhone = '', this.receiverAddress = '', this.shippingFee = 0.0, this.discount = 0.0, this.totalAmount = 0.0, this.paymentMethod = '', this.paymentTime = '', this.paymentStatus = '', this.atmBankCode = '', this.atmVirtualAccount = '', this.atmExpireDate = '', this.totalActualAmount = '', this.shippingMethod = '', this.deliveryTemp = '', this.pickupStoreName = '', this.shippingStatus = '', this.statusTime = '', this.statusSummary = ''}): _items = items;
  

@override final  String id;
@override@JsonKey() final  String orderSn;
@override@JsonKey() final  String statusCode;
@override@JsonKey() final  String title;
@override@JsonKey() final  String subtitle;
@override@JsonKey() final  String dateTime;
@override@JsonKey() final  String productName;
@override@JsonKey() final  String price;
@override final  String? imageUrl;
@override final  String? mainSkuId;
 final  List<OrderItem> _items;
@override@JsonKey() List<OrderItem> get items {
  if (_items is EqualUnmodifiableListView) return _items;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_items);
}

@override@JsonKey() final  String merchantId;
@override@JsonKey() final  String merchantName;
@override@JsonKey() final  String merchantAvatar;
@override@JsonKey() final  String receiverName;
@override@JsonKey() final  String receiverPhone;
@override@JsonKey() final  String receiverAddress;
@override@JsonKey() final  double shippingFee;
@override@JsonKey() final  double discount;
@override@JsonKey() final  double totalAmount;
@override@JsonKey() final  String paymentMethod;
@override@JsonKey() final  String paymentTime;
@override@JsonKey() final  String paymentStatus;
@override@JsonKey() final  String atmBankCode;
@override@JsonKey() final  String atmVirtualAccount;
@override@JsonKey() final  String atmExpireDate;
@override@JsonKey() final  String totalActualAmount;
@override@JsonKey() final  String shippingMethod;
@override@JsonKey() final  String deliveryTemp;
@override@JsonKey() final  String pickupStoreName;
@override@JsonKey() final  String shippingStatus;
@override@JsonKey() final  String statusTime;
@override@JsonKey() final  String statusSummary;

/// Create a copy of OrderSummary
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderSummaryCopyWith<_OrderSummary> get copyWith => __$OrderSummaryCopyWithImpl<_OrderSummary>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OrderSummary&&(identical(other.id, id) || other.id == id)&&(identical(other.orderSn, orderSn) || other.orderSn == orderSn)&&(identical(other.statusCode, statusCode) || other.statusCode == statusCode)&&(identical(other.title, title) || other.title == title)&&(identical(other.subtitle, subtitle) || other.subtitle == subtitle)&&(identical(other.dateTime, dateTime) || other.dateTime == dateTime)&&(identical(other.productName, productName) || other.productName == productName)&&(identical(other.price, price) || other.price == price)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.mainSkuId, mainSkuId) || other.mainSkuId == mainSkuId)&&const DeepCollectionEquality().equals(other._items, _items)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.merchantAvatar, merchantAvatar) || other.merchantAvatar == merchantAvatar)&&(identical(other.receiverName, receiverName) || other.receiverName == receiverName)&&(identical(other.receiverPhone, receiverPhone) || other.receiverPhone == receiverPhone)&&(identical(other.receiverAddress, receiverAddress) || other.receiverAddress == receiverAddress)&&(identical(other.shippingFee, shippingFee) || other.shippingFee == shippingFee)&&(identical(other.discount, discount) || other.discount == discount)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount)&&(identical(other.paymentMethod, paymentMethod) || other.paymentMethod == paymentMethod)&&(identical(other.paymentTime, paymentTime) || other.paymentTime == paymentTime)&&(identical(other.paymentStatus, paymentStatus) || other.paymentStatus == paymentStatus)&&(identical(other.atmBankCode, atmBankCode) || other.atmBankCode == atmBankCode)&&(identical(other.atmVirtualAccount, atmVirtualAccount) || other.atmVirtualAccount == atmVirtualAccount)&&(identical(other.atmExpireDate, atmExpireDate) || other.atmExpireDate == atmExpireDate)&&(identical(other.totalActualAmount, totalActualAmount) || other.totalActualAmount == totalActualAmount)&&(identical(other.shippingMethod, shippingMethod) || other.shippingMethod == shippingMethod)&&(identical(other.deliveryTemp, deliveryTemp) || other.deliveryTemp == deliveryTemp)&&(identical(other.pickupStoreName, pickupStoreName) || other.pickupStoreName == pickupStoreName)&&(identical(other.shippingStatus, shippingStatus) || other.shippingStatus == shippingStatus)&&(identical(other.statusTime, statusTime) || other.statusTime == statusTime)&&(identical(other.statusSummary, statusSummary) || other.statusSummary == statusSummary));
}


@override
int get hashCode => Object.hashAll([runtimeType,id,orderSn,statusCode,title,subtitle,dateTime,productName,price,imageUrl,mainSkuId,const DeepCollectionEquality().hash(_items),merchantId,merchantName,merchantAvatar,receiverName,receiverPhone,receiverAddress,shippingFee,discount,totalAmount,paymentMethod,paymentTime,paymentStatus,atmBankCode,atmVirtualAccount,atmExpireDate,totalActualAmount,shippingMethod,deliveryTemp,pickupStoreName,shippingStatus,statusTime,statusSummary]);

@override
String toString() {
  return 'OrderSummary(id: $id, orderSn: $orderSn, statusCode: $statusCode, title: $title, subtitle: $subtitle, dateTime: $dateTime, productName: $productName, price: $price, imageUrl: $imageUrl, mainSkuId: $mainSkuId, items: $items, merchantId: $merchantId, merchantName: $merchantName, merchantAvatar: $merchantAvatar, receiverName: $receiverName, receiverPhone: $receiverPhone, receiverAddress: $receiverAddress, shippingFee: $shippingFee, discount: $discount, totalAmount: $totalAmount, paymentMethod: $paymentMethod, paymentTime: $paymentTime, paymentStatus: $paymentStatus, atmBankCode: $atmBankCode, atmVirtualAccount: $atmVirtualAccount, atmExpireDate: $atmExpireDate, totalActualAmount: $totalActualAmount, shippingMethod: $shippingMethod, deliveryTemp: $deliveryTemp, pickupStoreName: $pickupStoreName, shippingStatus: $shippingStatus, statusTime: $statusTime, statusSummary: $statusSummary)';
}


}

/// @nodoc
abstract mixin class _$OrderSummaryCopyWith<$Res> implements $OrderSummaryCopyWith<$Res> {
  factory _$OrderSummaryCopyWith(_OrderSummary value, $Res Function(_OrderSummary) _then) = __$OrderSummaryCopyWithImpl;
@override @useResult
$Res call({
 String id, String orderSn, String statusCode, String title, String subtitle, String dateTime, String productName, String price, String? imageUrl, String? mainSkuId, List<OrderItem> items, String merchantId, String merchantName, String merchantAvatar, String receiverName, String receiverPhone, String receiverAddress, double shippingFee, double discount, double totalAmount, String paymentMethod, String paymentTime, String paymentStatus, String atmBankCode, String atmVirtualAccount, String atmExpireDate, String totalActualAmount, String shippingMethod, String deliveryTemp, String pickupStoreName, String shippingStatus, String statusTime, String statusSummary
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
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? orderSn = null,Object? statusCode = null,Object? title = null,Object? subtitle = null,Object? dateTime = null,Object? productName = null,Object? price = null,Object? imageUrl = freezed,Object? mainSkuId = freezed,Object? items = null,Object? merchantId = null,Object? merchantName = null,Object? merchantAvatar = null,Object? receiverName = null,Object? receiverPhone = null,Object? receiverAddress = null,Object? shippingFee = null,Object? discount = null,Object? totalAmount = null,Object? paymentMethod = null,Object? paymentTime = null,Object? paymentStatus = null,Object? atmBankCode = null,Object? atmVirtualAccount = null,Object? atmExpireDate = null,Object? totalActualAmount = null,Object? shippingMethod = null,Object? deliveryTemp = null,Object? pickupStoreName = null,Object? shippingStatus = null,Object? statusTime = null,Object? statusSummary = null,}) {
  return _then(_OrderSummary(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,orderSn: null == orderSn ? _self.orderSn : orderSn // ignore: cast_nullable_to_non_nullable
as String,statusCode: null == statusCode ? _self.statusCode : statusCode // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,subtitle: null == subtitle ? _self.subtitle : subtitle // ignore: cast_nullable_to_non_nullable
as String,dateTime: null == dateTime ? _self.dateTime : dateTime // ignore: cast_nullable_to_non_nullable
as String,productName: null == productName ? _self.productName : productName // ignore: cast_nullable_to_non_nullable
as String,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,mainSkuId: freezed == mainSkuId ? _self.mainSkuId : mainSkuId // ignore: cast_nullable_to_non_nullable
as String?,items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<OrderItem>,merchantId: null == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as String,merchantName: null == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String,merchantAvatar: null == merchantAvatar ? _self.merchantAvatar : merchantAvatar // ignore: cast_nullable_to_non_nullable
as String,receiverName: null == receiverName ? _self.receiverName : receiverName // ignore: cast_nullable_to_non_nullable
as String,receiverPhone: null == receiverPhone ? _self.receiverPhone : receiverPhone // ignore: cast_nullable_to_non_nullable
as String,receiverAddress: null == receiverAddress ? _self.receiverAddress : receiverAddress // ignore: cast_nullable_to_non_nullable
as String,shippingFee: null == shippingFee ? _self.shippingFee : shippingFee // ignore: cast_nullable_to_non_nullable
as double,discount: null == discount ? _self.discount : discount // ignore: cast_nullable_to_non_nullable
as double,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as double,paymentMethod: null == paymentMethod ? _self.paymentMethod : paymentMethod // ignore: cast_nullable_to_non_nullable
as String,paymentTime: null == paymentTime ? _self.paymentTime : paymentTime // ignore: cast_nullable_to_non_nullable
as String,paymentStatus: null == paymentStatus ? _self.paymentStatus : paymentStatus // ignore: cast_nullable_to_non_nullable
as String,atmBankCode: null == atmBankCode ? _self.atmBankCode : atmBankCode // ignore: cast_nullable_to_non_nullable
as String,atmVirtualAccount: null == atmVirtualAccount ? _self.atmVirtualAccount : atmVirtualAccount // ignore: cast_nullable_to_non_nullable
as String,atmExpireDate: null == atmExpireDate ? _self.atmExpireDate : atmExpireDate // ignore: cast_nullable_to_non_nullable
as String,totalActualAmount: null == totalActualAmount ? _self.totalActualAmount : totalActualAmount // ignore: cast_nullable_to_non_nullable
as String,shippingMethod: null == shippingMethod ? _self.shippingMethod : shippingMethod // ignore: cast_nullable_to_non_nullable
as String,deliveryTemp: null == deliveryTemp ? _self.deliveryTemp : deliveryTemp // ignore: cast_nullable_to_non_nullable
as String,pickupStoreName: null == pickupStoreName ? _self.pickupStoreName : pickupStoreName // ignore: cast_nullable_to_non_nullable
as String,shippingStatus: null == shippingStatus ? _self.shippingStatus : shippingStatus // ignore: cast_nullable_to_non_nullable
as String,statusTime: null == statusTime ? _self.statusTime : statusTime // ignore: cast_nullable_to_non_nullable
as String,statusSummary: null == statusSummary ? _self.statusSummary : statusSummary // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$OrderItem {

 String get name; int get quantity; String get price; String? get imageUrl; String? get skuId; String? get spuId; String? get merchantId;
/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderItemCopyWith<OrderItem> get copyWith => _$OrderItemCopyWithImpl<OrderItem>(this as OrderItem, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OrderItem&&(identical(other.name, name) || other.name == name)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.price, price) || other.price == price)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.spuId, spuId) || other.spuId == spuId)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId));
}


@override
int get hashCode => Object.hash(runtimeType,name,quantity,price,imageUrl,skuId,spuId,merchantId);

@override
String toString() {
  return 'OrderItem(name: $name, quantity: $quantity, price: $price, imageUrl: $imageUrl, skuId: $skuId, spuId: $spuId, merchantId: $merchantId)';
}


}

/// @nodoc
abstract mixin class $OrderItemCopyWith<$Res>  {
  factory $OrderItemCopyWith(OrderItem value, $Res Function(OrderItem) _then) = _$OrderItemCopyWithImpl;
@useResult
$Res call({
 String name, int quantity, String price, String? imageUrl, String? skuId, String? spuId, String? merchantId
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
@pragma('vm:prefer-inline') @override $Res call({Object? name = null,Object? quantity = null,Object? price = null,Object? imageUrl = freezed,Object? skuId = freezed,Object? spuId = freezed,Object? merchantId = freezed,}) {
  return _then(_self.copyWith(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as int,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,skuId: freezed == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String?,spuId: freezed == spuId ? _self.spuId : spuId // ignore: cast_nullable_to_non_nullable
as String?,merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as String?,
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String name,  int quantity,  String price,  String? imageUrl,  String? skuId,  String? spuId,  String? merchantId)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
return $default(_that.name,_that.quantity,_that.price,_that.imageUrl,_that.skuId,_that.spuId,_that.merchantId);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String name,  int quantity,  String price,  String? imageUrl,  String? skuId,  String? spuId,  String? merchantId)  $default,) {final _that = this;
switch (_that) {
case _OrderItem():
return $default(_that.name,_that.quantity,_that.price,_that.imageUrl,_that.skuId,_that.spuId,_that.merchantId);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String name,  int quantity,  String price,  String? imageUrl,  String? skuId,  String? spuId,  String? merchantId)?  $default,) {final _that = this;
switch (_that) {
case _OrderItem() when $default != null:
return $default(_that.name,_that.quantity,_that.price,_that.imageUrl,_that.skuId,_that.spuId,_that.merchantId);case _:
  return null;

}
}

}

/// @nodoc


class _OrderItem implements OrderItem {
  const _OrderItem({this.name = '', this.quantity = 1, this.price = '', this.imageUrl, this.skuId, this.spuId, this.merchantId});
  

@override@JsonKey() final  String name;
@override@JsonKey() final  int quantity;
@override@JsonKey() final  String price;
@override final  String? imageUrl;
@override final  String? skuId;
@override final  String? spuId;
@override final  String? merchantId;

/// Create a copy of OrderItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderItemCopyWith<_OrderItem> get copyWith => __$OrderItemCopyWithImpl<_OrderItem>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OrderItem&&(identical(other.name, name) || other.name == name)&&(identical(other.quantity, quantity) || other.quantity == quantity)&&(identical(other.price, price) || other.price == price)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.spuId, spuId) || other.spuId == spuId)&&(identical(other.merchantId, merchantId) || other.merchantId == merchantId));
}


@override
int get hashCode => Object.hash(runtimeType,name,quantity,price,imageUrl,skuId,spuId,merchantId);

@override
String toString() {
  return 'OrderItem(name: $name, quantity: $quantity, price: $price, imageUrl: $imageUrl, skuId: $skuId, spuId: $spuId, merchantId: $merchantId)';
}


}

/// @nodoc
abstract mixin class _$OrderItemCopyWith<$Res> implements $OrderItemCopyWith<$Res> {
  factory _$OrderItemCopyWith(_OrderItem value, $Res Function(_OrderItem) _then) = __$OrderItemCopyWithImpl;
@override @useResult
$Res call({
 String name, int quantity, String price, String? imageUrl, String? skuId, String? spuId, String? merchantId
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
@override @pragma('vm:prefer-inline') $Res call({Object? name = null,Object? quantity = null,Object? price = null,Object? imageUrl = freezed,Object? skuId = freezed,Object? spuId = freezed,Object? merchantId = freezed,}) {
  return _then(_OrderItem(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,quantity: null == quantity ? _self.quantity : quantity // ignore: cast_nullable_to_non_nullable
as int,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,skuId: freezed == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as String?,spuId: freezed == spuId ? _self.spuId : spuId // ignore: cast_nullable_to_non_nullable
as String?,merchantId: freezed == merchantId ? _self.merchantId : merchantId // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
