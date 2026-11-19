// This is a generated file - do not edit.
//
// Generated from hawkr/assistant/v1alpha1/assistant.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// AssistantTopic 是這一則訊息要助理處理的主題，由畫面上的入口決定。
class AssistantTopic extends $pb.ProtobufEnum {
  static const AssistantTopic ASSISTANT_TOPIC_UNSPECIFIED =
      AssistantTopic._(0, _omitEnumNames ? '' : 'ASSISTANT_TOPIC_UNSPECIFIED');

  /// 查詢我的訂單。
  static const AssistantTopic ASSISTANT_TOPIC_ORDERS =
      AssistantTopic._(1, _omitEnumNames ? '' : 'ASSISTANT_TOPIC_ORDERS');

  /// 查詢物流進度。
  static const AssistantTopic ASSISTANT_TOPIC_TRACKING =
      AssistantTopic._(2, _omitEnumNames ? '' : 'ASSISTANT_TOPIC_TRACKING');

  /// 退貨政策；回覆是固定文案，不呼叫 LLM。
  static const AssistantTopic ASSISTANT_TOPIC_RETURN_POLICY = AssistantTopic._(
      3, _omitEnumNames ? '' : 'ASSISTANT_TOPIC_RETURN_POLICY');

  /// 一般問答。
  static const AssistantTopic ASSISTANT_TOPIC_CHAT =
      AssistantTopic._(4, _omitEnumNames ? '' : 'ASSISTANT_TOPIC_CHAT');

  static const $core.List<AssistantTopic> values = <AssistantTopic>[
    ASSISTANT_TOPIC_UNSPECIFIED,
    ASSISTANT_TOPIC_ORDERS,
    ASSISTANT_TOPIC_TRACKING,
    ASSISTANT_TOPIC_RETURN_POLICY,
    ASSISTANT_TOPIC_CHAT,
  ];

  static final $core.List<AssistantTopic?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static AssistantTopic? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AssistantTopic._(super.value, super.name);
}

/// AssistantMessageRole 是對話中一則訊息的發言者。
class AssistantMessageRole extends $pb.ProtobufEnum {
  static const AssistantMessageRole ASSISTANT_MESSAGE_ROLE_UNSPECIFIED =
      AssistantMessageRole._(
          0, _omitEnumNames ? '' : 'ASSISTANT_MESSAGE_ROLE_UNSPECIFIED');
  static const AssistantMessageRole ASSISTANT_MESSAGE_ROLE_USER =
      AssistantMessageRole._(
          1, _omitEnumNames ? '' : 'ASSISTANT_MESSAGE_ROLE_USER');
  static const AssistantMessageRole ASSISTANT_MESSAGE_ROLE_ASSISTANT =
      AssistantMessageRole._(
          2, _omitEnumNames ? '' : 'ASSISTANT_MESSAGE_ROLE_ASSISTANT');

  static const $core.List<AssistantMessageRole> values = <AssistantMessageRole>[
    ASSISTANT_MESSAGE_ROLE_UNSPECIFIED,
    ASSISTANT_MESSAGE_ROLE_USER,
    ASSISTANT_MESSAGE_ROLE_ASSISTANT,
  ];

  static final $core.List<AssistantMessageRole?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static AssistantMessageRole? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AssistantMessageRole._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
