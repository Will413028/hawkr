import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as seller;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

/// 店家檢舉的結果文字：沒設結果是「審核中」，設了照目錄；目錄沒有文字的值回 null。
String? storeReportOutcomeText(seller.StoreReport report, UiLocale locale) =>
    report.hasOutcome()
    ? report.outcome.uiText(locale)
    : sellerStoreReportOutcomeUnset(locale);

String formatReportTime(DateTime time) {
  final local = time.toLocal();
  return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
      '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
}

class ReportOutcomeChip extends StatelessWidget {
  const ReportOutcomeChip(this.text, {super.key});

  final String text;

  @override
  Widget build(BuildContext context) => Container(
    padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
    decoration: BoxDecoration(
      color: AppColors.gray100,
      borderRadius: BorderRadius.circular(999.r),
    ),
    child: Text(
      text,
      style: const TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w700,
        color: AppColors.textSecondary,
      ),
    ),
  );
}
