// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/report.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'report.pb.dart' as $3;
import 'report.pbjson.dart';

export 'report.pb.dart';

abstract class SellerReportAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.CreateReportResponse> createReport(
      $pb.ServerContext ctx, $3.CreateReportRequest request);
  $async.Future<$3.ListMyReportsResponse> listMyReports(
      $pb.ServerContext ctx, $3.ListMyReportsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'CreateReport':
        return $3.CreateReportRequest();
      case 'ListMyReports':
        return $3.ListMyReportsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'CreateReport':
        return createReport(ctx, request as $3.CreateReportRequest);
      case 'ListMyReports':
        return listMyReports(ctx, request as $3.ListMyReportsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerReportAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerReportAppServiceBase$messageJson;
}

abstract class SellerReportPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$3.SubmitFoodSafetyReportResponse> submitFoodSafetyReport(
      $pb.ServerContext ctx, $3.SubmitFoodSafetyReportRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SubmitFoodSafetyReport':
        return $3.SubmitFoodSafetyReportRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SubmitFoodSafetyReport':
        return submitFoodSafetyReport(
            ctx, request as $3.SubmitFoodSafetyReportRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerReportPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerReportPublicServiceBase$messageJson;
}
