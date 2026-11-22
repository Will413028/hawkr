// This is a generated file - do not edit.
//
// Generated from hawkr/chat/v1alpha1/chat.proto.

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

import 'package:protobuf/well_known_types/google/protobuf/duration.pbjson.dart'
    as $3;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pbjson.dart'
    as $1;

import '../../common/v1alpha1/image.pbjson.dart' as $0;
import '../../common/v1alpha1/store_card.pbjson.dart' as $2;

@$core.Deprecated('Use messageKindDescriptor instead')
const MessageKind$json = {
  '1': 'MessageKind',
  '2': [
    {'1': 'MESSAGE_KIND_UNSPECIFIED', '2': 0},
    {'1': 'MESSAGE_KIND_TEXT', '2': 1},
    {'1': 'MESSAGE_KIND_IMAGE', '2': 2},
  ],
};

/// Descriptor for `MessageKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List messageKindDescriptor = $convert.base64Decode(
    'CgtNZXNzYWdlS2luZBIcChhNRVNTQUdFX0tJTkRfVU5TUEVDSUZJRUQQABIVChFNRVNTQUdFX0'
    'tJTkRfVEVYVBABEhYKEk1FU1NBR0VfS0lORF9JTUFHRRAC');

@$core.Deprecated('Use chatPartyDescriptor instead')
const ChatParty$json = {
  '1': 'ChatParty',
  '2': [
    {'1': 'CHAT_PARTY_UNSPECIFIED', '2': 0},
    {'1': 'CHAT_PARTY_BUYER', '2': 1},
    {'1': 'CHAT_PARTY_STORE', '2': 2},
  ],
};

/// Descriptor for `ChatParty`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List chatPartyDescriptor = $convert.base64Decode(
    'CglDaGF0UGFydHkSGgoWQ0hBVF9QQVJUWV9VTlNQRUNJRklFRBAAEhQKEENIQVRfUEFSVFlfQl'
    'VZRVIQARIUChBDSEFUX1BBUlRZX1NUT1JFEAI=');

@$core.Deprecated('Use messageDescriptor instead')
const Message$json = {
  '1': 'Message',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'conversation_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
    {'1': 'seq', '3': 3, '4': 1, '5': 3, '8': {}, '10': 'seq'},
    {
      '1': 'sender',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.hawkr.chat.v1alpha1.ChatParty',
      '8': {},
      '10': 'sender'
    },
    {
      '1': 'kind',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.chat.v1alpha1.MessageKind',
      '8': {},
      '10': 'kind'
    },
    {
      '1': 'text',
      '3': 6,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'text',
      '17': true
    },
    {
      '1': 'image',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'image'
    },
    {'1': 'hidden', '3': 8, '4': 1, '5': 8, '8': {}, '10': 'hidden'},
    {'1': 'erased', '3': 9, '4': 1, '5': 8, '8': {}, '10': 'erased'},
    {
      '1': 'create_time',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'createTime'
    },
  ],
  '8': [
    {'1': '_text'},
  ],
};

/// Descriptor for `Message`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageDescriptor = $convert.base64Decode(
    'CgdNZXNzYWdlEhMKAmlkGAEgASgJQgPgQQhSAmlkEiwKD2NvbnZlcnNhdGlvbl9pZBgCIAEoCU'
    'ID4EEDUg5jb252ZXJzYXRpb25JZBIVCgNzZXEYAyABKANCA+BBA1IDc2VxEjsKBnNlbmRlchgE'
    'IAEoDjIeLmhhd2tyLmNoYXQudjFhbHBoYTEuQ2hhdFBhcnR5QgPgQQNSBnNlbmRlchI5CgRraW'
    '5kGAUgASgOMiAuaGF3a3IuY2hhdC52MWFscGhhMS5NZXNzYWdlS2luZEID4EEDUgRraW5kEhwK'
    'BHRleHQYBiABKAlCA+BBA0gAUgR0ZXh0iAEBEjcKBWltYWdlGAcgASgLMhwuaGF3a3IuY29tbW'
    '9uLnYxYWxwaGExLkltYWdlQgPgQQNSBWltYWdlEhsKBmhpZGRlbhgIIAEoCEID4EEDUgZoaWRk'
    'ZW4SGwoGZXJhc2VkGAkgASgIQgPgQQNSBmVyYXNlZBJACgtjcmVhdGVfdGltZRgKIAEoCzIaLm'
    'dvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBCA+BBA1IKY3JlYXRlVGltZUIHCgVfdGV4dA==');

@$core.Deprecated('Use previewDescriptor instead')
const Preview$json = {
  '1': 'Preview',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.hawkr.chat.v1alpha1.MessageKind',
      '8': {},
      '10': 'kind'
    },
    {
      '1': 'text',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'text',
      '17': true
    },
    {'1': 'hidden', '3': 3, '4': 1, '5': 8, '8': {}, '10': 'hidden'},
    {'1': 'erased', '3': 4, '4': 1, '5': 8, '8': {}, '10': 'erased'},
    {
      '1': 'sender',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.hawkr.chat.v1alpha1.ChatParty',
      '8': {},
      '10': 'sender'
    },
  ],
  '8': [
    {'1': '_text'},
  ],
};

/// Descriptor for `Preview`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List previewDescriptor = $convert.base64Decode(
    'CgdQcmV2aWV3EjkKBGtpbmQYASABKA4yIC5oYXdrci5jaGF0LnYxYWxwaGExLk1lc3NhZ2VLaW'
    '5kQgPgQQNSBGtpbmQSHAoEdGV4dBgCIAEoCUID4EEDSABSBHRleHSIAQESGwoGaGlkZGVuGAMg'
    'ASgIQgPgQQNSBmhpZGRlbhIbCgZlcmFzZWQYBCABKAhCA+BBA1IGZXJhc2VkEjsKBnNlbmRlch'
    'gFIAEoDjIeLmhhd2tyLmNoYXQudjFhbHBoYTEuQ2hhdFBhcnR5QgPgQQNSBnNlbmRlckIHCgVf'
    'dGV4dA==');

@$core.Deprecated('Use chatBuyerDescriptor instead')
const ChatBuyer$json = {
  '1': 'ChatBuyer',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'name',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'name',
      '17': true
    },
    {
      '1': 'avatar',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.Image',
      '8': {},
      '10': 'avatar'
    },
    {'1': 'deleted', '3': 4, '4': 1, '5': 8, '8': {}, '10': 'deleted'},
  ],
  '8': [
    {'1': '_name'},
  ],
};

