// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'ai_message_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$AiOrderItem {

 String get orderId; String get status; int get amountTwd; int get createTimeMs; String get buyerName; String get itemsSummary;
/// Create a copy of AiOrderItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AiOrderItemCopyWith<AiOrderItem> get copyWith => _$AiOrderItemCopyWithImpl<AiOrderItem>(this as AiOrderItem, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AiOrderItem&&(identical(other.orderId, orderId) || other.orderId == orderId)&&(identical(other.status, status) || other.status == status)&&(identical(other.amountTwd, amountTwd) || other.amountTwd == amountTwd)&&(identical(other.createTimeMs, createTimeMs) || other.createTimeMs == createTimeMs)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.itemsSummary, itemsSummary) || other.itemsSummary == itemsSummary));
}


@override
int get hashCode => Object.hash(runtimeType,orderId,status,amountTwd,createTimeMs,buyerName,itemsSummary);

@override
String toString() {
  return 'AiOrderItem(orderId: $orderId, status: $status, amountTwd: $amountTwd, createTimeMs: $createTimeMs, buyerName: $buyerName, itemsSummary: $itemsSummary)';
}


}

/// @nodoc
abstract mixin class $AiOrderItemCopyWith<$Res>  {
  factory $AiOrderItemCopyWith(AiOrderItem value, $Res Function(AiOrderItem) _then) = _$AiOrderItemCopyWithImpl;
@useResult
$Res call({
 String orderId, String status, int amountTwd, int createTimeMs, String buyerName, String itemsSummary
});




}
/// @nodoc
class _$AiOrderItemCopyWithImpl<$Res>
    implements $AiOrderItemCopyWith<$Res> {
  _$AiOrderItemCopyWithImpl(this._self, this._then);

  final AiOrderItem _self;
  final $Res Function(AiOrderItem) _then;

/// Create a copy of AiOrderItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? orderId = null,Object? status = null,Object? amountTwd = null,Object? createTimeMs = null,Object? buyerName = null,Object? itemsSummary = null,}) {
  return _then(_self.copyWith(
orderId: null == orderId ? _self.orderId : orderId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,amountTwd: null == amountTwd ? _self.amountTwd : amountTwd // ignore: cast_nullable_to_non_nullable
as int,createTimeMs: null == createTimeMs ? _self.createTimeMs : createTimeMs // ignore: cast_nullable_to_non_nullable
as int,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,itemsSummary: null == itemsSummary ? _self.itemsSummary : itemsSummary // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AiOrderItem].
extension AiOrderItemPatterns on AiOrderItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AiOrderItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AiOrderItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AiOrderItem value)  $default,){
final _that = this;
switch (_that) {
case _AiOrderItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AiOrderItem value)?  $default,){
final _that = this;
switch (_that) {
case _AiOrderItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String orderId,  String status,  int amountTwd,  int createTimeMs,  String buyerName,  String itemsSummary)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AiOrderItem() when $default != null:
return $default(_that.orderId,_that.status,_that.amountTwd,_that.createTimeMs,_that.buyerName,_that.itemsSummary);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String orderId,  String status,  int amountTwd,  int createTimeMs,  String buyerName,  String itemsSummary)  $default,) {final _that = this;
switch (_that) {
case _AiOrderItem():
return $default(_that.orderId,_that.status,_that.amountTwd,_that.createTimeMs,_that.buyerName,_that.itemsSummary);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String orderId,  String status,  int amountTwd,  int createTimeMs,  String buyerName,  String itemsSummary)?  $default,) {final _that = this;
switch (_that) {
case _AiOrderItem() when $default != null:
return $default(_that.orderId,_that.status,_that.amountTwd,_that.createTimeMs,_that.buyerName,_that.itemsSummary);case _:
  return null;

}
}

}

/// @nodoc


