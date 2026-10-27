import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../data/post_publish.dart';

/// 賣家自己頁面被平台下架貼文的縮圖角標。實心紅底白字（沿用商家認證
/// rejected 的 #E05050 主色），縮圖上比 pale pill 更清楚。
class HiddenPostBadge extends StatelessWidget {
  const HiddenPostBadge({super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: AppColors.danger,
        borderRadius: BorderRadius.circular(6.r),
      ),
      child: Text(
        kPostPlatformHiddenLabel,
        style: TextStyle(
          fontSize: 12.sp,
          fontWeight: FontWeight.w700,
          color: AppColors.bgCard,
        ),
      ),
    );
  }
}
