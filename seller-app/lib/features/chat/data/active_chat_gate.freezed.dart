// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'active_chat_gate.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ActiveChat {

 String? get peerId;// seller 側 = buyer/customer accountId
 String? get conversationId;
/// Create a copy of ActiveChat
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ActiveChatCopyWith<ActiveChat> get copyWith => _$ActiveChatCopyWithImpl<ActiveChat>(this as ActiveChat, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ActiveChat&&(identical(other.peerId, peerId) || other.peerId == peerId)&&(identical(other.conversationId, conversationId) || other.conversationId == conversationId));
}


@override
int get hashCode => Object.hash(runtimeType,peerId,conversationId);

@override
String toString() {
  return 'ActiveChat(peerId: $peerId, conversationId: $conversationId)';
}


}

/// @nodoc
abstract mixin class $ActiveChatCopyWith<$Res>  {
  factory $ActiveChatCopyWith(ActiveChat value, $Res Function(ActiveChat) _then) = _$ActiveChatCopyWithImpl;
@useResult
$Res call({
 String? peerId, String? conversationId
});




}
/// @nodoc
class _$ActiveChatCopyWithImpl<$Res>
    implements $ActiveChatCopyWith<$Res> {
  _$ActiveChatCopyWithImpl(this._self, this._then);

  final ActiveChat _self;
  final $Res Function(ActiveChat) _then;

/// Create a copy of ActiveChat
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? peerId = freezed,Object? conversationId = freezed,}) {
  return _then(_self.copyWith(
peerId: freezed == peerId ? _self.peerId : peerId // ignore: cast_nullable_to_non_nullable
as String?,conversationId: freezed == conversationId ? _self.conversationId : conversationId // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [ActiveChat].
extension ActiveChatPatterns on ActiveChat {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ActiveChat value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ActiveChat() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ActiveChat value)  $default,){
final _that = this;
switch (_that) {
case _ActiveChat():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ActiveChat value)?  $default,){
final _that = this;
switch (_that) {
case _ActiveChat() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String? peerId,  String? conversationId)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ActiveChat() when $default != null:
return $default(_that.peerId,_that.conversationId);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String? peerId,  String? conversationId)  $default,) {final _that = this;
switch (_that) {
case _ActiveChat():
return $default(_that.peerId,_that.conversationId);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String? peerId,  String? conversationId)?  $default,) {final _that = this;
switch (_that) {
case _ActiveChat() when $default != null:
return $default(_that.peerId,_that.conversationId);case _:
  return null;

}
}

}

/// @nodoc


class _ActiveChat implements ActiveChat {
  const _ActiveChat({this.peerId, this.conversationId});
  

@override final  String? peerId;
// seller 側 = buyer/customer accountId
@override final  String? conversationId;

/// Create a copy of ActiveChat
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ActiveChatCopyWith<_ActiveChat> get copyWith => __$ActiveChatCopyWithImpl<_ActiveChat>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ActiveChat&&(identical(other.peerId, peerId) || other.peerId == peerId)&&(identical(other.conversationId, conversationId) || other.conversationId == conversationId));
}


@override
int get hashCode => Object.hash(runtimeType,peerId,conversationId);

@override
String toString() {
  return 'ActiveChat(peerId: $peerId, conversationId: $conversationId)';
}


}

/// @nodoc
abstract mixin class _$ActiveChatCopyWith<$Res> implements $ActiveChatCopyWith<$Res> {
  factory _$ActiveChatCopyWith(_ActiveChat value, $Res Function(_ActiveChat) _then) = __$ActiveChatCopyWithImpl;
@override @useResult
$Res call({
 String? peerId, String? conversationId
});




}
/// @nodoc
class __$ActiveChatCopyWithImpl<$Res>
    implements _$ActiveChatCopyWith<$Res> {
  __$ActiveChatCopyWithImpl(this._self, this._then);

  final _ActiveChat _self;
  final $Res Function(_ActiveChat) _then;

/// Create a copy of ActiveChat
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? peerId = freezed,Object? conversationId = freezed,}) {
  return _then(_ActiveChat(
peerId: freezed == peerId ? _self.peerId : peerId // ignore: cast_nullable_to_non_nullable
as String?,conversationId: freezed == conversationId ? _self.conversationId : conversationId // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
