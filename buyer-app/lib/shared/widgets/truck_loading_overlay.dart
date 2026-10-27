import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hover_icon.dart';

/// 卡車動畫 Loading 彈窗
class TruckLoadingOverlay extends StatelessWidget {
  const TruckLoadingOverlay({super.key, this.message});

  final String? message;

  static void show(BuildContext context, {String? message}) {
    showDialog(
      context: context,
      barrierDismissible: false,
      barrierColor: AppColors.borderSubtle,
      builder: (_) => TruckLoadingOverlay(message: message),
    );
  }

  static void hide(BuildContext context) {
    Navigator.of(context, rootNavigator: true).pop();
  }

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
                  blurRadius: 24,
                  spreadRadius: 2,
                  offset: const Offset(0, 4),
                ),
              ],
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                HoverIcon(
                  icon: AppIcons.localShippingOutlined,
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
                    fontSize: 14,
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
