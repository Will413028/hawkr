import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import '../../main.dart';
import '../utils/hk_log.dart';

/// 浮動 Toast 通知工具
/// 用於顯示應用內的通知訊息
class FloatingToast {
  static OverlayEntry? _currentEntry;

  /// 顯示通知 Toast
  static void show({
    required String title,
    String? message,
    IconData icon = AppIcons.notificationsOutlined,
    Color iconColor = AppColors.primary,
    Duration duration = const Duration(seconds: 3),
    VoidCallback? onTap,
  }) {
    // 移除現有的 toast
    dismiss();

    final navigatorState = MyApp.navigatorKey.currentState;
    if (navigatorState == null) {
      HKLog.logW('[FloatingToast] navigatorKey.currentState is null');
      return;
    }

    final overlay = navigatorState.overlay;
    if (overlay == null) {
      HKLog.logW('[FloatingToast] overlay is null');
      return;
    }

    _currentEntry = OverlayEntry(
      builder: (context) => _FloatingToastWidget(
        title: title,
        message: message,
        icon: icon,
        iconColor: iconColor,
        onTap: onTap,
        onDismiss: dismiss,
      ),
    );

    overlay.insert(_currentEntry!);

    // 自動移除
    Future.delayed(duration, dismiss);
  }

  /// 顯示通知類型的 Toast
  static void showNotification({
    required String title,
    String? body,
    VoidCallback? onTap,
  }) {
    show(
      title: title,
      message: body,
      icon: AppIcons.notificationsActiveOutlined,
      iconColor: AppColors.brandPrimaryStrong,
      onTap: onTap,
    );
  }

  /// 顯示成功 Toast
  static void showSuccess(String message) {
    show(
      title: message,
      icon: AppIcons.checkCircleOutline,
      iconColor: AppColors.success,
      duration: const Duration(seconds: 2),
    );
  }

  /// 顯示錯誤 Toast
  static void showError(String message) {
    show(
      title: message,
      icon: AppIcons.errorOutline,
      iconColor: AppColors.danger,
      duration: const Duration(seconds: 3),
    );
  }

  /// 關閉當前 Toast
  static void dismiss() {
    if (_currentEntry?.mounted == true) {
      _currentEntry?.remove();
    }
    _currentEntry = null;
  }
}

class _FloatingToastWidget extends StatefulWidget {
  final String title;
  final String? message;
  final IconData icon;
  final Color iconColor;
  final VoidCallback? onTap;
  final VoidCallback onDismiss;

  const _FloatingToastWidget({
    required this.title,
    this.message,
    required this.icon,
    required this.iconColor,
    this.onTap,
    required this.onDismiss,
  });

  @override
  State<_FloatingToastWidget> createState() => _FloatingToastWidgetState();
}

class _FloatingToastWidgetState extends State<_FloatingToastWidget>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _fadeAnimation;
  late Animation<Offset> _slideAnimation;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      duration: const Duration(milliseconds: 300),
      vsync: this,
    );
    _fadeAnimation = Tween<double>(
      begin: 0.0,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOut));
    _slideAnimation = Tween<Offset>(
      begin: const Offset(0, -0.5),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOut));
    _controller.forward();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Positioned(
      top: MediaQuery.of(context).padding.top + 16.h,
      left: 16.w,
      right: 16.w,
      child: SlideTransition(
        position: _slideAnimation,
        child: FadeTransition(
          opacity: _fadeAnimation,
          child: GestureDetector(
            onTap: () {
              widget.onTap?.call();
              widget.onDismiss();
            },
            onHorizontalDragEnd: (details) {
              if (details.primaryVelocity != null &&
                  details.primaryVelocity!.abs() > 100) {
                widget.onDismiss();
              }
            },
            child: Material(
              color: Colors.transparent,
              child: Container(
                padding: EdgeInsets.all(16.w),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.circular(12.r),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 16,
                      offset: const Offset(0, 4),
                    ),
                  ],
                ),
                child: Row(
                  children: [
                    Container(
                      padding: EdgeInsets.all(8.w),
                      decoration: BoxDecoration(
                        color: widget.iconColor.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                      child: Icon(
                        widget.icon,
                        color: widget.iconColor,
                        size: 24.sp,
                      ),
                    ),
                    SizedBox(width: 12.w),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            widget.title,
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textEmphasis,
                            ),
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                          ),
                          if (widget.message != null) ...[
                            SizedBox(height: 4.h),
                            Text(
                              widget.message!,
                              style: TextStyle(
                                fontSize: 12,
                                color: AppColors.textSecondary,
                              ),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                        ],
                      ),
                    ),
                    SizedBox(width: 8.w),
                    Icon(
                      AppIcons.close,
                      size: 18.sp,
                      color: AppIconColors.muted,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
