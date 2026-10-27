// This is a generated file - do not edit.
//
// Generated from hawkr/agent/v1alpha1/agent.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'agent.pb.dart' as $1;
import 'agent.pbjson.dart';

export 'agent.pb.dart';

abstract class VendorPortalPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$1.PreviewInviteResponse> previewInvite(
      $pb.ServerContext ctx, $1.PreviewInviteRequest request);
  $async.Future<$1.ExchangeInviteResponse> exchangeInvite(
      $pb.ServerContext ctx, $1.ExchangeInviteRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'PreviewInvite':
        return $1.PreviewInviteRequest();
      case 'ExchangeInvite':
        return $1.ExchangeInviteRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'PreviewInvite':
        return previewInvite(ctx, request as $1.PreviewInviteRequest);
      case 'ExchangeInvite':
        return exchangeInvite(ctx, request as $1.ExchangeInviteRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      VendorPortalPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => VendorPortalPublicServiceBase$messageJson;
}

abstract class VendorPortalInviteeServiceBase extends $pb.GeneratedService {
  $async.Future<$1.GetSessionResponse> getSession(
      $pb.ServerContext ctx, $1.GetSessionRequest request);
  $async.Future<$1.VerifySessionResponse> verifySession(
      $pb.ServerContext ctx, $1.VerifySessionRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetSession':
        return $1.GetSessionRequest();
      case 'VerifySession':
        return $1.VerifySessionRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetSession':
        return getSession(ctx, request as $1.GetSessionRequest);
      case 'VerifySession':
        return verifySession(ctx, request as $1.VerifySessionRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      VendorPortalInviteeServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => VendorPortalInviteeServiceBase$messageJson;
}
