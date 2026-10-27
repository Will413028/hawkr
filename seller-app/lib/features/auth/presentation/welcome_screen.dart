import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/brand_startup_splash.dart';
import 'auto_login_controller.dart';

class WelcomeScreen extends HookConsumerWidget {
  const WelcomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final bootstrapping = useState(false);
    final navigated = useState(false);

    void goTo(String path) {
      if (!context.mounted || navigated.value) return;
      // 防禦（issue #1493 Finding 2 根因）：bootstrap() 是多段 await 長鏈
      // （讀 storage 再打 /me，實測可達 2 秒以上），執行期間頂層
      // redirect（`redirect_logic.dart` 的登入/onboarding/停權 gating，SSOT）
      // 可能已把使用者導去別處（例如停權賣家被導到公告頁、再自行前往白名單
      // 頁）。`MaterialPageRoute` 預設 `maintainState=true`，State 在退場動畫
      // 完成前仍 `context.mounted=true`，擋不住這個 race——bootstrap 結束時
      // 仍會盲目呼叫這裡的 goTo（例：無條件 `goTo('/main/dashboard')`），
      // 蓋掉使用者當下實際所在頁面，讓其被頂層 redirect 二次攔截彈回。
      // 用 `ModalRoute.isCurrent` 判斷本頁是否仍是 Navigator 當前頁——不是
      // 就代表這個導頁決策已過期，放棄執行，交還給頂層 redirect（唯一真相
      // 來源）決定使用者該在哪。
      if (ModalRoute.of(context)?.isCurrent != true) {
        HKLog.logD('【Welcome】bootstrap 決策已過期（本頁非當前路由），略過導頁 $path');
        return;
      }
      navigated.value = true;
      context.go(path);
    }

    Future<void> bootstrap() async {
      if (bootstrapping.value || navigated.value) return;
      bootstrapping.value = true;
      // 先取 notifier 再 await：await 之後只用 context（goTo 自己檢查 mounted），不再碰 ref。
      final autoLogin = ref.read(autoLoginControllerProvider.notifier);
      try {
        switch (await autoLogin.check()) {
          case AutoLoginSignedOut():
            goTo('/auth');
          case AutoLoginSignedIn(needsOnboarding: true):
            HKLog.logD('【Welcome】needsOnboarding=true，跳轉到商家資訊頁面');
            goTo('/onboard');
          case AutoLoginSignedIn():
            goTo('/main/dashboard');
        }
      } finally {
        // 同上 goTo() 的 race：State 可能已在這個 await 鏈結束前被 dispose
        // （route 早已不 current），此時對已 dispose 的 hook state 賦值會
        // 拋 ValueNotifier-used-after-dispose——這裡也一併補上 mounted 檢查。
        if (context.mounted) {
          bootstrapping.value = false;
        }
      }
    }

    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (context.mounted) bootstrap();
      });
      return null;
    }, const []);

    // 啟動期間只維持同一套 Hawkr 品牌載入動畫；登入狀態與賣家資料完成
    // 判斷後直接切往 auth、onboard 或 dashboard，不再插入牛牛歡迎頁。
    return const BrandStartupSplash();
  }
}
