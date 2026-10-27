// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'chat_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ChatMessage {

 String get id; String get senderId; String get receiverId; String get messageContent; String get messageType; DateTime get timestamp; String? get status; String? get conversationId; bool get deleted; bool get read; int get seqId; String? get senderName; String? get senderAvatar;
/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ChatMessageCopyWith<ChatMessage> get copyWith => _$ChatMessageCopyWithImpl<ChatMessage>(this as ChatMessage, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ChatMessage&&(identical(other.id, id) || other.id == id)&&(identical(other.senderId, senderId) || other.senderId == senderId)&&(identical(other.receiverId, receiverId) || other.receiverId == receiverId)&&(identical(other.messageContent, messageContent) || other.messageContent == messageContent)&&(identical(other.messageType, messageType) || other.messageType == messageType)&&(identical(other.timestamp, timestamp) || other.timestamp == timestamp)&&(identical(other.status, status) || other.status == status)&&(identical(other.conversationId, conversationId) || other.conversationId == conversationId)&&(identical(other.deleted, deleted) || other.deleted == deleted)&&(identical(other.read, read) || other.read == read)&&(identical(other.seqId, seqId) || other.seqId == seqId)&&(identical(other.senderName, senderName) || other.senderName == senderName)&&(identical(other.senderAvatar, senderAvatar) || other.senderAvatar == senderAvatar));
}


@override
int get hashCode => Object.hash(runtimeType,id,senderId,receiverId,messageContent,messageType,timestamp,status,conversationId,deleted,read,seqId,senderName,senderAvatar);

@override
String toString() {
  return 'ChatMessage(id: $id, senderId: $senderId, receiverId: $receiverId, messageContent: $messageContent, messageType: $messageType, timestamp: $timestamp, status: $status, conversationId: $conversationId, deleted: $deleted, read: $read, seqId: $seqId, senderName: $senderName, senderAvatar: $senderAvatar)';
}


}

/// @nodoc
abstract mixin class $ChatMessageCopyWith<$Res>  {
  factory $ChatMessageCopyWith(ChatMessage value, $Res Function(ChatMessage) _then) = _$ChatMessageCopyWithImpl;
@useResult
$Res call({
 String id, String senderId, String receiverId, String messageContent, String messageType, DateTime timestamp, String? status, String? conversationId, bool deleted, bool read, int seqId, String? senderName, String? senderAvatar
});




}
/// @nodoc
class _$ChatMessageCopyWithImpl<$Res>
    implements $ChatMessageCopyWith<$Res> {
  _$ChatMessageCopyWithImpl(this._self, this._then);

  final ChatMessage _self;
  final $Res Function(ChatMessage) _then;

/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? senderId = null,Object? receiverId = null,Object? messageContent = null,Object? messageType = null,Object? timestamp = null,Object? status = freezed,Object? conversationId = freezed,Object? deleted = null,Object? read = null,Object? seqId = null,Object? senderName = freezed,Object? senderAvatar = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,senderId: null == senderId ? _self.senderId : senderId // ignore: cast_nullable_to_non_nullable
as String,receiverId: null == receiverId ? _self.receiverId : receiverId // ignore: cast_nullable_to_non_nullable
as String,messageContent: null == messageContent ? _self.messageContent : messageContent // ignore: cast_nullable_to_non_nullable
as String,messageType: null == messageType ? _self.messageType : messageType // ignore: cast_nullable_to_non_nullable
as String,timestamp: null == timestamp ? _self.timestamp : timestamp // ignore: cast_nullable_to_non_nullable
as DateTime,status: freezed == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String?,conversationId: freezed == conversationId ? _self.conversationId : conversationId // ignore: cast_nullable_to_non_nullable
as String?,deleted: null == deleted ? _self.deleted : deleted // ignore: cast_nullable_to_non_nullable
as bool,read: null == read ? _self.read : read // ignore: cast_nullable_to_non_nullable
as bool,seqId: null == seqId ? _self.seqId : seqId // ignore: cast_nullable_to_non_nullable
as int,senderName: freezed == senderName ? _self.senderName : senderName // ignore: cast_nullable_to_non_nullable
as String?,senderAvatar: freezed == senderAvatar ? _self.senderAvatar : senderAvatar // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [ChatMessage].
extension ChatMessagePatterns on ChatMessage {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ChatMessage value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ChatMessage value)  $default,){
final _that = this;
switch (_that) {
case _ChatMessage():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ChatMessage value)?  $default,){
final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String senderId,  String receiverId,  String messageContent,  String messageType,  DateTime timestamp,  String? status,  String? conversationId,  bool deleted,  bool read,  int seqId,  String? senderName,  String? senderAvatar)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
return $default(_that.id,_that.senderId,_that.receiverId,_that.messageContent,_that.messageType,_that.timestamp,_that.status,_that.conversationId,_that.deleted,_that.read,_that.seqId,_that.senderName,_that.senderAvatar);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String senderId,  String receiverId,  String messageContent,  String messageType,  DateTime timestamp,  String? status,  String? conversationId,  bool deleted,  bool read,  int seqId,  String? senderName,  String? senderAvatar)  $default,) {final _that = this;
switch (_that) {
case _ChatMessage():
return $default(_that.id,_that.senderId,_that.receiverId,_that.messageContent,_that.messageType,_that.timestamp,_that.status,_that.conversationId,_that.deleted,_that.read,_that.seqId,_that.senderName,_that.senderAvatar);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String senderId,  String receiverId,  String messageContent,  String messageType,  DateTime timestamp,  String? status,  String? conversationId,  bool deleted,  bool read,  int seqId,  String? senderName,  String? senderAvatar)?  $default,) {final _that = this;
switch (_that) {
case _ChatMessage() when $default != null:
return $default(_that.id,_that.senderId,_that.receiverId,_that.messageContent,_that.messageType,_that.timestamp,_that.status,_that.conversationId,_that.deleted,_that.read,_that.seqId,_that.senderName,_that.senderAvatar);case _:
  return null;

}
}

}

