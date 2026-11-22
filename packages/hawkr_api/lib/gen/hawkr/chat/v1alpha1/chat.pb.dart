// This is a generated file - do not edit.
//
// Generated from hawkr/chat/v1alpha1/chat.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../common/v1alpha1/image.pb.dart' as $0;
import '../../common/v1alpha1/store_card.pb.dart' as $2;
import 'chat.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'chat.pbenum.dart';

/// Message 是一則訊息。隱藏或抹除的訊息是墓碑：hidden 或 erased 為真，沒有 text 與 image。
class Message extends $pb.GeneratedMessage {
  factory Message({
    $core.String? id,
    $core.String? conversationId,
    $fixnum.Int64? seq,
    ChatParty? sender,
    MessageKind? kind,
    $core.String? text,
    $0.Image? image,
    $core.bool? hidden,
    $core.bool? erased,
    $1.Timestamp? createTime,
  }) {
    final result = Message._();
    if (id != null) result.id = id;
    if (conversationId != null) result.conversationId = conversationId;
    if (seq != null) result.seq = seq;
    if (sender != null) result.sender = sender;
    if (kind != null) result.kind = kind;
    if (text != null) result.text = text;
    if (image != null) result.image = image;
    if (hidden != null) result.hidden = hidden;
    if (erased != null) result.erased = erased;
    if (createTime != null) result.createTime = createTime;
    return result;
  }

  Message._();

  factory Message.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Message()..mergeFromBuffer(data, registry);
  factory Message.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Message()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Message',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: Message.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'conversationId')
    ..aInt64(3, _omitFieldNames ? '' : 'seq')
    ..aE<ChatParty>(4, _omitFieldNames ? '' : 'sender',
        enumValues: ChatParty.values)
    ..aE<MessageKind>(5, _omitFieldNames ? '' : 'kind',
        enumValues: MessageKind.values)
    ..aOS(6, _omitFieldNames ? '' : 'text')
    ..aOM<$0.Image>(7, _omitFieldNames ? '' : 'image',
        subBuilder: $0.Image.$_createMessage)
    ..aOB(8, _omitFieldNames ? '' : 'hidden')
    ..aOB(9, _omitFieldNames ? '' : 'erased')
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'createTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Message clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Message copyWith(void Function(Message) updates) =>
      super.copyWith((message) => updates(message as Message)) as Message;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Message() / Message.new instead')
  static Message create() => Message._();
  static $pb.GeneratedMessage $_createMessage() => Message._();
  @$core.override
  Message createEmptyInstance() => Message._();
  @$core.pragma('dart2js:noInline')
  static Message getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Message>(Message.$_createMessage);
  static Message? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get conversationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set conversationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConversationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearConversationId() => $_clearField(2);

