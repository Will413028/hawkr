// This is a generated file - do not edit.
//
// Generated from hawkr/assistant/v1alpha1/assistant.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

import 'package:protobuf/well_known_types/google/protobuf/struct.pbjson.dart'
    as $0;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $1;

@$core.Deprecated('Use assistantTopicDescriptor instead')
const AssistantTopic$json = {
  '1': 'AssistantTopic',
  '2': [
    {'1': 'ASSISTANT_TOPIC_UNSPECIFIED', '2': 0},
    {'1': 'ASSISTANT_TOPIC_ORDERS', '2': 1},
    {'1': 'ASSISTANT_TOPIC_TRACKING', '2': 2},
    {'1': 'ASSISTANT_TOPIC_RETURN_POLICY', '2': 3},
    {'1': 'ASSISTANT_TOPIC_CHAT', '2': 4},
  ],
};

/// Descriptor for `AssistantTopic`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List assistantTopicDescriptor = $convert.base64Decode(
    'Cg5Bc3Npc3RhbnRUb3BpYxIfChtBU1NJU1RBTlRfVE9QSUNfVU5TUEVDSUZJRUQQABIaChZBU1'
    'NJU1RBTlRfVE9QSUNfT1JERVJTEAESHAoYQVNTSVNUQU5UX1RPUElDX1RSQUNLSU5HEAISIQod'
    'QVNTSVNUQU5UX1RPUElDX1JFVFVSTl9QT0xJQ1kQAxIYChRBU1NJU1RBTlRfVE9QSUNfQ0hBVB'
    'AE');

@$core.Deprecated('Use assistantMessageRoleDescriptor instead')
const AssistantMessageRole$json = {
  '1': 'AssistantMessageRole',
  '2': [
    {'1': 'ASSISTANT_MESSAGE_ROLE_UNSPECIFIED', '2': 0},
    {'1': 'ASSISTANT_MESSAGE_ROLE_USER', '2': 1},
    {'1': 'ASSISTANT_MESSAGE_ROLE_ASSISTANT', '2': 2},
  ],
};

/// Descriptor for `AssistantMessageRole`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List assistantMessageRoleDescriptor = $convert.base64Decode(
    'ChRBc3Npc3RhbnRNZXNzYWdlUm9sZRImCiJBU1NJU1RBTlRfTUVTU0FHRV9ST0xFX1VOU1BFQ0'
    'lGSUVEEAASHwobQVNTSVNUQU5UX01FU1NBR0VfUk9MRV9VU0VSEAESJAogQVNTSVNUQU5UX01F'
    'U1NBR0VfUk9MRV9BU1NJU1RBTlQQAg==');

@$core.Deprecated('Use sendAssistantMessageRequestDescriptor instead')
const SendAssistantMessageRequest$json = {
  '1': 'SendAssistantMessageRequest',
  '2': [
    {
      '1': 'topic',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.assistant.v1alpha1.AssistantTopic',
      '8': {},
      '10': 'topic'
    },
    {'1': 'session_id', '3': 2, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'message', '3': 3, '4': 1, '5': 9, '8': {}, '10': 'message'},
    {
      '1': 'image_url',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'imageUrl',
      '17': true
    },
    {
      '1': 'client_request_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'clientRequestId'
    },
  ],
  '8': [
    {'1': '_image_url'},
  ],
};

/// Descriptor for `SendAssistantMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendAssistantMessageRequestDescriptor = $convert.base64Decode(
    'ChtTZW5kQXNzaXN0YW50TWVzc2FnZVJlcXVlc3QSQwoFdG9waWMYASABKA4yKC5oYXdrci5hc3'
    'Npc3RhbnQudjFhbHBoYTEuQXNzaXN0YW50VG9waWNCA+BBAlIFdG9waWMSHQoKc2Vzc2lvbl9p'
    'ZBgCIAEoCVIJc2Vzc2lvbklkEh0KB21lc3NhZ2UYAyABKAlCA+BBAlIHbWVzc2FnZRIgCglpbW'
    'FnZV91cmwYBCABKAlIAFIIaW1hZ2VVcmyIAQESNwoRY2xpZW50X3JlcXVlc3RfaWQYBSABKAlC'
    'C+BBAuKMz9cIAggBUg9jbGllbnRSZXF1ZXN0SWRCDAoKX2ltYWdlX3VybA==');

