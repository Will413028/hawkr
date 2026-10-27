import 'package:flutter_svg/flutter_svg.dart';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/app_version_utils.dart';
import '../../../shared/widgets/animated_orbs_background.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../core/analytics/analytics_providers.dart';
import 'auth_controller.dart';
import 'widgets/auth_action_glass_panel.dart';
import 'widgets/auth_back_button.dart';
import 'widgets/auth_entrance_sequence.dart';
import 'widgets/auth_moon_hero.dart';
import 'widgets/auth_oauth_pill_button.dart';

export 'widgets/auth_oauth_pill_button.dart' show AuthOAuthPillButton;

class AuthScreen extends HookConsumerWidget {
  const AuthScreen({super.key, this.continueUrl});
  final String? continueUrl;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // 進行中的登入方式；一次只進行一個。
    final loadingProvider = useState<OAuthProvider?>(null);
    final appVersionFuture = useMemoized(fetchAppVersion);

    Future<void> handleSignIn(OAuthProvider provider) async {
      if (loadingProvider.value != null) return;
      loadingProvider.value = provider;
      FocusScope.of(context).unfocus();

      try {
        final signedIn = await ref
            .read(authControllerProvider.notifier)
            .signInWith(provider);
        if (!signedIn || !context.mounted) return;
        // session 已由 AuthController 寫入；只負責導頁。
        await ref.read(analyticsEventsProvider).trackLogin(provider.name);
        if (!context.mounted) return;
        context.go(continueUrl ?? '/');
      } catch (e) {
        if (!context.mounted) return;
        showCommonSnackBar(
          context,
          e.toString().replaceFirst('Exception: ', ''),
        );
      } finally {
        if (context.mounted) loadingProvider.value = null;
      }
    }

    final media = MediaQuery.of(context);
    final bool keyboardVisible = media.viewInsets.bottom > 0;
    final double screenHeight = media.size.height;
    final bool largeTextLayout = media.textScaler.scale(16) / 16 >= 1.1;
    // Apple Guideline 4.8：提供其他第三方登入就必須提供 SIWA；App 只發 iOS，一律顯示。

    void handleBack() {
      // Apple App Review 合規：訪客可從登入頁返回繼續瀏覽。
      if (context.canPop()) {
        context.pop();
      } else {
        context.go('/');
      }
    }

