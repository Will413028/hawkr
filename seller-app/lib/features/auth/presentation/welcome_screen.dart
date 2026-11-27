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

    return const BrandStartupSplash();
  }
}
