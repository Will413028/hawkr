// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_refresh_signal.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 要求重讀店家（`GetMyStore`）的訊號：session 收到「賣家已停權」的即時訊號時發出，`currentStoreProvider` 收到後重讀。
/// 讓 session 不必依賴店家狀態（店家狀態反過來依賴 session 的帳號）。

@ProviderFor(StoreRefreshSignal)
final storeRefreshSignalProvider = StoreRefreshSignalProvider._();

/// 要求重讀店家（`GetMyStore`）的訊號：session 收到「賣家已停權」的即時訊號時發出，`currentStoreProvider` 收到後重讀。
/// 讓 session 不必依賴店家狀態（店家狀態反過來依賴 session 的帳號）。
final class StoreRefreshSignalProvider
    extends $NotifierProvider<StoreRefreshSignal, int> {
  /// 要求重讀店家（`GetMyStore`）的訊號：session 收到「賣家已停權」的即時訊號時發出，`currentStoreProvider` 收到後重讀。
  /// 讓 session 不必依賴店家狀態（店家狀態反過來依賴 session 的帳號）。
  StoreRefreshSignalProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeRefreshSignalProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeRefreshSignalHash();

  @$internal
  @override
  StoreRefreshSignal create() => StoreRefreshSignal();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$storeRefreshSignalHash() =>
    r'44474e95d083c617115ab445810136c04178bf25';

/// 要求重讀店家（`GetMyStore`）的訊號：session 收到「賣家已停權」的即時訊號時發出，`currentStoreProvider` 收到後重讀。
/// 讓 session 不必依賴店家狀態（店家狀態反過來依賴 session 的帳號）。

abstract class _$StoreRefreshSignal extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
