/// Connect 錯誤的分派與重試（ADR backend/docs/adr/2026-09-25-api-error-model.md 的 D7）。
///
/// [classifyError] 是純函式，以 `api/testdata/error-dispatch.json` 的情境表測試，四個 client 的結果必須相同。
/// [ErrorFacts.fromException] 是 Dart 的轉接層：從 [ConnectException] 取出 details 與「沒收到回應」。
library;

import 'dart:async';
import 'dart:io';
import 'dart:math';

import 'package:connectrpc/connect.dart';
import 'package:http2/http2.dart' as http2;
import 'package:protobuf/protobuf.dart';

import 'gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'gen/google/rpc/error_details.pb.dart';

/// ErrorInfo 的 domain：只有這個 domain 的 reason 是我們的 [ErrorReason]。
const errorDomain = 'hawkr';

bool isSessionUnavailableCode(Code code) =>
    code != Code.unauthenticated && code != Code.permissionDenied;

/// 錯誤的處理方式。
enum ErrorAction {
  /// 呼叫方自己取消的，不處理。
  ignore,

  /// 自動重試；[ErrorDecision.retryDelay] 有值時照它等待，否則用退避。
  retry,

  /// session 失效：回到登入（沒有 refresh）。
  reauthenticate,

  /// 執行 [ErrorDecision.reason] 的流程，例如版本衝突時保留輸入、顯示新舊版本對照。
  reason,

  /// 把 [ErrorDecision.fields] 標在欄位上。
  fields,

  /// 顯示 [ErrorDecision.message]；null 時顯示該 code 的預設文字。
  message,
}

/// 一個欄位的錯誤。[field] 是請求訊息的 proto 路徑，例如 `banner.title`。
final class FieldError {
  const FieldError(this.field, this.message);

  final String field;

  /// 伺服器給使用者的欄位文字（LocalizedMessage）；null 表示沒有，由呼叫端補預設文字。
  final String? message;

  @override
  bool operator ==(Object other) =>
      other is FieldError && other.field == field && other.message == message;

  @override
  int get hashCode => Object.hash(field, message);

  @override
  String toString() => '$field: $message';
}

/// 分類器的輸入。
final class ErrorFacts {
  const ErrorFacts({
    required this.code,
    this.noResponse = false,
    this.idempotency,
    this.hasRequestId = false,
    this.reason,
    this.reasonDomain,
    this.localizedMessage,
    this.fieldErrors = const [],
    this.retryDelay,
  });

  /// 從 [ConnectException] 與呼叫的方法取出分類需要的事實。[request] 是請求訊息，用來判斷有沒有帶
  /// AIP-155 的 `request_id`。
  factory ErrorFacts.fromException(
    ConnectException e, {
    Idempotency? idempotency,
    Object? request,
  }) {
    String? reason;
    String? domain;
    String? localized;
    Duration? retryDelay;
    final fields = <FieldError>[];
    for (final d in e.details) {
      switch (d.type) {
        case 'google.rpc.ErrorInfo':
          final info = ErrorInfo.fromBuffer(d.value);
          reason = info.reason;
          domain = info.domain;
        case 'google.rpc.LocalizedMessage':
          localized = LocalizedMessage.fromBuffer(d.value).message;
        case 'google.rpc.BadRequest':
          for (final v in BadRequest.fromBuffer(d.value).fieldViolations) {
            fields.add(
              FieldError(
                v.field_1,
                v.hasLocalizedMessage() ? v.localizedMessage.message : null,
              ),
            );
          }
        case 'google.rpc.RetryInfo':
          final delay = RetryInfo.fromBuffer(d.value).retryDelay;
          retryDelay = Duration(
            seconds: delay.seconds.toInt(),
            microseconds: delay.nanos ~/ 1000,
          );
      }
    }
    return ErrorFacts(
      code: e.code,
      noResponse: _noResponse(e),
      idempotency: idempotency,
      hasRequestId: carriesRequestId(request),
      reason: reason,
      reasonDomain: domain,
      localizedMessage: localized,
      fieldErrors: fields,
      retryDelay: retryDelay,
    );
  }

  final Code code;

  /// 沒有收到伺服器的回應（連線失敗、斷線）。
  final bool noResponse;

  /// 方法的 idempotency_level；null 表示 UNKNOWN。
  final Idempotency? idempotency;
  final bool hasRequestId;
  final String? reason;
  final String? reasonDomain;
  final String? localizedMessage;
  final List<FieldError> fieldErrors;
  final Duration? retryDelay;
}

/// 分類的結果。
final class ErrorDecision {
  const ErrorDecision(
    this.action, {
    this.retryDelay,
    this.reason,
    this.fields = const [],
    this.message,
  });

  final ErrorAction action;
  final Duration? retryDelay;
  final ErrorReason? reason;
  final List<FieldError> fields;

  /// 伺服器給使用者的文字（LocalizedMessage）；null 表示顯示該 code 的預設文字。
  final String? message;
}

