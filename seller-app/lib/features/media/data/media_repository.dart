import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../config/flavor.dart';
import '../../../config/media_image.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/image_processing_service.dart';
import 'media_model.dart';

/// 媒體上傳 Repository（Dio-based）
///
/// 從 `MediaUploadService` 提取，使用 Dio provider 管理 HTTP 連線。
/// 上傳流程：CreateSession → PUT to OSS → CompleteSession → stagedToken
class MediaRepository {
  final Dio _dio;

  MediaRepository(this._dio);

  /// 媒體 API base path
  static const String _mediaPath = '/api/v1/app/media';

  // ============================================================
  // Session 上傳（核心流程）
  // ============================================================

  /// 使用 Upload Session 流程上傳媒體檔案
  ///
  /// 流程：CreateSession → PUT to OSS → CompleteSession → stagedToken
  ///
  /// [bytes] 檔案 bytes
  /// [filename] 檔案名稱
  /// [compress] 是否壓縮（預設 true）
  Future<MediaUploadResult> uploadWithSession({
    required Uint8List bytes,
    required String filename,
    bool compress = true,
  }) async {
    try {
      final contentType = _getMimeType(filename);

      // 1. 可選壓縮
      Uint8List finalBytes = bytes;
      if (shouldCompressForUpload(
        bytes: bytes,
        contentType: contentType,
        compress: compress,
      )) {
        final compressed = await ImageProcessingService.compress(bytes);
        if (compressed != null) {
          finalBytes = compressed;
        }
      }

      // 2. 建立上傳會話
      final session = await _createUploadSession(
        filename: filename,
        contentType: contentType,
        fileSize: finalBytes.length,
      );

      // 3. PUT 檔案到 OSS（直連 OSS，不走 Dio interceptor）
      await _putToOss(
        uploadUrl: session.uploadUrl,
        bytes: finalBytes,
        contentType: contentType,
      );

      // 4. 完成會話 → 取得 stagedToken
      final result = await _completeUploadSession(
        sessionId: session.sessionId,
        objectKey: session.objectKey,
        fileSize: finalBytes.length,
      );

      HKLog.logD(
        '[MediaRepo] Session upload success: stagedToken=${result.stagedToken != null ? "present" : "null"}, previewUrl=${result.previewUrl?.isNotEmpty == true ? "present" : "empty"}',
      );
      return result;
    } on AppException {
      rethrow;
    } catch (e, st) {
      HKLog.logE('[MediaRepo] Session upload error', e, st);
      throw UnknownException(message: '上傳失敗: $e');
    }
  }

