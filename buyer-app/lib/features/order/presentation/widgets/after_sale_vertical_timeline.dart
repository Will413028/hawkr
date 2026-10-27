import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import '../../../../shared/widgets/hk_icon.dart';

/// 垂直 timeline 的單一步驟資料。
class AfterSaleTimelineStep {
  const AfterSaleTimelineStep({
    required this.label,
    this.timestamp,
    this.done = false,
    this.current = false,
    this.icon,
    this.iconColor,
  });

  /// 步驟名稱（例：「司機到府收件」）
  final String label;

  /// 時間戳記（例：「11/08 14:30」或「等待中」）。null 則不顯示。
  final String? timestamp;

  /// 是否已完成（綠色點）
  final bool done;

  /// 是否為當前進行中的步驟（橘色點，會有 glow）
  final bool current;

  /// 步驟左側 icon（optional，與圓點並列）
  final HkIconName? icon;

  /// icon 顏色（預設依 done/current 自動推導）
  final Color? iconColor;
}

/// 售後流程垂直 timeline（物流步驟、退款進度、申訴時間線共用）。
///
/// 使用範例：
/// ```dart
/// AfterSaleVerticalTimeline(
///   title: '退貨流程',
///   steps: [
///     AfterSaleTimelineStep(label: '賣家預約出車', timestamp: '11/06 14:30', done: true),
///     AfterSaleTimelineStep(label: '黑貓派車', timestamp: '11/07 09:00', current: true),
///     AfterSaleTimelineStep(label: '到府收件', timestamp: '預計 11/08'),
///   ],
/// )
/// ```
class AfterSaleVerticalTimeline extends StatelessWidget {
  const AfterSaleVerticalTimeline({
    super.key,
    required this.steps,
    this.title,
    this.titleIcon,
    this.doneColor = AppColors.primary,
    this.currentColor = AppColors.warning,
    this.backgroundColor,
    this.padding,
  });

  final List<AfterSaleTimelineStep> steps;

  /// 區塊標題（optional，會顯示在頂部）
  final String? title;

  /// 區塊標題的 icon（optional）
  final HkIconName? titleIcon;

  final Color doneColor;
  final Color currentColor;
  final Color? backgroundColor;
  final EdgeInsetsGeometry? padding;

  /// 當前步驟文字／icon 用的深金色（currentColor #E0C44C 太亮，
  /// 當白底 13sp 小字對比不足，故文字/icon 改用此深金色，dot+glow 仍用 currentColor）
  static const Color _currentTextColor = AppColors.warningText;

  @override
  Widget build(BuildContext context) {
    const mutedColor = AppColors.cardBorder;

    final content = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (title != null) ...[
          Row(
            children: [
              if (titleIcon != null) ...[
                HkIcon(titleIcon!, size: 16.sp, color: _currentTextColor),
                SizedBox(width: 6.w),
              ],
              Text(
                title!,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                  color: _currentTextColor,
                ),
              ),
            ],
          ),
          SizedBox(height: 10.h),
        ],
        for (int i = 0; i < steps.length; i++)
          _buildStep(
            step: steps[i],
            isLast: i == steps.length - 1,
            mutedColor: mutedColor,
          ),
      ],
    );

    if (backgroundColor == null) {
      return Padding(padding: padding ?? EdgeInsets.zero, child: content);
    }

    return Container(
      padding: padding ?? EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: content,
    );
  }

  Widget _buildStep({
    required AfterSaleTimelineStep step,
    required bool isLast,
    required Color mutedColor,
  }) {
    final dotColor = step.current
        ? currentColor
        : step.done
        ? doneColor
        : mutedColor;

    final textColor = step.current
        ? _currentTextColor
        : step.done
        ? AppColors.textPrimary
        : const Color(0x7342493F);

    final timeColor = step.current
        ? _currentTextColor
        : step.done
        ? doneColor
        : const Color(0x5942493F);

    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Dot + vertical line column ──
          SizedBox(
            width: 20.w,
            child: Column(
              children: [
                SizedBox(height: 4.h),
                Container(
                  width: 10.w,
                  height: 10.w,
                  decoration: BoxDecoration(
                    color: dotColor,
                    shape: BoxShape.circle,
                    boxShadow: step.current
                        ? [
                            BoxShadow(
                              color: dotColor.withValues(alpha: 0.25),
                              blurRadius: 6,
                              spreadRadius: 2,
                            ),
                          ]
                        : null,
                  ),
                ),
                if (!isLast)
                  Expanded(
                    child: Container(
                      width: 2.w,
                      margin: EdgeInsets.symmetric(vertical: 4.h),
                      color: step.done ? doneColor : mutedColor,
                    ),
                  ),
              ],
            ),
          ),
          SizedBox(width: 10.w),
          // ── Label + timestamp column ──
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : 12.h),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      if (step.icon != null) ...[
                        HkIcon(
                          step.icon!,
                          size: 14.sp,
                          color: step.iconColor ?? textColor,
                        ),
                        SizedBox(width: 4.w),
                      ],
                      Expanded(
                        child: Text(
                          step.label,
                          style: TextStyle(
                            fontSize: 14,
                            color: textColor,
                            fontWeight: step.current
                                ? FontWeight.w700
                                : FontWeight.w500,
                          ),
                        ),
                      ),
                    ],
                  ),
                  if (step.timestamp != null) ...[
                    SizedBox(height: 2.h),
                    Text(
                      step.timestamp!,
                      style: TextStyle(fontSize: 12, color: timeColor),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
