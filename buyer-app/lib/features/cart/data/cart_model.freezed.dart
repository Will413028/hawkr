// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'cart_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$CartItem {

 int get id;@JsonKey(name: 'skuId') int get skuId; int get count; int get price;// PayUni #758: 加入購物車當下的原價快照（刪除線用），缺欄位時為 null。
@JsonKey(name: 'listPrice') int? get listPrice; String? get title; String? get image;@JsonKey(name: 'sellerId') int? get sellerId;@JsonKey(name: 'merchantName') String? get merchantName;@JsonKey(name: 'specName') String? get specName; bool? get check;@JsonKey(name: 'userId') String? get userId;@JsonKey(name: 'userKey') String? get userKey;// issue #1127 後端 row 級可購買性
@JsonKey(name: 'stockQty') String? get stockQty;@JsonKey(name: 'purchasable') bool get purchasable;@JsonKey(name: 'unavailableReason') String? get unavailableReason;
/// Create a copy of CartItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CartItemCopyWith<CartItem> get copyWith => _$CartItemCopyWithImpl<CartItem>(this as CartItem, _$identity);

  /// Serializes this CartItem to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CartItem&&(identical(other.id, id) || other.id == id)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.count, count) || other.count == count)&&(identical(other.price, price) || other.price == price)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.title, title) || other.title == title)&&(identical(other.image, image) || other.image == image)&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.specName, specName) || other.specName == specName)&&(identical(other.check, check) || other.check == check)&&(identical(other.userId, userId) || other.userId == userId)&&(identical(other.userKey, userKey) || other.userKey == userKey)&&(identical(other.stockQty, stockQty) || other.stockQty == stockQty)&&(identical(other.purchasable, purchasable) || other.purchasable == purchasable)&&(identical(other.unavailableReason, unavailableReason) || other.unavailableReason == unavailableReason));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,skuId,count,price,listPrice,title,image,sellerId,merchantName,specName,check,userId,userKey,stockQty,purchasable,unavailableReason);

@override
String toString() {
  return 'CartItem(id: $id, skuId: $skuId, count: $count, price: $price, listPrice: $listPrice, title: $title, image: $image, sellerId: $sellerId, merchantName: $merchantName, specName: $specName, check: $check, userId: $userId, userKey: $userKey, stockQty: $stockQty, purchasable: $purchasable, unavailableReason: $unavailableReason)';
}


}

