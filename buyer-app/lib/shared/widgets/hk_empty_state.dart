import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_icon.dart';

/// Shared empty-state treatment for full-page, section, and sheet content.
///
/// A 64px semantic icon inside an 88px deep ink halo, with a message and an
/// optional description.
class HkEmptyState extends StatelessWidget {
  const HkEmptyState({
    super.key,
    required this.icon,
    required this.message,
    this.iconKey,
    this.description,
    this.topPadding = topInset,
    this.iconDimension = iconSize,
    this.haloDimension = haloSize,
  });

  static const double iconSize = 64;
  static const double haloSize = 88;
  static const double compactIconSize = 48;
  static const double compactHaloSize = 64;
  static const double topInset = HkSpace.section;
  static const double labelGap = HkSpace.sm;

  final HkIconName icon;
  final String message;
  final Key? iconKey;
  final String? description;
  final double topPadding;
  final double iconDimension;
  final double haloDimension;

  @override
  Widget build(BuildContext context) {
    final dark = Theme.of(context).brightness == Brightness.dark;

    return Align(
      alignment: Alignment.topCenter,
      child: Padding(
        padding: EdgeInsets.only(top: topPadding),
        child: Semantics(
          container: true,
          label: message,
          child: ExcludeSemantics(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox.square(
                  dimension: haloDimension,
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      gradient: RadialGradient(
                        colors: [
                          AppColors.brandPrimaryStrong.withValues(
                            alpha: dark ? 0.24 : 0.12,
                          ),
                          AppColors.brandPrimaryStrong.withValues(
                            alpha: dark ? 0.06 : 0.025,
                          ),
                        ],
                        stops: const [0, 0.68],
                      ),
                    ),
                    child: Center(
                      child: HkIcon(
                        icon,
                        key: iconKey,
                        size: iconDimension,
                        color: AppIconColors.active,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: labelGap),
                Text(
                  message,
                  textAlign: TextAlign.center,
                  style: AppTypography.bodyBold,
                ),
                if (description case final description?) ...[
                  const SizedBox(height: HkSpace.xs),
                  ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 320),
                    child: Text(
                      description,
                      textAlign: TextAlign.center,
                      style: AppTypography.caption,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
