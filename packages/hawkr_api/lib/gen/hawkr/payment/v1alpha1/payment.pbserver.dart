// This is a generated file - do not edit.
//
// Generated from hawkr/payment/v1alpha1/payment.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'payment.pb.dart' as $1;
import 'payment.pbjson.dart';

export 'payment.pb.dart';

abstract class PaymentAppServiceBase extends $pb.GeneratedService {
  $async.Future<$1.CreatePaymentResponse> createPayment(
      $pb.ServerContext ctx, $1.CreatePaymentRequest request);
  $async.Future<$1.GetPaymentResponse> getPayment(
      $pb.ServerContext ctx, $1.GetPaymentRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'CreatePayment':
        return $1.CreatePaymentRequest();
      case 'GetPayment':
        return $1.GetPaymentRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'CreatePayment':
        return createPayment(ctx, request as $1.CreatePaymentRequest);
      case 'GetPayment':
        return getPayment(ctx, request as $1.GetPaymentRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      PaymentAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => PaymentAppServiceBase$messageJson;
}
