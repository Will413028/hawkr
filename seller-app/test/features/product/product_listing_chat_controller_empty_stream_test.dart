/// `ProductListingChatRepository.stream()` 在 `resp.data?.stream == null`
/// 時會 yield `SseEvent(type: 'error', data: 'empty stream', terminal: true)`
/// ——這支測試釘的是 `ProductListingChatController` 收到這個特定 fallback
/// 事件形狀時的反應：`terminal: true` 必須讓 `case 'error':` 的語意動作
/// （設定 `state.error`）真的執行，不能被 controller 新的
/// `if (event.terminal)` 終止判斷擋下來而悄悄吞掉。
library;

import 'package:hawkr_seller/features/product/data/product_listing_chat_repository.dart';
import 'package:hawkr_seller/features/product/data/product_listing_model.dart';
import 'package:hawkr_seller/features/product/presentation/product_listing_chat/product_listing_chat_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test(
    'repository 吐出 empty stream fallback（terminal:true）：state.error 真的被設定',
    () async {
      final container = ProviderContainer(
        overrides: [
          productListingChatRepositoryProvider.overrideWithValue(
            _FakeEmptyStreamRepository(),
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
        isNotNull,
        reason:
            '若 empty stream 這個事件的 terminal 是 false（或 controller '
            '沒有正確依 terminal 執行 case error 的語意動作），state.error '
            '會永遠是 null，使用者完全沒有錯誤回饋',
      );
      expect(state.isStreaming, isFalse);
    },
  );
}

class _FakeEmptyStreamRepository implements ProductListingChatRepository {
  @override
  Stream<SseEvent> stream({
    required String message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    yield SseEvent(type: 'error', data: 'empty stream', terminal: true);
  }
}
