// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'customer_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$CustomerTag {

 int get id; String get name;
/// Create a copy of CustomerTag
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CustomerTagCopyWith<CustomerTag> get copyWith => _$CustomerTagCopyWithImpl<CustomerTag>(this as CustomerTag, _$identity);

  /// Serializes this CustomerTag to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CustomerTag&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name);

@override
String toString() {
  return 'CustomerTag(id: $id, name: $name)';
}


}

/// @nodoc
abstract mixin class $CustomerTagCopyWith<$Res>  {
  factory $CustomerTagCopyWith(CustomerTag value, $Res Function(CustomerTag) _then) = _$CustomerTagCopyWithImpl;
@useResult
$Res call({
 int id, String name
});




}
/// @nodoc
class _$CustomerTagCopyWithImpl<$Res>
    implements $CustomerTagCopyWith<$Res> {
  _$CustomerTagCopyWithImpl(this._self, this._then);

  final CustomerTag _self;
  final $Res Function(CustomerTag) _then;

/// Create a copy of CustomerTag
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? name = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [CustomerTag].
extension CustomerTagPatterns on CustomerTag {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CustomerTag value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CustomerTag() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CustomerTag value)  $default,){
final _that = this;
switch (_that) {
case _CustomerTag():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CustomerTag value)?  $default,){
final _that = this;
switch (_that) {
case _CustomerTag() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int id,  String name)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CustomerTag() when $default != null:
return $default(_that.id,_that.name);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int id,  String name)  $default,) {final _that = this;
switch (_that) {
case _CustomerTag():
return $default(_that.id,_that.name);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int id,  String name)?  $default,) {final _that = this;
switch (_that) {
case _CustomerTag() when $default != null:
return $default(_that.id,_that.name);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _CustomerTag implements CustomerTag {
  const _CustomerTag({required this.id, required this.name});
  factory _CustomerTag.fromJson(Map<String, dynamic> json) => _$CustomerTagFromJson(json);

@override final  int id;
@override final  String name;

/// Create a copy of CustomerTag
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CustomerTagCopyWith<_CustomerTag> get copyWith => __$CustomerTagCopyWithImpl<_CustomerTag>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CustomerTagToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CustomerTag&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name);

@override
String toString() {
  return 'CustomerTag(id: $id, name: $name)';
}


}

/// @nodoc
abstract mixin class _$CustomerTagCopyWith<$Res> implements $CustomerTagCopyWith<$Res> {
  factory _$CustomerTagCopyWith(_CustomerTag value, $Res Function(_CustomerTag) _then) = __$CustomerTagCopyWithImpl;
@override @useResult
$Res call({
 int id, String name
});




}
/// @nodoc
class __$CustomerTagCopyWithImpl<$Res>
    implements _$CustomerTagCopyWith<$Res> {
  __$CustomerTagCopyWithImpl(this._self, this._then);

  final _CustomerTag _self;
  final $Res Function(_CustomerTag) _then;

/// Create a copy of CustomerTag
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? name = null,}) {
  return _then(_CustomerTag(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}


/// @nodoc
mixin _$Customer {

/// 客戶識別碼，值語意是 buyer_id（API `buyerId`）。
 String get id; String get name; String get phone; int get totalSpent;/// 訂單數（API `order_count`）
 int get visitCount; int get couponCount; bool get isVip; bool get isBlacklisted;/// API 原始值：'normal' / 'vip' / 'blocklist'
 String get customerType; String? get avatarUrl; String? get address; String? get email; List<String> get frequentItems; List<CustomerTag> get tags;
/// Create a copy of Customer
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CustomerCopyWith<Customer> get copyWith => _$CustomerCopyWithImpl<Customer>(this as Customer, _$identity);

  /// Serializes this Customer to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Customer&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.totalSpent, totalSpent) || other.totalSpent == totalSpent)&&(identical(other.visitCount, visitCount) || other.visitCount == visitCount)&&(identical(other.couponCount, couponCount) || other.couponCount == couponCount)&&(identical(other.isVip, isVip) || other.isVip == isVip)&&(identical(other.isBlacklisted, isBlacklisted) || other.isBlacklisted == isBlacklisted)&&(identical(other.customerType, customerType) || other.customerType == customerType)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl)&&(identical(other.address, address) || other.address == address)&&(identical(other.email, email) || other.email == email)&&const DeepCollectionEquality().equals(other.frequentItems, frequentItems)&&const DeepCollectionEquality().equals(other.tags, tags));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name,phone,totalSpent,visitCount,couponCount,isVip,isBlacklisted,customerType,avatarUrl,address,email,const DeepCollectionEquality().hash(frequentItems),const DeepCollectionEquality().hash(tags));

@override
String toString() {
  return 'Customer(id: $id, name: $name, phone: $phone, totalSpent: $totalSpent, visitCount: $visitCount, couponCount: $couponCount, isVip: $isVip, isBlacklisted: $isBlacklisted, customerType: $customerType, avatarUrl: $avatarUrl, address: $address, email: $email, frequentItems: $frequentItems, tags: $tags)';
}


}

/// @nodoc
abstract mixin class $CustomerCopyWith<$Res>  {
  factory $CustomerCopyWith(Customer value, $Res Function(Customer) _then) = _$CustomerCopyWithImpl;
@useResult
$Res call({
 String id, String name, String phone, int totalSpent, int visitCount, int couponCount, bool isVip, bool isBlacklisted, String customerType, String? avatarUrl, String? address, String? email, List<String> frequentItems, List<CustomerTag> tags
});




}
/// @nodoc
class _$CustomerCopyWithImpl<$Res>
    implements $CustomerCopyWith<$Res> {
  _$CustomerCopyWithImpl(this._self, this._then);

  final Customer _self;
  final $Res Function(Customer) _then;

/// Create a copy of Customer
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? name = null,Object? phone = null,Object? totalSpent = null,Object? visitCount = null,Object? couponCount = null,Object? isVip = null,Object? isBlacklisted = null,Object? customerType = null,Object? avatarUrl = freezed,Object? address = freezed,Object? email = freezed,Object? frequentItems = null,Object? tags = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,totalSpent: null == totalSpent ? _self.totalSpent : totalSpent // ignore: cast_nullable_to_non_nullable
as int,visitCount: null == visitCount ? _self.visitCount : visitCount // ignore: cast_nullable_to_non_nullable
as int,couponCount: null == couponCount ? _self.couponCount : couponCount // ignore: cast_nullable_to_non_nullable
as int,isVip: null == isVip ? _self.isVip : isVip // ignore: cast_nullable_to_non_nullable
as bool,isBlacklisted: null == isBlacklisted ? _self.isBlacklisted : isBlacklisted // ignore: cast_nullable_to_non_nullable
as bool,customerType: null == customerType ? _self.customerType : customerType // ignore: cast_nullable_to_non_nullable
as String,avatarUrl: freezed == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String?,address: freezed == address ? _self.address : address // ignore: cast_nullable_to_non_nullable
as String?,email: freezed == email ? _self.email : email // ignore: cast_nullable_to_non_nullable
as String?,frequentItems: null == frequentItems ? _self.frequentItems : frequentItems // ignore: cast_nullable_to_non_nullable
as List<String>,tags: null == tags ? _self.tags : tags // ignore: cast_nullable_to_non_nullable
as List<CustomerTag>,
  ));
}

}


