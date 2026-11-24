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

String _$connectTransportHash() => r'd6c54218de03a7416f91a265e611afce7bef6c03';

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

String _$identityClientHash() => r'073ae0319f0c732d9244e8a20a6cb8736e257511';

/// 圖片與機敏檔案的上傳會話（MediaAppService），登入後呼叫。

@ProviderFor(mediaClient)
final mediaClientProvider = MediaClientProvider._();

/// 圖片與機敏檔案的上傳會話（MediaAppService），登入後呼叫。

final class MediaClientProvider
    extends
        $FunctionalProvider<
          MediaAppServiceClient,
          MediaAppServiceClient,
          MediaAppServiceClient
        >
    with $Provider<MediaAppServiceClient> {
  /// 圖片與機敏檔案的上傳會話（MediaAppService），登入後呼叫。
  MediaClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'mediaClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$mediaClientHash();

  @$internal
  @override
  $ProviderElement<MediaAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  MediaAppServiceClient create(Ref ref) {
    return mediaClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MediaAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MediaAppServiceClient>(value),
    );
  }
}

String _$mediaClientHash() => r'473fabd3f9cced88e92e2c5f2c57cb3075ad3f1c';

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

/// 貼文與留言的互動寫入與店主的貼文管理（SocialAppService），登入後呼叫。

@ProviderFor(socialClient)
final socialClientProvider = SocialClientProvider._();

/// 貼文與留言的互動寫入與店主的貼文管理（SocialAppService），登入後呼叫。

final class SocialClientProvider
    extends
        $FunctionalProvider<
          SocialAppServiceClient,
          SocialAppServiceClient,
          SocialAppServiceClient
        >
    with $Provider<SocialAppServiceClient> {
  /// 貼文與留言的互動寫入與店主的貼文管理（SocialAppService），登入後呼叫。
  SocialClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'socialClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$socialClientHash();

  @$internal
  @override
  $ProviderElement<SocialAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SocialAppServiceClient create(Ref ref) {
    return socialClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SocialAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SocialAppServiceClient>(value),
    );
  }
}

String _$socialClientHash() => r'cf9129264e0b4784af3cffc1f7764c352eabc323';

/// 貼文與留言、店家討論區的讀取（SocialPublicService）。帶登入身分的 transport：店主讀自己的貼文留言與討論區
/// 不受公開狀態限制，伺服器依憑證判斷。

@ProviderFor(socialPublicClient)
final socialPublicClientProvider = SocialPublicClientProvider._();

/// 貼文與留言、店家討論區的讀取（SocialPublicService）。帶登入身分的 transport：店主讀自己的貼文留言與討論區
/// 不受公開狀態限制，伺服器依憑證判斷。

final class SocialPublicClientProvider
    extends
        $FunctionalProvider<
          SocialPublicServiceClient,
          SocialPublicServiceClient,
          SocialPublicServiceClient
        >
    with $Provider<SocialPublicServiceClient> {
  /// 貼文與留言、店家討論區的讀取（SocialPublicService）。帶登入身分的 transport：店主讀自己的貼文留言與討論區
  /// 不受公開狀態限制，伺服器依憑證判斷。
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
    r'6385043b29b731eb108b73e4cf314ffff63bb9da';

/// 店家側的聊天（ChatAppService 的 `*Store*` 方法與對話上的方法），登入後呼叫；店家由伺服器依呼叫者此刻的店推導。

@ProviderFor(chatClient)
final chatClientProvider = ChatClientProvider._();

/// 店家側的聊天（ChatAppService 的 `*Store*` 方法與對話上的方法），登入後呼叫；店家由伺服器依呼叫者此刻的店推導。

final class ChatClientProvider
    extends
        $FunctionalProvider<
          ChatAppServiceClient,
          ChatAppServiceClient,
          ChatAppServiceClient
        >
    with $Provider<ChatAppServiceClient> {
  /// 店家側的聊天（ChatAppService 的 `*Store*` 方法與對話上的方法），登入後呼叫；店家由伺服器依呼叫者此刻的店推導。
  ChatClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'chatClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$chatClientHash();

  @$internal
  @override
  $ProviderElement<ChatAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ChatAppServiceClient create(Ref ref) {
    return chatClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChatAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChatAppServiceClient>(value),
    );
  }
}

