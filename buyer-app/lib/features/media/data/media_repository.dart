import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.client.dart';
import 'package:hawkr_api/upload.dart' as upload;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/image_processing_service.dart';
import 'media_model.dart';

part 'media_repository.g.dart';

@riverpod
MediaRepository mediaRepository(Ref ref) => MediaRepository(
  client: ref.watch(mediaClientProvider),
  http: upload.IoUploadHttp(),
  locale: () => ref.read(uiLocaleProvider),
);

class MediaRepository {
  MediaRepository({
    required MediaAppServiceClient client,
    required upload.UploadHttp http,
    required UiLocale Function() locale,
  }) : _client = client,
       _http = http,
       _locale = locale;

  final MediaAppServiceClient _client;
  final upload.UploadHttp _http;
  final UiLocale Function() _locale;

  /// 上傳圖片（流程在 `hawkr_api` 的 `upload.dart`），回傳 READY 的 [UploadResult.stagedToken]。
  /// 伺服器拒絕時丟帶拒絕原因文字的 [ServerException]。
  Future<UploadResult> uploadImage({
    required File file,
    String contentType = 'image/jpeg',
  }) async {
    final Uint8List imageBytes;
    try {
      imageBytes = await file.readAsBytes();
    } catch (_) {
      throw const NetworkException();
    }

    final compression = await ImageProcessingService.compressIfNeeded(
      imageBytes,
    );
    final uploadContentType = compression.compressed
        ? 'image/jpeg'
        : contentType;
    try {
      final token = await upload.uploadImage(
        client: _client,
        http: _http,
        bytes: compression.bytes,
        contentType: uploadContentType,
      );
      return UploadResult(stagedToken: token);
    } on upload.UploadRejected catch (e) {
      throw ServerException(e.reason.uiText(_locale()) ?? '圖片上傳失敗');
    } on upload.UploadFailure {
      throw const ServerException('圖片上傳失敗');
    }
  }
}
