import 'dart:convert';
import 'dart:io';

import 'package:connectrpc/connect.dart';
import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as pbduration;
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:fixnum/fixnum.dart';
import 'package:http2/http2.dart' as http2;
import 'package:test/test.dart';

/// 四個 client 共用的情境表（ADR 2026-09-25-api-error-model 的 D7）。
final _table =
    jsonDecode(
          File('../../api/testdata/error-dispatch.json').readAsStringSync(),
        )
        as Map<String, dynamic>;

Code _code(String name) => Code.values.firstWhere(
  (c) => _snake(c.name) == name,
  orElse: () => throw ArgumentError('情境表的 code $name 在 Dart 的 Code 找不到'),
);

String _snake(String camel) =>
    camel.replaceAllMapped(RegExp('[A-Z]'), (m) => '_${m[0]!.toLowerCase()}');

ErrorFacts _facts(Map<String, dynamic> input) {
  final method = input['method'] as Map<String, dynamic>;
  final error = input['error'] as Map<String, dynamic>;
  final info = error['errorInfo'] as Map<String, dynamic>?;
  final delay = error['retryDelayMs'] as int?;
  return ErrorFacts(
    code: _code(error['code'] as String),
    noResponse: error['noResponse'] as bool? ?? false,
    idempotency: switch (method['idempotency']) {
      'NO_SIDE_EFFECTS' => Idempotency.noSideEffects,
      'IDEMPOTENT' => Idempotency.idempotent,
      _ => null,
    },
    hasRequestId: method['requestId'] as bool,
    reason: info?['reason'] as String?,
    reasonDomain: info?['domain'] as String?,
    localizedMessage: error['localizedMessage'] as String?,
    fieldErrors: [
      for (final f in (error['badRequest'] as List<dynamic>? ?? []))
        FieldError(f['field'] as String, f['message'] as String),
    ],
    retryDelay: delay == null ? null : Duration(milliseconds: delay),
  );
}

/// 預設文字由呼叫端從目錄注入；替身讓「用了預設文字」可以和任何伺服器文字區分。
String _defaults(Code code) => '預設:${code.name}';

