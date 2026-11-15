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

import 'social.pb.dart' as $4;
import 'social.pbjson.dart';

export 'social.pb.dart';

abstract class SocialPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$4.GetPostResponse> getPost(
      $pb.ServerContext ctx, $4.GetPostRequest request);
  $async.Future<$4.ListPostsResponse> listPosts(
      $pb.ServerContext ctx, $4.ListPostsRequest request);
  $async.Future<$4.ListCommentsResponse> listComments(
      $pb.ServerContext ctx, $4.ListCommentsRequest request);
  $async.Future<$4.SuggestTrendingPostsResponse> suggestTrendingPosts(
      $pb.ServerContext ctx, $4.SuggestTrendingPostsRequest request);
  $async.Future<$4.SuggestTagsResponse> suggestTags(
      $pb.ServerContext ctx, $4.SuggestTagsRequest request);
  $async.Future<$4.SuggestTrendingTagsResponse> suggestTrendingTags(
      $pb.ServerContext ctx, $4.SuggestTrendingTagsRequest request);
  $async.Future<$4.ListForumEntriesResponse> listForumEntries(
      $pb.ServerContext ctx, $4.ListForumEntriesRequest request);
  $async.Future<$4.RecordPostViewResponse> recordPostView(
      $pb.ServerContext ctx, $4.RecordPostViewRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetPost':
        return $4.GetPostRequest();
      case 'ListPosts':
        return $4.ListPostsRequest();
      case 'ListComments':
        return $4.ListCommentsRequest();
      case 'SuggestTrendingPosts':
        return $4.SuggestTrendingPostsRequest();
      case 'SuggestTags':
        return $4.SuggestTagsRequest();
      case 'SuggestTrendingTags':
        return $4.SuggestTrendingTagsRequest();
      case 'ListForumEntries':
        return $4.ListForumEntriesRequest();
      case 'RecordPostView':
        return $4.RecordPostViewRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetPost':
        return getPost(ctx, request as $4.GetPostRequest);
      case 'ListPosts':
        return listPosts(ctx, request as $4.ListPostsRequest);
      case 'ListComments':
        return listComments(ctx, request as $4.ListCommentsRequest);
      case 'SuggestTrendingPosts':
        return suggestTrendingPosts(
            ctx, request as $4.SuggestTrendingPostsRequest);
      case 'SuggestTags':
        return suggestTags(ctx, request as $4.SuggestTagsRequest);
      case 'SuggestTrendingTags':
        return suggestTrendingTags(
            ctx, request as $4.SuggestTrendingTagsRequest);
      case 'ListForumEntries':
        return listForumEntries(ctx, request as $4.ListForumEntriesRequest);
      case 'RecordPostView':
        return recordPostView(ctx, request as $4.RecordPostViewRequest);
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
  $async.Future<$4.SharePostResponse> sharePost(
      $pb.ServerContext ctx, $4.SharePostRequest request);
  $async.Future<$4.LikePostResponse> likePost(
      $pb.ServerContext ctx, $4.LikePostRequest request);
  $async.Future<$4.UnlikePostResponse> unlikePost(
      $pb.ServerContext ctx, $4.UnlikePostRequest request);
  $async.Future<$4.LikeCommentResponse> likeComment(
      $pb.ServerContext ctx, $4.LikeCommentRequest request);
  $async.Future<$4.UnlikeCommentResponse> unlikeComment(
      $pb.ServerContext ctx, $4.UnlikeCommentRequest request);
  $async.Future<$4.CreateCommentResponse> createComment(
      $pb.ServerContext ctx, $4.CreateCommentRequest request);
  $async.Future<$4.UpdateCommentResponse> updateComment(
      $pb.ServerContext ctx, $4.UpdateCommentRequest request);
  $async.Future<$4.DeleteCommentResponse> deleteComment(
      $pb.ServerContext ctx, $4.DeleteCommentRequest request);
  $async.Future<$4.CreateForumEntryResponse> createForumEntry(
      $pb.ServerContext ctx, $4.CreateForumEntryRequest request);
  $async.Future<$4.UpdateForumEntryResponse> updateForumEntry(
      $pb.ServerContext ctx, $4.UpdateForumEntryRequest request);
  $async.Future<$4.DeleteForumEntryResponse> deleteForumEntry(
      $pb.ServerContext ctx, $4.DeleteForumEntryRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SharePost':
        return $4.SharePostRequest();
      case 'LikePost':
        return $4.LikePostRequest();
      case 'UnlikePost':
        return $4.UnlikePostRequest();
      case 'LikeComment':
        return $4.LikeCommentRequest();
      case 'UnlikeComment':
        return $4.UnlikeCommentRequest();
      case 'CreateComment':
        return $4.CreateCommentRequest();
      case 'UpdateComment':
        return $4.UpdateCommentRequest();
      case 'DeleteComment':
        return $4.DeleteCommentRequest();
      case 'CreateForumEntry':
        return $4.CreateForumEntryRequest();
      case 'UpdateForumEntry':
        return $4.UpdateForumEntryRequest();
      case 'DeleteForumEntry':
        return $4.DeleteForumEntryRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SharePost':
        return sharePost(ctx, request as $4.SharePostRequest);
      case 'LikePost':
        return likePost(ctx, request as $4.LikePostRequest);
      case 'UnlikePost':
        return unlikePost(ctx, request as $4.UnlikePostRequest);
      case 'LikeComment':
        return likeComment(ctx, request as $4.LikeCommentRequest);
      case 'UnlikeComment':
        return unlikeComment(ctx, request as $4.UnlikeCommentRequest);
      case 'CreateComment':
        return createComment(ctx, request as $4.CreateCommentRequest);
      case 'UpdateComment':
        return updateComment(ctx, request as $4.UpdateCommentRequest);
      case 'DeleteComment':
        return deleteComment(ctx, request as $4.DeleteCommentRequest);
      case 'CreateForumEntry':
        return createForumEntry(ctx, request as $4.CreateForumEntryRequest);
      case 'UpdateForumEntry':
        return updateForumEntry(ctx, request as $4.UpdateForumEntryRequest);
      case 'DeleteForumEntry':
        return deleteForumEntry(ctx, request as $4.DeleteForumEntryRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SocialAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialAppServiceBase$messageJson;
}
