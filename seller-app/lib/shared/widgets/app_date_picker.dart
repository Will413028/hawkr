import 'dart:ui' as ui;

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_tap_target.dart';

/// 民國日曆日期選擇器（對齊 Figma 設計稿）
///
/// 兩種模式：
/// 1. 日曆模式 — 顯示月曆格子，點選日期
/// 2. 年月快選模式 — 點擊標題切換，左右兩欄滾動選年/月
///
/// 使用方式：
/// ```dart
/// final date = await AppDatePicker.show(context, initial: DateTime(1990, 1, 1));
/// ```
class AppDatePicker extends StatefulWidget {
  const AppDatePicker({
    super.key,
    this.initialDate,
    this.firstDate,
    this.lastDate,
  });

  final DateTime? initialDate;
  final DateTime? firstDate;
  final DateTime? lastDate;

  static const double barrierOpacity = 0.10;

  static Future<DateTime?> show(
    BuildContext context, {
    DateTime? initial,
    DateTime? firstDate,
    DateTime? lastDate,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return showDialog<DateTime>(
      context: context,
      barrierColor: scheme.scrim.withValues(alpha: barrierOpacity),
      builder: (dialogContext) {
        final viewportWidth = MediaQuery.sizeOf(dialogContext).width;
        // A seven-column calendar cannot provide even 44dp date targets in a
        // 320dp viewport with the old 32dp margins. Keep a comfortable 16dp
        // inset on current phones and use a compact 6dp inset only below 375.
        final horizontalInset = viewportWidth < 375 ? 6.0 : HkSpace.lg;
        return BackdropFilter(
          filter: ui.ImageFilter.blur(
            sigmaX: HkGlass.dialogBlurSigma,
            sigmaY: HkGlass.dialogBlurSigma,
          ),
          child: Dialog(
            backgroundColor: Colors.transparent,
            insetPadding: EdgeInsets.symmetric(
              horizontal: horizontalInset,
              vertical: HkSpace.sm,
            ),
            child: SingleChildScrollView(
              primary: false,
              child: AppDatePicker(
                initialDate: initial,
                firstDate: firstDate,
                lastDate: lastDate,
              ),
            ),
          ),
        );
      },
    );
  }

  @override
  State<AppDatePicker> createState() => _AppDatePickerState();
}

class _AppDatePickerState extends State<AppDatePicker> {
  // ── 設計稿顏色 ────────────────────────────────────────────────────
  static const _green = AppColors.primary;
  static const _weekendColor = AppColors.primary;
  static const _weekdayLabelColor = AppColors.textSecondary;
  static const _dateColor = AppColors.textEmphasis;
  static const _borderColor = AppColors.cardBorder; // rgba(86,86,86,0.2)
  static const _cancelTextColor = AppColors.textPrimary;
  static const _yearMonthUnselectedColor = AppColors.textSecondary;

  late DateTime _selectedDate;
  late DateTime _displayMonth;
  bool _yearMonthMode = false;

  late final DateTime _firstDate;
  late final DateTime _lastDate;
  late FixedExtentScrollController _yearWheelCtrl;
  late FixedExtentScrollController _monthWheelCtrl;
  late List<int> _years;

  @override
  void initState() {
    super.initState();
    _selectedDate = widget.initialDate ?? DateTime.now();
    _displayMonth = DateTime(_selectedDate.year, _selectedDate.month);
    _firstDate = widget.firstDate ?? DateTime(1920);
    _lastDate = widget.lastDate ?? DateTime.now();
    _years = List.generate(
      _lastDate.year - _firstDate.year + 1,
      (i) => _lastDate.year - i,
    );
    _yearWheelCtrl = FixedExtentScrollController(
      initialItem: _years
          .indexOf(_displayMonth.year)
          .clamp(0, _years.length - 1),
    );
    _monthWheelCtrl = FixedExtentScrollController(
      initialItem: _displayMonth.month - 1,
    );
  }

  @override
  void dispose() {
    _yearWheelCtrl.dispose();
    _monthWheelCtrl.dispose();
    super.dispose();
  }

  int _toRoc(int year) => year - 1911;
  String _rocLabel(DateTime dt) => '民國 ${_toRoc(dt.year)} 年 ${dt.month} 月';