  /// 訊息在對話裡的序號，從 1 起連號、沒有空洞；client 以序號偵測缺口並用 ListMessages 的 after_seq 補。
  @$pb.TagNumber(3)
  $fixnum.Int64 get seq => $_getI64(2);
  @$pb.TagNumber(3)
  set seq($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSeq() => $_has(2);
  @$pb.TagNumber(3)
  void clearSeq() => $_clearField(3);

  /// 寄件方；不回對方的帳號或業務 ID。
  @$pb.TagNumber(4)
  ChatParty get sender => $_getN(3);
  @$pb.TagNumber(4)
  set sender(ChatParty value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSender() => $_has(3);
  @$pb.TagNumber(4)
  void clearSender() => $_clearField(4);

  @$pb.TagNumber(5)
  MessageKind get kind => $_getN(4);
  @$pb.TagNumber(5)
  set kind(MessageKind value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasKind() => $_has(4);
  @$pb.TagNumber(5)
  void clearKind() => $_clearField(5);

  /// 文字內容；只有沒有被隱藏或抹除的文字訊息才有。
  @$pb.TagNumber(6)
  $core.String get text => $_getSZ(5);
  @$pb.TagNumber(6)
  set text($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasText() => $_has(5);
  @$pb.TagNumber(6)
  void clearText() => $_clearField(6);

  /// 圖片；只有沒有被隱藏或抹除的圖片訊息才有。
  @$pb.TagNumber(7)
  $0.Image get image => $_getN(6);
  @$pb.TagNumber(7)
  set image($0.Image value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasImage() => $_has(6);
  @$pb.TagNumber(7)
  void clearImage() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Image ensureImage() => $_ensure(6);

  /// 被平台隱藏。
  @$pb.TagNumber(8)
  $core.bool get hidden => $_getBF(7);
  @$pb.TagNumber(8)
  set hidden($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasHidden() => $_has(7);
  @$pb.TagNumber(8)
  void clearHidden() => $_clearField(8);

  /// 寄件方的帳號已刪除或店家已退場，內容已抹除。
  @$pb.TagNumber(9)
  $core.bool get erased => $_getBF(8);
  @$pb.TagNumber(9)
  set erased($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasErased() => $_has(8);
  @$pb.TagNumber(9)
  void clearErased() => $_clearField(9);

  @$pb.TagNumber(10)
  $1.Timestamp get createTime => $_getN(9);
  @$pb.TagNumber(10)
  set createTime($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCreateTime() => $_has(9);
  @$pb.TagNumber(10)
  void clearCreateTime() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureCreateTime() => $_ensure(9);
}

/// Preview 是對話列表上最後一則訊息的結構化預覽，不含顯示句子。
class Preview extends $pb.GeneratedMessage {
  factory Preview({
    MessageKind? kind,
    $core.String? text,
    $core.bool? hidden,
    $core.bool? erased,
    ChatParty? sender,
  }) {
    final result = Preview._();
    if (kind != null) result.kind = kind;
    if (text != null) result.text = text;
    if (hidden != null) result.hidden = hidden;
    if (erased != null) result.erased = erased;
    if (sender != null) result.sender = sender;
    return result;
  }

  Preview._();

  factory Preview.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Preview()..mergeFromBuffer(data, registry);
  factory Preview.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Preview()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Preview',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: Preview.$_createMessage)
    ..aE<MessageKind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: MessageKind.values)
    ..aOS(2, _omitFieldNames ? '' : 'text')
    ..aOB(3, _omitFieldNames ? '' : 'hidden')
    ..aOB(4, _omitFieldNames ? '' : 'erased')
    ..aE<ChatParty>(5, _omitFieldNames ? '' : 'sender',
        enumValues: ChatParty.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Preview clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Preview copyWith(void Function(Preview) updates) =>
      super.copyWith((message) => updates(message as Preview)) as Preview;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Preview() / Preview.new instead')
  static Preview create() => Preview._();
  static $pb.GeneratedMessage $_createMessage() => Preview._();
  @$core.override
  Preview createEmptyInstance() => Preview._();
  @$core.pragma('dart2js:noInline')
  static Preview getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Preview>(Preview.$_createMessage);
  static Preview? _defaultInstance;

  @$pb.TagNumber(1)
  MessageKind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(MessageKind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  /// 文字預覽（最多 100 個字元）；只有沒有被隱藏或抹除的文字訊息才有。
  @$pb.TagNumber(2)
  $core.String get text => $_getSZ(1);
  @$pb.TagNumber(2)
  set text($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasText() => $_has(1);
  @$pb.TagNumber(2)
  void clearText() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get hidden => $_getBF(2);
  @$pb.TagNumber(3)
  set hidden($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHidden() => $_has(2);
  @$pb.TagNumber(3)
  void clearHidden() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get erased => $_getBF(3);
  @$pb.TagNumber(4)
  set erased($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasErased() => $_has(3);
  @$pb.TagNumber(4)
  void clearErased() => $_clearField(4);

  @$pb.TagNumber(5)
  ChatParty get sender => $_getN(4);
  @$pb.TagNumber(5)
  set sender(ChatParty value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSender() => $_has(4);
  @$pb.TagNumber(5)
  void clearSender() => $_clearField(5);
}

/// ChatBuyer 是店家的對話裡的買家：名稱與頭像在讀取時由 buyer 補；帳號已刪除時 deleted 為真，沒有名稱與頭像。
class ChatBuyer extends $pb.GeneratedMessage {
  factory ChatBuyer({
    $core.String? id,
    $core.String? name,
    $0.Image? avatar,
    $core.bool? deleted,
  }) {
    final result = ChatBuyer._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (avatar != null) result.avatar = avatar;
    if (deleted != null) result.deleted = deleted;
    return result;
  }

  ChatBuyer._();

  factory ChatBuyer.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ChatBuyer()..mergeFromBuffer(data, registry);
  factory ChatBuyer.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ChatBuyer()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatBuyer',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ChatBuyer.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOM<$0.Image>(3, _omitFieldNames ? '' : 'avatar',
        subBuilder: $0.Image.$_createMessage)
    ..aOB(4, _omitFieldNames ? '' : 'deleted')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatBuyer clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatBuyer copyWith(void Function(ChatBuyer) updates) =>
      super.copyWith((message) => updates(message as ChatBuyer)) as ChatBuyer;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ChatBuyer() / ChatBuyer.new instead')
  static ChatBuyer create() => ChatBuyer._();
  static $pb.GeneratedMessage $_createMessage() => ChatBuyer._();
  @$core.override
  ChatBuyer createEmptyInstance() => ChatBuyer._();
  @$core.pragma('dart2js:noInline')
  static ChatBuyer getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatBuyer>(ChatBuyer.$_createMessage);
  static ChatBuyer? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 買家的暱稱；沒有設定或已刪除時不設。
  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $0.Image get avatar => $_getN(2);
  @$pb.TagNumber(3)
  set avatar($0.Image value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasAvatar() => $_has(2);
  @$pb.TagNumber(3)
  void clearAvatar() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.Image ensureAvatar() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.bool get deleted => $_getBF(3);
  @$pb.TagNumber(4)
  set deleted($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeleted() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeleted() => $_clearField(4);
}

/// Conversation 是對話列表的一列。買家版（ListConversations）設 store，店家版（ListStoreConversations）設 buyer。
class Conversation extends $pb.GeneratedMessage {
  factory Conversation({
    $core.String? id,
    $2.StoreCard? store,
    ChatBuyer? buyer,
    $fixnum.Int64? unreadCount,
    $1.Timestamp? lastMessageTime,
    Preview? preview,
    $fixnum.Int64? peerReadSeq,
  }) {
    final result = Conversation._();
    if (id != null) result.id = id;
    if (store != null) result.store = store;
    if (buyer != null) result.buyer = buyer;
    if (unreadCount != null) result.unreadCount = unreadCount;
    if (lastMessageTime != null) result.lastMessageTime = lastMessageTime;
    if (preview != null) result.preview = preview;
    if (peerReadSeq != null) result.peerReadSeq = peerReadSeq;
    return result;
  }

  Conversation._();

  factory Conversation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Conversation()..mergeFromBuffer(data, registry);
  factory Conversation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Conversation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Conversation',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: Conversation.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<$2.StoreCard>(2, _omitFieldNames ? '' : 'store',
        subBuilder: $2.StoreCard.$_createMessage)
    ..aOM<ChatBuyer>(3, _omitFieldNames ? '' : 'buyer',
        subBuilder: ChatBuyer.$_createMessage)
    ..aInt64(4, _omitFieldNames ? '' : 'unreadCount')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'lastMessageTime',
        subBuilder: $1.Timestamp.$_createMessage)
    ..aOM<Preview>(6, _omitFieldNames ? '' : 'preview',
        subBuilder: Preview.$_createMessage)
    ..aInt64(7, _omitFieldNames ? '' : 'peerReadSeq')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Conversation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Conversation copyWith(void Function(Conversation) updates) =>
      super.copyWith((message) => updates(message as Conversation))
          as Conversation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Conversation() / Conversation.new instead')
  static Conversation create() => Conversation._();
  static $pb.GeneratedMessage $_createMessage() => Conversation._();
  @$core.override
  Conversation createEmptyInstance() => Conversation._();
  @$core.pragma('dart2js:noInline')
  static Conversation getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Conversation>(
          Conversation.$_createMessage);
  static Conversation? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 對方的店家卡片；只有買家版有。店家從沒公開過時不設（同 StoreCard 的可見性規則），client 顯示「店家」。
  @$pb.TagNumber(2)
  $2.StoreCard get store => $_getN(1);
  @$pb.TagNumber(2)
  set store($2.StoreCard value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasStore() => $_has(1);
  @$pb.TagNumber(2)
  void clearStore() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.StoreCard ensureStore() => $_ensure(1);

  /// 對方的買家；只有店家版有。
  @$pb.TagNumber(3)
  ChatBuyer get buyer => $_getN(2);
  @$pb.TagNumber(3)
  set buyer(ChatBuyer value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasBuyer() => $_has(2);
  @$pb.TagNumber(3)
  void clearBuyer() => $_clearField(3);
  @$pb.TagNumber(3)
  ChatBuyer ensureBuyer() => $_ensure(2);

  /// 呼叫者這一方的未讀訊息數。
  @$pb.TagNumber(4)
  $fixnum.Int64 get unreadCount => $_getI64(3);
  @$pb.TagNumber(4)
  set unreadCount($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUnreadCount() => $_has(3);
  @$pb.TagNumber(4)
  void clearUnreadCount() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get lastMessageTime => $_getN(4);
  @$pb.TagNumber(5)
  set lastMessageTime($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLastMessageTime() => $_has(4);
  @$pb.TagNumber(5)
  void clearLastMessageTime() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureLastMessageTime() => $_ensure(4);

  @$pb.TagNumber(6)
  Preview get preview => $_getN(5);
  @$pb.TagNumber(6)
  set preview(Preview value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasPreview() => $_has(5);
  @$pb.TagNumber(6)
  void clearPreview() => $_clearField(6);
  @$pb.TagNumber(6)
  Preview ensurePreview() => $_ensure(5);

  /// 對方的已讀游標：seq 不大於它的訊息，對方已讀。
  @$pb.TagNumber(7)
  $fixnum.Int64 get peerReadSeq => $_getI64(6);
  @$pb.TagNumber(7)
  set peerReadSeq($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasPeerReadSeq() => $_has(6);
  @$pb.TagNumber(7)
  void clearPeerReadSeq() => $_clearField(7);
}

enum MessageContent_Content { text, imageStagedToken, notSet }

/// MessageContent 是送出的內容，text 與 image_staged_token 恰好設一個。
class MessageContent extends $pb.GeneratedMessage {
  factory MessageContent({
    $core.String? text,
    $core.String? imageStagedToken,
  }) {
    final result = MessageContent._();
    if (text != null) result.text = text;
    if (imageStagedToken != null) result.imageStagedToken = imageStagedToken;
    return result;
  }

  MessageContent._();

  factory MessageContent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MessageContent()..mergeFromBuffer(data, registry);
  factory MessageContent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MessageContent()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, MessageContent_Content>
      _MessageContent_ContentByTag = {
    1: MessageContent_Content.text,
    2: MessageContent_Content.imageStagedToken,
    0: MessageContent_Content.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageContent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MessageContent.$_createMessage)
    ..oo(0, [1, 2])
    ..aOS(1, _omitFieldNames ? '' : 'text')
    ..aOS(2, _omitFieldNames ? '' : 'imageStagedToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageContent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageContent copyWith(void Function(MessageContent) updates) =>
      super.copyWith((message) => updates(message as MessageContent))
          as MessageContent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MessageContent() / MessageContent.new instead')
  static MessageContent create() => MessageContent._();
  static $pb.GeneratedMessage $_createMessage() => MessageContent._();
  @$core.override
  MessageContent createEmptyInstance() => MessageContent._();
  @$core.pragma('dart2js:noInline')
  static MessageContent getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MessageContent>(
          MessageContent.$_createMessage);
  static MessageContent? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  MessageContent_Content whichContent() =>
      _MessageContent_ContentByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearContent() => $_clearField($_whichOneof(0));

  /// 文字，去除首尾空白後 1 到 4096 個字元（以 Unicode 字元計）。
  @$pb.TagNumber(1)
  $core.String get text => $_getSZ(0);
  @$pb.TagNumber(1)
  set text($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);

  /// 圖片的 staged token（media 上傳完成後拿到的）；token 只能用一次。
  @$pb.TagNumber(2)
  $core.String get imageStagedToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set imageStagedToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasImageStagedToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearImageStagedToken() => $_clearField(2);
}

class ListConversationsRequest extends $pb.GeneratedMessage {
  factory ListConversationsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListConversationsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListConversationsRequest._();

  factory ListConversationsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsRequest()..mergeFromBuffer(data, registry);
  factory ListConversationsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListConversationsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ListConversationsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsRequest copyWith(
          void Function(ListConversationsRequest) updates) =>
      super.copyWith((message) => updates(message as ListConversationsRequest))
          as ListConversationsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListConversationsRequest() / ListConversationsRequest.new instead')
  static ListConversationsRequest create() => ListConversationsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListConversationsRequest._();
  @$core.override
  ListConversationsRequest createEmptyInstance() =>
      ListConversationsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListConversationsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListConversationsRequest>(
          ListConversationsRequest.$_createMessage);
  static ListConversationsRequest? _defaultInstance;

  /// 預設 20，上限 100。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListConversationsResponse extends $pb.GeneratedMessage {
  factory ListConversationsResponse({
    $core.Iterable<Conversation>? conversations,
    $core.String? nextPageToken,
  }) {
    final result = ListConversationsResponse._();
    if (conversations != null) result.conversations.addAll(conversations);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListConversationsResponse._();

  factory ListConversationsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsResponse()..mergeFromBuffer(data, registry);
  factory ListConversationsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListConversationsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListConversationsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ListConversationsResponse.$_createMessage)
    ..pPM<Conversation>(1, _omitFieldNames ? '' : 'conversations',
        subBuilder: Conversation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListConversationsResponse copyWith(
          void Function(ListConversationsResponse) updates) =>
      super.copyWith((message) => updates(message as ListConversationsResponse))
          as ListConversationsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListConversationsResponse() / ListConversationsResponse.new instead')
  static ListConversationsResponse create() => ListConversationsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListConversationsResponse._();
  @$core.override
  ListConversationsResponse createEmptyInstance() =>
      ListConversationsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListConversationsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListConversationsResponse>(
          ListConversationsResponse.$_createMessage);
  static ListConversationsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Conversation> get conversations => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListStoreConversationsRequest extends $pb.GeneratedMessage {
  factory ListStoreConversationsRequest({
    $core.int? pageSize,
    $core.String? pageToken,
  }) {
    final result = ListStoreConversationsRequest._();
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    return result;
  }

  ListStoreConversationsRequest._();

  factory ListStoreConversationsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreConversationsRequest()..mergeFromBuffer(data, registry);
  factory ListStoreConversationsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreConversationsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreConversationsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ListStoreConversationsRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pageSize')
    ..aOS(2, _omitFieldNames ? '' : 'pageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreConversationsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreConversationsRequest copyWith(
          void Function(ListStoreConversationsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListStoreConversationsRequest))
          as ListStoreConversationsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreConversationsRequest() / ListStoreConversationsRequest.new instead')
  static ListStoreConversationsRequest create() =>
      ListStoreConversationsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListStoreConversationsRequest._();
  @$core.override
  ListStoreConversationsRequest createEmptyInstance() =>
      ListStoreConversationsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListStoreConversationsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreConversationsRequest>(
          ListStoreConversationsRequest.$_createMessage);
  static ListStoreConversationsRequest? _defaultInstance;

  /// 預設 20，上限 100。
  @$pb.TagNumber(1)
  $core.int get pageSize => $_getIZ(0);
  @$pb.TagNumber(1)
  set pageSize($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPageSize() => $_has(0);
  @$pb.TagNumber(1)
  void clearPageSize() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get pageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set pageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageToken() => $_clearField(2);
}

class ListStoreConversationsResponse extends $pb.GeneratedMessage {
  factory ListStoreConversationsResponse({
    $core.Iterable<Conversation>? conversations,
    $core.String? nextPageToken,
  }) {
    final result = ListStoreConversationsResponse._();
    if (conversations != null) result.conversations.addAll(conversations);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  ListStoreConversationsResponse._();

  factory ListStoreConversationsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreConversationsResponse()..mergeFromBuffer(data, registry);
  factory ListStoreConversationsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListStoreConversationsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListStoreConversationsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ListStoreConversationsResponse.$_createMessage)
    ..pPM<Conversation>(1, _omitFieldNames ? '' : 'conversations',
        subBuilder: Conversation.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreConversationsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListStoreConversationsResponse copyWith(
          void Function(ListStoreConversationsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListStoreConversationsResponse))
          as ListStoreConversationsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListStoreConversationsResponse() / ListStoreConversationsResponse.new instead')
  static ListStoreConversationsResponse create() =>
      ListStoreConversationsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListStoreConversationsResponse._();
  @$core.override
  ListStoreConversationsResponse createEmptyInstance() =>
      ListStoreConversationsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListStoreConversationsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListStoreConversationsResponse>(
          ListStoreConversationsResponse.$_createMessage);
  static ListStoreConversationsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Conversation> get conversations => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class ListMessagesRequest extends $pb.GeneratedMessage {
  factory ListMessagesRequest({
    $core.String? conversationId,
    $core.int? pageSize,
    $fixnum.Int64? beforeSeq,
    $fixnum.Int64? afterSeq,
  }) {
    final result = ListMessagesRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    if (pageSize != null) result.pageSize = pageSize;
    if (beforeSeq != null) result.beforeSeq = beforeSeq;
    if (afterSeq != null) result.afterSeq = afterSeq;
    return result;
  }

  ListMessagesRequest._();

  factory ListMessagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMessagesRequest()..mergeFromBuffer(data, registry);
  factory ListMessagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMessagesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMessagesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ListMessagesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aInt64(3, _omitFieldNames ? '' : 'beforeSeq')
    ..aInt64(4, _omitFieldNames ? '' : 'afterSeq')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMessagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMessagesRequest copyWith(void Function(ListMessagesRequest) updates) =>
      super.copyWith((message) => updates(message as ListMessagesRequest))
          as ListMessagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListMessagesRequest() / ListMessagesRequest.new instead')
  static ListMessagesRequest create() => ListMessagesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMessagesRequest._();
  @$core.override
  ListMessagesRequest createEmptyInstance() => ListMessagesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMessagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMessagesRequest>(
          ListMessagesRequest.$_createMessage);
  static ListMessagesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  /// 預設 20，上限 100。
  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  /// 只列 seq 小於它的訊息（新的在前）；不設是從最新開始。不能與 after_seq 同時給。
  @$pb.TagNumber(3)
  $fixnum.Int64 get beforeSeq => $_getI64(2);
  @$pb.TagNumber(3)
  set beforeSeq($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBeforeSeq() => $_has(2);
  @$pb.TagNumber(3)
  void clearBeforeSeq() => $_clearField(3);

  /// 只列 seq 大於它的訊息（舊的在前），給斷線補洞。不能與 before_seq 同時給。
  @$pb.TagNumber(4)
  $fixnum.Int64 get afterSeq => $_getI64(3);
  @$pb.TagNumber(4)
  set afterSeq($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAfterSeq() => $_has(3);
  @$pb.TagNumber(4)
  void clearAfterSeq() => $_clearField(4);
}

class ListMessagesResponse extends $pb.GeneratedMessage {
  factory ListMessagesResponse({
    $core.Iterable<Message>? messages,
    $core.bool? hasMore,
    $fixnum.Int64? peerReadSeq,
  }) {
    final result = ListMessagesResponse._();
    if (messages != null) result.messages.addAll(messages);
    if (hasMore != null) result.hasMore = hasMore;
    if (peerReadSeq != null) result.peerReadSeq = peerReadSeq;
    return result;
  }

  ListMessagesResponse._();

  factory ListMessagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMessagesResponse()..mergeFromBuffer(data, registry);
  factory ListMessagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMessagesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMessagesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ListMessagesResponse.$_createMessage)
    ..pPM<Message>(1, _omitFieldNames ? '' : 'messages',
        subBuilder: Message.$_createMessage)
    ..aOB(2, _omitFieldNames ? '' : 'hasMore')
    ..aInt64(3, _omitFieldNames ? '' : 'peerReadSeq')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMessagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMessagesResponse copyWith(void Function(ListMessagesResponse) updates) =>
      super.copyWith((message) => updates(message as ListMessagesResponse))
          as ListMessagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMessagesResponse() / ListMessagesResponse.new instead')
  static ListMessagesResponse create() => ListMessagesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMessagesResponse._();
  @$core.override
  ListMessagesResponse createEmptyInstance() => ListMessagesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMessagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMessagesResponse>(
          ListMessagesResponse.$_createMessage);
  static ListMessagesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Message> get messages => $_getList(0);

  /// 為真表示在這一頁之外還有訊息：往舊的方向用最小的 seq 當 before_seq，補洞用最大的 seq 當 after_seq 繼續。
  @$pb.TagNumber(2)
  $core.bool get hasMore => $_getBF(1);
  @$pb.TagNumber(2)
  set hasMore($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHasMore() => $_has(1);
  @$pb.TagNumber(2)
  void clearHasMore() => $_clearField(2);

  /// 讀取當下對方在這個對話的已讀游標。
  @$pb.TagNumber(3)
  $fixnum.Int64 get peerReadSeq => $_getI64(2);
  @$pb.TagNumber(3)
  set peerReadSeq($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPeerReadSeq() => $_has(2);
  @$pb.TagNumber(3)
  void clearPeerReadSeq() => $_clearField(3);
}

class SendMessageRequest extends $pb.GeneratedMessage {
  factory SendMessageRequest({
    $core.String? conversationId,
    $core.String? requestId,
    MessageContent? content,
  }) {
    final result = SendMessageRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    if (requestId != null) result.requestId = requestId;
    if (content != null) result.content = content;
    return result;
  }

  SendMessageRequest._();

  factory SendMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageRequest()..mergeFromBuffer(data, registry);
  factory SendMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: SendMessageRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<MessageContent>(3, _omitFieldNames ? '' : 'content',
        subBuilder: MessageContent.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageRequest copyWith(void Function(SendMessageRequest) updates) =>
      super.copyWith((message) => updates(message as SendMessageRequest))
          as SendMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SendMessageRequest() / SendMessageRequest.new instead')
  static SendMessageRequest create() => SendMessageRequest._();
  static $pb.GeneratedMessage $_createMessage() => SendMessageRequest._();
  @$core.override
  SendMessageRequest createEmptyInstance() => SendMessageRequest._();
  @$core.pragma('dart2js:noInline')
  static SendMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageRequest>(
          SendMessageRequest.$_createMessage);
  static SendMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  /// UUIDv4（AIP-155），必填：同一個對話、同一個寄件方以同一個 request_id 重送回第一次的訊息；內容不同回 invalid_argument。
  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  MessageContent get content => $_getN(2);
  @$pb.TagNumber(3)
  set content(MessageContent value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasContent() => $_has(2);
  @$pb.TagNumber(3)
  void clearContent() => $_clearField(3);
  @$pb.TagNumber(3)
  MessageContent ensureContent() => $_ensure(2);
}

class SendMessageResponse extends $pb.GeneratedMessage {
  factory SendMessageResponse({
    Message? message,
  }) {
    final result = SendMessageResponse._();
    if (message != null) result.message = message;
    return result;
  }

  SendMessageResponse._();

  factory SendMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageResponse()..mergeFromBuffer(data, registry);
  factory SendMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: SendMessageResponse.$_createMessage)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageResponse copyWith(void Function(SendMessageResponse) updates) =>
      super.copyWith((message) => updates(message as SendMessageResponse))
          as SendMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use SendMessageResponse() / SendMessageResponse.new instead')
  static SendMessageResponse create() => SendMessageResponse._();
  static $pb.GeneratedMessage $_createMessage() => SendMessageResponse._();
  @$core.override
  SendMessageResponse createEmptyInstance() => SendMessageResponse._();
  @$core.pragma('dart2js:noInline')
  static SendMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageResponse>(
          SendMessageResponse.$_createMessage);
  static SendMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class SendMessageToStoreRequest extends $pb.GeneratedMessage {
  factory SendMessageToStoreRequest({
    $core.String? storeId,
    $core.String? requestId,
    MessageContent? content,
  }) {
    final result = SendMessageToStoreRequest._();
    if (storeId != null) result.storeId = storeId;
    if (requestId != null) result.requestId = requestId;
    if (content != null) result.content = content;
    return result;
  }

  SendMessageToStoreRequest._();

  factory SendMessageToStoreRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToStoreRequest()..mergeFromBuffer(data, registry);
  factory SendMessageToStoreRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToStoreRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageToStoreRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: SendMessageToStoreRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'storeId')
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<MessageContent>(3, _omitFieldNames ? '' : 'content',
        subBuilder: MessageContent.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToStoreRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToStoreRequest copyWith(
          void Function(SendMessageToStoreRequest) updates) =>
      super.copyWith((message) => updates(message as SendMessageToStoreRequest))
          as SendMessageToStoreRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SendMessageToStoreRequest() / SendMessageToStoreRequest.new instead')
  static SendMessageToStoreRequest create() => SendMessageToStoreRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SendMessageToStoreRequest._();
  @$core.override
  SendMessageToStoreRequest createEmptyInstance() =>
      SendMessageToStoreRequest._();
  @$core.pragma('dart2js:noInline')
  static SendMessageToStoreRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageToStoreRequest>(
          SendMessageToStoreRequest.$_createMessage);
  static SendMessageToStoreRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoreId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreId() => $_clearField(1);

  /// 同 SendMessageRequest.request_id；冪等的範圍是（對話, 寄件方, request_id）。
  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  MessageContent get content => $_getN(2);
  @$pb.TagNumber(3)
  set content(MessageContent value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasContent() => $_has(2);
  @$pb.TagNumber(3)
  void clearContent() => $_clearField(3);
  @$pb.TagNumber(3)
  MessageContent ensureContent() => $_ensure(2);
}

class SendMessageToStoreResponse extends $pb.GeneratedMessage {
  factory SendMessageToStoreResponse({
    Message? message,
  }) {
    final result = SendMessageToStoreResponse._();
    if (message != null) result.message = message;
    return result;
  }

  SendMessageToStoreResponse._();

  factory SendMessageToStoreResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToStoreResponse()..mergeFromBuffer(data, registry);
  factory SendMessageToStoreResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToStoreResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageToStoreResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: SendMessageToStoreResponse.$_createMessage)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToStoreResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToStoreResponse copyWith(
          void Function(SendMessageToStoreResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SendMessageToStoreResponse))
          as SendMessageToStoreResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SendMessageToStoreResponse() / SendMessageToStoreResponse.new instead')
  static SendMessageToStoreResponse create() => SendMessageToStoreResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SendMessageToStoreResponse._();
  @$core.override
  SendMessageToStoreResponse createEmptyInstance() =>
      SendMessageToStoreResponse._();
  @$core.pragma('dart2js:noInline')
  static SendMessageToStoreResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageToStoreResponse>(
          SendMessageToStoreResponse.$_createMessage);
  static SendMessageToStoreResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class SendMessageToBuyerRequest extends $pb.GeneratedMessage {
  factory SendMessageToBuyerRequest({
    $core.String? buyerId,
    $core.String? requestId,
    MessageContent? content,
  }) {
    final result = SendMessageToBuyerRequest._();
    if (buyerId != null) result.buyerId = buyerId;
    if (requestId != null) result.requestId = requestId;
    if (content != null) result.content = content;
    return result;
  }

  SendMessageToBuyerRequest._();

  factory SendMessageToBuyerRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToBuyerRequest()..mergeFromBuffer(data, registry);
  factory SendMessageToBuyerRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToBuyerRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageToBuyerRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: SendMessageToBuyerRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'buyerId')
    ..aOS(2, _omitFieldNames ? '' : 'requestId')
    ..aOM<MessageContent>(3, _omitFieldNames ? '' : 'content',
        subBuilder: MessageContent.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToBuyerRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToBuyerRequest copyWith(
          void Function(SendMessageToBuyerRequest) updates) =>
      super.copyWith((message) => updates(message as SendMessageToBuyerRequest))
          as SendMessageToBuyerRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SendMessageToBuyerRequest() / SendMessageToBuyerRequest.new instead')
  static SendMessageToBuyerRequest create() => SendMessageToBuyerRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SendMessageToBuyerRequest._();
  @$core.override
  SendMessageToBuyerRequest createEmptyInstance() =>
      SendMessageToBuyerRequest._();
  @$core.pragma('dart2js:noInline')
  static SendMessageToBuyerRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageToBuyerRequest>(
          SendMessageToBuyerRequest.$_createMessage);
  static SendMessageToBuyerRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get buyerId => $_getSZ(0);
  @$pb.TagNumber(1)
  set buyerId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBuyerId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBuyerId() => $_clearField(1);

  /// 同 SendMessageRequest.request_id；冪等的範圍是（對話, 寄件方, request_id）。
  @$pb.TagNumber(2)
  $core.String get requestId => $_getSZ(1);
  @$pb.TagNumber(2)
  set requestId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRequestId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRequestId() => $_clearField(2);

  @$pb.TagNumber(3)
  MessageContent get content => $_getN(2);
  @$pb.TagNumber(3)
  set content(MessageContent value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasContent() => $_has(2);
  @$pb.TagNumber(3)
  void clearContent() => $_clearField(3);
  @$pb.TagNumber(3)
  MessageContent ensureContent() => $_ensure(2);
}

class SendMessageToBuyerResponse extends $pb.GeneratedMessage {
  factory SendMessageToBuyerResponse({
    Message? message,
  }) {
    final result = SendMessageToBuyerResponse._();
    if (message != null) result.message = message;
    return result;
  }

  SendMessageToBuyerResponse._();

  factory SendMessageToBuyerResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToBuyerResponse()..mergeFromBuffer(data, registry);
  factory SendMessageToBuyerResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SendMessageToBuyerResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendMessageToBuyerResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: SendMessageToBuyerResponse.$_createMessage)
    ..aOM<Message>(1, _omitFieldNames ? '' : 'message',
        subBuilder: Message.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToBuyerResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendMessageToBuyerResponse copyWith(
          void Function(SendMessageToBuyerResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SendMessageToBuyerResponse))
          as SendMessageToBuyerResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SendMessageToBuyerResponse() / SendMessageToBuyerResponse.new instead')
  static SendMessageToBuyerResponse create() => SendMessageToBuyerResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SendMessageToBuyerResponse._();
  @$core.override
  SendMessageToBuyerResponse createEmptyInstance() =>
      SendMessageToBuyerResponse._();
  @$core.pragma('dart2js:noInline')
  static SendMessageToBuyerResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendMessageToBuyerResponse>(
          SendMessageToBuyerResponse.$_createMessage);
  static SendMessageToBuyerResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Message get message => $_getN(0);
  @$pb.TagNumber(1)
  set message(Message value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMessage() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessage() => $_clearField(1);
  @$pb.TagNumber(1)
  Message ensureMessage() => $_ensure(0);
}

class MarkConversationReadRequest extends $pb.GeneratedMessage {
  factory MarkConversationReadRequest({
    $core.String? conversationId,
    $fixnum.Int64? uptoSeq,
  }) {
    final result = MarkConversationReadRequest._();
    if (conversationId != null) result.conversationId = conversationId;
    if (uptoSeq != null) result.uptoSeq = uptoSeq;
    return result;
  }

  MarkConversationReadRequest._();

  factory MarkConversationReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkConversationReadRequest()..mergeFromBuffer(data, registry);
  factory MarkConversationReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkConversationReadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkConversationReadRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MarkConversationReadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aInt64(2, _omitFieldNames ? '' : 'uptoSeq')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkConversationReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkConversationReadRequest copyWith(
          void Function(MarkConversationReadRequest) updates) =>
      super.copyWith(
              (message) => updates(message as MarkConversationReadRequest))
          as MarkConversationReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkConversationReadRequest() / MarkConversationReadRequest.new instead')
  static MarkConversationReadRequest create() =>
      MarkConversationReadRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      MarkConversationReadRequest._();
  @$core.override
  MarkConversationReadRequest createEmptyInstance() =>
      MarkConversationReadRequest._();
  @$core.pragma('dart2js:noInline')
  static MarkConversationReadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkConversationReadRequest>(
          MarkConversationReadRequest.$_createMessage);
  static MarkConversationReadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  /// 已讀到的 seq，不小於 0。
  @$pb.TagNumber(2)
  $fixnum.Int64 get uptoSeq => $_getI64(1);
  @$pb.TagNumber(2)
  set uptoSeq($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUptoSeq() => $_has(1);
  @$pb.TagNumber(2)
  void clearUptoSeq() => $_clearField(2);
}

class MarkConversationReadResponse extends $pb.GeneratedMessage {
  factory MarkConversationReadResponse() => MarkConversationReadResponse._();

  MarkConversationReadResponse._();

  factory MarkConversationReadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkConversationReadResponse()..mergeFromBuffer(data, registry);
  factory MarkConversationReadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkConversationReadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkConversationReadResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MarkConversationReadResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkConversationReadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkConversationReadResponse copyWith(
          void Function(MarkConversationReadResponse) updates) =>
      super.copyWith(
              (message) => updates(message as MarkConversationReadResponse))
          as MarkConversationReadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkConversationReadResponse() / MarkConversationReadResponse.new instead')
  static MarkConversationReadResponse create() =>
      MarkConversationReadResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      MarkConversationReadResponse._();
  @$core.override
  MarkConversationReadResponse createEmptyInstance() =>
      MarkConversationReadResponse._();
  @$core.pragma('dart2js:noInline')
  static MarkConversationReadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkConversationReadResponse>(
          MarkConversationReadResponse.$_createMessage);
  static MarkConversationReadResponse? _defaultInstance;
}

class MarkAllReadRequest extends $pb.GeneratedMessage {
  factory MarkAllReadRequest() => MarkAllReadRequest._();

  MarkAllReadRequest._();

  factory MarkAllReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllReadRequest()..mergeFromBuffer(data, registry);
  factory MarkAllReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllReadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkAllReadRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MarkAllReadRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllReadRequest copyWith(void Function(MarkAllReadRequest) updates) =>
      super.copyWith((message) => updates(message as MarkAllReadRequest))
          as MarkAllReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MarkAllReadRequest() / MarkAllReadRequest.new instead')
  static MarkAllReadRequest create() => MarkAllReadRequest._();
  static $pb.GeneratedMessage $_createMessage() => MarkAllReadRequest._();
  @$core.override
  MarkAllReadRequest createEmptyInstance() => MarkAllReadRequest._();
  @$core.pragma('dart2js:noInline')
  static MarkAllReadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkAllReadRequest>(
          MarkAllReadRequest.$_createMessage);
  static MarkAllReadRequest? _defaultInstance;
}

class MarkAllReadResponse extends $pb.GeneratedMessage {
  factory MarkAllReadResponse() => MarkAllReadResponse._();

  MarkAllReadResponse._();

  factory MarkAllReadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllReadResponse()..mergeFromBuffer(data, registry);
  factory MarkAllReadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllReadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkAllReadResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MarkAllReadResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllReadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllReadResponse copyWith(void Function(MarkAllReadResponse) updates) =>
      super.copyWith((message) => updates(message as MarkAllReadResponse))
          as MarkAllReadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use MarkAllReadResponse() / MarkAllReadResponse.new instead')
  static MarkAllReadResponse create() => MarkAllReadResponse._();
  static $pb.GeneratedMessage $_createMessage() => MarkAllReadResponse._();
  @$core.override
  MarkAllReadResponse createEmptyInstance() => MarkAllReadResponse._();
  @$core.pragma('dart2js:noInline')
  static MarkAllReadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkAllReadResponse>(
          MarkAllReadResponse.$_createMessage);
  static MarkAllReadResponse? _defaultInstance;
}

class MarkAllStoreReadRequest extends $pb.GeneratedMessage {
  factory MarkAllStoreReadRequest() => MarkAllStoreReadRequest._();

  MarkAllStoreReadRequest._();

  factory MarkAllStoreReadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllStoreReadRequest()..mergeFromBuffer(data, registry);
  factory MarkAllStoreReadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllStoreReadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkAllStoreReadRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MarkAllStoreReadRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllStoreReadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllStoreReadRequest copyWith(
          void Function(MarkAllStoreReadRequest) updates) =>
      super.copyWith((message) => updates(message as MarkAllStoreReadRequest))
          as MarkAllStoreReadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkAllStoreReadRequest() / MarkAllStoreReadRequest.new instead')
  static MarkAllStoreReadRequest create() => MarkAllStoreReadRequest._();
  static $pb.GeneratedMessage $_createMessage() => MarkAllStoreReadRequest._();
  @$core.override
  MarkAllStoreReadRequest createEmptyInstance() => MarkAllStoreReadRequest._();
  @$core.pragma('dart2js:noInline')
  static MarkAllStoreReadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkAllStoreReadRequest>(
          MarkAllStoreReadRequest.$_createMessage);
  static MarkAllStoreReadRequest? _defaultInstance;
}

class MarkAllStoreReadResponse extends $pb.GeneratedMessage {
  factory MarkAllStoreReadResponse() => MarkAllStoreReadResponse._();

  MarkAllStoreReadResponse._();

  factory MarkAllStoreReadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllStoreReadResponse()..mergeFromBuffer(data, registry);
  factory MarkAllStoreReadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MarkAllStoreReadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkAllStoreReadResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: MarkAllStoreReadResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllStoreReadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkAllStoreReadResponse copyWith(
          void Function(MarkAllStoreReadResponse) updates) =>
      super.copyWith((message) => updates(message as MarkAllStoreReadResponse))
          as MarkAllStoreReadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MarkAllStoreReadResponse() / MarkAllStoreReadResponse.new instead')
  static MarkAllStoreReadResponse create() => MarkAllStoreReadResponse._();
  static $pb.GeneratedMessage $_createMessage() => MarkAllStoreReadResponse._();
  @$core.override
  MarkAllStoreReadResponse createEmptyInstance() =>
      MarkAllStoreReadResponse._();
  @$core.pragma('dart2js:noInline')
  static MarkAllStoreReadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkAllStoreReadResponse>(
          MarkAllStoreReadResponse.$_createMessage);
  static MarkAllStoreReadResponse? _defaultInstance;
}

class GetUnreadCountRequest extends $pb.GeneratedMessage {
  factory GetUnreadCountRequest() => GetUnreadCountRequest._();

  GetUnreadCountRequest._();

  factory GetUnreadCountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountRequest()..mergeFromBuffer(data, registry);
  factory GetUnreadCountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUnreadCountRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetUnreadCountRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountRequest copyWith(
          void Function(GetUnreadCountRequest) updates) =>
      super.copyWith((message) => updates(message as GetUnreadCountRequest))
          as GetUnreadCountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetUnreadCountRequest() / GetUnreadCountRequest.new instead')
  static GetUnreadCountRequest create() => GetUnreadCountRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetUnreadCountRequest._();
  @$core.override
  GetUnreadCountRequest createEmptyInstance() => GetUnreadCountRequest._();
  @$core.pragma('dart2js:noInline')
  static GetUnreadCountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetUnreadCountRequest>(
          GetUnreadCountRequest.$_createMessage);
  static GetUnreadCountRequest? _defaultInstance;
}

class GetUnreadCountResponse extends $pb.GeneratedMessage {
  factory GetUnreadCountResponse({
    $fixnum.Int64? unreadCount,
  }) {
    final result = GetUnreadCountResponse._();
    if (unreadCount != null) result.unreadCount = unreadCount;
    return result;
  }

  GetUnreadCountResponse._();

  factory GetUnreadCountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountResponse()..mergeFromBuffer(data, registry);
  factory GetUnreadCountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnreadCountResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUnreadCountResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetUnreadCountResponse.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'unreadCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnreadCountResponse copyWith(
          void Function(GetUnreadCountResponse) updates) =>
      super.copyWith((message) => updates(message as GetUnreadCountResponse))
          as GetUnreadCountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetUnreadCountResponse() / GetUnreadCountResponse.new instead')
  static GetUnreadCountResponse create() => GetUnreadCountResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetUnreadCountResponse._();
  @$core.override
  GetUnreadCountResponse createEmptyInstance() => GetUnreadCountResponse._();
  @$core.pragma('dart2js:noInline')
  static GetUnreadCountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetUnreadCountResponse>(
          GetUnreadCountResponse.$_createMessage);
  static GetUnreadCountResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get unreadCount => $_getI64(0);
  @$pb.TagNumber(1)
  set unreadCount($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUnreadCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearUnreadCount() => $_clearField(1);
}

class GetStoreUnreadCountRequest extends $pb.GeneratedMessage {
  factory GetStoreUnreadCountRequest() => GetStoreUnreadCountRequest._();

  GetStoreUnreadCountRequest._();

  factory GetStoreUnreadCountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreUnreadCountRequest()..mergeFromBuffer(data, registry);
  factory GetStoreUnreadCountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreUnreadCountRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreUnreadCountRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetStoreUnreadCountRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreUnreadCountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreUnreadCountRequest copyWith(
          void Function(GetStoreUnreadCountRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetStoreUnreadCountRequest))
          as GetStoreUnreadCountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreUnreadCountRequest() / GetStoreUnreadCountRequest.new instead')
  static GetStoreUnreadCountRequest create() => GetStoreUnreadCountRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetStoreUnreadCountRequest._();
  @$core.override
  GetStoreUnreadCountRequest createEmptyInstance() =>
      GetStoreUnreadCountRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStoreUnreadCountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreUnreadCountRequest>(
          GetStoreUnreadCountRequest.$_createMessage);
  static GetStoreUnreadCountRequest? _defaultInstance;
}

class GetStoreUnreadCountResponse extends $pb.GeneratedMessage {
  factory GetStoreUnreadCountResponse({
    $fixnum.Int64? unreadCount,
  }) {
    final result = GetStoreUnreadCountResponse._();
    if (unreadCount != null) result.unreadCount = unreadCount;
    return result;
  }

  GetStoreUnreadCountResponse._();

  factory GetStoreUnreadCountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreUnreadCountResponse()..mergeFromBuffer(data, registry);
  factory GetStoreUnreadCountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStoreUnreadCountResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoreUnreadCountResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetStoreUnreadCountResponse.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'unreadCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreUnreadCountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoreUnreadCountResponse copyWith(
          void Function(GetStoreUnreadCountResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetStoreUnreadCountResponse))
          as GetStoreUnreadCountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStoreUnreadCountResponse() / GetStoreUnreadCountResponse.new instead')
  static GetStoreUnreadCountResponse create() =>
      GetStoreUnreadCountResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetStoreUnreadCountResponse._();
  @$core.override
  GetStoreUnreadCountResponse createEmptyInstance() =>
      GetStoreUnreadCountResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStoreUnreadCountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoreUnreadCountResponse>(
          GetStoreUnreadCountResponse.$_createMessage);
  static GetStoreUnreadCountResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get unreadCount => $_getI64(0);
  @$pb.TagNumber(1)
  set unreadCount($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUnreadCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearUnreadCount() => $_clearField(1);
}

/// ConversationPresence 是一個對話的對方是否在線。
class ConversationPresence extends $pb.GeneratedMessage {
  factory ConversationPresence({
    $core.String? conversationId,
    $core.bool? online,
  }) {
    final result = ConversationPresence._();
    if (conversationId != null) result.conversationId = conversationId;
    if (online != null) result.online = online;
    return result;
  }

  ConversationPresence._();

  factory ConversationPresence.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConversationPresence()..mergeFromBuffer(data, registry);
  factory ConversationPresence.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ConversationPresence()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConversationPresence',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: ConversationPresence.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'conversationId')
    ..aOB(2, _omitFieldNames ? '' : 'online')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConversationPresence clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConversationPresence copyWith(void Function(ConversationPresence) updates) =>
      super.copyWith((message) => updates(message as ConversationPresence))
          as ConversationPresence;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ConversationPresence() / ConversationPresence.new instead')
  static ConversationPresence create() => ConversationPresence._();
  static $pb.GeneratedMessage $_createMessage() => ConversationPresence._();
  @$core.override
  ConversationPresence createEmptyInstance() => ConversationPresence._();
  @$core.pragma('dart2js:noInline')
  static ConversationPresence getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConversationPresence>(
          ConversationPresence.$_createMessage);
  static ConversationPresence? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get conversationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set conversationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasConversationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearConversationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get online => $_getBF(1);
  @$pb.TagNumber(2)
  set online($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOnline() => $_has(1);
  @$pb.TagNumber(2)
  void clearOnline() => $_clearField(2);
}

class GetPresenceRequest extends $pb.GeneratedMessage {
  factory GetPresenceRequest({
    $core.Iterable<$core.String>? conversationIds,
  }) {
    final result = GetPresenceRequest._();
    if (conversationIds != null) result.conversationIds.addAll(conversationIds);
    return result;
  }

  GetPresenceRequest._();

  factory GetPresenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPresenceRequest()..mergeFromBuffer(data, registry);
  factory GetPresenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPresenceRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPresenceRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetPresenceRequest.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'conversationIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceRequest copyWith(void Function(GetPresenceRequest) updates) =>
      super.copyWith((message) => updates(message as GetPresenceRequest))
          as GetPresenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetPresenceRequest() / GetPresenceRequest.new instead')
  static GetPresenceRequest create() => GetPresenceRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetPresenceRequest._();
  @$core.override
  GetPresenceRequest createEmptyInstance() => GetPresenceRequest._();
  @$core.pragma('dart2js:noInline')
  static GetPresenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPresenceRequest>(
          GetPresenceRequest.$_createMessage);
  static GetPresenceRequest? _defaultInstance;

  /// 1 到 50 個對話 ID。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get conversationIds => $_getList(0);
}

class GetPresenceResponse extends $pb.GeneratedMessage {
  factory GetPresenceResponse({
    $core.Iterable<ConversationPresence>? presences,
  }) {
    final result = GetPresenceResponse._();
    if (presences != null) result.presences.addAll(presences);
    return result;
  }

  GetPresenceResponse._();

  factory GetPresenceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPresenceResponse()..mergeFromBuffer(data, registry);
  factory GetPresenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPresenceResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPresenceResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetPresenceResponse.$_createMessage)
    ..pPM<ConversationPresence>(1, _omitFieldNames ? '' : 'presences',
        subBuilder: ConversationPresence.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceResponse copyWith(void Function(GetPresenceResponse) updates) =>
      super.copyWith((message) => updates(message as GetPresenceResponse))
          as GetPresenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use GetPresenceResponse() / GetPresenceResponse.new instead')
  static GetPresenceResponse create() => GetPresenceResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetPresenceResponse._();
  @$core.override
  GetPresenceResponse createEmptyInstance() => GetPresenceResponse._();
  @$core.pragma('dart2js:noInline')
  static GetPresenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPresenceResponse>(
          GetPresenceResponse.$_createMessage);
  static GetPresenceResponse? _defaultInstance;

  /// 每個請求的對話一項。
  @$pb.TagNumber(1)
  $pb.PbList<ConversationPresence> get presences => $_getList(0);
}

class GetStorePresenceRequest extends $pb.GeneratedMessage {
  factory GetStorePresenceRequest({
    $core.Iterable<$core.String>? conversationIds,
  }) {
    final result = GetStorePresenceRequest._();
    if (conversationIds != null) result.conversationIds.addAll(conversationIds);
    return result;
  }

  GetStorePresenceRequest._();

  factory GetStorePresenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePresenceRequest()..mergeFromBuffer(data, registry);
  factory GetStorePresenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePresenceRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStorePresenceRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetStorePresenceRequest.$_createMessage)
    ..pPS(1, _omitFieldNames ? '' : 'conversationIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePresenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePresenceRequest copyWith(
          void Function(GetStorePresenceRequest) updates) =>
      super.copyWith((message) => updates(message as GetStorePresenceRequest))
          as GetStorePresenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStorePresenceRequest() / GetStorePresenceRequest.new instead')
  static GetStorePresenceRequest create() => GetStorePresenceRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetStorePresenceRequest._();
  @$core.override
  GetStorePresenceRequest createEmptyInstance() => GetStorePresenceRequest._();
  @$core.pragma('dart2js:noInline')
  static GetStorePresenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStorePresenceRequest>(
          GetStorePresenceRequest.$_createMessage);
  static GetStorePresenceRequest? _defaultInstance;

  /// 1 到 50 個對話 ID。
  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get conversationIds => $_getList(0);
}

class GetStorePresenceResponse extends $pb.GeneratedMessage {
  factory GetStorePresenceResponse({
    $core.Iterable<ConversationPresence>? presences,
  }) {
    final result = GetStorePresenceResponse._();
    if (presences != null) result.presences.addAll(presences);
    return result;
  }

  GetStorePresenceResponse._();

  factory GetStorePresenceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePresenceResponse()..mergeFromBuffer(data, registry);
  factory GetStorePresenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetStorePresenceResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStorePresenceResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'hawkr.chat.v1alpha1'),
      createEmptyInstance: GetStorePresenceResponse.$_createMessage)
    ..pPM<ConversationPresence>(1, _omitFieldNames ? '' : 'presences',
        subBuilder: ConversationPresence.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePresenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStorePresenceResponse copyWith(
          void Function(GetStorePresenceResponse) updates) =>
      super.copyWith((message) => updates(message as GetStorePresenceResponse))
          as GetStorePresenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetStorePresenceResponse() / GetStorePresenceResponse.new instead')
  static GetStorePresenceResponse create() => GetStorePresenceResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetStorePresenceResponse._();
  @$core.override
  GetStorePresenceResponse createEmptyInstance() =>
      GetStorePresenceResponse._();
  @$core.pragma('dart2js:noInline')
  static GetStorePresenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStorePresenceResponse>(
          GetStorePresenceResponse.$_createMessage);
  static GetStorePresenceResponse? _defaultInstance;

  /// 每個請求的對話一項。
  @$pb.TagNumber(1)
  $pb.PbList<ConversationPresence> get presences => $_getList(0);
}

/// ChatAppService 是登入者的聊天。
class ChatAppServiceApi {
  final $pb.RpcClient _client;

  ChatAppServiceApi(this._client);

  /// ListConversations 列出呼叫者（買家）的對話，最近有訊息的在前（最後訊息時間，相同時以 ID 排序）。不因封鎖隱藏對話，不回總數。
  $async.Future<ListConversationsResponse> listConversations(
          $pb.ClientContext? ctx, ListConversationsRequest request) =>
      _client.invoke<ListConversationsResponse>(ctx, 'ChatAppService',
          'ListConversations', request, ListConversationsResponse());

  /// ListStoreConversations 列出呼叫者此刻的店的對話，規則同 ListConversations。呼叫者沒有店回 not_found；店家關閉之後沒有權限，回 not_found。
  $async.Future<ListStoreConversationsResponse> listStoreConversations(
          $pb.ClientContext? ctx, ListStoreConversationsRequest request) =>
      _client.invoke<ListStoreConversationsResponse>(ctx, 'ChatAppService',
          'ListStoreConversations', request, ListStoreConversationsResponse());

  /// ListMessages 列出對話裡的訊息。預設與 before_seq 是 seq 由大到小（新的在前），before_seq 只列 seq 小於它的訊息；
  /// after_seq 給斷線補洞，只列 seq 大於它的訊息，seq 由小到大（舊的在前）。before_seq 與 after_seq 不能同時給。
  /// 呼叫者不是對話的任一方，與對話不存在一律回 not_found，不區分原因。已隱藏或已抹除的訊息是墓碑，不帶內容。
  $async.Future<ListMessagesResponse> listMessages(
          $pb.ClientContext? ctx, ListMessagesRequest request) =>
      _client.invoke<ListMessagesResponse>(ctx, 'ChatAppService',
          'ListMessages', request, ListMessagesResponse());

  /// SendMessage 對既有的對話送出一則訊息，寄件方由對話的歸屬推導。同一個對話、同一個寄件方以同一個 request_id 重送，
  /// 回第一次寫入的那則訊息；內容不同回 invalid_argument。對方封鎖、店家停業、對方帳號已刪除等見 ErrorReason。
  $async.Future<SendMessageResponse> sendMessage(
          $pb.ClientContext? ctx, SendMessageRequest request) =>
      _client.invoke<SendMessageResponse>(
          ctx, 'ChatAppService', 'SendMessage', request, SendMessageResponse());

  /// SendMessageToStore 以買家身分對一家店送出訊息：沒有對話時開新對話，有就接續。其餘規則同 SendMessage。店家不存在回 not_found。
  $async.Future<SendMessageToStoreResponse> sendMessageToStore(
          $pb.ClientContext? ctx, SendMessageToStoreRequest request) =>
      _client.invoke<SendMessageToStoreResponse>(ctx, 'ChatAppService',
          'SendMessageToStore', request, SendMessageToStoreResponse());

  /// SendMessageToBuyer 以呼叫者此刻的店的身分對一位買家送出訊息：沒有對話時開新對話，有就接續。其餘規則同 SendMessage。
  /// 呼叫者沒有店或買家不存在回 not_found。
  $async.Future<SendMessageToBuyerResponse> sendMessageToBuyer(
          $pb.ClientContext? ctx, SendMessageToBuyerRequest request) =>
      _client.invoke<SendMessageToBuyerResponse>(ctx, 'ChatAppService',
          'SendMessageToBuyer', request, SendMessageToBuyerResponse());

  /// MarkConversationRead 把呼叫者這一方在對話裡的已讀游標推到 upto_seq：游標只前進，超過對話最後一個 seq 以最後一個計。
  $async.Future<MarkConversationReadResponse> markConversationRead(
          $pb.ClientContext? ctx, MarkConversationReadRequest request) =>
      _client.invoke<MarkConversationReadResponse>(ctx, 'ChatAppService',
          'MarkConversationRead', request, MarkConversationReadResponse());

  /// MarkAllRead 把呼叫者（買家）的全部對話標成已讀。
  $async.Future<MarkAllReadResponse> markAllRead(
          $pb.ClientContext? ctx, MarkAllReadRequest request) =>
      _client.invoke<MarkAllReadResponse>(
          ctx, 'ChatAppService', 'MarkAllRead', request, MarkAllReadResponse());

  /// MarkAllStoreRead 把呼叫者此刻的店的全部對話標成已讀。
  $async.Future<MarkAllStoreReadResponse> markAllStoreRead(
          $pb.ClientContext? ctx, MarkAllStoreReadRequest request) =>
      _client.invoke<MarkAllStoreReadResponse>(ctx, 'ChatAppService',
          'MarkAllStoreRead', request, MarkAllStoreReadResponse());

  /// GetUnreadCount 是呼叫者（買家）的總未讀訊息數。
  $async.Future<GetUnreadCountResponse> getUnreadCount(
          $pb.ClientContext? ctx, GetUnreadCountRequest request) =>
      _client.invoke<GetUnreadCountResponse>(ctx, 'ChatAppService',
          'GetUnreadCount', request, GetUnreadCountResponse());

  /// GetStoreUnreadCount 是呼叫者此刻的店的總未讀訊息數。
  $async.Future<GetStoreUnreadCountResponse> getStoreUnreadCount(
          $pb.ClientContext? ctx, GetStoreUnreadCountRequest request) =>
      _client.invoke<GetStoreUnreadCountResponse>(ctx, 'ChatAppService',
          'GetStoreUnreadCount', request, GetStoreUnreadCountResponse());

  /// GetPresence 回報呼叫者（買家）的對話的對方此刻是否在線，最多一次 50 個對話。沒有最後上線時間。
  /// 其中任何一個不是呼叫者的對話回 not_found（整個請求失敗）。任一方向有封鎖的對話回離線。
  $async.Future<GetPresenceResponse> getPresence(
          $pb.ClientContext? ctx, GetPresenceRequest request) =>
      _client.invoke<GetPresenceResponse>(
          ctx, 'ChatAppService', 'GetPresence', request, GetPresenceResponse());

  /// GetStorePresence 同 GetPresence，對象是呼叫者此刻的店的對話。
  $async.Future<GetStorePresenceResponse> getStorePresence(
          $pb.ClientContext? ctx, GetStorePresenceRequest request) =>
      _client.invoke<GetStorePresenceResponse>(ctx, 'ChatAppService',
          'GetStorePresence', request, GetStorePresenceResponse());
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
