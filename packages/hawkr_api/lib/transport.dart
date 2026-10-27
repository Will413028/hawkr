/// App 呼叫對外 API（Protobuf＋Connect）的 transport 與共用的 interceptor（試點計畫步驟 6.6）。錯誤的分派與
/// 重試在 errors.dart。
library;

import 'dart:async';

import 'package:connectrpc/connect.dart';
import 'package:connectrpc/http2.dart';
import 'package:connectrpc/io.dart' show GzipCompression;
import 'package:connectrpc/protobuf.dart';
import 'package:connectrpc/protocol/connect.dart' as protocol;

import 'errors.dart';
import 'gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';

export 'package:connectrpc/connect.dart';

/// 串流開著時多久送一次 HTTP/2 PING。`Http2ClientTransport` 預設不送，沒有 FIN 的斷線（例如離開 Wi-Fi）要等
/// TCP 逾時才會發現（試點步驟 5 的 iOS 結果）；20 秒加上 PING 的 15 秒逾時，約 35 秒內發現。
const kPingInterval = Duration(seconds: 20);

/// App 共用的 HTTP/2 連線。同一個 gateway 的 transport 共用它，只開一條連線。
HttpClient apiHttpClient() => createHttpClient(
  transport: Http2ClientTransport(pingInterval: kPingInterval),
);

/// 對外 API 的 transport。[baseUrl] 是 gateway 的根目錄：Connect 的路徑是 `/{service}/{method}`，不在
/// `/api/v1` 底下。讀取（`NO_SIDE_EFFECTS`）以 GET 呼叫，與 website 相同。
///
/// 接受 gzip 的回應（單體對 1 KB 以上的回應壓縮）：content 的回應 gzip 後約剩 1/5。connectrpc 預設不接受，
/// 不設就收到未壓縮的回應。編碼用 JSON：gzip 之後 binary 只再小 15–20%，解碼快的部分以微秒計，JSON 在 log
/// 與 `buf curl` 看得懂，與 website 相同（試點步驟 7 的量測）。
Transport apiTransport({
  required String baseUrl,
  required HttpClient httpClient,
  List<Interceptor> interceptors = const [],
  Codec codec = const JsonCodec(),
}) {
  return protocol.Transport(
    baseUrl: baseUrl,
    codec: codec,
    httpClient: httpClient,
    interceptors: interceptors,
    useHttpGet: true,
    acceptCompressions: [GzipCompression()],
  );
}

/// 每個請求加上 [headers] 回傳的 header；每次呼叫時讀取，值是空字串的不送。
Interceptor headersInterceptor(Map<String, String> Function() headers) {
  return <I extends Object, O extends Object>(next) {
    return (req) {
      for (final MapEntry(:key, :value) in headers().entries) {
        if (value.isNotEmpty) {
          req.headers[key] = value;
        }
      }
      return next(req);
    };
  };
}

/// 每次送出時帶當下的 session token。沒有 session 或 token 換發失敗時不送出，也不進 RPC 重試與錯誤上報。
/// [isTokenRefused] 判定換發遭拒時，以及伺服器回 unauthenticated 時，呼叫 [onSessionInvalid]。
Interceptor authInterceptor({
  required Future<String?> Function() sessionToken,
  required void Function() onSessionInvalid,
  bool Function(Object)? isTokenRefused,
}) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      final String? token;
      try {
        token = await sessionToken();
      } catch (raw) {
        final refused = isTokenRefused?.call(raw) ?? false;
        if (refused) onSessionInvalid();
        throw ConnectException(
          refused ? Code.unauthenticated : Code.unavailable,
          'session token unavailable',
          cause: LocalAuthFailure(raw),
        );
      }
      if (token == null || token.isEmpty) {
        throw ConnectException(
          Code.unauthenticated,
          'no session token',
          cause: const LocalAuthFailure(),
        );
      } else {
        req.headers['authorization'] = 'Bearer $token';
      }
      try {
        return await next(req);
      } catch (raw) {
        if (ConnectException.from(raw).code == Code.unauthenticated) {
          onSessionInvalid();
        }
        rethrow;
      }
    };
  };
}

/// App 的 interceptor，順序固定：整個呼叫的逾時（涵蓋重試）→ 上報重試之後的結果 → 重試 → 賣家停權 → 身分 → 每次送出都
/// 帶的 header（[headers]）→ log（每次送出一行）。[auth] 只給登入後的
/// service。[onSellerSuspended] 只有賣家的 App 給：分類器認得 `SELLER_SUSPENDED`（D10）時呼叫，錯誤原樣丟出。
List<Interceptor> appInterceptors({
  required Duration timeout,
  required void Function(ConnectException error, FailureContext context) report,
  required Map<String, String> Function() headers,
  Interceptor? auth,
  void Function()? onSellerSuspended,
  void Function(String line)? log,
}) => [
  timeoutInterceptor(timeout),
  reportInterceptor(report),
  retryInterceptor(),
  if (onSellerSuspended != null) sellerSuspendedInterceptor(onSellerSuspended),
  ?auth,
  headersInterceptor(headers),
  if (log != null) logInterceptor(log),
];