/// Descriptor for `ChatBuyer`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatBuyerDescriptor = $convert.base64Decode(
    'CglDaGF0QnV5ZXISEwoCaWQYASABKAlCA+BBCFICaWQSHAoEbmFtZRgCIAEoCUID4EEDSABSBG'
    '5hbWWIAQESOQoGYXZhdGFyGAMgASgLMhwuaGF3a3IuY29tbW9uLnYxYWxwaGExLkltYWdlQgPg'
    'QQNSBmF2YXRhchIdCgdkZWxldGVkGAQgASgIQgPgQQNSB2RlbGV0ZWRCBwoFX25hbWU=');

@$core.Deprecated('Use conversationDescriptor instead')
const Conversation$json = {
  '1': 'Conversation',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'id'},
    {
      '1': 'store',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.common.v1alpha1.StoreCard',
      '8': {},
      '10': 'store'
    },
    {
      '1': 'buyer',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.ChatBuyer',
      '8': {},
      '10': 'buyer'
    },
    {'1': 'unread_count', '3': 4, '4': 1, '5': 3, '8': {}, '10': 'unreadCount'},
    {
      '1': 'last_message_time',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '8': {},
      '10': 'lastMessageTime'
    },
    {
      '1': 'preview',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Preview',
      '8': {},
      '10': 'preview'
    },
    {
      '1': 'peer_read_seq',
      '3': 7,
      '4': 1,
      '5': 3,
      '8': {},
      '10': 'peerReadSeq'
    },
  ],
};

/// Descriptor for `Conversation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List conversationDescriptor = $convert.base64Decode(
    'CgxDb252ZXJzYXRpb24SEwoCaWQYASABKAlCA+BBCFICaWQSOwoFc3RvcmUYAiABKAsyIC5oYX'
    'drci5jb21tb24udjFhbHBoYTEuU3RvcmVDYXJkQgPgQQNSBXN0b3JlEjkKBWJ1eWVyGAMgASgL'
    'Mh4uaGF3a3IuY2hhdC52MWFscGhhMS5DaGF0QnV5ZXJCA+BBA1IFYnV5ZXISJgoMdW5yZWFkX2'
    'NvdW50GAQgASgDQgPgQQNSC3VucmVhZENvdW50EksKEWxhc3RfbWVzc2FnZV90aW1lGAUgASgL'
    'MhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEID4EEDUg9sYXN0TWVzc2FnZVRpbWUSOwoHcH'
    'JldmlldxgGIAEoCzIcLmhhd2tyLmNoYXQudjFhbHBoYTEuUHJldmlld0ID4EEDUgdwcmV2aWV3'
    'EicKDXBlZXJfcmVhZF9zZXEYByABKANCA+BBA1ILcGVlclJlYWRTZXE=');

@$core.Deprecated('Use messageContentDescriptor instead')
const MessageContent$json = {
  '1': 'MessageContent',
  '2': [
    {'1': 'text', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'text'},
    {
      '1': 'image_staged_token',
      '3': 2,
      '4': 1,
      '5': 9,
      '8': {},
      '9': 0,
      '10': 'imageStagedToken'
    },
  ],
  '8': [
    {'1': 'content'},
  ],
};

/// Descriptor for `MessageContent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageContentDescriptor = $convert.base64Decode(
    'Cg5NZXNzYWdlQ29udGVudBIUCgR0ZXh0GAEgASgJSABSBHRleHQSMwoSaW1hZ2Vfc3RhZ2VkX3'
    'Rva2VuGAIgASgJQgPgQQRIAFIQaW1hZ2VTdGFnZWRUb2tlbkIJCgdjb250ZW50');

@$core.Deprecated('Use listConversationsRequestDescriptor instead')
const ListConversationsRequest$json = {
  '1': 'ListConversationsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListConversationsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listConversationsRequestDescriptor =
    $convert.base64Decode(
        'ChhMaXN0Q29udmVyc2F0aW9uc1JlcXVlc3QSGwoJcGFnZV9zaXplGAEgASgFUghwYWdlU2l6ZR'
        'IdCgpwYWdlX3Rva2VuGAIgASgJUglwYWdlVG9rZW4=');

@$core.Deprecated('Use listConversationsResponseDescriptor instead')
const ListConversationsResponse$json = {
  '1': 'ListConversationsResponse',
  '2': [
    {
      '1': 'conversations',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Conversation',
      '10': 'conversations'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListConversationsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listConversationsResponseDescriptor = $convert.base64Decode(
    'ChlMaXN0Q29udmVyc2F0aW9uc1Jlc3BvbnNlEkcKDWNvbnZlcnNhdGlvbnMYASADKAsyIS5oYX'
    'drci5jaGF0LnYxYWxwaGExLkNvbnZlcnNhdGlvblINY29udmVyc2F0aW9ucxImCg9uZXh0X3Bh'
    'Z2VfdG9rZW4YAiABKAlSDW5leHRQYWdlVG9rZW4=');

@$core.Deprecated('Use listStoreConversationsRequestDescriptor instead')
const ListStoreConversationsRequest$json = {
  '1': 'ListStoreConversationsRequest',
  '2': [
    {'1': 'page_size', '3': 1, '4': 1, '5': 5, '10': 'pageSize'},
    {'1': 'page_token', '3': 2, '4': 1, '5': 9, '10': 'pageToken'},
  ],
};

/// Descriptor for `ListStoreConversationsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreConversationsRequestDescriptor =
    $convert.base64Decode(
        'Ch1MaXN0U3RvcmVDb252ZXJzYXRpb25zUmVxdWVzdBIbCglwYWdlX3NpemUYASABKAVSCHBhZ2'
        'VTaXplEh0KCnBhZ2VfdG9rZW4YAiABKAlSCXBhZ2VUb2tlbg==');

@$core.Deprecated('Use listStoreConversationsResponseDescriptor instead')
const ListStoreConversationsResponse$json = {
  '1': 'ListStoreConversationsResponse',
  '2': [
    {
      '1': 'conversations',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Conversation',
      '10': 'conversations'
    },
    {'1': 'next_page_token', '3': 2, '4': 1, '5': 9, '10': 'nextPageToken'},
  ],
};

/// Descriptor for `ListStoreConversationsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listStoreConversationsResponseDescriptor =
    $convert.base64Decode(
        'Ch5MaXN0U3RvcmVDb252ZXJzYXRpb25zUmVzcG9uc2USRwoNY29udmVyc2F0aW9ucxgBIAMoCz'
        'IhLmhhd2tyLmNoYXQudjFhbHBoYTEuQ29udmVyc2F0aW9uUg1jb252ZXJzYXRpb25zEiYKD25l'
        'eHRfcGFnZV90b2tlbhgCIAEoCVINbmV4dFBhZ2VUb2tlbg==');

@$core.Deprecated('Use listMessagesRequestDescriptor instead')
const ListMessagesRequest$json = {
  '1': 'ListMessagesRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
    {'1': 'page_size', '3': 2, '4': 1, '5': 5, '10': 'pageSize'},
    {
      '1': 'before_seq',
      '3': 3,
      '4': 1,
      '5': 3,
      '9': 0,
      '10': 'beforeSeq',
      '17': true
    },
    {
      '1': 'after_seq',
      '3': 4,
      '4': 1,
      '5': 3,
      '9': 1,
      '10': 'afterSeq',
      '17': true
    },
  ],
  '8': [
    {'1': '_before_seq'},
    {'1': '_after_seq'},
  ],
};

/// Descriptor for `ListMessagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMessagesRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0TWVzc2FnZXNSZXF1ZXN0EiwKD2NvbnZlcnNhdGlvbl9pZBgBIAEoCUID4EECUg5jb2'
    '52ZXJzYXRpb25JZBIbCglwYWdlX3NpemUYAiABKAVSCHBhZ2VTaXplEiIKCmJlZm9yZV9zZXEY'
    'AyABKANIAFIJYmVmb3JlU2VxiAEBEiAKCWFmdGVyX3NlcRgEIAEoA0gBUghhZnRlclNlcYgBAU'
    'INCgtfYmVmb3JlX3NlcUIMCgpfYWZ0ZXJfc2Vx');

@$core.Deprecated('Use listMessagesResponseDescriptor instead')
const ListMessagesResponse$json = {
  '1': 'ListMessagesResponse',
  '2': [
    {
      '1': 'messages',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Message',
      '10': 'messages'
    },
    {'1': 'has_more', '3': 2, '4': 1, '5': 8, '10': 'hasMore'},
    {'1': 'peer_read_seq', '3': 3, '4': 1, '5': 3, '10': 'peerReadSeq'},
  ],
};

/// Descriptor for `ListMessagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMessagesResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0TWVzc2FnZXNSZXNwb25zZRI4CghtZXNzYWdlcxgBIAMoCzIcLmhhd2tyLmNoYXQudj'
    'FhbHBoYTEuTWVzc2FnZVIIbWVzc2FnZXMSGQoIaGFzX21vcmUYAiABKAhSB2hhc01vcmUSIgoN'
    'cGVlcl9yZWFkX3NlcRgDIAEoA1ILcGVlclJlYWRTZXE=');

@$core.Deprecated('Use sendMessageRequestDescriptor instead')
const SendMessageRequest$json = {
  '1': 'SendMessageRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'content',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.MessageContent',
      '8': {},
      '10': 'content'
    },
  ],
};

