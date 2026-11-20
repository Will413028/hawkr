//
//  Generated code. Do not modify.
//  source: hawkr/assistant/v1alpha1/assistant.proto
//

import "package:connectrpc/connect.dart" as connect;
import "assistant.pb.dart" as hawkrassistantv1alpha1assistant;

/// AssistantAppService 是登入的買家的助理。身分取自 session，帳號沒有買家角色時回 permission_denied。
abstract final class AssistantAppService {
  /// Fully-qualified name of the AssistantAppService service.
  static const name = 'hawkr.assistant.v1alpha1.AssistantAppService';

  /// SendAssistantMessage 送出一則訊息，開始一輪助理回覆。伺服器受理後立刻回輪次 ID，回覆內容從
  /// WatchAssistantTurn 取得。session_id 空字串表示開新的對話。
  /// 每位買家同時進行中的輪次（跨對話）最多 2 輪、每分鐘最多送出 10 次，超過回 resource_exhausted，
  /// 沒有 reason，帶 RetryInfo；以同一個 client_request_id 重送已受理的請求不計入上限。
  static const sendAssistantMessage = connect.Spec(
    '/$name/SendAssistantMessage',
    connect.StreamType.unary,
    hawkrassistantv1alpha1assistant.SendAssistantMessageRequest.new,
    hawkrassistantv1alpha1assistant.SendAssistantMessageResponse.new,
  );

  /// WatchAssistantTurn 訂閱一輪的事件，從 cursor 之後開始（空字串表示從頭）。只讀；伺服器在這一輪結束
  /// （finished 或 error 事件之後）或滿 100 秒時結束串流。
  static const watchAssistantTurn = connect.Spec(
    '/$name/WatchAssistantTurn',
    connect.StreamType.server,
    hawkrassistantv1alpha1assistant.WatchAssistantTurnRequest.new,
    hawkrassistantv1alpha1assistant.WatchAssistantTurnResponse.new,
  );

  /// GetLatestAssistantSession 回傳呼叫者最近一次的對話與最近 100 則訊息；沒有對話時 session_id 是空字串。
  static const getLatestAssistantSession = connect.Spec(
    '/$name/GetLatestAssistantSession',
    connect.StreamType.unary,
    hawkrassistantv1alpha1assistant.GetLatestAssistantSessionRequest.new,
    hawkrassistantv1alpha1assistant.GetLatestAssistantSessionResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