class _AiOrderItem implements AiOrderItem {
  const _AiOrderItem({required this.orderId, required this.status, required this.amountTwd, this.createTimeMs = 0, this.buyerName = '', this.itemsSummary = ''});
  

@override final  String orderId;
@override final  String status;
@override final  int amountTwd;
@override@JsonKey() final  int createTimeMs;
@override@JsonKey() final  String buyerName;
@override@JsonKey() final  String itemsSummary;

/// Create a copy of AiOrderItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AiOrderItemCopyWith<_AiOrderItem> get copyWith => __$AiOrderItemCopyWithImpl<_AiOrderItem>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AiOrderItem&&(identical(other.orderId, orderId) || other.orderId == orderId)&&(identical(other.status, status) || other.status == status)&&(identical(other.amountTwd, amountTwd) || other.amountTwd == amountTwd)&&(identical(other.createTimeMs, createTimeMs) || other.createTimeMs == createTimeMs)&&(identical(other.buyerName, buyerName) || other.buyerName == buyerName)&&(identical(other.itemsSummary, itemsSummary) || other.itemsSummary == itemsSummary));
}


@override
int get hashCode => Object.hash(runtimeType,orderId,status,amountTwd,createTimeMs,buyerName,itemsSummary);

@override
String toString() {
  return 'AiOrderItem(orderId: $orderId, status: $status, amountTwd: $amountTwd, createTimeMs: $createTimeMs, buyerName: $buyerName, itemsSummary: $itemsSummary)';
}


}

/// @nodoc
abstract mixin class _$AiOrderItemCopyWith<$Res> implements $AiOrderItemCopyWith<$Res> {
  factory _$AiOrderItemCopyWith(_AiOrderItem value, $Res Function(_AiOrderItem) _then) = __$AiOrderItemCopyWithImpl;
@override @useResult
$Res call({
 String orderId, String status, int amountTwd, int createTimeMs, String buyerName, String itemsSummary
});




}
/// @nodoc
class __$AiOrderItemCopyWithImpl<$Res>
    implements _$AiOrderItemCopyWith<$Res> {
  __$AiOrderItemCopyWithImpl(this._self, this._then);

  final _AiOrderItem _self;
  final $Res Function(_AiOrderItem) _then;

/// Create a copy of AiOrderItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? orderId = null,Object? status = null,Object? amountTwd = null,Object? createTimeMs = null,Object? buyerName = null,Object? itemsSummary = null,}) {
  return _then(_AiOrderItem(
orderId: null == orderId ? _self.orderId : orderId // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,amountTwd: null == amountTwd ? _self.amountTwd : amountTwd // ignore: cast_nullable_to_non_nullable
as int,createTimeMs: null == createTimeMs ? _self.createTimeMs : createTimeMs // ignore: cast_nullable_to_non_nullable
as int,buyerName: null == buyerName ? _self.buyerName : buyerName // ignore: cast_nullable_to_non_nullable
as String,itemsSummary: null == itemsSummary ? _self.itemsSummary : itemsSummary // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$AiMessage {

 String get role; String get content; DateTime get timestamp; List<AiOrderItem>? get orderList;
/// Create a copy of AiMessage
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AiMessageCopyWith<AiMessage> get copyWith => _$AiMessageCopyWithImpl<AiMessage>(this as AiMessage, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AiMessage&&(identical(other.role, role) || other.role == role)&&(identical(other.content, content) || other.content == content)&&(identical(other.timestamp, timestamp) || other.timestamp == timestamp)&&const DeepCollectionEquality().equals(other.orderList, orderList));
}


@override
int get hashCode => Object.hash(runtimeType,role,content,timestamp,const DeepCollectionEquality().hash(orderList));

@override
String toString() {
  return 'AiMessage(role: $role, content: $content, timestamp: $timestamp, orderList: $orderList)';
}


}

/// @nodoc
abstract mixin class $AiMessageCopyWith<$Res>  {
  factory $AiMessageCopyWith(AiMessage value, $Res Function(AiMessage) _then) = _$AiMessageCopyWithImpl;
@useResult
$Res call({
 String role, String content, DateTime timestamp, List<AiOrderItem>? orderList
});




}
/// @nodoc
class _$AiMessageCopyWithImpl<$Res>
    implements $AiMessageCopyWith<$Res> {
  _$AiMessageCopyWithImpl(this._self, this._then);

  final AiMessage _self;
  final $Res Function(AiMessage) _then;

/// Create a copy of AiMessage
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? role = null,Object? content = null,Object? timestamp = null,Object? orderList = freezed,}) {
  return _then(_self.copyWith(
role: null == role ? _self.role : role // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,timestamp: null == timestamp ? _self.timestamp : timestamp // ignore: cast_nullable_to_non_nullable
as DateTime,orderList: freezed == orderList ? _self.orderList : orderList // ignore: cast_nullable_to_non_nullable
as List<AiOrderItem>?,
  ));
}

}


