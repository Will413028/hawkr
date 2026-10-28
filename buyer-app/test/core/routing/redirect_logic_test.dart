import 'package:hawkr_buyer/core/routing/redirect_logic.dart';
import 'package:flutter_test/flutter_test.dart';

/// `resolveBuyerRedirect` 是 GoRouter redirect 的純函式決策核心。
///
/// 本檔是 `packages/hawkr_app_update/docs/spec.md` 指名的風險緩解手段：兩個不變式（強更早於登入、
/// 截圖模式豁免強更）過去只活在 closure 裡不可測，改壞了只有 Apple 送審當下
/// 才會發現。
void main() {
  group('強制更新 × App Store 截圖模式', () {
    test('截圖模式即使 updateRequired 也不得攔去 /force-update', () {
      expect(
        resolveBuyerRedirect(
          location: '/',
          isLoggedIn: false,
          needsForceUpdate: true,
          isScreenshotMode: true,
        ),
        isNull,
        reason: '截圖模式被強更攔死 = Apple App Review 的 guest 流程走不完',
      );
    });

    test('截圖模式下帳號頁也不得被導去 /login（guest 截圖流程）', () {
      expect(
        resolveBuyerRedirect(
          location: '/orders',
          isLoggedIn: false,
          needsForceUpdate: true,
          isScreenshotMode: true,
        ),
        isNull,
      );
    });

    test('非截圖模式 + updateRequired → 導 /force-update', () {
      expect(
        resolveBuyerRedirect(
          location: '/',
          isLoggedIn: true,
          needsForceUpdate: true,
          isScreenshotMode: false,
        ),
        '/force-update',
      );
    });

    test('updateRequired + 未登入 + 需登入頁 → 導 /force-update 而非 /login', () {
      expect(
        resolveBuyerRedirect(
          location: '/orders',
          isLoggedIn: false,
          needsForceUpdate: true,
          isScreenshotMode: false,
          currentUri: '/orders',
        ),
        '/force-update',
        reason: '強更判斷必須早於登入判斷，否則訪客永遠看不到更新畫面',
      );
    });

    test('updateRequired + 未登入 + 停在 /login → 導 /force-update', () {
      expect(
        resolveBuyerRedirect(
          location: '/login',
          isLoggedIn: false,
          needsForceUpdate: true,
          isScreenshotMode: false,
        ),
        '/force-update',
        reason: '/login 分支不得搶在強更之前 return null',
      );
    });

    test('已在 /force-update → return null（不自我循環）', () {
      expect(
        resolveBuyerRedirect(
          location: '/force-update',
          isLoggedIn: false,
          needsForceUpdate: true,
          isScreenshotMode: false,
        ),
        isNull,
      );
    });
  });

  group('needsForceUpdate=false 時的既有登入行為', () {
    test('已登入停在 /login + 有 continue → 導 continue 目標', () {
      expect(
        resolveBuyerRedirect(
          location: '/login',
          isLoggedIn: true,
          needsForceUpdate: false,
          isScreenshotMode: false,
          loginContinueParam: '/orders',
        ),
        '/orders',
      );
    });

    test('已登入停在 /login + 無 continue → 導首頁', () {
      expect(
        resolveBuyerRedirect(
          location: '/login',
          isLoggedIn: true,
          needsForceUpdate: false,
          isScreenshotMode: false,
        ),
        '/',
      );
    });

    test('未登入停在 /login → 放行', () {
      expect(
        resolveBuyerRedirect(
          location: '/login',
          isLoggedIn: false,
          needsForceUpdate: false,
          isScreenshotMode: false,
        ),
        isNull,
      );
    });

    test('未登入 + 帳號頁 → 導 /login?continue=<encoded>', () {
      expect(
        resolveBuyerRedirect(
          location: '/orders',
          isLoggedIn: false,
          needsForceUpdate: false,
          isScreenshotMode: false,
          currentUri: '/orders?tab=3',
        ),
        '/login?continue=${Uri.encodeComponent('/orders?tab=3')}',
      );
    });

    test('未登入 + 公開頁 → 放行', () {
      expect(
        resolveBuyerRedirect(
          location: '/merchant/2683382637',
          isLoggedIn: false,
          needsForceUpdate: false,
          isScreenshotMode: false,
        ),
        isNull,
      );
    });

    test('已登入 + 帳號頁 → 放行', () {
      expect(
        resolveBuyerRedirect(
          location: '/orders',
          isLoggedIn: true,
          needsForceUpdate: false,
          isScreenshotMode: false,
        ),
        isNull,
      );
    });
  });
}
