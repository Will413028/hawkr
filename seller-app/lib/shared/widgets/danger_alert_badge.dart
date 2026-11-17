import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../core/theme/app_colors.dart';
import '../icons/app_icon_colors.dart';
import '../icons/app_icons.dart';

/// 停權、受限頁頂端的危險圖示徽章：淡紅圓底加警示圖示。
class DangerAlertBadge extends StatelessWidget {
  const DangerAlertBadge({super.key});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        width: 72.w,
        height: 72.w,
        decoration: BoxDecoration(
          color: AppColors.dangerStrong.withValues(alpha: 0.12),
          shape: BoxShape.circle,
        ),
        child: Icon(
          AppIcons.warningAmberRounded,
          color: AppIconColors.warning,
          size: 40.sp,
        ),
      ),
    );
  }
}
