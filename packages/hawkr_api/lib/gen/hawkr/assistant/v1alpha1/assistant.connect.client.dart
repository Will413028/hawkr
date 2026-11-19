//
//  Generated code. Do not modify.
//  source: hawkr/assistant/v1alpha1/assistant.proto
//

import "package:connectrpc/connect.dart" as connect;
import "assistant.pb.dart" as hawkrassistantv1alpha1assistant;
import "assistant.connect.spec.dart" as specs;

/// AssistantAppService 是登入的買家的助理。身分取自 session，帳號沒有買家角色時回 permission_denied。
extension type AssistantAppServiceClient (connect.Transport _transport) {
  /// SendAssistantMessage 送出一則訊息，開始一輪助理回覆。伺服器受理後立刻回輪次 ID，回覆內容從
  /// WatchAssistantTurn 取得。session_id 空字串表示開新的對話。
  Future<hawkrassistantv1alpha1assistant.SendAssistantMessageResponse> sendAssistantMessage(
    hawkrassistantv1alpha1assistant.SendAssistantMessageRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.AssistantAppService.sendAssistantMessage,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// WatchAssistantTurn 訂閱一輪的事件，從 cursor 之後開始（空字串表示從頭）。只讀；伺服器在這一輪結束
  /// （finished 或 error 事件之後）或滿 100 秒時結束串流。
  Stream<hawkrassistantv1alpha1assistant.WatchAssistantTurnResponse> watchAssistantTurn(
    hawkrassistantv1alpha1assistant.WatchAssistantTurnRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).server(
      specs.AssistantAppService.watchAssistantTurn,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetLatestAssistantSession 回傳呼叫者最近一次的對話與訊息；沒有對話時 session_id 是空字串。
  Future<hawkrassistantv1alpha1assistant.GetLatestAssistantSessionResponse> getLatestAssistantSession(
    hawkrassistantv1alpha1assistant.GetLatestAssistantSessionRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.AssistantAppService.getLatestAssistantSession,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
