// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'connect_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 所有 Connect transport 共用的 HTTP/2 連線。

@ProviderFor(connectHttpClient)
final connectHttpClientProvider = ConnectHttpClientProvider._();

/// 所有 Connect transport 共用的 HTTP/2 連線。

final class ConnectHttpClientProvider
    extends $FunctionalProvider<HttpClient, HttpClient, HttpClient>
    with $Provider<HttpClient> {
  /// 所有 Connect transport 共用的 HTTP/2 連線。
  ConnectHttpClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'connectHttpClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$connectHttpClientHash();

  @$internal
  @override
  $ProviderElement<HttpClient> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  HttpClient create(Ref ref) {
    return connectHttpClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(HttpClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<HttpClient>(value),
    );
  }
}

String _$connectHttpClientHash() => r'ca7d9b0b909e108eb8636f6111757d74d22acecc';

/// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header。

@ProviderFor(connectTransport)
final connectTransportProvider = ConnectTransportProvider._();

/// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header。

final class ConnectTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header。
  ConnectTransportProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'connectTransportProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$connectTransportHash();

  @$internal
  @override
  $ProviderElement<Transport> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Transport create(Ref ref) {
    return connectTransport(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Transport value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Transport>(value),
    );
  }
}

String _$connectTransportHash() => r'f455e4d38b76be2661bbcbf11df9ed3362e04f48';

@ProviderFor(contentClient)
final contentClientProvider = ContentClientProvider._();

final class ContentClientProvider
    extends
        $FunctionalProvider<
          ContentPublicServiceClient,
          ContentPublicServiceClient,
          ContentPublicServiceClient
        >
    with $Provider<ContentPublicServiceClient> {
  ContentClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'contentClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$contentClientHash();

  @$internal
  @override
  $ProviderElement<ContentPublicServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ContentPublicServiceClient create(Ref ref) {
    return contentClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ContentPublicServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ContentPublicServiceClient>(value),
    );
  }
}

String _$contentClientHash() => r'02a325cac6180f94d43b9770aba7203355e6ed50';

/// 登入的帳號（GetSession）與刪帳前撤銷 Apple 的授權。登入與登出由 Clerk 處理（`clerkSessionProvider`）。

@ProviderFor(identityClient)
final identityClientProvider = IdentityClientProvider._();

/// 登入的帳號（GetSession）與刪帳前撤銷 Apple 的授權。登入與登出由 Clerk 處理（`clerkSessionProvider`）。

final class IdentityClientProvider
    extends
        $FunctionalProvider<
          IdentityAppServiceClient,
          IdentityAppServiceClient,
          IdentityAppServiceClient
        >
    with $Provider<IdentityAppServiceClient> {
  /// 登入的帳號（GetSession）與刪帳前撤銷 Apple 的授權。登入與登出由 Clerk 處理（`clerkSessionProvider`）。
  IdentityClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'identityClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$identityClientHash();

  @$internal
  @override
  $ProviderElement<IdentityAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  IdentityAppServiceClient create(Ref ref) {
    return identityClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(IdentityAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<IdentityAppServiceClient>(value),
    );
  }
}

String _$identityClientHash() => r'073ae0319f0c732d9244e8a20a6cb8736e257511';

/// 賣家的店家、入駐與 KYC（SellerAppService），登入後呼叫。

@ProviderFor(sellerClient)
final sellerClientProvider = SellerClientProvider._();

/// 賣家的店家、入駐與 KYC（SellerAppService），登入後呼叫。

final class SellerClientProvider
    extends
        $FunctionalProvider<
          SellerAppServiceClient,
          SellerAppServiceClient,
          SellerAppServiceClient
        >
    with $Provider<SellerAppServiceClient> {
  /// 賣家的店家、入駐與 KYC（SellerAppService），登入後呼叫。
  SellerClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerClientHash();

  @$internal
  @override
  $ProviderElement<SellerAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SellerAppServiceClient create(Ref ref) {
    return sellerClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerAppServiceClient>(value),
    );
  }
}

String _$sellerClientHash() => r'be61a35edbfb961a1af888c45ab11591b268c4a3';

/// 賣家入駐文件的現行版本（SellerPublicService），不需登入。

@ProviderFor(sellerPublicClient)
final sellerPublicClientProvider = SellerPublicClientProvider._();

/// 賣家入駐文件的現行版本（SellerPublicService），不需登入。

final class SellerPublicClientProvider
    extends
        $FunctionalProvider<
          SellerPublicServiceClient,
          SellerPublicServiceClient,
          SellerPublicServiceClient
        >
    with $Provider<SellerPublicServiceClient> {
  /// 賣家入駐文件的現行版本（SellerPublicService），不需登入。
  SellerPublicClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerPublicClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerPublicClientHash();

  @$internal
  @override
  $ProviderElement<SellerPublicServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SellerPublicServiceClient create(Ref ref) {
    return sellerPublicClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerPublicServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerPublicServiceClient>(value),
    );
  }
}

String _$sellerPublicClientHash() =>
    r'33b02a400574f91d3a4dcb34e3885114f52bb233';

@ProviderFor(sessionIdentityClient)
final sessionIdentityClientProvider = SessionIdentityClientProvider._();

final class SessionIdentityClientProvider
    extends
        $FunctionalProvider<
          IdentityAppServiceClient,
          IdentityAppServiceClient,
          IdentityAppServiceClient
        >
    with $Provider<IdentityAppServiceClient> {
  SessionIdentityClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sessionIdentityClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sessionIdentityClientHash();

  @$internal
  @override
  $ProviderElement<IdentityAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  IdentityAppServiceClient create(Ref ref) {
    return sessionIdentityClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(IdentityAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<IdentityAppServiceClient>(value),
    );
  }
}

String _$sessionIdentityClientHash() =>
    r'fe12a260e10877ee7a238127ea604118a2282126';

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 Clerk 的 session token；
/// unauthenticated 時登出）。

@ProviderFor(connectAuthTransport)
final connectAuthTransportProvider = ConnectAuthTransportProvider._();

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 Clerk 的 session token；
/// unauthenticated 時登出）。

final class ConnectAuthTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 Clerk 的 session token；
  /// unauthenticated 時登出）。
  ConnectAuthTransportProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'connectAuthTransportProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$connectAuthTransportHash();

  @$internal
  @override
  $ProviderElement<Transport> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Transport create(Ref ref) {
    return connectAuthTransport(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Transport value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Transport>(value),
    );
  }
}

String _$connectAuthTransportHash() =>
    r'3e1ef2122e560205a197c2dfb0d264f753d5bc68';
