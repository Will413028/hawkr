import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_buyer/features/ai_assistant/presentation/ai_assistant_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('收到 error event 時把 message 顯示在對話泡泡並結束串流', () async {
    final repository = _FakeErrorRepository();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);

    final notifier = container.read(aiAssistantControllerProvider.notifier);
    await notifier.sendMessage('幫我查訂單');

    final messages = container.read(aiAssistantControllerProvider);
    expect(messages.last.role, 'assistant');
    expect(messages.last.content, contains('系統忙碌中，請稍後再試'));
    expect(notifier.isStreaming, isFalse);
  });
}

class _FakeErrorRepository extends AiAssistantRepository {
  _FakeErrorRepository() : super(Dio());

  @override
  Future<({String sessionId, List<AiMessage> messages})>
  loadLatestSession() async {
    return (sessionId: '', messages: <AiMessage>[]);
  }

  @override
  Stream<AiSseEvent> streamChat({
    BuyerAssistantEndpoint endpoint = BuyerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    // terminal: true——真實的 RUN_ERROR 事件一定是 terminal，這個 fake
    // 才能準確代表 repository 層的實際契約（見 AiSseEvent.terminal）。
    yield const AiSseEvent(
      type: 'error',
      data: '{"code":500,"message":"系統忙碌中，請稍後再試"}',
      terminal: true,
    );
  }
}