    return Scaffold(
      resizeToAvoidBottomInset: true,
      backgroundColor: AppColors.bgPrimary,
      body: AnimatedOrbsBackground(
        child: AuthEntranceSequence(
          child: SafeArea(
            child: Stack(
              children: [
                LayoutBuilder(
                  builder: (context, constraints) {
                    final horizontalPadding = largeTextLayout
                        ? HkSpace.lg
                        : HkBreakpoints.pageGutter(constraints.maxWidth);
                    const verticalPadding = HkSpace.lg;
                    return SingleChildScrollView(
                      padding: EdgeInsets.symmetric(
                        horizontal: horizontalPadding,
                        vertical: verticalPadding,
                      ),
                      child: ConstrainedBox(
                        constraints: BoxConstraints(
                          minHeight:
                              (constraints.maxHeight - verticalPadding * 2)
                                  .clamp(0, double.infinity),
                        ),
                        child: _buildHeroLoginLayout(
                          constraints: constraints,
                          contentWidth:
                              constraints.maxWidth - horizontalPadding * 2,
                          screenHeight: screenHeight,
                          keyboardVisible: keyboardVisible,
                          largeTextLayout: largeTextLayout,
                          loadingProvider: loadingProvider.value,
                          onSignIn: handleSignIn,
                          appVersionFuture: appVersionFuture,
                        ),
                      ),
                    );
                  },
                ),
                Positioned(
                  // The 36dp visual stays at the original 16/8 position while
                  // the surrounding hit target grows to the 48dp minimum.
                  top: 2.h,
                  left: 10.w,
                  child: AuthBackButton(onTap: handleBack),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildHeroLoginLayout({
    required BoxConstraints constraints,
    required double contentWidth,
    required double screenHeight,
    required bool keyboardVisible,
    required bool largeTextLayout,
    required OAuthProvider? loadingProvider,
    required ValueChanged<OAuthProvider> onSignIn,
    required Future<String> appVersionFuture,
  }) {
    final double maxWidth = contentWidth.clamp(280.0, 420.0);
    final double availableHeight = constraints.maxHeight.isFinite
        ? constraints.maxHeight
        : screenHeight;
    final compact = keyboardVisible || availableHeight < 700;
    final moonSize = (maxWidth * 0.38).clamp(
      compact ? 104.0 : 124.0,
      compact ? 136.0 : 168.0,
    );
    final logoWidth = (maxWidth * 0.42).clamp(128.0, 172.0);
    return Column(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Center(
          child: SizedBox(
            width: maxWidth,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(height: compact ? HkSpace.xs : HkSpace.md),
                AuthEntranceItem(
                  stage: AuthEntranceStage.wordmark,
                  child: AuthFooterBrandLogo(width: logoWidth),
                ),
                SizedBox(height: compact ? HkSpace.sm : HkSpace.md),
                AuthEntranceItem(
                  stage: AuthEntranceStage.moon,
                  child: AuthMoonHero(size: moonSize),
                ),
                const SizedBox(height: HkSpace.sm),
                AuthEntranceItem(
                  stage: AuthEntranceStage.panel,
                  child: _buildHeroCard(
                    cardWidth: maxWidth,
                    loadingProvider: loadingProvider,
                    onSignIn: onSignIn,
                    largeTextLayout: largeTextLayout,
                  ),
                ),
              ],
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.only(top: HkSpace.xl),
          child: AuthEntranceItem(
            stage: AuthEntranceStage.footer,
            child: _buildFooter(appVersionFuture),
          ),
        ),
      ],
    );
  }

  Widget _buildHeroCard({
    required double cardWidth,
    required OAuthProvider? loadingProvider,
    required ValueChanged<OAuthProvider> onSignIn,
    required bool largeTextLayout,
  }) {
    return Builder(
      builder: (context) {
        final useDarkInk = AmbientBrightnessScope.prefersDarkTextOf(context);
        return SizedBox(
          key: const ValueKey('auth-action-glass-panel'),
          width: cardWidth,
          child: AuthActionGlassPanel(
            useDarkInk: useDarkInk,
            child: _buildHeroCardContent(
              loadingProvider: loadingProvider,
              onSignIn: onSignIn,
              largeTextLayout: largeTextLayout,
            ),
          ),
        );
      },
    );
  }

  Widget _buildHeroCardContent({
    required OAuthProvider? loadingProvider,
    required ValueChanged<OAuthProvider> onSignIn,
    required bool largeTextLayout,
  }) {
    Widget button(OAuthProvider provider) => AuthOAuthPillButton(
      provider: provider,
      text: '透過${provider.label}繼續',
      isLoading: loadingProvider == provider,
      isDisabled: loadingProvider != null && loadingProvider != provider,
      onTap: () => onSignIn(provider),
    );
    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: largeTextLayout ? HkSpace.sm : HkSpace.lg,
        vertical: largeTextLayout ? HkSpace.xl : HkSpace.xxl,
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          AuthEntranceItem(
            stage: AuthEntranceStage.headline,
            child: Builder(
              builder: (ctx) {
                final dark = AmbientBrightnessScope.prefersDarkTextOf(ctx);
                return Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      '歡迎來到 Hawkr',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontSize: 24,
                        height: 1.2,
                        color: dark
                            ? AppColors.brandPrimaryStrong
                            : AppColors.neutralCream,
                      ),
                    ),
                    const SizedBox(height: HkSpace.sm),
                    Text(
                      '請選擇以下一種方式登入',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontFamily: AppTypography.family,
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                        height: 1.5,
                        color: dark
                            ? AppColors.neutralBody
                            : AppColors.neutralCream.withValues(alpha: 0.88),
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
          const SizedBox(height: HkSpace.lg),
          AuthEntranceItem(
            stage: AuthEntranceStage.google,
            child: button(OAuthProvider.google),
          ),
          const SizedBox(height: HkSpace.md),
          AuthEntranceItem(
            stage: AuthEntranceStage.apple,
            child: button(OAuthProvider.apple),
          ),
          const SizedBox(height: HkSpace.md),
          AuthEntranceItem(
            stage: AuthEntranceStage.line,
            child: button(OAuthProvider.line),
          ),
        ],
      ),
    );
  }

  Widget _buildFooter(Future<String> appVersionFuture) {
    // Footer 用 RepaintBoundary 隔離：父層動畫 60fps 重繪不會帶到這層 →
    // 文字保持像素穩定（解決深色細字 + 動態背景看起來抖動的視覺問題）
    return RepaintBoundary(
      child: AuthFooterCopy(appVersionFuture: appVersionFuture),
    );
  }
}

class AuthFooterCopy extends StatelessWidget {
  const AuthFooterCopy({super.key, required this.appVersionFuture});

  final Future<String> appVersionFuture;

  @override
  Widget build(BuildContext context) {
    final useDarkInk = AmbientBrightnessScope.prefersDarkTextOf(context);
    final foreground = useDarkInk
        ? AppColors.neutralBody
        : AppColors.neutralCream.withValues(alpha: 0.94);
    final muted = useDarkInk
        ? AppColors.brandPrimaryStrong
        : AppColors.neutralCream.withValues(alpha: 0.82);
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '註冊與登入即表示您同意服務條款等相關規範',
          key: const ValueKey('auth-footer-legal'),
          style: TextStyle(
            fontFamily: AppTypography.family,
            fontSize: 14,
            fontWeight: FontWeight.w500,
            height: 1.44,
            color: foreground,
            letterSpacing: 0.5,
          ),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: HkSpace.xs),
        _VersionLabel(future: appVersionFuture, color: muted),
      ],
    );
  }
}

class AuthFooterBrandLogo extends StatelessWidget {
  const AuthFooterBrandLogo({super.key, required this.width});

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

class _VersionLabel extends HookWidget {
  const _VersionLabel({required this.future, required this.color});
  final Future<String> future;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final snap = useFuture(future, initialData: '');
    final label = formatAuthVersion(snap.data);
    return Text(
      label,
      key: const ValueKey('auth-footer-version'),
      style: TextStyle(
        fontFamily: AppTypography.family,
        fontSize: 14,
        fontWeight: FontWeight.w400,
        height: 1.44,
        color: color,
        letterSpacing: 1.0,
      ),
    );
  }
}

@visibleForTesting
String formatAuthVersion(String? version) {
  if (version == null || version.isEmpty) return 'v—';

  final versionWithBuild = RegExp(
    r'^([^()]+)\(([^()]*)\)$',
  ).firstMatch(version);
  if (versionWithBuild != null) {
    final semanticVersion = versionWithBuild.group(1)!;
    final build = versionWithBuild.group(2)!;
    final isSemanticVersion = RegExp(
      r'^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$',
    ).hasMatch(semanticVersion);
    if (isSemanticVersion && build == semanticVersion) {
      return 'v$semanticVersion';
    }
  }

  return 'v$version';
}