@$core.Deprecated('Use sendAssistantMessageResponseDescriptor instead')
const SendAssistantMessageResponse$json = {
  '1': 'SendAssistantMessageResponse',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '10': 'turnId'},
    {'1': 'session_id', '3': 2, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `SendAssistantMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendAssistantMessageResponseDescriptor =
    $convert.base64Decode(
        'ChxTZW5kQXNzaXN0YW50TWVzc2FnZVJlc3BvbnNlEhcKB3R1cm5faWQYASABKAlSBnR1cm5JZB'
        'IdCgpzZXNzaW9uX2lkGAIgASgJUglzZXNzaW9uSWQ=');

@$core.Deprecated('Use watchAssistantTurnRequestDescriptor instead')
const WatchAssistantTurnRequest$json = {
  '1': 'WatchAssistantTurnRequest',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'turnId'},
    {'1': 'cursor', '3': 2, '4': 1, '5': 9, '10': 'cursor'},
  ],
};

/// Descriptor for `WatchAssistantTurnRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchAssistantTurnRequestDescriptor =
    $convert.base64Decode(
        'ChlXYXRjaEFzc2lzdGFudFR1cm5SZXF1ZXN0EhwKB3R1cm5faWQYASABKAlCA+BBAlIGdHVybk'
        'lkEhYKBmN1cnNvchgCIAEoCVIGY3Vyc29y');

@$core.Deprecated('Use watchAssistantTurnResponseDescriptor instead')
const WatchAssistantTurnResponse$json = {
  '1': 'WatchAssistantTurnResponse',
  '2': [
    {'1': 'cursor', '3': 1, '4': 1, '5': 9, '10': 'cursor'},
    {
      '1': 'text',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.assistant.v1alpha1.AssistantTextDelta',
      '9': 0,
      '10': 'text'
    },
    {
      '1': 'tool_result',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.assistant.v1alpha1.AssistantToolResult',
      '9': 0,
      '10': 'toolResult'
    },
    {
      '1': 'finished',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.assistant.v1alpha1.AssistantFinished',
      '9': 0,
      '10': 'finished'
    },
    {
      '1': 'error',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.assistant.v1alpha1.AssistantStreamError',
      '9': 0,
      '10': 'error'
    },
  ],
  '8': [
    {'1': 'event'},
  ],
};

/// Descriptor for `WatchAssistantTurnResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchAssistantTurnResponseDescriptor = $convert.base64Decode(
    'ChpXYXRjaEFzc2lzdGFudFR1cm5SZXNwb25zZRIWCgZjdXJzb3IYASABKAlSBmN1cnNvchJCCg'
    'R0ZXh0GAIgASgLMiwuaGF3a3IuYXNzaXN0YW50LnYxYWxwaGExLkFzc2lzdGFudFRleHREZWx0'
    'YUgAUgR0ZXh0ElAKC3Rvb2xfcmVzdWx0GAMgASgLMi0uaGF3a3IuYXNzaXN0YW50LnYxYWxwaG'
    'ExLkFzc2lzdGFudFRvb2xSZXN1bHRIAFIKdG9vbFJlc3VsdBJJCghmaW5pc2hlZBgEIAEoCzIr'
    'Lmhhd2tyLmFzc2lzdGFudC52MWFscGhhMS5Bc3Npc3RhbnRGaW5pc2hlZEgAUghmaW5pc2hlZB'
    'JGCgVlcnJvchgFIAEoCzIuLmhhd2tyLmFzc2lzdGFudC52MWFscGhhMS5Bc3Npc3RhbnRTdHJl'
    'YW1FcnJvckgAUgVlcnJvckIHCgVldmVudA==');

@$core.Deprecated('Use assistantTextDeltaDescriptor instead')
const AssistantTextDelta$json = {
  '1': 'AssistantTextDelta',
  '2': [
    {'1': 'text', '3': 1, '4': 1, '5': 9, '10': 'text'},
  ],
};

/// Descriptor for `AssistantTextDelta`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assistantTextDeltaDescriptor = $convert
    .base64Decode('ChJBc3Npc3RhbnRUZXh0RGVsdGESEgoEdGV4dBgBIAEoCVIEdGV4dA==');

@$core.Deprecated('Use assistantToolResultDescriptor instead')
const AssistantToolResult$json = {
  '1': 'AssistantToolResult',
  '2': [
    {'1': 'tool_name', '3': 1, '4': 1, '5': 9, '10': 'toolName'},
    {'1': 'failed', '3': 2, '4': 1, '5': 8, '10': 'failed'},
    {
      '1': 'arguments',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Struct',
      '10': 'arguments'
    },
  ],
};

