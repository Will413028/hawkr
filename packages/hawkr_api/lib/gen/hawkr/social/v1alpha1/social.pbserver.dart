// This is a generated file - do not edit.
//
// Generated from hawkr/social/v1alpha1/social.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'social.pb.dart' as $3;
import 'social.pbjson.dart';

export 'social.pb.dart';

abstract class SocialPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$3.GetPostResponse> getPost(
      $pb.ServerContext ctx, $3.GetPostRequest request);
  $async.Future<$3.ListPostsResponse> listPosts(
      $pb.ServerContext ctx, $3.ListPostsRequest request);
  $async.Future<$3.ListCommentsResponse> listComments(
      $pb.ServerContext ctx, $3.ListCommentsRequest request);
  $async.Future<$3.SuggestTrendingPostsResponse> suggestTrendingPosts(
      $pb.ServerContext ctx, $3.SuggestTrendingPostsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetPost':
        return $3.GetPostRequest();
      case 'ListPosts':
        return $3.ListPostsRequest();
      case 'ListComments':
        return $3.ListCommentsRequest();
      case 'SuggestTrendingPosts':
        return $3.SuggestTrendingPostsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetPost':
        return getPost(ctx, request as $3.GetPostRequest);
      case 'ListPosts':
        return listPosts(ctx, request as $3.ListPostsRequest);
      case 'ListComments':
        return listComments(ctx, request as $3.ListCommentsRequest);
      case 'SuggestTrendingPosts':
        return suggestTrendingPosts(
            ctx, request as $3.SuggestTrendingPostsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SocialPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialPublicServiceBase$messageJson;
}
