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

String _$connectTransportHash() => r'd6c54218de03a7416f91a265e611afce7bef6c03';

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
/// unauthenticated 時強制登出）與停權的處理（`ACCOUNT_SUSPENDED` 進受限狀態，不登出）。

@ProviderFor(connectAuthTransport)
final connectAuthTransportProvider = ConnectAuthTransportProvider._();

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
/// unauthenticated 時強制登出）與停權的處理（`ACCOUNT_SUSPENDED` 進受限狀態，不登出）。

final class ConnectAuthTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
  /// unauthenticated 時強制登出）與停權的處理（`ACCOUNT_SUSPENDED` 進受限狀態，不登出）。
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
    r'f221b4fdb5f49843f9a15562243208786cebb957';

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
    r'8aa81aec0f4a5d528885da17664ad0d3c05fe648';

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

/// 貼文、留言與討論區的公開讀取：有 session 時帶 Bearer，伺服器才填 `viewer_*`（讚過、能不能留言與刪除、留言身分）；
/// 訪客不帶。RecordPostView 也走這裡。

@ProviderFor(socialPublicClient)
final socialPublicClientProvider = SocialPublicClientProvider._();

/// 貼文、留言與討論區的公開讀取：有 session 時帶 Bearer，伺服器才填 `viewer_*`（讚過、能不能留言與刪除、留言身分）；
/// 訪客不帶。RecordPostView 也走這裡。

final class SocialPublicClientProvider
    extends
        $FunctionalProvider<
          SocialPublicServiceClient,
          SocialPublicServiceClient,
          SocialPublicServiceClient
        >
    with $Provider<SocialPublicServiceClient> {
  /// 貼文、留言與討論區的公開讀取：有 session 時帶 Bearer，伺服器才填 `viewer_*`（讚過、能不能留言與刪除、留言身分）；
  /// 訪客不帶。RecordPostView 也走這裡。
  SocialPublicClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'socialPublicClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$socialPublicClientHash();

  @$internal
  @override
  $ProviderElement<SocialPublicServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SocialPublicServiceClient create(Ref ref) {
    return socialPublicClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SocialPublicServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SocialPublicServiceClient>(value),
    );
  }
}

String _$socialPublicClientHash() =>
    r'cb66caf2cc54005109302040cce95961a21acc8c';

/// 按讚、留言與討論區發言的寫入：登入後才能呼叫，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。

@ProviderFor(socialAppClient)
final socialAppClientProvider = SocialAppClientProvider._();

/// 按讚、留言與討論區發言的寫入：登入後才能呼叫，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。

final class SocialAppClientProvider
    extends
        $FunctionalProvider<
          SocialAppServiceClient,
          SocialAppServiceClient,
          SocialAppServiceClient
        >
    with $Provider<SocialAppServiceClient> {
  /// 按讚、留言與討論區發言的寫入：登入後才能呼叫，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。
  SocialAppClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'socialAppClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$socialAppClientHash();

  @$internal
  @override
  $ProviderElement<SocialAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SocialAppServiceClient create(Ref ref) {
    return socialAppClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SocialAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SocialAppServiceClient>(value),
    );
  }
}

String _$socialAppClientHash() => r'f8da4bb52e94947a0c9f4e2619a24b9ca47673c8';

/// 買家本人的封鎖與封鎖名單（`BuyerAppService`）：使用者主動的寫入，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。

@ProviderFor(buyerAppClient)
final buyerAppClientProvider = BuyerAppClientProvider._();

/// 買家本人的封鎖與封鎖名單（`BuyerAppService`）：使用者主動的寫入，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。

final class BuyerAppClientProvider
    extends
        $FunctionalProvider<
          BuyerAppServiceClient,
          BuyerAppServiceClient,
          BuyerAppServiceClient
        >
    with $Provider<BuyerAppServiceClient> {
  /// 買家本人的封鎖與封鎖名單（`BuyerAppService`）：使用者主動的寫入，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。
  BuyerAppClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'buyerAppClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$buyerAppClientHash();

  @$internal
  @override
  $ProviderElement<BuyerAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  BuyerAppServiceClient create(Ref ref) {
    return buyerAppClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(BuyerAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<BuyerAppServiceClient>(value),
    );
  }
}

String _$buyerAppClientHash() => r'0843c0945207433da5159c77adbbe14a6cf70806';

/// 檢舉店家（`SellerReportAppService`）。

@ProviderFor(sellerReportAppClient)
final sellerReportAppClientProvider = SellerReportAppClientProvider._();

/// 檢舉店家（`SellerReportAppService`）。

final class SellerReportAppClientProvider
    extends
        $FunctionalProvider<
          SellerReportAppServiceClient,
          SellerReportAppServiceClient,
          SellerReportAppServiceClient
        >
    with $Provider<SellerReportAppServiceClient> {
  /// 檢舉店家（`SellerReportAppService`）。
  SellerReportAppClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerReportAppClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerReportAppClientHash();

  @$internal
  @override
  $ProviderElement<SellerReportAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SellerReportAppServiceClient create(Ref ref) {
    return sellerReportAppClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerReportAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerReportAppServiceClient>(value),
    );
  }
}

