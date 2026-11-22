// This is a generated file - do not edit.
//
// Generated from hawkr/media/v1alpha1/media.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'media.pb.dart' as $1;
import 'media.pbjson.dart';

export 'media.pb.dart';

abstract class MediaAppServiceBase extends $pb.GeneratedService {
  $async.Future<$1.BeginUploadResponse> beginUpload(
      $pb.ServerContext ctx, $1.BeginUploadRequest request);
  $async.Future<$1.CompleteUploadResponse> completeUpload(
      $pb.ServerContext ctx, $1.CompleteUploadRequest request);
  $async.Future<$1.GetUploadResponse> getUpload(
      $pb.ServerContext ctx, $1.GetUploadRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'BeginUpload':
        return $1.BeginUploadRequest();
      case 'CompleteUpload':
        return $1.CompleteUploadRequest();
      case 'GetUpload':
        return $1.GetUploadRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'BeginUpload':
        return beginUpload(ctx, request as $1.BeginUploadRequest);
      case 'CompleteUpload':
        return completeUpload(ctx, request as $1.CompleteUploadRequest);
      case 'GetUpload':
        return getUpload(ctx, request as $1.GetUploadRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => MediaAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => MediaAppServiceBase$messageJson;
}
