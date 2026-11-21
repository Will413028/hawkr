import 'dart:async';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/discovery/v1alpha1/discovery.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sentry_flutter/sentry_flutter.dart' show Sentry;

import '../../config/flavor.dart';
import '../../shared/utils/hk_log.dart';
import '../l10n/ui_locale.dart';
import '../session/account_restriction.dart';
import '../session/auth_session_provider.dart';
import '../session/force_logout_provider.dart';
import '../session/session_provider.dart';
import 'app_version_headers.dart';
import 'suspension_signal.dart';

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
  interceptors: connectInterceptors(acceptLanguage: uiLocaleTag(ref)),
);

/// 登入後的 service 的 transport：一般呼叫的 interceptor 加上身分（同 Dio 的 `AuthInterceptor`：帶 Bearer，
/// unauthenticated 時強制登出）與停權的處理（`ACCOUNT_SUSPENDED` 進受限狀態，不登出）。
@Riverpod(keepAlive: true)
Transport connectAuthTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(
    acceptLanguage: uiLocaleTag(ref),
    auth: _chain(suspensionInterceptor(ref), sessionAuthInterceptor(ref)),
  ),
);

/// 公開的 service 但伺服器依身分決定行為的呼叫（例如 RecordProductView）：有 session 就帶 Bearer，沒有（訪客）
/// 或換發失敗就不帶，不中斷呼叫。
@Riverpod(keepAlive: true)
Transport connectOptionalAuthTransport(Ref ref) => apiTransport(
  baseUrl: F.appFlavor.gatewayHost,
  httpClient: ref.watch(connectHttpClientProvider),
  interceptors: connectInterceptors(
    acceptLanguage: uiLocaleTag(ref),
    auth: optionalSessionInterceptor(ref),
  ),
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

/// 貼文、留言與討論區的公開讀取：有 session 時帶 Bearer，伺服器才填 `viewer_*`（讚過、能不能留言與刪除、留言身分）；
/// 訪客不帶。RecordPostView 也走這裡。
@Riverpod(keepAlive: true)
SocialPublicServiceClient socialPublicClient(Ref ref) =>
    SocialPublicServiceClient(ref.watch(connectOptionalAuthTransportProvider));

/// 按讚、留言與討論區發言的寫入：登入後才能呼叫，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。
@Riverpod(keepAlive: true)
SocialAppServiceClient socialAppClient(Ref ref) =>
    SocialAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 買家本人的封鎖與封鎖名單（`BuyerAppService`）：使用者主動的寫入，`ACCOUNT_SUSPENDED` 照使用者主動的寫入處理。
@Riverpod(keepAlive: true)
BuyerAppServiceClient buyerAppClient(Ref ref) =>
    BuyerAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 檢舉店家（`SellerReportAppService`）。
@Riverpod(keepAlive: true)
SellerReportAppServiceClient sellerReportAppClient(Ref ref) =>
    SellerReportAppServiceClient(ref.watch(connectAuthTransportProvider));

/// 檢舉社群內容與讀自己送出的檢舉（`ModerationAppService`）。
@Riverpod(keepAlive: true)
ModerationAppServiceClient moderationAppClient(Ref ref) =>
    ModerationAppServiceClient(ref.watch(connectAuthTransportProvider));

@Riverpod(keepAlive: true)
DiscoveryPublicServiceClient discoveryPublicClient(Ref ref) =>
    DiscoveryPublicServiceClient(ref.watch(connectTransportProvider));

/// 登入的帳號（GetSession）。登入（`IdentityPublicService`）與 session token 的保存見 `authSessionProvider`。
@Riverpod(keepAlive: true)
IdentityAppServiceClient identityClient(Ref ref) => IdentityAppServiceClient(
  apiTransport(
    baseUrl: F.appFlavor.gatewayHost,
    httpClient: ref.watch(connectHttpClientProvider),
    interceptors: connectInterceptors(
      acceptLanguage: uiLocaleTag(ref),
      auth: sessionBootstrapInterceptor(ref),
    ),
  ),
);

/// 套件的 [appInterceptors] 加上這支 App 的設定：逾時照 flavor、Sentry、X-App-* header、debug 時的 log。
List<Interceptor> connectInterceptors({
  Interceptor? auth,
  required String Function() acceptLanguage,
}) => appInterceptors(
  timeout: F.appFlavor.apiTimeout,
  report: _captureWithSentry,
  headers: () => AppVersionHeaders.headers,
  auth: auth,
  acceptLanguage: acceptLanguage,
  log: kDebugMode || F.appFlavor.enableApiLogging ? HKLog.logD : null,
);

/// 以 identity 的 session token 提供身分；session 失效時強制登出（同 Dio 的 `AuthInterceptor`）。
Interceptor sessionAuthInterceptor(Ref ref) => authInterceptor(
  sessionToken: () => currentSessionToken(ref),
  onSessionInvalid: () {
    // 訪客（從未登入、沒有 session）不彈「登入已過期」（同 AuthInterceptor）。
    if (ref.read(sessionProvider).value == null) return;
    ref.read(forceLogoutProvider.notifier).trigger();
  },
);

/// 收到 `ACCOUNT_SUSPENDED` 時：背景寫入（[isBackgroundConnectProcedure]）靜默略過，其餘視為使用者主動的寫入，
/// 進受限狀態並提示。錯誤原樣丟出，不登出。
Interceptor suspensionInterceptor(Ref ref) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      try {
        return await next(req);
      } catch (raw) {
        if (isAccountSuspended(ConnectException.from(raw)) &&
            !isBackgroundConnectProcedure(req.spec.procedure) &&
            ref.mounted) {
          ref.read(accountRestrictionProvider.notifier).rejected();
        }
        rethrow;
      }
    };
  };
}

Interceptor _chain(Interceptor outer, Interceptor inner) {
  return <I extends Object, O extends Object>(next) =>
      outer<I, O>(inner<I, O>(next));
}

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
