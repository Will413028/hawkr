/// 殘留缺陷（reviewer 在買家分支上抓到、同一份設計在賣家分支上同樣存在）：
/// `terminal` 只走到 repository 就被消化掉了——`ProductListingChatController`
/// 舊寫法用 `case 'error':` 的控制流（直接 `return`）決定要不要結束對話，
/// 若有個 CUSTOM 事件的 `name` 剛好叫 `error`，舊寫法會把它當成真正的錯誤，
/// `state.error` 被設定、整個對話被中斷，即使後面還有正常的 text 事件。
///
/// 這支測試釘住修好後的行為：`error`/`done` case 的控制流改看
/// `event.terminal`，`type=='error'` 的巧合事件不該打斷後續事件的消費，也
/// 不該被誤判成真正的錯誤。
library;

import 'package:hawkr_seller/features/product/data/product_listing_chat_repository.dart';
import 'package:hawkr_seller/features/product/data/product_listing_model.dart';
import 'package:hawkr_seller/features/product/presentation/product_listing_chat/product_listing_chat_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('CUSTOM 事件 name 剛好叫 error：type==error 但 terminal==false 不該設定 '
      'state.error 或打斷後續 text', () async {
    final container = ProviderContainer(
      overrides: [
        productListingChatRepositoryProvider.overrideWithValue(
          _FakeCustomNamedErrorRepository(),
        ),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(
      productListingChatControllerProvider.notifier,
    );
    await notifier.sendMessage('幫我上架高麗菜');

    final state = container.read(productListingChatControllerProvider);
    expect(
      state.error,
      isNull,
      reason: '巧合的 CUSTOM name==error 不是真正的錯誤，不該設定 state.error',
    );
    expect(
      state.messages.last.text,
      contains('看得到我嗎'),
      reason:
          '若舊寫法用 case error 誤判提前 return，這段在巧合 error 事件之後才到達的 '
          'text 永遠不會被消費。實際內容：${state.messages.last.text}',
    );
    expect(state.isStreaming, isFalse);
  });
}

class _FakeCustomNamedErrorRepository implements ProductListingChatRepository {
  @override
  Stream<SseEvent> stream({
    required String message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    // 對應一個 name 剛好叫 "error" 的 CUSTOM 事件：type==error，但它不是
    // RUN_ERROR，terminal 該是 false。
    yield SseEvent(
      type: 'error',
      data: '{"note":"not a real error"}',
      terminal: false,
    );
    // 巧合事件之後應該還能繼續消費到真正的 text。
    yield SseEvent(type: 'text', data: '看得到我嗎');
    // 真正的 RUN_FINISHED：terminal 是 true。
    yield SseEvent(type: 'done', data: '{"sessionId":"s1"}', terminal: true);
  }
}
