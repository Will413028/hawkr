import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

import '../widgets/truck_loading_overlay.dart';

/// 浮動 Toast 通知工具類
///
/// 使用方式：
/// ```dart
/// // 成功提示
/// FloatingToast.show(context, '操作成功');
///
/// // 失敗提示
/// FloatingToast.show(context, '操作失敗', isSuccess: false);
///
/// // 自定義 icon
/// FloatingToast.show(context, '已複製', icon: AppIcons.contentCopy);
/// ```
///
/// 樣式規格：
/// - 位置：距離底部 120px，水平置中
/// - 最大寬度：280px
/// - 內距：水平 32px，垂直 20px
/// - 背景色：白色
/// - 圓角：20px
/// - 陰影：黑色 12% 透明度，模糊 24px，偏移 (0, 8)
/// - Icon 大小：24x24px
/// - Icon 顏色：成功 [AppColors.primary] / 失敗 紅色
/// - 文字大小：16px，w500，#2F2F2F
/// - 動畫：淡入 + 向上滑動 300ms
/// - 顯示時間：2 秒後自動消失
class FloatingToast {
  FloatingToast._();

  static OverlayEntry? _currentEntry;

  /// 顯示浮動卡片式的通知（icon 在上，文字在下）
  ///
  /// [context] - BuildContext
  /// [message] - 顯示的訊息
  /// [icon] - 自定義 icon（可選）
  /// [isSuccess] - 是否為成功狀態，影響 icon 和顏色
  /// [duration] - 顯示時長，預設 2 秒
  static void show(
    BuildContext context,
    String message, {
    IconData? icon,
    bool isSuccess = true,
    Duration duration = const Duration(seconds: 2),
  }) {
    // 移除現有的 toast（避免重疊）
    _currentEntry?.remove();
    _currentEntry = null;

    final overlay = Overlay.of(context);
    late OverlayEntry overlayEntry;

    overlayEntry = OverlayEntry(
      builder: (context) => Positioned(
        bottom: 120,
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
                constraints: const BoxConstraints(maxWidth: 280),
                padding: const EdgeInsets.symmetric(
                  horizontal: 32,
                  vertical: 20,
                ),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.circular(24),
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
                      size: 24,
                      color: isSuccess
                          ? AppIconColors.secondary
                          : AppIconColors.danger,
                    ),
                    const SizedBox(height: 12),
                    // 文字在下方
                    Text(
                      message,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
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
    _currentEntry = overlayEntry;

    Future.delayed(duration, () {
      if (overlayEntry.mounted) {
        overlayEntry.remove();
        if (_currentEntry == overlayEntry) {
          _currentEntry = null;
        }
      }
    });
  }

  /// 成功提示的快捷方法
  static void success(BuildContext context, String message, {IconData? icon}) {
    show(context, message, icon: icon, isSuccess: true);
  }

  /// 失敗提示的快捷方法
  static void error(BuildContext context, String message, {IconData? icon}) {
    show(context, message, icon: icon, isSuccess: false);
  }

  /// 手動關閉當前 toast
  static void dismiss() {
    _currentEntry?.remove();
    _currentEntry = null;
  }

  /// 顯示卡車載入動畫 toast（用於訂單/商品載入）
  ///
  /// 使用 Overlay（非 showDialog），不會與後續 bottom sheet 衝突。
  /// 載入完成後呼叫 [dismiss] 關閉。
  static void showCarLoading(
    BuildContext context,
    String message, {
    Duration autoDismissAfter = const Duration(seconds: 5),
  }) {
    _currentEntry?.remove();
    _currentEntry = null;

    final overlay = Overlay.of(context);
    late OverlayEntry overlayEntry;

    overlayEntry = OverlayEntry(
      builder: (_) => Positioned.fill(
        child: Material(
          color: Colors.transparent,
          child: Center(child: TruckLoadingOverlay(message: message)),
        ),
      ),
    );

    overlay.insert(overlayEntry);
    _currentEntry = overlayEntry;

    Future.delayed(autoDismissAfter, () {
      if (overlayEntry.mounted) {
        overlayEntry.remove();
        if (_currentEntry == overlayEntry) {
          _currentEntry = null;
        }
      }
    });
  }

  /// 顯示帶有 icon 的垂直排列 toast
  ///
  /// [context] - BuildContext
  /// [message] - 顯示的訊息
  /// [icon] - 顯示的圖示
  /// [iconSize] - icon 大小，預設 20
  /// [iconColor] - icon 顏色，預設 #565656
  /// [duration] - 顯示時長，預設 2 秒
  static void showWithIcon(
    BuildContext context,
    String message, {
    required IconData icon,
    double iconSize = 20,
    Color iconColor = AppColors.textPrimary,
    Duration duration = const Duration(seconds: 2),
  }) {
    // 移除現有的 toast（避免重疊）
    _currentEntry?.remove();
    _currentEntry = null;

    final overlay = Overlay.of(context);
    late OverlayEntry overlayEntry;

    overlayEntry = OverlayEntry(
      builder: (context) => Positioned(
        bottom: 120,
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
                constraints: const BoxConstraints(maxWidth: 320),
                padding: const EdgeInsets.symmetric(
                  horizontal: 32,
                  vertical: 20,
                ),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.circular(24),
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
                    // Icon 在上方
                    Icon(icon, size: iconSize, color: iconColor),
                    const SizedBox(height: 12),
                    // 文字在下方
                    Text(
                      message,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w400,
                        color: AppColors.textPrimary,
                        height: 1.43,
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
    _currentEntry = overlayEntry;

    Future.delayed(duration, () {
      if (overlayEntry.mounted) {
        overlayEntry.remove();
        if (_currentEntry == overlayEntry) {
          _currentEntry = null;
        }
      }
    });
  }
}
