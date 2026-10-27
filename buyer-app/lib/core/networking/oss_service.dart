import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../config/media_image.dart';
import 'dio_provider.dart';

part 'oss_service.g.dart';

@Riverpod(keepAlive: true)
OssService ossService(Ref ref) => OssService(ref.watch(dioProvider));

/// 媒體上傳結果
class MediaUploadResult {
  final String previewUrl;
  final String stagedToken;

  MediaUploadResult({required this.previewUrl, required this.stagedToken});
}

class OssService {
  final Dio _dio;

  OssService(this._dio);

  /// 上傳圖片（3-step: createSession → PUT → complete）
  ///
  /// Returns [MediaUploadResult] with previewUrl (displayable) and
  /// stagedToken (for passing to backend when creating/updating resources).
  Future<MediaUploadResult?> uploadImage({
    required Uint8List imageBytes,
    required String filename,
    String contentType = 'image/jpeg',
  }) async {
    // Step 1: Create upload session
    final session = await _createSession(
      filename: filename,
      contentType: contentType,
      fileSize: imageBytes.length,
    );
    if (session == null) return null;

    final sessionId = session['sessionId'] as String?;
    final uploadUrl = session['uploadUrl'] as String?;
    if (sessionId == null || uploadUrl == null) return null;

    // Step 2: Upload file to presigned URL (use standalone Dio to avoid baseUrl)
    try {
      final uploadDio = Dio();
      await uploadDio.put(
        uploadUrl,
        data: Stream.fromIterable([imageBytes]),
        options: Options(
          headers: {
            'Content-Type': contentType,
            'Content-Length': imageBytes.length,
          },
        ),
      );
    } catch (_) {
      return null;
    }

    // Step 3: Complete session
    return _completeSession(sessionId);
  }

  Future<Map<String, dynamic>?> _createSession({
    required String filename,
    required String contentType,
    required int fileSize,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/media/upload_sessions',
        data: {
          'filename': filename,
          'contentType': contentType,
          'fileSize': fileSize.toString(),
        },
      );
      final body = resp.data;
      if (body == null || body['code'] != 0) return null;
      return body['data'] as Map<String, dynamic>?;
    } catch (_) {
      return null;
    }
  }

  Future<MediaUploadResult?> _completeSession(String sessionId) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/media/upload_sessions/$sessionId/complete',
      );
      final body = resp.data;
      if (body == null || body['code'] != 0) return null;
      final data = body['data'] as Map<String, dynamic>?;
      if (data == null) return null;
      final previewUrl = data['previewUrl'] as String? ?? '';
      final stagedToken = data['stagedToken'] as String? ?? '';
      if (previewUrl.isEmpty && stagedToken.isEmpty) return null;
      return MediaUploadResult(
        previewUrl: previewUrl,
        stagedToken: stagedToken,
      );
    } catch (_) {
      return null;
    }
  }

  // ---------------------------------------------------------------------------
  // Image URL resolution (static, no network calls)
  // ---------------------------------------------------------------------------

  /// Resolve an image source to a displayable URL.
  ///
  /// 統一走 [buildImageUrl]（49 §8.6 唯一組裝入口）：完整 URL 先還原 objectKey
  /// 再經 Worker `original` 直出，外部域名無法還原則 passthrough。
  static String? resolveImageUrl(String value) {
    if (value.isEmpty) return null;
    final url = buildImageUrl(value, ImagePreset.original);
    return url.isEmpty ? null : url;
  }

  /// Batch-resolve image sources to displayable URLs.
  static Map<String, String?> resolveImageUrls(List<String> values) {
    final result = <String, String?>{};
    for (final v in values) {
      result[v] = resolveImageUrl(v);
    }
    return result;
  }
}
