import 'package:freezed_annotation/freezed_annotation.dart';

part 'media_model.freezed.dart';
part 'media_model.g.dart';

// ============================================================
// Upload Session（CreateSession API response）
// ============================================================

/// 上傳會話（CreateSession API 返回）
@freezed
abstract class UploadSession with _$UploadSession {
  const factory UploadSession({
    /// 上傳會話 ID（UUID）
    required String sessionId,

    /// OSS 預簽名上傳 URL（isSecure=true 時為空）
    @Default('') String uploadUrl,

    /// OSS 物件路徑
    @Default('') String objectKey,
  }) = _UploadSession;

  factory UploadSession.fromJson(Map<String, dynamic> json) =>
      _$UploadSessionFromJson(json);
}

// ============================================================
// Media Upload Result（Session 上傳結果）
// ============================================================

/// 媒體上傳結果（Session 流程）
@freezed
abstract class MediaUploadResult with _$MediaUploadResult {
  const MediaUploadResult._();

  const factory MediaUploadResult({
    /// CDN URL（優先使用）
    @Default('') String url,

    /// Gateway URL（備用）
    @Default('') String fileUrl,

    /// 物件路徑
    required String objectKey,

    /// 檔案大小（bytes）
    required int size,

    /// 原始大小（壓縮前）
    int? originalSize,

    /// 原始檔名
    String? filename,

    /// Staged Token（Session 上傳流程產生，用於 finalize / product creation）
    String? stagedToken,

    /// 預覽 URL（有效期 24 小時）
    String? previewUrl,
  }) = _MediaUploadResult;

  factory MediaUploadResult.fromJson(Map<String, dynamic> json) =>
      _$MediaUploadResultFromJson(json);

  /// 壓縮比例
  String? get compressionRatio {
    if (originalSize == null || originalSize == 0) return null;
    final ratio = (1 - size / originalSize!) * 100;
    return '${ratio.toStringAsFixed(1)}%';
  }
}

// ============================================================
// Private Upload Result（私密檔案上傳結果）
// ============================================================

/// 私密檔案上傳結果
@freezed
abstract class PrivateUploadResult with _$PrivateUploadResult {
  const factory PrivateUploadResult({
    /// Upload Session ID（用於追蹤）
    required String referenceId,

    /// 物件路徑
    required String objectKey,

    /// 原始大小
    required int originalSize,

    /// 文件類型（id_card, bank_account, contract, document）
    required String docType,

    /// Staged Token（用於 finalize 轉為正式 asset）
    String? stagedToken,
  }) = _PrivateUploadResult;

  factory PrivateUploadResult.fromJson(Map<String, dynamic> json) =>
      _$PrivateUploadResultFromJson(json);
}

// ============================================================
// Finalized Asset（暫存 → 正式資產）
// ============================================================

/// Finalize 後的正式資產
@freezed
abstract class FinalizedAsset with _$FinalizedAsset {
  const factory FinalizedAsset({
    required String stagedToken,
    required String assetId,
    required String objectKey,
  }) = _FinalizedAsset;

  factory FinalizedAsset.fromJson(Map<String, dynamic> json) =>
      _$FinalizedAssetFromJson(json);
}
