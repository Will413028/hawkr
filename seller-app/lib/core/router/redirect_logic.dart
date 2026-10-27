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
}) {
  // App 版本低於後端設定的最低可用版本（54 號設計書 §4.1）：**先於所有其他
  // 判斷**。未登入者也要被攔——否則會先被導去 /login，在一個後端已宣告不再
  // 支援的版本上繼續操作。/force-update 自身放行，否則 redirect 自我循環。
  //
  // 刻意**不**併進能力表（[routeCapabilities]）：能力表的語意是「能力被收回時仍可用的
  // 頁面」，強更沒有任何可用頁。
  if (needsForceUpdate) {
    return location == SellerRoutes.forceUpdate
        ? null
        : SellerRoutes.forceUpdate;
  }

  final isWhitelisted =
      location == AuthRoutes.welcome ||
      location.startsWith('/auth') ||
      location == AuthRoutes.onboard;

  // 未登入：只允許白名單路徑，其餘踢回登入頁。
  if (!isLoggedIn) {
    return isWhitelisted ? null : AuthRoutes.auth;
  }

  // 已登入、GetMyStore 讀取失敗而且沒有上一次的快照：不知道有沒有店家，不能放行進主畫面（fail-closed），
  // 顯示重試頁。
  if (storeUnavailable) {
    return location == SellerRoutes.storeUnavailable
        ? null
        : SellerRoutes.storeUnavailable;
  }

  // 已登入、GetMyStore 還沒回答（needsOnboarding 是 null）：還不知道該去 onboarding 還是主畫面，不導頁，
  // 停在目前的載入狀態（登入頁的按鈕、welcome 的 splash），等 GetMyStore 的答案。
  if (needsOnboarding == null) return null;

  // 已登入但尚未完成 onboarding（新申請 / 刪除後重登）：
  // 除了沒有店家也能進的頁面（[routesAllowedWithoutStore]：onboarding 本身、welcome 的 splash、帳號層級的措施與申覆）
  // 外，一律擋去 onboarding——涵蓋重登後停在 /auth、以及任何主畫面穿透。
  if (needsOnboarding) {
    return routesAllowedWithoutStore.contains(location)
        ? null
        : AuthRoutes.onboard;
  }

  // 已登入且已完成 onboarding：這個頁面需要的能力被收回（停權、等待刪除；46 號設計書
  // 「賣家能力權限層」§7.2）就導公告頁。能力來自 GetMyStore，頁面對應的能力在
  // [routeCapabilities]。
  if (isBlockedByCapabilities(myStore, location)) {
    return SellerRoutes.suspensionNotice;
  }

  // 已登入且已完成 onboarding：停在登入頁時推進主畫面，其餘放行。
  if (location == AuthRoutes.auth ||
      location == SellerRoutes.storeUnavailable) {
    return MainRoutes.dashboard;
  }
  return null;
}
