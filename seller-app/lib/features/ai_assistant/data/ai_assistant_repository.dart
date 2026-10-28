import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/agui_sse.dart';
import '../../../core/networking/dio_provider.dart';
import 'ai_message_model.dart';

part 'ai_assistant_repository.g.dart';

/// SSE event parsed from the backend stream.
///
/// [terminal] 表示這個事件是否代表串流真正結束——只有當底層
/// `AgUiEvent.terminal`（見 `core/networking/agui_sse.dart`）為 true 時才會
/// 是 true。[type] 對 `tool_result` 事件而言可能是後端可控的自由字串（見
/// `AgUiEvent.name` 的說明），呼叫端判斷串流是否該停止消費一律看
/// [terminal]，不要用 [type] 字串比對（例如 `type == 'done'`）——哪天有個
/// 工具或自訂事件剛好叫 done/error，字串比對會把它誤判成控制流訊號，串流
/// 被提前截斷。
class AiSseEvent {
  const AiSseEvent({
    required this.type,
    required this.data,
    this.terminal = false,
  });
  final String type; // 'text' | 'tool_result' | 'done' | 'error'
  final String data;
  final bool terminal;
}

/// Endpoint variants supported by the seller AI assistant (issue #447).
enum SellerAssistantEndpoint {
  chat('/api/v1/app/sellers/me/assistant/chat'),
  orders('/api/v1/app/sellers/me/assistant/orders'),
  inventory('/api/v1/app/sellers/me/assistant/inventory'),
  payment('/api/v1/app/sellers/me/assistant/payment');

  const SellerAssistantEndpoint(this.path);
  final String path;
}

class AiAssistantRepository {
  AiAssistantRepository(this._dio);
  final Dio _dio;

  /// Streams SSE events from the seller AI assistant endpoint.
  ///
  /// - [endpoint] picks one of the agent endpoints (chat / orders / inventory / payment / price)
  /// - [message] optional for the button endpoints, required for chat
  /// - [sessionId] omit on first call; pass back the value returned in `done` event
  Stream<AiSseEvent> streamChat({
    SellerAssistantEndpoint endpoint = SellerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) {
    final controller = StreamController<AiSseEvent>();
    _startStream(controller, endpoint, message, sessionId, imageUrl);
    return controller.stream;
  }

  Future<void> _startStream(
    StreamController<AiSseEvent> controller,
    SellerAssistantEndpoint endpoint,
    String? message,
    String? sessionId,
    String? imageUrl,
  ) async {
    try {
      // 2026-08-16: 契約遷移至 camelCase。Gateway 與 ai-service 皆已更新。
      final body = <String, dynamic>{
        if (sessionId != null && sessionId.isNotEmpty) 'sessionId': sessionId,
        if (message != null && message.isNotEmpty) 'message': message,
        if (imageUrl != null && imageUrl.isNotEmpty) 'imageUrl': imageUrl,
      };

      final response = await _dio.post<ResponseBody>(
        endpoint.path,
        data: body,
        options: Options(
          responseType: ResponseType.stream,
          receiveTimeout: const Duration(minutes: 3),
          headers: {'Accept': 'text/event-stream'},
          validateStatus: (_) => true,
        ),
      );

      if (response.statusCode != 200) {
        controller.addError(
          Exception('AI service error: ${response.statusCode}'),
        );
        await controller.close();
        return;
      }

      final stream = response.data!;

      await for (final event in decodeAgUiSse(stream.stream)) {
        controller.add(
          AiSseEvent(
            type: event.name,
            data: event.data,
            terminal: event.terminal,
          ),
        );
        // 判斷是否終止一律看 event.terminal，不看 event.name——name 對
        // TOOL_CALL_RESULT/CUSTOM 是後端可控的自由字串，字串比對 'done'/
        // 'error' 會被剛好同名的工具或自訂事件誤判成提前終止。
        if (event.terminal) {
          await controller.close();
          return;
        }
      }
    } catch (e) {
      controller.addError(e);
    } finally {
      if (!controller.isClosed) {
        await controller.close();
      }
    }
  }

  /// Load latest persisted session (24h TTL).
  /// Returns `(sessionId, messages)` — sessionId may be empty.
  Future<({String sessionId, List<AiMessage> messages})>
  loadLatestSession() async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/api/v1/app/sellers/me/assistant/sessions/latest',
        options: Options(validateStatus: (_) => true),
      );
      final body = resp.data ?? const {};
      if (body['code'] != 0) return (sessionId: '', messages: <AiMessage>[]);
      final data = body['data'] as Map<String, dynamic>? ?? const {};
      // REST `/assistant/sessions/latest` 已遷移至 camelCase。
      final sid = (data['sessionId'] ?? '').toString();
      final raw = (data['messages'] as List?) ?? const [];
      final msgs = raw.whereType<Map>().map((m) {
        final role = (m['role'] ?? '').toString();
        // Backend uses 'user' / 'model'; UI uses 'user' / 'assistant'
        final uiRole = role == 'model' ? 'assistant' : role;
        DateTime ts;
        try {
          ts = DateTime.parse((m['timestamp'] ?? '').toString()).toLocal();
        } catch (_) {
          ts = DateTime.now();
        }
        return AiMessage(
          role: uiRole,
          content: (m['text'] ?? '').toString(),
          timestamp: ts,
        );
      }).toList();
      return (sessionId: sid, messages: msgs);
    } catch (_) {
      return (sessionId: '', messages: <AiMessage>[]);
    }
  }
}

/// Parse a `tool_result` JSON payload into a list of [AiOrderItem].
List<AiOrderItem> parseOrderListResult(String jsonStr) {
  try {
    final map = jsonDecode(jsonStr) as Map<String, dynamic>;
    final orders = map['orders'] as List<dynamic>? ?? [];
    return orders.map((o) {
      final j = o as Map<String, dynamic>;
      return AiOrderItem(
        orderId: j['order_no'] as String? ?? '',
        status: j['status'] as String? ?? '',
        amountTwd: (j['amount_twd'] as num?)?.toInt() ?? 0,
        createTimeMs: (j['create_time_ms'] as num?)?.toInt() ?? 0,
        buyerName: j['buyer_name'] as String? ?? '',
        itemsSummary: j['items_summary'] as String? ?? '',
      );
    }).toList();
  } catch (_) {
    return [];
  }
}

@riverpod
AiAssistantRepository aiAssistantRepository(Ref ref) =>
    AiAssistantRepository(ref.watch(dioProvider));
