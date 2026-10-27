import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// 賣家主頁共用的單一連續品牌畫布。
///
/// 漸層固定在頁面最底層，讓內容、浮動導覽與底部安全區共享同一背景；
/// 卡片與導覽只負責提供霧面景深，不應各自重新開始另一段漸層。
class SellerGradientCanvas extends StatelessWidget {
  const SellerGradientCanvas({
    super.key,
    required this.child,
    this.sproutOpacity = 0.10,
    this.goldOpacity = 0.07,
  });

  final Widget child;
  final double sproutOpacity;
  final double goldOpacity;

  static BoxDecoration decoration({
    double sproutOpacity = 0.10,
    double goldOpacity = 0.07,
  }) {
    return BoxDecoration(
      color: AppColors.bgPrimary,
      gradient: LinearGradient(
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
        colors: [
          AppColors.brandPrimaryStrong.withValues(alpha: sproutOpacity),
          AppColors.bgPrimary,
          AppColors.brandSecondary.withValues(alpha: goldOpacity),
        ],
        stops: const [0, 0.54, 1],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      key: const ValueKey('seller-continuous-gradient-canvas'),
      decoration: decoration(
        sproutOpacity: sproutOpacity,
        goldOpacity: goldOpacity,
      ),
      child: child,
    );
  }
}
