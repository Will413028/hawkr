import 'dart:ui' as ui;

import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// 可捲動內容使用的克制型霧面玻璃卡片。
///
/// 模糊區域先裁切並以 [RepaintBoundary] 隔離，避免整頁重繪。
class HkFrostedCardSurface extends StatelessWidget {
  const HkFrostedCardSurface({
    super.key,
    required this.child,
    this.radius = HkRadius.xl,
    this.shadows,
    this.blurSigma = HkGlass.navBarBlurSigma,
  });

  final Widget child;
  final double radius;
  final List<BoxShadow>? shadows;
  final double blurSigma;

  @override
  Widget build(BuildContext context) {
    final borderRadius = BorderRadius.circular(radius);
    return RepaintBoundary(
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: borderRadius,
          boxShadow: shadows ?? HkShadows.card,
        ),
        child: ClipRRect(
          borderRadius: borderRadius,
          child: BackdropFilter(
            filter: ui.ImageFilter.blur(sigmaX: blurSigma, sigmaY: blurSigma),
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    AppColors.neutralWhite.withValues(alpha: 0.90),
                    AppColors.neutralCream.withValues(alpha: 0.76),
                  ],
                ),
                borderRadius: borderRadius,
                border: Border.all(
                  color: AppColors.neutralWhite.withValues(alpha: 0.94),
                ),
              ),
              child: child,
            ),
          ),
        ),
      ),
    );
  }
}
