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

import 'chat.pb.dart' as $4;
import 'chat.pbjson.dart';

export 'chat.pb.dart';

abstract class ChatAppServiceBase extends $pb.GeneratedService {
  $async.Future<$4.ListConversationsResponse> listConversations(
      $pb.ServerContext ctx, $4.ListConversationsRequest request);
  $async.Future<$4.ListStoreConversationsResponse> listStoreConversations(
      $pb.ServerContext ctx, $4.ListStoreConversationsRequest request);
  $async.Future<$4.GetStoreConversationByBuyerResponse>
      getStoreConversationByBuyer(
          $pb.ServerContext ctx, $4.GetStoreConversationByBuyerRequest request);
  $async.Future<$4.ListMessagesResponse> listMessages(
      $pb.ServerContext ctx, $4.ListMessagesRequest request);
  $async.Future<$4.SendMessageResponse> sendMessage(
      $pb.ServerContext ctx, $4.SendMessageRequest request);
  $async.Future<$4.SendMessageToStoreResponse> sendMessageToStore(
      $pb.ServerContext ctx, $4.SendMessageToStoreRequest request);
  $async.Future<$4.SendMessageToBuyerResponse> sendMessageToBuyer(
      $pb.ServerContext ctx, $4.SendMessageToBuyerRequest request);
  $async.Future<$4.MarkConversationReadResponse> markConversationRead(
      $pb.ServerContext ctx, $4.MarkConversationReadRequest request);
  $async.Future<$4.MarkAllReadResponse> markAllRead(
      $pb.ServerContext ctx, $4.MarkAllReadRequest request);
  $async.Future<$4.MarkAllStoreReadResponse> markAllStoreRead(
      $pb.ServerContext ctx, $4.MarkAllStoreReadRequest request);
  $async.Future<$4.GetUnreadCountResponse> getUnreadCount(
      $pb.ServerContext ctx, $4.GetUnreadCountRequest request);
  $async.Future<$4.GetStoreUnreadCountResponse> getStoreUnreadCount(
      $pb.ServerContext ctx, $4.GetStoreUnreadCountRequest request);
  $async.Future<$4.GetPresenceResponse> getPresence(
      $pb.ServerContext ctx, $4.GetPresenceRequest request);
  $async.Future<$4.GetStorePresenceResponse> getStorePresence(
      $pb.ServerContext ctx, $4.GetStorePresenceRequest request);
  $async.Future<$4.StreamEventsResponse> streamEvents(
      $pb.ServerContext ctx, $4.StreamEventsRequest request);
  $async.Future<$4.StreamStoreEventsResponse> streamStoreEvents(
      $pb.ServerContext ctx, $4.StreamStoreEventsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListConversations':
        return $4.ListConversationsRequest();
      case 'ListStoreConversations':
        return $4.ListStoreConversationsRequest();
      case 'GetStoreConversationByBuyer':
        return $4.GetStoreConversationByBuyerRequest();
      case 'ListMessages':
        return $4.ListMessagesRequest();
      case 'SendMessage':
        return $4.SendMessageRequest();
      case 'SendMessageToStore':
        return $4.SendMessageToStoreRequest();
      case 'SendMessageToBuyer':
        return $4.SendMessageToBuyerRequest();
      case 'MarkConversationRead':
        return $4.MarkConversationReadRequest();
      case 'MarkAllRead':
        return $4.MarkAllReadRequest();
      case 'MarkAllStoreRead':
        return $4.MarkAllStoreReadRequest();
      case 'GetUnreadCount':
        return $4.GetUnreadCountRequest();
      case 'GetStoreUnreadCount':
        return $4.GetStoreUnreadCountRequest();
      case 'GetPresence':
        return $4.GetPresenceRequest();
      case 'GetStorePresence':
        return $4.GetStorePresenceRequest();
      case 'StreamEvents':
        return $4.StreamEventsRequest();
      case 'StreamStoreEvents':
        return $4.StreamStoreEventsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListConversations':
        return listConversations(ctx, request as $4.ListConversationsRequest);
      case 'ListStoreConversations':
        return listStoreConversations(
            ctx, request as $4.ListStoreConversationsRequest);
      case 'GetStoreConversationByBuyer':
        return getStoreConversationByBuyer(
            ctx, request as $4.GetStoreConversationByBuyerRequest);
      case 'ListMessages':
        return listMessages(ctx, request as $4.ListMessagesRequest);
      case 'SendMessage':
        return sendMessage(ctx, request as $4.SendMessageRequest);
      case 'SendMessageToStore':
        return sendMessageToStore(ctx, request as $4.SendMessageToStoreRequest);
      case 'SendMessageToBuyer':
        return sendMessageToBuyer(ctx, request as $4.SendMessageToBuyerRequest);
      case 'MarkConversationRead':
        return markConversationRead(
            ctx, request as $4.MarkConversationReadRequest);
      case 'MarkAllRead':
        return markAllRead(ctx, request as $4.MarkAllReadRequest);
      case 'MarkAllStoreRead':
        return markAllStoreRead(ctx, request as $4.MarkAllStoreReadRequest);
      case 'GetUnreadCount':
        return getUnreadCount(ctx, request as $4.GetUnreadCountRequest);
      case 'GetStoreUnreadCount':
        return getStoreUnreadCount(
            ctx, request as $4.GetStoreUnreadCountRequest);
      case 'GetPresence':
        return getPresence(ctx, request as $4.GetPresenceRequest);
      case 'GetStorePresence':
        return getStorePresence(ctx, request as $4.GetStorePresenceRequest);
      case 'StreamEvents':
        return streamEvents(ctx, request as $4.StreamEventsRequest);
      case 'StreamStoreEvents':
        return streamStoreEvents(ctx, request as $4.StreamStoreEventsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => ChatAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ChatAppServiceBase$messageJson;
}
