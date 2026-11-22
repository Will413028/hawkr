// This is a generated file - do not edit.
//
// Generated from hawkr/chat/v1alpha1/chat.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// MessageKind 是訊息的種類。client 遇到 UNSPECIFIED 或不認得的值，顯示成「不支援的訊息」，不當成錯誤。
class MessageKind extends $pb.ProtobufEnum {
  static const MessageKind MESSAGE_KIND_UNSPECIFIED =
      MessageKind._(0, _omitEnumNames ? '' : 'MESSAGE_KIND_UNSPECIFIED');

  /// 文字訊息。
  static const MessageKind MESSAGE_KIND_TEXT =
      MessageKind._(1, _omitEnumNames ? '' : 'MESSAGE_KIND_TEXT');

  /// 圖片訊息。
  static const MessageKind MESSAGE_KIND_IMAGE =
      MessageKind._(2, _omitEnumNames ? '' : 'MESSAGE_KIND_IMAGE');

  static const $core.List<MessageKind> values = <MessageKind>[
    MESSAGE_KIND_UNSPECIFIED,
    MESSAGE_KIND_TEXT,
    MESSAGE_KIND_IMAGE,
  ];

  static final $core.List<MessageKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static MessageKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MessageKind._(super.value, super.name);
}

/// ChatParty 是對話的一方。
class ChatParty extends $pb.ProtobufEnum {
  static const ChatParty CHAT_PARTY_UNSPECIFIED =
      ChatParty._(0, _omitEnumNames ? '' : 'CHAT_PARTY_UNSPECIFIED');

  /// 買家方。
  static const ChatParty CHAT_PARTY_BUYER =
      ChatParty._(1, _omitEnumNames ? '' : 'CHAT_PARTY_BUYER');

  /// 店家方。
  static const ChatParty CHAT_PARTY_STORE =
      ChatParty._(2, _omitEnumNames ? '' : 'CHAT_PARTY_STORE');

  static const $core.List<ChatParty> values = <ChatParty>[
    CHAT_PARTY_UNSPECIFIED,
    CHAT_PARTY_BUYER,
    CHAT_PARTY_STORE,
  ];

  static final $core.List<ChatParty?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static ChatParty? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ChatParty._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
