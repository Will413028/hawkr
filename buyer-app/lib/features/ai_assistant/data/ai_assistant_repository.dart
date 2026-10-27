import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/agui_sse.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/amount_utils.dart';
import 'ai_message_model.dart';

part 'ai_assistant_repository.g.dart';

/// App 內部語意事件；[type] 例：text / done / error / tool_result /
/// draft_patch / step_detail，controller 靠字串比對分派內容。
///
/// [terminal] 只在事件源自 AG-UI `RUN_FINISHED` / `RUN_ERROR`（見
/// `AgUiEvent.terminal`）時為 true。理由跟 `AgUiEvent.terminal` 一樣：
/// [type] 對 tool_result／CUSTOM 事件而言是後端可控的自由字串，哪天有個
/// 工具或 CUSTOM 事件剛好叫 `done`／`error`，靠 `type == 'done' || type
/// == 'error'` 判斷串流是否該終止就會誤判、把工具結果誤當控制流訊號，
/// 呼叫端（controller）的迴圈會被提前 break，使用者看到空氣泡——這正是
/// AG-UI SSE 改依 JSON type 分派要根除的失敗形狀，不可在這一層重新引入。
/// 判斷是否終止一律看 [terminal]，[type] 只用來決定語意分支（append 文字
/// / 存 tool 結果 / 顯示錯誤 / 存 sessionId）。
class AiSseEvent {
  const AiSseEvent({
    required this.type,
    required this.data,
    this.terminal = false,
  });
  final String type;
  final String data;
  final bool terminal;
}

/// 買家 AI 小幫手後端 endpoint（issue #447）
///
/// 注意：buyer Dio baseUrl 已包含 `/api/v1`，路徑不需重複前綴。
enum BuyerAssistantEndpoint {
  chat('/app/buyers/me/assistant/chat'),
  orders('/app/buyers/me/assistant/orders'),
  tracking('/app/buyers/me/assistant/tracking'),
  returnPolicy('/app/buyers/me/assistant/return-policy'),
  seasonal('/app/buyers/me/assistant/seasonal');

  const BuyerAssistantEndpoint(this.path);
  final String path;
}

class AiAssistantRepository {
  AiAssistantRepository(this._dio);
  final Dio _dio;

  Stream<AiSseEvent> streamChat({
    BuyerAssistantEndpoint endpoint = BuyerAssistantEndpoint.chat,
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
    BuyerAssistantEndpoint endpoint,
    String? message,
    String? sessionId,
    String? imageUrl,
  ) async {
    try {
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

  /// 載入最新一段對話歷史（後端 24h TTL）。
  Future<({String sessionId, List<AiMessage> messages})>
  loadLatestSession() async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/buyers/me/assistant/sessions/latest',
        options: Options(validateStatus: (_) => true),
      );
      final body = resp.data ?? const {};
      if (body['code'] != 0) return (sessionId: '', messages: <AiMessage>[]);
      final data = body['data'] as Map<String, dynamic>? ?? const {};
      final sid = (data['sessionId'] ?? '').toString();
      final raw = (data['messages'] as List?) ?? const [];
      final msgs = raw.whereType<Map>().map((m) {
        final role = (m['role'] ?? '').toString();
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

List<AiOrderItem> parseOrderListResult(String jsonStr) {
  try {
    final map = jsonDecode(jsonStr) as Map<String, dynamic>;
    final orders = map['orders'] as List<dynamic>? ?? [];
    return orders.map((o) {
      final j = o as Map<String, dynamic>;
      return AiOrderItem(
        orderId: j['order_no'] as String? ?? '',
        status: j['status'] as String? ?? '',
        // B-109：後端金額/時間戳一律字串，用 AmountUtils 做容錯
        amountTwd: AmountUtils.parseToInt(j['amount_twd']),
        createTimeMs: AmountUtils.parseToInt(j['create_time_ms']),
      );
    }).toList();
  } catch (_) {
    return [];
  }
}

@riverpod
AiAssistantRepository aiAssistantRepository(Ref ref) =>
    AiAssistantRepository(ref.watch(dioProvider));
