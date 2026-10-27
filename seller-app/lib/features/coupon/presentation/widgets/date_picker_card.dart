import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

import 'section_card.dart';
import 'section_header.dart';

/// 優惠券有效期限：常用天數、無期限或指定日期。
///
/// 月曆只在使用者選擇「指定日期」後展開，避免初始畫面被低頻控制佔滿。
class CouponDatePickerCard extends StatelessWidget {
  const CouponDatePickerCard({
    super.key,
    required this.noExpiry,
    required this.selectedDays,
    required this.useCustomDate,
    required this.validUntil,
    required this.hasExpiry,
    required this.calendarMonth,
    required this.onNoExpiryChanged,
    required this.onCustomDateChanged,
    required this.onDaysSelected,
    required this.onDateSelected,
    required this.onCalendarMonthChanged,
  });

  final bool noExpiry;
  final int? selectedDays;
  final bool useCustomDate;
  final DateTime validUntil;
  final bool hasExpiry;
  final DateTime calendarMonth;
  final ValueChanged<bool> onNoExpiryChanged;
  final ValueChanged<bool> onCustomDateChanged;
  final ValueChanged<int> onDaysSelected;
  final ValueChanged<DateTime> onDateSelected;
  final ValueChanged<DateTime> onCalendarMonthChanged;

