import 'dart:async';
import 'dart:convert';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/ai_assistant_repository.dart';
import '../data/ai_message_model.dart';

part 'ai_assistant_controller.g.dart';

@riverpod
class AiAssistantController extends _$AiAssistantController {
  @override
  List<AiMessage> build() {
    ref.onDispose(() => _events?.cancel());
    return [];
  }

  StreamIterator<AiSseEvent>? _events;

  bool _streaming = false;
  bool get isStreaming => _streaming;

  String? _sessionId;
  String? get sessionId => _sessionId;

  /// Load the latest persisted session from backend (24h TTL).
  /// Should be called once when the screen opens.
  Future<void> loadLatestSession() async {
    if (state.isNotEmpty) return; // already loaded / fresh session
    final result = await ref
        .read(aiAssistantRepositoryProvider)
        .loadLatestSession();
    if (!ref.mounted || state.isNotEmpty || _streaming) return;
    if (result.sessionId.isNotEmpty) {
      _sessionId = result.sessionId;
    }
    if (result.messages.isNotEmpty) {
      state = result.messages;
    }
  }

  /// Send a free-text message via the chat endpoint.
  Future<void> sendMessage(String text) async {
    return _dispatch(
      endpoint: SellerAssistantEndpoint.chat,
      message: text,
      userBubbleText: text,
    );
  }

  /// Trigger a quick-action button (no user-typed message).
  /// Adds a user bubble using [defaultMessage] for context, then streams reply.
  Future<void> sendQuickAction({
    required SellerAssistantEndpoint endpoint,
    required String defaultMessage,
  }) async {
    return _dispatch(
      endpoint: endpoint,
      message: null, // backend uses default for button endpoints
      userBubbleText: defaultMessage,
    );
  }

  Future<void> _dispatch({
    required SellerAssistantEndpoint endpoint,
    required String? message,
    required String userBubbleText,
  }) async {
    if (_streaming) return;
    if (userBubbleText.trim().isEmpty) return;

    // user bubble
    state = [
      ...state,
      AiMessage(
        role: 'user',
        content: userBubbleText.trim(),
        timestamp: DateTime.now(),
      ),
      // placeholder for assistant response
      AiMessage(role: 'assistant', content: '', timestamp: DateTime.now()),
    ];

    _streaming = true;

    try {
      final stream = ref
          .read(aiAssistantRepositoryProvider)
          .streamChat(
            endpoint: endpoint,
            message: message,
            sessionId: _sessionId,
          );

      // 判斷串流是否該停止一律看 event.terminal，不是 event.type=='done'/
      // 'error'：type 對 tool_result 事件而言可能是後端可控的自由字串
      // （例如剛好有個工具叫 done），字串比對會把它誤判成控制流訊號，讓
      // 後面真正的 text/done 事件永遠等不到——這正是本輪要修的殘留缺陷。
      final events = _events = StreamIterator(stream);
      while (await events.moveNext()) {
        if (!ref.mounted) return;
        final event = events.current;
        if (event.terminal) {
          if (event.type == 'error') {
            final msg = _parseErrorMessage(event.data);
            _appendToLast('\n\n$msg');
          } else {
            // done event payload 第一次呼叫時會帶 sessionId
            // （ai-service seller_assistant.go / Gateway ai.go fallback 皆已 camel）。
            try {
              final m = jsonDecode(event.data) as Map<String, dynamic>;
              final sid = (m['sessionId'] ?? '').toString();
              if (sid.isNotEmpty) _sessionId = sid;
            } catch (_) {}
          }
          break;
        } else if (event.type == 'text') {
          _appendToLast(event.data);
        } else if (event.type == 'tool_result') {
          final orders = parseOrderListResult(event.data);
          if (orders.isNotEmpty) _setOrderListOnLast(orders);
        }
      }
    } catch (_) {
      if (ref.mounted) _appendToLast('\n\n（無法連接 AI 服務，請稍後再試）');
    } finally {
      await _events?.cancel();
      _streaming = false;
    }
  }

  void _appendToLast(String chunk) {
    if (state.isEmpty) return;
    final last = state.last;
    final updated = AiMessage(
      role: last.role,
      content: last.content + chunk,
      timestamp: last.timestamp,
      orderList: last.orderList,
    );
    state = [...state.sublist(0, state.length - 1), updated];
  }

  String _parseErrorMessage(String data) {
    try {
      final m = jsonDecode(data);
      if (m is Map<String, dynamic>) {
        final msg = (m['message'] ?? '').toString();
        if (msg.isNotEmpty) return msg;
      }
    } catch (_) {}
    return '系統忙碌中，請稍後再試';
  }

  void _setOrderListOnLast(List<AiOrderItem> orders) {
    if (state.isEmpty) return;
    final last = state.last;
    final updated = AiMessage(
      role: last.role,
      content: last.content,
      timestamp: last.timestamp,
      orderList: orders,
    );
    state = [...state.sublist(0, state.length - 1), updated];
  }

  /// 加入系統/AI 訊息（如歡迎訊息）
  void addSystemMessage(String content) {
    state = [
      ...state,
      AiMessage(role: 'assistant', content: content, timestamp: DateTime.now()),
    ];
  }

  void clearMessages() {
    state = [];
    _sessionId = null;
  }
}
