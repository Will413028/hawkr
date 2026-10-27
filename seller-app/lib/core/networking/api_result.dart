import '../exceptions/app_exception.dart';

/// Typed result for API calls that must not silently downgrade failures.
sealed class ApiResult<T> {
  const ApiResult();

  bool get isSuccess => this is ApiSuccess<T>;

  T get valueOrThrow {
    return switch (this) {
      ApiSuccess<T>(:final value) => value,
      ApiFailure<T>(:final error) => throw error,
    };
  }
}

final class ApiSuccess<T> extends ApiResult<T> {
  const ApiSuccess(this.value);

  final T value;
}

final class ApiFailure<T> extends ApiResult<T> {
  const ApiFailure(this.error);

  final AppException error;
}
