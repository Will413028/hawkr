import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'hk_tap_target.dart';

/// 共用 X 關閉按鈕 — 預設 32x32 白色圓形 + close icon 20sp #565656。
///
/// 實際點擊區至少為 48dp；[size] 只控制圓形的視覺大小。
/// [visualAlignment] 可在點擊區變大時保留原本的視覺錨點。
///
/// 用法：
/// ```dart
/// Stack(
///   clipBehavior: Clip.none,
///   children: [
///     Container(...), // dialog body
///     Positioned(
///       top: -8.h, right: -8.w,
///       child: DialogCloseButton(
///         visualAlignment: Alignment.topRight,
///       ),
///     ),
///   ],
/// )
/// ```
class DialogCloseButton extends StatelessWidget {
  const DialogCloseButton({
    super.key,
    this.onTap,
    this.size = 32,
    this.iconSize = 20,
    this.semanticLabel = '關閉',
    this.visualAlignment = Alignment.center,
  });

  /// 點擊回調，預設 Navigator.pop(context)
  final VoidCallback? onTap;

  /// 按鈕大小（預設 32）
  final double size;

  /// icon 大小（預設 20）
  final double iconSize;

  /// 輔助技術與 tooltip 顯示的動作名稱。
  final String semanticLabel;

  /// 圓形視覺在至少 48dp 點擊區內的對齊方式。
  ///
  /// 預設置中。若既有版位以右上角定位圓形，請使用 [Alignment.topRight]；
  /// 若要保留既有右側邊界，請使用 [Alignment.centerRight]。
  final AlignmentGeometry visualAlignment;

  @override
  Widget build(BuildContext context) {
    final visualExtent = size.w;
    final targetExtent = visualExtent < 48 ? 48.0 : visualExtent;

    return HkTapTarget(
      semanticLabel: semanticLabel,
      hint: '關閉目前視窗',
      tooltip: semanticLabel,
      onTap: onTap ?? () => Navigator.pop(context),
      child: SizedBox.square(
        dimension: targetExtent,
        child: Align(
          alignment: visualAlignment,
          child: Container(
            width: visualExtent,
            height: visualExtent,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: AppColors.bgCard,
              boxShadow: [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 4,
                  offset: Offset(0, 2.h),
                ),
              ],
            ),
            child: Icon(
              AppIcons.close,
              size: iconSize.sp,
              color: AppIconColors.defaultColor,
            ),
          ),
        ),
      ),
    );
  }
}
