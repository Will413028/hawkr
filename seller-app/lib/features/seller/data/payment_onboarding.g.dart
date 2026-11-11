// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'payment_onboarding.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// website 的 origin（`--dart-define=HAWKR_WEB_ORIGIN`，見 [FlavorExtension.webOrigin]）。

@ProviderFor(webOrigin)
final webOriginProvider = WebOriginProvider._();

/// website 的 origin（`--dart-define=HAWKR_WEB_ORIGIN`，見 [FlavorExtension.webOrigin]）。

final class WebOriginProvider
    extends $FunctionalProvider<String, String, String>
    with $Provider<String> {
  /// website 的 origin（`--dart-define=HAWKR_WEB_ORIGIN`，見 [FlavorExtension.webOrigin]）。
  WebOriginProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'webOriginProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$webOriginHash();

  @$internal
  @override
  $ProviderElement<String> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  String create(Ref ref) {
    return webOrigin(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String>(value),
    );
  }
}

String _$webOriginHash() => r'3040edafbef14ddcb78d9036502c709eaf56716b';

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

/// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次（畫面各自的回前景重讀走 [onResumed]，不會重複消耗記號）。

@ProviderFor(PaymentOnboarding)
final paymentOnboardingProvider = PaymentOnboardingProvider._();

/// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次（畫面各自的回前景重讀走 [onResumed]，不會重複消耗記號）。
final class PaymentOnboardingProvider
    extends $NotifierProvider<PaymentOnboarding, PaymentRefreshFailure?> {
  /// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
  /// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
  /// 還在都會以 `fresh: true` 重讀一次（畫面各自的回前景重讀走 [onResumed]，不會重複消耗記號）。
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
  Override overrideWithValue(PaymentRefreshFailure? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PaymentRefreshFailure?>(value),
    );
  }
}

String _$paymentOnboardingHash() => r'6efc1a647d0272b0789923fae1b8cf1b44420ecc';

/// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次（畫面各自的回前景重讀走 [onResumed]，不會重複消耗記號）。

abstract class _$PaymentOnboarding extends $Notifier<PaymentRefreshFailure?> {
  PaymentRefreshFailure? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<PaymentRefreshFailure?, PaymentRefreshFailure?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<PaymentRefreshFailure?, PaymentRefreshFailure?>,
              PaymentRefreshFailure?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