String _$sellerReportAppClientHash() =>
    r'4ec4db7b77272300ed62f01270c7576e93d28426';

/// 檢舉社群內容與讀自己送出的檢舉（`ModerationAppService`）。

@ProviderFor(moderationAppClient)
final moderationAppClientProvider = ModerationAppClientProvider._();

/// 檢舉社群內容與讀自己送出的檢舉（`ModerationAppService`）。

final class ModerationAppClientProvider
    extends
        $FunctionalProvider<
          ModerationAppServiceClient,
          ModerationAppServiceClient,
          ModerationAppServiceClient
        >
    with $Provider<ModerationAppServiceClient> {
  /// 檢舉社群內容與讀自己送出的檢舉（`ModerationAppService`）。
  ModerationAppClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'moderationAppClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$moderationAppClientHash();

  @$internal
  @override
  $ProviderElement<ModerationAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ModerationAppServiceClient create(Ref ref) {
    return moderationAppClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ModerationAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ModerationAppServiceClient>(value),
    );
  }
}

String _$moderationAppClientHash() =>
    r'8b3fcf02b8505f77423f433df99664ee1a4bf171';

/// 買家的聊天（`ChatAppService`）：對話、訊息、已讀與即時事件串流（`StreamEvents`，長連線不設逾時）。

@ProviderFor(chatAppClient)
final chatAppClientProvider = ChatAppClientProvider._();

/// 買家的聊天（`ChatAppService`）：對話、訊息、已讀與即時事件串流（`StreamEvents`，長連線不設逾時）。

final class ChatAppClientProvider
    extends
        $FunctionalProvider<
          ChatAppServiceClient,
          ChatAppServiceClient,
          ChatAppServiceClient
        >
    with $Provider<ChatAppServiceClient> {
  /// 買家的聊天（`ChatAppService`）：對話、訊息、已讀與即時事件串流（`StreamEvents`，長連線不設逾時）。
  ChatAppClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatAppClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatAppClientHash();

  @$internal
  @override
  $ProviderElement<ChatAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ChatAppServiceClient create(Ref ref) {
    return chatAppClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatAppServiceClient>(value),
    );
  }
}

String _$chatAppClientHash() => r'82a8a1be2af41db22b2bd53d0758cc34b233d64d';

/// 評價的公開讀取：有 session 時帶 Bearer，伺服器才套用觀看者的封鎖；訪客不帶。

@ProviderFor(reviewPublicClient)
final reviewPublicClientProvider = ReviewPublicClientProvider._();

/// 評價的公開讀取：有 session 時帶 Bearer，伺服器才套用觀看者的封鎖；訪客不帶。

final class ReviewPublicClientProvider
    extends
        $FunctionalProvider<
          ReviewPublicServiceClient,
          ReviewPublicServiceClient,
          ReviewPublicServiceClient
        >
    with $Provider<ReviewPublicServiceClient> {
  /// 評價的公開讀取：有 session 時帶 Bearer，伺服器才套用觀看者的封鎖；訪客不帶。
  ReviewPublicClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'reviewPublicClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$reviewPublicClientHash();

  @$internal
  @override
  $ProviderElement<ReviewPublicServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ReviewPublicServiceClient create(Ref ref) {
    return reviewPublicClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ReviewPublicServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ReviewPublicServiceClient>(value),
    );
  }
}

String _$reviewPublicClientHash() =>
    r'b60aeb1270379a004543c9afb489d5d7174e238a';

/// 買家的評價（`ReviewAppService`）：寫評價與讀自己寫的評價。

@ProviderFor(reviewAppClient)
final reviewAppClientProvider = ReviewAppClientProvider._();

/// 買家的評價（`ReviewAppService`）：寫評價與讀自己寫的評價。

final class ReviewAppClientProvider
    extends
        $FunctionalProvider<
          ReviewAppServiceClient,
          ReviewAppServiceClient,
          ReviewAppServiceClient
        >
    with $Provider<ReviewAppServiceClient> {
  /// 買家的評價（`ReviewAppService`）：寫評價與讀自己寫的評價。
  ReviewAppClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'reviewAppClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$reviewAppClientHash();

  @$internal
  @override
  $ProviderElement<ReviewAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ReviewAppServiceClient create(Ref ref) {
    return reviewAppClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ReviewAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ReviewAppServiceClient>(value),
    );
  }
}

String _$reviewAppClientHash() => r'5c6ea7ec43ab70c2468579cb4edc2c769fa49289';

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

String _$identityClientHash() => r'1f4a491d57f25c25f6e2e5f1fed47371cd2a2ac1';
