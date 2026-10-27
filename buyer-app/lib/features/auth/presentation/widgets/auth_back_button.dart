import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/icons/app_icons.dart';
import '../../../../shared/widgets/hk_tap_target.dart';

/// Accessible auth back control with pointer, focus and press feedback.
class AuthBackButton extends StatefulWidget {
  const AuthBackButton({super.key, required this.onTap});

  final VoidCallback onTap;

  @override
  State<AuthBackButton> createState() => _AuthBackButtonState();
}

class _AuthBackButtonState extends State<AuthBackButton>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pressController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 180),
  );
  bool _hovered = false;
  bool _focused = false;
  bool _reduceMotion = false;

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
    }
  }

  @override
  void dispose() {
    _pressController.dispose();
    super.dispose();
  }

  void _setHovered(bool value) {
    if (_hovered == value) return;
    setState(() => _hovered = value);
  }

  void _setFocused(bool value) {
    if (_focused == value) return;
    setState(() => _focused = value);
  }

  void _press() {
    if (_reduceMotion) return;
    _pressController.animateTo(
      1,
      duration: const Duration(milliseconds: 70),
      curve: Curves.easeOut,
    );
  }

  void _release() {
    if (_reduceMotion) {
      _pressController.value = 0;
      return;
    }
    _pressController.animateBack(0, curve: HkMotion.ease);
  }

  @override
  Widget build(BuildContext context) {
    final duration = _reduceMotion
        ? Duration.zero
        : const Duration(milliseconds: 160);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => _setHovered(true),
      onExit: (_) => _setHovered(false),
      child: FocusableActionDetector(
        onShowHoverHighlight: _setHovered,
        onShowFocusHighlight: _setFocused,
        shortcuts: const {
          SingleActivator(LogicalKeyboardKey.enter): ActivateIntent(),
          SingleActivator(LogicalKeyboardKey.space): ActivateIntent(),
        },
        actions: {
          ActivateIntent: CallbackAction<ActivateIntent>(
            onInvoke: (_) {
              widget.onTap();
              return null;
            },
          ),
        },
        child: Listener(
          behavior: HitTestBehavior.opaque,
          onPointerDown: (_) => _press(),
          onPointerUp: (_) => _release(),
          onPointerCancel: (_) => _release(),
          child: AnimatedScale(
            key: const ValueKey('auth-back-hover-motion'),
            scale: _hovered && !_reduceMotion ? 1.05 : 1,
            duration: duration,
            curve: HkMotion.ease,
            child: AnimatedBuilder(
              animation: _pressController,
              child: SizedBox.square(
                dimension: HkControl.minTap,
                child: HkTapTarget(
                  key: const ValueKey('auth-back-button'),
                  semanticLabel: '返回繼續瀏覽',
                  onTap: widget.onTap,
                  child: SizedBox.square(
                    key: const ValueKey('auth-back-visual'),
                    dimension: 36,
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        AnimatedContainer(
                          duration: duration,
                          decoration: BoxDecoration(
                            color: _hovered
                                ? AppColors.neutralCream.withValues(alpha: 0.94)
                                : AppColors.bgCard.withValues(alpha: 0.76),
                            shape: BoxShape.circle,
                            boxShadow: _hovered
                                ? [
                                    BoxShadow(
                                      color: AppColors.brandPrimaryStrong
                                          .withValues(alpha: 0.18),
                                      blurRadius: 14,
                                      offset: const Offset(0, 6),
                                    ),
                                  ]
                                : null,
                          ),
                        ),
                        const Center(
                          child: Icon(
                            AppIcons.arrowBack,
                            size: 20,
                            color: AppIconColors.defaultColor,
                          ),
                        ),
                        if (_focused)
                          DecoratedBox(
                            key: const ValueKey('auth-back-focus-ring'),
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              border: Border.all(
                                color: AppColors.brandSecondary,
                                width: 2,
                              ),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ),
              builder: (context, staticChild) => Transform.scale(
                key: const ValueKey('auth-back-press-scale'),
                scale: 1 - 0.06 * _pressController.value,
                child: staticChild,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
