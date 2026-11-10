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

import 'seller.pb.dart' as $10;
import 'seller.pbjson.dart';

export 'seller.pb.dart';

abstract class SellerAppServiceBase extends $pb.GeneratedService {
  $async.Future<$10.ApplyStoreResponse> applyStore(
      $pb.ServerContext ctx, $10.ApplyStoreRequest request);
  $async.Future<$10.GetMyStoreOpeningResponse> getMyStoreOpening(
      $pb.ServerContext ctx, $10.GetMyStoreOpeningRequest request);
  $async.Future<$10.GetMyStoreResponse> getMyStore(
      $pb.ServerContext ctx, $10.GetMyStoreRequest request);
  $async.Future<$10.GetMyOnboardingResponse> getMyOnboarding(
      $pb.ServerContext ctx, $10.GetMyOnboardingRequest request);
  $async.Future<$10.RefreshMyPaymentAccountResponse> refreshMyPaymentAccount(
      $pb.ServerContext ctx, $10.RefreshMyPaymentAccountRequest request);
  $async.Future<$10.UpdateMyStoreResponse> updateMyStore(
      $pb.ServerContext ctx, $10.UpdateMyStoreRequest request);
  $async.Future<$10.CompleteMyStoreOnboardingResponse>
      completeMyStoreOnboarding(
          $pb.ServerContext ctx, $10.CompleteMyStoreOnboardingRequest request);
  $async.Future<$10.ListMyFollowersResponse> listMyFollowers(
      $pb.ServerContext ctx, $10.ListMyFollowersRequest request);
  $async.Future<$10.GetMyDashboardResponse> getMyDashboard(
      $pb.ServerContext ctx, $10.GetMyDashboardRequest request);
  $async.Future<$10.CloseMyStoreResponse> closeMyStore(
      $pb.ServerContext ctx, $10.CloseMyStoreRequest request);
  $async.Future<$10.SaveKYCDraftResponse> saveKYCDraft(
      $pb.ServerContext ctx, $10.SaveKYCDraftRequest request);
  $async.Future<$10.GetMyKYCResponse> getMyKYC(
      $pb.ServerContext ctx, $10.GetMyKYCRequest request);
  $async.Future<$10.SubmitKYCResponse> submitKYC(
      $pb.ServerContext ctx, $10.SubmitKYCRequest request);
  $async.Future<$10.CancelKYCResponse> cancelKYC(
      $pb.ServerContext ctx, $10.CancelKYCRequest request);
  $async.Future<$10.GetMyPaymentAccountResponse> getMyPaymentAccount(
      $pb.ServerContext ctx, $10.GetMyPaymentAccountRequest request);
  $async.Future<$10.CreatePaymentOnboardingSessionResponse>
      createPaymentOnboardingSession($pb.ServerContext ctx,
          $10.CreatePaymentOnboardingSessionRequest request);
  $async.Future<$10.LookupBusinessRegistrationResponse>
      lookupBusinessRegistration(
          $pb.ServerContext ctx, $10.LookupBusinessRegistrationRequest request);
  $async.Future<$10.ListMyMeasuresResponse> listMyMeasures(
      $pb.ServerContext ctx, $10.ListMyMeasuresRequest request);
  $async.Future<$10.GetMyMeasureResponse> getMyMeasure(
      $pb.ServerContext ctx, $10.GetMyMeasureRequest request);
  $async.Future<$10.SubmitAppealResponse> submitAppeal(
      $pb.ServerContext ctx, $10.SubmitAppealRequest request);
  $async.Future<$10.RequestRemediationResponse> requestRemediation(
      $pb.ServerContext ctx, $10.RequestRemediationRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ApplyStore':
        return $10.ApplyStoreRequest();
      case 'GetMyStoreOpening':
        return $10.GetMyStoreOpeningRequest();
      case 'GetMyStore':
        return $10.GetMyStoreRequest();
      case 'GetMyOnboarding':
        return $10.GetMyOnboardingRequest();
      case 'RefreshMyPaymentAccount':
        return $10.RefreshMyPaymentAccountRequest();
      case 'UpdateMyStore':
        return $10.UpdateMyStoreRequest();
      case 'CompleteMyStoreOnboarding':
        return $10.CompleteMyStoreOnboardingRequest();
      case 'ListMyFollowers':
        return $10.ListMyFollowersRequest();
      case 'GetMyDashboard':
        return $10.GetMyDashboardRequest();
      case 'CloseMyStore':
        return $10.CloseMyStoreRequest();
      case 'SaveKYCDraft':
        return $10.SaveKYCDraftRequest();
      case 'GetMyKYC':
        return $10.GetMyKYCRequest();
      case 'SubmitKYC':
        return $10.SubmitKYCRequest();
      case 'CancelKYC':
        return $10.CancelKYCRequest();
      case 'GetMyPaymentAccount':
        return $10.GetMyPaymentAccountRequest();
      case 'CreatePaymentOnboardingSession':
        return $10.CreatePaymentOnboardingSessionRequest();
      case 'LookupBusinessRegistration':
        return $10.LookupBusinessRegistrationRequest();
      case 'ListMyMeasures':
        return $10.ListMyMeasuresRequest();
      case 'GetMyMeasure':
        return $10.GetMyMeasureRequest();
      case 'SubmitAppeal':
        return $10.SubmitAppealRequest();
      case 'RequestRemediation':
        return $10.RequestRemediationRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ApplyStore':
        return applyStore(ctx, request as $10.ApplyStoreRequest);
      case 'GetMyStoreOpening':
        return getMyStoreOpening(ctx, request as $10.GetMyStoreOpeningRequest);
      case 'GetMyStore':
        return getMyStore(ctx, request as $10.GetMyStoreRequest);
      case 'GetMyOnboarding':
        return getMyOnboarding(ctx, request as $10.GetMyOnboardingRequest);
      case 'RefreshMyPaymentAccount':
        return refreshMyPaymentAccount(
            ctx, request as $10.RefreshMyPaymentAccountRequest);
      case 'UpdateMyStore':
        return updateMyStore(ctx, request as $10.UpdateMyStoreRequest);
      case 'CompleteMyStoreOnboarding':
        return completeMyStoreOnboarding(
            ctx, request as $10.CompleteMyStoreOnboardingRequest);
      case 'ListMyFollowers':
        return listMyFollowers(ctx, request as $10.ListMyFollowersRequest);
      case 'GetMyDashboard':
        return getMyDashboard(ctx, request as $10.GetMyDashboardRequest);
      case 'CloseMyStore':
        return closeMyStore(ctx, request as $10.CloseMyStoreRequest);
      case 'SaveKYCDraft':
        return saveKYCDraft(ctx, request as $10.SaveKYCDraftRequest);
      case 'GetMyKYC':
        return getMyKYC(ctx, request as $10.GetMyKYCRequest);
      case 'SubmitKYC':
        return submitKYC(ctx, request as $10.SubmitKYCRequest);
      case 'CancelKYC':
        return cancelKYC(ctx, request as $10.CancelKYCRequest);
      case 'GetMyPaymentAccount':
        return getMyPaymentAccount(
            ctx, request as $10.GetMyPaymentAccountRequest);
      case 'CreatePaymentOnboardingSession':
        return createPaymentOnboardingSession(
            ctx, request as $10.CreatePaymentOnboardingSessionRequest);
      case 'LookupBusinessRegistration':
        return lookupBusinessRegistration(
            ctx, request as $10.LookupBusinessRegistrationRequest);
      case 'ListMyMeasures':
        return listMyMeasures(ctx, request as $10.ListMyMeasuresRequest);
      case 'GetMyMeasure':
        return getMyMeasure(ctx, request as $10.GetMyMeasureRequest);
      case 'SubmitAppeal':
        return submitAppeal(ctx, request as $10.SubmitAppealRequest);
      case 'RequestRemediation':
        return requestRemediation(
            ctx, request as $10.RequestRemediationRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SellerAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerAppServiceBase$messageJson;
}

abstract class SellerPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$10.GetStoreResponse> getStore(
      $pb.ServerContext ctx, $10.GetStoreRequest request);
  $async.Future<$10.ListStoresResponse> listStores(
      $pb.ServerContext ctx, $10.ListStoresRequest request);
  $async.Future<$10.BatchGetStoresResponse> batchGetStores(
      $pb.ServerContext ctx, $10.BatchGetStoresRequest request);
  $async.Future<$10.GetSellerAgreementsResponse> getSellerAgreements(
      $pb.ServerContext ctx, $10.GetSellerAgreementsRequest request);
  $async.Future<$10.RecordProductViewResponse> recordProductView(
      $pb.ServerContext ctx, $10.RecordProductViewRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetStore':
        return $10.GetStoreRequest();
      case 'ListStores':
        return $10.ListStoresRequest();
      case 'BatchGetStores':
        return $10.BatchGetStoresRequest();
      case 'GetSellerAgreements':
        return $10.GetSellerAgreementsRequest();
      case 'RecordProductView':
        return $10.RecordProductViewRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetStore':
        return getStore(ctx, request as $10.GetStoreRequest);
      case 'ListStores':
        return listStores(ctx, request as $10.ListStoresRequest);
      case 'BatchGetStores':
        return batchGetStores(ctx, request as $10.BatchGetStoresRequest);
      case 'GetSellerAgreements':
        return getSellerAgreements(
            ctx, request as $10.GetSellerAgreementsRequest);
      case 'RecordProductView':
        return recordProductView(ctx, request as $10.RecordProductViewRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerPublicServiceBase$messageJson;
}
