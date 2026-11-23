import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../data/block_target.dart';
import '../data/report_target.dart';
import 'block_confirm_dialog.dart';
import 'report_sheet.dart';

enum _SheetAction { delete, report, block }

/// 「⋯」共用觸發選單（bottom sheet + ListTile pattern，對齊聊天室既有的
/// `showBuyerMessageContextMenu` 風格）。點「檢舉」開 [showReportSheet]，
/// 點「封鎖此使用者」開 [showBlockConfirmDialog]。
///
/// [blockTarget] 為 null 時不顯示封鎖選項（例如自己的內容、尚未取得對象）。
/// 回傳 true 表示這次已成功封鎖，呼叫端據此重讀受影響的社群狀態。
///
/// 給了 [onDelete]（觀看者能刪這則別人的內容，例如店主刪討論區留言）時選單第一項是「刪除」，
/// 選了之後呼叫它；刪除不算封鎖，回傳 false。
Future<bool> showReportBlockActionSheet(
  BuildContext context, {
  required SocialReportTarget reportTarget,
  SocialBlockTarget? blockTarget,
  String? blockUserLabel,
  String blockActionLabel = '封鎖此使用者',
  VoidCallback? onDelete,
}) async {
  final action = await showModalBottomSheet<_SheetAction>(
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
          if (onDelete != null)
            ListTile(
              leading: Icon(
                AppIcons.deleteOutline,
                color: AppIconColors.danger,
                size: 22.sp,
              ),
              title: Text('刪除', style: TextStyle(fontSize: 16)),
              onTap: () => Navigator.pop(ctx, _SheetAction.delete),
            ),
          ListTile(
            leading: Icon(
              AppIcons.reportOutlined,
              color: AppIconColors.warning,
              size: 22.sp,
            ),
            title: Text('檢舉', style: TextStyle(fontSize: 16)),
            onTap: () {
              Navigator.pop(ctx, _SheetAction.report);
            },
          ),
          if (blockTarget != null)
            ListTile(
              leading: Icon(
                AppIcons.blockOutlined,
                color: AppIconColors.danger,
                size: 22.sp,
              ),
              title: Text(blockActionLabel, style: TextStyle(fontSize: 16)),
              onTap: () => Navigator.pop(ctx, _SheetAction.block),
            ),
          SizedBox(height: 8.h),
        ],
      ),
    ),
  );
  if (action == null || !context.mounted) return false;
  switch (action) {
    case _SheetAction.delete:
      onDelete?.call();
      return false;
    case _SheetAction.report:
      await showReportSheet(context, target: reportTarget);
      return false;
    case _SheetAction.block:
      final blocked = await showBlockConfirmDialog(
        context,
        target: blockTarget!,
        userLabel: blockUserLabel ?? '此使用者',
      );
      return blocked ?? false;
  }
}
