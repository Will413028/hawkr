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

/// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header
/// （`headers`，只有版本與 App 類型）。

@ProviderFor(connectTransport)
final connectTransportProvider = ConnectTransportProvider._();

/// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header
/// （`headers`，只有版本與 App 類型）。

final class ConnectTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header
  /// （`headers`，只有版本與 App 類型）。
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

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
/// unauthenticated 時強制登出）。

@ProviderFor(connectAuthTransport)
final connectAuthTransportProvider = ConnectAuthTransportProvider._();

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
/// unauthenticated 時強制登出）。

final class ConnectAuthTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
  /// unauthenticated 時強制登出）。
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
    r'dd5e385653b66d1ea73b74316e75114c6fc7decf';

/// 公開的 service 但伺服器依身分決定行為的呼叫（例如 RecordProductView）：有 session 就帶 Bearer，沒有（訪客）
/// 或換發失敗就不帶，不中斷呼叫。

@ProviderFor(connectOptionalAuthTransport)
final connectOptionalAuthTransportProvider =
    ConnectOptionalAuthTransportProvider._();

/// 公開的 service 但伺服器依身分決定行為的呼叫（例如 RecordProductView）：有 session 就帶 Bearer，沒有（訪客）
/// 或換發失敗就不帶，不中斷呼叫。

final class ConnectOptionalAuthTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 公開的 service 但伺服器依身分決定行為的呼叫（例如 RecordProductView）：有 session 就帶 Bearer，沒有（訪客）
  /// 或換發失敗就不帶，不中斷呼叫。
  ConnectOptionalAuthTransportProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'connectOptionalAuthTransportProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$connectOptionalAuthTransportHash();

  @$internal
  @override
  $ProviderElement<Transport> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Transport create(Ref ref) {
    return connectOptionalAuthTransport(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Transport value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Transport>(value),
    );
  }
}

String _$connectOptionalAuthTransportHash() =>
    r'10b70e08ac1d10a63dec7bef6d7c886063d98919';

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

@ProviderFor(sellerPublicClient)
final sellerPublicClientProvider = SellerPublicClientProvider._();

final class SellerPublicClientProvider
    extends
        $FunctionalProvider<
          SellerPublicServiceClient,
          SellerPublicServiceClient,
          SellerPublicServiceClient
        >
    with $Provider<SellerPublicServiceClient> {
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

/// RecordProductView：登入與否由伺服器判斷，所以有 session 時帶 Bearer。

@ProviderFor(sellerPublicOptionalAuthClient)
final sellerPublicOptionalAuthClientProvider =
    SellerPublicOptionalAuthClientProvider._();

/// RecordProductView：登入與否由伺服器判斷，所以有 session 時帶 Bearer。

final class SellerPublicOptionalAuthClientProvider
    extends
        $FunctionalProvider<
          SellerPublicServiceClient,
          SellerPublicServiceClient,
          SellerPublicServiceClient
        >
    with $Provider<SellerPublicServiceClient> {
  /// RecordProductView：登入與否由伺服器判斷，所以有 session 時帶 Bearer。
  SellerPublicOptionalAuthClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerPublicOptionalAuthClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerPublicOptionalAuthClientHash();

  @$internal
  @override
  $ProviderElement<SellerPublicServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SellerPublicServiceClient create(Ref ref) {
    return sellerPublicOptionalAuthClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerPublicServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerPublicServiceClient>(value),
    );
  }
}

String _$sellerPublicOptionalAuthClientHash() =>
    r'4f505af0cb6f075a95f80c461c4f3abf9451d307';

@ProviderFor(discoveryPublicClient)
final discoveryPublicClientProvider = DiscoveryPublicClientProvider._();

final class DiscoveryPublicClientProvider
    extends
        $FunctionalProvider<
          DiscoveryPublicServiceClient,
          DiscoveryPublicServiceClient,
          DiscoveryPublicServiceClient
        >
    with $Provider<DiscoveryPublicServiceClient> {
  DiscoveryPublicClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'discoveryPublicClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$discoveryPublicClientHash();

  @$internal
  @override
  $ProviderElement<DiscoveryPublicServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  DiscoveryPublicServiceClient create(Ref ref) {
    return discoveryPublicClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DiscoveryPublicServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DiscoveryPublicServiceClient>(value),
    );
  }
}

String _$discoveryPublicClientHash() =>
    r'0fb25558c420b2cab6c2080e1e82a1276329ec40';

/// 登入的帳號（GetSession）。登入（`IdentityPublicService`）與 session token 的保存見 `authSessionProvider`。

@ProviderFor(identityClient)
final identityClientProvider = IdentityClientProvider._();

/// 登入的帳號（GetSession）。登入（`IdentityPublicService`）與 session token 的保存見 `authSessionProvider`。

final class IdentityClientProvider
    extends
        $FunctionalProvider<
          IdentityAppServiceClient,
          IdentityAppServiceClient,
          IdentityAppServiceClient
        >
    with $Provider<IdentityAppServiceClient> {
  /// 登入的帳號（GetSession）。登入（`IdentityPublicService`）與 session token 的保存見 `authSessionProvider`。
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

String _$identityClientHash() => r'35c932995cdde039b931c7d5cde363a7591a814b';