void main() {
  group('情境表', () {
    final scenarios = _table['scenarios'] as List<dynamic>;
    test('情境表不是空的', () => expect(scenarios.length, greaterThan(20)));
    for (final s in scenarios.cast<Map<String, dynamic>>()) {
      test(s['name'], () {
        final got = classifyError(_facts(s['input'] as Map<String, dynamic>));
        final want = s['expected'] as Map<String, dynamic>;
        expect(got.action.name, want['action']);
        if (want.containsKey('message')) {
          expect(got.message, want['message']);
        }
        if (want.containsKey('retryDelayMs')) {
          expect(got.retryDelay?.inMilliseconds, want['retryDelayMs']);
        }
        if (want.containsKey('reason')) {
          expect(got.reason?.name, want['reason']);
        }
        if (want.containsKey('fields')) {
          expect(got.fields, [
            for (final f in want['fields'] as List<dynamic>)
              FieldError(f['field'] as String, f['message'] as String),
          ]);
        }
      });
    }
  });

  test('使用者看到的文字：LocalizedMessage 優先，其次是注入的預設文字，取消不顯示', () {
    final localized = ConnectException(
      Code.notFound,
      'banner 1 not found',
      details: [
        ErrorDetail(
          'google.rpc.LocalizedMessage',
          (LocalizedMessage()
                ..locale = 'zh-TW'
                ..message = '找不到這份食譜')
              .writeToBuffer(),
        ),
      ],
    );
    expect(userMessage(localized, defaults: _defaults), '找不到這份食譜');
    expect(
      userMessage(ConnectException(Code.notFound, 'dev'), defaults: _defaults),
      '預設:not_found',
    );
    expect(
      userMessage(const SocketException('down'), defaults: _defaults),
      '預設:unknown',
    );
    expect(
      userMessage(ConnectException(Code.canceled, 'x'), defaults: _defaults),
      isNull,
    );
  });

  test('errorText：一定有字，取消當成 internal 的預設文字（與 submitError 同規則）', () {
    expect(
      errorText(ConnectException(Code.notFound, 'dev'), defaults: _defaults),
      '預設:not_found',
    );
    expect(
      errorText(ConnectException(Code.canceled, 'dev'), defaults: _defaults),
      '預設:internal',
    );
  });

  test(
    '欄位只有 description 沒有 LocalizedMessage 時，欄位文字是 null（不退回 description）',
    () {
      final e = ConnectException(
        Code.invalidArgument,
        'x',
        details: [
          ErrorDetail(
            'google.rpc.BadRequest',
            (BadRequest()
                  ..fieldViolations.add(
                    BadRequest_FieldViolation()
                      ..field_1 = 'banner.title'
                      ..description = 'dev only',
                  ))
                .writeToBuffer(),
          ),
        ],
      );
      expect(ErrorFacts.fromException(e).fieldErrors, [
        const FieldError('banner.title', null),
      ]);
    },
  );

  group('轉接層', () {
    test('從 details 取出 reason、文字、欄位與 RetryInfo', () {
      final e = ConnectException(
        Code.invalidArgument,
        'dev message',
        details: [
          ErrorDetail(
            'google.rpc.ErrorInfo',
            (ErrorInfo()
                  ..reason = 'VERSION_CONFLICT'
                  ..domain = 'hawkr')
                .writeToBuffer(),
          ),
          ErrorDetail(
            'google.rpc.LocalizedMessage',
            (LocalizedMessage()
                  ..locale = 'zh-TW'
                  ..message = '已被修改')
                .writeToBuffer(),
          ),
          ErrorDetail(
            'google.rpc.BadRequest',
            (BadRequest()
                  ..fieldViolations.add(
                    BadRequest_FieldViolation()
                      ..field_1 = 'banner.title'
                      ..description = '標題 必填'
                      ..localizedMessage = (LocalizedMessage()
                        ..message = '標題 必填'),
                  ))
                .writeToBuffer(),
          ),
          ErrorDetail(
            'google.rpc.RetryInfo',
            (RetryInfo()
                  ..retryDelay = (pbduration.Duration()
                    ..seconds = Int64(2)
                    ..nanos = 500000000))
                .writeToBuffer(),
          ),
        ],
      );
      final f = ErrorFacts.fromException(
        e,
        idempotency: Idempotency.noSideEffects,
      );
      expect(f.reason, 'VERSION_CONFLICT');
      expect(f.reasonDomain, 'hawkr');
      expect(f.localizedMessage, '已被修改');
      expect(f.fieldErrors, [const FieldError('banner.title', '標題 必填')]);
      expect(f.retryDelay, const Duration(milliseconds: 2500));
      expect(f.noResponse, isFalse);
    });

    test('傳輸層的例外是「沒收到回應」，伺服器與解碼的錯誤不是', () {
      for (final cause in <Object>[
        const SocketException('connection refused'),
        http2.TransportConnectionException(
          10,
          'Connection is being forcefully terminated.',
        ),
      ]) {
        expect(
          ErrorFacts.fromException(ConnectException.from(cause)).noResponse,
          isTrue,
          reason: '$cause',
        );
      }
      expect(
        ErrorFacts.fromException(
          ConnectException(Code.unavailable, 'from server'),
        ).noResponse,
        isFalse,
      );
      expect(
        ErrorFacts.fromException(
          ConnectException.from(
            const FormatException('bad json'),
            Code.internal,
          ),
        ).noResponse,
        isFalse,
      );
    });

    test('request_id 只有請求帶了才算', () {
      expect(carriesRequestId(CheckAppVersionRequest()), isFalse);
      expect(carriesRequestId(null), isFalse);
    });
  });

  group('重試的 interceptor', () {
    const read = Spec<CheckAppVersionRequest, CheckAppVersionResponse>(
      '/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion',
      StreamType.unary,
      CheckAppVersionRequest.new,
      CheckAppVersionResponse.new,
      idempotency: Idempotency.noSideEffects,
    );
    const write = Spec<CheckAppVersionRequest, CheckAppVersionResponse>(
      '/hawkr.content.v1alpha1.ContentPublicService/Write',
      StreamType.unary,
      CheckAppVersionRequest.new,
      CheckAppVersionResponse.new,
    );

    Future<(int, List<Duration>, Object?)> run(
      Spec<CheckAppVersionRequest, CheckAppVersionResponse> spec,
      List<Object> failures,
    ) async {
      var calls = 0;
      final slept = <Duration>[];
      Future<Response<CheckAppVersionRequest, CheckAppVersionResponse>> next(
        Request<CheckAppVersionRequest, CheckAppVersionResponse> req,
      ) async {
        calls++;
        if (calls <= failures.length) {
          throw failures[calls - 1];
        }
        return UnaryResponse(
          req.spec,
          Headers(),
          CheckAppVersionResponse(),
          Headers(),
        );
      }

      final call = retryInterceptor(
        sleep: (d) async => slept.add(d),
        jitter: () => 0,
      )<CheckAppVersionRequest, CheckAppVersionResponse>(next);
      Object? error;
      try {
        await call(
          UnaryRequest(
            spec,
            'https://api.test',
            Headers(),
            CheckAppVersionRequest(),
            CancelableSignal(),
          ),
        );
      } on ConnectException catch (e) {
        error = e;
      }
      return (calls, slept, error);
    }

    test('讀取遇到 unavailable 以退避重試，成功就停', () async {
      final (calls, slept, error) = await run(read, [
        ConnectException(Code.unavailable, 'down'),
        ConnectException(Code.unavailable, 'down'),
      ]);
      expect(error, isNull);
      expect(calls, 3);
      expect(slept, const [
        Duration(milliseconds: 200),
        Duration(milliseconds: 400),
      ]);
    });

    test('還沒包成 ConnectException 的網路錯誤也會重試', () async {
      final (calls, _, error) = await run(read, [
        const SocketException('connection refused'),
      ]);
      expect(calls, 2);
      expect(error, isNull);
    });

    test('最多重試兩次', () async {
      final (calls, _, error) = await run(
        read,
        List.filled(5, ConnectException(Code.unavailable, 'down')),
      );
      expect(calls, 3);
      expect((error! as ConnectException).code, Code.unavailable);
    });

    test('非冪等的方法遇到 unavailable 不重試', () async {
      final (calls, slept, error) = await run(write, [
        ConnectException(Code.unavailable, 'down'),
      ]);
      expect(calls, 1);
      expect(slept, isEmpty);
      expect(error, isNotNull);
    });

    test('RetryInfo 的時間優先於退避', () async {
      final limited = ConnectException(
        Code.resourceExhausted,
        'slow down',
        details: [
          ErrorDetail(
            'google.rpc.RetryInfo',
            (RetryInfo()
                  ..retryDelay = (pbduration.Duration()..seconds = Int64(3)))
                .writeToBuffer(),
          ),
        ],
      );
      final (calls, slept, _) = await run(read, [limited]);
      expect(calls, 2);
      expect(slept, const [Duration(seconds: 3)]);
    });
  });
}
