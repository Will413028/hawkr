/// controller 的終止判斷必須看 `AiSseEvent.terminal`，不能看 `event.type`。
///
/// `type` 對 tool_result/CUSTOM 事件而言是後端可控的自由字串（見
/// `AiSseEvent.terminal` 的說明）。這支測試釘的是：即使有個工具的
/// `toolCallName` 剛好叫 `done`（因此 repository 產生的 `AiSseEvent.type`
/// 也剛好是 `'done'`，但 `terminal` 為 false），controller 的 `await for`
/// 迴圈都不該被提前 `break`——後面真正的文字事件必須顯示在對話氣泡裡。
///
/// 這條在 controller 只看 `event.type` 判斷終止的舊寫法下會紅
/// （氣泡內容是空字串，因為迴圈在撞名的 tool 事件那裡就提前結束了）。
library;

import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_buyer/features/ai_assistant/presentation/ai_assistant_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('工具事件的 type 剛好叫 done 且 terminal==false：不提前結束，'
      '後面的 text 事件仍要顯示在氣泡裡', () async {
    final repository = _FakeToolNamedDoneRepository();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);

    final notifier = container.read(aiAssistantControllerProvider.notifier);
    await notifier.sendMessage('查一下工具');

    final messages = container.read(aiAssistantControllerProvider);
    expect(messages.last.role, 'assistant');
    expect(
      messages.last.content,
      contains('看得到我'),
      reason:
          '撞名的 tool 事件（type==done, terminal==false）不該提前結束串流，'
          '後面真正的 text 事件應該被 append 進氣泡',
    );
    expect(notifier.isStreaming, isFalse);
  });
}

class _FakeToolNamedDoneRepository extends AiAssistantRepository {
  _FakeToolNamedDoneRepository() : super(Dio());

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
    // 工具的 toolCallName 剛好叫 "done"：type=='done' 但 terminal==false，
    // 不是真正的 RUN_FINISHED。
    yield const AiSseEvent(type: 'done', data: '{"ok":true}', terminal: false);
    // 撞名事件之後應該還會繼續收到真正的內容。
    yield const AiSseEvent(type: 'text', data: '看得到我');
    // 只有這個才是真正的 RUN_FINISHED，terminal==true。
    yield const AiSseEvent(
      type: 'done',
      data: '{"sessionId":"s1"}',
      terminal: true,
    );
  }
}
