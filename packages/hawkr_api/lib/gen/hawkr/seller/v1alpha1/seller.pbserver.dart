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

import 'seller.pb.dart' as $11;
import 'seller.pbjson.dart';

export 'seller.pb.dart';

abstract class SellerAppServiceBase extends $pb.GeneratedService {
  $async.Future<$11.ApplyStoreResponse> applyStore(
      $pb.ServerContext ctx, $11.ApplyStoreRequest request);
  $async.Future<$11.GetMyStoreOpeningResponse> getMyStoreOpening(
      $pb.ServerContext ctx, $11.GetMyStoreOpeningRequest request);
  $async.Future<$11.GetMyStoreResponse> getMyStore(
      $pb.ServerContext ctx, $11.GetMyStoreRequest request);
  $async.Future<$11.GetMyOnboardingResponse> getMyOnboarding(
      $pb.ServerContext ctx, $11.GetMyOnboardingRequest request);
  $async.Future<$11.RefreshMyPaymentAccountResponse> refreshMyPaymentAccount(
      $pb.ServerContext ctx, $11.RefreshMyPaymentAccountRequest request);
  $async.Future<$11.UpdateMyStoreResponse> updateMyStore(
      $pb.ServerContext ctx, $11.UpdateMyStoreRequest request);
  $async.Future<$11.CompleteMyStoreOnboardingResponse>
      completeMyStoreOnboarding(
          $pb.ServerContext ctx, $11.CompleteMyStoreOnboardingRequest request);
  $async.Future<$11.ListMyFollowersResponse> listMyFollowers(
      $pb.ServerContext ctx, $11.ListMyFollowersRequest request);
  $async.Future<$11.GetMyDashboardResponse> getMyDashboard(
      $pb.ServerContext ctx, $11.GetMyDashboardRequest request);
  $async.Future<$11.CloseMyStoreResponse> closeMyStore(
      $pb.ServerContext ctx, $11.CloseMyStoreRequest request);
  $async.Future<$11.SaveKYCDraftResponse> saveKYCDraft(
      $pb.ServerContext ctx, $11.SaveKYCDraftRequest request);
  $async.Future<$11.GetMyKYCResponse> getMyKYC(
      $pb.ServerContext ctx, $11.GetMyKYCRequest request);
  $async.Future<$11.SubmitKYCResponse> submitKYC(
      $pb.ServerContext ctx, $11.SubmitKYCRequest request);
  $async.Future<$11.CancelKYCResponse> cancelKYC(
      $pb.ServerContext ctx, $11.CancelKYCRequest request);
  $async.Future<$11.GetMyPaymentAccountResponse> getMyPaymentAccount(
      $pb.ServerContext ctx, $11.GetMyPaymentAccountRequest request);
  $async.Future<$11.CreatePaymentOnboardingSessionResponse>
      createPaymentOnboardingSession($pb.ServerContext ctx,
          $11.CreatePaymentOnboardingSessionRequest request);
  $async.Future<$11.LookupBusinessRegistrationResponse>
      lookupBusinessRegistration(
          $pb.ServerContext ctx, $11.LookupBusinessRegistrationRequest request);
  $async.Future<$11.ListMyMeasuresResponse> listMyMeasures(
      $pb.ServerContext ctx, $11.ListMyMeasuresRequest request);
  $async.Future<$11.GetMyMeasureResponse> getMyMeasure(
      $pb.ServerContext ctx, $11.GetMyMeasureRequest request);
  $async.Future<$11.SubmitAppealResponse> submitAppeal(
      $pb.ServerContext ctx, $11.SubmitAppealRequest request);
  $async.Future<$11.RequestRemediationResponse> requestRemediation(
      $pb.ServerContext ctx, $11.RequestRemediationRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ApplyStore':
        return $11.ApplyStoreRequest();
      case 'GetMyStoreOpening':
        return $11.GetMyStoreOpeningRequest();
      case 'GetMyStore':
        return $11.GetMyStoreRequest();
      case 'GetMyOnboarding':
        return $11.GetMyOnboardingRequest();
      case 'RefreshMyPaymentAccount':
        return $11.RefreshMyPaymentAccountRequest();
      case 'UpdateMyStore':
        return $11.UpdateMyStoreRequest();
      case 'CompleteMyStoreOnboarding':
        return $11.CompleteMyStoreOnboardingRequest();
      case 'ListMyFollowers':
        return $11.ListMyFollowersRequest();
      case 'GetMyDashboard':
        return $11.GetMyDashboardRequest();
      case 'CloseMyStore':
        return $11.CloseMyStoreRequest();
      case 'SaveKYCDraft':
        return $11.SaveKYCDraftRequest();
      case 'GetMyKYC':
        return $11.GetMyKYCRequest();
      case 'SubmitKYC':
        return $11.SubmitKYCRequest();
      case 'CancelKYC':
        return $11.CancelKYCRequest();
      case 'GetMyPaymentAccount':
        return $11.GetMyPaymentAccountRequest();
      case 'CreatePaymentOnboardingSession':
        return $11.CreatePaymentOnboardingSessionRequest();
      case 'LookupBusinessRegistration':
        return $11.LookupBusinessRegistrationRequest();
      case 'ListMyMeasures':
        return $11.ListMyMeasuresRequest();
      case 'GetMyMeasure':
        return $11.GetMyMeasureRequest();
      case 'SubmitAppeal':
        return $11.SubmitAppealRequest();
      case 'RequestRemediation':
        return $11.RequestRemediationRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ApplyStore':
        return applyStore(ctx, request as $11.ApplyStoreRequest);
      case 'GetMyStoreOpening':
        return getMyStoreOpening(ctx, request as $11.GetMyStoreOpeningRequest);
      case 'GetMyStore':
        return getMyStore(ctx, request as $11.GetMyStoreRequest);
      case 'GetMyOnboarding':
        return getMyOnboarding(ctx, request as $11.GetMyOnboardingRequest);
      case 'RefreshMyPaymentAccount':
        return refreshMyPaymentAccount(
            ctx, request as $11.RefreshMyPaymentAccountRequest);
      case 'UpdateMyStore':
        return updateMyStore(ctx, request as $11.UpdateMyStoreRequest);
      case 'CompleteMyStoreOnboarding':
        return completeMyStoreOnboarding(
            ctx, request as $11.CompleteMyStoreOnboardingRequest);
      case 'ListMyFollowers':
        return listMyFollowers(ctx, request as $11.ListMyFollowersRequest);
      case 'GetMyDashboard':
        return getMyDashboard(ctx, request as $11.GetMyDashboardRequest);
      case 'CloseMyStore':
        return closeMyStore(ctx, request as $11.CloseMyStoreRequest);
      case 'SaveKYCDraft':
        return saveKYCDraft(ctx, request as $11.SaveKYCDraftRequest);
      case 'GetMyKYC':
        return getMyKYC(ctx, request as $11.GetMyKYCRequest);
      case 'SubmitKYC':
        return submitKYC(ctx, request as $11.SubmitKYCRequest);
      case 'CancelKYC':
        return cancelKYC(ctx, request as $11.CancelKYCRequest);
      case 'GetMyPaymentAccount':
        return getMyPaymentAccount(
            ctx, request as $11.GetMyPaymentAccountRequest);
      case 'CreatePaymentOnboardingSession':
        return createPaymentOnboardingSession(
            ctx, request as $11.CreatePaymentOnboardingSessionRequest);
      case 'LookupBusinessRegistration':
        return lookupBusinessRegistration(
            ctx, request as $11.LookupBusinessRegistrationRequest);
      case 'ListMyMeasures':
        return listMyMeasures(ctx, request as $11.ListMyMeasuresRequest);
      case 'GetMyMeasure':
        return getMyMeasure(ctx, request as $11.GetMyMeasureRequest);
      case 'SubmitAppeal':
        return submitAppeal(ctx, request as $11.SubmitAppealRequest);
      case 'RequestRemediation':
        return requestRemediation(
            ctx, request as $11.RequestRemediationRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json => SellerAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerAppServiceBase$messageJson;
}

abstract class SellerPublicServiceBase extends $pb.GeneratedService {
  $async.Future<$11.GetStoreResponse> getStore(
      $pb.ServerContext ctx, $11.GetStoreRequest request);
  $async.Future<$11.ListStoresResponse> listStores(
      $pb.ServerContext ctx, $11.ListStoresRequest request);
  $async.Future<$11.BatchGetStoresResponse> batchGetStores(
      $pb.ServerContext ctx, $11.BatchGetStoresRequest request);
  $async.Future<$11.GetSellerAgreementsResponse> getSellerAgreements(
      $pb.ServerContext ctx, $11.GetSellerAgreementsRequest request);
  $async.Future<$11.RecordProductViewResponse> recordProductView(
      $pb.ServerContext ctx, $11.RecordProductViewRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'GetStore':
        return $11.GetStoreRequest();
      case 'ListStores':
        return $11.ListStoresRequest();
      case 'BatchGetStores':
        return $11.BatchGetStoresRequest();
      case 'GetSellerAgreements':
        return $11.GetSellerAgreementsRequest();
      case 'RecordProductView':
        return $11.RecordProductViewRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'GetStore':
        return getStore(ctx, request as $11.GetStoreRequest);
      case 'ListStores':
        return listStores(ctx, request as $11.ListStoresRequest);
      case 'BatchGetStores':
        return batchGetStores(ctx, request as $11.BatchGetStoresRequest);
      case 'GetSellerAgreements':
        return getSellerAgreements(
            ctx, request as $11.GetSellerAgreementsRequest);
      case 'RecordProductView':
        return recordProductView(ctx, request as $11.RecordProductViewRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerPublicServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerPublicServiceBase$messageJson;
}
