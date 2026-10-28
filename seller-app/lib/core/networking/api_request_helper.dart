import 'package:dio/dio.dart';

import '../exceptions/app_exception.dart';
import 'api_result.dart';

/// Repository 共用的 API 請求 mixin
///
/// 提供 `apiGet` / `apiPost` / `apiPut` / `apiPatch` / `apiDelete` 方法，
/// 行為與原 ApiService 的 `_get` / `_post` 等一致：
/// - 使用 `validateStatus: (_) => true` 避免 Dio 拋出例外
/// - 統一解析回應為 `Map<String, dynamic>`
/// - 網路錯誤時回傳 `{'code': -1, 'message': '...'}`
///
/// 使用方式：
/// ```dart
/// class AuthRepository with ApiRequestHelper {
///   AuthRepository(this._dio);
///   final Dio _dio;
///   @override
///   Dio get dio => _dio;
/// }
/// ```
mixin ApiRequestHelper {
  Dio get dio;

  static final _opts = Options(validateStatus: (_) => true);

  Future<Map<String, dynamic>> apiGet(
    String path, {
    Map<String, String>? queryParams,
    Duration? timeout,
  }) async {
    try {
      final resp = await dio.get(
        path,
        queryParameters: queryParams,
        options: _opts.copyWith(receiveTimeout: timeout),
      );
      return _parse(resp);
    } catch (e) {
      return {'code': -1, 'message': e.toString()};
    }
  }

  Future<Map<String, dynamic>> apiPost(
    String path, {
    Map<String, dynamic>? body,
    Object? rawBody,
    Duration? timeout,
    Options? options,
  }) async {
    try {
      final resp = await dio.post(
        path,
        data: rawBody ?? body,
        options: options ?? _opts.copyWith(receiveTimeout: timeout),
      );
      return _parse(resp);
    } catch (e) {
      return {'code': -1, 'message': e.toString()};
    }
  }

  Future<Map<String, dynamic>> apiPut(
    String path, {
    Map<String, dynamic>? body,
    Duration? timeout,
  }) async {
    try {
      final resp = await dio.put(
        path,
        data: body,
        options: _opts.copyWith(receiveTimeout: timeout),
      );
      return _parse(resp);
    } catch (e) {
      return {'code': -1, 'message': e.toString()};
    }
  }

  Future<Map<String, dynamic>> apiPatch(
    String path, {
    Map<String, dynamic>? body,
    Duration? timeout,
  }) async {
    try {
      final resp = await dio.patch(
        path,
        data: body,
        options: _opts.copyWith(receiveTimeout: timeout),
      );
      return _parse(resp);
    } catch (e) {
      return {'code': -1, 'message': e.toString()};
    }
  }

  Future<Map<String, dynamic>> apiDelete(
    String path, {
    Map<String, dynamic>? body,
    Duration? timeout,
  }) async {
    try {
      final resp = await dio.delete(
        path,
        data: body,
        options: _opts.copyWith(receiveTimeout: timeout),
      );
      return _parse(resp);
    } catch (e) {
      return {'code': -1, 'message': e.toString()};
    }
  }

  /// Typed GET contract. Unlike [apiGet], transport, HTTP and business
  /// failures remain distinct and can never be mistaken for success — a caller
  /// that must fail open (App 版本 gate，`packages/hawkr_app_update/docs/spec.md`) has to tell a
  /// transport error apart from a legitimate `updateRequired: false`.
  Future<ApiResult<Map<String, dynamic>>> apiGetResult(
    String path, {
    Map<String, String>? queryParams,
    Duration? timeout,
    Options? options,
  }) {
    return _requestResult(
      () => dio.get(
        path,
        queryParameters: queryParams,
        options: options ?? _opts.copyWith(receiveTimeout: timeout),
      ),
    );
  }

  /// Typed POST contract for mutations. Unlike [apiPost], transport, HTTP and
  /// business failures remain distinct and can never be mistaken for success.
  Future<ApiResult<Map<String, dynamic>>> apiPostResult(
    String path, {
    Map<String, dynamic>? body,
    Object? rawBody,
    Duration? timeout,
    Options? options,
  }) {
    return _requestResult(
      () => dio.post(
        path,
        data: rawBody ?? body,
        options: options ?? _opts.copyWith(receiveTimeout: timeout),
      ),
    );
  }

  Future<Map<String, dynamic>> apiPostOrThrow(
    String path, {
    Map<String, dynamic>? body,
    Object? rawBody,
    Duration? timeout,
    Options? options,
  }) async {
    final result = await apiPostResult(
      path,
      body: body,
      rawBody: rawBody,
      timeout: timeout,
      options: options,
    );
    return result.valueOrThrow;
  }

  /// Typed DELETE contract for mutations.
  Future<ApiResult<Map<String, dynamic>>> apiDeleteResult(
    String path, {
    Map<String, dynamic>? body,
    Duration? timeout,
  }) {
    return _requestResult(
      () => dio.delete(
        path,
        data: body,
        options: _opts.copyWith(receiveTimeout: timeout),
      ),
    );
  }

  Future<Map<String, dynamic>> apiDeleteOrThrow(
    String path, {
    Map<String, dynamic>? body,
    Duration? timeout,
  }) async {
    final result = await apiDeleteResult(path, body: body, timeout: timeout);
    return result.valueOrThrow;
  }

  Future<ApiResult<Map<String, dynamic>>> _requestResult(
    Future<Response<dynamic>> Function() request,
  ) async {
    try {
      final response = await request();
      final statusCode = response.statusCode ?? -1;
      final parsed = _parse(response);
      final message = _messageFrom(
        parsed,
        fallback: statusCode >= 500
            ? '伺服器錯誤 ($statusCode)，請稍後再試'
            : '請求失敗 ($statusCode)',
      );

      if (statusCode < 200 || statusCode >= 300) {
        return ApiFailure(AppException.fromHttpResponse(statusCode, message));
      }

      final rawCode = parsed['code'];
      if (rawCode != null && rawCode.toString() != '0') {
        final code = rawCode is int
            ? rawCode
            : int.tryParse(rawCode.toString()) ?? -1;
        return ApiFailure(AppException.fromApiError(code, message));
      }
      return ApiSuccess(parsed);
    } on DioException catch (error) {
      return ApiFailure(AppException.fromDioException(error));
    } catch (error) {
      return ApiFailure(
        UnknownException(message: '無法完成請求，請稍後再試', originalError: error),
      );
    }
  }

  String _messageFrom(
    Map<String, dynamic> response, {
    required String fallback,
  }) {
    return (response['messageZh'] ?? response['message'] ?? fallback)
        .toString();
  }

  /// 解析 Dio Response（與原 ApiService._parseDioResponse 相同邏輯）
  Map<String, dynamic> _parse(Response<dynamic> resp) {
    final statusCode = resp.statusCode ?? -1;
    if (statusCode >= 200 && statusCode < 300) {
      final data = resp.data;
      if (data == null) return {'code': 0, 'message': 'OK'};
      if (data is Map<String, dynamic>) return data;
      if (data is Map) return Map<String, dynamic>.from(data);
      return {'code': 0, 'data': data};
    }
    final data = resp.data;
    if (data is Map<String, dynamic>) {
      return {
        'code': data['code'] ?? statusCode,
        'message': data['message'] ?? 'HTTP $statusCode',
        'httpStatus': statusCode,
        ...data,
      };
    }
    return {
      'code': -1,
      'message': 'HTTP $statusCode',
      'raw': data?.toString() ?? '',
    };
  }
}