/// 依 D7 的順序分派：取消 → 可以安全重送而且可以重試 → 重新取得身分 → 認得的 reason → 欄位錯誤 → 文字。
ErrorDecision classifyError(ErrorFacts f) {
  if (f.code == Code.canceled) {
    return const ErrorDecision(ErrorAction.ignore);
  }
  final safe = f.idempotency != null || f.hasRequestId;
  final retryable =
      f.code == Code.unavailable ||
      f.noResponse ||
      (f.code == Code.resourceExhausted && f.retryDelay != null);
  if (safe && retryable) {
    return ErrorDecision(ErrorAction.retry, retryDelay: f.retryDelay);
  }
  if (f.code == Code.unauthenticated) {
    return const ErrorDecision(ErrorAction.reauthenticate);
  }
  final reason = f.reasonDomain == errorDomain ? _knownReason(f.reason) : null;
  if (reason != null) {
    return ErrorDecision(
      ErrorAction.reason,
      reason: reason,
      message: f.localizedMessage,
    );
  }
  if (f.fieldErrors.isNotEmpty) {
    return ErrorDecision(
      ErrorAction.fields,
      fields: f.fieldErrors,
      message: f.localizedMessage,
    );
  }
  return ErrorDecision(ErrorAction.message, message: f.localizedMessage);
}

/// 沒有 LocalizedMessage 時顯示的該 code 的預設文字（D3，client 的呈現層）。文字在共用文案目錄
/// （`hawkr_ui_text`，依賴本套件，所以由呼叫端注入）；[Code.canceled] 不顯示，不會傳進來。
typedef DefaultErrorText = String Function(Code code);

/// 顯示給使用者的文字：伺服器的 LocalizedMessage，沒有時用 [defaults] 給的該 code 的預設文字；取消時是 null。
String? userMessage(Object error, {required DefaultErrorText defaults}) {
  final e = ConnectException.from(error);
  if (e.code == Code.canceled) {
    return null;
  }
  return ErrorFacts.fromException(e).localizedMessage ?? defaults(e.code);
}

/// 一定有字的 [userMessage]：伺服器的 LocalizedMessage，沒有時是該 code 的預設文字；取消（目錄沒有 canceled 的
/// 文字）當成 [Code.internal]，與 `submitError` 同一條規則。呼叫點不各自補一句寫死的文字。
String errorText(Object error, {required DefaultErrorText defaults}) =>
    userMessage(error, defaults: defaults) ?? defaults(Code.internal);

/// 請求訊息有沒有帶 AIP-155 的 `request_id`。
bool carriesRequestId(Object? request) {
  if (request is! GeneratedMessage) {
    return false;
  }
  final field = request.info_.byName['requestId'];
  return field != null && request.hasField(field.tagNumber);
}

ErrorReason? _knownReason(String? name) {
  if (name == null) {
    return null;
  }
  for (final r in ErrorReason.values) {
    if (r != ErrorReason.ERROR_REASON_UNSPECIFIED && r.name == name) {
      return r;
    }
  }
  return null;
}

/// connectrpc 把傳輸層的例外以 [ConnectException.from] 包起來（code 是 unknown，cause 是原本的例外）；
/// 伺服器回的錯誤沒有 cause。
bool _noResponse(ConnectException e) => switch (e.cause) {
  SocketException() ||
  HandshakeException() ||
  TimeoutException() ||
  http2.TransportException() => true,
  _ => false,
};

final class LocalAuthFailure implements Exception {
  const LocalAuthFailure([this.cause]);
  final Object? cause;
}

/// 自動重試的設定：最多重送 [maxRetries] 次，退避從 [baseDelay] 起每次加倍、上限 [maxDelay]，加上
/// 0 到同樣長度的 jitter。伺服器帶了 RetryInfo 時照它的時間。
final class RetryPolicy {
  const RetryPolicy({
    this.maxRetries = 2,
    this.baseDelay = const Duration(milliseconds: 200),
    this.maxDelay = const Duration(seconds: 2),
  });

  final int maxRetries;
  final Duration baseDelay;
  final Duration maxDelay;

  Duration backoff(int retry, double jitter) {
    final exp = baseDelay * (1 << retry);
    final capped = exp > maxDelay ? maxDelay : exp;
    return capped + capped * jitter;
  }
}

/// unary 呼叫依 [classifyError] 自動重試的 interceptor。串流不重試：已經收到的訊息無法撤回。
Interceptor retryInterceptor({
  RetryPolicy policy = const RetryPolicy(),
  Future<void> Function(Duration) sleep = Future<void>.delayed,
  double Function() jitter = _randomJitter,
}) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      if (req is! UnaryRequest<I, O>) {
        return next(req);
      }
      for (var retry = 0; ; retry++) {
        try {
          return await next(req);
        } catch (raw) {
          if (ConnectException.from(raw).cause is LocalAuthFailure ||
              retry >= policy.maxRetries) {
            rethrow;
          }
          // 傳輸層的例外可能還沒被包成 ConnectException；以 connectrpc 同樣的規則轉換後再分類。
          final decision = classifyError(
            ErrorFacts.fromException(
              ConnectException.from(raw),
              idempotency: req.spec.idempotency,
              request: req.message,
            ),
          );
          if (decision.action != ErrorAction.retry) {
            rethrow;
          }
          // 等待中被取消或到了 deadline 就不再重送，以中止的原因結束。
          await Future.any<void>([
            sleep(decision.retryDelay ?? policy.backoff(retry, jitter())),
            req.signal.future.then<void>((reason) => throw reason),
          ]);
        }
      }
    };
  };
}

final _random = Random();

double _randomJitter() => _random.nextDouble();
