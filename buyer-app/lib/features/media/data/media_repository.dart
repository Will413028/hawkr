import 'dart:io';
import 'dart:typed_data';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/oss_service.dart';
import '../../../shared/utils/image_processing_service.dart';
import 'media_model.dart';

part 'media_repository.g.dart';

@riverpod
MediaRepository mediaRepository(Ref ref) =>
    MediaRepository(ref.watch(ossServiceProvider));

class MediaRepository {
  final OssService _ossService;

  MediaRepository(this._ossService);

  /// 上傳圖片，回傳 [UploadResult]（含 previewUrl 與 stagedToken）
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
    final filename = compression.compressed
        ? _asJpegFilename(file.uri.pathSegments.last)
        : file.uri.pathSegments.last;

    final result = await _ossService.uploadImage(
      imageBytes: compression.bytes,
      filename: filename,
      contentType: uploadContentType,
    );

    if (result == null) throw const ServerException('圖片上傳失敗');

    return UploadResult(
      previewUrl: result.previewUrl,
      stagedToken: result.stagedToken,
    );
  }

  static String _asJpegFilename(String filename) {
    final dotIndex = filename.lastIndexOf('.');
    if (dotIndex <= 0) return '$filename.jpg';
    return '${filename.substring(0, dotIndex)}.jpg';
  }
}