/// Adds pattern-matching-related methods to [Customer].
extension CustomerPatterns on Customer {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Customer value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Customer() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Customer value)  $default,){
final _that = this;
switch (_that) {
case _Customer():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Customer value)?  $default,){
final _that = this;
switch (_that) {
case _Customer() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String name,  String phone,  int totalSpent,  int visitCount,  int couponCount,  bool isVip,  bool isBlacklisted,  String customerType,  String? avatarUrl,  String? address,  String? email,  List<String> frequentItems,  List<CustomerTag> tags)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Customer() when $default != null:
return $default(_that.id,_that.name,_that.phone,_that.totalSpent,_that.visitCount,_that.couponCount,_that.isVip,_that.isBlacklisted,_that.customerType,_that.avatarUrl,_that.address,_that.email,_that.frequentItems,_that.tags);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String name,  String phone,  int totalSpent,  int visitCount,  int couponCount,  bool isVip,  bool isBlacklisted,  String customerType,  String? avatarUrl,  String? address,  String? email,  List<String> frequentItems,  List<CustomerTag> tags)  $default,) {final _that = this;
switch (_that) {
case _Customer():
return $default(_that.id,_that.name,_that.phone,_that.totalSpent,_that.visitCount,_that.couponCount,_that.isVip,_that.isBlacklisted,_that.customerType,_that.avatarUrl,_that.address,_that.email,_that.frequentItems,_that.tags);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String name,  String phone,  int totalSpent,  int visitCount,  int couponCount,  bool isVip,  bool isBlacklisted,  String customerType,  String? avatarUrl,  String? address,  String? email,  List<String> frequentItems,  List<CustomerTag> tags)?  $default,) {final _that = this;
switch (_that) {
case _Customer() when $default != null:
return $default(_that.id,_that.name,_that.phone,_that.totalSpent,_that.visitCount,_that.couponCount,_that.isVip,_that.isBlacklisted,_that.customerType,_that.avatarUrl,_that.address,_that.email,_that.frequentItems,_that.tags);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _Customer implements Customer {
  const _Customer({required this.id, required this.name, required this.phone, this.totalSpent = 0, this.visitCount = 0, this.couponCount = 0, this.isVip = false, this.isBlacklisted = false, this.customerType = 'normal', this.avatarUrl, this.address, this.email, final  List<String> frequentItems = const [], final  List<CustomerTag> tags = const []}): _frequentItems = frequentItems,_tags = tags;
  factory _Customer.fromJson(Map<String, dynamic> json) => _$CustomerFromJson(json);

/// 客戶識別碼，值語意是 buyer_id（API `buyerId`）。
@override final  String id;
@override final  String name;
@override final  String phone;
@override@JsonKey() final  int totalSpent;
/// 訂單數（API `order_count`）
@override@JsonKey() final  int visitCount;
@override@JsonKey() final  int couponCount;
@override@JsonKey() final  bool isVip;
@override@JsonKey() final  bool isBlacklisted;
/// API 原始值：'normal' / 'vip' / 'blocklist'
@override@JsonKey() final  String customerType;
@override final  String? avatarUrl;
@override final  String? address;
@override final  String? email;
 final  List<String> _frequentItems;
@override@JsonKey() List<String> get frequentItems {
  if (_frequentItems is EqualUnmodifiableListView) return _frequentItems;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_frequentItems);
}

 final  List<CustomerTag> _tags;
@override@JsonKey() List<CustomerTag> get tags {
  if (_tags is EqualUnmodifiableListView) return _tags;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_tags);
}


/// Create a copy of Customer
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CustomerCopyWith<_Customer> get copyWith => __$CustomerCopyWithImpl<_Customer>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$CustomerToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Customer&&(identical(other.id, id) || other.id == id)&&(identical(other.name, name) || other.name == name)&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.totalSpent, totalSpent) || other.totalSpent == totalSpent)&&(identical(other.visitCount, visitCount) || other.visitCount == visitCount)&&(identical(other.couponCount, couponCount) || other.couponCount == couponCount)&&(identical(other.isVip, isVip) || other.isVip == isVip)&&(identical(other.isBlacklisted, isBlacklisted) || other.isBlacklisted == isBlacklisted)&&(identical(other.customerType, customerType) || other.customerType == customerType)&&(identical(other.avatarUrl, avatarUrl) || other.avatarUrl == avatarUrl)&&(identical(other.address, address) || other.address == address)&&(identical(other.email, email) || other.email == email)&&const DeepCollectionEquality().equals(other._frequentItems, _frequentItems)&&const DeepCollectionEquality().equals(other._tags, _tags));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,name,phone,totalSpent,visitCount,couponCount,isVip,isBlacklisted,customerType,avatarUrl,address,email,const DeepCollectionEquality().hash(_frequentItems),const DeepCollectionEquality().hash(_tags));

