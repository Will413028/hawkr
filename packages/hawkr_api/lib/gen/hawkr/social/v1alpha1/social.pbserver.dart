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
  $async.Future<$3.SuggestTagsResponse> suggestTags(
      $pb.ServerContext ctx, $3.SuggestTagsRequest request);
  $async.Future<$3.SuggestTrendingTagsResponse> suggestTrendingTags(
      $pb.ServerContext ctx, $3.SuggestTrendingTagsRequest request);
  $async.Future<$3.ListForumEntriesResponse> listForumEntries(
      $pb.ServerContext ctx, $3.ListForumEntriesRequest request);
  $async.Future<$3.RecordPostViewResponse> recordPostView(
      $pb.ServerContext ctx, $3.RecordPostViewRequest request);

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
      case 'SuggestTags':
        return $3.SuggestTagsRequest();
      case 'SuggestTrendingTags':
        return $3.SuggestTrendingTagsRequest();
      case 'ListForumEntries':
        return $3.ListForumEntriesRequest();
      case 'RecordPostView':
        return $3.RecordPostViewRequest();
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
      case 'SuggestTags':
        return suggestTags(ctx, request as $3.SuggestTagsRequest);
      case 'SuggestTrendingTags':
        return suggestTrendingTags(
            ctx, request as $3.SuggestTrendingTagsRequest);
      case 'ListForumEntries':
        return listForumEntries(ctx, request as $3.ListForumEntriesRequest);
      case 'RecordPostView':
        return recordPostView(ctx, request as $3.RecordPostViewRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SocialPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialPublicServiceBase$messageJson;
}

abstract class SocialAppServiceBase extends $pb.GeneratedService {
  $async.Future<$3.SharePostResponse> sharePost(
      $pb.ServerContext ctx, $3.SharePostRequest request);
  $async.Future<$3.LikePostResponse> likePost(
      $pb.ServerContext ctx, $3.LikePostRequest request);
  $async.Future<$3.UnlikePostResponse> unlikePost(
      $pb.ServerContext ctx, $3.UnlikePostRequest request);
  $async.Future<$3.LikeCommentResponse> likeComment(
      $pb.ServerContext ctx, $3.LikeCommentRequest request);
  $async.Future<$3.UnlikeCommentResponse> unlikeComment(
      $pb.ServerContext ctx, $3.UnlikeCommentRequest request);
  $async.Future<$3.CreateCommentResponse> createComment(
      $pb.ServerContext ctx, $3.CreateCommentRequest request);
  $async.Future<$3.UpdateCommentResponse> updateComment(
      $pb.ServerContext ctx, $3.UpdateCommentRequest request);
  $async.Future<$3.DeleteCommentResponse> deleteComment(
      $pb.ServerContext ctx, $3.DeleteCommentRequest request);
  $async.Future<$3.CreateForumEntryResponse> createForumEntry(
      $pb.ServerContext ctx, $3.CreateForumEntryRequest request);
  $async.Future<$3.UpdateForumEntryResponse> updateForumEntry(
      $pb.ServerContext ctx, $3.UpdateForumEntryRequest request);
  $async.Future<$3.DeleteForumEntryResponse> deleteForumEntry(
      $pb.ServerContext ctx, $3.DeleteForumEntryRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SharePost':
        return $3.SharePostRequest();
      case 'LikePost':
        return $3.LikePostRequest();
      case 'UnlikePost':
        return $3.UnlikePostRequest();
      case 'LikeComment':
        return $3.LikeCommentRequest();
      case 'UnlikeComment':
        return $3.UnlikeCommentRequest();
      case 'CreateComment':
        return $3.CreateCommentRequest();
      case 'UpdateComment':
        return $3.UpdateCommentRequest();
      case 'DeleteComment':
        return $3.DeleteCommentRequest();
      case 'CreateForumEntry':
        return $3.CreateForumEntryRequest();
      case 'UpdateForumEntry':
        return $3.UpdateForumEntryRequest();
      case 'DeleteForumEntry':
        return $3.DeleteForumEntryRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SharePost':
        return sharePost(ctx, request as $3.SharePostRequest);
      case 'LikePost':
        return likePost(ctx, request as $3.LikePostRequest);
      case 'UnlikePost':
        return unlikePost(ctx, request as $3.UnlikePostRequest);
      case 'LikeComment':
        return likeComment(ctx, request as $3.LikeCommentRequest);
      case 'UnlikeComment':
        return unlikeComment(ctx, request as $3.UnlikeCommentRequest);
      case 'CreateComment':
        return createComment(ctx, request as $3.CreateCommentRequest);
      case 'UpdateComment':
        return updateComment(ctx, request as $3.UpdateCommentRequest);
      case 'DeleteComment':
        return deleteComment(ctx, request as $3.DeleteCommentRequest);
      case 'CreateForumEntry':
        return createForumEntry(ctx, request as $3.CreateForumEntryRequest);
      case 'UpdateForumEntry':
        return updateForumEntry(ctx, request as $3.UpdateForumEntryRequest);
      case 'DeleteForumEntry':
        return deleteForumEntry(ctx, request as $3.DeleteForumEntryRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SocialAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialAppServiceBase$messageJson;
}
