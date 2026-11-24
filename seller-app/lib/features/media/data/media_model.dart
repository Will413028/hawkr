import 'package:freezed_annotation/freezed_annotation.dart';

part 'media_model.freezed.dart';
part 'media_model.g.dart';

/// 公開圖片上傳完成的結果；[stagedToken] 交給擁有者的 API 轉正。
@freezed
abstract class MediaUploadResult with _$MediaUploadResult {
  const factory MediaUploadResult({
    /// Staged token（`GetUpload` 為 READY 後才回傳）
    required String stagedToken,

    /// 上傳的位元組數
    required int size,
  }) = _MediaUploadResult;

  factory MediaUploadResult.fromJson(Map<String, dynamic> json) =>
      _$MediaUploadResultFromJson(json);
}
