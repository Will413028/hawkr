import 'dart:async';

import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sentry_flutter/sentry_flutter.dart' show Sentry;

import '../../config/flavor.dart';
import '../../shared/utils/hk_log.dart';
import '../session/clerk_session_provider.dart';
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
  interceptors: connectInterceptors(),
);

@Riverpod(keepAlive: true)
ContentPublicServiceClient contentClient(Ref ref) =>
    ContentPublicServiceClient(ref.watch(connectTransportProvider));

/// 登入的帳號（GetSession）與刪帳前撤銷 Apple 的授權。登入與登出由 Clerk 處理（`clerkSessionProvider`）。
@Riverpod(keepAlive: true)
IdentityAppServiceClient identityClient(Ref ref) =>
    IdentityAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 賣家的店家、入駐與 KYC（SellerAppService），登入後呼叫。
@Riverpod(keepAlive: true)
SellerAppServiceClient sellerClient(Ref ref) =>
    SellerAppServiceClient(ref.watch(connectAuthTransportProvider));

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
          auth: sessionBootstrapInterceptor(ref),
        ),
      ),
    );

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（帶 Clerk 的 session token；
/// unauthenticated 時登出）。
@Riverpod(keepAlive: true)
Transport connectAuthTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(
    auth: sessionAuthInterceptor(ref),
    onSellerSuspended: () =>
        ref.read(sessionProvider.notifier).onSellerSuspendedSignal(),
  ),
);

/// 套件的 [appInterceptors] 加上這支 App 的設定：逾時照 flavor、Sentry、X-App-* header、debug 時的
/// log。登入後的 service 另外帶 [onSellerSuspended]（D10 的即時訊號）。
List<Interceptor> connectInterceptors({
  Interceptor? auth,
  void Function()? onSellerSuspended,
}) => appInterceptors(
  timeout: Duration(milliseconds: F.appFlavor.apiTimeout),
  report: _captureWithSentry,
  headers: appConnectHeaders,
  auth: auth,
  onSellerSuspended: onSellerSuspended,
  log: kDebugMode || F.appFlavor.enableApiLogging ? HKLog.logD : null,
);

/// 以 Clerk 的 session token 提供身分；session 失效時登出。
Interceptor sessionAuthInterceptor(Ref ref) => authInterceptor(
  sessionToken: () => currentSessionToken(ref),
  isTokenRefused: auth.isSessionTokenRefused,
  onSessionInvalid: () {
    if (!ref.read(sessionProvider).isLoggedIn) return;
    unawaited(ref.read(sessionProvider.notifier).logout());
  },
);

Interceptor sessionBootstrapInterceptor(Ref ref) => authInterceptor(
  sessionToken: () => auth.currentSessionToken(ref),
  isTokenRefused: auth.isSessionTokenRefused,
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
