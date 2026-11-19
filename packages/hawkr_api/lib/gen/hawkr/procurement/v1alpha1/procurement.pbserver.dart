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

import 'procurement.pb.dart' as $5;
import 'procurement.pbjson.dart';

export 'procurement.pb.dart';

abstract class BuyerProcurementAppServiceBase extends $pb.GeneratedService {
  $async.Future<$5.SubmitTurnResponse> submitTurn(
      $pb.ServerContext ctx, $5.SubmitTurnRequest request);
  $async.Future<$5.GetSnapshotResponse> getSnapshot(
      $pb.ServerContext ctx, $5.GetSnapshotRequest request);
  $async.Future<$5.StartCandidateSearchResponse> startCandidateSearch(
      $pb.ServerContext ctx, $5.StartCandidateSearchRequest request);
  $async.Future<$5.GetCandidateSearchResponse> getCandidateSearch(
      $pb.ServerContext ctx, $5.GetCandidateSearchRequest request);
  $async.Future<$5.ToggleSelectionResponse> toggleSelection(
      $pb.ServerContext ctx, $5.ToggleSelectionRequest request);
  $async.Future<$5.SubmitAnswerResponse> submitAnswer(
      $pb.ServerContext ctx, $5.SubmitAnswerRequest request);
  $async.Future<$5.RewindResponse> rewind(
      $pb.ServerContext ctx, $5.RewindRequest request);
  $async.Future<$5.PreviewRfqPriceResponse> previewRfqPrice(
      $pb.ServerContext ctx, $5.PreviewRfqPriceRequest request);
  $async.Future<$5.ConfirmRfqSnapshotResponse> confirmRfqSnapshot(
      $pb.ServerContext ctx, $5.ConfirmRfqSnapshotRequest request);
  $async.Future<$5.PreviewRfqPublicationResponse> previewRfqPublication(
      $pb.ServerContext ctx, $5.PreviewRfqPublicationRequest request);
  $async.Future<$5.ExecuteRfqPublicationResponse> executeRfqPublication(
      $pb.ServerContext ctx, $5.ExecuteRfqPublicationRequest request);
  $async.Future<$5.ListConversationsResponse> listConversations(
      $pb.ServerContext ctx, $5.ListConversationsRequest request);
  $async.Future<$5.GetConversationMessagesResponse> getConversationMessages(
      $pb.ServerContext ctx, $5.GetConversationMessagesRequest request);
  $async.Future<$5.GetRuntimeSnapshotResponse> getRuntimeSnapshot(
      $pb.ServerContext ctx, $5.GetRuntimeSnapshotRequest request);
  $async.Future<$5.ListSellerResponsesResponse> listSellerResponses(
      $pb.ServerContext ctx, $5.ListSellerResponsesRequest request);
  $async.Future<$5.PutShortlistEntryResponse> putShortlistEntry(
      $pb.ServerContext ctx, $5.PutShortlistEntryRequest request);
  $async.Future<$5.DeleteShortlistEntryResponse> deleteShortlistEntry(
      $pb.ServerContext ctx, $5.DeleteShortlistEntryRequest request);
  $async.Future<$5.AcceptSellerQuoteResponse> acceptSellerQuote(
      $pb.ServerContext ctx, $5.AcceptSellerQuoteRequest request);
  $async.Future<$5.RenameConversationResponse> renameConversation(
      $pb.ServerContext ctx, $5.RenameConversationRequest request);
  $async.Future<$5.DeleteConversationResponse> deleteConversation(
      $pb.ServerContext ctx, $5.DeleteConversationRequest request);
  $async.Future<$5.WatchRunResponse> watchRun(
      $pb.ServerContext ctx, $5.WatchRunRequest request);
  $async.Future<$5.CancelRunResponse> cancelRun(
      $pb.ServerContext ctx, $5.CancelRunRequest request);
  $async.Future<$5.ResumeRunResponse> resumeRun(
      $pb.ServerContext ctx, $5.ResumeRunRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'SubmitTurn':
        return $5.SubmitTurnRequest();
      case 'GetSnapshot':
        return $5.GetSnapshotRequest();
      case 'StartCandidateSearch':
        return $5.StartCandidateSearchRequest();
      case 'GetCandidateSearch':
        return $5.GetCandidateSearchRequest();
      case 'ToggleSelection':
        return $5.ToggleSelectionRequest();
      case 'SubmitAnswer':
        return $5.SubmitAnswerRequest();
      case 'Rewind':
        return $5.RewindRequest();
      case 'PreviewRfqPrice':
        return $5.PreviewRfqPriceRequest();
      case 'ConfirmRfqSnapshot':
        return $5.ConfirmRfqSnapshotRequest();
      case 'PreviewRfqPublication':
        return $5.PreviewRfqPublicationRequest();
      case 'ExecuteRfqPublication':
        return $5.ExecuteRfqPublicationRequest();
      case 'ListConversations':
        return $5.ListConversationsRequest();
      case 'GetConversationMessages':
        return $5.GetConversationMessagesRequest();
      case 'GetRuntimeSnapshot':
        return $5.GetRuntimeSnapshotRequest();
      case 'ListSellerResponses':
        return $5.ListSellerResponsesRequest();
      case 'PutShortlistEntry':
        return $5.PutShortlistEntryRequest();
      case 'DeleteShortlistEntry':
        return $5.DeleteShortlistEntryRequest();
      case 'AcceptSellerQuote':
        return $5.AcceptSellerQuoteRequest();
      case 'RenameConversation':
        return $5.RenameConversationRequest();
      case 'DeleteConversation':
        return $5.DeleteConversationRequest();
      case 'WatchRun':
        return $5.WatchRunRequest();
      case 'CancelRun':
        return $5.CancelRunRequest();
      case 'ResumeRun':
        return $5.ResumeRunRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'SubmitTurn':
        return submitTurn(ctx, request as $5.SubmitTurnRequest);
      case 'GetSnapshot':
        return getSnapshot(ctx, request as $5.GetSnapshotRequest);
      case 'StartCandidateSearch':
        return startCandidateSearch(
            ctx, request as $5.StartCandidateSearchRequest);
      case 'GetCandidateSearch':
        return getCandidateSearch(ctx, request as $5.GetCandidateSearchRequest);
      case 'ToggleSelection':
        return toggleSelection(ctx, request as $5.ToggleSelectionRequest);
      case 'SubmitAnswer':
        return submitAnswer(ctx, request as $5.SubmitAnswerRequest);
      case 'Rewind':
        return rewind(ctx, request as $5.RewindRequest);
      case 'PreviewRfqPrice':
        return previewRfqPrice(ctx, request as $5.PreviewRfqPriceRequest);
      case 'ConfirmRfqSnapshot':
        return confirmRfqSnapshot(ctx, request as $5.ConfirmRfqSnapshotRequest);
      case 'PreviewRfqPublication':
        return previewRfqPublication(
            ctx, request as $5.PreviewRfqPublicationRequest);
      case 'ExecuteRfqPublication':
        return executeRfqPublication(
            ctx, request as $5.ExecuteRfqPublicationRequest);
      case 'ListConversations':
        return listConversations(ctx, request as $5.ListConversationsRequest);
      case 'GetConversationMessages':
        return getConversationMessages(
            ctx, request as $5.GetConversationMessagesRequest);
      case 'GetRuntimeSnapshot':
        return getRuntimeSnapshot(ctx, request as $5.GetRuntimeSnapshotRequest);
      case 'ListSellerResponses':
        return listSellerResponses(
            ctx, request as $5.ListSellerResponsesRequest);
      case 'PutShortlistEntry':
        return putShortlistEntry(ctx, request as $5.PutShortlistEntryRequest);
      case 'DeleteShortlistEntry':
        return deleteShortlistEntry(
            ctx, request as $5.DeleteShortlistEntryRequest);
      case 'AcceptSellerQuote':
        return acceptSellerQuote(ctx, request as $5.AcceptSellerQuoteRequest);
      case 'RenameConversation':
        return renameConversation(ctx, request as $5.RenameConversationRequest);
      case 'DeleteConversation':
        return deleteConversation(ctx, request as $5.DeleteConversationRequest);
      case 'WatchRun':
        return watchRun(ctx, request as $5.WatchRunRequest);
      case 'CancelRun':
        return cancelRun(ctx, request as $5.CancelRunRequest);
      case 'ResumeRun':
        return resumeRun(ctx, request as $5.ResumeRunRequest);
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
  $async.Future<$5.ListSellerPublicationsResponse> listSellerPublications(
      $pb.ServerContext ctx, $5.ListSellerPublicationsRequest request);
  $async.Future<$5.GetSellerPublicationResponse> getSellerPublication(
      $pb.ServerContext ctx, $5.GetSellerPublicationRequest request);
  $async.Future<$5.UpsertSellerResponseDraftResponse> upsertSellerResponseDraft(
      $pb.ServerContext ctx, $5.UpsertSellerResponseDraftRequest request);
  $async.Future<$5.SubmitSellerResponseResponse> submitSellerResponse(
      $pb.ServerContext ctx, $5.SubmitSellerResponseRequest request);
  $async.Future<$5.WithdrawSellerResponseResponse> withdrawSellerResponse(
      $pb.ServerContext ctx, $5.WithdrawSellerResponseRequest request);
  $async.Future<$5.DeclineRfqPublicationResponse> declineRfqPublication(
      $pb.ServerContext ctx, $5.DeclineRfqPublicationRequest request);

  $pb.GeneratedMessage createRequest($core.String methodName) {
    switch (methodName) {
      case 'ListSellerPublications':
        return $5.ListSellerPublicationsRequest();
      case 'GetSellerPublication':
        return $5.GetSellerPublicationRequest();
      case 'UpsertSellerResponseDraft':
        return $5.UpsertSellerResponseDraftRequest();
      case 'SubmitSellerResponse':
        return $5.SubmitSellerResponseRequest();
      case 'WithdrawSellerResponse':
        return $5.WithdrawSellerResponseRequest();
      case 'DeclineRfqPublication':
        return $5.DeclineRfqPublicationRequest();
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $async.Future<$pb.GeneratedMessage> handleCall($pb.ServerContext ctx,
      $core.String methodName, $pb.GeneratedMessage request) {
    switch (methodName) {
      case 'ListSellerPublications':
        return listSellerPublications(
            ctx, request as $5.ListSellerPublicationsRequest);
      case 'GetSellerPublication':
        return getSellerPublication(
            ctx, request as $5.GetSellerPublicationRequest);
      case 'UpsertSellerResponseDraft':
        return upsertSellerResponseDraft(
            ctx, request as $5.UpsertSellerResponseDraftRequest);
      case 'SubmitSellerResponse':
        return submitSellerResponse(
            ctx, request as $5.SubmitSellerResponseRequest);
      case 'WithdrawSellerResponse':
        return withdrawSellerResponse(
            ctx, request as $5.WithdrawSellerResponseRequest);
      case 'DeclineRfqPublication':
        return declineRfqPublication(
            ctx, request as $5.DeclineRfqPublicationRequest);
      default:
        throw $core.ArgumentError('Unknown method: $methodName');
    }
  }

  $core.Map<$core.String, $core.dynamic> get $json =>
      SellerProcurementAppServiceBase$json;
  $core.Map<$core.String, $core.Map<$core.String, $core.dynamic>>
      get $messageJson => SellerProcurementAppServiceBase$messageJson;
}
