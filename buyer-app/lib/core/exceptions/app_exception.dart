sealed class AppException implements Exception {
  const AppException(this.message);
  final String message;

  @override
  String toString() => message;
}

class NetworkException extends AppException {
  const NetworkException([super.message = '網路連線失敗，請稍後再試']);
}

class UnauthorizedException extends AppException {
  const UnauthorizedException([super.message = '登入已過期，請重新登入']);
}

class SuspendedException extends AppException {
  const SuspendedException(super.message);
}

/// 讀取的資源不存在或此刻不公開（HTTP 404，或 envelope 的 `code` 為 404）。
class NotFoundException extends AppException {
  const NotFoundException(super.message);
}

class ServerException extends AppException {
  const ServerException(super.message);
}

class TimeoutException extends AppException {
  const TimeoutException([super.message = '連線逾時，請稍後再試']);
}
