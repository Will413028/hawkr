import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_liquid_glass.dart';

/// iOS 26 functional-layer glass for compact navigation chrome.
///
/// Keep this surface clipped and small. It is not intended for list bodies,
/// media grids or large form backgrounds.
class HkGlassChromeSurface extends StatelessWidget {
  const HkGlassChromeSurface({
    super.key,
    required this.child,
    this.height,
    this.padding,
    this.backgroundColor = AppColors.topBarGlass,
    this.surfaceOpacity = HkGlass.topBarSurfaceOpacity,
    this.borderRadius = BorderRadius.zero,
    this.showBottomBorder = true,
    this.surfaceKey,
  });

  final Widget child;
  final double? height;
  final EdgeInsetsGeometry? padding;
  final Color backgroundColor;
  final double surfaceOpacity;
  final BorderRadius borderRadius;
  final bool showBottomBorder;
  final Key? surfaceKey;

  @override
  Widget build(BuildContext context) {
    final highContrast = MediaQuery.maybeHighContrastOf(context) ?? false;

    return LayoutBuilder(
      builder: (context, constraints) => HkLiquidGlassSurface(
        variant: HkLiquidGlassVariant.regular,
        backgroundColor: backgroundColor,
        surfaceOpacity: surfaceOpacity,
        blurSigma: HkGlass.topBarBlurSigma,
        borderRadius: borderRadius,
        border: showBottomBorder
            ? Border(
                bottom: BorderSide(
                  color: highContrast
                      ? AppColors.borderStrong
                      : AppColors.borderSubtle,
                ),
              )
            : null,
        showBorder: showBottomBorder,
        shadows: HkShadows.xs,
        padding: padding,
        // Full-width bars still fill a bounded parent. Compact chrome inside
        // Positioned/Row must instead size itself to its child; forcing
        // infinity there poisons layout and then Semantics.
        width: constraints.hasBoundedWidth ? double.infinity : null,
        height: height,
        surfaceKey: surfaceKey,
        child: child,
      ),
    );
  }
}
