import 'package:freezed_annotation/freezed_annotation.dart';

part 'media_model.freezed.dart';

/// 圖片上傳結果
@freezed
abstract class UploadResult with _$UploadResult {
  const factory UploadResult({
    /// 可讀的圖片預覽 URL
    required String previewUrl,

    /// 暫存 Token，用於後端建立/更新資源時傳遞
    required String stagedToken,
  }) = _UploadResult;
}
