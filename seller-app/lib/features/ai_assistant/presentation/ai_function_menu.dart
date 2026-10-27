import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/motion/hk_motion_transitions.dart';

/// 單筆功能選單項目
class AiFunctionMenuItem {
  const AiFunctionMenuItem({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final VoidCallback onTap;
}

/// AI 小幫手功能選單彈出層
///
/// 對應 Figma node `9628:103284`（賣家）/ `9632:67983`（買家）。
/// 點擊輸入框左側的「+」按鈕後出現，顯示一個白色卡片，
/// 內含多筆功能項目，每筆 icon + 文字 + 分隔線。
///
/// 用法：
/// ```dart
/// showAiFunctionMenu(
///   context,
///   anchor: someKey, // 可選，指定彈出位置
///   items: [
///     AiFunctionMenuItem(icon: someIcon, label: '查詢未出貨訂單', onTap: () { ... }),
///     ...
///   ],
/// );
/// ```
Future<void> showAiFunctionMenu({
  required BuildContext context,
  required List<AiFunctionMenuItem> items,
  Offset? position,
}) async {
  await showGeneralDialog<void>(
    context: context,
    barrierDismissible: true,
    barrierLabel: 'AI Function Menu',
    barrierColor: Colors.transparent,
    transitionDuration: HkMotionTransitions.durationOf(context),
    pageBuilder: (ctx, _, __) {
      return _AiFunctionMenuOverlay(items: items, position: position);
    },
    transitionBuilder: (ctx, anim, __, child) {
      return HkMotionTransitions.modal(
        context: ctx,
        animation: anim,
        presentation: HkModalPresentation.popup,
        alignment: position != null ? Alignment.bottomLeft : Alignment.center,
        child: child,
      );
    },
  );
}

class _AiFunctionMenuOverlay extends StatelessWidget {
  const _AiFunctionMenuOverlay({required this.items, this.position});

  final List<AiFunctionMenuItem> items;
  final Offset? position;

  @override
  Widget build(BuildContext context) {
    final menu = _AiFunctionMenuCard(items: items);

    if (position != null) {
      return Stack(
        children: [
          // 透明全螢幕點擊關閉
          Positioned.fill(
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => Navigator.of(context).pop(),
            ),
          ),
          Positioned(
            left: position!.dx,
            top: position!.dy,
            child: Material(color: Colors.transparent, child: menu),
          ),
        ],
      );
    }

    return Stack(
      children: [
        Positioned.fill(
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () => Navigator.of(context).pop(),
          ),
        ),
        Positioned(
          left: 16.w,
          bottom: 96.h,
          child: Material(color: Colors.transparent, child: menu),
        ),
      ],
    );
  }
}

class _AiFunctionMenuCard extends StatelessWidget {
  const _AiFunctionMenuCard({required this.items});

  final List<AiFunctionMenuItem> items;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          for (var i = 0; i < items.length; i++) ...[
            _MenuItemRow(item: items[i]),
            if (i < items.length - 1)
              Padding(
                padding: EdgeInsets.symmetric(vertical: 8.h),
                child: Container(
                  width: 159.w,
                  height: 1,
                  color: AppColors.cardBorder,
                ),
              ),
          ],
        ],
      ),
    );
  }
}

class _MenuItemRow extends StatelessWidget {
  const _MenuItemRow({required this.item});

  final AiFunctionMenuItem item;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: () {
        Navigator.of(context).pop();
        item.onTap();
      },
      borderRadius: BorderRadius.circular(8.r),
      child: Padding(
        padding: EdgeInsets.symmetric(vertical: 4.h),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(item.icon, size: 24.sp, color: AppIconColors.defaultColor),
            SizedBox(width: 9.w),
            Text(
              item.label,
              style: TextStyle(
                fontSize: 14.sp,
                color: AppColors.textPrimary,
                height: 20 / 14,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
