import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// The single visual surface used by seller storefront post detail pages.
///
/// Media, author metadata, copy, linked products, comments, and actions all
/// remain inside this boundary so the post reads and scrolls as one card.
class HkPostCardSurface extends StatelessWidget {
  const HkPostCardSurface({super.key, required this.child, this.surfaceKey});

  final Widget child;
  final Key? surfaceKey;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      key: surfaceKey,
      decoration: BoxDecoration(
        color: AppColors.bgElevated,
        borderRadius: BorderRadius.circular(HkRadius.xl),
        border: Border.all(color: AppColors.borderSubtle),
        boxShadow: HkShadows.card,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(HkRadius.xl),
        child: child,
      ),
    );
  }
}

/// Opaque content layer paired with [HkPostCardSurface].
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
