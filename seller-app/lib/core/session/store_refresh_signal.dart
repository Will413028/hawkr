import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'store_refresh_signal.g.dart';

/// 要求重讀店家（`GetMyStore`）的訊號：session 收到「賣家已停權」的即時訊號時發出，`currentStoreProvider` 收到後重讀。
/// 讓 session 不必依賴店家狀態（店家狀態反過來依賴 session 的帳號）。
@Riverpod(keepAlive: true)
class StoreRefreshSignal extends _$StoreRefreshSignal {
  @override
  int build() => 0;

  void request() => state++;
}