  @override
  Widget build(BuildContext context) {
    final summary = noExpiry
        ? '這張優惠券沒有使用期限'
        : hasExpiry
        ? '顧客可使用至 ${_formatDate(validUntil)}'
        : '請選擇優惠券的最後使用日';

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const CouponSectionHeader(
            step: '2',
            icon: HkIconName.calendar,
            title: '設定使用期限',
            description: '常用期限一鍵完成，也可以指定日期或設為長期有效。',
          ),
          const SizedBox(height: HkSpace.xxl),
          const _FieldTitle(title: '常用期限', description: '從今天起計算'),
          const SizedBox(height: HkSpace.sm),
          LayoutBuilder(
            builder: (context, constraints) {
              const spacing = HkSpace.sm;
              final columns = constraints.maxWidth < 300 ? 2 : 4;
              final itemWidth =
                  (constraints.maxWidth - (columns - 1) * spacing) / columns;
              return Wrap(
                spacing: spacing,
                runSpacing: spacing,
                children: [7, 15, 30, 60]
                    .map(
                      (days) => SizedBox(
                        width: itemWidth,
                        child: _DayButton(
                          days: days,
                          isSelected: selectedDays == days,
                          onTap: () => onDaysSelected(days),
                        ),
                      ),
                    )
                    .toList(),
              );
            },
          ),
          const SizedBox(height: HkSpace.xl),
          const _FieldTitle(title: '其他期限', description: '無期限與指定日期只能擇一'),
          const SizedBox(height: HkSpace.sm),
          LayoutBuilder(
            builder: (context, constraints) {
              final stack = constraints.maxWidth < 340;
              final noExpiryOption = _ExpiryModeButton(
                icon: HkIconName.history,
                label: '無使用期限',
                description: '適合長期會員福利',
                isSelected: noExpiry,
                onTap: () => onNoExpiryChanged(!noExpiry),
              );
              final customOption = _ExpiryModeButton(
                icon: HkIconName.calendar,
                label: '指定日期',
                description: '選擇確切截止日',
                isSelected: useCustomDate,
                onTap: () => onCustomDateChanged(!useCustomDate),
              );
              if (stack) {
                return Column(
                  children: [
                    noExpiryOption,
                    const SizedBox(height: HkSpace.sm),
                    customOption,
                  ],
                );
              }
              return Row(
                children: [
                  Expanded(child: noExpiryOption),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(child: customOption),
                ],
              );
            },
          ),
          AnimatedSwitcher(
            duration: HkMotion.duration,
            switchInCurve: HkMotion.ease,
            switchOutCurve: HkMotion.ease,
            child: useCustomDate && !noExpiry
                ? Padding(
                    key: const ValueKey('calendar'),
                    padding: const EdgeInsets.only(top: HkSpace.xl),
                    child: _CalendarWidget(
                      calendarMonth: calendarMonth,
                      validUntil: validUntil,
                      hasExpiry: hasExpiry,
                      onDateSelected: onDateSelected,
                      onMonthChanged: onCalendarMonthChanged,
                    ),
                  )
                : const SizedBox.shrink(key: ValueKey('calendar-hidden')),
          ),
          const SizedBox(height: HkSpace.xl),
          Semantics(
            liveRegion: true,
            label: summary,
            child: Container(
              width: double.infinity,
              padding: const EdgeInsets.all(HkSpace.md),
              decoration: BoxDecoration(
                color: hasExpiry || noExpiry
                    ? AppColors.successSoft
                    : AppColors.surfaceMuted,
                borderRadius: BorderRadius.circular(HkRadius.md),
                border: Border.all(
                  color: hasExpiry || noExpiry
                      ? AppColors.success.withValues(alpha: 0.24)
                      : AppColors.cardBorder,
                ),
              ),
              child: Row(
                children: [
                  HkIcon(
                    hasExpiry || noExpiry ? HkIconName.check : HkIconName.clock,
                    size: 20,
                    color: hasExpiry || noExpiry
                        ? AppIconColors.success
                        : AppIconColors.muted,
                  ),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: Text(
                      summary,
                      style: TextStyle(
                        color: hasExpiry || noExpiry
                            ? AppColors.success
                            : AppColors.textMuted,
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  static String _formatDate(DateTime date) =>
      '${date.year}/${date.month.toString().padLeft(2, '0')}/${date.day.toString().padLeft(2, '0')}';
}

class _FieldTitle extends StatelessWidget {
  const _FieldTitle({required this.title, required this.description});

  final String title;
  final String description;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.baseline,
      textBaseline: TextBaseline.alphabetic,
      children: [
        Text(
          title,
          style: const TextStyle(
            color: AppColors.textEmphasis,
            fontSize: 16,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(width: HkSpace.sm),
        Expanded(
          child: Text(
            description,
            style: const TextStyle(color: AppColors.textMuted, fontSize: 12),
          ),
        ),
      ],
    );
  }
}

class _DayButton extends StatelessWidget {
  const _DayButton({
    required this.days,
    required this.isSelected,
    required this.onTap,
  });

  final int days;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      label: '有效 $days 天',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: AnimatedContainer(
            duration: HkMotion.duration,
            curve: HkMotion.ease,
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            decoration: BoxDecoration(
              color: isSelected
                  ? AppColors.brandPrimaryStrong
                  : AppColors.surfacePage,
              borderRadius: BorderRadius.circular(HkRadius.md),
              border: Border.all(
                color: isSelected
                    ? AppColors.brandPrimaryStrong
                    : AppColors.borderInput,
              ),
            ),
            child: Center(
              child: Text(
                '$days 天',
                style: TextStyle(
                  color: isSelected
                      ? AppColors.textInverse
                      : AppColors.textPrimary,
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ExpiryModeButton extends StatelessWidget {
  const _ExpiryModeButton({
    required this.icon,
    required this.label,
    required this.description,
    required this.isSelected,
    required this.onTap,
  });

  final HkIconName icon;
  final String label;
  final String description;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      label: '$label，$description',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: AnimatedContainer(
            duration: HkMotion.duration,
            constraints: const BoxConstraints(minHeight: 72),
            padding: const EdgeInsets.all(HkSpace.md),
            decoration: BoxDecoration(
              color: isSelected ? AppColors.primarySoft : AppColors.surfacePage,
              borderRadius: BorderRadius.circular(HkRadius.md),
              border: Border.all(
                color: isSelected
                    ? AppColors.borderPrimary
                    : AppColors.borderInput,
                width: isSelected ? 1.5 : 1,
              ),
            ),
            child: Row(
              children: [
                HkIcon(
                  icon,
                  size: 20,
                  color: isSelected
                      ? AppIconColors.active
                      : AppIconColors.muted,
                ),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        label,
                        style: TextStyle(
                          color: isSelected
                              ? AppColors.brandPrimaryStrong
                              : AppColors.textEmphasis,
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        description,
                        maxLines: 2,
                        style: const TextStyle(
                          color: AppColors.textMuted,
                          fontSize: 12,
                          height: 1.3,
                        ),
                      ),
                    ],
                  ),
                ),
                if (isSelected)
                  const HkIcon(
                    HkIconName.check,
                    size: 18,
                    color: AppIconColors.active,
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _CalendarWidget extends StatelessWidget {
  const _CalendarWidget({
    required this.calendarMonth,
    required this.validUntil,
    required this.hasExpiry,
    required this.onDateSelected,
    required this.onMonthChanged,
  });

  final DateTime calendarMonth;
  final DateTime validUntil;
  final bool hasExpiry;
  final ValueChanged<DateTime> onDateSelected;
  final ValueChanged<DateTime> onMonthChanged;

  @override
  Widget build(BuildContext context) {
    final now = DateTime.now();
    final year = calendarMonth.year;
    final month = calendarMonth.month;
    final firstWeekday = DateTime(year, month, 1).weekday % 7;
    final daysInMonth = DateTime(year, month + 1, 0).day;
    const weekdays = ['日', '一', '二', '三', '四', '五', '六'];

    return Container(
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: AppColors.surfacePage,
        borderRadius: BorderRadius.circular(HkRadius.lg),
        border: Border.all(color: AppColors.borderInput),
      ),
      child: Column(
        children: [
          Row(
            children: [
              _CalendarNavButton(
                semanticLabel: '上一個月',
                icon: HkIconName.back,
                onTap: () => onMonthChanged(DateTime(year, month - 1, 1)),
              ),
              Expanded(
                child: Text(
                  '$year 年 $month 月',
                  textAlign: TextAlign.center,
                  style: const TextStyle(
                    color: AppColors.textEmphasis,
                    fontSize: 16,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ),
              _CalendarNavButton(
                semanticLabel: '下一個月',
                icon: HkIconName.chevronRight,
                onTap: () => onMonthChanged(DateTime(year, month + 1, 1)),
              ),
            ],
          ),
          const SizedBox(height: HkSpace.sm),
          Row(
            children: weekdays
                .map(
                  (day) => Expanded(
                    child: Center(
                      child: Text(
                        day,
                        style: const TextStyle(
                          color: AppColors.textMuted,
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  ),
                )
                .toList(),
          ),
          const SizedBox(height: HkSpace.xs),
          ...List.generate(6, (weekIndex) {
            return Row(
              children: List.generate(7, (dayIndex) {
                final dayNumber = weekIndex * 7 + dayIndex - firstWeekday + 1;
                if (dayNumber < 1 || dayNumber > daysInMonth) {
                  return const Expanded(child: SizedBox(height: 42));
                }
                final date = DateTime(year, month, dayNumber);
                final isPast = date.isBefore(
                  DateTime(now.year, now.month, now.day),
                );
                final isSelected =
                    hasExpiry &&
                    validUntil.year == year &&
                    validUntil.month == month &&
                    validUntil.day == dayNumber;
                return Expanded(
                  child: Semantics(
                    button: !isPast,
                    selected: isSelected,
                    label: '$year 年 $month 月 $dayNumber 日',
                    child: Material(
                      color: Colors.transparent,
                      child: InkWell(
                        onTap: isPast ? null : () => onDateSelected(date),
                        customBorder: const CircleBorder(),
                        child: Container(
                          height: 42,
                          margin: const EdgeInsets.all(2),
                          decoration: BoxDecoration(
                            color: isSelected
                                ? AppColors.brandPrimaryStrong
                                : Colors.transparent,
                            shape: BoxShape.circle,
                          ),
                          child: Center(
                            child: Text(
                              '$dayNumber',
                              style: TextStyle(
                                color: isSelected
                                    ? AppColors.textInverse
                                    : isPast
                                    ? AppColors.textDisabled
                                    : AppColors.textPrimary,
                                fontSize: 14,
                                fontWeight: isSelected
                                    ? FontWeight.w900
                                    : FontWeight.w500,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                );
              }),
            );
          }),
        ],
      ),
    );
  }
}

class _CalendarNavButton extends StatelessWidget {
  const _CalendarNavButton({
    required this.semanticLabel,
    required this.icon,
    required this.onTap,
  });

  final String semanticLabel;
  final HkIconName icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: semanticLabel,
      child: Tooltip(
        message: semanticLabel,
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            customBorder: const CircleBorder(),
            child: SizedBox.square(
              dimension: HkControl.minTap,
              child: Center(
                child: HkIcon(icon, size: 20, color: AppIconColors.active),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
