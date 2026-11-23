import 'dart:async';

import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart'
    show SellerAppService;
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sentry_flutter/sentry_flutter.dart' show Sentry;

import '../../config/flavor.dart';
import '../../shared/utils/hk_log.dart';
import '../l10n/ui_locale.dart';
import '../session/auth_session_provider.dart';
import '../session/session_provider.dart';
import 'app_version_headers.dart';

part 'connect_provider.g.dart';

// 對外 API（根目錄 api/ 的 Protobuf＋Connect）的 client。已經搬進單體的模組走這裡，其他模組仍走 Dio，
// 試點期間兩套並存。公開的 service（*PublicService）用 [connectTransport]，不帶身分；登入後的 service
// （*AppService）用 [connectAuthTransport]，收到 `SELLER_SUSPENDED` 時重讀 `GetMyStore`（`onSellerSuspendedSignal`）。

/// 所有 Connect transport 共用的 HTTP/2 連線。
@Riverpod(keepAlive: true)
HttpClient connectHttpClient(Ref ref) => apiHttpClient();

/// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header。
@Riverpod(keepAlive: true)
Transport connectTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(acceptLanguage: uiLocaleTag(ref)),
);

@Riverpod(keepAlive: true)
ContentPublicServiceClient contentClient(Ref ref) =>
    ContentPublicServiceClient(ref.watch(connectTransportProvider));

/// 登入的帳號（GetSession）。登入（`IdentityPublicService`）與 session token 的保存見 `authSessionProvider`。
@Riverpod(keepAlive: true)
IdentityAppServiceClient identityClient(Ref ref) =>
    IdentityAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 賣家的店家、入駐與 KYC（SellerAppService），登入後呼叫。
@Riverpod(keepAlive: true)
SellerAppServiceClient sellerClient(Ref ref) =>
    SellerAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 貼文與留言的互動寫入與店主的貼文管理（SocialAppService），登入後呼叫。
@Riverpod(keepAlive: true)
SocialAppServiceClient socialClient(Ref ref) =>
    SocialAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 貼文與留言、店家討論區的讀取（SocialPublicService）。帶登入身分的 transport：店主讀自己的貼文留言與討論區
/// 不受公開狀態限制，伺服器依憑證判斷。
@Riverpod(keepAlive: true)
SocialPublicServiceClient socialPublicClient(Ref ref) =>
    SocialPublicServiceClient(ref.watch(connectAuthTransportProvider));

/// 店家側的聊天（ChatAppService 的 `*Store*` 方法與對話上的方法），登入後呼叫；店家由伺服器依呼叫者此刻的店推導。
@Riverpod(keepAlive: true)
ChatAppServiceClient chatClient(Ref ref) =>
    ChatAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 店家的客戶與標籤（CrmAppService），登入後呼叫。
@Riverpod(keepAlive: true)
CrmAppServiceClient crmClient(Ref ref) =>
    CrmAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 店家的評價（ReviewAppService：自己店的評價列表與回覆），登入後呼叫。
@Riverpod(keepAlive: true)
ReviewAppServiceClient reviewAppClient(Ref ref) =>
    ReviewAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 商品評價的公開讀取（ReviewPublicService）；帶登入身分的 transport 與 [socialPublicClient] 相同理由。
@Riverpod(keepAlive: true)
ReviewPublicServiceClient reviewPublicClient(Ref ref) =>
    ReviewPublicServiceClient(ref.watch(connectAuthTransportProvider));

/// 檢舉（ModerationAppService：店家檢舉聊天訊息、讀自己店送出的檢舉），登入後呼叫。
@Riverpod(keepAlive: true)
ModerationAppServiceClient moderationAppClient(Ref ref) =>
    ModerationAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 賣家入駐文件的現行版本（SellerPublicService），不需登入。
@Riverpod(keepAlive: true)
SellerPublicServiceClient sellerPublicClient(Ref ref) =>
    SellerPublicServiceClient(ref.watch(connectTransportProvider));

@Riverpod(keepAlive: true)
IdentityAppServiceClient sessionIdentityClient(Ref ref) =>
    IdentityAppServiceClient(
      apiTransport(
        baseUrl: F.appFlavor.gatewayHost,
        httpClient: ref.watch(connectHttpClientProvider),
        interceptors: connectInterceptors(
          acceptLanguage: uiLocaleTag(ref),
          auth: sessionBootstrapInterceptor(ref),
        ),
      ),
    );

/// 使用者沒有主動操作的背景寫入（App 自動呼叫）：帳號停權時收到 `ACCOUNT_SUSPENDED` 靜默略過，不轉到受限頁。
/// 範圍是這支 App 經 Connect 呼叫、宣告為寫入（`customerEffects`）的 AppService 方法。
final Set<String> backgroundWriteProcedures = {
  SellerAppService.refreshMyPaymentAccount.procedure,
};

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 identity 的 session token；
/// unauthenticated 時登出）。
@Riverpod(keepAlive: true)
Transport connectAuthTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(
    acceptLanguage: uiLocaleTag(ref),
    auth: sessionAuthInterceptor(ref),
    onSellerSuspended: () =>
        ref.read(sessionProvider.notifier).onSellerSuspendedSignal(),
    onAccountSuspended: () => unawaited(
      ref.read(sessionProvider.notifier).onAccountSuspendedSignal(),
    ),
  ),
);

/// 套件的 [appInterceptors] 加上這支 App 的設定：逾時照 flavor、Sentry、X-App-* header、debug 時的
/// log。登入後的 service 另外帶 [onSellerSuspended]（D10 的即時訊號）。
List<Interceptor> connectInterceptors({
  Interceptor? auth,
  required String Function() acceptLanguage,
  void Function()? onSellerSuspended,
  void Function()? onAccountSuspended,
}) => [
  ...appInterceptors(
    timeout: Duration(milliseconds: F.appFlavor.apiTimeout),
    report: _captureWithSentry,
    headers: appConnectHeaders,
    auth: auth,
    acceptLanguage: acceptLanguage,
    onSellerSuspended: onSellerSuspended,
    onAccountSuspended: onAccountSuspended,
    accountSuspendedSilent: backgroundWriteProcedures,
    log: kDebugMode || F.appFlavor.enableApiLogging ? HKLog.logD : null,
  ),
];

/// 以 identity 的 session token 提供身分；session 失效時登出。
Interceptor sessionAuthInterceptor(Ref ref) => authInterceptor(
  sessionToken: () => currentSessionToken(ref),
  onSessionInvalid: () {
    if (!ref.read(sessionProvider).isLoggedIn) return;
    unawaited(ref.read(sessionProvider.notifier).logout());
  },
);

Interceptor sessionBootstrapInterceptor(Ref ref) => authInterceptor(
  sessionToken: () => auth.currentSessionToken(
    ref,
    session: ref.read(authSessionProvider.future),
  ),
  onSessionInvalid: () {},
);

void _captureWithSentry(ConnectException error, FailureContext context) {
  unawaited(
    Sentry.captureException(
      error,
      withScope: (scope) async {
        scope.fingerprint = [
          'api-connect',
          context.procedure,
          context.code.name,
        ];
        await scope.setTag('api_endpoint', context.procedure);
        await scope.setTag('connect_code', context.code.name);
        await scope.setTag('trace_id', context.traceId);
      },
    ),
  );
}
