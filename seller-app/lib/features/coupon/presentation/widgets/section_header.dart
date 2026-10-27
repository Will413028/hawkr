import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 優惠券建立流程的區塊標題。
///
/// 固定的步驟編號與圖示讓長表單仍可快速掃讀，說明文字則提供
/// 欄位目的，不以 placeholder 取代 label。
class CouponSectionHeader extends StatelessWidget {
  const CouponSectionHeader({
    super.key,
    required this.step,
    required this.icon,
    required this.title,
    required this.description,
  });

  final String step;
  final HkIconName icon;
  final String title;
  final String description;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 44,
          height: 44,
          decoration: BoxDecoration(
            color: AppColors.primarySoft,
            borderRadius: BorderRadius.circular(HkRadius.md),
            border: Border.all(color: AppColors.borderSoft),
          ),
          child: Center(
            child: HkIcon(icon, size: 22, color: AppIconColors.active),
          ),
        ),
        const SizedBox(width: HkSpace.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '步驟 $step',
                style: const TextStyle(
                  color: AppColors.brandPrimaryStrong,
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 0.4,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                title,
                style: const TextStyle(
                  color: AppColors.textEmphasis,
                  fontSize: 18,
                  height: 1.3,
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: HkSpace.xs),
              Text(
                description,
                style: const TextStyle(
                  color: AppColors.textMuted,
                  fontSize: 14,
                  height: 1.45,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
