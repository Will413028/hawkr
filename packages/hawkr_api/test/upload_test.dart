import 'dart:async';
import 'dart:convert';
import 'dart:io' as io;
import 'dart:typed_data';

import 'package:connectrpc/connect.dart';
import 'package:fixnum/fixnum.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/upload.dart';
import 'package:test/test.dart';

class _Put {
  _Put(this.url, this.headers, this.body);
  final Uri url;
  final Map<String, String> headers;
  final Uint8List body;
}

class _FakeHttp implements UploadHttp {
  _FakeHttp(this.respond);
  final UploadHttpResult Function(_Put put, int call) respond;
  final puts = <_Put>[];
  @override
  Future<UploadHttpResult> put(
    Uri url,
    Map<String, String> headers,
    Uint8List body,
  ) async {
    final p = _Put(url, headers, body);
    puts.add(p);
    return respond(p, puts.length - 1);
  }
}

Future<void> _noSleep(Duration _) async {}

void main() {
  ioUploadHttpTests();
  group('uploadImage', () {
    final bytes = Uint8List.fromList(utf8.encode('hello image'));

    MediaAppServiceClient clientWith(
      List<GetUploadResponse Function()> polls,
      List<BeginUploadRequest> begins,
    ) {
      var n = 0;
      final transport =
          (FakeTransportBuilder()
                  .unary(MediaAppService.beginUpload, (req, ctx) {
                    begins.add(req);
                    return BeginUploadResponse(
                      stagedToken: 'tok',
                      uploadUrl: 'https://store.example/put?sig=1',
                      uploadHeaders: {'x-amz-meta-a': 'b'}.entries,
                    );
                  })
                  .unary(
                    MediaAppService.completeUpload,
                    (req, ctx) => CompleteUploadResponse(),
                  )
                  .unary(MediaAppService.getUpload, (req, ctx) {
                    expect(req.stagedToken, 'tok');
                    return polls[n++]();
                  }))
              .build();
      return MediaAppServiceClient(transport);
    }

    test('PUT 帶 MD5 與回應標頭，輪詢到 READY 回 token', () async {
      final begins = <BeginUploadRequest>[];
      final client = clientWith([
        () => GetUploadResponse(state: UploadState.UPLOAD_STATE_UPLOADED),
        () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
      ], begins);
      final http = _FakeHttp((p, _) => const UploadHttpResult(200));
      final tok = await uploadImage(
        client: client,
        http: http,
        bytes: bytes,
        contentType: 'image/png',
        sleep: _noSleep,
      );
      expect(tok, 'tok');
      expect(begins.single.size, Int64(bytes.length));
      expect(begins.single.md5, 'GoFw9rQl1mexq6KX2+T/iA==');
      expect(http.puts.single.headers, {'x-amz-meta-a': 'b'});
      expect(http.puts.single.body, bytes);
      expect(
        http.puts.single.url.toString(),
        'https://store.example/put?sig=1',
      );
    });

    test('REJECTED 丟 UploadRejected 帶原因', () async {
      final client = clientWith([
        () => GetUploadResponse(
          state: UploadState.UPLOAD_STATE_REJECTED,
          rejectReason: RejectReason.REJECT_REASON_CHECKSUM_MISMATCH,
        ),
      ], []);
      await expectLater(
        uploadImage(
          client: client,
          http: _FakeHttp((_, _) => const UploadHttpResult(200)),
          bytes: bytes,
          contentType: 'image/png',
          sleep: _noSleep,
        ),
        throwsA(
          isA<UploadRejected>().having(
            (e) => e.reason,
            'reason',
            RejectReason.REJECT_REASON_CHECKSUM_MISMATCH,
          ),
        ),
      );
    });

    test('輪詢遇到暫時錯誤下一輪再讀', () async {
      var first = true;
      final client = clientWith([
        () {
          if (first) {
            first = false;
            throw ConnectException(Code.unavailable, 'x');
          }
          return GetUploadResponse();
        },
        () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
      ], []);
      final tok = await uploadImage(
        client: client,
        http: _FakeHttp((_, _) => const UploadHttpResult(200)),
        bytes: bytes,
        contentType: 'image/png',
        sleep: _noSleep,
      );
      expect(tok, 'tok');
    });

    test('儲存空間 PUT 非 2xx 丟 UploadFailure 且不 Complete', () async {
      final client = clientWith([], []);
      await expectLater(
        uploadImage(
          client: client,
          http: _FakeHttp((_, _) => const UploadHttpResult(403)),
          bytes: bytes,
          contentType: 'image/png',
          sleep: _noSleep,
        ),
        throwsA(isA<UploadFailure>().having((e) => e.status, 'status', 403)),
      );
    });

    test('輪詢超過 maxPolls 丟逾時', () async {
      final client = clientWith(
        List.filled(
          3,
          () => GetUploadResponse(state: UploadState.UPLOAD_STATE_UPLOADED),
        ),
        [],
      );
      await expectLater(
        uploadImage(
          client: client,
          http: _FakeHttp((_, _) => const UploadHttpResult(200)),
          bytes: bytes,
          contentType: 'image/png',
          maxPolls: 3,
          sleep: _noSleep,
        ),
        throwsA(isA<UploadFailure>()),
      );
    });

    test('不支援的型別不發請求', () async {
      final begins = <BeginUploadRequest>[];
      await expectLater(
        uploadImage(
          client: clientWith([], begins),
          http: _FakeHttp((_, _) => const UploadHttpResult(200)),
          bytes: bytes,
          contentType: 'image/heic',
          sleep: _noSleep,
        ),
        throwsA(isA<UploadFailure>()),
      );
      expect(begins, isEmpty);
    });
  });

  group('uploadSecure', () {
    final bytes = Uint8List.fromList(List.generate(10, (i) => i));

    MediaAppServiceClient client({int chunkSize = 4, int count = 3}) =>
        MediaAppServiceClient(
          (FakeTransportBuilder().unary(
            MediaAppService.beginSecureUpload,
            (req, ctx) => BeginSecureUploadResponse(
              stagedToken: 'stg',
              chunkSize: chunkSize,
              chunkCount: count,
            ),
          )).build(),
        );

    Future<String> run(_FakeHttp http, {int maxAttempts = 3}) => uploadSecure(
      client: client(),
      http: http,
      filesBase: Uri.parse('https://app.example'),
      headers: () => {'Authorization': 'Bearer t'},
      bytes: bytes,
      maxAttempts: maxAttempts,
      sleep: _noSleep,
    );

    test('依序送每段，最後一段是餘數，帶授權與 octet-stream', () async {
      final http = _FakeHttp((_, _) => const UploadHttpResult(204));
      expect(await run(http), 'stg');
      expect(http.puts.map((p) => p.url.path), [
        '/api/files/secure_uploads/stg/chunks/0',
        '/api/files/secure_uploads/stg/chunks/1',
        '/api/files/secure_uploads/stg/chunks/2',
      ]);
      expect(http.puts.map((p) => p.body.length), [4, 4, 2]);
      expect(http.puts.first.headers['Authorization'], 'Bearer t');
      expect(
        http.puts.first.headers['Content-Type'],
        'application/octet-stream',
      );
    });

    test('409 依 Hawkr-Next-Index 續傳（跳到伺服器已有的下一段）', () async {
      final http = _FakeHttp((p, call) {
        if (call == 0)
          return const UploadHttpResult(409, {kNextIndexHeader: '2'});
        return const UploadHttpResult(204);
      });
      await run(http);
      expect(http.puts.map((p) => p.url.pathSegments.last), ['0', '2']);
    });

    test('409 沒有合法序號丟 UploadFailure', () async {
      final http = _FakeHttp((_, _) => const UploadHttpResult(409));
      await expectLater(run(http), throwsA(isA<UploadFailure>()));
      final http2 = _FakeHttp(
        (_, _) => const UploadHttpResult(409, {kNextIndexHeader: '9'}),
      );
      await expectLater(run(http2), throwsA(isA<UploadFailure>()));
    });

    test('5xx 同一段重送，用完次數失敗；4xx 不重送', () async {
      final http = _FakeHttp(
        (_, call) => call == 0
            ? const UploadHttpResult(503)
            : const UploadHttpResult(204),
      );
      await run(http);
      expect(http.puts.map((p) => p.url.pathSegments.last), [
        '0',
        '0',
        '1',
        '2',
      ]);

      final always = _FakeHttp((_, _) => const UploadHttpResult(500));
      await expectLater(run(always), throwsA(isA<UploadFailure>()));
      expect(always.puts, hasLength(3));

      final forbidden = _FakeHttp((_, _) => const UploadHttpResult(403));
      await expectLater(
        run(forbidden),
        throwsA(isA<UploadFailure>().having((e) => e.status, 'status', 403)),
      );
      expect(forbidden.puts, hasLength(1));
    });

    test('段計畫與大小不符丟 UploadFailure 且不送段', () async {
      final http = _FakeHttp((_, _) => const UploadHttpResult(204));
      await expectLater(
        uploadSecure(
          client: client(chunkSize: 4, count: 5),
          http: http,
          filesBase: Uri.parse('https://app.example'),
          headers: () => {},
          bytes: bytes,
          sleep: _noSleep,
        ),
        throwsA(isA<UploadFailure>()),
      );
      expect(http.puts, isEmpty);
    });
  });
}

