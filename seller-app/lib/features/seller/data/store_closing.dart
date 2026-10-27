import 'package:hawkr_api/errors.dart' show DefaultErrorText;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/submit.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'current_store.dart';

part 'store_closing.g.dart';

const closeStoreFailedMessage = '關閉商店失敗，請稍後再試';

sealed class StoreClosingOutcome {
  const StoreClosingOutcome();
}

final class StoreClosed extends StoreClosingOutcome {
  const StoreClosed();
}

final class StoreClosingFailed extends StoreClosingOutcome {
  const StoreClosingFailed(this.message);

  final String message;
}

/// 關閉商店（`CloseMyStore`）：帳號與買家身分留著，賣家角色移除。成功後重讀店家狀態，路由由此導去開店頁。
class StoreClosing {
  StoreClosing({
    required SellerAppServiceClient client,
    required void Function() adoptClosed,
    required Future<bool> Function() reload,
    required DefaultErrorText defaults,
  }) : _client = client,
       _adoptClosed = adoptClosed,
       _reload = reload,
       _defaults = defaults;

  final SellerAppServiceClient _client;
  final void Function() _adoptClosed;
  final Future<bool> Function() _reload;
  final DefaultErrorText _defaults;

  Future<StoreClosingOutcome> close() async {
    try {
      await _client.closeMyStore(CloseMyStoreRequest());
    } catch (error) {
      HKLog.logW('[StoreClosing] CloseMyStore failed: $error');
      return switch (submitError(error, defaults: _defaults)) {
        SubmitMessage(:final message) => StoreClosingFailed(message),
        _ => const StoreClosingFailed(closeStoreFailedMessage),
      };
    }
    _adoptClosed();
    await _reload();
    return const StoreClosed();
  }
}

@Riverpod(keepAlive: true)
StoreClosing storeClosing(Ref ref) => StoreClosing(
  client: ref.watch(sellerClientProvider),
  adoptClosed: () => ref.read(currentStoreProvider.notifier).adoptClosed(),
  reload: () => ref.read(currentStoreProvider.notifier).refresh(),
  defaults: ref.watch(defaultErrorTextProvider),
);
