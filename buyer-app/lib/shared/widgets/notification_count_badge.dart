import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 通知數量徽章 — 對應 Figma 設計稿 node 6562:63306。
///
/// 規格：
/// - 紅底 #FF2E31
/// - 最小尺寸 20×20、左右 padding 4
/// - 圓角 10（pill 形狀）
/// - 白色 12sp Bold、letter-spacing 1、line-height 1.5、置中
///
/// `withWhiteBorder=true` 時加 1px 白色外框，用於浮在 icon / 按鈕上需要
/// 跟底色分隔的場景（購物車 / 通知鈴鐺 / 對話 icon 等）。
class NotificationCountBadge extends StatelessWidget {
  const NotificationCountBadge({
    super.key,
    required this.count,
    this.maxCount = 99,
    this.withWhiteBorder = true,
  });

  final int count;
  final int maxCount;
  final bool withWhiteBorder;

  @override
  Widget build(BuildContext context) {
    final label = count > maxCount ? '$maxCount+' : '$count';
    return Container(
      constraints: BoxConstraints(minWidth: 20.w, minHeight: 20.w),
      padding: EdgeInsets.symmetric(horizontal: 4.w),
      decoration: BoxDecoration(
        color: AppColors.danger,
        borderRadius: BorderRadius.circular(12.r),
        border: withWhiteBorder
            ? Border.all(color: AppColors.bgCard, width: 1)
            : null,
      ),
      alignment: Alignment.center,
      child: Text(
        label,
        textAlign: TextAlign.center,
        style: TextStyle(
          color: AppColors.bgCard,
          fontSize: 12,
          fontWeight: FontWeight.bold,
          letterSpacing: 1,
          height: 1.5,
        ),
      ),
    );
  }
}
