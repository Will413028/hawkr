import '../../config/media_image.dart';

/// 圖片來源到可顯示網址的解析（靜態，沒有網路呼叫）。
class OssService {
  const OssService._();

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
