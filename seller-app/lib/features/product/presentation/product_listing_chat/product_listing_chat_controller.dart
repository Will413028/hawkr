import 'dart:async';
import 'dart:convert';

import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../data/product_listing_model.dart';
import '../product_listing_draft_controller.dart';
import '../../data/product_listing_chat_repository.dart';

part 'product_listing_chat_controller.freezed.dart';
part 'product_listing_chat_controller.g.dart';

@freezed
abstract class ChatState with _$ChatState {
  const factory ChatState({
    @Default([]) List<ChatMessage> messages,
    String? sessionId,
    @Default(false) bool isStreaming,
    String? error,
  }) = _ChatState;
}

/// 對話式上架助手的狀態機：
/// - sendMessage：append user 訊息 → 建立空的 assistant 訊息 → 串流 SSE
/// - 收到 text event：把 delta 串接到最後一則 assistant 訊息
/// - 收到 draft_patch event：把 patch merge 到 [productListingDraftControllerProvider]
/// - 收到 done event：存下 sessionId，結束串流
@riverpod
class ProductListingChatController extends _$ProductListingChatController {
  StreamIterator<SseEvent>? _events;

  @override
  ChatState build() {
    ref.onDispose(() {
      _events?.cancel();
    });
    return const ChatState();
  }

  Future<void> sendMessage(String text) async {
    final trimmed = text.trim();
    if (trimmed.isEmpty || state.isStreaming) return;

    final messages = [
      ...state.messages,
      ChatMessage(role: 'user', text: trimmed),
      const ChatMessage(role: 'assistant', text: ''),
    ];
    state = state.copyWith(messages: messages, isStreaming: true, error: null);

    final repo = ref.read(productListingChatRepositoryProvider);
    final draftNotifier = ref.read(
      productListingDraftControllerProvider.notifier,
    );

    StreamIterator<SseEvent>? events;
    try {
      events = _events = StreamIterator(
        repo.stream(message: trimmed, sessionId: state.sessionId),
      );
      while (await events.moveNext()) {
        if (!ref.mounted || !identical(_events, events)) return;
        final event = events.current;
        // error/done 兩個 case 的控制流（要不要結束這次對話）一律看
        // event.terminal，不是 event.type=='error'/'done'：type 對
        // tool_result/CUSTOM 事件而言可能是後端可控的自由字串（例如剛好
        // 有個 CUSTOM 事件叫 error），字串比對會把它誤判成控制流訊號，把
        // 整段對話提前中斷，或把不相干的內容誤存成 sessionId。
        switch (event.type) {
          case 'text':
            _appendAssistantText(event.data);
          case 'draft_patch':
            try {
              final patch = json.decode(event.data);
              if (patch is Map<String, dynamic>) {
                draftNotifier.mergePatch(patch);
              }
            } catch (_) {
              // ignore malformed patch
            }
          case 'error':
            if (event.terminal) {
              state = state.copyWith(
                error: _parseStreamErrorMessage(event.data),
              );
            }
          case 'done':
            if (event.terminal) {
              try {
                final decoded = json.decode(event.data);
                if (decoded is Map<String, dynamic>) {
                  final sid = decoded['sessionId'] as String?;
                  if (sid != null && sid.isNotEmpty) {
                    state = state.copyWith(sessionId: sid);
                  }
                }
              } catch (_) {}
            }
        }
        if (event.terminal) return;
      }
    } catch (e) {
      if (ref.mounted && identical(_events, events)) {
        state = state.copyWith(error: _friendlyError(e));
      }
    } finally {
      await events?.cancel();
      if (ref.mounted && identical(_events, events)) {
        _events = null;
        state = state.copyWith(isStreaming: false);
      }
    }
  }

  void reset() {
    final events = _events;
    _events = null;
    events?.cancel();
    state = const ChatState();
  }

  void _appendAssistantText(String delta) {
    final messages = [...state.messages];
    if (messages.isEmpty) return;
    final last = messages.last;
    if (last.role != 'assistant') return;
    messages[messages.length - 1] = last.copyWith(text: last.text + delta);
    state = state.copyWith(messages: messages);
  }

  String _friendlyError(Object e) {
    final msg = e.toString();
    if (msg.contains('429') || msg.contains('503')) {
      return 'AI 服務忙線中，請稍後再試';
    }
    if (msg.contains('SocketException') || msg.contains('Connection')) {
      return '網路不穩，請檢查後重試';
    }
    return '連線中斷，請稍後再試';
  }

  String _parseStreamErrorMessage(String data) {
    try {
      final m = json.decode(data);
      if (m is Map<String, dynamic>) {
        final msg = (m['message'] ?? '').toString();
        if (msg.isNotEmpty) return msg;
      }
    } catch (_) {}
    return '系統忙碌中，請稍後再試';
  }
}
