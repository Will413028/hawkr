import '../../config/media_image.dart';

extension AvatarUrlExtension on String {
  /// Normalizes an avatar URL string.
  ///
  /// Handles R2 媒體 objectKey、外部 URL（Google OAuth 頭像）與空值。
  /// Returns a fully qualified URL that can be used directly.
  String normalizeAvatarUrl() {
    final v = trim();
    if (v.isEmpty) return v;
    if (v.startsWith('http')) return v;

    // R2 媒體 objectKey（49 號遷移後後端回裸 key，如 public/buyer_avatar/...）→
    // 走媒體 URL helper（avatar preset）；buildImageUrl 組 {mediaBaseHost}/i/avatar/{key}，
    // base 跟隨 flavor（dev→test-media、prod→media），外部 URL 已於上方 http 分支
    // passthrough。
    if (v.startsWith('public/')) {
      return buildImageUrl(v, ImagePreset.avatar);
    }

    // 未知格式：回原值 fallback。
    return v;
  }
}
