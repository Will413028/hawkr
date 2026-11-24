import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_buyer/features/ai_assistant/presentation/ai_assistant_controller.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/default_error_text.dart';

class _Repository extends AiAssistantRepository {
  _Repository(this.script)
    : super(
        AssistantAppServiceClient(FakeTransportBuilder().build()),
        defaults: zhDefaultErrorText,
      );

  final Stream<AiTurnEvent> Function() script;

  @override
  Future<({String sessionId, List<AiMessage> messages})>
  loadLatestSession() async => (sessionId: '', messages: <AiMessage>[]);

  @override
  Stream<AiTurnEvent> streamChat({
    required AssistantTopic topic,
    required String message,
    String? sessionId,
  }) => script();
}

Future<List<AiMessage>> _run(Stream<AiTurnEvent> Function() script) async {
  final container = ProviderContainer(
    overrides: [
      aiAssistantRepositoryProvider.overrideWithValue(_Repository(script)),
    ],
  );
  addTearDown(container.dispose);
  final notifier = container.read(aiAssistantControllerProvider.notifier);
  await notifier.sendMessage('幫我查訂單');
  expect(notifier.isStreaming, isFalse);
  return container.read(aiAssistantControllerProvider);
}

void main() {
  test('串流上的 error 事件把伺服器文字顯示在對話泡泡並結束', () async {
    final messages = await _run(
      () => Stream.value(const AiTurnFailed('系統忙碌中，請稍後再試')),
    );
    expect(messages.last.role, 'assistant');
    expect(messages.last.content, contains('系統忙碌中，請稍後再試'));
  });

  test('連線錯誤顯示該 code 的預設文字，不是寫死的句子', () async {
    final messages = await _run(
      () => Stream.error(ConnectException(Code.unavailable, 'down')),
    );
    expect(
      messages.last.content,
      contains(zhDefaultErrorText(Code.unavailable)),
    );
  });

  test('tool_result 不改變氣泡，後面的文字照常顯示', () async {
    final messages = await _run(
      () => Stream.fromIterable(const [
        AiToolResult(
          AssistantTool.ASSISTANT_TOOL_GET_MY_ORDERS,
          succeeded: true,
        ),
        AiTextDelta('看得到我'),
        AiTurnFinished('s1'),
      ]),
    );
    expect(messages.last.content, '看得到我');
  });
}
