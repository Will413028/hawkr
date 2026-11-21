import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as seller;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../seller/data/store_card_provider.dart';

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

/// 店名：讀取中與失敗時顯示中性的骨架條；失敗時點一下重讀。
class StoreNameText extends ConsumerWidget {
  const StoreNameText(this.storeId, {super.key, required this.style});

  final String storeId;
  final TextStyle style;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final card = ref.watch(storeCardProvider(storeId));
    if (card.hasError && !card.isLoading) {
      return GestureDetector(
        key: const ValueKey('store-name-retry'),
        behavior: HitTestBehavior.opaque,
        onTap: () => ref.invalidate(storeCardProvider(storeId)),
        child: _bar(),
      );
    }
    if (!card.hasValue) return _bar();
    return Text(card.requireValue?.name ?? '', style: style);
  }

  Widget _bar() => Align(
    alignment: Alignment.centerLeft,
    child: Container(
      key: const ValueKey('store-name-skeleton'),
      width: 96.w,
      height: 18.h,
      decoration: BoxDecoration(
        color: AppColors.gray200,
        borderRadius: BorderRadius.circular(HkRadius.xs),
      ),
    ),
  );
}
