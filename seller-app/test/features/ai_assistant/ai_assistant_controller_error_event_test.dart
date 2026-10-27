import 'package:hawkr_seller/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_seller/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_seller/features/ai_assistant/presentation/ai_assistant_controller.dart';
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
    await notifier.sendMessage('查詢本週收款狀況');

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
    SellerAssistantEndpoint endpoint = SellerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    // terminal: true——這個 fake 代表真正的 RUN_ERROR（見
    // ai_assistant_controller.dart 新的終止判斷：只有 terminal 為 true
    // 時才會顯示錯誤訊息並結束串流，避免同名的 tool_result/CUSTOM 事件
    // 被誤判成控制流訊號）。
    yield const AiSseEvent(
      type: 'error',
      data: '{"code":500,"message":"系統忙碌中，請稍後再試"}',
      terminal: true,
    );
  }
}
