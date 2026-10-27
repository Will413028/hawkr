/// 殘留缺陷（reviewer 在買家分支上抓到、同一份設計在賣家分支上同樣存在）：
/// `terminal` 只走到 repository 就被消化掉了——`AiAssistantController` 舊寫法
/// 用 `event.type == 'done'`/`'error'` 字串比對決定要不要 `break`，若有個
/// 工具剛好叫 `done`，對應的 `TOOL_CALL_RESULT` 事件 `type` 就會是 `done`
/// （但 `terminal` 是 false），舊寫法會在這裡誤判提前結束，後面真正的
/// text/done 事件永遠不會被消費，使用者看到空氣泡。
///
/// 這支測試釘住修好後的行為：終止判斷改看 `event.terminal`，`type=='done'`
/// 的巧合事件不該打斷後續事件的消費。
library;

import 'package:hawkr_seller/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_seller/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_seller/features/ai_assistant/presentation/ai_assistant_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('工具叫 done：type==done 但 terminal==false 不該提前截斷後續 text', () async {
    final repository = _FakeToolNamedDoneRepository();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repository)],
    );
    addTearDown(container.dispose);

    final notifier = container.read(aiAssistantControllerProvider.notifier);
    await notifier.sendMessage('查詢出貨進度');

    final messages = container.read(aiAssistantControllerProvider);
    expect(messages.last.role, 'assistant');
    expect(
      messages.last.content,
      contains('看得到我嗎'),
      reason:
          '若舊寫法用 type==done 誤判提前終止，這段在巧合 done 事件之後才到達的 '
          'text 永遠不會被消費，氣泡會是空的。實際內容：${messages.last.content}',
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
    SellerAssistantEndpoint endpoint = SellerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    // 對應一個 toolCallName 剛好叫 "done" 的 TOOL_CALL_RESULT：type==done，
    // 但它不是 RUN_FINISHED，terminal 該是 false。
    yield const AiSseEvent(type: 'done', data: '{"ok":true}', terminal: false);
    // 巧合事件之後應該還能繼續消費到真正的 text。
    yield const AiSseEvent(type: 'text', data: '看得到我嗎');
    // 真正的 RUN_FINISHED：type 也是 done，但 terminal 是 true。
    yield const AiSseEvent(
      type: 'done',
      data: '{"sessionId":"s1"}',
      terminal: true,
    );
  }
}
