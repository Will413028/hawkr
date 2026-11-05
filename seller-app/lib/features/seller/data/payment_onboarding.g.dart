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
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上。

@ProviderFor(PaymentOnboarding)
final paymentOnboardingProvider = PaymentOnboardingProvider._();

/// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上。
final class PaymentOnboardingProvider
    extends $NotifierProvider<PaymentOnboarding, void> {
  /// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
  /// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上。
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

String _$paymentOnboardingHash() => r'e44ee81ebea2c17082e1e7c28cadd5bd14541e68';

/// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上。

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
