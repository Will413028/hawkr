// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_status_refresh.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 重讀賣家的伺服器狀態：店家（`GetMyStore`）、認證流程（`GetMyKYC`）、收款帳戶（`GetMyPaymentAccount`）與開通摘要
/// （`GetMyOnboarding`）一起，四者不會一個新一個舊。

@ProviderFor(SellerStatusRefresh)
final sellerStatusRefreshProvider = SellerStatusRefreshProvider._();

/// 重讀賣家的伺服器狀態：店家（`GetMyStore`）、認證流程（`GetMyKYC`）、收款帳戶（`GetMyPaymentAccount`）與開通摘要
/// （`GetMyOnboarding`）一起，四者不會一個新一個舊。
final class SellerStatusRefreshProvider
    extends $NotifierProvider<SellerStatusRefresh, void> {
  /// 重讀賣家的伺服器狀態：店家（`GetMyStore`）、認證流程（`GetMyKYC`）、收款帳戶（`GetMyPaymentAccount`）與開通摘要
  /// （`GetMyOnboarding`）一起，四者不會一個新一個舊。
  SellerStatusRefreshProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerStatusRefreshProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerStatusRefreshHash();

  @$internal
  @override
  SellerStatusRefresh create() => SellerStatusRefresh();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$sellerStatusRefreshHash() =>
    r'3fc887a7f3aa9a76415b9eabe476d48a50d0ff2a';

/// 重讀賣家的伺服器狀態：店家（`GetMyStore`）、認證流程（`GetMyKYC`）、收款帳戶（`GetMyPaymentAccount`）與開通摘要
/// （`GetMyOnboarding`）一起，四者不會一個新一個舊。

abstract class _$SellerStatusRefresh extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<void, void>,
              void,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
