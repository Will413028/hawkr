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

    if (v.startsWith('public/')) {
      return buildImageUrl(v, ImagePreset.avatar);
    }

    return v;
  }
}
