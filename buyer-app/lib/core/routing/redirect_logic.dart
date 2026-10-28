import 'route_access_policy.dart';

/// 強制更新攔截頁路徑（`packages/hawkr_app_update/docs/spec.md`）。
const String forceUpdateRoute = '/force-update';

/// 登入頁路徑。
const String loginRoute = '/login';

/// GoRouter redirect 的純函式決策核心。
///
/// 抽成 top-level 純函式而非留在 closure 內，是為了讓兩個不變式可被單元測試
/// 直接驗證（`packages/hawkr_app_update/docs/spec.md` 指名的風險緩解手段）：
///
/// 1. **強更判斷早於登入判斷** —— 否則未登入者會先被導去 [loginRoute]，
///    在一個後端已宣告不再支援的版本上繼續操作，永遠看不到更新畫面。
/// 2. **App Store 截圖模式一律豁免強更** —— 否則 Apple App Review 的 guest
///    截圖流程會被更新畫面攔死，App 上不了架。
///
/// 參數一律由 caller 傳入（compile-time const、Riverpod provider、GoRouter
/// state 都在 caller 側取），本函式不讀任何全域狀態。
///
/// - [location]：`state.matchedLocation`
/// - [isScreenshotMode]：`APP_STORE_SCREENSHOT` dart-define
/// - [loginContinueParam]：停在 [loginRoute] 時的 `continue` query 參數
/// - [currentUri]：`state.uri.toString()`，未登入被攔時寫進 `continue`；
///   省略時退回 [location]
String? resolveBuyerRedirect({
  required String location,
  required bool isLoggedIn,
  required bool needsForceUpdate,
  required bool isScreenshotMode,
  String? loginContinueParam,
  String? currentUri,
}) {
  // App 強制更新（`packages/hawkr_app_update/docs/spec.md`）：必須早於登入判斷。截圖模式豁免。
  // [forceUpdateRoute] 自身放行，否則 redirect 自我循環。
  if (!isScreenshotMode && needsForceUpdate) {
    return location == forceUpdateRoute ? null : forceUpdateRoute;
  }

  // Login: if already authenticated, go to continue URL or home
  if (location == loginRoute) {
    if (isLoggedIn) {
      return loginContinueParam ?? '/';
    }
    return null;
  }

  // Apple App Review: guest mode required. All routes default to
  // guest-accessible; only account-scoped routes force login.
  if (!isScreenshotMode && !isLoggedIn && requiresLoginForRoute(location)) {
    final encoded = Uri.encodeComponent(currentUri ?? location);
    return '$loginRoute?continue=$encoded';
  }

  return null;
}
