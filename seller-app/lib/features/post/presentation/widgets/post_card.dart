import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// Anchored action menu that stays on screen and dismisses with its route.
Future<void> showPostActionMenu(
  BuildContext context, {
  VoidCallback? onEdit,
  VoidCallback? onDelete,
  String editLabel = '編輯貼文',
  String deleteLabel = '刪除貼文',
}) async {
  final button = context.findRenderObject()! as RenderBox;
  final overlay =
      Navigator.of(context).overlay!.context.findRenderObject()! as RenderBox;
  final selected = await showMenu<String>(
    context: context,
    position: RelativeRect.fromRect(
      Rect.fromPoints(
        button.localToGlobal(Offset.zero, ancestor: overlay),
        button.localToGlobal(
          button.size.bottomRight(Offset.zero),
          ancestor: overlay,
        ),
      ),
      Offset.zero & overlay.size,
    ),
    color: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.circular(12.r),
      side: BorderSide(color: AppColors.textPrimary.withValues(alpha: 0.2)),
    ),
    items: [
      PopupMenuItem<String>(
        value: 'edit',
        enabled: onEdit != null,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              AppIcons.editOutlined,
              size: 24.sp,
              color: AppIconColors.defaultColor,
            ),
            SizedBox(width: 6.w),
            Flexible(
              child: Text(editLabel, style: TextStyle(fontSize: 16.sp)),
            ),
          ],
        ),
      ),
      const PopupMenuDivider(),
      PopupMenuItem<String>(
        value: 'delete',
        enabled: onDelete != null,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              AppIcons.deleteOutline,
              size: 24.sp,
              color: AppIconColors.defaultColor,
            ),
            SizedBox(width: 6.w),
            Flexible(
              child: Text(deleteLabel, style: TextStyle(fontSize: 16.sp)),
            ),
          ],
        ),
      ),
    ],
  );
  if (!context.mounted) return;
  if (selected == 'edit') onEdit?.call();
  if (selected == 'delete') onDelete?.call();
}
