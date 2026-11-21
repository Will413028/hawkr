// This is a generated file - do not edit.
//
// Generated from hawkr/procurement/v1alpha1/procurement.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'procurement.pb.dart' as $4;
import 'procurement.pbjson.dart';

export 'procurement.pb.dart';

abstract class BuyerProcurementAppServiceBase extends $pb.GeneratedService {
  $async.Future<$4.SubmitTurnResponse> submitTurn(
      $pb.ServerContext ctx, $4.SubmitTurnRequest request);
  $async.Future<$4.GetSnapshotResponse> getSnapshot(
      $pb.ServerContext ctx, $4.GetSnapshotRequest request);
  $async.Future<$4.StartCandidateSearchResponse> startCandidateSearch(
      $pb.ServerContext ctx, $4.StartCandidateSearchRequest request);
  $async.Future<$4.GetCandidateSearchResponse> getCandidateSearch(
      $pb.ServerContext ctx, $4.GetCandidateSearchRequest request);
  $async.Future<$4.ToggleSelectionResponse> toggleSelection(
      $pb.ServerContext ctx, $4.ToggleSelectionRequest request);
  $async.Future<$4.SubmitAnswerResponse> submitAnswer(
      $pb.ServerContext ctx, $4.SubmitAnswerRequest request);
  $async.Future<$4.RewindResponse> rewind(
      $pb.ServerContext ctx, $4.RewindRequest request);
  $async.Future<$4.PreviewRfqPriceResponse> previewRfqPrice(
      $pb.ServerContext ctx, $4.PreviewRfqPriceRequest request);
  $async.Future<$4.ConfirmRfqSnapshotResponse> confirmRfqSnapshot(
      $pb.ServerContext ctx, $4.ConfirmRfqSnapshotRequest request);
  $async.Future<$4.PreviewRfqPublicationResponse> previewRfqPublication(
      $pb.ServerContext ctx, $4.PreviewRfqPublicationRequest request);
  $async.Future<$4.ExecuteRfqPublicationResponse> executeRfqPublication(
      $pb.ServerContext ctx, $4.ExecuteRfqPublicationRequest request);
  $async.Future<$4.ListConversationsResponse> listConversations(
      $pb.ServerContext ctx, $4.ListConversationsRequest request);
  $async.Future<$4.GetConversationMessagesResponse> getConversationMessages(
      $pb.ServerContext ctx, $4.GetConversationMessagesRequest request);
  $async.Future<$4.GetRuntimeSnapshotResponse> getRuntimeSnapshot(
      $pb.ServerContext ctx, $4.GetRuntimeSnapshotRequest request);
  $async.Future<$4.ListSellerResponsesResponse> listSellerResponses(
      $pb.ServerContext ctx, $4.ListSellerResponsesRequest request);
  $async.Future<$4.PutShortlistEntryResponse> putShortlistEntry(
      $pb.ServerContext ctx, $4.PutShortlistEntryRequest request);
  $async.Future<$4.DeleteShortlistEntryResponse> deleteShortlistEntry(
      $pb.ServerContext ctx, $4.DeleteShortlistEntryRequest request);
  $async.Future<$4.AcceptSellerQuoteResponse> acceptSellerQuote(
      $pb.ServerContext ctx, $4.AcceptSellerQuoteRequest request);
  $async.Future<$4.RenameConversationResponse> renameConversation(
      $pb.ServerContext ctx, $4.RenameConversationRequest request);
  $async.Future<$4.DeleteConversationResponse> deleteConversation(
      $pb.ServerContext ctx, $4.DeleteConversationRequest request);
  $async.Future<$4.WatchRunResponse> watchRun(
      $pb.ServerContext ctx, $4.WatchRunRequest request);
  $async.Future<$4.CancelRunResponse> cancelRun(
      $pb.ServerContext ctx, $4.CancelRunRequest request);
  $async.Future<$4.ResumeRunResponse> resumeRun(
      $pb.ServerContext ctx, $4.ResumeRunRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SubmitTurn':
        return $4.SubmitTurnRequest();
      case 'GetSnapshot':
        return $4.GetSnapshotRequest();
      case 'StartCandidateSearch':
        return $4.StartCandidateSearchRequest();
      case 'GetCandidateSearch':
        return $4.GetCandidateSearchRequest();
      case 'ToggleSelection':
        return $4.ToggleSelectionRequest();
      case 'SubmitAnswer':
        return $4.SubmitAnswerRequest();
      case 'Rewind':
        return $4.RewindRequest();
      case 'PreviewRfqPrice':
        return $4.PreviewRfqPriceRequest();
      case 'ConfirmRfqSnapshot':
        return $4.ConfirmRfqSnapshotRequest();
      case 'PreviewRfqPublication':
        return $4.PreviewRfqPublicationRequest();
      case 'ExecuteRfqPublication':
        return $4.ExecuteRfqPublicationRequest();
      case 'ListConversations':
        return $4.ListConversationsRequest();
      case 'GetConversationMessages':
        return $4.GetConversationMessagesRequest();
      case 'GetRuntimeSnapshot':
        return $4.GetRuntimeSnapshotRequest();
      case 'ListSellerResponses':
        return $4.ListSellerResponsesRequest();
      case 'PutShortlistEntry':
        return $4.PutShortlistEntryRequest();
      case 'DeleteShortlistEntry':
        return $4.DeleteShortlistEntryRequest();
      case 'AcceptSellerQuote':
        return $4.AcceptSellerQuoteRequest();
      case 'RenameConversation':
        return $4.RenameConversationRequest();
      case 'DeleteConversation':
        return $4.DeleteConversationRequest();
      case 'WatchRun':
        return $4.WatchRunRequest();
      case 'CancelRun':
        return $4.CancelRunRequest();
      case 'ResumeRun':
        return $4.ResumeRunRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SubmitTurn':
        return submitTurn(ctx, request as $4.SubmitTurnRequest);
      case 'GetSnapshot':
        return getSnapshot(ctx, request as $4.GetSnapshotRequest);
      case 'StartCandidateSearch':
        return startCandidateSearch(
            ctx, request as $4.StartCandidateSearchRequest);
      case 'GetCandidateSearch':
        return getCandidateSearch(ctx, request as $4.GetCandidateSearchRequest);
      case 'ToggleSelection':
        return toggleSelection(ctx, request as $4.ToggleSelectionRequest);
      case 'SubmitAnswer':
        return submitAnswer(ctx, request as $4.SubmitAnswerRequest);
      case 'Rewind':
        return rewind(ctx, request as $4.RewindRequest);
      case 'PreviewRfqPrice':
        return previewRfqPrice(ctx, request as $4.PreviewRfqPriceRequest);
      case 'ConfirmRfqSnapshot':
        return confirmRfqSnapshot(ctx, request as $4.ConfirmRfqSnapshotRequest);
      case 'PreviewRfqPublication':
        return previewRfqPublication(
            ctx, request as $4.PreviewRfqPublicationRequest);
      case 'ExecuteRfqPublication':
        return executeRfqPublication(
            ctx, request as $4.ExecuteRfqPublicationRequest);
      case 'ListConversations':
        return listConversations(ctx, request as $4.ListConversationsRequest);
      case 'GetConversationMessages':
        return getConversationMessages(
            ctx, request as $4.GetConversationMessagesRequest);
      case 'GetRuntimeSnapshot':
        return getRuntimeSnapshot(ctx, request as $4.GetRuntimeSnapshotRequest);
      case 'ListSellerResponses':
        return listSellerResponses(
            ctx, request as $4.ListSellerResponsesRequest);
      case 'PutShortlistEntry':
        return putShortlistEntry(ctx, request as $4.PutShortlistEntryRequest);
      case 'DeleteShortlistEntry':
        return deleteShortlistEntry(
            ctx, request as $4.DeleteShortlistEntryRequest);
      case 'AcceptSellerQuote':
        return acceptSellerQuote(ctx, request as $4.AcceptSellerQuoteRequest);
      case 'RenameConversation':
        return renameConversation(ctx, request as $4.RenameConversationRequest);
      case 'DeleteConversation':
        return deleteConversation(ctx, request as $4.DeleteConversationRequest);
      case 'WatchRun':
        return watchRun(ctx, request as $4.WatchRunRequest);
      case 'CancelRun':
        return cancelRun(ctx, request as $4.CancelRunRequest);
      case 'ResumeRun':
        return resumeRun(ctx, request as $4.ResumeRunRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      BuyerProcurementAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => BuyerProcurementAppServiceBase$messageJson;
}

abstract class SellerProcurementAppServiceBase extends $pb.GeneratedService {
  $async.Future<$4.ListSellerPublicationsResponse> listSellerPublications(
      $pb.ServerContext ctx, $4.ListSellerPublicationsRequest request);
  $async.Future<$4.GetSellerPublicationResponse> getSellerPublication(
      $pb.ServerContext ctx, $4.GetSellerPublicationRequest request);
  $async.Future<$4.UpsertSellerResponseDraftResponse> upsertSellerResponseDraft(
      $pb.ServerContext ctx, $4.UpsertSellerResponseDraftRequest request);
  $async.Future<$4.SubmitSellerResponseResponse> submitSellerResponse(
      $pb.ServerContext ctx, $4.SubmitSellerResponseRequest request);
  $async.Future<$4.WithdrawSellerResponseResponse> withdrawSellerResponse(
      $pb.ServerContext ctx, $4.WithdrawSellerResponseRequest request);
  $async.Future<$4.DeclineRfqPublicationResponse> declineRfqPublication(
      $pb.ServerContext ctx, $4.DeclineRfqPublicationRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListSellerPublications':
        return $4.ListSellerPublicationsRequest();
      case 'GetSellerPublication':
        return $4.GetSellerPublicationRequest();
      case 'UpsertSellerResponseDraft':
        return $4.UpsertSellerResponseDraftRequest();
      case 'SubmitSellerResponse':
        return $4.SubmitSellerResponseRequest();
      case 'WithdrawSellerResponse':
        return $4.WithdrawSellerResponseRequest();
      case 'DeclineRfqPublication':
        return $4.DeclineRfqPublicationRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListSellerPublications':
        return listSellerPublications(
            ctx, request as $4.ListSellerPublicationsRequest);
      case 'GetSellerPublication':
        return getSellerPublication(
            ctx, request as $4.GetSellerPublicationRequest);
      case 'UpsertSellerResponseDraft':
        return upsertSellerResponseDraft(
            ctx, request as $4.UpsertSellerResponseDraftRequest);
      case 'SubmitSellerResponse':
        return submitSellerResponse(
            ctx, request as $4.SubmitSellerResponseRequest);
      case 'WithdrawSellerResponse':
        return withdrawSellerResponse(
            ctx, request as $4.WithdrawSellerResponseRequest);
      case 'DeclineRfqPublication':
        return declineRfqPublication(
            ctx, request as $4.DeclineRfqPublicationRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerProcurementAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerProcurementAppServiceBase$messageJson;
}