  /// 建立上傳會話
  Future<UploadSession> _createUploadSession({
    required String filename,
    required String contentType,
    required int fileSize,
    bool isSecure = false,
  }) async {
    try {
      final resp = await _dio.post(
        '$_mediaPath/upload_sessions',
        data: {
          'filename': filename,
          'contentType': contentType,
          'fileSize': fileSize.toString(),
          if (isSecure) 'isSecure': true,
        },
      );

      final data = resp.data['data'] as Map<String, dynamic>;
      return UploadSession.fromJson(data);
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  /// PUT 檔案到 OSS（直連 OSS presigned URL）
  Future<void> _putToOss({
    required String uploadUrl,
    required Uint8List bytes,
    required String contentType,
  }) async {
    try {
      // 使用獨立 Dio 實例直連 OSS（不走 app interceptor）
      final ossDio = Dio(
        BaseOptions(
          connectTimeout: const Duration(seconds: 30),
          sendTimeout: const Duration(seconds: 120),
          receiveTimeout: const Duration(seconds: 30),
        ),
      );

      await ossDio.put(
        uploadUrl,
        data: Stream.fromIterable([bytes]),
        options: Options(
          headers: {
            'Content-Type': contentType,
            'Content-Length': bytes.length,
          },
        ),
      );
    } on DioException catch (e) {
      throw NetworkException(message: 'OSS 上傳失敗: ${e.message}');
    }
  }

  /// 完成上傳會話
  Future<MediaUploadResult> _completeUploadSession({
    required String sessionId,
    required String objectKey,
    required int fileSize,
  }) async {
    try {
      final resp = await _dio.post(
        '$_mediaPath/upload_sessions/$sessionId/complete',
      );

      final data = resp.data['data'] as Map<String, dynamic>;
      final stagedToken = data['stagedToken'] as String;
      final previewUrl = data['previewUrl'] as String? ?? '';

      return MediaUploadResult(
        url: previewUrl,
        fileUrl: previewUrl,
        objectKey: objectKey,
        size: fileSize,
        stagedToken: stagedToken,
        previewUrl: previewUrl,
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    }
  }

  // ============================================================
  // 頭像上傳
  // ============================================================

  /// 上傳頭像（Session 流程 + 前端預處理）
  Future<MediaUploadResult> uploadAvatar({
    required Uint8List bytes,
    bool processOnClient = true,
  }) async {
    Uint8List finalBytes = bytes;
    if (processOnClient) {
      final processed = await ImageProcessingService.processAvatar(bytes);
      if (processed != null) {
        finalBytes = processed;
      }
    }

    HKLog.logD('[MediaRepo] Uploading avatar: ${finalBytes.length} bytes');
    return uploadWithSession(
      bytes: finalBytes,
      filename: 'avatar_${DateTime.now().millisecondsSinceEpoch}.jpg',
      compress: false, // 已預處理
    );
  }

  // ============================================================
  // 聊天圖片上傳
  // ============================================================

  /// 上傳聊天圖片
  Future<MediaUploadResult> uploadChatImage({required Uint8List bytes}) async {
    HKLog.logD('[MediaRepo] Uploading chat image: ${bytes.length} bytes');
    return uploadWithSession(
      bytes: bytes,
      filename: 'chat_${DateTime.now().millisecondsSinceEpoch}.jpg',
      compress: true,
    );
  }

  // ============================================================
  // 批次上傳
  // ============================================================

  /// 批次上傳圖片（最多 9 個）
  Future<List<MediaUploadResult>> uploadBatch({
    required List<({Uint8List bytes, String filename})> files,
    bool compress = true,
  }) async {
    if (files.isEmpty) return [];

    var fileList = files;
    if (fileList.length > 9) {
      HKLog.logW('[MediaRepo] Batch limited to 9 files');
      fileList = fileList.sublist(0, 9);
    }

    HKLog.logD('[MediaRepo] Batch upload: ${fileList.length} files');

    final futures = fileList.map(
      (f) => uploadWithSession(
        bytes: f.bytes,
        filename: f.filename,
        compress: compress,
      ),
    );

    final results = <MediaUploadResult>[];
    for (final future in futures) {
      try {
        results.add(await future);
      } catch (e) {
        HKLog.logW('[MediaRepo] Batch item failed: $e');
      }
    }

    HKLog.logD(
      '[MediaRepo] Batch complete: ${results.length}/${fileList.length}',
    );
    return results;
  }

  // ============================================================
  // 私密文件上傳
  // ============================================================

  /// 上傳私密文件（伺服器端加密）
  Future<PrivateUploadResult> uploadPrivateDocument({
    required Uint8List bytes,
    required String docType,
    String? filename,
  }) async {
    try {
      final actualFilename =
          filename ?? '${docType}_${DateTime.now().millisecondsSinceEpoch}.jpg';
      final contentType = _getMimeType(actualFilename);

      HKLog.logD('[MediaRepo] Uploading private document: docType=$docType');

      // Step 1: 建立安全上傳會話
      final session = await _createUploadSession(
        filename: actualFilename,
        contentType: contentType,
        fileSize: bytes.length,
        isSecure: true,
      );

      // Step 2: PUT 到 secure_upload 端點
      final resp = await _dio.put(
        '$_mediaPath/secure_upload/${session.sessionId}',
        data: Stream.fromIterable([bytes]),
        options: Options(
          headers: {
            'Content-Type': contentType,
            'Content-Length': bytes.length,
          },
        ),
      );

      final data = resp.data['data'] as Map<String, dynamic>?;
      final stagedToken = data?['stagedToken'] as String? ?? '';

      HKLog.logD(
        '[MediaRepo] Private upload success: sessionId=${session.sessionId}',
      );

      return PrivateUploadResult(
        referenceId: session.sessionId,
        objectKey: session.objectKey,
        originalSize: bytes.length,
        docType: docType,
        stagedToken: stagedToken,
      );
    } on DioException catch (e) {
      throw AppException.fromDioException(e);
    } catch (e, st) {
      HKLog.logE('[MediaRepo] Private upload error', e, st);
      throw UnknownException(message: '私密文件上傳失敗: $e');
    }
  }

  // ============================================================
  // URL 工具方法
  // ============================================================

  /// 取得私密檔案下載 URL（絕對路徑）
  String getSecureDownloadUrl({required String assetId}) {
    return '${F.appFlavor.gatewayHost}$_mediaPath/secure_download/$assetId';
  }

  /// 取得上傳會話預覽 URL（絕對路徑）
  String getSecurePreviewUrl({required String sessionId}) {
    return '${F.appFlavor.gatewayHost}$_mediaPath/secure_preview/$sessionId';
  }

  // ============================================================
  // 媒體 URL 解析（靜態，無網路呼叫）
  // ============================================================

  /// 將 objectKey 或 URL 轉換為可顯示的 URL
  ///
  /// 統一走 [buildImageUrl]（49 §8.6 唯一組裝入口）：完整 URL 先還原 objectKey
  /// 再經 Worker `original` 直出，外部域名無法還原則 passthrough。
  static String? resolveUrl(String? value) {
    if (value == null || value.isEmpty) return null;
    final url = buildImageUrl(value, ImagePreset.original);
    return url.isEmpty ? null : url;
  }

  /// 批量將 objectKey 轉換為 CDN URL
  static Map<String, String> resolveUrls(List<String> objectKeys) {
    final result = <String, String>{};
    for (final key in objectKeys) {
      final url = resolveUrl(key);
      if (url != null) result[key] = url;
    }
    return result;
  }

  /// 檢查是否為 OSS objectKey 格式（非完整 URL）
  static bool isObjectKey(String value) {
    if (value.isEmpty) return false;
    return !value.startsWith('http') && value.contains('/');
  }

  /// 從媒體 URL 提取 objectKey
  static String? extractObjectKey(String url) {
    final uri = Uri.tryParse(url);
    if (uri != null && uri.path.isNotEmpty) {
      return uri.path.startsWith('/') ? uri.path.substring(1) : uri.path;
    }
    return null;
  }

  // ============================================================
  // 內部工具
  // ============================================================

  /// 是否要在上傳前進入 JPEG 壓縮器。
  ///
  /// 宣告為 GIF 的媒體不進入 JPEG 壓縮器；貼文層會先依檔頭產生正確的
  /// `.gif` filename/MIME，再以 `compress: false` 呼叫此層。
  ///
  /// 這裡刻意不覆寫副檔名與 MIME，避免聊天、頭像等既有固定格式流程
  /// 因單看 magic bytes 而改變全域上傳契約。
  static bool shouldCompressForUpload({
    required Uint8List bytes,
    required String contentType,
    bool compress = true,
  }) {
    if (!compress || !ImageProcessingService.needsCompression(bytes)) {
      return false;
    }

    final normalizedContentType = contentType
        .split(';')
        .first
        .trim()
        .toLowerCase();
    return normalizedContentType != 'image/gif';
  }

  /// 根據檔名獲取 MIME type
  static String _getMimeType(String filename) {
    final ext = filename.split('.').last.toLowerCase();
    return switch (ext) {
      'jpg' || 'jpeg' => 'image/jpeg',
      'png' => 'image/png',
      'gif' => 'image/gif',
      'webp' => 'image/webp',
      'heic' || 'heif' => 'image/heic',
      'mp4' => 'video/mp4',
      'mov' => 'video/quicktime',
      'pdf' => 'application/pdf',
      _ => 'application/octet-stream',
    };
  }
}

// ============================================================
// Riverpod Provider
// ============================================================

/// MediaRepository Provider
final mediaRepositoryProvider = Provider<MediaRepository>((ref) {
  final dio = ref.watch(dioProvider);
  return MediaRepository(dio);
});
