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

import 'buyer.pb.dart' as $7;
import 'buyer.pbjson.dart';

export 'buyer.pb.dart';

abstract class BuyerAppServiceBase extends $pb.GeneratedService {
  $async.Future<$7.GetProfileResponse> getProfile(
      $pb.ServerContext ctx, $7.GetProfileRequest request);
  $async.Future<$7.UpdateProfileResponse> updateProfile(
      $pb.ServerContext ctx, $7.UpdateProfileRequest request);
  $async.Future<$7.ListAddressesResponse> listAddresses(
      $pb.ServerContext ctx, $7.ListAddressesRequest request);
  $async.Future<$7.GetAddressResponse> getAddress(
      $pb.ServerContext ctx, $7.GetAddressRequest request);
  $async.Future<$7.CreateAddressResponse> createAddress(
      $pb.ServerContext ctx, $7.CreateAddressRequest request);
  $async.Future<$7.UpdateAddressResponse> updateAddress(
      $pb.ServerContext ctx, $7.UpdateAddressRequest request);
  $async.Future<$7.DeleteAddressResponse> deleteAddress(
      $pb.ServerContext ctx, $7.DeleteAddressRequest request);
  $async.Future<$7.ListFavoritesResponse> listFavorites(
      $pb.ServerContext ctx, $7.ListFavoritesRequest request);
  $async.Future<$7.CheckFavoriteResponse> checkFavorite(
      $pb.ServerContext ctx, $7.CheckFavoriteRequest request);
  $async.Future<$7.BatchCheckFavoritesResponse> batchCheckFavorites(
      $pb.ServerContext ctx, $7.BatchCheckFavoritesRequest request);
  $async.Future<$7.AddFavoriteResponse> addFavorite(
      $pb.ServerContext ctx, $7.AddFavoriteRequest request);
  $async.Future<$7.RemoveFavoriteResponse> removeFavorite(
      $pb.ServerContext ctx, $7.RemoveFavoriteRequest request);
  $async.Future<$7.ListFollowedStoreProductsResponse> listFollowedStoreProducts(
      $pb.ServerContext ctx, $7.ListFollowedStoreProductsRequest request);
  $async.Future<$7.ListFollowsResponse> listFollows(
      $pb.ServerContext ctx, $7.ListFollowsRequest request);
  $async.Future<$7.CheckFollowResponse> checkFollow(
      $pb.ServerContext ctx, $7.CheckFollowRequest request);
  $async.Future<$7.BatchCheckFollowsResponse> batchCheckFollows(
      $pb.ServerContext ctx, $7.BatchCheckFollowsRequest request);
  $async.Future<$7.FollowStoreResponse> followStore(
      $pb.ServerContext ctx, $7.FollowStoreRequest request);
  $async.Future<$7.UnfollowStoreResponse> unfollowStore(
      $pb.ServerContext ctx, $7.UnfollowStoreRequest request);
  $async.Future<$7.ListMemberCardsResponse> listMemberCards(
      $pb.ServerContext ctx, $7.ListMemberCardsRequest request);
  $async.Future<$7.ListSpendingRecordsResponse> listSpendingRecords(
      $pb.ServerContext ctx, $7.ListSpendingRecordsRequest request);
  $async.Future<$7.ListVisitsResponse> listVisits(
      $pb.ServerContext ctx, $7.ListVisitsRequest request);
  $async.Future<$7.RecordVisitResponse> recordVisit(
      $pb.ServerContext ctx, $7.RecordVisitRequest request);
  $async.Future<$7.ClearVisitsResponse> clearVisits(
      $pb.ServerContext ctx, $7.ClearVisitsRequest request);
  $async.Future<$7.ListRestockSubscriptionsResponse> listRestockSubscriptions(
      $pb.ServerContext ctx, $7.ListRestockSubscriptionsRequest request);
  $async.Future<$7.SubscribeRestockResponse> subscribeRestock(
      $pb.ServerContext ctx, $7.SubscribeRestockRequest request);
  $async.Future<$7.UnsubscribeRestockResponse> unsubscribeRestock(
      $pb.ServerContext ctx, $7.UnsubscribeRestockRequest request);
  $async.Future<$7.BlockAuthorResponse> blockAuthor(
      $pb.ServerContext ctx, $7.BlockAuthorRequest request);
  $async.Future<$7.BlockStoreResponse> blockStore(
      $pb.ServerContext ctx, $7.BlockStoreRequest request);
  $async.Future<$7.UnblockBuyerResponse> unblockBuyer(
      $pb.ServerContext ctx, $7.UnblockBuyerRequest request);
  $async.Future<$7.UnblockStoreResponse> unblockStore(
      $pb.ServerContext ctx, $7.UnblockStoreRequest request);
  $async.Future<$7.ListBlockedBuyersResponse> listBlockedBuyers(
      $pb.ServerContext ctx, $7.ListBlockedBuyersRequest request);
  $async.Future<$7.ListBlockedStoresResponse> listBlockedStores(
      $pb.ServerContext ctx, $7.ListBlockedStoresRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetProfile':
        return $7.GetProfileRequest();
      case 'UpdateProfile':
        return $7.UpdateProfileRequest();
      case 'ListAddresses':
        return $7.ListAddressesRequest();
      case 'GetAddress':
        return $7.GetAddressRequest();
      case 'CreateAddress':
        return $7.CreateAddressRequest();
      case 'UpdateAddress':
        return $7.UpdateAddressRequest();
      case 'DeleteAddress':
        return $7.DeleteAddressRequest();
      case 'ListFavorites':
        return $7.ListFavoritesRequest();
      case 'CheckFavorite':
        return $7.CheckFavoriteRequest();
      case 'BatchCheckFavorites':
        return $7.BatchCheckFavoritesRequest();
      case 'AddFavorite':
        return $7.AddFavoriteRequest();
      case 'RemoveFavorite':
        return $7.RemoveFavoriteRequest();
      case 'ListFollowedStoreProducts':
        return $7.ListFollowedStoreProductsRequest();
      case 'ListFollows':
        return $7.ListFollowsRequest();
      case 'CheckFollow':
        return $7.CheckFollowRequest();
      case 'BatchCheckFollows':
        return $7.BatchCheckFollowsRequest();
      case 'FollowStore':
        return $7.FollowStoreRequest();
      case 'UnfollowStore':
        return $7.UnfollowStoreRequest();
      case 'ListMemberCards':
        return $7.ListMemberCardsRequest();
      case 'ListSpendingRecords':
        return $7.ListSpendingRecordsRequest();
      case 'ListVisits':
        return $7.ListVisitsRequest();
      case 'RecordVisit':
        return $7.RecordVisitRequest();
      case 'ClearVisits':
        return $7.ClearVisitsRequest();
      case 'ListRestockSubscriptions':
        return $7.ListRestockSubscriptionsRequest();
      case 'SubscribeRestock':
        return $7.SubscribeRestockRequest();
      case 'UnsubscribeRestock':
        return $7.UnsubscribeRestockRequest();
      case 'BlockAuthor':
        return $7.BlockAuthorRequest();
      case 'BlockStore':
        return $7.BlockStoreRequest();
      case 'UnblockBuyer':
        return $7.UnblockBuyerRequest();
      case 'UnblockStore':
        return $7.UnblockStoreRequest();
      case 'ListBlockedBuyers':
        return $7.ListBlockedBuyersRequest();
      case 'ListBlockedStores':
        return $7.ListBlockedStoresRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetProfile':
        return getProfile(ctx, request as $7.GetProfileRequest);
      case 'UpdateProfile':
        return updateProfile(ctx, request as $7.UpdateProfileRequest);
      case 'ListAddresses':
        return listAddresses(ctx, request as $7.ListAddressesRequest);
      case 'GetAddress':
        return getAddress(ctx, request as $7.GetAddressRequest);
      case 'CreateAddress':
        return createAddress(ctx, request as $7.CreateAddressRequest);
      case 'UpdateAddress':
        return updateAddress(ctx, request as $7.UpdateAddressRequest);
      case 'DeleteAddress':
        return deleteAddress(ctx, request as $7.DeleteAddressRequest);
      case 'ListFavorites':
        return listFavorites(ctx, request as $7.ListFavoritesRequest);
      case 'CheckFavorite':
        return checkFavorite(ctx, request as $7.CheckFavoriteRequest);
      case 'BatchCheckFavorites':
        return batchCheckFavorites(
            ctx, request as $7.BatchCheckFavoritesRequest);
      case 'AddFavorite':
        return addFavorite(ctx, request as $7.AddFavoriteRequest);
      case 'RemoveFavorite':
        return removeFavorite(ctx, request as $7.RemoveFavoriteRequest);
      case 'ListFollowedStoreProducts':
        return listFollowedStoreProducts(
            ctx, request as $7.ListFollowedStoreProductsRequest);
      case 'ListFollows':
        return listFollows(ctx, request as $7.ListFollowsRequest);
      case 'CheckFollow':
        return checkFollow(ctx, request as $7.CheckFollowRequest);
      case 'BatchCheckFollows':
        return batchCheckFollows(ctx, request as $7.BatchCheckFollowsRequest);
      case 'FollowStore':
        return followStore(ctx, request as $7.FollowStoreRequest);
      case 'UnfollowStore':
        return unfollowStore(ctx, request as $7.UnfollowStoreRequest);
      case 'ListMemberCards':
        return listMemberCards(ctx, request as $7.ListMemberCardsRequest);
      case 'ListSpendingRecords':
        return listSpendingRecords(
            ctx, request as $7.ListSpendingRecordsRequest);
      case 'ListVisits':
        return listVisits(ctx, request as $7.ListVisitsRequest);
      case 'RecordVisit':
        return recordVisit(ctx, request as $7.RecordVisitRequest);
      case 'ClearVisits':
        return clearVisits(ctx, request as $7.ClearVisitsRequest);
      case 'ListRestockSubscriptions':
        return listRestockSubscriptions(
            ctx, request as $7.ListRestockSubscriptionsRequest);
      case 'SubscribeRestock':
        return subscribeRestock(ctx, request as $7.SubscribeRestockRequest);
      case 'UnsubscribeRestock':
        return unsubscribeRestock(ctx, request as $7.UnsubscribeRestockRequest);
      case 'BlockAuthor':
        return blockAuthor(ctx, request as $7.BlockAuthorRequest);
      case 'BlockStore':
        return blockStore(ctx, request as $7.BlockStoreRequest);
      case 'UnblockBuyer':
        return unblockBuyer(ctx, request as $7.UnblockBuyerRequest);
      case 'UnblockStore':
        return unblockStore(ctx, request as $7.UnblockStoreRequest);
      case 'ListBlockedBuyers':
        return listBlockedBuyers(ctx, request as $7.ListBlockedBuyersRequest);
      case 'ListBlockedStores':
        return listBlockedStores(ctx, request as $7.ListBlockedStoresRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => BuyerAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => BuyerAppServiceBase$messageJson;
}
