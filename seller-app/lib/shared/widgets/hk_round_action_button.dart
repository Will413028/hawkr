import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:flutter/material.dart';

/// Canonical circular plus/minus action shared across Hawkr surfaces.
///
/// The glyph slot and visual circle are fixed at 18/32 logical pixels;
/// [HkTapTarget] keeps the invisible interaction area at least 48 logical
/// pixels. Feature code therefore cannot introduce near-duplicate sizes or a
/// visible square frame.
enum HkRoundActionSymbol { minus, plus }

class HkRoundActionButton extends StatelessWidget {
  const HkRoundActionButton({
    super.key,
    required this.semanticLabel,
    required this.symbol,
    required this.onPressed,
    this.semanticHint,
    this.tooltip,
    this.visualKey,
    this.iconKey,
  });

  static const double visualSize = 32;
  static const double iconSize = 18;
  static const double glyphLength = 16;

  final String semanticLabel;
  final HkRoundActionSymbol symbol;
  final VoidCallback? onPressed;
  final String? semanticHint;
  final String? tooltip;
  final Key? visualKey;
  final Key? iconKey;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null;

    return SizedBox.square(
      dimension: HkControl.minTap,
      child: HkTapTarget(
        semanticLabel: semanticLabel,
        hint: semanticHint,
        tooltip: tooltip,
        onTap: onPressed,
        child: DecoratedBox(
          key: visualKey,
          decoration: BoxDecoration(
            color: enabled ? AppColors.primary : AppColors.gray300,
            shape: BoxShape.circle,
          ),
          child: SizedBox.square(
            dimension: visualSize,
            child: Center(
              child: SizedBox.square(
                key: iconKey,
                dimension: iconSize,
                child: Center(
                  child: HkIcon(
                    symbol == HkRoundActionSymbol.plus
                        ? HkIconName.add
                        : HkIconName.remove,
                    size: glyphLength,
                    color: AppIconColors.onBrand,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
