import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/hk_content_card.dart';
import 'package:flutter/material.dart';

/// Coupon feature 統一內容卡片容器。
///
/// 長表單使用接近不透明的 cream surface；霧面玻璃只保留給票券預覽與
/// 小型控制，避免每張卡片都做 blur。
class CouponSectionCard extends StatelessWidget {
  const CouponSectionCard({super.key, required this.child, this.padding});

  final Widget child;

  /// 卡片內距，預設 16 logical pixels。
  final EdgeInsetsGeometry? padding;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      child: HkContentCard(
        padding: padding ?? const EdgeInsets.all(HkSpace.lg),
        radius: HkRadius.xl,
        elevated: true,
        backgroundColor: AppColors.surfaceCard,
        borderColor: AppColors.cardBorder,
        child: child,
      ),
    );
  }
}
