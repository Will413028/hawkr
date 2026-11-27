import '../../config/flavor.dart';

class AvatarUtils {
  static String normalizeAvatarUrl(String? url) {
    if (url == null || url.isEmpty) return '';
    if (url.startsWith('http')) return url;
    final base = F.appFlavor.gatewayHost;
    return '$base/$url'.replaceAll('//', '/').replaceFirst(':/', '://');
  }
}
