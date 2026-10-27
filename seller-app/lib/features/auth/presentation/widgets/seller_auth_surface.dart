import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/animated_orbs_background.dart';
import 'package:hawkr_auth/oauth.dart';
import 'auth_action_glass_panel.dart';
import 'auth_entrance_sequence.dart';
import 'auth_moon_hero.dart';
import 'auth_oauth_pill_button.dart';

typedef AuthWordmarkBuilder = Widget Function(double width);

/// Seller login presentation kept in parity with the buyer auth surface.
///
/// Authentication, routing and the hidden review login stay owned by
/// [AuthScreen]; this widget owns only responsive layout and interaction.
class SellerAuthSurface extends StatelessWidget {
  const SellerAuthSurface({
    super.key,
    required this.loadingProvider,
    required this.onSignIn,
    required this.appVersionFuture,
    required this.wordmarkBuilder,
  });

  /// 進行中的登入方式；其他按鈕在這時停用。
  final OAuthProvider? loadingProvider;
  final ValueChanged<OAuthProvider> onSignIn;
  final Future<String> appVersionFuture;
  final AuthWordmarkBuilder wordmarkBuilder;

  @override
  Widget build(BuildContext context) {
    final media = MediaQuery.of(context);
    final keyboardVisible = media.viewInsets.bottom > 0;
    final screenHeight = media.size.height;
    final largeTextLayout = media.textScaler.scale(16) / 16 >= 1.1;

    return Scaffold(
      resizeToAvoidBottomInset: true,
      backgroundColor: AppColors.bgPrimary,
      body: AnimatedOrbsBackground(
        child: AuthEntranceSequence(
          child: SafeArea(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final horizontalPadding = largeTextLayout
                    ? HkSpace.lg
                    : HkBreakpoints.pageGutter(constraints.maxWidth);
                const verticalPadding = HkSpace.lg;
                return SingleChildScrollView(
                  key: const ValueKey('seller-auth-scroll-view'),
                  padding: EdgeInsets.symmetric(
                    horizontal: horizontalPadding,
                    vertical: verticalPadding,
                  ),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(
                      minHeight: (constraints.maxHeight - verticalPadding * 2)
                          .clamp(0, double.infinity),
                    ),
                    child: _buildHeroLoginLayout(
                      constraints: constraints,
                      contentWidth:
                          constraints.maxWidth - horizontalPadding * 2,
                      screenHeight: screenHeight,
                      keyboardVisible: keyboardVisible,
                      largeTextLayout: largeTextLayout,
                    ),
                  ),
                );
              },
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
  }) {
    final maxWidth = contentWidth.clamp(280.0, 420.0);
    final availableHeight = constraints.maxHeight.isFinite
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
                  child: KeyedSubtree(
                    key: const ValueKey('seller-auth-wordmark'),
                    child: wordmarkBuilder(logoWidth),
                  ),
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
            child: SellerAuthFooterCopy(appVersionFuture: appVersionFuture),
          ),
        ),
      ],
    );
  }

  Widget _buildHeroCard({
    required double cardWidth,
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
            child: Padding(
              padding: EdgeInsets.symmetric(
                horizontal: largeTextLayout ? HkSpace.sm : HkSpace.lg,
                vertical: largeTextLayout ? HkSpace.xl : HkSpace.xxl,
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  AuthEntranceItem(
                    stage: AuthEntranceStage.headline,
                    child: _buildHeadline(useDarkInk),
                  ),
                  const SizedBox(height: HkSpace.lg),
                  AuthEntranceItem(
                    stage: AuthEntranceStage.google,
                    child: _button(OAuthProvider.google),
                  ),
                  const SizedBox(height: HkSpace.md),
                  AuthEntranceItem(
                    stage: AuthEntranceStage.apple,
                    child: _button(OAuthProvider.apple),
                  ),
                  const SizedBox(height: HkSpace.md),
                  AuthEntranceItem(
                    stage: AuthEntranceStage.line,
                    child: _button(OAuthProvider.line),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _button(OAuthProvider provider) => AuthOAuthPillButton(
    provider: provider,
    text: '透過${provider.label}繼續',
    isLoading: loadingProvider == provider,
    isDisabled: loadingProvider != null && loadingProvider != provider,
    onTap: () => onSignIn(provider),
  );

  Widget _buildHeadline(bool useDarkInk) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '歡迎來到 Hawkr',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 24,
            height: 1.2,
            color: useDarkInk
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
            color: useDarkInk
                ? AppColors.neutralBody
                : AppColors.neutralCream.withValues(alpha: 0.88),
          ),
        ),
      ],
    );
  }
}

class SellerAuthFooterCopy extends StatelessWidget {
  const SellerAuthFooterCopy({super.key, required this.appVersionFuture});

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

    return RepaintBoundary(
      child: Column(
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
      ),
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
    return Text(
      formatSellerAuthVersion(snap.data),
      key: const ValueKey('auth-footer-version'),
      style: TextStyle(
        fontFamily: AppTypography.family,
        fontSize: 14,
        fontWeight: FontWeight.w400,
        height: 1.44,
        color: color,
        letterSpacing: 1,
      ),
    );
  }
}

@visibleForTesting
String formatSellerAuthVersion(String? version) {
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
