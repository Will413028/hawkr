// This is a generated file - do not edit.
//
// Generated from hawkr/discovery/v1alpha1/discovery.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'discovery.pb.dart' as $2;
import 'discovery.pbjson.dart';

export 'discovery.pb.dart';

abstract class DiscoveryPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$2.SuggestRecommendedSellersResponse> suggestRecommendedSellers(
      $pb.ServerContext ctx, $2.SuggestRecommendedSellersRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SuggestRecommendedSellers':
        return $2.SuggestRecommendedSellersRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SuggestRecommendedSellers':
        return suggestRecommendedSellers(
            ctx, request as $2.SuggestRecommendedSellersRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      DiscoveryPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => DiscoveryPublicServiceBase$messageJson;
}
