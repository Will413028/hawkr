// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'payment_onboarding.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 開通連結允許 http（本機測試）：只有 dev。stage 與 prod 的金流商頁面一律 https。

@ProviderFor(allowInsecureOnboardingUrl)
final allowInsecureOnboardingUrlProvider =
    AllowInsecureOnboardingUrlProvider._();

/// 開通連結允許 http（本機測試）：只有 dev。stage 與 prod 的金流商頁面一律 https。

final class AllowInsecureOnboardingUrlProvider
    extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// 開通連結允許 http（本機測試）：只有 dev。stage 與 prod 的金流商頁面一律 https。
  AllowInsecureOnboardingUrlProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'allowInsecureOnboardingUrlProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$allowInsecureOnboardingUrlHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return allowInsecureOnboardingUrl(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$allowInsecureOnboardingUrlHash() =>
    r'b59b5ee50fd8141befae59c7861318c63fd2cc03';

/// 在外部瀏覽器開網址（託管頁面不支援 WebView）；回傳有沒有開成功。

@ProviderFor(externalBrowserLauncher)
final externalBrowserLauncherProvider = ExternalBrowserLauncherProvider._();

/// 在外部瀏覽器開網址（託管頁面不支援 WebView）；回傳有沒有開成功。

final class ExternalBrowserLauncherProvider
    extends
        $FunctionalProvider<
          Future<bool> Function(Uri),
          Future<bool> Function(Uri),
          Future<bool> Function(Uri)
        >
    with $Provider<Future<bool> Function(Uri)> {
  /// 在外部瀏覽器開網址（託管頁面不支援 WebView）；回傳有沒有開成功。
  ExternalBrowserLauncherProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'externalBrowserLauncherProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$externalBrowserLauncherHash();

  @$internal
  @override
  $ProviderElement<Future<bool> Function(Uri)> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  Future<bool> Function(Uri) create(Ref ref) {
    return externalBrowserLauncher(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Future<bool> Function(Uri) value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Future<bool> Function(Uri)>(value),
    );
  }
}

String _$externalBrowserLauncherHash() =>
    r'6ab8c083515890c713d21dc29e6b84f60b96893c';

/// 收款帳戶的開通與自動重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次。回前景的觸發只有這裡一個：畫面用 [watchForeground] 登記「我在前景時要重讀」，
/// 一次回前景最多呼叫一次 [onResumed]。
///
/// `RefreshMyPaymentAccount` 是背景動作：失敗（含 `resource_exhausted`、停權的 `permission_denied`）不顯示，也不擋讀開通摘要。

@ProviderFor(PaymentOnboarding)
final paymentOnboardingProvider = PaymentOnboardingProvider._();

/// 收款帳戶的開通與自動重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次。回前景的觸發只有這裡一個：畫面用 [watchForeground] 登記「我在前景時要重讀」，
/// 一次回前景最多呼叫一次 [onResumed]。
///
/// `RefreshMyPaymentAccount` 是背景動作：失敗（含 `resource_exhausted`、停權的 `permission_denied`）不顯示，也不擋讀開通摘要。
final class PaymentOnboardingProvider
    extends $NotifierProvider<PaymentOnboarding, void> {
  /// 收款帳戶的開通與自動重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
  /// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
  /// 還在都會以 `fresh: true` 重讀一次。回前景的觸發只有這裡一個：畫面用 [watchForeground] 登記「我在前景時要重讀」，
  /// 一次回前景最多呼叫一次 [onResumed]。
  ///
  /// `RefreshMyPaymentAccount` 是背景動作：失敗（含 `resource_exhausted`、停權的 `permission_denied`）不顯示，也不擋讀開通摘要。
  PaymentOnboardingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'paymentOnboardingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$paymentOnboardingHash();

  @$internal
  @override
  PaymentOnboarding create() => PaymentOnboarding();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$paymentOnboardingHash() => r'719555cb6445efa17d4b49fe68b95798345adceb';

/// 收款帳戶的開通與自動重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次。回前景的觸發只有這裡一個：畫面用 [watchForeground] 登記「我在前景時要重讀」，
/// 一次回前景最多呼叫一次 [onResumed]。
///
/// `RefreshMyPaymentAccount` 是背景動作：失敗（含 `resource_exhausted`、停權的 `permission_denied`）不顯示，也不擋讀開通摘要。

abstract class _$PaymentOnboarding extends $Notifier<void> {
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
