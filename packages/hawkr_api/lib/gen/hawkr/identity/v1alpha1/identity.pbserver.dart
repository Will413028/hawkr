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

import 'identity.pb.dart' as $2;
import 'identity.pbjson.dart';

export 'identity.pb.dart';

abstract class IdentityPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$2.IssueAppLoginNonceResponse> issueAppLoginNonce(
      $pb.ServerContext ctx, $2.IssueAppLoginNonceRequest request);
  $async.Future<$2.ExchangeIdTokenResponse> exchangeIdToken(
      $pb.ServerContext ctx, $2.ExchangeIdTokenRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'IssueAppLoginNonce':
        return $2.IssueAppLoginNonceRequest();
      case 'ExchangeIdToken':
        return $2.ExchangeIdTokenRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'IssueAppLoginNonce':
        return issueAppLoginNonce(ctx, request as $2.IssueAppLoginNonceRequest);
      case 'ExchangeIdToken':
        return exchangeIdToken(ctx, request as $2.ExchangeIdTokenRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      IdentityPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => IdentityPublicServiceBase$messageJson;
}

abstract class IdentityAppServiceBase extends $pb.GeneratedService {
  $async.Future<$2.GetSessionResponse> getSession(
      $pb.ServerContext ctx, $2.GetSessionRequest request);
  $async.Future<$2.LinkAppleCredentialResponse> linkAppleCredential(
      $pb.ServerContext ctx, $2.LinkAppleCredentialRequest request);
  $async.Future<$2.SignOutResponse> signOut(
      $pb.ServerContext ctx, $2.SignOutRequest request);
  $async.Future<$2.LinkIdentityResponse> linkIdentity(
      $pb.ServerContext ctx, $2.LinkIdentityRequest request);
  $async.Future<$2.ListSessionsResponse> listSessions(
      $pb.ServerContext ctx, $2.ListSessionsRequest request);
  $async.Future<$2.ListLinkedIdentitiesResponse> listLinkedIdentities(
      $pb.ServerContext ctx, $2.ListLinkedIdentitiesRequest request);
  $async.Future<$2.RevokeSessionResponse> revokeSession(
      $pb.ServerContext ctx, $2.RevokeSessionRequest request);
  $async.Future<$2.DeleteAccountResponse> deleteAccount(
      $pb.ServerContext ctx, $2.DeleteAccountRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetSession':
        return $2.GetSessionRequest();
      case 'LinkAppleCredential':
        return $2.LinkAppleCredentialRequest();
      case 'SignOut':
        return $2.SignOutRequest();
      case 'LinkIdentity':
        return $2.LinkIdentityRequest();
      case 'ListSessions':
        return $2.ListSessionsRequest();
      case 'ListLinkedIdentities':
        return $2.ListLinkedIdentitiesRequest();
      case 'RevokeSession':
        return $2.RevokeSessionRequest();
      case 'DeleteAccount':
        return $2.DeleteAccountRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetSession':
        return getSession(ctx, request as $2.GetSessionRequest);
      case 'LinkAppleCredential':
        return linkAppleCredential(
            ctx, request as $2.LinkAppleCredentialRequest);
      case 'SignOut':
        return signOut(ctx, request as $2.SignOutRequest);
      case 'LinkIdentity':
        return linkIdentity(ctx, request as $2.LinkIdentityRequest);
      case 'ListSessions':
        return listSessions(ctx, request as $2.ListSessionsRequest);
      case 'ListLinkedIdentities':
        return listLinkedIdentities(
            ctx, request as $2.ListLinkedIdentitiesRequest);
      case 'RevokeSession':
        return revokeSession(ctx, request as $2.RevokeSessionRequest);
      case 'DeleteAccount':
        return deleteAccount(ctx, request as $2.DeleteAccountRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      IdentityAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => IdentityAppServiceBase$messageJson;
}
