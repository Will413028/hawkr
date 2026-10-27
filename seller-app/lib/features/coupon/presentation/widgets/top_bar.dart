import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_chrome_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 優惠券建立、編輯流程共用的玻璃導覽列。
class CouponTopBar extends StatelessWidget {
  const CouponTopBar({super.key, required this.title, required this.onBack});

  final String title;
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    return HkGlassChromeSurface(
      child: SafeArea(
        bottom: false,
        child: SizedBox(
          height: 60,
          child: Stack(
            alignment: Alignment.center,
            children: [
              Align(
                alignment: Alignment.centerLeft,
                child: Padding(
                  padding: const EdgeInsets.only(left: 4),
                  child: IconButton(
                    onPressed: onBack,
                    tooltip: '返回',
                    icon: const HkIcon(HkIconName.back),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 68),
                child: Text(
                  title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ),
              const Align(
                alignment: Alignment.centerRight,
                child: SizedBox(width: 52, height: 48),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
