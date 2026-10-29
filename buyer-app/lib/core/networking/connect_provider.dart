import 'dart:async';

import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/discovery/v1alpha1/discovery.connect.client.dart';
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
import '../session/force_logout_provider.dart';
import '../session/session_provider.dart';
import 'app_version_headers.dart';

part 'connect_provider.g.dart';

// 對外 API（根目錄 api/ 的 Protobuf＋Connect）的 client。已經搬進單體的模組走這裡，其他模組仍走 Dio，
// 試點期間兩套並存。公開的 service（*PublicService）用 [connectTransport]，不帶身分；登入後的 service
// （*AppService）用 [connectAuthTransport]。

/// 所有 Connect transport 共用的 HTTP/2 連線。
@Riverpod(keepAlive: true)
HttpClient connectHttpClient(Ref ref) => apiHttpClient();

/// 一般呼叫的 transport：逾時照 flavor 的設定，依錯誤模型 D7 重試，伺服器端的失敗上報 Sentry，帶 X-App-* header
/// （`headers`，只有版本與 App 類型）。
@Riverpod(keepAlive: true)
Transport connectTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(),
);

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
/// unauthenticated 時強制登出）。
@Riverpod(keepAlive: true)
Transport connectAuthTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(auth: sessionAuthInterceptor(ref)),
);

/// 公開的 service 但伺服器依身分決定行為的呼叫（例如 RecordProductView）：有 session 就帶 Bearer，沒有（訪客）
/// 或換發失敗就不帶，不中斷呼叫。
@Riverpod(keepAlive: true)
Transport connectOptionalAuthTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(auth: optionalSessionInterceptor(ref)),
);

@Riverpod(keepAlive: true)
ContentPublicServiceClient contentClient(Ref ref) =>
    ContentPublicServiceClient(ref.watch(connectTransportProvider));

@Riverpod(keepAlive: true)
SellerPublicServiceClient sellerPublicClient(Ref ref) =>
    SellerPublicServiceClient(ref.watch(connectTransportProvider));

/// RecordProductView：登入與否由伺服器判斷，所以有 session 時帶 Bearer。
@Riverpod(keepAlive: true)
SellerPublicServiceClient sellerPublicOptionalAuthClient(Ref ref) =>
    SellerPublicServiceClient(ref.watch(connectOptionalAuthTransportProvider));

@Riverpod(keepAlive: true)
DiscoveryPublicServiceClient discoveryPublicClient(Ref ref) =>
    DiscoveryPublicServiceClient(ref.watch(connectTransportProvider));

/// 登入的帳號（GetSession）。登入與登出由 Clerk 處理（`clerkSessionProvider`）。
@Riverpod(keepAlive: true)
IdentityAppServiceClient identityClient(Ref ref) => IdentityAppServiceClient(
  apiTransport(
    baseUrl: F.appFlavor.gatewayHost,
    httpClient: ref.watch(connectHttpClientProvider),
    interceptors: connectInterceptors(auth: sessionBootstrapInterceptor(ref)),
  ),
);

/// 套件的 [appInterceptors] 加上這支 App 的設定：逾時照 flavor、Sentry、X-App-* header、debug 時的 log。
List<Interceptor> connectInterceptors({Interceptor? auth}) => appInterceptors(
  timeout: F.appFlavor.apiTimeout,
  report: _captureWithSentry,
  headers: () => AppVersionHeaders.headers,
  auth: auth,
  log: kDebugMode || F.appFlavor.enableApiLogging ? HKLog.logD : null,
);

/// 以 Clerk 的 session token 提供身分；session 失效時強制登出（同 Dio 的 `AuthInterceptor`）。
Interceptor sessionAuthInterceptor(Ref ref) => authInterceptor(
  sessionToken: () => currentSessionToken(ref),
  isTokenRefused: auth.isSessionTokenRefused,
  onSessionInvalid: () {
    // 訪客（從未登入、沒有 session）不彈「登入已過期」（同 AuthInterceptor）。
    if (ref.read(sessionProvider).value == null) return;
    ref.read(forceLogoutProvider.notifier).trigger();
  },
);

Interceptor optionalSessionInterceptor(Ref ref) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      String? token;
      try {
        token = await currentSessionToken(ref);
      } catch (_) {
        token = null;
      }
      if (token != null && token.isNotEmpty) {
        req.headers['authorization'] = 'Bearer $token';
      }
      return next(req);
    };
  };
}

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
