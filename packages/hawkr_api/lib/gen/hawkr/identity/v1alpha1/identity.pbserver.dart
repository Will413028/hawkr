// This is a generated file - do not edit.
//
// Generated from hawkr/identity/v1alpha1/identity.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'identity.pb.dart' as $1;
import 'identity.pbjson.dart';

export 'identity.pb.dart';

abstract class IdentityAppServiceBase extends $pb.GeneratedService {
  $async.Future<$1.GetSessionResponse> getSession(
      $pb.ServerContext ctx, $1.GetSessionRequest request);
  $async.Future<$1.LinkAppleCredentialResponse> linkAppleCredential(
      $pb.ServerContext ctx, $1.LinkAppleCredentialRequest request);
  $async.Future<$1.DeleteAccountResponse> deleteAccount(
      $pb.ServerContext ctx, $1.DeleteAccountRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetSession':
        return $1.GetSessionRequest();
      case 'LinkAppleCredential':
        return $1.LinkAppleCredentialRequest();
      case 'DeleteAccount':
        return $1.DeleteAccountRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetSession':
        return getSession(ctx, request as $1.GetSessionRequest);
      case 'LinkAppleCredential':
        return linkAppleCredential(
            ctx, request as $1.LinkAppleCredentialRequest);
      case 'DeleteAccount':
        return deleteAccount(ctx, request as $1.DeleteAccountRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      IdentityAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => IdentityAppServiceBase$messageJson;
}