@override
String toString() {
  return 'Customer(id: $id, name: $name, phone: $phone, totalSpent: $totalSpent, visitCount: $visitCount, couponCount: $couponCount, isVip: $isVip, isBlacklisted: $isBlacklisted, customerType: $customerType, avatarUrl: $avatarUrl, address: $address, email: $email, frequentItems: $frequentItems, tags: $tags)';
}


}

/// @nodoc
abstract mixin class _$CustomerCopyWith<$Res> implements $CustomerCopyWith<$Res> {
  factory _$CustomerCopyWith(_Customer value, $Res Function(_Customer) _then) = __$CustomerCopyWithImpl;
@override @useResult
$Res call({
 String id, String name, String phone, int totalSpent, int visitCount, int couponCount, bool isVip, bool isBlacklisted, String customerType, String? avatarUrl, String? address, String? email, List<String> frequentItems, List<CustomerTag> tags
});




}
/// @nodoc
class __$CustomerCopyWithImpl<$Res>
    implements _$CustomerCopyWith<$Res> {
  __$CustomerCopyWithImpl(this._self, this._then);

  final _Customer _self;
  final $Res Function(_Customer) _then;

/// Create a copy of Customer
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? name = null,Object? phone = null,Object? totalSpent = null,Object? visitCount = null,Object? couponCount = null,Object? isVip = null,Object? isBlacklisted = null,Object? customerType = null,Object? avatarUrl = freezed,Object? address = freezed,Object? email = freezed,Object? frequentItems = null,Object? tags = null,}) {
  return _then(_Customer(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,totalSpent: null == totalSpent ? _self.totalSpent : totalSpent // ignore: cast_nullable_to_non_nullable
as int,visitCount: null == visitCount ? _self.visitCount : visitCount // ignore: cast_nullable_to_non_nullable
as int,couponCount: null == couponCount ? _self.couponCount : couponCount // ignore: cast_nullable_to_non_nullable
as int,isVip: null == isVip ? _self.isVip : isVip // ignore: cast_nullable_to_non_nullable
as bool,isBlacklisted: null == isBlacklisted ? _self.isBlacklisted : isBlacklisted // ignore: cast_nullable_to_non_nullable
as bool,customerType: null == customerType ? _self.customerType : customerType // ignore: cast_nullable_to_non_nullable
as String,avatarUrl: freezed == avatarUrl ? _self.avatarUrl : avatarUrl // ignore: cast_nullable_to_non_nullable
as String?,address: freezed == address ? _self.address : address // ignore: cast_nullable_to_non_nullable
as String?,email: freezed == email ? _self.email : email // ignore: cast_nullable_to_non_nullable
as String?,frequentItems: null == frequentItems ? _self._frequentItems : frequentItems // ignore: cast_nullable_to_non_nullable
as List<String>,tags: null == tags ? _self._tags : tags // ignore: cast_nullable_to_non_nullable
as List<CustomerTag>,
  ));
}


}


