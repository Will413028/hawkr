import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:connectrpc/test.dart';
import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/transport.dart';
import 'package:test/test.dart';

final _checkAppVersion = ContentPublicService.checkAppVersion;

ContentPublicServiceClient _client(
  FutureOr<CheckAppVersionResponse> Function(CheckAppVersionRequest, FakeHandlerContext)
  handler,
  List<Interceptor> interceptors,
) {
  final transport = FakeTransportBuilder()
      .unary(_checkAppVersion, handler)
      .build(interceptors: interceptors);
  return ContentPublicServiceClient(transport);
}

void main() {
  group('apiTransport', () {
    test('讀取以 GET 呼叫、接受 gzip，並解開壓縮過的回應', () async {
      HttpRequest? sent;
      final body = gzip.encode(
        utf8.encode('{"storeUrl":"https://store.example/app"}'),
      );
      final transport = apiTransport(
        baseUrl: 'https://gateway.example',
        httpClient: (req) async {
          sent = req;
          return HttpResponse(
            200,
            Headers()
              ..['content-type'] = 'application/json'
              ..['content-encoding'] = 'gzip',
            Stream.value(Uint8List.fromList(body)),
            Headers(),
          );
        },
      );

      final res = await ContentPublicServiceClient(
        transport,
      ).checkAppVersion(CheckAppVersionRequest(clientVersion: '7.0.0'));

      expect(sent?.method, 'GET');
      expect(
        sent?.url,
        startsWith(
          'https://gateway.example/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion?',
        ),
      );
      expect(sent?.header['accept-encoding'], contains('gzip'));
      expect(res.storeUrl, 'https://store.example/app');
    });
  });

  group('headersInterceptor', () {
    test('每次呼叫時讀取 header，空字串的不送', () async {
      var version = '';
      Headers? seen;
      final client = _client(
        (req, ctx) {
          seen = ctx.requestHeaders;
          return CheckAppVersionResponse();
        },
        [
          headersInterceptor(
            () => {'X-App-Version': version, 'X-App-Type': 'BUYER'},
          ),
        ],
      );

      await client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));
      expect(seen!['x-app-type'], 'BUYER');
      expect(seen!.contains('x-app-version'), isFalse);

      version = '1.2.3';
      await client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));
      expect(seen!['x-app-version'], '1.2.3');
    });
  });

  group('timeoutInterceptor', () {
    // fake transport 不會因為 signal 中止，handler 自己等 signal，模擬真的 transport。
    Future<CheckAppVersionResponse> hang(
      CheckAppVersionRequest _,
      FakeHandlerContext ctx,
    ) async => throw await ctx.signal.future;

    test('沒有 deadline 的呼叫到時間就以 deadline_exceeded 結束，並告訴伺服器等多久', () async {
      Headers? seen;
      final client = _client((req, ctx) {
        seen = ctx.requestHeaders;
        return hang(req, ctx);
      }, [timeoutInterceptor(const Duration(milliseconds: 50))]);

      final watch = Stopwatch()..start();
      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(
          isA<ConnectException>().having(
            (e) => e.code,
            'code',
            Code.deadlineExceeded,
          ),
        ),
      );
      expect(watch.elapsed, lessThan(const Duration(seconds: 2)));
      expect(seen!['connect-timeout-ms'], '50');
    });

    // connect-timeout-ms 由真的 protocol 依 signal 的 deadline 產生，fake transport 不產生，這裡只驗中止。
    test('withTimeout 的上限優先於預設的上限', () async {
      final client = _client(hang, [
        timeoutInterceptor(const Duration(seconds: 30)),
      ]);

      final watch = Stopwatch()..start();
      await expectLater(
        withTimeout(
          const Duration(milliseconds: 50),
          (signal) =>
              client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'), signal: signal),
        ),
        throwsA(
          isA<ConnectException>().having(
            (e) => e.code,
            'code',
            Code.deadlineExceeded,
          ),
        ),
      );
      expect(watch.elapsed, lessThan(const Duration(seconds: 2)));
    });

    test('呼叫端帶了 deadline 就照呼叫端的', () async {
      DateTime? deadline;
      final client = _client((req, ctx) {
        deadline = ctx.signal.deadline;
        return CheckAppVersionResponse();
      }, [timeoutInterceptor(const Duration(seconds: 30))]);

      final before = DateTime.now();
      await client.checkAppVersion(
        CheckAppVersionRequest(clientVersion: '1.0.0'),
        signal: TimeoutSignal(const Duration(seconds: 3)),
      );
      expect(
        deadline!.difference(before),
        lessThan(const Duration(seconds: 4)),
      );
    });

    test('上限涵蓋重試：一直斷線的呼叫在上限內結束', () async {
      var attempts = 0;
      final client = _client(
        (req, ctx) async {
          attempts++;
          throw ConnectException.from(const SocketException('down'));
        },
        [
          timeoutInterceptor(const Duration(milliseconds: 100)),
          retryInterceptor(
            policy: const RetryPolicy(maxRetries: 100),
            sleep: (_) =>
                Future<void>.delayed(const Duration(milliseconds: 20)),
            jitter: () => 0,
          ),
        ],
      );

      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(isA<ConnectException>()),
      );
      expect(attempts, lessThan(20));
    });
  });

  for (final refused in [false, true]) {
    test(
      'local token failure refused=$refused 不重試、不回報 backend failure',
      () async {
        var tokens = 0;
        var requests = 0;
        var reports = 0;
        var invalid = 0;
        final client = _client(
          (req, ctx) {
            requests++;
            return CheckAppVersionResponse();
          },
          appInterceptors(
            timeout: const Duration(seconds: 1),
            headers: () => {},
            report: (_, _) => reports++,
            auth: authInterceptor(
              sessionToken: () async {
                tokens++;
                throw StateError('local');
              },
              isTokenRefused: (_) => refused,
              onSessionInvalid: () => invalid++,
            ),
          ),
        );
        await expectLater(
          client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
          throwsA(
            isA<ConnectException>().having(
              (e) => e.code,
              'code',
              refused ? Code.unauthenticated : Code.unavailable,
            ),
          ),
        );
        expect(tokens, 1);
        expect(requests, 0);
        expect(reports, 0);
        expect(invalid, refused ? 1 : 0);
      },
    );
  }

  group('authInterceptor', () {
    // 模擬單體：只接受 [valid] 這個 token。
    ({List<String?> seen, ContentPublicServiceClient client}) harness({
      required Future<String?> Function() token,
      required void Function() onInvalid,
      String valid = 'live',
      Object? failure,
    }) {
      final seen = <String?>[];
      final client = _client((req, ctx) {
        final auth = ctx.requestHeaders['authorization'];
        seen.add(auth);
        if (auth != 'Bearer $valid') {
          throw ConnectException(Code.unauthenticated, 'expired');
        }
        if (failure != null) throw failure;
        return CheckAppVersionResponse();
      }, [authInterceptor(sessionToken: token, onSessionInvalid: onInvalid)]);
      return (seen: seen, client: client);
    }

    test('每次送出時帶當下的 token', () async {
      var token = 'live';
      final h = harness(token: () async => token, onInvalid: () {});

      await h.client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));
      token = 'other';
      await expectLater(
        h.client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(isA<ConnectException>()),
      );

      expect(h.seen, ['Bearer live', 'Bearer other']);
    });

    test('沒有 token 時不送出', () async {
      final h = harness(token: () async => null, onInvalid: () {});

      await expectLater(
        h.client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(isA<ConnectException>()),
      );

      expect(h.seen, isEmpty);
    });

    test('unauthenticated：回到登入，不重送，錯誤原樣往外丟', () async {
      var invalid = 0;
      final h = harness(
        token: () async => 'revoked',
        onInvalid: () => invalid++,
      );

      await expectLater(
        h.client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(
          isA<ConnectException>().having(
            (e) => e.code,
            'code',
            Code.unauthenticated,
          ),
        ),
      );
      expect(h.seen, ['Bearer revoked']);
      expect(invalid, 1);
    });

    test('取不到 token（連不上 Clerk）：不送出，以 unavailable 結束，原本的例外在 cause', () async {
      var invalid = 0;
      final h = harness(
        token: () async => throw const SocketException('offline'),
        onInvalid: () => invalid++,
      );

      await expectLater(
        h.client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(
          isA<ConnectException>()
              .having((e) => e.code, 'code', Code.unavailable)
              .having(
                (e) => (e.cause as LocalAuthFailure).cause,
                'cause',
                isA<SocketException>(),
              ),
        ),
      );
      expect(h.seen, isEmpty);
      expect(invalid, 0);
    });

    test('其他錯誤與 session 無關，不登出', () async {
      var invalid = 0;
      final h = harness(
        token: () async => 'live',
        onInvalid: () => invalid++,
        failure: ConnectException(Code.permissionDenied, 'suspended'),
      );

      await expectLater(
        h.client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(
          isA<ConnectException>().having(
            (e) => e.code,
            'code',
            Code.permissionDenied,
          ),
        ),
      );
      expect(invalid, 0);
    });
  });

  group('appInterceptors', () {
    test('順序：身分在重試之內（重送時帶當下的 token），每次送出都帶 header，log 每次送出一行', () async {
      final lines = <String>[];
      final seenHeaders = <Headers>[];
      var attempts = 0;
      var token = 'a';
      final client = _client(
        (req, ctx) {
          attempts++;
          seenHeaders.add(ctx.requestHeaders);
          if (attempts == 1) {
            token = 'b';
            throw ConnectException.from(const SocketException('down'));
          }
          return CheckAppVersionResponse(
            storeUrl: ctx.requestHeaders['authorization'],
          );
        },
        appInterceptors(
          timeout: const Duration(seconds: 5),
          report: (_, __) {},
          headers: () => const {'X-App-Type': 'SELLER'},
          auth: authInterceptor(
            sessionToken: () async => token,
            onSessionInvalid: () {},
          ),
          log: lines.add,
        ),
      );

      final res = await client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));

      expect(res.storeUrl, 'Bearer b');
      expect(seenHeaders.map((h) => h['x-app-type']), ['SELLER', 'SELLER']);
      expect(seenHeaders.any((h) => h.contains('x-active-role')), isFalse);
      expect(lines, hasLength(2));
      expect(lines.first, contains('unknown'));
      expect(lines.last, contains(' ok '));
    });

    test('Connect 不送 X-Active-Role（工作區是 client 的狀態，只有舊 REST 帶它）', () async {
      Headers? seen;
      final client = _client(
        (req, ctx) {
          seen = ctx.requestHeaders;
          return CheckAppVersionResponse();
        },
        appInterceptors(
          timeout: const Duration(seconds: 5),
          report: (_, __) {},
          headers: () => const {},
        ),
      );

      await client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));

      expect(seen!.contains('x-active-role'), isFalse);
      expect(seen!.contains('authorization'), isFalse);
    });
  });

  group('sellerSuspendedInterceptor', () {
    ConnectException denied(String reason, {String domain = 'hawkr'}) =>
        ConnectException(
          Code.permissionDenied,
          'suspended',
          details: [
            ErrorDetail(
              'google.rpc.ErrorInfo',
              (ErrorInfo()
                    ..reason = reason
                    ..domain = domain)
                  .writeToBuffer(),
            ),
          ],
        );

    test('SELLER_SUSPENDED：呼叫一次，錯誤原樣丟出', () async {
      var suspended = 0;
      final error = denied('SELLER_SUSPENDED');
      final client = _client(
        (req, ctx) => throw error,
        appInterceptors(
          timeout: const Duration(seconds: 5),
          report: (_, __) {},
          headers: () => const {},
          onSellerSuspended: () => suspended++,
        ),
      );

      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(
          isA<ConnectException>()
              .having((e) => e.code, 'code', Code.permissionDenied)
              .having((e) => e.message, 'message', contains('suspended')),
        ),
      );
      expect(suspended, 1);
    });

    test('別的 reason、別的 domain、沒有 details 都不呼叫', () async {
      for (final error in [
        denied('VERSION_CONFLICT'),
        denied('SELLER_SUSPENDED', domain: 'example.com'),
        ConnectException(Code.permissionDenied, 'nope'),
      ]) {
        var suspended = 0;
        final client = _client(
          (req, ctx) => throw error,
          appInterceptors(
            timeout: const Duration(seconds: 5),
            report: (_, __) {},
            headers: () => const {},
            onSellerSuspended: () => suspended++,
          ),
        );
        await expectLater(
          client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
          throwsA(isA<ConnectException>()),
        );
        expect(suspended, 0);
      }
    });

    test('沒給 onSellerSuspended（買家的 App）不加 interceptor，錯誤照常丟出', () async {
      final client = _client(
        (req, ctx) => throw denied('SELLER_SUSPENDED'),
        appInterceptors(
          timeout: const Duration(seconds: 5),
          report: (_, __) {},
          headers: () => const {},
        ),
      );
      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(isA<ConnectException>()),
      );
    });
  });

  group('reportInterceptor', () {
    test('伺服器端的失敗上報一次，重送的錯誤原樣丟出', () async {
      final reported = <FailureContext>[];
      var clock = DateTime(2026, 9, 26, 12);
      final err = ConnectException(Code.internal, 'boom')
        ..metadata['x-trace-id'] = 'trace-1';
      final client = _client((req, ctx) => throw err, [
        reportInterceptor((e, c) => reported.add(c), now: () => clock),
      ]);

      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(same(err)),
      );
      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(same(err)),
      );
      expect(reported, hasLength(1));
      expect(reported.single.procedure, _checkAppVersion.procedure);
      expect(reported.single.code, Code.internal);
      expect(reported.single.traceId, 'trace-1');

      clock = clock.add(const Duration(seconds: 30));
      await expectLater(
        client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
        throwsA(same(err)),
      );
      expect(reported, hasLength(2));
    });

    test('沒收到回應的網路錯誤與 client 的錯誤不上報', () async {
      final reported = <FailureContext>[];
      for (final raw in <Object>[
        const SocketException('down'),
        ConnectException(Code.invalidArgument, 'bad'),
        ConnectException(Code.notFound, 'gone'),
      ]) {
        final client = _client((req, ctx) => throw raw, [
          reportInterceptor((e, c) => reported.add(c)),
        ]);
        await expectLater(
          client.checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
          throwsA(anything),
        );
      }
      expect(reported, isEmpty);
    });
  });
}
