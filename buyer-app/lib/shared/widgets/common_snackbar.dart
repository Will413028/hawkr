import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// ============================================================
/// 新版：浮動卡片式 Toast 通知（推薦使用）
/// ============================================================

/// 顯示浮動卡片式的通知（icon 在上，文字在下）
///
/// 用法：
/// ```dart
/// showFloatingToast(context, '操作成功');
/// showFloatingToast(context, '操作失敗', isSuccess: false);
/// showFloatingToast(context, '已複製', icon: AppIcons.contentCopy);
/// ```
void showFloatingToast(
  BuildContext context,
  String message, {
  IconData? icon,
  bool isSuccess = true,
  Duration duration = const Duration(seconds: 2),
}) {
  final overlay = Overlay.of(context);
  late OverlayEntry overlayEntry;

  overlayEntry = OverlayEntry(
    builder: (context) => Positioned(
      bottom: 120.h,
      left: 0,
      right: 0,
      child: Center(
        child: Material(
          color: Colors.transparent,
          child: TweenAnimationBuilder<double>(
            tween: Tween(begin: 0.0, end: 1.0),
            duration: const Duration(milliseconds: 300),
            curve: Curves.easeOut,
            builder: (context, value, child) {
              return Opacity(
                opacity: value,
                child: Transform.translate(
                  offset: Offset(0, 20 * (1 - value)),
                  child: child,
                ),
              );
            },
            child: Container(
              constraints: BoxConstraints(maxWidth: 280.w),
              padding: EdgeInsets.symmetric(horizontal: 32.w, vertical: 20.h),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(24.r),
                boxShadow: [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    blurRadius: 24,
                    offset: const Offset(0, 8),
                  ),
                ],
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Icon 在上方 (24x24)
                  Icon(
                    icon ??
                        (isSuccess ? AppIcons.check : AppIcons.errorOutline),
                    size: 24.sp,
                    color: isSuccess
                        ? AppIconColors.secondary
                        : AppIconColors.danger,
                  ),
                  SizedBox(height: 12.h),
                  // 文字在下方
                  Text(
                    message,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w500,
                      color: AppColors.textEmphasis,
                      height: 1.4,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    ),
  );

  overlay.insert(overlayEntry);

  Future.delayed(duration, () {
    if (overlayEntry.mounted) {
      overlayEntry.remove();
    }
  });
}

/// ============================================================
/// 舊版：SnackBar（保留但不建議使用）
/// ============================================================

/// 顯示統一樣式的 SnackBar（舊版，建議改用 showFloatingToast）
///
/// 用法：
/// ```dart
/// showCommonSnackBar(context, '操作成功');
/// ```
void showCommonSnackBar(
  BuildContext context,
  String message, {
  Duration duration = const Duration(seconds: 2),
  SnackBarAction? action,
}) {
  // [舊版 SnackBar - 已棄用，改用 showFloatingToast]
  // ScaffoldMessenger.of(context).showSnackBar(
  //   SnackBar(
  //     content: Text(message),
  //     backgroundColor: kSnackBarBackground,
  //     duration: duration,
  //     action: action,
  //   ),
  // );

  // 新版：直接呼叫浮動 Toast
  showFloatingToast(context, message, duration: duration);
}
