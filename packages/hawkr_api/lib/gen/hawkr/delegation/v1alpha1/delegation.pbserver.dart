// This is a generated file - do not edit.
//
// Generated from hawkr/delegation/v1alpha1/delegation.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'delegation.pb.dart' as $5;
import 'delegation.pbjson.dart';

export 'delegation.pb.dart';

abstract class DelegationDelegatedServiceBase extends $pb.GeneratedService {
  $async.Future<$5.ListPostsResponse> listPosts(
      $pb.ServerContext ctx, $5.ListPostsRequest request);
  $async.Future<$5.GetPostResponse> getPost(
      $pb.ServerContext ctx, $5.GetPostRequest request);
  $async.Future<$5.CreatePostResponse> createPost(
      $pb.ServerContext ctx, $5.CreatePostRequest request);
  $async.Future<$5.UpdatePostResponse> updatePost(
      $pb.ServerContext ctx, $5.UpdatePostRequest request);
  $async.Future<$5.PublishPostResponse> publishPost(
      $pb.ServerContext ctx, $5.PublishPostRequest request);
  $async.Future<$5.UnpublishPostResponse> unpublishPost(
      $pb.ServerContext ctx, $5.UnpublishPostRequest request);
  $async.Future<$5.SchedulePostResponse> schedulePost(
      $pb.ServerContext ctx, $5.SchedulePostRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListPosts':
        return $5.ListPostsRequest();
      case 'GetPost':
        return $5.GetPostRequest();
      case 'CreatePost':
        return $5.CreatePostRequest();
      case 'UpdatePost':
        return $5.UpdatePostRequest();
      case 'PublishPost':
        return $5.PublishPostRequest();
      case 'UnpublishPost':
        return $5.UnpublishPostRequest();
      case 'SchedulePost':
        return $5.SchedulePostRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListPosts':
        return listPosts(ctx, request as $5.ListPostsRequest);
      case 'GetPost':
        return getPost(ctx, request as $5.GetPostRequest);
      case 'CreatePost':
        return createPost(ctx, request as $5.CreatePostRequest);
      case 'UpdatePost':
        return updatePost(ctx, request as $5.UpdatePostRequest);
      case 'PublishPost':
        return publishPost(ctx, request as $5.PublishPostRequest);
      case 'UnpublishPost':
        return unpublishPost(ctx, request as $5.UnpublishPostRequest);
      case 'SchedulePost':
        return schedulePost(ctx, request as $5.SchedulePostRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      DelegationDelegatedServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => DelegationDelegatedServiceBase$messageJson;
}
