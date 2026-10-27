// This is a generated file - do not edit.
//
// Generated from hawkr/seller/v1alpha1/seller.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'seller.pb.dart' as $9;
import 'seller.pbjson.dart';

export 'seller.pb.dart';

abstract class SellerAppServiceBase extends $pb.GeneratedService {
  $async.Future<$9.ApplyStoreResponse> applyStore(
      $pb.ServerContext ctx, $9.ApplyStoreRequest request);
  $async.Future<$9.GetMyStoreOpeningResponse> getMyStoreOpening(
      $pb.ServerContext ctx, $9.GetMyStoreOpeningRequest request);
  $async.Future<$9.GetMyStoreResponse> getMyStore(
      $pb.ServerContext ctx, $9.GetMyStoreRequest request);
  $async.Future<$9.UpdateMyStoreResponse> updateMyStore(
      $pb.ServerContext ctx, $9.UpdateMyStoreRequest request);
  $async.Future<$9.CompleteMyStoreOnboardingResponse> completeMyStoreOnboarding(
      $pb.ServerContext ctx, $9.CompleteMyStoreOnboardingRequest request);
  $async.Future<$9.ListMyFollowersResponse> listMyFollowers(
      $pb.ServerContext ctx, $9.ListMyFollowersRequest request);
  $async.Future<$9.GetMyDashboardResponse> getMyDashboard(
      $pb.ServerContext ctx, $9.GetMyDashboardRequest request);
  $async.Future<$9.SubmitFoodRegistrationResponse> submitFoodRegistration(
      $pb.ServerContext ctx, $9.SubmitFoodRegistrationRequest request);
  $async.Future<$9.WithdrawFoodRegistrationResponse> withdrawFoodRegistration(
      $pb.ServerContext ctx, $9.WithdrawFoodRegistrationRequest request);
  $async.Future<$9.ClearFoodRegistrationResponse> clearFoodRegistration(
      $pb.ServerContext ctx, $9.ClearFoodRegistrationRequest request);
  $async.Future<$9.CloseMyStoreResponse> closeMyStore(
      $pb.ServerContext ctx, $9.CloseMyStoreRequest request);
  $async.Future<$9.SaveKYCDraftResponse> saveKYCDraft(
      $pb.ServerContext ctx, $9.SaveKYCDraftRequest request);
  $async.Future<$9.GetMyKYCResponse> getMyKYC(
      $pb.ServerContext ctx, $9.GetMyKYCRequest request);
  $async.Future<$9.SubmitKYCResponse> submitKYC(
      $pb.ServerContext ctx, $9.SubmitKYCRequest request);
  $async.Future<$9.CancelKYCResponse> cancelKYC(
      $pb.ServerContext ctx, $9.CancelKYCRequest request);
  $async.Future<$9.LookupBusinessRegistrationResponse>
      lookupBusinessRegistration(
          $pb.ServerContext ctx, $9.LookupBusinessRegistrationRequest request);
  $async.Future<$9.ListMyMeasuresResponse> listMyMeasures(
      $pb.ServerContext ctx, $9.ListMyMeasuresRequest request);
  $async.Future<$9.GetMyMeasureResponse> getMyMeasure(
      $pb.ServerContext ctx, $9.GetMyMeasureRequest request);
  $async.Future<$9.SubmitAppealResponse> submitAppeal(
      $pb.ServerContext ctx, $9.SubmitAppealRequest request);
  $async.Future<$9.RequestRemediationResponse> requestRemediation(
      $pb.ServerContext ctx, $9.RequestRemediationRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ApplyStore':
        return $9.ApplyStoreRequest();
      case 'GetMyStoreOpening':
        return $9.GetMyStoreOpeningRequest();
      case 'GetMyStore':
        return $9.GetMyStoreRequest();
      case 'UpdateMyStore':
        return $9.UpdateMyStoreRequest();
      case 'CompleteMyStoreOnboarding':
        return $9.CompleteMyStoreOnboardingRequest();
      case 'ListMyFollowers':
        return $9.ListMyFollowersRequest();
      case 'GetMyDashboard':
        return $9.GetMyDashboardRequest();
      case 'SubmitFoodRegistration':
        return $9.SubmitFoodRegistrationRequest();
      case 'WithdrawFoodRegistration':
        return $9.WithdrawFoodRegistrationRequest();
      case 'ClearFoodRegistration':
        return $9.ClearFoodRegistrationRequest();
      case 'CloseMyStore':
        return $9.CloseMyStoreRequest();
      case 'SaveKYCDraft':
        return $9.SaveKYCDraftRequest();
      case 'GetMyKYC':
        return $9.GetMyKYCRequest();
      case 'SubmitKYC':
        return $9.SubmitKYCRequest();
      case 'CancelKYC':
        return $9.CancelKYCRequest();
      case 'LookupBusinessRegistration':
        return $9.LookupBusinessRegistrationRequest();
      case 'ListMyMeasures':
        return $9.ListMyMeasuresRequest();
      case 'GetMyMeasure':
        return $9.GetMyMeasureRequest();
      case 'SubmitAppeal':
        return $9.SubmitAppealRequest();
      case 'RequestRemediation':
        return $9.RequestRemediationRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ApplyStore':
        return applyStore(ctx, request as $9.ApplyStoreRequest);
      case 'GetMyStoreOpening':
        return getMyStoreOpening(ctx, request as $9.GetMyStoreOpeningRequest);
      case 'GetMyStore':
        return getMyStore(ctx, request as $9.GetMyStoreRequest);
      case 'UpdateMyStore':
        return updateMyStore(ctx, request as $9.UpdateMyStoreRequest);
      case 'CompleteMyStoreOnboarding':
        return completeMyStoreOnboarding(
            ctx, request as $9.CompleteMyStoreOnboardingRequest);
      case 'ListMyFollowers':
        return listMyFollowers(ctx, request as $9.ListMyFollowersRequest);
      case 'GetMyDashboard':
        return getMyDashboard(ctx, request as $9.GetMyDashboardRequest);
      case 'SubmitFoodRegistration':
        return submitFoodRegistration(
            ctx, request as $9.SubmitFoodRegistrationRequest);
      case 'WithdrawFoodRegistration':
        return withdrawFoodRegistration(
            ctx, request as $9.WithdrawFoodRegistrationRequest);
      case 'ClearFoodRegistration':
        return clearFoodRegistration(
            ctx, request as $9.ClearFoodRegistrationRequest);
      case 'CloseMyStore':
        return closeMyStore(ctx, request as $9.CloseMyStoreRequest);
      case 'SaveKYCDraft':
        return saveKYCDraft(ctx, request as $9.SaveKYCDraftRequest);
      case 'GetMyKYC':
        return getMyKYC(ctx, request as $9.GetMyKYCRequest);
      case 'SubmitKYC':
        return submitKYC(ctx, request as $9.SubmitKYCRequest);
      case 'CancelKYC':
        return cancelKYC(ctx, request as $9.CancelKYCRequest);
      case 'LookupBusinessRegistration':
        return lookupBusinessRegistration(
            ctx, request as $9.LookupBusinessRegistrationRequest);
      case 'ListMyMeasures':
        return listMyMeasures(ctx, request as $9.ListMyMeasuresRequest);
      case 'GetMyMeasure':
        return getMyMeasure(ctx, request as $9.GetMyMeasureRequest);
      case 'SubmitAppeal':
        return submitAppeal(ctx, request as $9.SubmitAppealRequest);
      case 'RequestRemediation':
        return requestRemediation(ctx, request as $9.RequestRemediationRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SellerAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerAppServiceBase$messageJson;
}

abstract class SellerPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$9.GetStoreResponse> getStore(
      $pb.ServerContext ctx, $9.GetStoreRequest request);
  $async.Future<$9.ListStoresResponse> listStores(
      $pb.ServerContext ctx, $9.ListStoresRequest request);
  $async.Future<$9.BatchGetStoresResponse> batchGetStores(
      $pb.ServerContext ctx, $9.BatchGetStoresRequest request);
  $async.Future<$9.GetSellerAgreementsResponse> getSellerAgreements(
      $pb.ServerContext ctx, $9.GetSellerAgreementsRequest request);
  $async.Future<$9.RecordProductViewResponse> recordProductView(
      $pb.ServerContext ctx, $9.RecordProductViewRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetStore':
        return $9.GetStoreRequest();
      case 'ListStores':
        return $9.ListStoresRequest();
      case 'BatchGetStores':
        return $9.BatchGetStoresRequest();
      case 'GetSellerAgreements':
        return $9.GetSellerAgreementsRequest();
      case 'RecordProductView':
        return $9.RecordProductViewRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetStore':
        return getStore(ctx, request as $9.GetStoreRequest);
      case 'ListStores':
        return listStores(ctx, request as $9.ListStoresRequest);
      case 'BatchGetStores':
        return batchGetStores(ctx, request as $9.BatchGetStoresRequest);
      case 'GetSellerAgreements':
        return getSellerAgreements(
            ctx, request as $9.GetSellerAgreementsRequest);
      case 'RecordProductView':
        return recordProductView(ctx, request as $9.RecordProductViewRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerPublicServiceBase$messageJson;
}