/// @nodoc
abstract mixin class $CartItemCopyWith<$Res>  {
  factory $CartItemCopyWith(CartItem value, $Res Function(CartItem) _then) = _$CartItemCopyWithImpl;
@useResult
$Res call({
 int id,@JsonKey(name: 'skuId') int skuId, int count, int price,@JsonKey(name: 'listPrice') int? listPrice, String? title, String? image,@JsonKey(name: 'sellerId') int? sellerId,@JsonKey(name: 'merchantName') String? merchantName,@JsonKey(name: 'specName') String? specName, bool? check,@JsonKey(name: 'userId') String? userId,@JsonKey(name: 'userKey') String? userKey,@JsonKey(name: 'stockQty') String? stockQty,@JsonKey(name: 'purchasable') bool purchasable,@JsonKey(name: 'unavailableReason') String? unavailableReason
});




}
/// @nodoc
class _$CartItemCopyWithImpl<$Res>
    implements $CartItemCopyWith<$Res> {
  _$CartItemCopyWithImpl(this._self, this._then);

  final CartItem _self;
  final $Res Function(CartItem) _then;

/// Create a copy of CartItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? skuId = null,Object? count = null,Object? price = null,Object? listPrice = freezed,Object? title = freezed,Object? image = freezed,Object? sellerId = freezed,Object? merchantName = freezed,Object? specName = freezed,Object? check = freezed,Object? userId = freezed,Object? userKey = freezed,Object? stockQty = freezed,Object? purchasable = null,Object? unavailableReason = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as int,count: null == count ? _self.count : count // ignore: cast_nullable_to_non_nullable
as int,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,listPrice: freezed == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as int?,title: freezed == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String?,image: freezed == image ? _self.image : image // ignore: cast_nullable_to_non_nullable
as String?,sellerId: freezed == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as int?,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,specName: freezed == specName ? _self.specName : specName // ignore: cast_nullable_to_non_nullable
as String?,check: freezed == check ? _self.check : check // ignore: cast_nullable_to_non_nullable
as bool?,userId: freezed == userId ? _self.userId : userId // ignore: cast_nullable_to_non_nullable
as String?,userKey: freezed == userKey ? _self.userKey : userKey // ignore: cast_nullable_to_non_nullable
as String?,stockQty: freezed == stockQty ? _self.stockQty : stockQty // ignore: cast_nullable_to_non_nullable
as String?,purchasable: null == purchasable ? _self.purchasable : purchasable // ignore: cast_nullable_to_non_nullable
as bool,unavailableReason: freezed == unavailableReason ? _self.unavailableReason : unavailableReason // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [CartItem].
extension CartItemPatterns on CartItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CartItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CartItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CartItem value)  $default,){
final _that = this;
switch (_that) {
case _CartItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CartItem value)?  $default,){
final _that = this;
switch (_that) {
case _CartItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id, @JsonKey(name: 'skuId')  int skuId,  int count,  int price, @JsonKey(name: 'listPrice')  int? listPrice,  String? title,  String? image, @JsonKey(name: 'sellerId')  int? sellerId, @JsonKey(name: 'merchantName')  String? merchantName, @JsonKey(name: 'specName')  String? specName,  bool? check, @JsonKey(name: 'userId')  String? userId, @JsonKey(name: 'userKey')  String? userKey, @JsonKey(name: 'stockQty')  String? stockQty, @JsonKey(name: 'purchasable')  bool purchasable, @JsonKey(name: 'unavailableReason')  String? unavailableReason)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CartItem() when $default != null:
return $default(_that.id,_that.skuId,_that.count,_that.price,_that.listPrice,_that.title,_that.image,_that.sellerId,_that.merchantName,_that.specName,_that.check,_that.userId,_that.userKey,_that.stockQty,_that.purchasable,_that.unavailableReason);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id, @JsonKey(name: 'skuId')  int skuId,  int count,  int price, @JsonKey(name: 'listPrice')  int? listPrice,  String? title,  String? image, @JsonKey(name: 'sellerId')  int? sellerId, @JsonKey(name: 'merchantName')  String? merchantName, @JsonKey(name: 'specName')  String? specName,  bool? check, @JsonKey(name: 'userId')  String? userId, @JsonKey(name: 'userKey')  String? userKey, @JsonKey(name: 'stockQty')  String? stockQty, @JsonKey(name: 'purchasable')  bool purchasable, @JsonKey(name: 'unavailableReason')  String? unavailableReason)  $default,) {final _that = this;
switch (_that) {
case _CartItem():
return $default(_that.id,_that.skuId,_that.count,_that.price,_that.listPrice,_that.title,_that.image,_that.sellerId,_that.merchantName,_that.specName,_that.check,_that.userId,_that.userKey,_that.stockQty,_that.purchasable,_that.unavailableReason);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id, @JsonKey(name: 'skuId')  int skuId,  int count,  int price, @JsonKey(name: 'listPrice')  int? listPrice,  String? title,  String? image, @JsonKey(name: 'sellerId')  int? sellerId, @JsonKey(name: 'merchantName')  String? merchantName, @JsonKey(name: 'specName')  String? specName,  bool? check, @JsonKey(name: 'userId')  String? userId, @JsonKey(name: 'userKey')  String? userKey, @JsonKey(name: 'stockQty')  String? stockQty, @JsonKey(name: 'purchasable')  bool purchasable, @JsonKey(name: 'unavailableReason')  String? unavailableReason)?  $default,) {final _that = this;
switch (_that) {
case _CartItem() when $default != null:
return $default(_that.id,_that.skuId,_that.count,_that.price,_that.listPrice,_that.title,_that.image,_that.sellerId,_that.merchantName,_that.specName,_that.check,_that.userId,_that.userKey,_that.stockQty,_that.purchasable,_that.unavailableReason);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _CartItem implements CartItem {
  const _CartItem({required this.id, @JsonKey(name: 'skuId') required this.skuId, required this.count, required this.price, @JsonKey(name: 'listPrice') this.listPrice, this.title, this.image, @JsonKey(name: 'sellerId') this.sellerId, @JsonKey(name: 'merchantName') this.merchantName, @JsonKey(name: 'specName') this.specName, this.check, @JsonKey(name: 'userId') this.userId, @JsonKey(name: 'userKey') this.userKey, @JsonKey(name: 'stockQty') this.stockQty, @JsonKey(name: 'purchasable') this.purchasable = true, @JsonKey(name: 'unavailableReason') this.unavailableReason});
  factory _CartItem.fromJson(Map<String, dynamic> json) => _$CartItemFromJson(json);

@override final  int id;
@override@JsonKey(name: 'skuId') final  int skuId;
@override final  int count;
@override final  int price;
// PayUni #758: 加入購物車當下的原價快照（刪除線用），缺欄位時為 null。
@override@JsonKey(name: 'listPrice') final  int? listPrice;
@override final  String? title;
@override final  String? image;
@override@JsonKey(name: 'sellerId') final  int? sellerId;
@override@JsonKey(name: 'merchantName') final  String? merchantName;
@override@JsonKey(name: 'specName') final  String? specName;
@override final  bool? check;
@override@JsonKey(name: 'userId') final  String? userId;
@override@JsonKey(name: 'userKey') final  String? userKey;
// issue #1127 後端 row 級可購買性
@override@JsonKey(name: 'stockQty') final  String? stockQty;
@override@JsonKey(name: 'purchasable') final  bool purchasable;
@override@JsonKey(name: 'unavailableReason') final  String? unavailableReason;

/// Create a copy of CartItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CartItemCopyWith<_CartItem> get copyWith => __$CartItemCopyWithImpl<_CartItem>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CartItemToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CartItem&&(identical(other.id, id) || other.id == id)&&(identical(other.skuId, skuId) || other.skuId == skuId)&&(identical(other.count, count) || other.count == count)&&(identical(other.price, price) || other.price == price)&&(identical(other.listPrice, listPrice) || other.listPrice == listPrice)&&(identical(other.title, title) || other.title == title)&&(identical(other.image, image) || other.image == image)&&(identical(other.sellerId, sellerId) || other.sellerId == sellerId)&&(identical(other.merchantName, merchantName) || other.merchantName == merchantName)&&(identical(other.specName, specName) || other.specName == specName)&&(identical(other.check, check) || other.check == check)&&(identical(other.userId, userId) || other.userId == userId)&&(identical(other.userKey, userKey) || other.userKey == userKey)&&(identical(other.stockQty, stockQty) || other.stockQty == stockQty)&&(identical(other.purchasable, purchasable) || other.purchasable == purchasable)&&(identical(other.unavailableReason, unavailableReason) || other.unavailableReason == unavailableReason));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,skuId,count,price,listPrice,title,image,sellerId,merchantName,specName,check,userId,userKey,stockQty,purchasable,unavailableReason);

@override
String toString() {
  return 'CartItem(id: $id, skuId: $skuId, count: $count, price: $price, listPrice: $listPrice, title: $title, image: $image, sellerId: $sellerId, merchantName: $merchantName, specName: $specName, check: $check, userId: $userId, userKey: $userKey, stockQty: $stockQty, purchasable: $purchasable, unavailableReason: $unavailableReason)';
}


}

/// @nodoc
abstract mixin class _$CartItemCopyWith<$Res> implements $CartItemCopyWith<$Res> {
  factory _$CartItemCopyWith(_CartItem value, $Res Function(_CartItem) _then) = __$CartItemCopyWithImpl;
@override @useResult
$Res call({
 int id,@JsonKey(name: 'skuId') int skuId, int count, int price,@JsonKey(name: 'listPrice') int? listPrice, String? title, String? image,@JsonKey(name: 'sellerId') int? sellerId,@JsonKey(name: 'merchantName') String? merchantName,@JsonKey(name: 'specName') String? specName, bool? check,@JsonKey(name: 'userId') String? userId,@JsonKey(name: 'userKey') String? userKey,@JsonKey(name: 'stockQty') String? stockQty,@JsonKey(name: 'purchasable') bool purchasable,@JsonKey(name: 'unavailableReason') String? unavailableReason
});




}
/// @nodoc
class __$CartItemCopyWithImpl<$Res>
    implements _$CartItemCopyWith<$Res> {
  __$CartItemCopyWithImpl(this._self, this._then);

  final _CartItem _self;
  final $Res Function(_CartItem) _then;

/// Create a copy of CartItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? skuId = null,Object? count = null,Object? price = null,Object? listPrice = freezed,Object? title = freezed,Object? image = freezed,Object? sellerId = freezed,Object? merchantName = freezed,Object? specName = freezed,Object? check = freezed,Object? userId = freezed,Object? userKey = freezed,Object? stockQty = freezed,Object? purchasable = null,Object? unavailableReason = freezed,}) {
  return _then(_CartItem(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,skuId: null == skuId ? _self.skuId : skuId // ignore: cast_nullable_to_non_nullable
as int,count: null == count ? _self.count : count // ignore: cast_nullable_to_non_nullable
as int,price: null == price ? _self.price : price // ignore: cast_nullable_to_non_nullable
as int,listPrice: freezed == listPrice ? _self.listPrice : listPrice // ignore: cast_nullable_to_non_nullable
as int?,title: freezed == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String?,image: freezed == image ? _self.image : image // ignore: cast_nullable_to_non_nullable
as String?,sellerId: freezed == sellerId ? _self.sellerId : sellerId // ignore: cast_nullable_to_non_nullable
as int?,merchantName: freezed == merchantName ? _self.merchantName : merchantName // ignore: cast_nullable_to_non_nullable
as String?,specName: freezed == specName ? _self.specName : specName // ignore: cast_nullable_to_non_nullable
as String?,check: freezed == check ? _self.check : check // ignore: cast_nullable_to_non_nullable
as bool?,userId: freezed == userId ? _self.userId : userId // ignore: cast_nullable_to_non_nullable
as String?,userKey: freezed == userKey ? _self.userKey : userKey // ignore: cast_nullable_to_non_nullable
as String?,stockQty: freezed == stockQty ? _self.stockQty : stockQty // ignore: cast_nullable_to_non_nullable
as String?,purchasable: null == purchasable ? _self.purchasable : purchasable // ignore: cast_nullable_to_non_nullable
as bool,unavailableReason: freezed == unavailableReason ? _self.unavailableReason : unavailableReason // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}


/// @nodoc
mixin _$CartSummary {

 List<CartItem> get items;@JsonKey(name: 'totalAmount') int get totalAmount;
/// Create a copy of CartSummary
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CartSummaryCopyWith<CartSummary> get copyWith => _$CartSummaryCopyWithImpl<CartSummary>(this as CartSummary, _$identity);

  /// Serializes this CartSummary to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CartSummary&&const DeepCollectionEquality().equals(other.items, items)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(items),totalAmount);

@override
String toString() {
  return 'CartSummary(items: $items, totalAmount: $totalAmount)';
}


}

/// @nodoc
abstract mixin class $CartSummaryCopyWith<$Res>  {
  factory $CartSummaryCopyWith(CartSummary value, $Res Function(CartSummary) _then) = _$CartSummaryCopyWithImpl;
@useResult
$Res call({
 List<CartItem> items,@JsonKey(name: 'totalAmount') int totalAmount
});




}
/// @nodoc
class _$CartSummaryCopyWithImpl<$Res>
    implements $CartSummaryCopyWith<$Res> {
  _$CartSummaryCopyWithImpl(this._self, this._then);

  final CartSummary _self;
  final $Res Function(CartSummary) _then;

/// Create a copy of CartSummary
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? items = null,Object? totalAmount = null,}) {
  return _then(_self.copyWith(
items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<CartItem>,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [CartSummary].
extension CartSummaryPatterns on CartSummary {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CartSummary value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CartSummary() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CartSummary value)  $default,){
final _that = this;
switch (_that) {
case _CartSummary():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CartSummary value)?  $default,){
final _that = this;
switch (_that) {
case _CartSummary() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<CartItem> items, @JsonKey(name: 'totalAmount')  int totalAmount)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CartSummary() when $default != null:
return $default(_that.items,_that.totalAmount);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<CartItem> items, @JsonKey(name: 'totalAmount')  int totalAmount)  $default,) {final _that = this;
switch (_that) {
case _CartSummary():
return $default(_that.items,_that.totalAmount);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<CartItem> items, @JsonKey(name: 'totalAmount')  int totalAmount)?  $default,) {final _that = this;
switch (_that) {
case _CartSummary() when $default != null:
return $default(_that.items,_that.totalAmount);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _CartSummary implements CartSummary {
  const _CartSummary({final  List<CartItem> items = const [], @JsonKey(name: 'totalAmount') this.totalAmount = 0}): _items = items;
  factory _CartSummary.fromJson(Map<String, dynamic> json) => _$CartSummaryFromJson(json);

 final  List<CartItem> _items;
@override@JsonKey() List<CartItem> get items {
  if (_items is EqualUnmodifiableListView) return _items;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_items);
}

@override@JsonKey(name: 'totalAmount') final  int totalAmount;

/// Create a copy of CartSummary
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CartSummaryCopyWith<_CartSummary> get copyWith => __$CartSummaryCopyWithImpl<_CartSummary>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CartSummaryToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CartSummary&&const DeepCollectionEquality().equals(other._items, _items)&&(identical(other.totalAmount, totalAmount) || other.totalAmount == totalAmount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_items),totalAmount);

@override
String toString() {
  return 'CartSummary(items: $items, totalAmount: $totalAmount)';
}


}

/// @nodoc
abstract mixin class _$CartSummaryCopyWith<$Res> implements $CartSummaryCopyWith<$Res> {
  factory _$CartSummaryCopyWith(_CartSummary value, $Res Function(_CartSummary) _then) = __$CartSummaryCopyWithImpl;
@override @useResult
$Res call({
 List<CartItem> items,@JsonKey(name: 'totalAmount') int totalAmount
});




}
/// @nodoc
class __$CartSummaryCopyWithImpl<$Res>
    implements _$CartSummaryCopyWith<$Res> {
  __$CartSummaryCopyWithImpl(this._self, this._then);

  final _CartSummary _self;
  final $Res Function(_CartSummary) _then;

/// Create a copy of CartSummary
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? items = null,Object? totalAmount = null,}) {
  return _then(_CartSummary(
items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<CartItem>,totalAmount: null == totalAmount ? _self.totalAmount : totalAmount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