/// @nodoc
mixin _$SpendingRecord {

 String get date; String get orderId; int get amount; int get refund;/// API `discountAmount` — 折扣金額
 int get discountAmount;
/// Create a copy of SpendingRecord
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SpendingRecordCopyWith<SpendingRecord> get copyWith => _$SpendingRecordCopyWithImpl<SpendingRecord>(this as SpendingRecord, _$identity);

  /// Serializes this SpendingRecord to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SpendingRecord&&(identical(other.date, date) || other.date == date)&&(identical(other.orderId, orderId) || other.orderId == orderId)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.refund, refund) || other.refund == refund)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,date,orderId,amount,refund,discountAmount);

@override
String toString() {
  return 'SpendingRecord(date: $date, orderId: $orderId, amount: $amount, refund: $refund, discountAmount: $discountAmount)';
}


}

/// @nodoc
abstract mixin class $SpendingRecordCopyWith<$Res>  {
  factory $SpendingRecordCopyWith(SpendingRecord value, $Res Function(SpendingRecord) _then) = _$SpendingRecordCopyWithImpl;
@useResult
$Res call({
 String date, String orderId, int amount, int refund, int discountAmount
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
@pragma('vm:prefer-inline') @override $Res call({Object? date = null,Object? orderId = null,Object? amount = null,Object? refund = null,Object? discountAmount = null,}) {
  return _then(_self.copyWith(
date: null == date ? _self.date : date // ignore: cast_nullable_to_non_nullable
as String,orderId: null == orderId ? _self.orderId : orderId // ignore: cast_nullable_to_non_nullable
as String,amount: null == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as int,refund: null == refund ? _self.refund : refund // ignore: cast_nullable_to_non_nullable
as int,discountAmount: null == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as int,
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String date,  String orderId,  int amount,  int refund,  int discountAmount)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SpendingRecord() when $default != null:
return $default(_that.date,_that.orderId,_that.amount,_that.refund,_that.discountAmount);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String date,  String orderId,  int amount,  int refund,  int discountAmount)  $default,) {final _that = this;
switch (_that) {
case _SpendingRecord():
return $default(_that.date,_that.orderId,_that.amount,_that.refund,_that.discountAmount);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String date,  String orderId,  int amount,  int refund,  int discountAmount)?  $default,) {final _that = this;
switch (_that) {
case _SpendingRecord() when $default != null:
return $default(_that.date,_that.orderId,_that.amount,_that.refund,_that.discountAmount);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SpendingRecord implements SpendingRecord {
  const _SpendingRecord({required this.date, required this.orderId, required this.amount, required this.refund, this.discountAmount = 0});
  factory _SpendingRecord.fromJson(Map<String, dynamic> json) => _$SpendingRecordFromJson(json);

@override final  String date;
@override final  String orderId;
@override final  int amount;
@override final  int refund;
/// API `discountAmount` — 折扣金額
@override@JsonKey() final  int discountAmount;

/// Create a copy of SpendingRecord
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SpendingRecordCopyWith<_SpendingRecord> get copyWith => __$SpendingRecordCopyWithImpl<_SpendingRecord>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SpendingRecordToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SpendingRecord&&(identical(other.date, date) || other.date == date)&&(identical(other.orderId, orderId) || other.orderId == orderId)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.refund, refund) || other.refund == refund)&&(identical(other.discountAmount, discountAmount) || other.discountAmount == discountAmount));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,date,orderId,amount,refund,discountAmount);

@override
String toString() {
  return 'SpendingRecord(date: $date, orderId: $orderId, amount: $amount, refund: $refund, discountAmount: $discountAmount)';
}


}

/// @nodoc
abstract mixin class _$SpendingRecordCopyWith<$Res> implements $SpendingRecordCopyWith<$Res> {
  factory _$SpendingRecordCopyWith(_SpendingRecord value, $Res Function(_SpendingRecord) _then) = __$SpendingRecordCopyWithImpl;
@override @useResult
$Res call({
 String date, String orderId, int amount, int refund, int discountAmount
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
@override @pragma('vm:prefer-inline') $Res call({Object? date = null,Object? orderId = null,Object? amount = null,Object? refund = null,Object? discountAmount = null,}) {
  return _then(_SpendingRecord(
date: null == date ? _self.date : date // ignore: cast_nullable_to_non_nullable
as String,orderId: null == orderId ? _self.orderId : orderId // ignore: cast_nullable_to_non_nullable
as String,amount: null == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as int,refund: null == refund ? _self.refund : refund // ignore: cast_nullable_to_non_nullable
as int,discountAmount: null == discountAmount ? _self.discountAmount : discountAmount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}


/// @nodoc
mixin _$OrderRecord {

 String get orderId; String get date; String get items; int get amount;// ignore: invalid_annotation_target
@JsonKey(fromJson: _parseOrderStatus) OrderStatus get status; int get rating; String get review;
/// Create a copy of OrderRecord
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$OrderRecordCopyWith<OrderRecord> get copyWith => _$OrderRecordCopyWithImpl<OrderRecord>(this as OrderRecord, _$identity);

  /// Serializes this OrderRecord to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is OrderRecord&&(identical(other.orderId, orderId) || other.orderId == orderId)&&(identical(other.date, date) || other.date == date)&&(identical(other.items, items) || other.items == items)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.status, status) || other.status == status)&&(identical(other.rating, rating) || other.rating == rating)&&(identical(other.review, review) || other.review == review));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,orderId,date,items,amount,status,rating,review);

@override
String toString() {
  return 'OrderRecord(orderId: $orderId, date: $date, items: $items, amount: $amount, status: $status, rating: $rating, review: $review)';
}


}

/// @nodoc
abstract mixin class $OrderRecordCopyWith<$Res>  {
  factory $OrderRecordCopyWith(OrderRecord value, $Res Function(OrderRecord) _then) = _$OrderRecordCopyWithImpl;
@useResult
$Res call({
 String orderId, String date, String items, int amount,@JsonKey(fromJson: _parseOrderStatus) OrderStatus status, int rating, String review
});




}
/// @nodoc
class _$OrderRecordCopyWithImpl<$Res>
    implements $OrderRecordCopyWith<$Res> {
  _$OrderRecordCopyWithImpl(this._self, this._then);

  final OrderRecord _self;
  final $Res Function(OrderRecord) _then;

/// Create a copy of OrderRecord
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? orderId = null,Object? date = null,Object? items = null,Object? amount = null,Object? status = null,Object? rating = null,Object? review = null,}) {
  return _then(_self.copyWith(
orderId: null == orderId ? _self.orderId : orderId // ignore: cast_nullable_to_non_nullable
as String,date: null == date ? _self.date : date // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as String,amount: null == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as int,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as OrderStatus,rating: null == rating ? _self.rating : rating // ignore: cast_nullable_to_non_nullable
as int,review: null == review ? _self.review : review // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [OrderRecord].
extension OrderRecordPatterns on OrderRecord {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _OrderRecord value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _OrderRecord() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _OrderRecord value)  $default,){
final _that = this;
switch (_that) {
case _OrderRecord():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _OrderRecord value)?  $default,){
final _that = this;
switch (_that) {
case _OrderRecord() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String orderId,  String date,  String items,  int amount, @JsonKey(fromJson: _parseOrderStatus)  OrderStatus status,  int rating,  String review)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _OrderRecord() when $default != null:
return $default(_that.orderId,_that.date,_that.items,_that.amount,_that.status,_that.rating,_that.review);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String orderId,  String date,  String items,  int amount, @JsonKey(fromJson: _parseOrderStatus)  OrderStatus status,  int rating,  String review)  $default,) {final _that = this;
switch (_that) {
case _OrderRecord():
return $default(_that.orderId,_that.date,_that.items,_that.amount,_that.status,_that.rating,_that.review);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String orderId,  String date,  String items,  int amount, @JsonKey(fromJson: _parseOrderStatus)  OrderStatus status,  int rating,  String review)?  $default,) {final _that = this;
switch (_that) {
case _OrderRecord() when $default != null:
return $default(_that.orderId,_that.date,_that.items,_that.amount,_that.status,_that.rating,_that.review);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _OrderRecord implements OrderRecord {
  const _OrderRecord({required this.orderId, required this.date, required this.items, required this.amount, @JsonKey(fromJson: _parseOrderStatus) this.status = OrderStatus.completed, this.rating = 0, this.review = ''});
  factory _OrderRecord.fromJson(Map<String, dynamic> json) => _$OrderRecordFromJson(json);

@override final  String orderId;
@override final  String date;
@override final  String items;
@override final  int amount;
// ignore: invalid_annotation_target
@override@JsonKey(fromJson: _parseOrderStatus) final  OrderStatus status;
@override@JsonKey() final  int rating;
@override@JsonKey() final  String review;

/// Create a copy of OrderRecord
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$OrderRecordCopyWith<_OrderRecord> get copyWith => __$OrderRecordCopyWithImpl<_OrderRecord>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$OrderRecordToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _OrderRecord&&(identical(other.orderId, orderId) || other.orderId == orderId)&&(identical(other.date, date) || other.date == date)&&(identical(other.items, items) || other.items == items)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.status, status) || other.status == status)&&(identical(other.rating, rating) || other.rating == rating)&&(identical(other.review, review) || other.review == review));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,orderId,date,items,amount,status,rating,review);

@override
String toString() {
  return 'OrderRecord(orderId: $orderId, date: $date, items: $items, amount: $amount, status: $status, rating: $rating, review: $review)';
}


}

/// @nodoc
abstract mixin class _$OrderRecordCopyWith<$Res> implements $OrderRecordCopyWith<$Res> {
  factory _$OrderRecordCopyWith(_OrderRecord value, $Res Function(_OrderRecord) _then) = __$OrderRecordCopyWithImpl;
@override @useResult
$Res call({
 String orderId, String date, String items, int amount,@JsonKey(fromJson: _parseOrderStatus) OrderStatus status, int rating, String review
});




}
/// @nodoc
class __$OrderRecordCopyWithImpl<$Res>
    implements _$OrderRecordCopyWith<$Res> {
  __$OrderRecordCopyWithImpl(this._self, this._then);

  final _OrderRecord _self;
  final $Res Function(_OrderRecord) _then;

/// Create a copy of OrderRecord
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? orderId = null,Object? date = null,Object? items = null,Object? amount = null,Object? status = null,Object? rating = null,Object? review = null,}) {
  return _then(_OrderRecord(
orderId: null == orderId ? _self.orderId : orderId // ignore: cast_nullable_to_non_nullable
as String,date: null == date ? _self.date : date // ignore: cast_nullable_to_non_nullable
as String,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as String,amount: null == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as int,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as OrderStatus,rating: null == rating ? _self.rating : rating // ignore: cast_nullable_to_non_nullable
as int,review: null == review ? _self.review : review // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
