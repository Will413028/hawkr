import 'dart:async';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pbenum.dart'
    show AssistantTopic;
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
    return _dispatch(topic: AssistantTopic.ASSISTANT_TOPIC_CHAT, text: text);
  }

  Future<void> sendQuickAction({
    required AssistantTopic topic,
    required String defaultMessage,
  }) async {
    return _dispatch(topic: topic, text: defaultMessage);
  }

  Future<void> _dispatch({
    required AssistantTopic topic,
    required String text,
  }) async {
    if (_streaming) return;
    final message = text.trim();
    if (message.isEmpty) return;

    state = [
      ...state,
      AiMessage(role: 'user', content: message, timestamp: DateTime.now()),
      AiMessage(role: 'assistant', content: '', timestamp: DateTime.now()),
    ];

    _streaming = true;
    final repository = ref.read(aiAssistantRepositoryProvider);
    try {
      final stream = repository.streamChat(
        topic: topic,
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
          switch (events.current) {
            case AiTextDelta(:final text):
              _appendToLast(text);
            case AiToolResult():
              break;
            case AiTurnFinished(:final sessionId):
              if (sessionId.isNotEmpty) _sessionId = sessionId;
            case AiTurnFailed(:final message):
              _appendToLast('\n\n（$message）');
          }
        }
      } finally {
        removeDisposeListener();
        await events.cancel();
      }
    } catch (error) {
      if (ref.mounted) {
        _appendToLast('\n\n（${repository.failureText(error)}）');
      }
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
