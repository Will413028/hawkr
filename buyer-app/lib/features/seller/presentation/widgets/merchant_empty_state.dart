import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 店舖頁各分頁共用的無資料狀態，保持畫布乾淨、不額外包卡片。
class MerchantEmptyState extends StatelessWidget {
  const MerchantEmptyState({
    super.key,
    required this.label,
    this.icon,
    this.semanticIcon,
    this.size = 64,
  }) : assert(icon != null || semanticIcon != null);

  final HkIconName? icon;
  final HkIconName? semanticIcon;
  final double size;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (semanticIcon != null)
          HkIcon(semanticIcon!, size: size, color: AppIconColors.muted)
        else
          HkIcon(icon!, key: const Key('merchant-empty-icon'), size: size),
        const SizedBox(height: HkSpace.sm),
        Text(
          label,
          style: AppTypography.bodyBold.copyWith(
            color: AppColors.textSecondary,
          ),
        ),
      ],
    );
  }
}
