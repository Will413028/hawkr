/// 跨環境不變的全域常數
///
/// 只放真正的 const——Payment 設定、OSS CDN base 等。
/// Flavor-dependent 的設定（URL、timeout 等）請用 [F.appFlavor.<getter>]。
abstract final class AppConstants {
  // ============================================================
  // Payment
  // ============================================================

  /// Deep link scheme for payment callback
  static const String paymentDeepLinkScheme = 'hawkrbuyer';

  // ============================================================
  // 分享連結
  // ============================================================

  /// 官網公開網址：`--dart-define=PUBLIC_SITE_ORIGIN=https://...`。
  /// 沒有提供時，伺服器沒給分享連結的情況下不開分享面板。
  static const String publicSiteOrigin = String.fromEnvironment(
    'PUBLIC_SITE_ORIGIN',
  );

  /// 以 [origin] 組出官網上 [path] 的分享連結；[origin] 為空時回 `null`（分享停用）。
  static String? shareFallbackUrl(
    String path, {
    String origin = publicSiteOrigin,
  }) {
    final base = origin.trim().replaceAll(RegExp(r'/+$'), '');
    if (base.isEmpty) return null;
    return '$base$path';
  }

  // Apple Pay merchant id 依環境隔離（dev/stage 用 .sandbox），
  // 故移至 [FlavorExtension.applePayMerchantId]，此處不再持有。
}