/// Descriptor for `SendMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageRequestDescriptor = $convert.base64Decode(
    'ChJTZW5kTWVzc2FnZVJlcXVlc3QSLAoPY29udmVyc2F0aW9uX2lkGAEgASgJQgPgQQJSDmNvbn'
    'ZlcnNhdGlvbklkEioKCnJlcXVlc3RfaWQYAiABKAlCC+BBAuKMz9cIAggBUglyZXF1ZXN0SWQS'
    'QgoHY29udGVudBgDIAEoCzIjLmhhd2tyLmNoYXQudjFhbHBoYTEuTWVzc2FnZUNvbnRlbnRCA+'
    'BBAlIHY29udGVudA==');

@$core.Deprecated('Use sendMessageResponseDescriptor instead')
const SendMessageResponse$json = {
  '1': 'SendMessageResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `SendMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageResponseDescriptor = $convert.base64Decode(
    'ChNTZW5kTWVzc2FnZVJlc3BvbnNlEjYKB21lc3NhZ2UYASABKAsyHC5oYXdrci5jaGF0LnYxYW'
    'xwaGExLk1lc3NhZ2VSB21lc3NhZ2U=');

@$core.Deprecated('Use sendMessageToStoreRequestDescriptor instead')
const SendMessageToStoreRequest$json = {
  '1': 'SendMessageToStoreRequest',
  '2': [
    {'1': 'store_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'storeId'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'content',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.MessageContent',
      '8': {},
      '10': 'content'
    },
  ],
};

/// Descriptor for `SendMessageToStoreRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageToStoreRequestDescriptor = $convert.base64Decode(
    'ChlTZW5kTWVzc2FnZVRvU3RvcmVSZXF1ZXN0Eh4KCHN0b3JlX2lkGAEgASgJQgPgQQJSB3N0b3'
    'JlSWQSKgoKcmVxdWVzdF9pZBgCIAEoCUIL4EEC4ozP1wgCCAFSCXJlcXVlc3RJZBJCCgdjb250'
    'ZW50GAMgASgLMiMuaGF3a3IuY2hhdC52MWFscGhhMS5NZXNzYWdlQ29udGVudEID4EECUgdjb2'
    '50ZW50');

@$core.Deprecated('Use sendMessageToStoreResponseDescriptor instead')
const SendMessageToStoreResponse$json = {
  '1': 'SendMessageToStoreResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `SendMessageToStoreResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageToStoreResponseDescriptor =
    $convert.base64Decode(
        'ChpTZW5kTWVzc2FnZVRvU3RvcmVSZXNwb25zZRI2CgdtZXNzYWdlGAEgASgLMhwuaGF3a3IuY2'
        'hhdC52MWFscGhhMS5NZXNzYWdlUgdtZXNzYWdl');

@$core.Deprecated('Use sendMessageToBuyerRequestDescriptor instead')
const SendMessageToBuyerRequest$json = {
  '1': 'SendMessageToBuyerRequest',
  '2': [
    {'1': 'buyer_id', '3': 1, '4': 1, '5': 9, '8': {}, '10': 'buyerId'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '8': {}, '10': 'requestId'},
    {
      '1': 'content',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.MessageContent',
      '8': {},
      '10': 'content'
    },
  ],
};

/// Descriptor for `SendMessageToBuyerRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageToBuyerRequestDescriptor = $convert.base64Decode(
    'ChlTZW5kTWVzc2FnZVRvQnV5ZXJSZXF1ZXN0Eh4KCGJ1eWVyX2lkGAEgASgJQgPgQQJSB2J1eW'
    'VySWQSKgoKcmVxdWVzdF9pZBgCIAEoCUIL4EEC4ozP1wgCCAFSCXJlcXVlc3RJZBJCCgdjb250'
    'ZW50GAMgASgLMiMuaGF3a3IuY2hhdC52MWFscGhhMS5NZXNzYWdlQ29udGVudEID4EECUgdjb2'
    '50ZW50');

