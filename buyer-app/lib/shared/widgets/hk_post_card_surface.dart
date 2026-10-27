import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// A single visual and interaction surface for a complete social post.
///
/// The merchant header, media, caption, related product and interaction row
/// belong to this one clipped surface. Callers should not add another card
/// around the lower content area.
class HkPostCardSurface extends StatelessWidget {
  const HkPostCardSurface({
    super.key,
    required this.child,
    this.onTap,
    this.surfaceKey,
  });

  final Widget child;
  final VoidCallback? onTap;
  final Key? surfaceKey;

  @override
  Widget build(BuildContext context) {
    final shape = RoundedRectangleBorder(
      borderRadius: BorderRadius.circular(HkRadius.lg),
      side: const BorderSide(color: AppColors.borderSoft),
    );

    return Material(
      key: surfaceKey,
      color: AppColors.bgElevated,
      shape: shape,
      clipBehavior: Clip.antiAlias,
      child: InkWell(onTap: onTap, customBorder: shape, child: child),
    );
  }
}

/// Opaque lower body shared by immersive post cards.
///
/// Caption, related product and interaction actions belong to this one warm
/// surface. The outer immersive card owns clipping and corner radii; this
/// widget deliberately adds neither blur nor another card outline.
class HkPostCardBodySurface extends StatelessWidget {
  const HkPostCardBodySurface({
    super.key,
    required this.child,
    this.surfaceKey,
  });

  final Widget child;
  final Key? surfaceKey;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      key: surfaceKey,
      color: AppColors.bgElevated,
      child: child,
    );
  }
}
