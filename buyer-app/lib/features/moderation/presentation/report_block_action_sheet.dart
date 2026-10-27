import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../data/report_target_type.dart';
import 'block_confirm_dialog.dart';
import 'report_sheet.dart';

/// 「⋯」共用觸發選單（bottom sheet + ListTile pattern，對齊聊天室既有的
/// `showBuyerMessageContextMenu` 風格）。點「檢舉」開 [showReportSheet]，
/// 點「封鎖此使用者」開 [showBlockConfirmDialog]。
///
/// [blockUserId] 為 null 或空字串時不顯示封鎖選項（例如呼叫端尚未取得對象
/// userId）。
Future<void> showReportBlockActionSheet(
  BuildContext context, {
  required ReportTargetType reportTargetType,
  required String reportTargetId,
  String? blockUserId,
  String? blockUserLabel,
}) {
  final hasBlockTarget = blockUserId != null && blockUserId.isNotEmpty;
  return showModalBottomSheet(
    context: context,
    showDragHandle: false,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (ctx) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(height: 8.h),
          Container(
            width: 36.w,
            height: 4.h,
            decoration: BoxDecoration(
              color: AppColors.gray300,
              borderRadius: BorderRadius.circular(2.r),
            ),
          ),
          SizedBox(height: 12.h),
          ListTile(
            leading: Icon(
              AppIcons.reportOutlined,
              color: AppIconColors.warning,
              size: 22.sp,
            ),
            title: Text('檢舉', style: TextStyle(fontSize: 16)),
            onTap: () {
              Navigator.pop(ctx);
              showReportSheet(
                context,
                targetType: reportTargetType,
                targetId: reportTargetId,
              );
            },
          ),
          if (hasBlockTarget)
            ListTile(
              leading: Icon(
                AppIcons.blockOutlined,
                color: AppIconColors.danger,
                size: 22.sp,
              ),
              title: Text('封鎖此使用者', style: TextStyle(fontSize: 16)),
              onTap: () {
                Navigator.pop(ctx);
                showBlockConfirmDialog(
                  context,
                  userId: blockUserId,
                  userLabel: blockUserLabel ?? '此使用者',
                );
              },
            ),
          SizedBox(height: 8.h),
        ],
      ),
    ),
  );
}