/// Descriptor for `AssistantToolResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assistantToolResultDescriptor = $convert.base64Decode(
    'ChNBc3Npc3RhbnRUb29sUmVzdWx0EhsKCXRvb2xfbmFtZRgBIAEoCVIIdG9vbE5hbWUSFgoGZm'
    'FpbGVkGAIgASgIUgZmYWlsZWQSNQoJYXJndW1lbnRzGAMgASgLMhcuZ29vZ2xlLnByb3RvYnVm'
    'LlN0cnVjdFIJYXJndW1lbnRz');

@$core.Deprecated('Use assistantFinishedDescriptor instead')
const AssistantFinished$json = {
  '1': 'AssistantFinished',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `AssistantFinished`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assistantFinishedDescriptor = $convert.base64Decode(
    'ChFBc3Npc3RhbnRGaW5pc2hlZBIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSWQ=');

@$core.Deprecated('Use assistantStreamErrorDescriptor instead')
const AssistantStreamError$json = {
  '1': 'AssistantStreamError',
  '2': [
    {
      '1': 'reason',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.common.v1alpha1.ErrorReason',
      '10': 'reason'
    },
    {
      '1': 'localized_message',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'localizedMessage'
    },
    {
      '1': 'metadata',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.hawkr.assistant.v1alpha1.AssistantStreamError.MetadataEntry',
      '10': 'metadata'
    },
  ],
  '3': [AssistantStreamError_MetadataEntry$json],
};

@$core.Deprecated('Use assistantStreamErrorDescriptor instead')
const AssistantStreamError_MetadataEntry$json = {
  '1': 'MetadataEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `AssistantStreamError`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assistantStreamErrorDescriptor = $convert.base64Decode(
    'ChRBc3Npc3RhbnRTdHJlYW1FcnJvchI6CgZyZWFzb24YASABKA4yIi5oYXdrci5jb21tb24udj'
    'FhbHBoYTEuRXJyb3JSZWFzb25SBnJlYXNvbhIrChFsb2NhbGl6ZWRfbWVzc2FnZRgCIAEoCVIQ'
    'bG9jYWxpemVkTWVzc2FnZRJYCghtZXRhZGF0YRgDIAMoCzI8Lmhhd2tyLmFzc2lzdGFudC52MW'
    'FscGhhMS5Bc3Npc3RhbnRTdHJlYW1FcnJvci5NZXRhZGF0YUVudHJ5UghtZXRhZGF0YRo7Cg1N'
    'ZXRhZGF0YUVudHJ5EhAKA2tleRgBIAEoCVIDa2V5EhQKBXZhbHVlGAIgASgJUgV2YWx1ZToCOA'
    'E=');

@$core.Deprecated('Use getLatestAssistantSessionRequestDescriptor instead')
const GetLatestAssistantSessionRequest$json = {
  '1': 'GetLatestAssistantSessionRequest',
};

/// Descriptor for `GetLatestAssistantSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getLatestAssistantSessionRequestDescriptor =
    $convert.base64Decode('CiBHZXRMYXRlc3RBc3Npc3RhbnRTZXNzaW9uUmVxdWVzdA==');

@$core.Deprecated('Use getLatestAssistantSessionResponseDescriptor instead')
const GetLatestAssistantSessionResponse$json = {
  '1': 'GetLatestAssistantSessionResponse',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {
      '1': 'messages',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.hawkr.assistant.v1alpha1.AssistantMessage',
      '10': 'messages'
    },
  ],
};

/// Descriptor for `GetLatestAssistantSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getLatestAssistantSessionResponseDescriptor =
    $convert.base64Decode(
        'CiFHZXRMYXRlc3RBc3Npc3RhbnRTZXNzaW9uUmVzcG9uc2USHQoKc2Vzc2lvbl9pZBgBIAEoCV'
        'IJc2Vzc2lvbklkEkYKCG1lc3NhZ2VzGAIgAygLMiouaGF3a3IuYXNzaXN0YW50LnYxYWxwaGEx'
        'LkFzc2lzdGFudE1lc3NhZ2VSCG1lc3NhZ2Vz');

@$core.Deprecated('Use assistantMessageDescriptor instead')
const AssistantMessage$json = {
  '1': 'AssistantMessage',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.assistant.v1alpha1.AssistantMessageRole',
      '10': 'role'
    },
    {'1': 'text', '3': 2, '4': 1, '5': 9, '10': 'text'},
    {
      '1': 'create_time',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
  ],
};

/// Descriptor for `AssistantMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assistantMessageDescriptor = $convert.base64Decode(
    'ChBBc3Npc3RhbnRNZXNzYWdlEkIKBHJvbGUYASABKA4yLi5oYXdrci5hc3Npc3RhbnQudjFhbH'
    'BoYTEuQXNzaXN0YW50TWVzc2FnZVJvbGVSBHJvbGUSEgoEdGV4dBgCIAEoCVIEdGV4dBJACgtj'
    'cmVhdGVfdGltZRgDIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1IKY3JlYX'
    'RlVGltZQ==');

