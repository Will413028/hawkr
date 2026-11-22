// This is a generated file - do not edit.
//
// Generated from hawkr/moderation/v1alpha1/moderation.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'moderation.pb.dart' as $3;
import 'moderation.pbjson.dart';

export 'moderation.pb.dart';

abstract class ModerationAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.SubmitReportResponse> submitReport(
      $pb.ServerContext ctx, $3.SubmitReportRequest request);
  $async.Future<$3.ListMyReportsResponse> listMyReports(
      $pb.ServerContext ctx, $3.ListMyReportsRequest request);
  $async.Future<$3.ListMyStoreReportsResponse> listMyStoreReports(
      $pb.ServerContext ctx, $3.ListMyStoreReportsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SubmitReport':
        return $3.SubmitReportRequest();
      case 'ListMyReports':
        return $3.ListMyReportsRequest();
      case 'ListMyStoreReports':
        return $3.ListMyStoreReportsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SubmitReport':
        return submitReport(ctx, request as $3.SubmitReportRequest);
      case 'ListMyReports':
        return listMyReports(ctx, request as $3.ListMyReportsRequest);
      case 'ListMyStoreReports':
        return listMyStoreReports(ctx, request as $3.ListMyStoreReportsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      ModerationAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ModerationAppServiceBase$messageJson;
}
