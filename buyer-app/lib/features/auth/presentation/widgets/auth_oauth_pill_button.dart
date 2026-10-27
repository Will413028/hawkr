import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icons.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// Full-width, high-contrast OAuth action for the moonlit auth panel.
class AuthOAuthPillButton extends StatefulWidget {
  const AuthOAuthPillButton({
    super.key,
    required this.provider,
    required this.text,
    required this.isLoading,
    required this.isDisabled,
    required this.onTap,
    this.scale = 1,
    this.adaptiveLayout = false,
  });

  final OAuthProvider provider;
  final String text;
  final bool isLoading;
  final bool isDisabled;
  final VoidCallback onTap;

  /// Retained for source compatibility with the previous auth layout.
  final double scale;

  /// Retained for source compatibility; width is now governed by its parent.
  final bool adaptiveLayout;

  @override
  State<AuthOAuthPillButton> createState() => _AuthOAuthPillButtonState();
}

class _AuthOAuthPillButtonState extends State<AuthOAuthPillButton>
    with TickerProviderStateMixin {
  static const _hoverDuration = Duration(milliseconds: 160);
  static const _pressDuration = Duration(milliseconds: 80);
  static const _releaseDuration = Duration(milliseconds: 220);
  static const _effectDuration = Duration(milliseconds: 420);

  late final AnimationController _pressController = AnimationController(
    vsync: this,
    duration: _releaseDuration,
  );
  late final AnimationController _effectController = AnimationController(
    vsync: this,
    duration: _effectDuration,
  );

  Offset? _activationPosition;
  bool _hovered = false;
  bool _focused = false;
  bool _reduceMotion = false;

  bool get _interactionLocked => widget.isLoading || widget.isDisabled;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final reduceMotion = MediaQuery.disableAnimationsOf(context);
    if (_reduceMotion == reduceMotion) return;
    _reduceMotion = reduceMotion;
    if (reduceMotion) {
      _pressController
        ..stop(canceled: false)
        ..value = 0;
      _effectController
        ..stop(canceled: false)
        ..value = 0;
    }
  }

  @override
  void didUpdateWidget(covariant AuthOAuthPillButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_interactionLocked && !_oldInteractionLocked(oldWidget)) {
      _hovered = false;
      _pressController
        ..stop(canceled: false)
        ..value = 0;
    }
  }

  bool _oldInteractionLocked(AuthOAuthPillButton oldWidget) =>
      oldWidget.isLoading || oldWidget.isDisabled;

  @override
  void dispose() {
    _pressController.dispose();
    _effectController.dispose();
    super.dispose();
  }

  void _handleHover(bool hovered) {
    if (_interactionLocked || _hovered == hovered) return;
    setState(() => _hovered = hovered);
  }

  void _handleFocus(bool focused) {
    if (_focused == focused) return;
    setState(() => _focused = focused);
  }

  void _handleTapDown(TapDownDetails details) {
    if (_interactionLocked) return;
    _activationPosition = details.localPosition;
    if (_reduceMotion) return;
    _pressController.animateTo(
      1,
      duration: _pressDuration,
      curve: Curves.easeOut,
    );
  }

  void _releasePress() {
    if (_reduceMotion) {
      _pressController.value = 0;
      return;
    }
    _pressController.animateBack(
      0,
      duration: _releaseDuration,
      curve: HkMotion.ease,
    );
  }

  void _activate() {
    if (_interactionLocked) return;
    _releasePress();
    if (!_reduceMotion) _effectController.forward(from: 0);
    HapticFeedback.selectionClick();
    widget.onTap();
  }

  @override
  Widget build(BuildContext context) {
    final providerName = widget.provider.name;
    final textScale = MediaQuery.textScalerOf(context).scale(16) / 16;
    final baseHeight = textScale >= 1.3
        ? 64.0
        : textScale >= 1.1
        ? 60.0
        : 56.0;
    final height = math.max(HkControl.minTap, baseHeight * widget.scale);
    final (background, foreground, focusColor) = switch (widget.provider) {
      OAuthProvider.google => (
        AppColors.neutralCream,
        AppColors.brandPrimaryStrong,
        AppColors.brandPrimaryStrong,
      ),
      OAuthProvider.apple => (
        AppColors.neutralInk,
        AppColors.neutralCream,
        AppColors.brandSecondary,
      ),
      OAuthProvider.line => (
        AppColors.lineBrand,
        AppColors.neutralWhite,
        AppColors.brandSecondary,
      ),
    };
    final lockedOpacity = widget.isDisabled ? 0.48 : 1.0;
    final hoverEnabled = _hovered && !_interactionLocked;

    final buttonVisual = SizedBox(
      width: double.infinity,
      height: height,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(HkRadius.pill),
        child: Stack(
          fit: StackFit.expand,
          children: [
            AnimatedContainer(
              key: ValueKey('auth-oauth-$providerName-surface'),
              duration: _reduceMotion ? Duration.zero : _hoverDuration,
              curve: HkMotion.ease,
              decoration: BoxDecoration(
                color: background,
                borderRadius: BorderRadius.circular(HkRadius.pill),
                border: Border.all(
                  color: hoverEnabled
                      ? focusColor.withValues(alpha: 0.46)
                      : foreground.withValues(alpha: 0.12),
                ),
                boxShadow: hoverEnabled
                    ? [
                        BoxShadow(
                          color: AppColors.brandPrimaryStrong.withValues(
                            alpha: 0.18,
                          ),
                          blurRadius: 18,
                          offset: const Offset(0, 8),
                        ),
                      ]
                    : null,
              ),
            ),
            _ButtonContents(
              providerName: providerName,
              provider: widget.provider,
              isLoading: widget.isLoading,
              text: widget.text,
              fontSize: 16,
              foreground: foreground,
              reduceMotion: _reduceMotion,
            ),
            Positioned.fill(
              child: IgnorePointer(
                child: AnimatedBuilder(
                  animation: _effectController,
                  builder: (context, _) => CustomPaint(
                    key: ValueKey('auth-oauth-$providerName-click-effect'),
                    painter: _AuthActionEffectPainter(
                      progress: _effectController.value,
                      origin: _activationPosition,
                      color: foreground,
                    ),
                  ),
                ),
              ),
            ),
            if (_focused && !_interactionLocked)
              Positioned.fill(
                child: IgnorePointer(
                  child: DecoratedBox(
                    key: ValueKey('auth-oauth-$providerName-focus-ring'),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(HkRadius.pill),
                      border: Border.all(color: focusColor, width: 2),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );

    final animatedButton = AnimatedSlide(
      key: ValueKey('auth-oauth-$providerName-hover-motion'),
      offset: hoverEnabled ? Offset(0, -2 / height) : Offset.zero,
      duration: _reduceMotion ? Duration.zero : _hoverDuration,
      curve: HkMotion.ease,
      child: AnimatedScale(
        scale: hoverEnabled ? 1.01 : 1,
        duration: _reduceMotion ? Duration.zero : _hoverDuration,
        curve: HkMotion.ease,
        child: AnimatedBuilder(
          animation: _pressController,
          child: buttonVisual,
          builder: (context, staticChild) => Transform.translate(
            key: ValueKey('auth-oauth-$providerName-press-translate'),
            offset: Offset(0, _pressController.value),
            child: Transform.scale(
              key: ValueKey('auth-oauth-$providerName-press-scale'),
              scale: 1 - 0.025 * _pressController.value,
              child: staticChild,
            ),
          ),
        ),
      ),
    );

    final gestures = GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTapDown: _interactionLocked ? null : _handleTapDown,
      onTapUp: _interactionLocked ? null : (_) => _releasePress(),
      onTapCancel: _interactionLocked ? null : _releasePress,
      onTap: _interactionLocked ? null : _activate,
      child: animatedButton,
    );

    return Semantics(
      key: ValueKey('auth-oauth-$providerName-button'),
      container: true,
      button: true,
      enabled: !_interactionLocked,
      liveRegion: widget.isLoading,
      label: widget.isLoading ? '${widget.text}，處理中' : widget.text,
      onTap: _interactionLocked ? null : _activate,
      child: ExcludeSemantics(
        child: Opacity(
          opacity: lockedOpacity,
          child: MouseRegion(
            cursor: _interactionLocked
                ? SystemMouseCursors.basic
                : SystemMouseCursors.click,
            onEnter: (_) => _handleHover(true),
            onExit: (_) => _handleHover(false),
            child: FocusableActionDetector(
              enabled: !_interactionLocked,
              onShowHoverHighlight: _handleHover,
              onShowFocusHighlight: _handleFocus,
              shortcuts: const {
                SingleActivator(LogicalKeyboardKey.enter): ActivateIntent(),
                SingleActivator(LogicalKeyboardKey.space): ActivateIntent(),
              },
              actions: {
                ActivateIntent: CallbackAction<ActivateIntent>(
                  onInvoke: (_) {
                    _activationPosition = null;
                    _activate();
                    return null;
                  },
                ),
              },
              child: gestures,
            ),
          ),
        ),
      ),
    );
  }
}

class _ButtonContents extends StatelessWidget {
  const _ButtonContents({
    required this.providerName,
    required this.provider,
    required this.isLoading,
    required this.text,
    required this.fontSize,
    required this.foreground,
    required this.reduceMotion,
  });

  final String providerName;
  final OAuthProvider provider;
  final bool isLoading;
  final String text;
  final double fontSize;
  final Color foreground;
  final bool reduceMotion;

  @override
  Widget build(BuildContext context) {
    final providerIcon = switch (provider) {
      OAuthProvider.google => Icon(
        AppIcons.login,
        size: HkControl.icon,
        color: foreground,
      ),
      OAuthProvider.apple => Icon(
        AppIcons.login,
        size: HkControl.icon,
        color: foreground,
      ),
      OAuthProvider.line => Icon(
        AppIcons.chatBubbleRounded,
        size: HkControl.icon,
        color: foreground,
      ),
    };
    final icon = SizedBox.square(
      key: ValueKey('auth-oauth-$providerName-icon'),
      dimension: HkControl.icon,
      child: AnimatedSwitcher(
        duration: reduceMotion
            ? Duration.zero
            : const Duration(milliseconds: 160),
        child: isLoading
            ? SizedBox.square(
                key: const ValueKey('auth-oauth-loading-indicator'),
                dimension: 18,
                child: HkBrandLoader(
                  tone: HkBrandLoaderTone.mono,
                  color: foreground,
                ),
              )
            : KeyedSubtree(
                key: ValueKey('auth-oauth-$providerName-provider-icon'),
                child: providerIcon,
              ),
      ),
    );
    final label = HkButtonLabel(
      text,
      key: ValueKey('auth-oauth-$providerName-label'),
      style: TextStyle(
        fontFamily: AppTypography.family,
        fontSize: fontSize,
        height: 1.25,
        fontWeight: FontWeight.w700,
        color: foreground,
      ),
    );
    final textScale =
        MediaQuery.textScalerOf(context).scale(fontSize) / fontSize;
    final useLargeTextLayout = textScale >= 1.25;

    return Padding(
      padding: useLargeTextLayout
          ? const EdgeInsetsDirectional.only(start: HkSpace.md, end: HkSpace.xs)
          : const EdgeInsets.symmetric(horizontal: HkSpace.md),
      child: useLargeTextLayout
          ? Row(
              children: [
                icon,
                const SizedBox(width: HkSpace.xs),
                Expanded(child: label),
              ],
            )
          : Stack(
              alignment: Alignment.center,
              children: [
                Align(alignment: Alignment.centerLeft, child: icon),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: HkSpace.xxl),
                  child: label,
                ),
              ],
            ),
    );
  }
}

class _AuthActionEffectPainter extends CustomPainter {
  const _AuthActionEffectPainter({
    required this.progress,
    required this.origin,
    required this.color,
  });

  final double progress;
  final Offset? origin;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    if (progress <= 0 || progress >= 1) return;
    final eased = Curves.easeOutCubic.transform(progress);
    final center = origin ?? size.center(Offset.zero);
    final maxRadius = math.sqrt(
      size.width * size.width + size.height * size.height,
    );
    canvas.drawCircle(
      center,
      maxRadius * eased,
      Paint()..color = color.withValues(alpha: (1 - progress) * 0.14),
    );

    final sweepWidth = math.max(48.0, size.width * 0.22);
    final left = -sweepWidth + (size.width + sweepWidth * 2) * eased;
    final rect = Rect.fromLTWH(left, 0, sweepWidth, size.height);
    final sweepPaint = Paint()
      ..shader = LinearGradient(
        colors: [
          Colors.transparent,
          color.withValues(alpha: math.sin(progress * math.pi) * 0.20),
          Colors.transparent,
        ],
      ).createShader(rect);
    canvas.drawRect(rect, sweepPaint);
  }

  @override
  bool shouldRepaint(_AuthActionEffectPainter oldDelegate) =>
      oldDelegate.progress != progress ||
      oldDelegate.origin != origin ||
      oldDelegate.color != color;
}
