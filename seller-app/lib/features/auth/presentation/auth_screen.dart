import 'dart:async';

import 'package:hawkr_seller/shared/utils/app_version_utils.dart';
import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/session/session_provider.dart'
    show sessionRestoreErrorProvider;
import '../../../core/theme/app_colors.dart';
import '../../../shared/utils/hk_log.dart';
import '../../seller/data/current_store.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/animated_orbs_background.dart';
import 'auth_controller.dart';
import 'widgets/seller_auth_surface.dart';

class AuthScreen extends HookConsumerWidget {
  const AuthScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final displayMessage = ref.watch(sessionRestoreErrorProvider);
    useEffect(() {
      if (displayMessage != null) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!context.mounted) return;
          FloatingToast.error(context, displayMessage);
          if (ref.read(sessionRestoreErrorProvider) == displayMessage) {
            ref.read(sessionRestoreErrorProvider.notifier).setMessage(null);
          }
        });
      }
      return null;
    }, [displayMessage]);
    // --- State hooks ---
    final appVersionFuture = useMemoized(fetchAppVersion);
    // 進行中的登入方式；一次只進行一個。
    final loadingProvider = useState<OAuthProvider?>(null);

    // --- Local closures ---

    // onboarding 由登入時讀到的 GetMyStore 決定（currentStoreProvider）。
    void navigateAfterAuth(BuildContext ctx) {
      if (!ctx.mounted) return;
      if (ref.read(needsOnboardingProvider) == true) {
        ctx.go('/onboard');
      } else {
        ctx.go('/main/dashboard');
      }
    }

    /// 以 Google、LINE 登入；session 與 onboarding 狀態由 [AuthController.signInWith] 寫好，這裡只負責導頁。
    Future<void> handleSignIn(OAuthProvider provider) async {
      if (loadingProvider.value != null) return;
      loadingProvider.value = provider;
      final label = provider.label;
      try {
        final signedIn = await ref
            .read(authControllerProvider.notifier)
            .signInWith(provider);
        if (!context.mounted) return;
        if (!signedIn) {
          HKLog.logD('[Auth][$label] cancelled by user');
          return;
        }
        unawaited(ref.read(analyticsEventsProvider).login(provider.name));
        FloatingToast.success(context, '$label 登入成功');
        navigateAfterAuth(context);
      } catch (e, st) {
        HKLog.logE('[Auth][$label] error: $e\n$st');
        if (context.mounted) {
          FloatingToast.error(
            context,
            e.toString().replaceFirst('Exception: ', ''),
          );
        }
      } finally {
        if (context.mounted) loadingProvider.value = null;
      }
    }

    return SellerAuthSurface(
      loadingProvider: loadingProvider.value,
      onSignIn: handleSignIn,
      appVersionFuture: appVersionFuture,
      wordmarkBuilder: (width) => _AuthWordmark(width: width),
    );
  }
}

/// 登入頁 logo。
class _AuthWordmark extends StatelessWidget {
  const _AuthWordmark({required this.width});
  final double width;

  @override
  Widget build(BuildContext context) {
    final useDeepGreen = AmbientBrightnessScope.prefersDarkTextOf(context);
    return SvgPicture.asset(
      'assets/brand/logo-horizontal-light.svg',
      key: Key(
        useDeepGreen ? 'auth-footer-forest-logo' : 'auth-footer-ivory-logo',
      ),
      width: width,
      height: width * 66 / 255,
      fit: BoxFit.contain,
      semanticsLabel: 'Hawkr',
      colorFilter: useDeepGreen
          ? const ColorFilter.mode(
              AppColors.brandPrimaryStrong,
              BlendMode.srcIn,
            )
          : null,
    );
  }
}
