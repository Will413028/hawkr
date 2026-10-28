import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sentry_dio/sentry_dio.dart';
import '../../config/flavor.dart';
import '../../shared/utils/hk_log.dart';
import 'app_version_headers.dart';
import 'auth_interceptor.dart';

part 'dio_provider.g.dart';

class _HKLogInterceptor extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    final buf = StringBuffer()
      ..writeln('┌── REQUEST ──────────────────────────────────────')
      ..writeln('│ ${options.method} ${options.uri}');
    if (options.queryParameters.isNotEmpty) {
      buf.writeln('│ Query: ${options.queryParameters}');
    }
    if (options.data != null) {
      buf.writeln('│ Body:  ${options.data}');
    }
    buf.write('└─────────────────────────────────────────────────');
    HKLog.logD(buf.toString());
    super.onRequest(options, handler);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    final req = response.requestOptions;
    final traceId = response.headers.value('x-trace-id');
    final traceSuffix = traceId != null ? ' [trace:$traceId]' : '';
    final buf = StringBuffer()
      ..writeln(
        '┌── RESPONSE ${response.statusCode} ─────────────────────────────',
      )
      ..writeln('│ ${req.method} ${req.uri}$traceSuffix')
      ..writeln('│ Data: ${response.data}')
      ..write('└─────────────────────────────────────────────────');
    HKLog.logD(buf.toString());
    super.onResponse(response, handler);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final req = err.requestOptions;
    final status = err.response?.statusCode;
    final traceId = err.response?.headers.value('x-trace-id');
    final traceSuffix = traceId != null ? ' [trace:$traceId]' : '';
    final buf = StringBuffer()
      ..writeln(
        '┌── ERROR ${status ?? err.type.name} ────────────────────────────',
      )
      ..writeln('│ ${req.method} ${req.uri}$traceSuffix')
      ..writeln('│ Message: ${err.message}');
    if (err.response?.data != null) {
      buf.writeln('│ Body:    ${err.response?.data}');
    }
    buf.write('└─────────────────────────────────────────────────');
    HKLog.logW(buf.toString(), status == null ? err : null);
    super.onError(err, handler);
  }
}

@Riverpod(keepAlive: true)
Dio dio(Ref ref) {
  final d = Dio(
    BaseOptions(
      baseUrl: F.appFlavor.apiBaseUrl,
      connectTimeout: const Duration(seconds: 30),
      receiveTimeout: const Duration(seconds: 30),
      // X-App-* 三欄供 Gateway access log 統計版本佔比（`packages/hawkr_app_update/docs/spec.md`）。
      headers: {'Accept': 'application/json', ...AppVersionHeaders.headers},
    ),
  );
  d.addSentry();
  d.interceptors.add(ref.watch(authInterceptorProvider));
  if (kDebugMode || F.appFlavor.enableApiLogging) {
    d.interceptors.add(_HKLogInterceptor());
  }
  return d;
}