@$core.Deprecated('Use sendMessageToBuyerResponseDescriptor instead')
const SendMessageToBuyerResponse$json = {
  '1': 'SendMessageToBuyerResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `SendMessageToBuyerResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageToBuyerResponseDescriptor =
    $convert.base64Decode(
        'ChpTZW5kTWVzc2FnZVRvQnV5ZXJSZXNwb25zZRI2CgdtZXNzYWdlGAEgASgLMhwuaGF3a3IuY2'
        'hhdC52MWFscGhhMS5NZXNzYWdlUgdtZXNzYWdl');

@$core.Deprecated('Use markConversationReadRequestDescriptor instead')
const MarkConversationReadRequest$json = {
  '1': 'MarkConversationReadRequest',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '8': {},
      '10': 'conversationId'
    },
    {'1': 'upto_seq', '3': 2, '4': 1, '5': 3, '8': {}, '10': 'uptoSeq'},
  ],
};

/// Descriptor for `MarkConversationReadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markConversationReadRequestDescriptor =
    $convert.base64Decode(
        'ChtNYXJrQ29udmVyc2F0aW9uUmVhZFJlcXVlc3QSLAoPY29udmVyc2F0aW9uX2lkGAEgASgJQg'
        'PgQQJSDmNvbnZlcnNhdGlvbklkEh4KCHVwdG9fc2VxGAIgASgDQgPgQQJSB3VwdG9TZXE=');

@$core.Deprecated('Use markConversationReadResponseDescriptor instead')
const MarkConversationReadResponse$json = {
  '1': 'MarkConversationReadResponse',
};

/// Descriptor for `MarkConversationReadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markConversationReadResponseDescriptor =
    $convert.base64Decode('ChxNYXJrQ29udmVyc2F0aW9uUmVhZFJlc3BvbnNl');

@$core.Deprecated('Use markAllReadRequestDescriptor instead')
const MarkAllReadRequest$json = {
  '1': 'MarkAllReadRequest',
};

/// Descriptor for `MarkAllReadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markAllReadRequestDescriptor =
    $convert.base64Decode('ChJNYXJrQWxsUmVhZFJlcXVlc3Q=');

@$core.Deprecated('Use markAllReadResponseDescriptor instead')
const MarkAllReadResponse$json = {
  '1': 'MarkAllReadResponse',
};

/// Descriptor for `MarkAllReadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markAllReadResponseDescriptor =
    $convert.base64Decode('ChNNYXJrQWxsUmVhZFJlc3BvbnNl');

@$core.Deprecated('Use markAllStoreReadRequestDescriptor instead')
const MarkAllStoreReadRequest$json = {
  '1': 'MarkAllStoreReadRequest',
};

/// Descriptor for `MarkAllStoreReadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markAllStoreReadRequestDescriptor =
    $convert.base64Decode('ChdNYXJrQWxsU3RvcmVSZWFkUmVxdWVzdA==');

@$core.Deprecated('Use markAllStoreReadResponseDescriptor instead')
const MarkAllStoreReadResponse$json = {
  '1': 'MarkAllStoreReadResponse',
};

/// Descriptor for `MarkAllStoreReadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markAllStoreReadResponseDescriptor =
    $convert.base64Decode('ChhNYXJrQWxsU3RvcmVSZWFkUmVzcG9uc2U=');

@$core.Deprecated('Use getUnreadCountRequestDescriptor instead')
const GetUnreadCountRequest$json = {
  '1': 'GetUnreadCountRequest',
};

/// Descriptor for `GetUnreadCountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUnreadCountRequestDescriptor =
    $convert.base64Decode('ChVHZXRVbnJlYWRDb3VudFJlcXVlc3Q=');

@$core.Deprecated('Use getUnreadCountResponseDescriptor instead')
const GetUnreadCountResponse$json = {
  '1': 'GetUnreadCountResponse',
  '2': [
    {'1': 'unread_count', '3': 1, '4': 1, '5': 3, '10': 'unreadCount'},
  ],
};

/// Descriptor for `GetUnreadCountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUnreadCountResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRVbnJlYWRDb3VudFJlc3BvbnNlEiEKDHVucmVhZF9jb3VudBgBIAEoA1ILdW5yZWFkQ2'
        '91bnQ=');

@$core.Deprecated('Use getStoreUnreadCountRequestDescriptor instead')
const GetStoreUnreadCountRequest$json = {
  '1': 'GetStoreUnreadCountRequest',
};

/// Descriptor for `GetStoreUnreadCountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreUnreadCountRequestDescriptor =
    $convert.base64Decode('ChpHZXRTdG9yZVVucmVhZENvdW50UmVxdWVzdA==');

@$core.Deprecated('Use getStoreUnreadCountResponseDescriptor instead')
const GetStoreUnreadCountResponse$json = {
  '1': 'GetStoreUnreadCountResponse',
  '2': [
    {'1': 'unread_count', '3': 1, '4': 1, '5': 3, '10': 'unreadCount'},
  ],
};

/// Descriptor for `GetStoreUnreadCountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoreUnreadCountResponseDescriptor =
    $convert.base64Decode(
        'ChtHZXRTdG9yZVVucmVhZENvdW50UmVzcG9uc2USIQoMdW5yZWFkX2NvdW50GAEgASgDUgt1bn'
        'JlYWRDb3VudA==');

@$core.Deprecated('Use conversationPresenceDescriptor instead')
const ConversationPresence$json = {
  '1': 'ConversationPresence',
  '2': [
    {'1': 'conversation_id', '3': 1, '4': 1, '5': 9, '10': 'conversationId'},
    {'1': 'online', '3': 2, '4': 1, '5': 8, '10': 'online'},
  ],
};

/// Descriptor for `ConversationPresence`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List conversationPresenceDescriptor = $convert.base64Decode(
    'ChRDb252ZXJzYXRpb25QcmVzZW5jZRInCg9jb252ZXJzYXRpb25faWQYASABKAlSDmNvbnZlcn'
    'NhdGlvbklkEhYKBm9ubGluZRgCIAEoCFIGb25saW5l');

@$core.Deprecated('Use getPresenceRequestDescriptor instead')
const GetPresenceRequest$json = {
  '1': 'GetPresenceRequest',
  '2': [
    {
      '1': 'conversation_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '8': {},
      '10': 'conversationIds'
    },
  ],
};

/// Descriptor for `GetPresenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPresenceRequestDescriptor = $convert.base64Decode(
    'ChJHZXRQcmVzZW5jZVJlcXVlc3QSLgoQY29udmVyc2F0aW9uX2lkcxgBIAMoCUID4EECUg9jb2'
    '52ZXJzYXRpb25JZHM=');

@$core.Deprecated('Use getPresenceResponseDescriptor instead')
const GetPresenceResponse$json = {
  '1': 'GetPresenceResponse',
  '2': [
    {
      '1': 'presences',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.ConversationPresence',
      '10': 'presences'
    },
  ],
};

