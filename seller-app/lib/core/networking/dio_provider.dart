import 'package:hawkr_auth/clerk.dart' show isSessionTokenRefused;
import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sentry_dio/sentry_dio.dart';

import '../../config/flavor.dart';
import '../../shared/utils/secret_redaction.dart';
import 'app_version_headers.dart';
import '../exceptions/seller_suspension_codes.dart';
import '../session/clerk_session_provider.dart';
import '../session/session_provider.dart';
import '../../shared/utils/hk_log.dart';

part 'dio_provider.g.dart';

/// `Options(extra: {...})` 旗標：帶此旗標的請求，401 時不登出。
///
/// 用於「本來就不預期帶得出有效憑證」的請求——例如登出流程中的裝置註銷。
/// 那種 401 是預期結果，不代表 session 需要被回收；若照常登出，logout 又會
/// 再發一次同樣的請求，形成自我循環。
const String kSkipAuthInterceptorExtra = 'skipAuthInterceptor';

/// Dio instance 的 Riverpod Provider
///
/// 新的 feature repository 應透過 `ref.watch(dioProvider)` 取得 Dio instance，
/// 而非直接使用 `DioManager.instance.dio`。
///
/// Interceptor 順序：
/// 1. Auth interceptor — 注入 Clerk 的 session token（[sessionTokenInterceptor]）
/// 2. Logging interceptor — 統一日誌輸出
/// 3. Unauthorized interceptor — 401 登出
/// 4. Suspension interceptor — 9240 全域偵測（46 §7.2）
@Riverpod(keepAlive: true)
Dio dio(Ref ref) {
  final dio = Dio(
    BaseOptions(
      baseUrl: F.appFlavor.gatewayHost,
      connectTimeout: Duration(milliseconds: F.appFlavor.apiTimeout),
      receiveTimeout: Duration(milliseconds: F.appFlavor.apiTimeout),
      // X-App-*（可觀測性）與 X-Active-Role（視角），都不是授權依據。
      headers: {'Content-Type': 'application/json', ...appVersionHeaders()},
    ),
  );

  dio.addSentry();

  // ─── 1. Auth Interceptor ───
  dio.interceptors.add(
    sessionTokenInterceptor(
      () => currentSessionToken(ref),
      onSessionInvalid: () {
        if (ref.read(sessionProvider).isLoggedIn) {
          unawaited(ref.read(sessionProvider.notifier).logout());
        }
      },
    ),
  );

  // ─── 2. Logging Interceptor ───
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        options.extra['_logBuffer'] = StringBuffer();
        final logBuffer = options.extra['_logBuffer'] as StringBuffer;

        logBuffer.writeln('📤 Request:');
        logBuffer.writeln(
          '  ${options.method} ${redactSensitiveUri(options.uri)}',
        );
        if (options.headers.isNotEmpty) {
          logBuffer.writeln('  Headers:');
          options.headers.forEach((key, value) {
            final safeValue = isSensitiveKey(key)
                ? redactedSecret
                : redactSensitiveData(value);
            logBuffer.writeln('    $key: $safeValue');
          });
        }
        if (options.data != null) {
          logBuffer.writeln('  Body: ${redactSensitiveData(options.data)}');
        }
        if (options.queryParameters.isNotEmpty) {
          logBuffer.writeln(
            '  Query: ${redactSensitiveData(options.queryParameters)}',
          );
        }

        return handler.next(options);
      },
      onResponse: (response, handler) {
        final logBuffer =
            response.requestOptions.extra['_logBuffer'] as StringBuffer?;

        if (logBuffer != null) {
          final traceId = response.headers.value('x-trace-id');
          logBuffer.writeln('📥 Response:');
          if (traceId != null) {
            logBuffer.writeln('  TraceID: $traceId');
          }
          logBuffer.writeln(
            '  Status: ${response.statusCode} ${response.statusMessage}',
          );
          if (response.data != null) {
            // Binary 響應（例如 ResponseType.bytes 下載圖片）直接印長度，
            // 不要嘗試 JSON encode — 否則會變成 [137, 80, 78, ...] 的 byte array。
            if (response.requestOptions.responseType == ResponseType.bytes ||
                response.data is List<int>) {
              final len = response.data is List
                  ? (response.data as List).length
                  : 0;
              logBuffer.writeln('  Body: <binary $len bytes>');
            } else {
              final prettyJson = _prettyJson(
                redactSensitiveData(response.data),
              );
              if (prettyJson.length > 2000) {
                logBuffer.writeln(
                  '  Body:\n${prettyJson.substring(0, 2000)}... (truncated)',
                );
              } else {
                logBuffer.writeln('  Body:\n$prettyJson');
              }
            }
          }
          HKLog.logD(logBuffer.toString());
        }

        return handler.next(response);
      },
      onError: (error, handler) {
        final logBuffer =
            error.requestOptions.extra['_logBuffer'] as StringBuffer?;

        if (logBuffer != null) {
          final traceId = error.response?.headers.value('x-trace-id');
          logBuffer.writeln('❌ Error:');
          if (traceId != null) {
            logBuffer.writeln('  TraceID: $traceId');
          }
          logBuffer.writeln('  Type: ${error.type}');
          logBuffer.writeln(
            '  Message: ${redactSensitiveText(error.message ?? '')}',
          );
          if (error.response != null) {
            logBuffer.writeln(
              '  Status: ${error.response?.statusCode} ${error.response?.statusMessage}',
            );
            if (error.response?.data != null) {
              final errorData = redactSensitiveData(
                error.response!.data,
              ).toString();
              if (errorData.length > 1000) {
                logBuffer.writeln(
                  '  Body: ${errorData.substring(0, 1000)}... (truncated)',
                );
              } else {
                logBuffer.writeln('  Body: $errorData');
              }
            }
          }
          HKLog.logD(logBuffer.toString());
        }

        return handler.next(error);
      },
    ),
  );

  // ─── 3. Unauthorized Interceptor ───────────────────────────────────────────
  //
  // token 由 Clerk 的 SDK 換新（ADR 2026-09-27-identity-clerk）：401 代表 Clerk 的
  // session 已結束，直接登出，由 router redirect 回到登入頁。

  bool shouldLogout(int? status, RequestOptions options) =>
      status == 401 && options.extra[kSkipAuthInterceptorExtra] != true;

  dio.interceptors.add(
    InterceptorsWrapper(
      // 處理 validateStatus: (_) => true 的情況（ApiRequestHelper 預設）
      onResponse: (response, handler) async {
        if (shouldLogout(response.statusCode, response.requestOptions)) {
          await ref.read(sessionProvider.notifier).logout();
        }
        return handler.next(response);
      },
      // 處理預設 validateStatus（Dio 對 401 拋出 DioException）的情況
      onError: (error, handler) async {
        if (shouldLogout(error.response?.statusCode, error.requestOptions)) {
          await ref.read(sessionProvider.notifier).logout();
        }
        return handler.next(error);
      },
    ),
  );

  // ─── 4. Suspension Interceptor（46 §7.2）───
  //
  // 賣家帳號停權時，能力層 deny 一律回 envelope code=9240（HTTP 409，已登錄
  // Gateway bizCodePassthrough，非 message `[NNNN]` 前綴 fallback）。
  // ApiRequestHelper 走 `validateStatus:(_)=>true`，409 落在 onResponse；
  // 未走該 helper 的直接 dio 呼叫（預設 validateStatus）409 會拋 DioException
  // 落在 onError——比照上面 401 攔截器，兩條路徑都要攔，否則會漏判。
  void handleSuspensionSignal(dynamic data) {
    if (!isSellerSuspensionPayload(data)) return;
    ref.read(sessionProvider.notifier).onSellerSuspendedSignal();
  }

  dio.interceptors.add(
    InterceptorsWrapper(
      onResponse: (response, handler) {
        handleSuspensionSignal(response.data);
        return handler.next(response);
      },
      onError: (error, handler) {
        handleSuspensionSignal(error.response?.data);
        return handler.next(error);
      },
    ),
  );

  return dio;
}