void ioUploadHttpTests() {
  group('IoUploadHttp', () {
    test('aborts a request the server never answers', () async {
      final server = await io.HttpServer.bind(
        io.InternetAddress.loopbackIPv4,
        0,
      );
      addTearDown(() => server.close(force: true));
      server.listen((request) {});
      final http = IoUploadHttp(
        requestTimeout: const Duration(milliseconds: 200),
      );
      await expectLater(
        http.put(
          Uri.parse('http://127.0.0.1:${server.port}/put'),
          const {},
          Uint8List.fromList([1, 2, 3]),
        ),
        throwsA(isA<TimeoutException>()),
      );
    });

    test(
      'returns the status and lower-cased headers of a normal response',
      () async {
        final server = await io.HttpServer.bind(
          io.InternetAddress.loopbackIPv4,
          0,
        );
        addTearDown(() => server.close(force: true));
        server.listen((request) async {
          await request.drain<void>();
          request.response
            ..statusCode = 200
            ..headers.set('ETag', 'abc');
          await request.response.close();
        });
        final res = await IoUploadHttp().put(
          Uri.parse('http://127.0.0.1:${server.port}/put'),
          const {},
          Uint8List.fromList([1]),
        );
        expect(res.status, 200);
        expect(res.headers['etag'], 'abc');
      },
    );
  });
}
