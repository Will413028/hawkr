import 'dart:ui';

import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/hk_tokens.dart';

/// The auth screen's only backdrop-filtered action surface.
class AuthActionGlassPanel extends StatelessWidget {
  const AuthActionGlassPanel({
    super.key,
    required this.useDarkInk,
    required this.child,
  });

  final bool useDarkInk;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final surfaceGradient = useDarkInk
        ? const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xE6FFFDF7), Color(0xD9F5F1E8)],
          )
        : const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xE61A314D), Color(0xD914253A)],
          );
    final borderColor = useDarkInk
        ? AppColors.brandPrimaryStrong.withValues(alpha: 0.18)
        : AppColors.neutralCream.withValues(alpha: 0.22);
    final highlight = useDarkInk
        ? AppColors.neutralWhite.withValues(alpha: 0.78)
        : AppColors.neutralCream.withValues(alpha: 0.74);

    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(HkRadius.hero),
        boxShadow: [
          BoxShadow(
            color: AppColors.brandPrimaryStrong.withValues(alpha: 0.14),
            blurRadius: 28,
            offset: const Offset(0, 12),
          ),
        ],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(HkRadius.hero),
        child: BackdropFilter(
          filter: ImageFilter.blur(sigmaX: 20, sigmaY: 20),
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: surfaceGradient,
              borderRadius: BorderRadius.circular(HkRadius.hero),
              border: Border.all(color: borderColor),
            ),
            child: Stack(
              children: [
                child,
                Positioned(
                  top: 0,
                  left: HkSpace.section,
                  right: HkSpace.section,
                  child: IgnorePointer(
                    child: SizedBox(
                      height: 1,
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          gradient: LinearGradient(
                            colors: [
                              Colors.transparent,
                              highlight,
                              Colors.transparent,
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
