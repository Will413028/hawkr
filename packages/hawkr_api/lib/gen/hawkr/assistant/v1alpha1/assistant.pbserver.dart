// This is a generated file - do not edit.
//
// Generated from hawkr/assistant/v1alpha1/assistant.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'assistant.pb.dart' as $3;
import 'assistant.pbjson.dart';

export 'assistant.pb.dart';

abstract class AssistantAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.SendAssistantMessageResponse> sendAssistantMessage(
      $pb.ServerContext ctx, $3.SendAssistantMessageRequest request);
  $async.Future<$3.WatchAssistantTurnResponse> watchAssistantTurn(
      $pb.ServerContext ctx, $3.WatchAssistantTurnRequest request);
  $async.Future<$3.GetLatestAssistantSessionResponse> getLatestAssistantSession(
      $pb.ServerContext ctx, $3.GetLatestAssistantSessionRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SendAssistantMessage':
        return $3.SendAssistantMessageRequest();
      case 'WatchAssistantTurn':
        return $3.WatchAssistantTurnRequest();
      case 'GetLatestAssistantSession':
        return $3.GetLatestAssistantSessionRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SendAssistantMessage':
        return sendAssistantMessage(
            ctx, request as $3.SendAssistantMessageRequest);
      case 'WatchAssistantTurn':
        return watchAssistantTurn(ctx, request as $3.WatchAssistantTurnRequest);
      case 'GetLatestAssistantSession':
        return getLatestAssistantSession(
            ctx, request as $3.GetLatestAssistantSessionRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      AssistantAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => AssistantAppServiceBase$messageJson;
}
