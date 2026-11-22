// This is a generated file - do not edit.
//
// Generated from hawkr/review/v1alpha1/review.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'review.pb.dart' as $2;
import 'review.pbjson.dart';

export 'review.pb.dart';

abstract class ReviewPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$2.ListProductReviewsResponse> listProductReviews(
      $pb.ServerContext ctx, $2.ListProductReviewsRequest request);
  $async.Future<$2.ListStoreReviewsResponse> listStoreReviews(
      $pb.ServerContext ctx, $2.ListStoreReviewsRequest request);
  $async.Future<$2.GetStoreReviewStatsResponse> getStoreReviewStats(
      $pb.ServerContext ctx, $2.GetStoreReviewStatsRequest request);
  $async.Future<$2.GetProductReviewStatsResponse> getProductReviewStats(
      $pb.ServerContext ctx, $2.GetProductReviewStatsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListProductReviews':
        return $2.ListProductReviewsRequest();
      case 'ListStoreReviews':
        return $2.ListStoreReviewsRequest();
      case 'GetStoreReviewStats':
        return $2.GetStoreReviewStatsRequest();
      case 'GetProductReviewStats':
        return $2.GetProductReviewStatsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListProductReviews':
        return listProductReviews(ctx, request as $2.ListProductReviewsRequest);
      case 'ListStoreReviews':
        return listStoreReviews(ctx, request as $2.ListStoreReviewsRequest);
      case 'GetStoreReviewStats':
        return getStoreReviewStats(
            ctx, request as $2.GetStoreReviewStatsRequest);
      case 'GetProductReviewStats':
        return getProductReviewStats(
            ctx, request as $2.GetProductReviewStatsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      ReviewPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ReviewPublicServiceBase$messageJson;
}

abstract class ReviewAppServiceBase extends $pb.GeneratedService {
  $async.Future<$2.GetReviewResponse> getReview(
      $pb.ServerContext ctx, $2.GetReviewRequest request);
  $async.Future<$2.ReplyReviewResponse> replyReview(
      $pb.ServerContext ctx, $2.ReplyReviewRequest request);
  $async.Future<$2.CreateReviewResponse> createReview(
      $pb.ServerContext ctx, $2.CreateReviewRequest request);
  $async.Future<$2.ListMyReviewsResponse> listMyReviews(
      $pb.ServerContext ctx, $2.ListMyReviewsRequest request);
  $async.Future<$2.ListOwnStoreReviewsResponse> listOwnStoreReviews(
      $pb.ServerContext ctx, $2.ListOwnStoreReviewsRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetReview':
        return $2.GetReviewRequest();
      case 'ReplyReview':
        return $2.ReplyReviewRequest();
      case 'CreateReview':
        return $2.CreateReviewRequest();
      case 'ListMyReviews':
        return $2.ListMyReviewsRequest();
      case 'ListOwnStoreReviews':
        return $2.ListOwnStoreReviewsRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetReview':
        return getReview(ctx, request as $2.GetReviewRequest);
      case 'ReplyReview':
        return replyReview(ctx, request as $2.ReplyReviewRequest);
      case 'CreateReview':
        return createReview(ctx, request as $2.CreateReviewRequest);
      case 'ListMyReviews':
        return listMyReviews(ctx, request as $2.ListMyReviewsRequest);
      case 'ListOwnStoreReviews':
        return listOwnStoreReviews(
            ctx, request as $2.ListOwnStoreReviewsRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => ReviewAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => ReviewAppServiceBase$messageJson;
}