/// Descriptor for `GetPresenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPresenceResponseDescriptor = $convert.base64Decode(
    'ChNHZXRQcmVzZW5jZVJlc3BvbnNlEkcKCXByZXNlbmNlcxgBIAMoCzIpLmhhd2tyLmNoYXQudj'
    'FhbHBoYTEuQ29udmVyc2F0aW9uUHJlc2VuY2VSCXByZXNlbmNlcw==');

@$core.Deprecated('Use getStorePresenceRequestDescriptor instead')
const GetStorePresenceRequest$json = {
  '1': 'GetStorePresenceRequest',
  '2': [
    {
      '1': 'conversation_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '8': {},
      '10': 'conversationIds'
    },
  ],
};

/// Descriptor for `GetStorePresenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStorePresenceRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRTdG9yZVByZXNlbmNlUmVxdWVzdBIuChBjb252ZXJzYXRpb25faWRzGAEgAygJQgPgQQ'
        'JSD2NvbnZlcnNhdGlvbklkcw==');

@$core.Deprecated('Use getStorePresenceResponseDescriptor instead')
const GetStorePresenceResponse$json = {
  '1': 'GetStorePresenceResponse',
  '2': [
    {
      '1': 'presences',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.ConversationPresence',
      '10': 'presences'
    },
  ],
};

/// Descriptor for `GetStorePresenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStorePresenceResponseDescriptor =
    $convert.base64Decode(
        'ChhHZXRTdG9yZVByZXNlbmNlUmVzcG9uc2USRwoJcHJlc2VuY2VzGAEgAygLMikuaGF3a3IuY2'
        'hhdC52MWFscGhhMS5Db252ZXJzYXRpb25QcmVzZW5jZVIJcHJlc2VuY2Vz');

@$core.Deprecated('Use streamEventsRequestDescriptor instead')
const StreamEventsRequest$json = {
  '1': 'StreamEventsRequest',
  '2': [
    {
      '1': 'cursors',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.StreamEventsRequest.CursorsEntry',
      '10': 'cursors'
    },
  ],
  '3': [StreamEventsRequest_CursorsEntry$json],
};

@$core.Deprecated('Use streamEventsRequestDescriptor instead')
const StreamEventsRequest_CursorsEntry$json = {
  '1': 'CursorsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 3, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `StreamEventsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List streamEventsRequestDescriptor = $convert.base64Decode(
    'ChNTdHJlYW1FdmVudHNSZXF1ZXN0Ek8KB2N1cnNvcnMYASADKAsyNS5oYXdrci5jaGF0LnYxYW'
    'xwaGExLlN0cmVhbUV2ZW50c1JlcXVlc3QuQ3Vyc29yc0VudHJ5UgdjdXJzb3JzGjoKDEN1cnNv'
    'cnNFbnRyeRIQCgNrZXkYASABKAlSA2tleRIUCgV2YWx1ZRgCIAEoA1IFdmFsdWU6AjgB');

@$core.Deprecated('Use streamStoreEventsRequestDescriptor instead')
const StreamStoreEventsRequest$json = {
  '1': 'StreamStoreEventsRequest',
  '2': [
    {
      '1': 'cursors',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.StreamStoreEventsRequest.CursorsEntry',
      '10': 'cursors'
    },
  ],
  '3': [StreamStoreEventsRequest_CursorsEntry$json],
};

@$core.Deprecated('Use streamStoreEventsRequestDescriptor instead')
const StreamStoreEventsRequest_CursorsEntry$json = {
  '1': 'CursorsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 3, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `StreamStoreEventsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List streamStoreEventsRequestDescriptor = $convert.base64Decode(
    'ChhTdHJlYW1TdG9yZUV2ZW50c1JlcXVlc3QSVAoHY3Vyc29ycxgBIAMoCzI6Lmhhd2tyLmNoYX'
    'QudjFhbHBoYTEuU3RyZWFtU3RvcmVFdmVudHNSZXF1ZXN0LkN1cnNvcnNFbnRyeVIHY3Vyc29y'
    'cxo6CgxDdXJzb3JzRW50cnkSEAoDa2V5GAEgASgJUgNrZXkSFAoFdmFsdWUYAiABKANSBXZhbH'
    'VlOgI4AQ==');

@$core.Deprecated('Use streamEventsResponseDescriptor instead')
const StreamEventsResponse$json = {
  '1': 'StreamEventsResponse',
  '2': [
    {
      '1': 'event',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.ChatEvent',
      '10': 'event'
    },
  ],
};

/// Descriptor for `StreamEventsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List streamEventsResponseDescriptor = $convert.base64Decode(
    'ChRTdHJlYW1FdmVudHNSZXNwb25zZRI0CgVldmVudBgBIAEoCzIeLmhhd2tyLmNoYXQudjFhbH'
    'BoYTEuQ2hhdEV2ZW50UgVldmVudA==');

@$core.Deprecated('Use streamStoreEventsResponseDescriptor instead')
const StreamStoreEventsResponse$json = {
  '1': 'StreamStoreEventsResponse',
  '2': [
    {
      '1': 'event',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.ChatEvent',
      '10': 'event'
    },
  ],
};

/// Descriptor for `StreamStoreEventsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List streamStoreEventsResponseDescriptor =
    $convert.base64Decode(
        'ChlTdHJlYW1TdG9yZUV2ZW50c1Jlc3BvbnNlEjQKBWV2ZW50GAEgASgLMh4uaGF3a3IuY2hhdC'
        '52MWFscGhhMS5DaGF0RXZlbnRSBWV2ZW50');

@$core.Deprecated('Use chatEventDescriptor instead')
const ChatEvent$json = {
  '1': 'ChatEvent',
  '2': [
    {
      '1': 'ready',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Ready',
      '9': 0,
      '10': 'ready'
    },
    {
      '1': 'message',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.MessageEvent',
      '9': 0,
      '10': 'message'
    },
    {
      '1': 'unread',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.UnreadChanged',
      '9': 0,
      '10': 'unread'
    },
    {
      '1': 'peer_read',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.PeerRead',
      '9': 0,
      '10': 'peerRead'
    },
    {
      '1': 'resync',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Resync',
      '9': 0,
      '10': 'resync'
    },
    {
      '1': 'heartbeat',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Heartbeat',
      '9': 0,
      '10': 'heartbeat'
    },
  ],
  '8': [
    {'1': 'event'},
  ],
};

