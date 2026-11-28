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

import 'social.pb.dart' as $6;
import 'social.pbjson.dart';

export 'social.pb.dart';

abstract class SocialPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$6.GetPostResponse> getPost(
      $pb.ServerContext ctx, $6.GetPostRequest request);
  $async.Future<$6.ListPostsResponse> listPosts(
      $pb.ServerContext ctx, $6.ListPostsRequest request);
  $async.Future<$6.ListCommentsResponse> listComments(
      $pb.ServerContext ctx, $6.ListCommentsRequest request);
  $async.Future<$6.SuggestTrendingPostsResponse> suggestTrendingPosts(
      $pb.ServerContext ctx, $6.SuggestTrendingPostsRequest request);
  $async.Future<$6.SuggestTagsResponse> suggestTags(
      $pb.ServerContext ctx, $6.SuggestTagsRequest request);
  $async.Future<$6.SuggestTrendingTagsResponse> suggestTrendingTags(
      $pb.ServerContext ctx, $6.SuggestTrendingTagsRequest request);
  $async.Future<$6.ListForumEntriesResponse> listForumEntries(
      $pb.ServerContext ctx, $6.ListForumEntriesRequest request);
  $async.Future<$6.GetForumThreadResponse> getForumThread(
      $pb.ServerContext ctx, $6.GetForumThreadRequest request);
  $async.Future<$6.RecordPostViewResponse> recordPostView(
      $pb.ServerContext ctx, $6.RecordPostViewRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetPost':
        return $6.GetPostRequest();
      case 'ListPosts':
        return $6.ListPostsRequest();
      case 'ListComments':
        return $6.ListCommentsRequest();
      case 'SuggestTrendingPosts':
        return $6.SuggestTrendingPostsRequest();
      case 'SuggestTags':
        return $6.SuggestTagsRequest();
      case 'SuggestTrendingTags':
        return $6.SuggestTrendingTagsRequest();
      case 'ListForumEntries':
        return $6.ListForumEntriesRequest();
      case 'GetForumThread':
        return $6.GetForumThreadRequest();
      case 'RecordPostView':
        return $6.RecordPostViewRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetPost':
        return getPost(ctx, request as $6.GetPostRequest);
      case 'ListPosts':
        return listPosts(ctx, request as $6.ListPostsRequest);
      case 'ListComments':
        return listComments(ctx, request as $6.ListCommentsRequest);
      case 'SuggestTrendingPosts':
        return suggestTrendingPosts(
            ctx, request as $6.SuggestTrendingPostsRequest);
      case 'SuggestTags':
        return suggestTags(ctx, request as $6.SuggestTagsRequest);
      case 'SuggestTrendingTags':
        return suggestTrendingTags(
            ctx, request as $6.SuggestTrendingTagsRequest);
      case 'ListForumEntries':
        return listForumEntries(ctx, request as $6.ListForumEntriesRequest);
      case 'GetForumThread':
        return getForumThread(ctx, request as $6.GetForumThreadRequest);
      case 'RecordPostView':
        return recordPostView(ctx, request as $6.RecordPostViewRequest);
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
  $async.Future<$6.SharePostResponse> sharePost(
      $pb.ServerContext ctx, $6.SharePostRequest request);
  $async.Future<$6.LikePostResponse> likePost(
      $pb.ServerContext ctx, $6.LikePostRequest request);
  $async.Future<$6.UnlikePostResponse> unlikePost(
      $pb.ServerContext ctx, $6.UnlikePostRequest request);
  $async.Future<$6.LikeCommentResponse> likeComment(
      $pb.ServerContext ctx, $6.LikeCommentRequest request);
  $async.Future<$6.UnlikeCommentResponse> unlikeComment(
      $pb.ServerContext ctx, $6.UnlikeCommentRequest request);
  $async.Future<$6.CreateCommentResponse> createComment(
      $pb.ServerContext ctx, $6.CreateCommentRequest request);
  $async.Future<$6.UpdateCommentResponse> updateComment(
      $pb.ServerContext ctx, $6.UpdateCommentRequest request);
  $async.Future<$6.DeleteCommentResponse> deleteComment(
      $pb.ServerContext ctx, $6.DeleteCommentRequest request);
  $async.Future<$6.CreateForumEntryResponse> createForumEntry(
      $pb.ServerContext ctx, $6.CreateForumEntryRequest request);
  $async.Future<$6.UpdateForumEntryResponse> updateForumEntry(
      $pb.ServerContext ctx, $6.UpdateForumEntryRequest request);
  $async.Future<$6.DeleteForumEntryResponse> deleteForumEntry(
      $pb.ServerContext ctx, $6.DeleteForumEntryRequest request);
  $async.Future<$6.CreatePostResponse> createPost(
      $pb.ServerContext ctx, $6.CreatePostRequest request);
  $async.Future<$6.GetMyPostResponse> getMyPost(
      $pb.ServerContext ctx, $6.GetMyPostRequest request);
  $async.Future<$6.ListMyPostsResponse> listMyPosts(
      $pb.ServerContext ctx, $6.ListMyPostsRequest request);
  $async.Future<$6.UpdatePostResponse> updatePost(
      $pb.ServerContext ctx, $6.UpdatePostRequest request);
  $async.Future<$6.PublishPostResponse> publishPost(
      $pb.ServerContext ctx, $6.PublishPostRequest request);
  $async.Future<$6.UnpublishPostResponse> unpublishPost(
      $pb.ServerContext ctx, $6.UnpublishPostRequest request);
  $async.Future<$6.DeletePostResponse> deletePost(
      $pb.ServerContext ctx, $6.DeletePostRequest request);
  $async.Future<$6.SchedulePostResponse> schedulePost(
      $pb.ServerContext ctx, $6.SchedulePostRequest request);
  $async.Future<$6.ListPendingSchedulesResponse> listPendingSchedules(
      $pb.ServerContext ctx, $6.ListPendingSchedulesRequest request);
  $async.Future<$6.CancelPostScheduleResponse> cancelPostSchedule(
      $pb.ServerContext ctx, $6.CancelPostScheduleRequest request);
  $async.Future<$6.ListStoreCommentsResponse> listStoreComments(
      $pb.ServerContext ctx, $6.ListStoreCommentsRequest request);
  $async.Future<$6.GetStoreSocialSummaryResponse> getStoreSocialSummary(
      $pb.ServerContext ctx, $6.GetStoreSocialSummaryRequest request);
  $async.Future<$6.ListMySanctionsResponse> listMySanctions(
      $pb.ServerContext ctx, $6.ListMySanctionsRequest request);
  $async.Future<$6.ListMyHiddenContentResponse> listMyHiddenContent(
      $pb.ServerContext ctx, $6.ListMyHiddenContentRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SharePost':
        return $6.SharePostRequest();
      case 'LikePost':
        return $6.LikePostRequest();
      case 'UnlikePost':
        return $6.UnlikePostRequest();
      case 'LikeComment':
        return $6.LikeCommentRequest();
      case 'UnlikeComment':
        return $6.UnlikeCommentRequest();
      case 'CreateComment':
        return $6.CreateCommentRequest();
      case 'UpdateComment':
        return $6.UpdateCommentRequest();
      case 'DeleteComment':
        return $6.DeleteCommentRequest();
      case 'CreateForumEntry':
        return $6.CreateForumEntryRequest();
      case 'UpdateForumEntry':
        return $6.UpdateForumEntryRequest();
      case 'DeleteForumEntry':
        return $6.DeleteForumEntryRequest();
      case 'CreatePost':
        return $6.CreatePostRequest();
      case 'GetMyPost':
        return $6.GetMyPostRequest();
      case 'ListMyPosts':
        return $6.ListMyPostsRequest();
      case 'UpdatePost':
        return $6.UpdatePostRequest();
      case 'PublishPost':
        return $6.PublishPostRequest();
      case 'UnpublishPost':
        return $6.UnpublishPostRequest();
      case 'DeletePost':
        return $6.DeletePostRequest();
      case 'SchedulePost':
        return $6.SchedulePostRequest();
      case 'ListPendingSchedules':
        return $6.ListPendingSchedulesRequest();
      case 'CancelPostSchedule':
        return $6.CancelPostScheduleRequest();
      case 'ListStoreComments':
        return $6.ListStoreCommentsRequest();
      case 'GetStoreSocialSummary':
        return $6.GetStoreSocialSummaryRequest();
      case 'ListMySanctions':
        return $6.ListMySanctionsRequest();
      case 'ListMyHiddenContent':
        return $6.ListMyHiddenContentRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SharePost':
        return sharePost(ctx, request as $6.SharePostRequest);
      case 'LikePost':
        return likePost(ctx, request as $6.LikePostRequest);
      case 'UnlikePost':
        return unlikePost(ctx, request as $6.UnlikePostRequest);
      case 'LikeComment':
        return likeComment(ctx, request as $6.LikeCommentRequest);
      case 'UnlikeComment':
        return unlikeComment(ctx, request as $6.UnlikeCommentRequest);
      case 'CreateComment':
        return createComment(ctx, request as $6.CreateCommentRequest);
      case 'UpdateComment':
        return updateComment(ctx, request as $6.UpdateCommentRequest);
      case 'DeleteComment':
        return deleteComment(ctx, request as $6.DeleteCommentRequest);
      case 'CreateForumEntry':
        return createForumEntry(ctx, request as $6.CreateForumEntryRequest);
      case 'UpdateForumEntry':
        return updateForumEntry(ctx, request as $6.UpdateForumEntryRequest);
      case 'DeleteForumEntry':
        return deleteForumEntry(ctx, request as $6.DeleteForumEntryRequest);
      case 'CreatePost':
        return createPost(ctx, request as $6.CreatePostRequest);
      case 'GetMyPost':
        return getMyPost(ctx, request as $6.GetMyPostRequest);
      case 'ListMyPosts':
        return listMyPosts(ctx, request as $6.ListMyPostsRequest);
      case 'UpdatePost':
        return updatePost(ctx, request as $6.UpdatePostRequest);
      case 'PublishPost':
        return publishPost(ctx, request as $6.PublishPostRequest);
      case 'UnpublishPost':
        return unpublishPost(ctx, request as $6.UnpublishPostRequest);
      case 'DeletePost':
        return deletePost(ctx, request as $6.DeletePostRequest);
      case 'SchedulePost':
        return schedulePost(ctx, request as $6.SchedulePostRequest);
      case 'ListPendingSchedules':
        return listPendingSchedules(
            ctx, request as $6.ListPendingSchedulesRequest);
      case 'CancelPostSchedule':
        return cancelPostSchedule(ctx, request as $6.CancelPostScheduleRequest);
      case 'ListStoreComments':
        return listStoreComments(ctx, request as $6.ListStoreCommentsRequest);
      case 'GetStoreSocialSummary':
        return getStoreSocialSummary(
            ctx, request as $6.GetStoreSocialSummaryRequest);
      case 'ListMySanctions':
        return listMySanctions(ctx, request as $6.ListMySanctionsRequest);
      case 'ListMyHiddenContent':
        return listMyHiddenContent(
            ctx, request as $6.ListMyHiddenContentRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SocialAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialAppServiceBase$messageJson;
}

abstract class SocialDelegatedServiceBase extends $pb.GeneratedService {
  $async.Future<$6.SocialDelegatedServiceGetStoreSocialSummaryResponse>
      getStoreSocialSummary($pb.ServerContext ctx,
          $6.SocialDelegatedServiceGetStoreSocialSummaryRequest request);
  $async.Future<$6.ListStorePostsResponse> listStorePosts(
      $pb.ServerContext ctx, $6.ListStorePostsRequest request);
  $async.Future<$6.GetStorePostResponse> getStorePost(
      $pb.ServerContext ctx, $6.GetStorePostRequest request);
  $async.Future<$6.CreateStorePostResponse> createStorePost(
      $pb.ServerContext ctx, $6.CreateStorePostRequest request);
  $async.Future<$6.UpdateStorePostResponse> updateStorePost(
      $pb.ServerContext ctx, $6.UpdateStorePostRequest request);
  $async.Future<$6.PublishStorePostResponse> publishStorePost(
      $pb.ServerContext ctx, $6.PublishStorePostRequest request);
  $async.Future<$6.UnpublishStorePostResponse> unpublishStorePost(
      $pb.ServerContext ctx, $6.UnpublishStorePostRequest request);
  $async.Future<$6.ScheduleStorePostResponse> scheduleStorePost(
      $pb.ServerContext ctx, $6.ScheduleStorePostRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetStoreSocialSummary':
        return $6.SocialDelegatedServiceGetStoreSocialSummaryRequest();
      case 'ListStorePosts':
        return $6.ListStorePostsRequest();
      case 'GetStorePost':
        return $6.GetStorePostRequest();
      case 'CreateStorePost':
        return $6.CreateStorePostRequest();
      case 'UpdateStorePost':
        return $6.UpdateStorePostRequest();
      case 'PublishStorePost':
        return $6.PublishStorePostRequest();
      case 'UnpublishStorePost':
        return $6.UnpublishStorePostRequest();
      case 'ScheduleStorePost':
        return $6.ScheduleStorePostRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetStoreSocialSummary':
        return getStoreSocialSummary(ctx,
            request as $6.SocialDelegatedServiceGetStoreSocialSummaryRequest);
      case 'ListStorePosts':
        return listStorePosts(ctx, request as $6.ListStorePostsRequest);
      case 'GetStorePost':
        return getStorePost(ctx, request as $6.GetStorePostRequest);
      case 'CreateStorePost':
        return createStorePost(ctx, request as $6.CreateStorePostRequest);
      case 'UpdateStorePost':
        return updateStorePost(ctx, request as $6.UpdateStorePostRequest);
      case 'PublishStorePost':
        return publishStorePost(ctx, request as $6.PublishStorePostRequest);
      case 'UnpublishStorePost':
        return unpublishStorePost(ctx, request as $6.UnpublishStorePostRequest);
      case 'ScheduleStorePost':
        return scheduleStorePost(ctx, request as $6.ScheduleStorePostRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SocialDelegatedServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SocialDelegatedServiceBase$messageJson;
}
