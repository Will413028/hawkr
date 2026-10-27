import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_icon.dart';
import 'hover_icon.dart';

/// 卡車動畫 Loading 彈窗
///
/// 用法：
/// ```dart
/// // 顯示
/// TruckLoadingOverlay.show(context);
///
/// // 隱藏
/// TruckLoadingOverlay.hide(context);
///
/// // 包裝非同步操作
/// await TruckLoadingOverlay.wrap(context, () async {
///   await fetchData();
/// });
/// ```
class TruckLoadingOverlay extends StatelessWidget {
  const TruckLoadingOverlay({super.key, this.message});

  final String? message;

  /// 顯示 loading overlay
  static void show(BuildContext context, {String? message}) {
    showDialog(
      context: context,
      barrierDismissible: false,
      barrierColor: AppColors.borderSubtle,
      builder: (_) => TruckLoadingOverlay(message: message),
    );
  }

  /// 隱藏 loading overlay
  static void hide(BuildContext context) {
    Navigator.of(context, rootNavigator: true).pop();
  }

  /// 包裝非同步操作：自動 show → 執行 → hide
  static Future<T> wrap<T>(
    BuildContext context,
    Future<T> Function() action, {
    String? message,
  }) async {
    show(context, message: message);
    try {
      return await action();
    } finally {
      if (context.mounted) hide(context);
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      child: Center(
        child: Material(
          color: Colors.transparent,
          child: Container(
            width: 140.w,
            padding: EdgeInsets.symmetric(vertical: 24.h, horizontal: 16.w),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(16.r),
              boxShadow: [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 24.r,
                  spreadRadius: 2.r,
                  offset: Offset(0, 4.h),
                ),
              ],
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                HoverIcon(
                  icon: HkIconName.truck,
                  animation: HoverAnimation.slideThrough,
                  size: 48.sp,
                  color: AppColors.primary,
                  autoPlay: true,
                  repeatInterval: const Duration(milliseconds: 600),
                  duration: const Duration(milliseconds: 900),
                ),
                SizedBox(height: 14.h),
                Text(
                  message ?? 'Loading',
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textSecondary,
                    fontWeight: FontWeight.w400,
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