/// 賣家身分在 session 中途被停權（D10）：`permission_denied` 加上 domain 是 hawkr 的 `SELLER_SUSPENDED`
/// 時呼叫 [onSuspended]，錯誤原樣丟出。只轉成狀態，導向停權公告頁由 router 依狀態決定。
Interceptor sellerSuspendedInterceptor(void Function() onSuspended) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      try {
        return await next(req);
      } catch (raw) {
        final d = classifyError(
          ErrorFacts.fromException(ConnectException.from(raw)),
        );
        if (d.action == ErrorAction.reason &&
            d.reason == ErrorReason.SELLER_SUSPENDED) {
          onSuspended();
        }
        rethrow;
      }
    };
  };
}

/// 每次送出（含重試）交給 [write] 一行：procedure、結果或 code、耗時與 trace id。
Interceptor logInterceptor(void Function(String line) write) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      final watch = Stopwatch()..start();
      try {
        final res = await next(req);
        write(
          '[Connect] ${req.spec.procedure} ok ${watch.elapsedMilliseconds}ms',
        );
        return res;
      } catch (raw) {
        final err = ConnectException.from(raw);
        final trace = err.metadata['x-trace-id'] ?? '-';
        write(
          '[Connect] ${req.spec.procedure} ${err.code.name} '
          '${watch.elapsedMilliseconds}ms [trace:$trace] ${err.message}',
        );
        rethrow;
      }
    };
  };
}

/// 單次呼叫的上限：[call] 拿到的 signal 在 [timeout] 後中止；以它呼叫時 [timeoutInterceptor] 不再套用預設的
/// 上限。呼叫結束時取消計時器（原因同 [timeoutInterceptor]）。
Future<T> withTimeout<T>(
  Duration timeout,
  Future<T> Function(AbortSignal signal) call,
) async {
  final signal = _DeadlineSignal(timeout);
  try {
    return await call(signal);
  } finally {
    signal.dispose();
  }
}

/// unary 呼叫沒有 deadline 時，整個呼叫（含重試）最多 [timeout]；呼叫端以 `signal` 帶了 deadline 就照呼叫端的
/// （[withTimeout]）。串流不設上限，長度由伺服器決定。
///
/// 不用 connectrpc 的 `TimeoutSignal`：它的計時器取消不了，呼叫結束後還會留著，widget 測試結束時會因為還有沒
/// 觸發的 timer 而失敗。
Interceptor timeoutInterceptor(Duration timeout) {
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      if (req is! UnaryRequest<I, O> || req.signal.deadline != null) {
        return next(req);
      }
      final signal = _DeadlineSignal(timeout, parent: req.signal);
      // transport 在 interceptor 之前就組好 header，這裡補上，伺服器才知道 client 等多久。
      req.headers['connect-timeout-ms'] = '${timeout.inMilliseconds}';
      try {
        return await next(
          UnaryRequest<I, O>(
            req.spec,
            req.url,
            req.headers,
            req.message,
            signal,
          ),
        );
      } finally {
        signal.dispose();
      }
    };
  };
}

final class _DeadlineSignal implements AbortSignal {
  _DeadlineSignal(Duration timeout, {AbortSignal? parent})
    : deadline = DateTime.now().add(timeout) {
    _timer = Timer(timeout, () {
      _expired.complete(
        ConnectException(Code.deadlineExceeded, 'operation timed out'),
      );
    });
    future = parent == null
        ? _expired.future
        : Future.any([parent.future, _expired.future]);
  }

  @override
  final DateTime deadline;

  @override
  late final Future<ConnectException> future;

  final _expired = Completer<ConnectException>();
  late final Timer _timer;

  void dispose() => _timer.cancel();
}

/// 上報一次失敗時的 context。
final class FailureContext {
  const FailureContext({
    required this.procedure,
    required this.code,
    required this.traceId,
  });

  /// 例如 `/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion`。
  final String procedure;
  final Code code;

  /// 伺服器回的 `x-trace-id`；沒有時是 `no-trace`。
  final String traceId;
}

// 伺服器端的失敗才上報（同 website 的 reportInterceptor）。
const _reportedCodes = {
  Code.unknown,
  Code.internal,
  Code.dataLoss,
  Code.unavailable,
  Code.deadlineExceeded,
  Code.unimplemented,
};

/// 把伺服器端的失敗交給 [capture]（例如 Sentry）；沒收到回應的網路錯誤不算。同一個方法、code、trace 在
/// [dedup] 之內只報一次，一次 outage 不會淹沒上報。放在重試的外層，只報重試之後的結果。
Interceptor reportInterceptor(
  void Function(ConnectException error, FailureContext context) capture, {
  Duration dedup = const Duration(seconds: 30),
  DateTime Function() now = DateTime.now,
}) {
  final recent = <String, DateTime>{};
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      try {
        return await next(req);
      } catch (raw) {
        final err = ConnectException.from(raw);
        if (err.cause is! LocalAuthFailure &&
            _reportedCodes.contains(err.code) &&
            !ErrorFacts.fromException(err).noResponse) {
          final context = FailureContext(
            procedure: req.spec.procedure,
            code: err.code,
            traceId: err.metadata['x-trace-id'] ?? 'no-trace',
          );
          final key = '${context.procedure}:${context.code}:${context.traceId}';
          final at = now();
          recent.removeWhere((_, t) => at.difference(t) >= dedup);
          final last = recent[key];
          if (last == null || at.difference(last) >= dedup) {
            recent[key] = at;
            capture(err, context);
          }
        }
        rethrow;
      }
    };
  };
}
