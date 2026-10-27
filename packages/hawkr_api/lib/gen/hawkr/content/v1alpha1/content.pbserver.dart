// This is a generated file - do not edit.
//
// Generated from hawkr/content/v1alpha1/content.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'content.pb.dart' as $1;
import 'content.pbjson.dart';

export 'content.pb.dart';

abstract class ContentPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$1.ListLiveBannersResponse> listLiveBanners(
      $pb.ServerContext ctx, $1.ListLiveBannersRequest request);
  $async.Future<$1.CheckAppVersionResponse> checkAppVersion(
      $pb.ServerContext ctx, $1.CheckAppVersionRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListLiveBanners':
        return $1.ListLiveBannersRequest();
      case 'CheckAppVersion':
        return $1.CheckAppVersionRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListLiveBanners':
        return listLiveBanners(ctx, request as $1.ListLiveBannersRequest);
      case 'CheckAppVersion':
        return checkAppVersion(ctx, request as $1.CheckAppVersionRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      ContentPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ContentPublicServiceBase$messageJson;
}
