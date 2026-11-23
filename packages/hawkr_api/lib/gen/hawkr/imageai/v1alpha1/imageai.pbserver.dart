// This is a generated file - do not edit.
//
// Generated from hawkr/imageai/v1alpha1/imageai.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'imageai.pb.dart' as $1;
import 'imageai.pbjson.dart';

export 'imageai.pb.dart';

abstract class ImageAiAppServiceBase extends $pb.GeneratedService {
  $async.Future<$1.TransformImageResponse> transformImage(
      $pb.ServerContext ctx, $1.TransformImageRequest request);
  $async.Future<$1.ListAiStylesResponse> listAiStyles(
      $pb.ServerContext ctx, $1.ListAiStylesRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'TransformImage':
        return $1.TransformImageRequest();
      case 'ListAiStyles':
        return $1.ListAiStylesRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'TransformImage':
        return transformImage(ctx, request as $1.TransformImageRequest);
      case 'ListAiStyles':
        return listAiStyles(ctx, request as $1.ListAiStylesRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      ImageAiAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ImageAiAppServiceBase$messageJson;
}
