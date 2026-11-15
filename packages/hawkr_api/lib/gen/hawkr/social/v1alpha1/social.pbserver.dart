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
  $async.Future<$4.CreatePostResponse> createPost(
      $pb.ServerContext ctx, $4.CreatePostRequest request);
  $async.Future<$4.GetMyPostResponse> getMyPost(
      $pb.ServerContext ctx, $4.GetMyPostRequest request);
  $async.Future<$4.ListMyPostsResponse> listMyPosts(
      $pb.ServerContext ctx, $4.ListMyPostsRequest request);
  $async.Future<$4.UpdatePostResponse> updatePost(
      $pb.ServerContext ctx, $4.UpdatePostRequest request);
  $async.Future<$4.PublishPostResponse> publishPost(
      $pb.ServerContext ctx, $4.PublishPostRequest request);
  $async.Future<$4.UnpublishPostResponse> unpublishPost(
      $pb.ServerContext ctx, $4.UnpublishPostRequest request);
  $async.Future<$4.DeletePostResponse> deletePost(
      $pb.ServerContext ctx, $4.DeletePostRequest request);
  $async.Future<$4.SchedulePostResponse> schedulePost(
      $pb.ServerContext ctx, $4.SchedulePostRequest request);
  $async.Future<$4.ListPendingSchedulesResponse> listPendingSchedules(
      $pb.ServerContext ctx, $4.ListPendingSchedulesRequest request);
  $async.Future<$4.CancelPostScheduleResponse> cancelPostSchedule(
      $pb.ServerContext ctx, $4.CancelPostScheduleRequest request);
  $async.Future<$4.ListStoreCommentsResponse> listStoreComments(
      $pb.ServerContext ctx, $4.ListStoreCommentsRequest request);
  $async.Future<$4.GetStoreSocialSummaryResponse> getStoreSocialSummary(
      $pb.ServerContext ctx, $4.GetStoreSocialSummaryRequest request);

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
      case 'CreatePost':
        return $4.CreatePostRequest();
      case 'GetMyPost':
        return $4.GetMyPostRequest();
      case 'ListMyPosts':
        return $4.ListMyPostsRequest();
      case 'UpdatePost':
        return $4.UpdatePostRequest();
      case 'PublishPost':
        return $4.PublishPostRequest();
      case 'UnpublishPost':
        return $4.UnpublishPostRequest();
      case 'DeletePost':
        return $4.DeletePostRequest();
      case 'SchedulePost':
        return $4.SchedulePostRequest();
      case 'ListPendingSchedules':
        return $4.ListPendingSchedulesRequest();
      case 'CancelPostSchedule':
        return $4.CancelPostScheduleRequest();
      case 'ListStoreComments':
        return $4.ListStoreCommentsRequest();
      case 'GetStoreSocialSummary':
        return $4.GetStoreSocialSummaryRequest();
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
      case 'CreatePost':
        return createPost(ctx, request as $4.CreatePostRequest);
      case 'GetMyPost':
        return getMyPost(ctx, request as $4.GetMyPostRequest);
      case 'ListMyPosts':
        return listMyPosts(ctx, request as $4.ListMyPostsRequest);
      case 'UpdatePost':
        return updatePost(ctx, request as $4.UpdatePostRequest);
      case 'PublishPost':
        return publishPost(ctx, request as $4.PublishPostRequest);
      case 'UnpublishPost':
        return unpublishPost(ctx, request as $4.UnpublishPostRequest);
      case 'DeletePost':
        return deletePost(ctx, request as $4.DeletePostRequest);
      case 'SchedulePost':
        return schedulePost(ctx, request as $4.SchedulePostRequest);
      case 'ListPendingSchedules':
        return listPendingSchedules(
            ctx, request as $4.ListPendingSchedulesRequest);
      case 'CancelPostSchedule':
        return cancelPostSchedule(ctx, request as $4.CancelPostScheduleRequest);
      case 'ListStoreComments':
        return listStoreComments(ctx, request as $4.ListStoreCommentsRequest);
      case 'GetStoreSocialSummary':
        return getStoreSocialSummary(
            ctx, request as $4.GetStoreSocialSummaryRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SocialAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialAppServiceBase$messageJson;
}

abstract class SocialDelegatedServiceBase extends $pb.GeneratedService {
  $async.Future<$4.ListStorePostsResponse> listStorePosts(
      $pb.ServerContext ctx, $4.ListStorePostsRequest request);
  $async.Future<$4.GetStorePostResponse> getStorePost(
      $pb.ServerContext ctx, $4.GetStorePostRequest request);
  $async.Future<$4.CreateStorePostResponse> createStorePost(
      $pb.ServerContext ctx, $4.CreateStorePostRequest request);
  $async.Future<$4.UpdateStorePostResponse> updateStorePost(
      $pb.ServerContext ctx, $4.UpdateStorePostRequest request);
  $async.Future<$4.PublishStorePostResponse> publishStorePost(
      $pb.ServerContext ctx, $4.PublishStorePostRequest request);
  $async.Future<$4.UnpublishStorePostResponse> unpublishStorePost(
      $pb.ServerContext ctx, $4.UnpublishStorePostRequest request);
  $async.Future<$4.ScheduleStorePostResponse> scheduleStorePost(
      $pb.ServerContext ctx, $4.ScheduleStorePostRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListStorePosts':
        return $4.ListStorePostsRequest();
      case 'GetStorePost':
        return $4.GetStorePostRequest();
      case 'CreateStorePost':
        return $4.CreateStorePostRequest();
      case 'UpdateStorePost':
        return $4.UpdateStorePostRequest();
      case 'PublishStorePost':
        return $4.PublishStorePostRequest();
      case 'UnpublishStorePost':
        return $4.UnpublishStorePostRequest();
      case 'ScheduleStorePost':
        return $4.ScheduleStorePostRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListStorePosts':
        return listStorePosts(ctx, request as $4.ListStorePostsRequest);
      case 'GetStorePost':
        return getStorePost(ctx, request as $4.GetStorePostRequest);
      case 'CreateStorePost':
        return createStorePost(ctx, request as $4.CreateStorePostRequest);
      case 'UpdateStorePost':
        return updateStorePost(ctx, request as $4.UpdateStorePostRequest);
      case 'PublishStorePost':
        return publishStorePost(ctx, request as $4.PublishStorePostRequest);
      case 'UnpublishStorePost':
        return unpublishStorePost(ctx, request as $4.UnpublishStorePostRequest);
      case 'ScheduleStorePost':
        return scheduleStorePost(ctx, request as $4.ScheduleStorePostRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SocialDelegatedServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialDelegatedServiceBase$messageJson;
}