/// Descriptor for `ChatEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatEventDescriptor = $convert.base64Decode(
    'CglDaGF0RXZlbnQSMgoFcmVhZHkYASABKAsyGi5oYXdrci5jaGF0LnYxYWxwaGExLlJlYWR5SA'
    'BSBXJlYWR5Ej0KB21lc3NhZ2UYAiABKAsyIS5oYXdrci5jaGF0LnYxYWxwaGExLk1lc3NhZ2VF'
    'dmVudEgAUgdtZXNzYWdlEjwKBnVucmVhZBgDIAEoCzIiLmhhd2tyLmNoYXQudjFhbHBoYTEuVW'
    '5yZWFkQ2hhbmdlZEgAUgZ1bnJlYWQSPAoJcGVlcl9yZWFkGAQgASgLMh0uaGF3a3IuY2hhdC52'
    'MWFscGhhMS5QZWVyUmVhZEgAUghwZWVyUmVhZBI1CgZyZXN5bmMYBSABKAsyGy5oYXdrci5jaG'
    'F0LnYxYWxwaGExLlJlc3luY0gAUgZyZXN5bmMSPgoJaGVhcnRiZWF0GAYgASgLMh4uaGF3a3Iu'
    'Y2hhdC52MWFscGhhMS5IZWFydGJlYXRIAFIJaGVhcnRiZWF0QgcKBWV2ZW50');

@$core.Deprecated('Use readyDescriptor instead')
const Ready$json = {
  '1': 'Ready',
  '2': [
    {'1': 'unread_count', '3': 1, '4': 1, '5': 3, '10': 'unreadCount'},
    {
      '1': 'heartbeat_interval',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '10': 'heartbeatInterval'
    },
    {
      '1': 'max_lifetime',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '10': 'maxLifetime'
    },
  ],
};

/// Descriptor for `Ready`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readyDescriptor = $convert.base64Decode(
    'CgVSZWFkeRIhCgx1bnJlYWRfY291bnQYASABKANSC3VucmVhZENvdW50EkgKEmhlYXJ0YmVhdF'
    '9pbnRlcnZhbBgCIAEoCzIZLmdvb2dsZS5wcm90b2J1Zi5EdXJhdGlvblIRaGVhcnRiZWF0SW50'
    'ZXJ2YWwSPAoMbWF4X2xpZmV0aW1lGAMgASgLMhkuZ29vZ2xlLnByb3RvYnVmLkR1cmF0aW9uUg'
    'ttYXhMaWZldGltZQ==');

@$core.Deprecated('Use messageEventDescriptor instead')
const MessageEvent$json = {
  '1': 'MessageEvent',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.hawkr.chat.v1alpha1.Message',
      '10': 'message'
    },
    {'1': 'updated', '3': 2, '4': 1, '5': 8, '10': 'updated'},
  ],
};

/// Descriptor for `MessageEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageEventDescriptor = $convert.base64Decode(
    'CgxNZXNzYWdlRXZlbnQSNgoHbWVzc2FnZRgBIAEoCzIcLmhhd2tyLmNoYXQudjFhbHBoYTEuTW'
    'Vzc2FnZVIHbWVzc2FnZRIYCgd1cGRhdGVkGAIgASgIUgd1cGRhdGVk');

@$core.Deprecated('Use unreadChangedDescriptor instead')
const UnreadChanged$json = {
  '1': 'UnreadChanged',
  '2': [
    {'1': 'unread_count', '3': 1, '4': 1, '5': 3, '10': 'unreadCount'},
  ],
};

/// Descriptor for `UnreadChanged`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unreadChangedDescriptor = $convert.base64Decode(
    'Cg1VbnJlYWRDaGFuZ2VkEiEKDHVucmVhZF9jb3VudBgBIAEoA1ILdW5yZWFkQ291bnQ=');

@$core.Deprecated('Use peerReadDescriptor instead')
const PeerRead$json = {
  '1': 'PeerRead',
  '2': [
    {'1': 'conversation_id', '3': 1, '4': 1, '5': 9, '10': 'conversationId'},
    {'1': 'peer_read_seq', '3': 2, '4': 1, '5': 3, '10': 'peerReadSeq'},
  ],
};

/// Descriptor for `PeerRead`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List peerReadDescriptor = $convert.base64Decode(
    'CghQZWVyUmVhZBInCg9jb252ZXJzYXRpb25faWQYASABKAlSDmNvbnZlcnNhdGlvbklkEiIKDX'
    'BlZXJfcmVhZF9zZXEYAiABKANSC3BlZXJSZWFkU2Vx');

@$core.Deprecated('Use resyncDescriptor instead')
const Resync$json = {
  '1': 'Resync',
  '2': [
    {
      '1': 'conversation_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'conversationId'
    },
    {'1': 'all', '3': 2, '4': 1, '5': 8, '9': 0, '10': 'all'},
  ],
  '8': [
    {'1': 'scope'},
  ],
};

/// Descriptor for `Resync`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resyncDescriptor = $convert.base64Decode(
    'CgZSZXN5bmMSKQoPY29udmVyc2F0aW9uX2lkGAEgASgJSABSDmNvbnZlcnNhdGlvbklkEhIKA2'
    'FsbBgCIAEoCEgAUgNhbGxCBwoFc2NvcGU=');

@$core.Deprecated('Use heartbeatDescriptor instead')
const Heartbeat$json = {
  '1': 'Heartbeat',
};

/// Descriptor for `Heartbeat`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List heartbeatDescriptor =
    $convert.base64Decode('CglIZWFydGJlYXQ=');

