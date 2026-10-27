import 'dart:async';

import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_buyer/features/ai_assistant/presentation/ai_assistant_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _Repository extends AiAssistantRepository {
  _Repository() : super(Dio()) {
    events = StreamController<AiSseEvent>(onCancel: () => cancelled = true);
  }

  bool cancelled = false;
  late final StreamController<AiSseEvent> events;
  final history = Completer<({String sessionId, List<AiMessage> messages})>();

  @override
  Future<({String sessionId, List<AiMessage> messages})> loadLatestSession() =>
      history.future;

  @override
  Stream<AiSseEvent> streamChat({
    BuyerAssistantEndpoint endpoint = BuyerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) => events.stream;
}

void main() {
  test('卸載AI頁面立即取消沒有新事件的閒置串流', () async {
    final repository = _Repository();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repository)],
    );
    container.listen(aiAssistantControllerProvider, (_, __) {});
    final controller = container.read(aiAssistantControllerProvider.notifier);
    final sending = controller.sendMessage('等待中的訊息');
    expect(repository.events.hasListener, isTrue);
    container.dispose();
    await sending;
    expect(repository.cancelled, isTrue);
    expect(repository.events.hasListener, isFalse);
    expect(controller.isStreaming, isFalse);
    await repository.events.close();
  });

  test('歷史讀取晚於新送出，不覆蓋新訊息或目前session', () async {
    final repository = _Repository();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);
    container.listen(aiAssistantControllerProvider, (_, __) {});
    final controller = container.read(aiAssistantControllerProvider.notifier);
    final history = controller.loadLatestSession();
    final sending = controller.sendMessage('最新問題');
    repository.history.complete((
      sessionId: 'old-session',
      messages: [
        AiMessage(
          role: 'assistant',
          content: '舊對話',
          timestamp: DateTime(2026, 1, 1),
        ),
      ],
    ));
    await history;
    expect(controller.sessionId, isNull);
    expect(container.read(aiAssistantControllerProvider).first.content, '最新問題');
    repository.events.add(const AiSseEvent(type: 'text', data: '最新回覆'));
    repository.events.add(
      const AiSseEvent(
        type: 'done',
        data: '{"sessionId":"new-session"}',
        terminal: true,
      ),
    );
    await sending;
    expect(controller.sessionId, 'new-session');
    expect(
      container.read(aiAssistantControllerProvider).map((m) => m.content),
      ['最新問題', '最新回覆'],
    );
    await repository.events.close();
  });
}
