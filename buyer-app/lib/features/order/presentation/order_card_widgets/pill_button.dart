import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_button_label.dart';

/// 圓角膠囊按鈕 — 訂單卡片內的操作按鈕（細節/再次付款/確認收貨等）
/// The visual pill stays compact while its semantic hit area is always 48dp.
class PillButton extends StatelessWidget {
  const PillButton({
    super.key,
    required this.label,
    required this.color,
    this.onTap,
  });
  final String label;
  final Color color;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final labelStyle = color.computeLuminance() > 0.4
        ? AppTypography.buttonSmall.copyWith(color: AppColors.onWarning)
        : AppTypography.buttonSmall;

    return Semantics(
      button: true,
      enabled: onTap != null,
      label: label,
      excludeSemantics: true,
      child: SizedBox(
        height: HkControl.minTap,
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(HkRadius.pill),
            child: Center(
              widthFactor: 1,
              child: Container(
                constraints: const BoxConstraints(
                  minHeight: HkControl.compactChoiceHeight,
                  maxHeight: 32,
                ),
                padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
                decoration: BoxDecoration(
                  color: color,
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                ),
                alignment: Alignment.center,
                child: HkButtonLabel(label, style: labelStyle),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
