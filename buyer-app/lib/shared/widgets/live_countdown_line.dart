import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 即時倒數行（label + HH:MM:SS / N天 HH:MM:SS），每秒 tick 一次。
///
/// 用 [FontFeature.tabularFigures] 保證每個數字寬度固定，秒數跳動時不會跳位。
/// 傳 ISO 8601 deadline 字串即可。
///
/// 配色固定：normal 橘 / urgent 紅 / expired 灰，跨頁面共用。
/// 訂單詳情頁、售後詳情頁均使用本 widget，避免重複實作。
class LiveCountdownLine extends StatefulWidget {
  const LiveCountdownLine({
    super.key,
    required this.label,
    required this.deadlineIso,
    this.expiredText = '已逾時',
    this.urgentThreshold = const Duration(hours: 1),
  });

  final String label;
  final String deadlineIso;
  final String expiredText;
  final Duration urgentThreshold;

  // 配色固定：normal 橘 / urgent 紅 / expired 灰，跟其他 deadline row 共用。
  static const Color kUrgent = AppColors.danger;
  static const Color kNormal = AppColors.primary;
  static const Color kExpired = AppColors.textSecondary;

  @override
  State<LiveCountdownLine> createState() => _LiveCountdownLineState();
}

class _LiveCountdownLineState extends State<LiveCountdownLine> {
  Timer? _ticker;

  @override
  void initState() {
    super.initState();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  String _format(Duration r) {
    if (r.isNegative) return widget.expiredText;
    final d = r.inDays;
    final h = r.inHours.remainder(24).toString().padLeft(2, '0');
    final m = r.inMinutes.remainder(60).toString().padLeft(2, '0');
    final s = r.inSeconds.remainder(60).toString().padLeft(2, '0');
    if (d >= 1) return '$d 天 $h:$m:$s';
    return '$h:$m:$s';
  }

  TextStyle _smallStyle() => TextStyle(
    fontSize: 14,
    fontWeight: FontWeight.w400,
    height: 20 / 13,
    color: AppColors.textSecondary,
  );

  @override
  Widget build(BuildContext context) {
    final dt = DateTime.tryParse(widget.deadlineIso);
    if (dt == null) {
      // 無有效時間：靜態顯示 dash，與 _InfoLine 保持一致排版
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 96.w,
            child: Text(
              widget.label,
              style: _smallStyle().copyWith(color: AppColors.textSecondary),
            ),
          ),
          SizedBox(width: 10.w),
          Expanded(
            child: Text(
              '—',
              textAlign: TextAlign.end,
              style: _smallStyle().copyWith(color: AppColors.textEmphasis),
            ),
          ),
        ],
      );
    }
    final remaining = dt.toLocal().difference(DateTime.now());
    final expired = remaining.isNegative;
    final urgent = !expired && remaining < widget.urgentThreshold;
    final color = expired
        ? LiveCountdownLine.kExpired
        : (urgent ? LiveCountdownLine.kUrgent : LiveCountdownLine.kNormal);

    final bool highlight = urgent || expired;
    final Widget valueText = Text(
      _format(remaining),
      textAlign: TextAlign.end,
      style: _smallStyle().copyWith(
        color: color,
        fontSize: 14,
        fontWeight: FontWeight.w700,
        letterSpacing: 0.5,
        // 等寬數字：tick 時不會因為字寬不同造成抖動
        fontFeatures: const [FontFeature.tabularFigures()],
      ),
    );

    // 緊急 / 已逾時 → 把倒數值包成淡色 chip 突出，normal 維持 plain text。
    final Widget valueWidget = highlight
        ? Container(
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8.r),
            ),
            child: valueText,
          )
        : valueText;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        SizedBox(
          width: 96.w,
          child: Text(
            widget.label,
            style: _smallStyle().copyWith(color: AppColors.textSecondary),
          ),
        ),
        SizedBox(width: 10.w),
        Expanded(
          child: Align(alignment: Alignment.centerRight, child: valueWidget),
        ),
      ],
    );
  }
}
