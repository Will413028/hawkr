// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pending_wallet_focus.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 要在錢包頁展開並捲到的區塊（見 `WalletSections`）。呼叫端（進度卡）在導覽到錢包之前設定，錢包套用時取走並清掉：
/// 每次導覽只套用一次，重建、還原網址都不會重播。

@ProviderFor(PendingWalletFocus)
final pendingWalletFocusProvider = PendingWalletFocusProvider._();

/// 要在錢包頁展開並捲到的區塊（見 `WalletSections`）。呼叫端（進度卡）在導覽到錢包之前設定，錢包套用時取走並清掉：
/// 每次導覽只套用一次，重建、還原網址都不會重播。
final class PendingWalletFocusProvider
    extends $NotifierProvider<PendingWalletFocus, String?> {
  /// 要在錢包頁展開並捲到的區塊（見 `WalletSections`）。呼叫端（進度卡）在導覽到錢包之前設定，錢包套用時取走並清掉：
  /// 每次導覽只套用一次，重建、還原網址都不會重播。
  PendingWalletFocusProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'pendingWalletFocusProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$pendingWalletFocusHash();

  @$internal
  @override
  PendingWalletFocus create() => PendingWalletFocus();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$pendingWalletFocusHash() =>
    r'f790d76d046c701921470ab80b3669345b0ec3f5';

/// 要在錢包頁展開並捲到的區塊（見 `WalletSections`）。呼叫端（進度卡）在導覽到錢包之前設定，錢包套用時取走並清掉：
/// 每次導覽只套用一次，重建、還原網址都不會重播。

abstract class _$PendingWalletFocus extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<String?, String?>,
              String?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
