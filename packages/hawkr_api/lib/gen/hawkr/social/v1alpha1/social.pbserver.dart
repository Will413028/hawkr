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

import 'social.pb.dart' as $5;
import 'social.pbjson.dart';

export 'social.pb.dart';

abstract class SocialPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$5.GetPostResponse> getPost(
      $pb.ServerContext ctx, $5.GetPostRequest request);
  $async.Future<$5.ListPostsResponse> listPosts(
      $pb.ServerContext ctx, $5.ListPostsRequest request);
  $async.Future<$5.ListCommentsResponse> listComments(
      $pb.ServerContext ctx, $5.ListCommentsRequest request);
  $async.Future<$5.SuggestTrendingPostsResponse> suggestTrendingPosts(
      $pb.ServerContext ctx, $5.SuggestTrendingPostsRequest request);
  $async.Future<$5.SuggestTagsResponse> suggestTags(
      $pb.ServerContext ctx, $5.SuggestTagsRequest request);
  $async.Future<$5.SuggestTrendingTagsResponse> suggestTrendingTags(
      $pb.ServerContext ctx, $5.SuggestTrendingTagsRequest request);
  $async.Future<$5.ListForumEntriesResponse> listForumEntries(
      $pb.ServerContext ctx, $5.ListForumEntriesRequest request);
  $async.Future<$5.RecordPostViewResponse> recordPostView(
      $pb.ServerContext ctx, $5.RecordPostViewRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetPost':
        return $5.GetPostRequest();
      case 'ListPosts':
        return $5.ListPostsRequest();
      case 'ListComments':
        return $5.ListCommentsRequest();
      case 'SuggestTrendingPosts':
        return $5.SuggestTrendingPostsRequest();
      case 'SuggestTags':
        return $5.SuggestTagsRequest();
      case 'SuggestTrendingTags':
        return $5.SuggestTrendingTagsRequest();
      case 'ListForumEntries':
        return $5.ListForumEntriesRequest();
      case 'RecordPostView':
        return $5.RecordPostViewRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetPost':
        return getPost(ctx, request as $5.GetPostRequest);
      case 'ListPosts':
        return listPosts(ctx, request as $5.ListPostsRequest);
      case 'ListComments':
        return listComments(ctx, request as $5.ListCommentsRequest);
      case 'SuggestTrendingPosts':
        return suggestTrendingPosts(
            ctx, request as $5.SuggestTrendingPostsRequest);
      case 'SuggestTags':
        return suggestTags(ctx, request as $5.SuggestTagsRequest);
      case 'SuggestTrendingTags':
        return suggestTrendingTags(
            ctx, request as $5.SuggestTrendingTagsRequest);
      case 'ListForumEntries':
        return listForumEntries(ctx, request as $5.ListForumEntriesRequest);
      case 'RecordPostView':
        return recordPostView(ctx, request as $5.RecordPostViewRequest);
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
  $async.Future<$5.SharePostResponse> sharePost(
      $pb.ServerContext ctx, $5.SharePostRequest request);
  $async.Future<$5.LikePostResponse> likePost(
      $pb.ServerContext ctx, $5.LikePostRequest request);
  $async.Future<$5.UnlikePostResponse> unlikePost(
      $pb.ServerContext ctx, $5.UnlikePostRequest request);
  $async.Future<$5.LikeCommentResponse> likeComment(
      $pb.ServerContext ctx, $5.LikeCommentRequest request);
  $async.Future<$5.UnlikeCommentResponse> unlikeComment(
      $pb.ServerContext ctx, $5.UnlikeCommentRequest request);
  $async.Future<$5.CreateCommentResponse> createComment(
      $pb.ServerContext ctx, $5.CreateCommentRequest request);
  $async.Future<$5.UpdateCommentResponse> updateComment(
      $pb.ServerContext ctx, $5.UpdateCommentRequest request);
  $async.Future<$5.DeleteCommentResponse> deleteComment(
      $pb.ServerContext ctx, $5.DeleteCommentRequest request);
  $async.Future<$5.CreateForumEntryResponse> createForumEntry(
      $pb.ServerContext ctx, $5.CreateForumEntryRequest request);
  $async.Future<$5.UpdateForumEntryResponse> updateForumEntry(
      $pb.ServerContext ctx, $5.UpdateForumEntryRequest request);
  $async.Future<$5.DeleteForumEntryResponse> deleteForumEntry(
      $pb.ServerContext ctx, $5.DeleteForumEntryRequest request);
  $async.Future<$5.CreatePostResponse> createPost(
      $pb.ServerContext ctx, $5.CreatePostRequest request);
  $async.Future<$5.GetMyPostResponse> getMyPost(
      $pb.ServerContext ctx, $5.GetMyPostRequest request);
  $async.Future<$5.ListMyPostsResponse> listMyPosts(
      $pb.ServerContext ctx, $5.ListMyPostsRequest request);
  $async.Future<$5.UpdatePostResponse> updatePost(
      $pb.ServerContext ctx, $5.UpdatePostRequest request);
  $async.Future<$5.PublishPostResponse> publishPost(
      $pb.ServerContext ctx, $5.PublishPostRequest request);
  $async.Future<$5.UnpublishPostResponse> unpublishPost(
      $pb.ServerContext ctx, $5.UnpublishPostRequest request);
  $async.Future<$5.DeletePostResponse> deletePost(
      $pb.ServerContext ctx, $5.DeletePostRequest request);
  $async.Future<$5.SchedulePostResponse> schedulePost(
      $pb.ServerContext ctx, $5.SchedulePostRequest request);
  $async.Future<$5.ListPendingSchedulesResponse> listPendingSchedules(
      $pb.ServerContext ctx, $5.ListPendingSchedulesRequest request);
  $async.Future<$5.CancelPostScheduleResponse> cancelPostSchedule(
      $pb.ServerContext ctx, $5.CancelPostScheduleRequest request);
  $async.Future<$5.ListStoreCommentsResponse> listStoreComments(
      $pb.ServerContext ctx, $5.ListStoreCommentsRequest request);
  $async.Future<$5.GetStoreSocialSummaryResponse> getStoreSocialSummary(
      $pb.ServerContext ctx, $5.GetStoreSocialSummaryRequest request);
  $async.Future<$5.ListMySanctionsResponse> listMySanctions(
      $pb.ServerContext ctx, $5.ListMySanctionsRequest request);
  $async.Future<$5.ListMyHiddenContentResponse> listMyHiddenContent(
      $pb.ServerContext ctx, $5.ListMyHiddenContentRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SharePost':
        return $5.SharePostRequest();
      case 'LikePost':
        return $5.LikePostRequest();
      case 'UnlikePost':
        return $5.UnlikePostRequest();
      case 'LikeComment':
        return $5.LikeCommentRequest();
      case 'UnlikeComment':
        return $5.UnlikeCommentRequest();
      case 'CreateComment':
        return $5.CreateCommentRequest();
      case 'UpdateComment':
        return $5.UpdateCommentRequest();
      case 'DeleteComment':
        return $5.DeleteCommentRequest();
      case 'CreateForumEntry':
        return $5.CreateForumEntryRequest();
      case 'UpdateForumEntry':
        return $5.UpdateForumEntryRequest();
      case 'DeleteForumEntry':
        return $5.DeleteForumEntryRequest();
      case 'CreatePost':
        return $5.CreatePostRequest();
      case 'GetMyPost':
        return $5.GetMyPostRequest();
      case 'ListMyPosts':
        return $5.ListMyPostsRequest();
      case 'UpdatePost':
        return $5.UpdatePostRequest();
      case 'PublishPost':
        return $5.PublishPostRequest();
      case 'UnpublishPost':
        return $5.UnpublishPostRequest();
      case 'DeletePost':
        return $5.DeletePostRequest();
      case 'SchedulePost':
        return $5.SchedulePostRequest();
      case 'ListPendingSchedules':
        return $5.ListPendingSchedulesRequest();
      case 'CancelPostSchedule':
        return $5.CancelPostScheduleRequest();
      case 'ListStoreComments':
        return $5.ListStoreCommentsRequest();
      case 'GetStoreSocialSummary':
        return $5.GetStoreSocialSummaryRequest();
      case 'ListMySanctions':
        return $5.ListMySanctionsRequest();
      case 'ListMyHiddenContent':
        return $5.ListMyHiddenContentRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SharePost':
        return sharePost(ctx, request as $5.SharePostRequest);
      case 'LikePost':
        return likePost(ctx, request as $5.LikePostRequest);
      case 'UnlikePost':
        return unlikePost(ctx, request as $5.UnlikePostRequest);
      case 'LikeComment':
        return likeComment(ctx, request as $5.LikeCommentRequest);
      case 'UnlikeComment':
        return unlikeComment(ctx, request as $5.UnlikeCommentRequest);
      case 'CreateComment':
        return createComment(ctx, request as $5.CreateCommentRequest);
      case 'UpdateComment':
        return updateComment(ctx, request as $5.UpdateCommentRequest);
      case 'DeleteComment':
        return deleteComment(ctx, request as $5.DeleteCommentRequest);
      case 'CreateForumEntry':
        return createForumEntry(ctx, request as $5.CreateForumEntryRequest);
      case 'UpdateForumEntry':
        return updateForumEntry(ctx, request as $5.UpdateForumEntryRequest);
      case 'DeleteForumEntry':
        return deleteForumEntry(ctx, request as $5.DeleteForumEntryRequest);
      case 'CreatePost':
        return createPost(ctx, request as $5.CreatePostRequest);
      case 'GetMyPost':
        return getMyPost(ctx, request as $5.GetMyPostRequest);
      case 'ListMyPosts':
        return listMyPosts(ctx, request as $5.ListMyPostsRequest);
      case 'UpdatePost':
        return updatePost(ctx, request as $5.UpdatePostRequest);
      case 'PublishPost':
        return publishPost(ctx, request as $5.PublishPostRequest);
      case 'UnpublishPost':
        return unpublishPost(ctx, request as $5.UnpublishPostRequest);
      case 'DeletePost':
        return deletePost(ctx, request as $5.DeletePostRequest);
      case 'SchedulePost':
        return schedulePost(ctx, request as $5.SchedulePostRequest);
      case 'ListPendingSchedules':
        return listPendingSchedules(
            ctx, request as $5.ListPendingSchedulesRequest);
      case 'CancelPostSchedule':
        return cancelPostSchedule(ctx, request as $5.CancelPostScheduleRequest);
      case 'ListStoreComments':
        return listStoreComments(ctx, request as $5.ListStoreCommentsRequest);
      case 'GetStoreSocialSummary':
        return getStoreSocialSummary(
            ctx, request as $5.GetStoreSocialSummaryRequest);
      case 'ListMySanctions':
        return listMySanctions(ctx, request as $5.ListMySanctionsRequest);
      case 'ListMyHiddenContent':
        return listMyHiddenContent(
            ctx, request as $5.ListMyHiddenContentRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SocialAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialAppServiceBase$messageJson;
}

abstract class SocialDelegatedServiceBase extends $pb.GeneratedService {
  $async.Future<$5.ListStorePostsResponse> listStorePosts(
      $pb.ServerContext ctx, $5.ListStorePostsRequest request);
  $async.Future<$5.GetStorePostResponse> getStorePost(
      $pb.ServerContext ctx, $5.GetStorePostRequest request);
  $async.Future<$5.CreateStorePostResponse> createStorePost(
      $pb.ServerContext ctx, $5.CreateStorePostRequest request);
  $async.Future<$5.UpdateStorePostResponse> updateStorePost(
      $pb.ServerContext ctx, $5.UpdateStorePostRequest request);
  $async.Future<$5.PublishStorePostResponse> publishStorePost(
      $pb.ServerContext ctx, $5.PublishStorePostRequest request);
  $async.Future<$5.UnpublishStorePostResponse> unpublishStorePost(
      $pb.ServerContext ctx, $5.UnpublishStorePostRequest request);
  $async.Future<$5.ScheduleStorePostResponse> scheduleStorePost(
      $pb.ServerContext ctx, $5.ScheduleStorePostRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListStorePosts':
        return $5.ListStorePostsRequest();
      case 'GetStorePost':
        return $5.GetStorePostRequest();
      case 'CreateStorePost':
        return $5.CreateStorePostRequest();
      case 'UpdateStorePost':
        return $5.UpdateStorePostRequest();
      case 'PublishStorePost':
        return $5.PublishStorePostRequest();
      case 'UnpublishStorePost':
        return $5.UnpublishStorePostRequest();
      case 'ScheduleStorePost':
        return $5.ScheduleStorePostRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListStorePosts':
        return listStorePosts(ctx, request as $5.ListStorePostsRequest);
      case 'GetStorePost':
        return getStorePost(ctx, request as $5.GetStorePostRequest);
      case 'CreateStorePost':
        return createStorePost(ctx, request as $5.CreateStorePostRequest);
      case 'UpdateStorePost':
        return updateStorePost(ctx, request as $5.UpdateStorePostRequest);
      case 'PublishStorePost':
        return publishStorePost(ctx, request as $5.PublishStorePostRequest);
      case 'UnpublishStorePost':
        return unpublishStorePost(ctx, request as $5.UnpublishStorePostRequest);
      case 'ScheduleStorePost':
        return scheduleStorePost(ctx, request as $5.ScheduleStorePostRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SocialDelegatedServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialDelegatedServiceBase$messageJson;
}
