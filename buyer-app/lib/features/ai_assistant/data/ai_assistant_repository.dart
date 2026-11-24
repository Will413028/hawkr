import 'dart:async';

import 'package:hawkr_api/errors.dart' show errorText, DefaultErrorText;
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import 'ai_message_model.dart';

part 'ai_assistant_repository.g.dart';

/// 一輪回覆串流上的事件（已轉成 App 的形狀）。[AiTurnFinished] 與 [AiTurnFailed] 是最後一個事件。
sealed class AiTurnEvent {
  const AiTurnEvent();
}

/// 回覆文字的增量。
final class AiTextDelta extends AiTurnEvent {
  const AiTextDelta(this.text);
  final String text;
}

/// 助理使用了工具；內容不對外，只有工具與成功與否。
final class AiToolResult extends AiTurnEvent {
  const AiToolResult(this.tool, {required this.succeeded});
  final AssistantTool tool;
  final bool succeeded;
}

/// 這一輪正常結束。
final class AiTurnFinished extends AiTurnEvent {
  const AiTurnFinished(this.sessionId);
  final String sessionId;
}

/// 這一輪失敗；[message] 是伺服器依語系組好的文字。
final class AiTurnFailed extends AiTurnEvent {
  const AiTurnFailed(this.message);
  final String message;
}

/// 買家的 AI 助理（`AssistantAppService`）。
///
/// 送出（unary，帶 `client_request_id`）與觀看（server streaming）分開：伺服器每條串流最長 100 秒就正常
/// 結束，沒有收到 finished 或 error 就是被截斷，以最後一個事件的 cursor 重新訂閱接續。cursor 是不透明的字串，
/// 接續之後伺服器不重送已送出的事件；仍以 cursor 去重，重送的事件只處理一次。
class AiAssistantRepository {
  AiAssistantRepository(
    this._client, {
    required DefaultErrorText defaults,
    Future<void> Function(Duration)? delay,
    String Function()? mint,
    this.maxConsecutiveFailures = 3,
  }) : _defaults = defaults,
       _delay = delay ?? Future<void>.delayed,
       _mint = mint ?? const Uuid().v4;

  final AssistantAppServiceClient _client;
  final DefaultErrorText _defaults;
  final Future<void> Function(Duration) _delay;
  final String Function() _mint;

  /// 連續幾次沒收到任何新事件就失敗（斷線、unavailable）；伺服器正常的 100 秒結束不計。
  final int maxConsecutiveFailures;

  /// 送出一則訊息並回傳這一輪的事件，直到 finished 或 error。連線錯誤以 [ConnectException] 丟出。
  Stream<AiTurnEvent> streamChat({
    required AssistantTopic topic,
    required String message,
    String? sessionId,
  }) async* {
    final sent = await _client.sendAssistantMessage(
      pb.SendAssistantMessageRequest(
        topic: topic,
        message: message,
        sessionId: sessionId ?? '',
        clientRequestId: _mint(),
      ),
    );
    yield* _watch(sent.turnId, sent.sessionId);
  }

  Stream<AiTurnEvent> _watch(String turnId, String sessionId) async* {
    var cursor = '';
    final seen = <String>{};
    var failures = 0;
    while (true) {
      var progressed = false;
      try {
        await for (final e in _client.watchAssistantTurn(
          pb.WatchAssistantTurnRequest(turnId: turnId, cursor: cursor),
        )) {
          if (e.cursor.isNotEmpty) {
            if (!seen.add(e.cursor)) continue;
            cursor = e.cursor;
          }
          progressed = true;
          final event = _convert(e, sessionId);
          if (event == null) continue;
          yield event;
          if (event is AiTurnFinished || event is AiTurnFailed) return;
        }
        // 被截斷（100 秒到）：立刻接續；一個事件都沒有時隔一秒，避免緊密迴圈。
        failures = progressed ? 0 : failures + 1;
        if (failures >= maxConsecutiveFailures) {
          throw ConnectException(Code.unavailable, 'assistant stream stalled');
        }
        if (!progressed) await _delay(const Duration(seconds: 1));
      } on ConnectException catch (e) {
        if (!_transient(e)) rethrow;
        failures = progressed ? 1 : failures + 1;
        if (failures >= maxConsecutiveFailures) rethrow;
        await _delay(Duration(seconds: failures));
      }
    }
  }

  static bool _transient(ConnectException e) =>
      e.code == Code.unavailable ||
      e.code == Code.unknown ||
      e.code == Code.deadlineExceeded;

  AiTurnEvent? _convert(pb.WatchAssistantTurnResponse e, String sessionId) {
    switch (e.whichEvent()) {
      case pb.WatchAssistantTurnResponse_Event.text:
        return AiTextDelta(e.text.text);
      case pb.WatchAssistantTurnResponse_Event.toolResult:
        return AiToolResult(
          e.toolResult.tool,
          succeeded: e.toolResult.succeeded,
        );
      case pb.WatchAssistantTurnResponse_Event.finished:
        final sid = e.finished.sessionId;
        return AiTurnFinished(sid.isNotEmpty ? sid : sessionId);
      case pb.WatchAssistantTurnResponse_Event.error:
        final m = e.error.localizedMessage;
        return AiTurnFailed(m.isNotEmpty ? m : _defaults(Code.internal));
      case pb.WatchAssistantTurnResponse_Event.notSet:
        return null;
    }
  }

  /// 載入最近一段對話歷史；沒有對話時 sessionId 是空字串。
  Future<({String sessionId, List<AiMessage> messages})>
  loadLatestSession() async {
    try {
      final r = await _client.getLatestAssistantSession(
        pb.GetLatestAssistantSessionRequest(),
      );
      return (
        sessionId: r.sessionId,
        messages: [
          for (final m in r.messages)
            AiMessage(
              role: m.role == AssistantMessageRole.ASSISTANT_MESSAGE_ROLE_USER
                  ? 'user'
                  : 'assistant',
              content: m.text,
              timestamp: m.hasCreateTime()
                  ? m.createTime.toDateTime()
                  : DateTime.now(),
            ),
        ],
      );
    } on ConnectException {
      return (sessionId: '', messages: <AiMessage>[]);
    }
  }

  /// 失敗時給使用者看的文字：伺服器的 LocalizedMessage，沒有時是該 code 的預設文字。
  String failureText(Object error) => errorText(error, defaults: _defaults);
}

@riverpod
AiAssistantRepository aiAssistantRepository(Ref ref) => AiAssistantRepository(
  ref.watch(assistantAppClientProvider),
  defaults: ref.watch(defaultErrorTextProvider),
);