/// Adds pattern-matching-related methods to [AiMessage].
extension AiMessagePatterns on AiMessage {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AiMessage value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AiMessage() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AiMessage value)  $default,){
final _that = this;
switch (_that) {
case _AiMessage():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AiMessage value)?  $default,){
final _that = this;
switch (_that) {
case _AiMessage() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String role,  String content,  DateTime timestamp,  List<AiOrderItem>? orderList)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AiMessage() when $default != null:
return $default(_that.role,_that.content,_that.timestamp,_that.orderList);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String role,  String content,  DateTime timestamp,  List<AiOrderItem>? orderList)  $default,) {final _that = this;
switch (_that) {
case _AiMessage():
return $default(_that.role,_that.content,_that.timestamp,_that.orderList);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String role,  String content,  DateTime timestamp,  List<AiOrderItem>? orderList)?  $default,) {final _that = this;
switch (_that) {
case _AiMessage() when $default != null:
return $default(_that.role,_that.content,_that.timestamp,_that.orderList);case _:
  return null;

}
}

}

/// @nodoc


class _AiMessage implements AiMessage {
  const _AiMessage({required this.role, required this.content, required this.timestamp, final  List<AiOrderItem>? orderList}): _orderList = orderList;
  

@override final  String role;
@override final  String content;
@override final  DateTime timestamp;
 final  List<AiOrderItem>? _orderList;
@override List<AiOrderItem>? get orderList {
  final value = _orderList;
  if (value == null) return null;
  if (_orderList is EqualUnmodifiableListView) return _orderList;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(value);
}


/// Create a copy of AiMessage
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AiMessageCopyWith<_AiMessage> get copyWith => __$AiMessageCopyWithImpl<_AiMessage>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AiMessage&&(identical(other.role, role) || other.role == role)&&(identical(other.content, content) || other.content == content)&&(identical(other.timestamp, timestamp) || other.timestamp == timestamp)&&const DeepCollectionEquality().equals(other._orderList, _orderList));
}


@override
int get hashCode => Object.hash(runtimeType,role,content,timestamp,const DeepCollectionEquality().hash(_orderList));

@override
String toString() {
  return 'AiMessage(role: $role, content: $content, timestamp: $timestamp, orderList: $orderList)';
}


}

/// @nodoc
abstract mixin class _$AiMessageCopyWith<$Res> implements $AiMessageCopyWith<$Res> {
  factory _$AiMessageCopyWith(_AiMessage value, $Res Function(_AiMessage) _then) = __$AiMessageCopyWithImpl;
@override @useResult
$Res call({
 String role, String content, DateTime timestamp, List<AiOrderItem>? orderList
});




}
/// @nodoc
class __$AiMessageCopyWithImpl<$Res>
    implements _$AiMessageCopyWith<$Res> {
  __$AiMessageCopyWithImpl(this._self, this._then);

  final _AiMessage _self;
  final $Res Function(_AiMessage) _then;

/// Create a copy of AiMessage
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? role = null,Object? content = null,Object? timestamp = null,Object? orderList = freezed,}) {
  return _then(_AiMessage(
role: null == role ? _self.role : role // ignore: cast_nullable_to_non_nullable
as String,content: null == content ? _self.content : content // ignore: cast_nullable_to_non_nullable
as String,timestamp: null == timestamp ? _self.timestamp : timestamp // ignore: cast_nullable_to_non_nullable
as DateTime,orderList: freezed == orderList ? _self._orderList : orderList // ignore: cast_nullable_to_non_nullable
as List<AiOrderItem>?,
  ));
}


}

// dart format on
