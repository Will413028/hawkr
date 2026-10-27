import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../icons/app_icons.dart';

/// Where a [HkPlaceholder] stands in for a photo, so the glyph and tint match
/// what is missing.
enum HkPlaceholderKind {
  avatar(
    icon: AppIcons.personOutline,
    background: AppColors.brandPrimarySoft,
    foreground: AppColors.brandPrimary,
  ),
  storeCover(
    icon: AppIcons.storefrontOutlined,
    background: AppColors.brandPrimaryStrong,
    foreground: AppColors.brandPrimarySoft,
  ),
  photo(
    icon: AppIcons.imageOutlined,
    background: AppColors.brandPrimarySoft,
    foreground: AppColors.brandPrimary,
  ),
  hero(
    icon: AppIcons.nightlightOutlined,
    background: AppColors.brandSecondarySoft,
    foreground: AppColors.brandSecondary,
  );

  const HkPlaceholderKind({
    required this.icon,
    required this.background,
    required this.foreground,
  });

  final IconData icon;
  final Color background;
  final Color foreground;
}

/// Flat tint with one Material icon that fills the box its parent gives it.
///
/// The caller owns the shape: wrap in `ClipOval` or `ClipRRect` for rounded
/// frames. It is decorative and never read out by screen readers.
class HkPlaceholder extends StatelessWidget {
  const HkPlaceholder(this.kind, {super.key});

  static const double iconScale = 0.4;

  final HkPlaceholderKind kind;

  @override
  Widget build(BuildContext context) {
    return ExcludeSemantics(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final shortestSide = math.min(
            constraints.maxWidth,
            constraints.maxHeight,
          );
          return ColoredBox(
            color: kind.background,
            child: Center(
              child: Icon(
                kind.icon,
                color: kind.foreground,
                size: shortestSide.isFinite ? shortestSide * iconScale : null,
              ),
            ),
          );
        },
      ),
    );
  }
}
