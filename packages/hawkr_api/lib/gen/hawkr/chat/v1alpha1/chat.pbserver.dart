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

import 'package:protobuf/protobuf.dart' as $pb;

import 'chat.pb.dart' as $3;
import 'chat.pbjson.dart';

export 'chat.pb.dart';

abstract class ChatAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.ListConversationsResponse> listConversations(
      $pb.ServerContext ctx, $3.ListConversationsRequest request);
  $async.Future<$3.ListStoreConversationsResponse> listStoreConversations(
      $pb.ServerContext ctx, $3.ListStoreConversationsRequest request);
  $async.Future<$3.ListMessagesResponse> listMessages(
      $pb.ServerContext ctx, $3.ListMessagesRequest request);
  $async.Future<$3.SendMessageResponse> sendMessage(
      $pb.ServerContext ctx, $3.SendMessageRequest request);
  $async.Future<$3.SendMessageToStoreResponse> sendMessageToStore(
      $pb.ServerContext ctx, $3.SendMessageToStoreRequest request);
  $async.Future<$3.SendMessageToBuyerResponse> sendMessageToBuyer(
      $pb.ServerContext ctx, $3.SendMessageToBuyerRequest request);
  $async.Future<$3.MarkConversationReadResponse> markConversationRead(
      $pb.ServerContext ctx, $3.MarkConversationReadRequest request);
  $async.Future<$3.MarkAllReadResponse> markAllRead(
      $pb.ServerContext ctx, $3.MarkAllReadRequest request);
  $async.Future<$3.MarkAllStoreReadResponse> markAllStoreRead(
      $pb.ServerContext ctx, $3.MarkAllStoreReadRequest request);
  $async.Future<$3.GetUnreadCountResponse> getUnreadCount(
      $pb.ServerContext ctx, $3.GetUnreadCountRequest request);
  $async.Future<$3.GetStoreUnreadCountResponse> getStoreUnreadCount(
      $pb.ServerContext ctx, $3.GetStoreUnreadCountRequest request);
  $async.Future<$3.GetPresenceResponse> getPresence(
      $pb.ServerContext ctx, $3.GetPresenceRequest request);
  $async.Future<$3.GetStorePresenceResponse> getStorePresence(
      $pb.ServerContext ctx, $3.GetStorePresenceRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListConversations':
        return $3.ListConversationsRequest();
      case 'ListStoreConversations':
        return $3.ListStoreConversationsRequest();
      case 'ListMessages':
        return $3.ListMessagesRequest();
      case 'SendMessage':
        return $3.SendMessageRequest();
      case 'SendMessageToStore':
        return $3.SendMessageToStoreRequest();
      case 'SendMessageToBuyer':
        return $3.SendMessageToBuyerRequest();
      case 'MarkConversationRead':
        return $3.MarkConversationReadRequest();
      case 'MarkAllRead':
        return $3.MarkAllReadRequest();
      case 'MarkAllStoreRead':
        return $3.MarkAllStoreReadRequest();
      case 'GetUnreadCount':
        return $3.GetUnreadCountRequest();
      case 'GetStoreUnreadCount':
        return $3.GetStoreUnreadCountRequest();
      case 'GetPresence':
        return $3.GetPresenceRequest();
      case 'GetStorePresence':
        return $3.GetStorePresenceRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListConversations':
        return listConversations(ctx, request as $3.ListConversationsRequest);
      case 'ListStoreConversations':
        return listStoreConversations(
            ctx, request as $3.ListStoreConversationsRequest);
      case 'ListMessages':
        return listMessages(ctx, request as $3.ListMessagesRequest);
      case 'SendMessage':
        return sendMessage(ctx, request as $3.SendMessageRequest);
      case 'SendMessageToStore':
        return sendMessageToStore(ctx, request as $3.SendMessageToStoreRequest);
      case 'SendMessageToBuyer':
        return sendMessageToBuyer(ctx, request as $3.SendMessageToBuyerRequest);
      case 'MarkConversationRead':
        return markConversationRead(
            ctx, request as $3.MarkConversationReadRequest);
      case 'MarkAllRead':
        return markAllRead(ctx, request as $3.MarkAllReadRequest);
      case 'MarkAllStoreRead':
        return markAllStoreRead(ctx, request as $3.MarkAllStoreReadRequest);
      case 'GetUnreadCount':
        return getUnreadCount(ctx, request as $3.GetUnreadCountRequest);
      case 'GetStoreUnreadCount':
        return getStoreUnreadCount(
            ctx, request as $3.GetStoreUnreadCountRequest);
      case 'GetPresence':
        return getPresence(ctx, request as $3.GetPresenceRequest);
      case 'GetStorePresence':
        return getStorePresence(ctx, request as $3.GetStorePresenceRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => ChatAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ChatAppServiceBase$messageJson;
}
