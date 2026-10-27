import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/widgets/hk_tap_target.dart';

/// 圓角膠囊按鈕 — 訂單卡片內的操作按鈕。
///
/// 規格對齊 buyer App（Figma node 10632:149278）：
/// height 32, fontSize 12, fontWeight w500, horizontal padding 12。
class PillButton extends StatelessWidget {
  const PillButton({
    super.key,
    required this.label,
    required this.color,
    this.onTap,
    this.outlined = false,
  });

  final String label;
  final Color color;
  final VoidCallback? onTap;
  final bool outlined;

  @override
  Widget build(BuildContext context) {
    // 實心底時，淺色底（如 warning 金）需改用深字，否則白字對比不足
    final fillForeground = color.computeLuminance() > 0.4
        ? AppColors.onWarning
        : AppColors.bgCard;
    final textStyle = AppTypography.meta.copyWith(
      fontWeight: FontWeight.w500,
      color: outlined ? color : fillForeground,
    );

    // A Wrap gives each child the whole available width. Bound the tap target
    // to its content so its Center does not turn every action into a full row.
    return IntrinsicWidth(
      child: HkTapTarget(
        semanticLabel: label,
        tooltip: label,
        onTap: onTap,
        child: Material(
          color: outlined ? Colors.transparent : color,
          borderRadius: BorderRadius.circular(999),
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(999),
            child: Container(
              constraints: BoxConstraints(minHeight: 32.h),
              padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
              alignment: Alignment.center,
              decoration: outlined
                  ? BoxDecoration(
                      borderRadius: BorderRadius.circular(999),
                      border: Border.all(color: color, width: 1.5),
                    )
                  : null,
              child: Text(label, style: textStyle),
            ),
          ),
        ),
      ),
    );
  }
}
