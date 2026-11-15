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

import 'identity.pb.dart' as $4;
import 'identity.pbjson.dart';

export 'identity.pb.dart';

abstract class IdentityPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$4.IssueAppLoginNonceResponse> issueAppLoginNonce(
      $pb.ServerContext ctx, $4.IssueAppLoginNonceRequest request);
  $async.Future<$4.ExchangeIdTokenResponse> exchangeIdToken(
      $pb.ServerContext ctx, $4.ExchangeIdTokenRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'IssueAppLoginNonce':
        return $4.IssueAppLoginNonceRequest();
      case 'ExchangeIdToken':
        return $4.ExchangeIdTokenRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'IssueAppLoginNonce':
        return issueAppLoginNonce(ctx, request as $4.IssueAppLoginNonceRequest);
      case 'ExchangeIdToken':
        return exchangeIdToken(ctx, request as $4.ExchangeIdTokenRequest);
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
  $async.Future<$4.GetSessionResponse> getSession(
      $pb.ServerContext ctx, $4.GetSessionRequest request);
  $async.Future<$4.SignOutResponse> signOut(
      $pb.ServerContext ctx, $4.SignOutRequest request);
  $async.Future<$4.LinkIdentityResponse> linkIdentity(
      $pb.ServerContext ctx, $4.LinkIdentityRequest request);
  $async.Future<$4.ListSessionsResponse> listSessions(
      $pb.ServerContext ctx, $4.ListSessionsRequest request);
  $async.Future<$4.ListLinkedIdentitiesResponse> listLinkedIdentities(
      $pb.ServerContext ctx, $4.ListLinkedIdentitiesRequest request);
  $async.Future<$4.RevokeSessionResponse> revokeSession(
      $pb.ServerContext ctx, $4.RevokeSessionRequest request);
  $async.Future<$4.DeleteAccountResponse> deleteAccount(
      $pb.ServerContext ctx, $4.DeleteAccountRequest request);
  $async.Future<$4.GetContactEmailResponse> getContactEmail(
      $pb.ServerContext ctx, $4.GetContactEmailRequest request);
  $async.Future<$4.SetContactEmailResponse> setContactEmail(
      $pb.ServerContext ctx, $4.SetContactEmailRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetSession':
        return $4.GetSessionRequest();
      case 'SignOut':
        return $4.SignOutRequest();
      case 'LinkIdentity':
        return $4.LinkIdentityRequest();
      case 'ListSessions':
        return $4.ListSessionsRequest();
      case 'ListLinkedIdentities':
        return $4.ListLinkedIdentitiesRequest();
      case 'RevokeSession':
        return $4.RevokeSessionRequest();
      case 'DeleteAccount':
        return $4.DeleteAccountRequest();
      case 'GetContactEmail':
        return $4.GetContactEmailRequest();
      case 'SetContactEmail':
        return $4.SetContactEmailRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetSession':
        return getSession(ctx, request as $4.GetSessionRequest);
      case 'SignOut':
        return signOut(ctx, request as $4.SignOutRequest);
      case 'LinkIdentity':
        return linkIdentity(ctx, request as $4.LinkIdentityRequest);
      case 'ListSessions':
        return listSessions(ctx, request as $4.ListSessionsRequest);
      case 'ListLinkedIdentities':
        return listLinkedIdentities(
            ctx, request as $4.ListLinkedIdentitiesRequest);
      case 'RevokeSession':
        return revokeSession(ctx, request as $4.RevokeSessionRequest);
      case 'DeleteAccount':
        return deleteAccount(ctx, request as $4.DeleteAccountRequest);
      case 'GetContactEmail':
        return getContactEmail(ctx, request as $4.GetContactEmailRequest);
      case 'SetContactEmail':
        return setContactEmail(ctx, request as $4.SetContactEmailRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      IdentityAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => IdentityAppServiceBase$messageJson;
}