const $core.Map<$core.String, $core.dynamic> ChatAppServiceBase$json = {
  '1': 'ChatAppService',
  '2': [
    {
      '1': 'ListConversations',
      '2': '.hawkr.chat.v1alpha1.ListConversationsRequest',
      '3': '.hawkr.chat.v1alpha1.ListConversationsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListStoreConversations',
      '2': '.hawkr.chat.v1alpha1.ListStoreConversationsRequest',
      '3': '.hawkr.chat.v1alpha1.ListStoreConversationsResponse',
      '4': {'34': 1},
    },
    {
      '1': 'ListMessages',
      '2': '.hawkr.chat.v1alpha1.ListMessagesRequest',
      '3': '.hawkr.chat.v1alpha1.ListMessagesResponse',
      '4': {'34': 1},
    },
    {
      '1': 'SendMessage',
      '2': '.hawkr.chat.v1alpha1.SendMessageRequest',
      '3': '.hawkr.chat.v1alpha1.SendMessageResponse'
    },
    {
      '1': 'SendMessageToStore',
      '2': '.hawkr.chat.v1alpha1.SendMessageToStoreRequest',
      '3': '.hawkr.chat.v1alpha1.SendMessageToStoreResponse'
    },
    {
      '1': 'SendMessageToBuyer',
      '2': '.hawkr.chat.v1alpha1.SendMessageToBuyerRequest',
      '3': '.hawkr.chat.v1alpha1.SendMessageToBuyerResponse'
    },
    {
      '1': 'MarkConversationRead',
      '2': '.hawkr.chat.v1alpha1.MarkConversationReadRequest',
      '3': '.hawkr.chat.v1alpha1.MarkConversationReadResponse'
    },
    {
      '1': 'MarkAllRead',
      '2': '.hawkr.chat.v1alpha1.MarkAllReadRequest',
      '3': '.hawkr.chat.v1alpha1.MarkAllReadResponse'
    },
    {
      '1': 'MarkAllStoreRead',
      '2': '.hawkr.chat.v1alpha1.MarkAllStoreReadRequest',
      '3': '.hawkr.chat.v1alpha1.MarkAllStoreReadResponse'
    },
    {
      '1': 'GetUnreadCount',
      '2': '.hawkr.chat.v1alpha1.GetUnreadCountRequest',
      '3': '.hawkr.chat.v1alpha1.GetUnreadCountResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetStoreUnreadCount',
      '2': '.hawkr.chat.v1alpha1.GetStoreUnreadCountRequest',
      '3': '.hawkr.chat.v1alpha1.GetStoreUnreadCountResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetPresence',
      '2': '.hawkr.chat.v1alpha1.GetPresenceRequest',
      '3': '.hawkr.chat.v1alpha1.GetPresenceResponse',
      '4': {'34': 1},
    },
    {
      '1': 'GetStorePresence',
      '2': '.hawkr.chat.v1alpha1.GetStorePresenceRequest',
      '3': '.hawkr.chat.v1alpha1.GetStorePresenceResponse',
      '4': {'34': 1},
    },
    {
      '1': 'StreamEvents',
      '2': '.hawkr.chat.v1alpha1.StreamEventsRequest',
      '3': '.hawkr.chat.v1alpha1.StreamEventsResponse',
      '6': true
    },
    {
      '1': 'StreamStoreEvents',
      '2': '.hawkr.chat.v1alpha1.StreamStoreEventsRequest',
      '3': '.hawkr.chat.v1alpha1.StreamStoreEventsResponse',
      '6': true
    },
  ],
};

@$core.Deprecated('Use chatAppServiceDescriptor instead')
const $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
    ChatAppServiceBase$messageJson = {
  '.hawkr.chat.v1alpha1.ListConversationsRequest':
      ListConversationsRequest$json,
  '.hawkr.chat.v1alpha1.ListConversationsResponse':
      ListConversationsResponse$json,
  '.hawkr.chat.v1alpha1.Conversation': Conversation$json,
  '.hawkr.common.v1alpha1.StoreCard': $2.StoreCard$json,
  '.hawkr.common.v1alpha1.Image': $0.Image$json,
  '.hawkr.chat.v1alpha1.ChatBuyer': ChatBuyer$json,
  '.google.protobuf.Timestamp': $1.Timestamp$json,
  '.hawkr.chat.v1alpha1.Preview': Preview$json,
  '.hawkr.chat.v1alpha1.ListStoreConversationsRequest':
      ListStoreConversationsRequest$json,
  '.hawkr.chat.v1alpha1.ListStoreConversationsResponse':
      ListStoreConversationsResponse$json,
  '.hawkr.chat.v1alpha1.ListMessagesRequest': ListMessagesRequest$json,
  '.hawkr.chat.v1alpha1.ListMessagesResponse': ListMessagesResponse$json,
  '.hawkr.chat.v1alpha1.Message': Message$json,
  '.hawkr.chat.v1alpha1.SendMessageRequest': SendMessageRequest$json,
  '.hawkr.chat.v1alpha1.MessageContent': MessageContent$json,
  '.hawkr.chat.v1alpha1.SendMessageResponse': SendMessageResponse$json,
  '.hawkr.chat.v1alpha1.SendMessageToStoreRequest':
      SendMessageToStoreRequest$json,
  '.hawkr.chat.v1alpha1.SendMessageToStoreResponse':
      SendMessageToStoreResponse$json,
  '.hawkr.chat.v1alpha1.SendMessageToBuyerRequest':
      SendMessageToBuyerRequest$json,
  '.hawkr.chat.v1alpha1.SendMessageToBuyerResponse':
      SendMessageToBuyerResponse$json,
  '.hawkr.chat.v1alpha1.MarkConversationReadRequest':
      MarkConversationReadRequest$json,
  '.hawkr.chat.v1alpha1.MarkConversationReadResponse':
      MarkConversationReadResponse$json,
  '.hawkr.chat.v1alpha1.MarkAllReadRequest': MarkAllReadRequest$json,
  '.hawkr.chat.v1alpha1.MarkAllReadResponse': MarkAllReadResponse$json,
  '.hawkr.chat.v1alpha1.MarkAllStoreReadRequest': MarkAllStoreReadRequest$json,
  '.hawkr.chat.v1alpha1.MarkAllStoreReadResponse':
      MarkAllStoreReadResponse$json,
  '.hawkr.chat.v1alpha1.GetUnreadCountRequest': GetUnreadCountRequest$json,
  '.hawkr.chat.v1alpha1.GetUnreadCountResponse': GetUnreadCountResponse$json,
  '.hawkr.chat.v1alpha1.GetStoreUnreadCountRequest':
      GetStoreUnreadCountRequest$json,
  '.hawkr.chat.v1alpha1.GetStoreUnreadCountResponse':
      GetStoreUnreadCountResponse$json,
  '.hawkr.chat.v1alpha1.GetPresenceRequest': GetPresenceRequest$json,
  '.hawkr.chat.v1alpha1.GetPresenceResponse': GetPresenceResponse$json,
  '.hawkr.chat.v1alpha1.ConversationPresence': ConversationPresence$json,
  '.hawkr.chat.v1alpha1.GetStorePresenceRequest': GetStorePresenceRequest$json,
  '.hawkr.chat.v1alpha1.GetStorePresenceResponse':
      GetStorePresenceResponse$json,
  '.hawkr.chat.v1alpha1.StreamEventsRequest': StreamEventsRequest$json,
  '.hawkr.chat.v1alpha1.StreamEventsRequest.CursorsEntry':
      StreamEventsRequest_CursorsEntry$json,
  '.hawkr.chat.v1alpha1.StreamEventsResponse': StreamEventsResponse$json,
  '.hawkr.chat.v1alpha1.ChatEvent': ChatEvent$json,
  '.hawkr.chat.v1alpha1.Ready': Ready$json,
  '.google.protobuf.Duration': $3.Duration$json,
  '.hawkr.chat.v1alpha1.MessageEvent': MessageEvent$json,
  '.hawkr.chat.v1alpha1.UnreadChanged': UnreadChanged$json,
  '.hawkr.chat.v1alpha1.PeerRead': PeerRead$json,
  '.hawkr.chat.v1alpha1.Resync': Resync$json,
  '.hawkr.chat.v1alpha1.Heartbeat': Heartbeat$json,
  '.hawkr.chat.v1alpha1.StreamStoreEventsRequest':
      StreamStoreEventsRequest$json,
  '.hawkr.chat.v1alpha1.StreamStoreEventsRequest.CursorsEntry':
      StreamStoreEventsRequest_CursorsEntry$json,
  '.hawkr.chat.v1alpha1.StreamStoreEventsResponse':
      StreamStoreEventsResponse$json,
};

