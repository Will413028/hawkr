import '../../config/flavor.dart';

/// URL 工具函式（Gateway proxy URL 轉換）
class UrlUtils {
  UrlUtils._();

  /// Gateway base URL
  static String get baseUrl => F.appFlavor.gatewayHost;

  /// 將相對路徑或 OSS URL 轉換為 Gateway 絕對 URL
  static String toAbsoluteUrl(String? path) {
    if (path == null || path.isEmpty) return '';
    if (path.contains('oss-ap-southeast') || path.contains('.aliyuncs.com')) {
      final proxyPath = _extractGatewayPath(path);
      if (proxyPath != null && proxyPath.isNotEmpty) {
        return '$baseUrl$proxyPath';
      }
    }
    if (path.startsWith('http://') || path.startsWith('https://')) return path;
    return '$baseUrl${path.startsWith('/') ? path : '/$path'}';
  }

  /// 從 OSS URL 提取路徑，加上 Gateway proxy 前綴 `/api/v1/files/`
  static String? _extractGatewayPath(String ossUrl) {
    try {
      final uri = Uri.parse(ossUrl);
      final path = uri.path;
      if (path.isEmpty) return null;
      if (path.startsWith('/')) return '/api/v1/files$path';
      return '/api/v1/files/$path';
    } catch (_) {
      return null;
    }
  }
}
