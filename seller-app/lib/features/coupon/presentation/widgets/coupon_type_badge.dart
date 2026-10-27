import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/codemeta/code_meta.dart';
import '../../../../core/codemeta/code_meta_display.dart';

/// 优惠券类型徽章组件（通用）
///
/// 支持兩種类型：
/// - CASH：现金折抵（绿色背景 + 金钱图标）
/// - DISCOUNT/PERCENT：折數折抵（灰色背景 + 百分比图标）
class CouponTypeBadge extends StatelessWidget {
  /// 优惠券类型（CASH | DISCOUNT | PERCENT）
  final String type;

  /// 徽章显示的文字
  final String label;

  /// 徽章尺寸（默认 48x58）
  final double? width;
  final double? height;

  const CouponTypeBadge({
    super.key,
    required this.type,
    required this.label,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    final config = _getTypeConfig(type);

    return Container(
      width: width ?? 48.w,
      height: height ?? 58.h,
      padding: EdgeInsets.only(top: 4.h),
      decoration: BoxDecoration(
        color: config.color,
        borderRadius: BorderRadius.only(
          bottomLeft: Radius.circular(999.r),
          bottomRight: Radius.circular(999.r),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2.r,
            offset: Offset(0, 1.h),
          ),
        ],
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Icon(config.icon, size: 24.sp, color: AppIconColors.onBrand),
          const Spacer(),
          Padding(
            padding: EdgeInsets.only(bottom: 4.h),
            child: Text(
              label,
              style: TextStyle(
                fontFamily: 'Roboto',
                fontSize: 14.sp,
                fontWeight: FontWeight.w700,
                height: 1.54,
                color: AppColors.bgCard,
              ),
              textAlign: TextAlign.center,
            ),
          ),
        ],
      ),
    );
  }

  /// 根据类型获取配置
  _TypeConfig _getTypeConfig(String type) {
    return switch (CodeMetaDisplay.couponTypeMeta(type)) {
      CouponType.percent => _TypeConfig(
        color: AppColors.textPrimary,
        icon: AppIcons.percent,
      ),
      CouponType.cash || null => _TypeConfig(
        color: AppColors.brandPrimaryStrong,
        icon: AppIcons.attachMoney,
      ),
    };
  }
}

/// 类型配置
class _TypeConfig {
  final Color color;
  final IconData icon;

  _TypeConfig({required this.color, required this.icon});
}