/// Descriptor for `ChatAppService`. Decode as a `google.protobuf.ServiceDescriptorProto`.
final $typed_data.Uint8List chatAppServiceDescriptor = $convert.base64Decode(
    'Cg5DaGF0QXBwU2VydmljZRJ3ChFMaXN0Q29udmVyc2F0aW9ucxItLmhhd2tyLmNoYXQudjFhbH'
    'BoYTEuTGlzdENvbnZlcnNhdGlvbnNSZXF1ZXN0Gi4uaGF3a3IuY2hhdC52MWFscGhhMS5MaXN0'
    'Q29udmVyc2F0aW9uc1Jlc3BvbnNlIgOQAgEShgEKFkxpc3RTdG9yZUNvbnZlcnNhdGlvbnMSMi'
    '5oYXdrci5jaGF0LnYxYWxwaGExLkxpc3RTdG9yZUNvbnZlcnNhdGlvbnNSZXF1ZXN0GjMuaGF3'
    'a3IuY2hhdC52MWFscGhhMS5MaXN0U3RvcmVDb252ZXJzYXRpb25zUmVzcG9uc2UiA5ACARJoCg'
    'xMaXN0TWVzc2FnZXMSKC5oYXdrci5jaGF0LnYxYWxwaGExLkxpc3RNZXNzYWdlc1JlcXVlc3Qa'
    'KS5oYXdrci5jaGF0LnYxYWxwaGExLkxpc3RNZXNzYWdlc1Jlc3BvbnNlIgOQAgESYAoLU2VuZE'
    '1lc3NhZ2USJy5oYXdrci5jaGF0LnYxYWxwaGExLlNlbmRNZXNzYWdlUmVxdWVzdBooLmhhd2ty'
    'LmNoYXQudjFhbHBoYTEuU2VuZE1lc3NhZ2VSZXNwb25zZRJ1ChJTZW5kTWVzc2FnZVRvU3Rvcm'
    'USLi5oYXdrci5jaGF0LnYxYWxwaGExLlNlbmRNZXNzYWdlVG9TdG9yZVJlcXVlc3QaLy5oYXdr'
    'ci5jaGF0LnYxYWxwaGExLlNlbmRNZXNzYWdlVG9TdG9yZVJlc3BvbnNlEnUKElNlbmRNZXNzYW'
    'dlVG9CdXllchIuLmhhd2tyLmNoYXQudjFhbHBoYTEuU2VuZE1lc3NhZ2VUb0J1eWVyUmVxdWVz'
    'dBovLmhhd2tyLmNoYXQudjFhbHBoYTEuU2VuZE1lc3NhZ2VUb0J1eWVyUmVzcG9uc2USewoUTW'
    'Fya0NvbnZlcnNhdGlvblJlYWQSMC5oYXdrci5jaGF0LnYxYWxwaGExLk1hcmtDb252ZXJzYXRp'
    'b25SZWFkUmVxdWVzdBoxLmhhd2tyLmNoYXQudjFhbHBoYTEuTWFya0NvbnZlcnNhdGlvblJlYW'
    'RSZXNwb25zZRJgCgtNYXJrQWxsUmVhZBInLmhhd2tyLmNoYXQudjFhbHBoYTEuTWFya0FsbFJl'
    'YWRSZXF1ZXN0GiguaGF3a3IuY2hhdC52MWFscGhhMS5NYXJrQWxsUmVhZFJlc3BvbnNlEm8KEE'
    '1hcmtBbGxTdG9yZVJlYWQSLC5oYXdrci5jaGF0LnYxYWxwaGExLk1hcmtBbGxTdG9yZVJlYWRS'
    'ZXF1ZXN0Gi0uaGF3a3IuY2hhdC52MWFscGhhMS5NYXJrQWxsU3RvcmVSZWFkUmVzcG9uc2USbg'
    'oOR2V0VW5yZWFkQ291bnQSKi5oYXdrci5jaGF0LnYxYWxwaGExLkdldFVucmVhZENvdW50UmVx'
    'dWVzdBorLmhhd2tyLmNoYXQudjFhbHBoYTEuR2V0VW5yZWFkQ291bnRSZXNwb25zZSIDkAIBEn'
    '0KE0dldFN0b3JlVW5yZWFkQ291bnQSLy5oYXdrci5jaGF0LnYxYWxwaGExLkdldFN0b3JlVW5y'
    'ZWFkQ291bnRSZXF1ZXN0GjAuaGF3a3IuY2hhdC52MWFscGhhMS5HZXRTdG9yZVVucmVhZENvdW'
    '50UmVzcG9uc2UiA5ACARJlCgtHZXRQcmVzZW5jZRInLmhhd2tyLmNoYXQudjFhbHBoYTEuR2V0'
    'UHJlc2VuY2VSZXF1ZXN0GiguaGF3a3IuY2hhdC52MWFscGhhMS5HZXRQcmVzZW5jZVJlc3Bvbn'
    'NlIgOQAgESdAoQR2V0U3RvcmVQcmVzZW5jZRIsLmhhd2tyLmNoYXQudjFhbHBoYTEuR2V0U3Rv'
    'cmVQcmVzZW5jZVJlcXVlc3QaLS5oYXdrci5jaGF0LnYxYWxwaGExLkdldFN0b3JlUHJlc2VuY2'
    'VSZXNwb25zZSIDkAIBEmUKDFN0cmVhbUV2ZW50cxIoLmhhd2tyLmNoYXQudjFhbHBoYTEuU3Ry'
    'ZWFtRXZlbnRzUmVxdWVzdBopLmhhd2tyLmNoYXQudjFhbHBoYTEuU3RyZWFtRXZlbnRzUmVzcG'
    '9uc2UwARJ0ChFTdHJlYW1TdG9yZUV2ZW50cxItLmhhd2tyLmNoYXQudjFhbHBoYTEuU3RyZWFt'
    'U3RvcmVFdmVudHNSZXF1ZXN0Gi4uaGF3a3IuY2hhdC52MWFscGhhMS5TdHJlYW1TdG9yZUV2ZW'
    '50c1Jlc3BvbnNlMAE=');
