// This is a generated file - do not edit.
//
// Generated from hawkr/buyer/v1alpha1/buyer.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'buyer.pb.dart' as $6;
import 'buyer.pbjson.dart';

export 'buyer.pb.dart';

abstract class BuyerAppServiceBase extends $pb.GeneratedService {
  $async.Future<$6.GetProfileResponse> getProfile(
      $pb.ServerContext ctx, $6.GetProfileRequest request);
  $async.Future<$6.UpdateProfileResponse> updateProfile(
      $pb.ServerContext ctx, $6.UpdateProfileRequest request);
  $async.Future<$6.ListAddressesResponse> listAddresses(
      $pb.ServerContext ctx, $6.ListAddressesRequest request);
  $async.Future<$6.GetAddressResponse> getAddress(
      $pb.ServerContext ctx, $6.GetAddressRequest request);
  $async.Future<$6.CreateAddressResponse> createAddress(
      $pb.ServerContext ctx, $6.CreateAddressRequest request);
  $async.Future<$6.UpdateAddressResponse> updateAddress(
      $pb.ServerContext ctx, $6.UpdateAddressRequest request);
  $async.Future<$6.DeleteAddressResponse> deleteAddress(
      $pb.ServerContext ctx, $6.DeleteAddressRequest request);
  $async.Future<$6.ListFavoritesResponse> listFavorites(
      $pb.ServerContext ctx, $6.ListFavoritesRequest request);
  $async.Future<$6.CheckFavoriteResponse> checkFavorite(
      $pb.ServerContext ctx, $6.CheckFavoriteRequest request);
  $async.Future<$6.BatchCheckFavoritesResponse> batchCheckFavorites(
      $pb.ServerContext ctx, $6.BatchCheckFavoritesRequest request);
  $async.Future<$6.AddFavoriteResponse> addFavorite(
      $pb.ServerContext ctx, $6.AddFavoriteRequest request);
  $async.Future<$6.RemoveFavoriteResponse> removeFavorite(
      $pb.ServerContext ctx, $6.RemoveFavoriteRequest request);
  $async.Future<$6.ListFollowsResponse> listFollows(
      $pb.ServerContext ctx, $6.ListFollowsRequest request);
  $async.Future<$6.CheckFollowResponse> checkFollow(
      $pb.ServerContext ctx, $6.CheckFollowRequest request);
  $async.Future<$6.BatchCheckFollowsResponse> batchCheckFollows(
      $pb.ServerContext ctx, $6.BatchCheckFollowsRequest request);
  $async.Future<$6.FollowStoreResponse> followStore(
      $pb.ServerContext ctx, $6.FollowStoreRequest request);
  $async.Future<$6.UnfollowStoreResponse> unfollowStore(
      $pb.ServerContext ctx, $6.UnfollowStoreRequest request);
  $async.Future<$6.ListMemberCardsResponse> listMemberCards(
      $pb.ServerContext ctx, $6.ListMemberCardsRequest request);
  $async.Future<$6.ListSpendingRecordsResponse> listSpendingRecords(
      $pb.ServerContext ctx, $6.ListSpendingRecordsRequest request);
  $async.Future<$6.ListVisitsResponse> listVisits(
      $pb.ServerContext ctx, $6.ListVisitsRequest request);
  $async.Future<$6.RecordVisitResponse> recordVisit(
      $pb.ServerContext ctx, $6.RecordVisitRequest request);
  $async.Future<$6.ClearVisitsResponse> clearVisits(
      $pb.ServerContext ctx, $6.ClearVisitsRequest request);
  $async.Future<$6.ListRestockSubscriptionsResponse> listRestockSubscriptions(
      $pb.ServerContext ctx, $6.ListRestockSubscriptionsRequest request);
  $async.Future<$6.SubscribeRestockResponse> subscribeRestock(
      $pb.ServerContext ctx, $6.SubscribeRestockRequest request);
  $async.Future<$6.UnsubscribeRestockResponse> unsubscribeRestock(
      $pb.ServerContext ctx, $6.UnsubscribeRestockRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetProfile':
        return $6.GetProfileRequest();
      case 'UpdateProfile':
        return $6.UpdateProfileRequest();
      case 'ListAddresses':
        return $6.ListAddressesRequest();
      case 'GetAddress':
        return $6.GetAddressRequest();
      case 'CreateAddress':
        return $6.CreateAddressRequest();
      case 'UpdateAddress':
        return $6.UpdateAddressRequest();
      case 'DeleteAddress':
        return $6.DeleteAddressRequest();
      case 'ListFavorites':
        return $6.ListFavoritesRequest();
      case 'CheckFavorite':
        return $6.CheckFavoriteRequest();
      case 'BatchCheckFavorites':
        return $6.BatchCheckFavoritesRequest();
      case 'AddFavorite':
        return $6.AddFavoriteRequest();
      case 'RemoveFavorite':
        return $6.RemoveFavoriteRequest();
      case 'ListFollows':
        return $6.ListFollowsRequest();
      case 'CheckFollow':
        return $6.CheckFollowRequest();
      case 'BatchCheckFollows':
        return $6.BatchCheckFollowsRequest();
      case 'FollowStore':
        return $6.FollowStoreRequest();
      case 'UnfollowStore':
        return $6.UnfollowStoreRequest();
      case 'ListMemberCards':
        return $6.ListMemberCardsRequest();
      case 'ListSpendingRecords':
        return $6.ListSpendingRecordsRequest();
      case 'ListVisits':
        return $6.ListVisitsRequest();
      case 'RecordVisit':
        return $6.RecordVisitRequest();
      case 'ClearVisits':
        return $6.ClearVisitsRequest();
      case 'ListRestockSubscriptions':
        return $6.ListRestockSubscriptionsRequest();
      case 'SubscribeRestock':
        return $6.SubscribeRestockRequest();
      case 'UnsubscribeRestock':
        return $6.UnsubscribeRestockRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetProfile':
        return getProfile(ctx, request as $6.GetProfileRequest);
      case 'UpdateProfile':
        return updateProfile(ctx, request as $6.UpdateProfileRequest);
      case 'ListAddresses':
        return listAddresses(ctx, request as $6.ListAddressesRequest);
      case 'GetAddress':
        return getAddress(ctx, request as $6.GetAddressRequest);
      case 'CreateAddress':
        return createAddress(ctx, request as $6.CreateAddressRequest);
      case 'UpdateAddress':
        return updateAddress(ctx, request as $6.UpdateAddressRequest);
      case 'DeleteAddress':
        return deleteAddress(ctx, request as $6.DeleteAddressRequest);
      case 'ListFavorites':
        return listFavorites(ctx, request as $6.ListFavoritesRequest);
      case 'CheckFavorite':
        return checkFavorite(ctx, request as $6.CheckFavoriteRequest);
      case 'BatchCheckFavorites':
        return batchCheckFavorites(
            ctx, request as $6.BatchCheckFavoritesRequest);
      case 'AddFavorite':
        return addFavorite(ctx, request as $6.AddFavoriteRequest);
      case 'RemoveFavorite':
        return removeFavorite(ctx, request as $6.RemoveFavoriteRequest);
      case 'ListFollows':
        return listFollows(ctx, request as $6.ListFollowsRequest);
      case 'CheckFollow':
        return checkFollow(ctx, request as $6.CheckFollowRequest);
      case 'BatchCheckFollows':
        return batchCheckFollows(ctx, request as $6.BatchCheckFollowsRequest);
      case 'FollowStore':
        return followStore(ctx, request as $6.FollowStoreRequest);
      case 'UnfollowStore':
        return unfollowStore(ctx, request as $6.UnfollowStoreRequest);
      case 'ListMemberCards':
        return listMemberCards(ctx, request as $6.ListMemberCardsRequest);
      case 'ListSpendingRecords':
        return listSpendingRecords(
            ctx, request as $6.ListSpendingRecordsRequest);
      case 'ListVisits':
        return listVisits(ctx, request as $6.ListVisitsRequest);
      case 'RecordVisit':
        return recordVisit(ctx, request as $6.RecordVisitRequest);
      case 'ClearVisits':
        return clearVisits(ctx, request as $6.ClearVisitsRequest);
      case 'ListRestockSubscriptions':
        return listRestockSubscriptions(
            ctx, request as $6.ListRestockSubscriptionsRequest);
      case 'SubscribeRestock':
        return subscribeRestock(ctx, request as $6.SubscribeRestockRequest);
      case 'UnsubscribeRestock':
        return unsubscribeRestock(ctx, request as $6.UnsubscribeRestockRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => BuyerAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => BuyerAppServiceBase$messageJson;
}
