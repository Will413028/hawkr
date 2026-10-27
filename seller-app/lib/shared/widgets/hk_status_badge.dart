import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_icon.dart';

enum HkStatusTone { neutral, success, warning, danger, info }

/// 狀態不能只靠顏色表達，因此 badge 永遠保留文字，可選擇語意 icon。
class HkStatusBadge extends StatelessWidget {
  const HkStatusBadge({
    super.key,
    required this.label,
    this.tone = HkStatusTone.neutral,
    this.icon,
  });

  final String label;
  final HkStatusTone tone;
  final HkIconName? icon;

  @override
  Widget build(BuildContext context) {
    final colors = switch (tone) {
      HkStatusTone.success => (
        foreground: AppColors.success,
        background: AppColors.successSoft,
        icon: AppIconColors.success,
      ),
      HkStatusTone.warning => (
        foreground: AppColors.warningText,
        background: AppColors.warningSoft,
        icon: AppIconColors.warning,
      ),
      HkStatusTone.danger => (
        foreground: AppColors.danger,
        background: AppColors.dangerSoft,
        icon: AppIconColors.danger,
      ),
      HkStatusTone.info => (
        foreground: AppColors.info,
        background: AppColors.infoSoft,
        icon: AppIconColors.info,
      ),
      HkStatusTone.neutral => (
        foreground: AppColors.textPrimary,
        background: AppColors.surfaceMuted,
        icon: AppIconColors.defaultColor,
      ),
    };

    return Semantics(
      label: '狀態：$label',
      child: ExcludeSemantics(
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: colors.background,
            borderRadius: BorderRadius.circular(HkRadius.pill),
            border: Border.all(
              color: colors.foreground.withValues(alpha: 0.22),
            ),
          ),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: HkSpace.md,
              vertical: HkSpace.xs,
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (icon case final icon?) ...[
                  HkIcon(icon, size: 16, color: colors.icon),
                  const SizedBox(width: HkSpace.xs),
                ],
                Flexible(
                  child: Text(
                    label,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: colors.foreground,
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
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
