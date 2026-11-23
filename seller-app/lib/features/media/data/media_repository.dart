import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hawkr_api/gen/hawkr/media/v1alpha1/media.connect.client.dart';
import 'package:hawkr_api/upload.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../config/flavor.dart';
import '../../../config/media_image.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/session/auth_session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/image_processing_service.dart';
import 'media_model.dart';

/// 媒體上傳 Repository：流程在 `hawkr_api` 的 `upload.dart`（公開圖片預簽 PUT 後輪詢 `GetUpload`，
/// 機敏檔分段 PUT 到單體的 `/api/files/secure_uploads`）。
class MediaRepository {
  MediaRepository({
    required MediaAppServiceClient client,
    required UploadHttp http,
    required Uri filesBase,
    required Future<String?> Function() sessionToken,
    required UiLocale Function() locale,
  }) : _client = client,
       _http = http,
       _filesBase = filesBase,
       _sessionToken = sessionToken,
       _locale = locale;

  final MediaAppServiceClient _client;
  final UploadHttp _http;
  final Uri _filesBase;
  final Future<String?> Function() _sessionToken;
  final UiLocale Function() _locale;

  /// 上傳公開圖片，回傳 READY 的 staged token。
  ///
  /// [bytes] 檔案 bytes；[filename] 只用來判斷格式；[compress] 是否壓縮（預設 true）。
  /// 伺服器只收 JPEG、PNG、WebP、GIF，其他格式（例如 HEIC）先經壓縮器轉成 JPEG，轉不成就丟錯。
  Future<MediaUploadResult> uploadWithSession({
    required Uint8List bytes,
    required String filename,
    bool compress = true,
  }) async {
    try {
      var contentType = _getMimeType(filename);
      var finalBytes = bytes;
      final shouldCompress = shouldCompressForUpload(
        bytes: bytes,
        contentType: contentType,
        compress: compress,
      );
      if (shouldCompress || !_serverImageTypes.contains(contentType)) {
        final compressed = await ImageProcessingService.compress(bytes);
        if (compressed != null) {
          finalBytes = compressed;
          contentType = 'image/jpeg';
        }
      }
      final token = await uploadImage(
        client: _client,
        http: _http,
        bytes: finalBytes,
        contentType: contentType,
      );
      HKLog.logD('[MediaRepo] Upload ready: ${finalBytes.length} bytes');
      return MediaUploadResult(stagedToken: token, size: finalBytes.length);
    } on UploadRejected catch (e) {
      throw UploadRejectedException(message: _rejectText(e.reason));
    } on UploadFailure catch (e, st) {
      HKLog.logE('[MediaRepo] Upload failed', e, st);
      throw UnknownException(message: '上傳失敗，請稍後再試');
    } on AppException {
      rethrow;
    } catch (e, st) {
      HKLog.logE('[MediaRepo] Upload error', e, st);
      throw UnknownException(message: '上傳失敗，請稍後再試');
    }
  }

  String _rejectText(RejectReason reason) =>
      reason.uiText(_locale()) ?? '上傳失敗，請稍後再試';

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

  /// 上傳私密文件（伺服器端加密），回傳 staged token。
  Future<String> uploadPrivateDocument({required Uint8List bytes}) async {
    try {
      final token = await _sessionToken();
      if (token == null || token.isEmpty) {
        throw const UnauthorizedException(message: '登入已過期，請重新登入');
      }
      return await uploadSecure(
        client: _client,
        http: _http,
        filesBase: _filesBase,
        headers: () => {'Authorization': 'Bearer $token'},
        bytes: bytes,
      );
    } on UploadFailure catch (e, st) {
      HKLog.logE('[MediaRepo] Private upload failed', e, st);
      throw UnknownException(message: '私密文件上傳失敗，請稍後再試');
    } on AppException {
      rethrow;
    } catch (e, st) {
      HKLog.logE('[MediaRepo] Private upload error', e, st);
      throw UnknownException(message: '私密文件上傳失敗，請稍後再試');
    }
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

const _serverImageTypes = {
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
};

// ============================================================
// Riverpod Provider
// ============================================================

/// MediaRepository Provider
final mediaRepositoryProvider = Provider<MediaRepository>((ref) {
  return MediaRepository(
    client: ref.watch(mediaClientProvider),
    http: IoUploadHttp(),
    filesBase: Uri.parse(F.appFlavor.gatewayHost),
    sessionToken: () => currentSessionToken(ref),
    locale: () => ref.read(uiLocaleProvider),
  );
});
