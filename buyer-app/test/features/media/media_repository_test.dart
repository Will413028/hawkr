import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/upload.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/media/data/media_repository.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

class _Http implements UploadHttp {
  _Http({this.status = 200});
  final int status;
  int puts = 0;

  @override
  Future<UploadHttpResult> put(
    Uri url,
    Map<String, String> headers,
    Uint8List body,
  ) async {
    puts++;
    return UploadHttpResult(status);
  }
}

MediaRepository _repo(_Http http, GetUploadResponse Function() poll) {
  final transport =
      (FakeTransportBuilder()
              .unary(
                MediaAppService.beginUpload,
                (req, ctx) => BeginUploadResponse(
                  stagedToken: 'tok',
                  uploadUrl: 'https://store.example/put',
                ),
              )
              .unary(
                MediaAppService.completeUpload,
                (req, ctx) => CompleteUploadResponse(),
              )
              .unary(MediaAppService.getUpload, (req, ctx) => poll()))
          .build();
  return MediaRepository(
    client: MediaAppServiceClient(transport),
    http: http,
    locale: () => UiLocale.zhTW,
    defaults: (code) => 'default:${code.name}',
  );
}

Future<File> _tmpImage() async {
  final dir = await Directory.systemTemp.createTemp('media_repo_test');
  addTearDown(() => dir.delete(recursive: true));
  final f = File('${dir.path}/a.jpg');
  await f.writeAsBytes([1, 2, 3, 4]);
  return f;
}

void main() {
  test('uploadImage returns the READY staged token', () async {
    final http = _Http();
    final repo = _repo(
      http,
      () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
    );
    final result = await repo.uploadImage(file: await _tmpImage());
    expect(result.stagedToken, 'tok');
    expect(http.puts, 1);
  });

  test('REJECTED surfaces the catalog text of the reason', () async {
    final repo = _repo(
      _Http(),
      () => GetUploadResponse(
        state: UploadState.UPLOAD_STATE_REJECTED,
        rejectReason: RejectReason.REJECT_REASON_UNRECOGNIZED_FORMAT,
      ),
    );
    await expectLater(
      repo.uploadImage(file: await _tmpImage()),
      throwsA(
        isA<ServerException>().having(
          (e) => e.message,
          'message',
          RejectReason.REJECT_REASON_UNRECOGNIZED_FORMAT.uiText(UiLocale.zhTW),
        ),
      ),
    );
  });

  test(
    'a failed transfer carries the catalog default text, not a literal',
    () async {
      final repo = _repo(
        _Http(status: 500),
        () => GetUploadResponse(state: UploadState.UPLOAD_STATE_READY),
      );
      await expectLater(
        repo.uploadImage(file: await _tmpImage()),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            'default:unavailable',
          ),
        ),
      );
    },
  );
}