String _$chatClientHash() => r'8bc23ee260c22d7978f587e4d1b4e1e6f1e4fd59';

/// 店家的客戶與標籤（CrmAppService），登入後呼叫。

@ProviderFor(crmClient)
final crmClientProvider = CrmClientProvider._();

/// 店家的客戶與標籤（CrmAppService），登入後呼叫。

final class CrmClientProvider
    extends
        $FunctionalProvider<
          CrmAppServiceClient,
          CrmAppServiceClient,
          CrmAppServiceClient
        >
    with $Provider<CrmAppServiceClient> {
  /// 店家的客戶與標籤（CrmAppService），登入後呼叫。
  CrmClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'crmClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$crmClientHash();

  @$internal
  @override
  $ProviderElement<CrmAppServiceClient> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  CrmAppServiceClient create(Ref ref) {
    return crmClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CrmAppServiceClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CrmAppServiceClient>(value),
    );
  }
}

String _$crmClientHash() => r'81d3f74653cbf6a070ea5058dfcf215231442778';

/// 店家的評價（ReviewAppService：自己店的評價列表與回覆），登入後呼叫。

@ProviderFor(reviewAppClient)
final reviewAppClientProvider = ReviewAppClientProvider._();

/// 店家的評價（ReviewAppService：自己店的評價列表與回覆），登入後呼叫。

final class ReviewAppClientProvider
    extends
        $FunctionalProvider<
          ReviewAppServiceClient,
          ReviewAppServiceClient,
          ReviewAppServiceClient
        >
    with $Provider<ReviewAppServiceClient> {
  /// 店家的評價（ReviewAppService：自己店的評價列表與回覆），登入後呼叫。
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

/// 商品評價的公開讀取（ReviewPublicService）；帶登入身分的 transport 與 [socialPublicClient] 相同理由。

@ProviderFor(reviewPublicClient)
final reviewPublicClientProvider = ReviewPublicClientProvider._();

/// 商品評價的公開讀取（ReviewPublicService）；帶登入身分的 transport 與 [socialPublicClient] 相同理由。

final class ReviewPublicClientProvider
    extends
        $FunctionalProvider<
          ReviewPublicServiceClient,
          ReviewPublicServiceClient,
          ReviewPublicServiceClient
        >
    with $Provider<ReviewPublicServiceClient> {
  /// 商品評價的公開讀取（ReviewPublicService）；帶登入身分的 transport 與 [socialPublicClient] 相同理由。
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
    r'af5bd0dcd8ae63aed245a3c02307d151d10a89ab';

/// 檢舉（ModerationAppService：店家檢舉聊天訊息、讀自己店送出的檢舉），登入後呼叫。

@ProviderFor(moderationAppClient)
final moderationAppClientProvider = ModerationAppClientProvider._();

/// 檢舉（ModerationAppService：店家檢舉聊天訊息、讀自己店送出的檢舉），登入後呼叫。

final class ModerationAppClientProvider
    extends
        $FunctionalProvider<
          ModerationAppServiceClient,
          ModerationAppServiceClient,
          ModerationAppServiceClient
        >
    with $Provider<ModerationAppServiceClient> {
  /// 檢舉（ModerationAppService：店家檢舉聊天訊息、讀自己店送出的檢舉），登入後呼叫。
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
    r'f7c8ded2ead753e4ca37959fdae594005b348fb5';

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 identity 的 session token；
/// unauthenticated 時登出）。

@ProviderFor(connectAuthTransport)
final connectAuthTransportProvider = ConnectAuthTransportProvider._();

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 identity 的 session token；
/// unauthenticated 時登出）。

final class ConnectAuthTransportProvider
    extends $FunctionalProvider<Transport, Transport, Transport>
    with $Provider<Transport> {
  /// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 identity 的 session token；
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
    r'4820eee5fe36a3892b936bfbd5cdffacc0c0c6ca';
