import 'package:dio/dio.dart';

/// Hawkr 統一例外層級
///
/// 所有 Repository 層應將 DioException 轉換為 AppException，
/// Controller 層使用 AsyncValue.guard() 自動捕捉。
sealed class AppException implements Exception {
  String get message;

  /// 從 DioException 轉換為對應的 AppException
  factory AppException.fromDioException(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return TimeoutException(message: '連線逾時，請檢查網路後再試', originalError: e);

      case DioExceptionType.connectionError:
        return NetworkException(message: '無法連線到伺服器，請檢查網路', originalError: e);

      case DioExceptionType.badResponse:
        final statusCode = e.response?.statusCode;
        final responseData = e.response?.data;
        final responseMessage = responseData is Map
            ? (responseData['messageZh'] ?? responseData['message'])?.toString()
            : null;
        if (statusCode == 401) {
          return UnauthorizedException(
            message: responseMessage ?? '登入已過期，請重新登入',
            originalError: e,
          );
        }
        if (statusCode != null && statusCode >= 500) {
          return ServerException(
            message: responseMessage ?? '伺服器錯誤 ($statusCode)，請稍後再試',
            statusCode: statusCode,
            originalError: e,
          );
        }
        if (statusCode != null && statusCode >= 400) {
          return ClientException(
            message: responseMessage ?? '請求失敗 ($statusCode)',
            statusCode: statusCode,
            originalError: e,
          );
        }
        return UnknownException(
          message: responseMessage ?? e.message ?? '請求失敗',
          originalError: e,
        );

      case DioExceptionType.cancel:
        return UnknownException(message: '請求已取消', originalError: e);

      case DioExceptionType.badCertificate:
        return NetworkException(message: '網路安全憑證錯誤', originalError: e);

      case DioExceptionType.unknown:
        return NetworkException(message: e.message ?? '網路錯誤', originalError: e);
    }
  }

  /// 從 API response (code != 0) 建立業務錯誤
  factory AppException.fromApiError(int code, String msg) {
    if (code == 1001 || code == 1003) {
      return UnauthorizedException(message: msg);
    }
    return ApiException(code: code, message: msg);
  }

  /// 從已收到的非 2xx HTTP response 建立 transport-level 錯誤。
  factory AppException.fromHttpResponse(int statusCode, String message) {
    if (statusCode == 401) {
      return UnauthorizedException(message: message);
    }
    if (statusCode >= 500) {
      return ServerException(message: message, statusCode: statusCode);
    }
    if (statusCode >= 400) {
      return ClientException(message: message, statusCode: statusCode);
    }
    return UnknownException(message: message);
  }
}

/// 網路不可達
class NetworkException implements AppException {
  @override
  final String message;
  final Object? originalError;

  const NetworkException({required this.message, this.originalError});

  @override
  String toString() => 'NetworkException: $message';
}

/// 401 / Token 無效
class UnauthorizedException implements AppException {
  @override
  final String message;
  final Object? originalError;

  const UnauthorizedException({required this.message, this.originalError});

  @override
  String toString() => 'UnauthorizedException: $message';
}

/// HTTP 4xx（401 除外）
class ClientException implements AppException {
  @override
  final String message;
  final int statusCode;
  final Object? originalError;

  const ClientException({
    required this.message,
    required this.statusCode,
    this.originalError,
  });

  @override
  String toString() => 'ClientException($statusCode): $message';
}

/// Server 5xx
class ServerException implements AppException {
  @override
  final String message;
  final int statusCode;
  final Object? originalError;

  const ServerException({
    required this.message,
    required this.statusCode,
    this.originalError,
  });

  @override
  String toString() => 'ServerException($statusCode): $message';
}

/// API 業務錯誤 (code != 0)
class ApiException implements AppException {
  final int code;
  @override
  final String message;

  const ApiException({required this.code, required this.message});

  @override
  String toString() => 'ApiException($code): $message';
}

/// 連線/讀取超時
class TimeoutException implements AppException {
  @override
  final String message;
  final Object? originalError;

  const TimeoutException({required this.message, this.originalError});

  @override
  String toString() => 'TimeoutException: $message';
}

/// 其他未分類錯誤
class UnknownException implements AppException {
  @override
  final String message;
  final Object? originalError;

  const UnknownException({required this.message, this.originalError});

  @override
  String toString() => 'UnknownException: $message';
}