/// 每個請求帶 [sessionToken] 給的 token（`Authorization: Bearer`，沒有時不帶）。取不到 token（連不上 Clerk）
/// 時不送出，以連線錯誤結束。
Interceptor sessionTokenInterceptor(
  Future<String?> Function() sessionToken, {
  void Function()? onSessionInvalid,
}) {
  return InterceptorsWrapper(
    onRequest: (options, handler) async {
      final String? token;
      try {
        token = await sessionToken();
      } catch (e) {
        final refused = isSessionTokenRefused(e);
        if (refused) onSessionInvalid?.call();
        return handler.reject(
          DioException(
            requestOptions: options,
            type: refused
                ? DioExceptionType.cancel
                : DioExceptionType.connectionError,
            error: e,
            message: 'session token unavailable',
          ),
        );
      }
      if (token != null && token.isNotEmpty) {
        options.headers['Authorization'] = 'Bearer $token';
      }
      return handler.next(options);
    },
  );
}

/// Detects the suspension envelope for JSON, decoded maps, and byte responses.
///
/// Binary endpoints request [ResponseType.bytes], so a 9240 JSON error also
/// arrives as bytes. Keeping this normalization in the central interceptor
/// prevents those protected calls from bypassing the global suspension policy.
bool isSellerSuspensionPayload(dynamic data) {
  dynamic payload = data;
  if (payload is List<int>) {
    try {
      payload = jsonDecode(utf8.decode(payload));
    } catch (_) {
      return false;
    }
  } else if (payload is String) {
    try {
      payload = jsonDecode(payload);
    } catch (_) {
      return false;
    }
  }
  if (payload is! Map) return false;
  return payload['code']?.toString() ==
      SellerSuspensionCode.sellerSuspendedOperationBlocked.toString();
}

/// 美化 JSON 輸出
String _prettyJson(dynamic data) {
  try {
    const encoder = JsonEncoder.withIndent('  ');
    return encoder.convert(data);
  } catch (_) {
    return data.toString();
  }
}
