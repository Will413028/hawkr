import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sentry_dio/sentry_dio.dart';

import '../../config/flavor.dart';
import '../../shared/utils/secret_redaction.dart';
import 'app_version_headers.dart';
import '../exceptions/seller_suspension_codes.dart';
import '../session/auth_session_provider.dart';
import '../session/session_provider.dart';
import '../../shared/utils/hk_log.dart';

part 'dio_provider.g.dart';

/// `Options(extra: {...})` 旗標：帶此旗標的請求，401 時不登出。
///
/// 用於「本來就不預期帶得出有效憑證」的請求——例如登出流程中的裝置註銷。
/// 那種 401 是預期結果，不代表 session 需要被回收；若照常登出，logout 又會
/// 再發一次同樣的請求，形成自我循環。
const String kSkipAuthInterceptorExtra = 'skipAuthInterceptor';

/// 標記使用者沒有主動操作的背景寫入（裝置註冊、已讀回報）：帳號停權時收到 `ACCOUNT_SUSPENDED` 靜默略過，不轉到受限頁。
/// 帶 [kSkipAuthInterceptorExtra] 的請求（登出時的註銷裝置）同樣靜默。
const String kBackgroundWriteExtra = 'backgroundWrite';

/// Dio instance 的 Riverpod Provider
///
/// 新的 feature repository 應透過 `ref.watch(dioProvider)` 取得 Dio instance，
/// 而非直接使用 `DioManager.instance.dio`。
///
/// Interceptor 順序：
/// 1. Auth interceptor — 注入 identity 的 session token（[sessionTokenInterceptor]）
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
      headers: {'Content-Type': 'application/json', ...appVersionHeaders()},
    ),
  );

  dio.addSentry();

  dio.interceptors.add(sessionTokenInterceptor(() => currentSessionToken(ref)));

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


  void handleAccountSuspended(
    int? status,
    RequestOptions options,
    dynamic data,
  ) {
    if (status != 403) return;
    if (options.extra[kBackgroundWriteExtra] == true ||
        options.extra[kSkipAuthInterceptorExtra] == true) {
      return;
    }
    if (restEnvelopeReason(data) != accountSuspendedReason) return;
    unawaited(ref.read(sessionProvider.notifier).onAccountSuspendedSignal());
  }

  bool shouldLogout(int? status, RequestOptions options) =>
      status == 401 && options.extra[kSkipAuthInterceptorExtra] != true;

  dio.interceptors.add(
    InterceptorsWrapper(
      onResponse: (response, handler) async {
        handleAccountSuspended(
          response.statusCode,
          response.requestOptions,
          response.data,
        );
        if (shouldLogout(response.statusCode, response.requestOptions)) {
          await ref.read(sessionProvider.notifier).logout();
        }
        return handler.next(response);
      },
      onError: (error, handler) async {
        handleAccountSuspended(
          error.response?.statusCode,
          error.requestOptions,
          error.response?.data,
        );
        if (shouldLogout(error.response?.statusCode, error.requestOptions)) {
          await ref.read(sessionProvider.notifier).logout();
        }
        return handler.next(error);
      },
    ),
  );

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

/// 每個請求帶 [sessionToken] 給的 token（`Authorization: Bearer`，沒有時不帶）。讀不到 token 時不送出，以連線錯誤結束。
Interceptor sessionTokenInterceptor(Future<String?> Function() sessionToken) {
  return InterceptorsWrapper(
    onRequest: (options, handler) async {
      final String? token;
      try {
        token = await sessionToken();
      } catch (e) {
        return handler.reject(
          DioException(
            requestOptions: options,
            type: DioExceptionType.connectionError,
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
  final payload = _decodeEnvelope(data);
  if (payload == null) return false;
  return payload['code']?.toString() ==
      SellerSuspensionCode.sellerSuspendedOperationBlocked.toString();
}

/// REST 的錯誤 envelope 的 `reason`（`ErrorReason` 的名稱字串）；沒有時是 null。
String? restEnvelopeReason(dynamic data) {
  final reason = _decodeEnvelope(data)?['reason'];
  return reason is String ? reason : null;
}

/// 伺服器回報帳號停權的 `ErrorReason` 名稱。
const String accountSuspendedReason = 'ACCOUNT_SUSPENDED';

Map? _decodeEnvelope(dynamic data) {
  dynamic payload = data;
  try {
    if (payload is List<int>) {
      payload = jsonDecode(utf8.decode(payload));
    } else if (payload is String) {
      payload = jsonDecode(payload);
    }
  } catch (_) {
    return null;
  }
  return payload is Map ? payload : null;
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
