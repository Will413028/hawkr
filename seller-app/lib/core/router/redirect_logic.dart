import 'package:go_router/go_router.dart';

import '../session/my_store.dart';
import 'app_router.dart';
import 'route_capabilities.dart';

/// redirect 用的 location：路由樣式（`/seller/measures/:measureId`），沒有路徑參數的路由與實際網址相同。
/// 能力表與放行規則都以樣式為鍵，所以含 id 的網址不需要逐一登記。
String redirectLocationOf(GoRouterState state) {
  final pattern = state.fullPath;
  return pattern == null || pattern.isEmpty ? state.matchedLocation : pattern;
}

/// GoRouter redirect 的純函式決策核心，也是 onboarding gating 的**單一真相來源**。
///
/// 設計緣由：原本 redirect 只看 JWT（已登入在 /auth 一律導 dashboard），
/// 而 `navigateAfterAuth` 另外看 `needsOnboarding`，兩套判斷在「刪除帳號後
/// 同一次 app 執行內重登」時競態——redirect 先把停在 /auth 的使用者導去
/// dashboard，蓋過 onboarding 導向。把 onboarding 判斷收斂到這裡，讓 redirect
/// 認得 `needsOnboarding`，競態即消失（兩處殊途同歸）。
///
/// [location] 是路由樣式（有路徑參數的路由是 `/seller/measures/:measureId`），不是實際網址。
///
/// 注意：Firebase phone-auth deep link 的豁免由 caller（redirect closure）處理，
/// 不在此純函式範圍內。
String? resolveAuthRedirect({
  required bool isLoggedIn,
  required bool? needsOnboarding,
  required MyStore? myStore,
  required String location,
  bool needsForceUpdate = false,
  bool storeUnavailable = false,
  bool accountRestricted = false,
}) {
  if (needsForceUpdate) {
    return location == SellerRoutes.forceUpdate
        ? null
        : SellerRoutes.forceUpdate;
  }

  final isWhitelisted =
      location == AuthRoutes.welcome ||
      location.startsWith('/auth') ||
      location == AuthRoutes.onboard;

  if (!isLoggedIn) {
    return isWhitelisted ? null : AuthRoutes.auth;
  }

  if (accountRestricted) {
    return location == SellerRoutes.accountRestricted
        ? null
        : SellerRoutes.accountRestricted;
  }

  if (storeUnavailable) {
    return location == SellerRoutes.storeUnavailable
        ? null
        : SellerRoutes.storeUnavailable;
  }

  if (needsOnboarding == null) return null;

  if (needsOnboarding) {
    return routesAllowedWithoutStore.contains(location)
        ? null
        : AuthRoutes.onboard;
  }

  if (isBlockedByCapabilities(myStore, location)) {
    return SellerRoutes.suspensionNotice;
  }

  if (location == AuthRoutes.auth ||
      location == SellerRoutes.storeUnavailable ||
      location == SellerRoutes.accountRestricted) {
    return MainRoutes.dashboard;
  }
  return null;
}