/// @nodoc


class _ChatMessage extends ChatMessage {
  const _ChatMessage({required this.id, required this.senderId, required this.receiverId, required this.messageContent, required this.messageType, required this.timestamp, this.status, this.conversationId, this.deleted = false, this.read = false, this.seqId = 0, this.senderName, this.senderAvatar}): super._();
  

@override final  String id;
@override final  String senderId;
@override final  String receiverId;
@override final  String messageContent;
@override final  String messageType;
@override final  DateTime timestamp;
@override final  String? status;
@override final  String? conversationId;
@override@JsonKey() final  bool deleted;
@override@JsonKey() final  bool read;
@override@JsonKey() final  int seqId;
@override final  String? senderName;
@override final  String? senderAvatar;

/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ChatMessageCopyWith<_ChatMessage> get copyWith => __$ChatMessageCopyWithImpl<_ChatMessage>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ChatMessage&&(identical(other.id, id) || other.id == id)&&(identical(other.senderId, senderId) || other.senderId == senderId)&&(identical(other.receiverId, receiverId) || other.receiverId == receiverId)&&(identical(other.messageContent, messageContent) || other.messageContent == messageContent)&&(identical(other.messageType, messageType) || other.messageType == messageType)&&(identical(other.timestamp, timestamp) || other.timestamp == timestamp)&&(identical(other.status, status) || other.status == status)&&(identical(other.conversationId, conversationId) || other.conversationId == conversationId)&&(identical(other.deleted, deleted) || other.deleted == deleted)&&(identical(other.read, read) || other.read == read)&&(identical(other.seqId, seqId) || other.seqId == seqId)&&(identical(other.senderName, senderName) || other.senderName == senderName)&&(identical(other.senderAvatar, senderAvatar) || other.senderAvatar == senderAvatar));
}


@override
int get hashCode => Object.hash(runtimeType,id,senderId,receiverId,messageContent,messageType,timestamp,status,conversationId,deleted,read,seqId,senderName,senderAvatar);

@override
String toString() {
  return 'ChatMessage(id: $id, senderId: $senderId, receiverId: $receiverId, messageContent: $messageContent, messageType: $messageType, timestamp: $timestamp, status: $status, conversationId: $conversationId, deleted: $deleted, read: $read, seqId: $seqId, senderName: $senderName, senderAvatar: $senderAvatar)';
}


}

/// @nodoc
abstract mixin class _$ChatMessageCopyWith<$Res> implements $ChatMessageCopyWith<$Res> {
  factory _$ChatMessageCopyWith(_ChatMessage value, $Res Function(_ChatMessage) _then) = __$ChatMessageCopyWithImpl;
@override @useResult
$Res call({
 String id, String senderId, String receiverId, String messageContent, String messageType, DateTime timestamp, String? status, String? conversationId, bool deleted, bool read, int seqId, String? senderName, String? senderAvatar
});




}
/// @nodoc
class __$ChatMessageCopyWithImpl<$Res>
    implements _$ChatMessageCopyWith<$Res> {
  __$ChatMessageCopyWithImpl(this._self, this._then);

  final _ChatMessage _self;
  final $Res Function(_ChatMessage) _then;

/// Create a copy of ChatMessage
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? senderId = null,Object? receiverId = null,Object? messageContent = null,Object? messageType = null,Object? timestamp = null,Object? status = freezed,Object? conversationId = freezed,Object? deleted = null,Object? read = null,Object? seqId = null,Object? senderName = freezed,Object? senderAvatar = freezed,}) {
  return _then(_ChatMessage(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,senderId: null == senderId ? _self.senderId : senderId // ignore: cast_nullable_to_non_nullable
as String,receiverId: null == receiverId ? _self.receiverId : receiverId // ignore: cast_nullable_to_non_nullable
as String,messageContent: null == messageContent ? _self.messageContent : messageContent // ignore: cast_nullable_to_non_nullable
as String,messageType: null == messageType ? _self.messageType : messageType // ignore: cast_nullable_to_non_nullable
as String,timestamp: null == timestamp ? _self.timestamp : timestamp // ignore: cast_nullable_to_non_nullable
as DateTime,status: freezed == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String?,conversationId: freezed == conversationId ? _self.conversationId : conversationId // ignore: cast_nullable_to_non_nullable
as String?,deleted: null == deleted ? _self.deleted : deleted // ignore: cast_nullable_to_non_nullable
as bool,read: null == read ? _self.read : read // ignore: cast_nullable_to_non_nullable
as bool,seqId: null == seqId ? _self.seqId : seqId // ignore: cast_nullable_to_non_nullable
as int,senderName: freezed == senderName ? _self.senderName : senderName // ignore: cast_nullable_to_non_nullable
as String?,senderAvatar: freezed == senderAvatar ? _self.senderAvatar : senderAvatar // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
