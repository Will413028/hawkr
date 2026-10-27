import 'package:hawkr_seller/features/product/data/product_listing_chat_repository.dart';
import 'package:hawkr_seller/features/product/data/product_listing_model.dart';
import 'package:hawkr_seller/features/product/presentation/product_listing_chat/product_listing_chat_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('收到 error event 時 state.error 被設定且串流結束', () async {
    final container = ProviderContainer(
      overrides: [
        productListingChatRepositoryProvider.overrideWithValue(
          _FakeErrorRepository(),
        ),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(
      productListingChatControllerProvider.notifier,
    );
    await notifier.sendMessage('幫我上架高麗菜');

    final state = container.read(productListingChatControllerProvider);
    expect(state.error, '系統忙碌中，請稍後再試');
    expect(state.isStreaming, isFalse);
  });
}

class _FakeErrorRepository implements ProductListingChatRepository {
  @override
  Stream<SseEvent> stream({
    required String message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    // terminal: true——這個 fake 代表真正的 RUN_ERROR（見
    // product_listing_chat_controller.dart 新的終止判斷：只有 terminal
    // 為 true 時才會設定 state.error 並結束串流，避免同名的
    // tool_result/CUSTOM 事件被誤判成控制流訊號）。
    yield SseEvent(
      type: 'error',
      data: '{"code":500,"message":"系統忙碌中，請稍後再試"}',
      terminal: true,
    );
  }
}
