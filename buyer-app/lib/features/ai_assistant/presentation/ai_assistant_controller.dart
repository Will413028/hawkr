import 'dart:async';
import 'dart:convert';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/ai_assistant_repository.dart';
import '../data/ai_message_model.dart';

part 'ai_assistant_controller.g.dart';

@riverpod
class AiAssistantController extends _$AiAssistantController {
  @override
  List<AiMessage> build() => [];

  bool _streaming = false;
  bool get isStreaming => _streaming;

  String? _sessionId;
  String? get sessionId => _sessionId;

  Future<void> loadLatestSession() async {
    if (state.isNotEmpty) return;
    final result = await ref
        .read(aiAssistantRepositoryProvider)
        .loadLatestSession();
    if (!ref.mounted || state.isNotEmpty) return;
    if (result.sessionId.isNotEmpty) {
      _sessionId = result.sessionId;
    }
    if (result.messages.isNotEmpty) {
      state = result.messages;
    }
  }

  Future<void> sendMessage(String text) async {
    return _dispatch(
      endpoint: BuyerAssistantEndpoint.chat,
      message: text,
      userBubbleText: text,
    );
  }

  Future<void> sendQuickAction({
    required BuyerAssistantEndpoint endpoint,
    required String defaultMessage,
  }) async {
    return _dispatch(
      endpoint: endpoint,
      message: null,
      userBubbleText: defaultMessage,
    );
  }

  Future<void> _dispatch({
    required BuyerAssistantEndpoint endpoint,
    required String? message,
    required String userBubbleText,
  }) async {
    if (_streaming) return;
    if (userBubbleText.trim().isEmpty) return;

    state = [
      ...state,
      AiMessage(
        role: 'user',
        content: userBubbleText.trim(),
        timestamp: DateTime.now(),
      ),
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

      final events = StreamIterator(stream);
      final removeDisposeListener = ref.onDispose(
        () => unawaited(events.cancel()),
      );
      try {
        while (await events.moveNext()) {
          if (!ref.mounted) return;
          final event = events.current;
          // 終止判斷一律看 event.terminal，不看 event.type——type 對
          // tool_result/CUSTOM 事件而言是後端可控的自由字串，哪天有個工具
          // 剛好叫 done/error，靠字串比對就會被誤判成提前終止（見
          // AiSseEvent.terminal 的說明）。語意分支（要不要 append 文字、
          // 存 sessionId、顯示錯誤）仍然看 event.type，一字不改。
          if (event.terminal) {
            if (event.type == 'error') {
              _appendToLast('\n\n（${_parseErrorMessage(event.data)}）');
            } else {
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
      } finally {
        removeDisposeListener();
        await events.cancel();
      }
    } catch (_) {
      if (ref.mounted) _appendToLast('\n\n（無法連接 AI 服務，請稍後再試）');
    } finally {
      _streaming = false;
    }
  }

  void _appendToLast(String chunk) {
    if (state.isEmpty) return;
    final last = state.last;
    state = [
      ...state.sublist(0, state.length - 1),
      AiMessage(
        role: last.role,
        content: last.content + chunk,
        timestamp: last.timestamp,
        orderList: last.orderList,
      ),
    ];
  }

  String _parseErrorMessage(String data) {
    try {
      final m = jsonDecode(data) as Map<String, dynamic>;
      final msg = (m['message'] ?? '').toString();
      if (msg.isNotEmpty) return msg;
    } catch (_) {}
    return '系統忙碌中，請稍後再試';
  }

  void _setOrderListOnLast(List<AiOrderItem> orders) {
    if (state.isEmpty) return;
    final last = state.last;
    state = [
      ...state.sublist(0, state.length - 1),
      AiMessage(
        role: last.role,
        content: last.content,
        timestamp: last.timestamp,
        orderList: orders,
      ),
    ];
  }

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