const $core.Map<$core.String, $core.dynamic> AssistantAppServiceBase$json = {
  '1': 'AssistantAppService',
  '2': [
    {
      '1': 'SendAssistantMessage',
      '2': '.hawkr.assistant.v1alpha1.SendAssistantMessageRequest',
      '3': '.hawkr.assistant.v1alpha1.SendAssistantMessageResponse'
    },
    {
      '1': 'WatchAssistantTurn',
      '2': '.hawkr.assistant.v1alpha1.WatchAssistantTurnRequest',
      '3': '.hawkr.assistant.v1alpha1.WatchAssistantTurnResponse',
      '6': true
    },
    {
      '1': 'GetLatestAssistantSession',
      '2': '.hawkr.assistant.v1alpha1.GetLatestAssistantSessionRequest',
      '3': '.hawkr.assistant.v1alpha1.GetLatestAssistantSessionResponse',
      '4': {'34': 1},
    },
  ],
};

@$core.Deprecated('Use assistantAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    AssistantAppServiceBase$messageJson = {
  '.hawkr.assistant.v1alpha1.SendAssistantMessageRequest':
      SendAssistantMessageRequest$json,
  '.hawkr.assistant.v1alpha1.SendAssistantMessageResponse':
      SendAssistantMessageResponse$json,
  '.hawkr.assistant.v1alpha1.WatchAssistantTurnRequest':
      WatchAssistantTurnRequest$json,
  '.hawkr.assistant.v1alpha1.WatchAssistantTurnResponse':
      WatchAssistantTurnResponse$json,
  '.hawkr.assistant.v1alpha1.AssistantTextDelta': AssistantTextDelta$json,
  '.hawkr.assistant.v1alpha1.AssistantToolResult': AssistantToolResult$json,
  '.google.protobuf.Struct': $0.Struct$json,
  '.google.protobuf.Struct.FieldsEntry': $0.Struct_FieldsEntry$json,
  '.google.protobuf.Value': $0.Value$json,
  '.google.protobuf.ListValue': $0.ListValue$json,
  '.hawkr.assistant.v1alpha1.AssistantFinished': AssistantFinished$json,
  '.hawkr.assistant.v1alpha1.AssistantStreamError': AssistantStreamError$json,
  '.hawkr.assistant.v1alpha1.AssistantStreamError.MetadataEntry':
      AssistantStreamError_MetadataEntry$json,
  '.hawkr.assistant.v1alpha1.GetLatestAssistantSessionRequest':
      GetLatestAssistantSessionRequest$json,
  '.hawkr.assistant.v1alpha1.GetLatestAssistantSessionResponse':
      GetLatestAssistantSessionResponse$json,
  '.hawkr.assistant.v1alpha1.AssistantMessage': AssistantMessage$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
};

/// Descriptor for `AssistantAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List assistantAppServiceDescriptor = $convert.base64Decode(
    'ChNBc3Npc3RhbnRBcHBTZXJ2aWNlEoUBChRTZW5kQXNzaXN0YW50TWVzc2FnZRI1Lmhhd2tyLm'
    'Fzc2lzdGFudC52MWFscGhhMS5TZW5kQXNzaXN0YW50TWVzc2FnZVJlcXVlc3QaNi5oYXdrci5h'
    'c3Npc3RhbnQudjFhbHBoYTEuU2VuZEFzc2lzdGFudE1lc3NhZ2VSZXNwb25zZRKBAQoSV2F0Y2'
    'hBc3Npc3RhbnRUdXJuEjMuaGF3a3IuYXNzaXN0YW50LnYxYWxwaGExLldhdGNoQXNzaXN0YW50'
    'VHVyblJlcXVlc3QaNC5oYXdrci5hc3Npc3RhbnQudjFhbHBoYTEuV2F0Y2hBc3Npc3RhbnRUdX'
    'JuUmVzcG9uc2UwARKZAQoZR2V0TGF0ZXN0QXNzaXN0YW50U2Vzc2lvbhI6Lmhhd2tyLmFzc2lz'
    'dGFudC52MWFscGhhMS5HZXRMYXRlc3RBc3Npc3RhbnRTZXNzaW9uUmVxdWVzdBo7Lmhhd2tyLm'
    'Fzc2lzdGFudC52MWFscGhhMS5HZXRMYXRlc3RBc3Npc3RhbnRTZXNzaW9uUmVzcG9uc2UiA5AC'
    'AQ==');