  // ── Build ───────────────────────────────────────────────────────────────
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: scheme.surface.withValues(alpha: 0.96),
        borderRadius: BorderRadius.circular(HkRadius.xl),
        boxShadow: HkShadows.modal,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildHeader(),
          if (_yearMonthMode) _buildYearMonthPicker() else _buildCalendar(),
          _buildFooter(),
        ],
      ),
    );
  }

  // ── Header (60px) ───────────────────────────────────────────────────────
  Widget _buildHeader() {
    return Container(
      height: 60.h,
      padding: EdgeInsets.symmetric(horizontal: 16.w),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: _borderColor, width: 1)),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          // Left nav
          if (!_yearMonthMode)
            _navButton(AppIcons.chevronLeft, '上個月', () {
              setState(() {
                _displayMonth = DateTime(
                  _displayMonth.year,
                  _displayMonth.month - 1,
                );
              });
            })
          else
            const SizedBox.square(dimension: HkControl.minTap),

          // Title (tappable to toggle year/month mode)
          Expanded(
            child: HkTapTarget(
              semanticLabel: '${_rocLabel(_displayMonth)}，切換年月選擇',
              hint: _yearMonthMode ? '關閉年月快速選擇' : '開啟年月快速選擇',
              toggled: _yearMonthMode,
              onTap: _toggleYearMonthMode,
              child: Container(
                padding: _yearMonthMode
                    ? EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h)
                    : EdgeInsets.zero,
                decoration: _yearMonthMode
                    ? BoxDecoration(
                        color: const Color(0x0A000000), // rgba(0,0,0,0.04)
                        borderRadius: BorderRadius.circular(12.r),
                      )
                    : null,
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Flexible(
                      child: Text(
                        _rocLabel(_displayMonth),
                        maxLines: 1,
                        softWrap: false,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                          height: 24 / 16,
                        ),
                      ),
                    ),
                    SizedBox(width: 6.w),
                    // Up/Down chevrons (12x25 container with 12x12 icons)
                    SizedBox(
                      width: 12.w,
                      height: 25.h,
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(
                            AppIcons.keyboardArrowUp,
                            size: 12.w,
                            color: AppIconColors.defaultColor,
                          ),
                          SizedBox(height: 1.h),
                          Icon(
                            AppIcons.keyboardArrowDown,
                            size: 12.w,
                            color: AppIconColors.defaultColor,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),

          // Right nav
          if (!_yearMonthMode)
            _navButton(AppIcons.chevronRight, '下個月', () {
              setState(() {
                _displayMonth = DateTime(
                  _displayMonth.year,
                  _displayMonth.month + 1,
                );
              });
            })
          else
            const SizedBox.square(dimension: HkControl.minTap),
        ],
      ),
    );
  }

  Widget _navButton(IconData icon, String label, VoidCallback onTap) {
    return HkTapTarget(
      semanticLabel: label,
      onTap: onTap,
      child: Container(
        width: 36.w,
        height: 36.w,
        decoration: const BoxDecoration(
          color: Color(0x0A000000), // rgba(0,0,0,0.04)
          shape: BoxShape.circle,
        ),
        child: Center(
          child: Icon(icon, size: 18.w, color: AppIconColors.defaultColor),
        ),
      ),
    );
  }

  void _toggleYearMonthMode() {
    if (!_yearMonthMode) {
      // Entering year/month mode — re-create controllers at current position.
      _yearWheelCtrl.dispose();
      _monthWheelCtrl.dispose();
      _yearWheelCtrl = FixedExtentScrollController(
        initialItem: _years
            .indexOf(_displayMonth.year)
            .clamp(0, _years.length - 1),
      );
      _monthWheelCtrl = FixedExtentScrollController(
        initialItem: _displayMonth.month - 1,
      );
    }
    setState(() => _yearMonthMode = !_yearMonthMode);
  }

  // ── Calendar Grid (RWD + 固定 6 週高度防抖動) ──────────────────────────
  static const _maxWeeks = 6; // 固定 6 週，避免月份切換時高度跳動
  static const _rowH = HkControl.minTap;

  Widget _buildCalendar() {
    const weekdays = ['日', '一', '二', '三', '四', '五', '六'];
    final firstOfMonth = DateTime(_displayMonth.year, _displayMonth.month, 1);
    final daysInMonth = DateTime(
      _displayMonth.year,
      _displayMonth.month + 1,
      0,
    ).day;
    final startWeekday = firstOfMonth.weekday % 7; // Sunday = 0

    return LayoutBuilder(
      builder: (context, constraints) {
        // 390dp and wider retain the designed 8dp calendar inset with 48dp+
        // date cells. Narrow widths spend that inset on target size; at the
        // 320dp lower bound each date remains 44dp wide and 48dp tall.
        final horizontalPadding = constraints.maxWidth >= 352 ? 8.0 : 0.0;
        return Padding(
          padding: EdgeInsets.symmetric(horizontal: horizontalPadding),
          child: Column(
            children: [
              // Weekday labels row
              Padding(
                padding: EdgeInsets.symmetric(vertical: 4.h),
                child: Row(
                  children: List.generate(7, (i) {
                    final isWeekend = i == 0 || i == 6;
                    return Expanded(
                      child: Center(
                        child: Text(
                          weekdays[i],
                          maxLines: 1,
                          softWrap: false,
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w500,
                            color: isWeekend
                                ? _weekendColor
                                : _weekdayLabelColor,
                            letterSpacing: -0.08,
                            height: 19.5 / 13,
                          ),
                        ),
                      ),
                    );
                  }),
                ),
              ),
              // Date grid — 固定 6 週高度，用 Expanded 自適應寬度
              ...List.generate(_maxWeeks, (week) {
                return SizedBox(
                  height: _rowH,
                  child: Row(
                    children: List.generate(7, (col) {
                      final dayIndex = week * 7 + col - startWeekday + 1;
                      if (dayIndex < 1 || dayIndex > daysInMonth) {
                        return const Expanded(child: SizedBox.shrink());
                      }
                      final date = DateTime(
                        _displayMonth.year,
                        _displayMonth.month,
                        dayIndex,
                      );
                      final isSelected = _isSameDay(date, _selectedDate);
                      final isWeekend = col == 0 || col == 6;

                      void selectDate() => setState(() => _selectedDate = date);
                      return Expanded(
                        child: Semantics(
                          button: true,
                          selected: isSelected,
                          label:
                              '民國 ${_toRoc(date.year)} 年 ${date.month} 月 ${date.day} 日',
                          onTap: selectDate,
                          child: ExcludeSemantics(
                            child: GestureDetector(
                              behavior: HitTestBehavior.opaque,
                              onTap: selectDate,
                              child: Center(
                                child: AspectRatio(
                                  aspectRatio: 1,
                                  child: Container(
                                    decoration: isSelected
                                        ? const BoxDecoration(
                                            color: _green,
                                            shape: BoxShape.circle,
                                            boxShadow: [
                                              BoxShadow(
                                                color: AppColors.cardShadow,
                                                blurRadius: 12,
                                                offset: Offset(0, 3),
                                              ),
                                            ],
                                          )
                                        : null,
                                    child: Center(
                                      child: Text(
                                        '$dayIndex',
                                        style: TextStyle(
                                          fontSize: 16,
                                          fontWeight: isSelected
                                              ? FontWeight.w700
                                              : FontWeight.w400,
                                          color: isSelected
                                              ? AppColors.bgCard
                                              : isWeekend
                                              ? _weekendColor
                                              : _dateColor,
                                          letterSpacing: -0.31,
                                          height: 24 / 16,
                                        ),
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ),
                      );
                    }),
                  ),
                );
              }),
            ],
          ),
        );
      },
    );
  }

  bool _isSameDay(DateTime a, DateTime b) =>
      a.year == b.year && a.month == b.month && a.day == b.day;

  // ── Year/Month Picker (wheel style) ─────────────────────────────────────
  Widget _buildYearMonthPicker() {
    const itemExtent = 44.0;

    return Container(
      height: 264.h,
      decoration: const BoxDecoration(color: Color(0x80FFFFFF)),
      child: Stack(
        children: [
          // 中間選中區域高亮背景
          Center(
            child: Container(
              height: itemExtent,
              margin: EdgeInsets.symmetric(horizontal: 12.w),
              decoration: BoxDecoration(
                color: _borderColor,
                borderRadius: BorderRadius.circular(8.r),
              ),
            ),
          ),
          Row(
            children: [
              // Year wheel
              Expanded(
                child: Container(
                  decoration: const BoxDecoration(
                    border: Border(
                      right: BorderSide(color: _borderColor, width: 1),
                    ),
                  ),
                  child: ListWheelScrollView.useDelegate(
                    controller: _yearWheelCtrl,
                    itemExtent: itemExtent,
                    perspective: 0.003,
                    diameterRatio: 1.5,
                    physics: const FixedExtentScrollPhysics(),
                    onSelectedItemChanged: (i) {
                      setState(() {
                        _displayMonth = DateTime(
                          _years[i],
                          _displayMonth.month,
                        );
                      });
                    },
                    childDelegate: ListWheelChildBuilderDelegate(
                      childCount: _years.length,
                      builder: (_, i) {
                        final year = _years[i];
                        final isSelected = year == _displayMonth.year;
                        return Center(
                          child: Text(
                            '民國 ${_toRoc(year)} 年',
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: isSelected
                                  ? FontWeight.w700
                                  : FontWeight.w500,
                              color: isSelected
                                  ? AppColors.textEmphasis
                                  : _yearMonthUnselectedColor,
                              letterSpacing: -0.23,
                              height: 22.5 / 15,
                            ),
                          ),
                        );
                      },
                    ),
                  ),
                ),
              ),
              // Month wheel
              Expanded(
                child: ListWheelScrollView.useDelegate(
                  controller: _monthWheelCtrl,
                  itemExtent: itemExtent,
                  perspective: 0.003,
                  diameterRatio: 1.5,
                  physics: const FixedExtentScrollPhysics(),
                  onSelectedItemChanged: (i) {
                    setState(() {
                      _displayMonth = DateTime(_displayMonth.year, i + 1);
                    });
                  },
                  childDelegate: ListWheelChildBuilderDelegate(
                    childCount: 12,
                    builder: (_, i) {
                      final month = i + 1;
                      final isSelected = month == _displayMonth.month;
                      return Center(
                        child: Text(
                          '$month 月',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: isSelected
                                ? FontWeight.w700
                                : FontWeight.w500,
                            color: isSelected
                                ? AppColors.textEmphasis
                                : _yearMonthUnselectedColor,
                            letterSpacing: -0.23,
                            height: 22.5 / 15,
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  // ── Footer ──────────────────────────────────────────────────────────────
  Widget _buildFooter() {
    if (_yearMonthMode) {
      // 年月模式 — Figma: border-top rgba(0,0,0,0.06), pb=20, pt≈13
      return Container(
        padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 20.h),
        decoration: const BoxDecoration(
          border: Border(top: BorderSide(color: Color(0x0F000000), width: 1)),
        ),
        child: SizedBox(
          width: double.infinity,
          height: HkControl.minTap,
          child: TextButton(
            onPressed: () => setState(() => _yearMonthMode = false),
            style: TextButton.styleFrom(
              backgroundColor: _green,
              foregroundColor: AppColors.bgCard,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8.r),
              ),
              padding: EdgeInsets.symmetric(vertical: 4.h),
              textStyle: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w700,
                height: 24 / 16,
              ),
            ),
            child: const HkButtonLabel('確定'),
          ),
        ),
      );
    }

    // 日曆模式 — Figma: gap=12, pb=20, pt=12
    return Container(
      padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 20.h),
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: Color(0x0F000000), width: 1)),
      ),
      child: Row(
        children: [
          // 取消 — border rgba(86,86,86,0.2), Inter Medium 16px, #555
          Expanded(
            child: SizedBox(
              height: HkControl.minTap,
              child: TextButton(
                onPressed: () => Navigator.of(context).pop(),
                style: TextButton.styleFrom(
                  foregroundColor: _cancelTextColor,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8.r),
                    side: const BorderSide(color: _borderColor),
                  ),
                  padding: EdgeInsets.symmetric(vertical: 4.h),
                  textStyle: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                    letterSpacing: -0.31,
                    height: 24 / 16,
                  ),
                ),
                child: const HkButtonLabel('取消'),
              ),
            ),
          ),
          SizedBox(width: 12.w),
          // 完成 — bg primary, Inter SemiBold 16px, white
          Expanded(
            child: SizedBox(
              height: HkControl.minTap,
              child: TextButton(
                onPressed: () => Navigator.of(context).pop(_selectedDate),
                style: TextButton.styleFrom(
                  backgroundColor: _green,
                  foregroundColor: AppColors.bgCard,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8.r),
                  ),
                  padding: EdgeInsets.symmetric(vertical: 4.h),
                  textStyle: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    letterSpacing: -0.31,
                    height: 24 / 16,
                  ),
                ),
                child: const HkButtonLabel('完成'),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
