import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_api/upload.dart';
import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/features/media/data/media_repository.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../support/connect_errors.dart';

class _Put {
  _Put(this.url, this.headers);
  final Uri url;
  final Map<String, String> headers;
}

class _Http implements UploadHttp {
  _Http({this.error});

  final Object? error;
  final puts = <_Put>[];

  @override
  Future<UploadHttpResult> put(
    Uri url,
    Map<String, String> headers,
    Uint8List body,
  ) async {
    if (error != null) throw error!;
    puts.add(_Put(url, headers));
    return const UploadHttpResult(200);
  }
}

MediaRepository _repo(
  _Http http, {
  required GetUploadResponse Function() poll,
  String? token = 'sess',
  void Function()? onBegin,
  void Function()? onBeginSecure,
  void Function()? onComplete,
}) {
  final transport =
      (FakeTransportBuilder()
              .unary(MediaAppService.beginUpload, (req, ctx) {
                onBegin?.call();
                return BeginUploadResponse(
                  stagedToken: 'tok',
                  uploadUrl: 'https://store.example/put',
                );
              })
              .unary(MediaAppService.completeUpload, (req, ctx) {
                onComplete?.call();
                return CompleteUploadResponse();
              })
              .unary(MediaAppService.getUpload, (req, ctx) => poll())
              .unary(MediaAppService.beginSecureUpload, (req, ctx) {
                onBeginSecure?.call();
                return BeginSecureUploadResponse(
                  stagedToken: 'sec',
                  chunkSize: 8,
                  chunkCount: 1,
                );
              }))
          .build();
  return MediaRepository(
    client: MediaAppServiceClient(transport),
    http: http,
    filesBase: Uri.parse('https://app.example'),
    sessionToken: () async => token,
    locale: () => UiLocale.zhTW,
    defaults: (code) => 'default:${code.name}',
  );
}

void main() {
  final bytes = Uint8List.fromList([1, 2, 3, 4]);

  test('public image upload returns the READY staged token', () async {
    final repo = _repo(
      _Http(),
      poll: () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
    );
    final result = await repo.uploadWithSession(
      bytes: bytes,
      filename: 'a.jpg',
      compress: false,
    );
    expect(result.stagedToken, 'tok');
    expect(result.size, 4);
  });

  test('REJECTED shows the catalog text of the reason', () async {
    final repo = _repo(
      _Http(),
      poll: () => GetUploadResponse(
        state: UploadState.UPLOAD_STATE_REJECTED,
        rejectReason: RejectReason.REJECT_REASON_UNRECOGNIZED_FORMAT,
      ),
    );
    await expectLater(
      repo.uploadWithSession(bytes: bytes, filename: 'a.jpg', compress: false),
      throwsA(
        isA<UploadRejectedException>().having(
          (e) => e.message,
          'message',
          RejectReason.REJECT_REASON_UNRECOGNIZED_FORMAT.uiText(UiLocale.zhTW),
        ),
      ),
    );
  });

  test('private upload sends chunks with the Bearer session token', () async {
    final http = _Http();
    final repo = _repo(
      http,
      poll: () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
    );
    final token = await repo.uploadPrivateDocument(bytes: bytes);
    expect(token, 'sec');
    expect(http.puts.single.url.path, '/api/files/secure_uploads/sec/chunks/0');
    expect(http.puts.single.headers['Authorization'], 'Bearer sess');
  });

  test(
    'private upload without a session token fails before any request',
    () async {
      final http = _Http();
      final repo = _repo(
        http,
        token: null,
        poll: () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
      );
      await expectLater(
        repo.uploadPrivateDocument(bytes: bytes),
        throwsA(
          isA<UnauthorizedException>().having(
            (e) => e.message,
            'message',
            'default:unauthenticated',
          ),
        ),
      );
      expect(http.puts, isEmpty);
    },
  );

  group('failure text', () {
    final ready = () =>
        GetUploadResponse(state: UploadState.UPLOAD_STATE_READY);
    final serverError = reasonError(
      Code.failedPrecondition,
      'SOME_REASON',
      message: '伺服器說的話',
    );

    Future<void> expectUnknown(Future<Object?> run, String text) => expectLater(
      run,
      throwsA(isA<AppException>().having((e) => e.message, 'message', text)),
    );

    test('beginUpload ConnectException shows its LocalizedMessage', () async {
      final repo = _repo(
        _Http(),
        poll: ready,
        onBegin: () => throw serverError,
      );
      await expectUnknown(
        repo.uploadWithSession(
          bytes: bytes,
          filename: 'a.jpg',
          compress: false,
        ),
        '伺服器說的話',
      );
    });

    test(
      'completeUpload ConnectException shows its LocalizedMessage',
      () async {
        final repo = _repo(
          _Http(),
          poll: ready,
          onComplete: () => throw serverError,
        );
        await expectUnknown(
          repo.uploadWithSession(
            bytes: bytes,
            filename: 'a.jpg',
            compress: false,
          ),
          '伺服器說的話',
        );
      },
    );

    test('private beginSecureUpload ConnectException shows its text', () async {
      final repo = _repo(
        _Http(),
        poll: ready,
        onBeginSecure: () => throw serverError,
      );
      await expectUnknown(repo.uploadPrivateDocument(bytes: bytes), '伺服器說的話');
    });

    test(
      'transport error on the presigned PUT shows the unavailable default',
      () async {
        final repo = _repo(
          _Http(error: const SocketException('offline')),
          poll: ready,
        );
        await expectUnknown(
          repo.uploadWithSession(
            bytes: bytes,
            filename: 'a.jpg',
            compress: false,
          ),
          'default:unavailable',
        );
      },
    );

    test(
      'transport error on a private chunk shows the unavailable default',
      () async {
        final repo = _repo(
          _Http(error: const HttpException('reset')),
          poll: ready,
        );
        await expectUnknown(
          repo.uploadPrivateDocument(bytes: bytes),
          'default:unavailable',
        );
      },
    );
  });
}
