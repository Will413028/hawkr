import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show MeasureState;
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../seller/data/measure_display.dart';
import '../../seller/data/measures.dart';

/// 首頁的終止合作預告提示卡：帳號每一筆預定生效（`SCHEDULED`）的終止合作各一張，顯示生效時間與申覆的提醒，點擊到措施與申覆。
///
/// 生效時間讀措施的 `period.start_time`（伺服器給）。預告是提醒不是已發生的處罰，用 warning 色調（出貨異常卡才是 error）。
/// 讀取中、讀取失敗或沒有預告時不顯示，也不擋首頁。
class TerminationNoticeBanner extends ConsumerWidget {
  const TerminationNoticeBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final measures = ref
        .watch(currentMyMeasuresProvider(MeasureState.MEASURE_STATE_SCHEDULED))
        ?.value;
    final notices = measures == null
        ? const <({String measureId, String text})>[]
        : terminationNotices(measures, context.uiLocale);
    if (notices.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [for (final notice in notices) _NoticeCard(notice.text)],
    );
  }
}

class _NoticeCard extends StatelessWidget {
  const _NoticeCard(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: 16.h),
      child: Material(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: () => context.push(SellerRoutes.measures),
          child: Container(
            decoration: BoxDecoration(
              border: Border.all(color: AppColors.warning, width: 1),
              borderRadius: BorderRadius.circular(16.r),
            ),
            padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
            child: Row(
              children: [
                Icon(
                  AppIcons.warningAmberRounded,
                  size: 24.sp,
                  color: AppIconColors.warning,
                ),
                SizedBox(width: 8.w),
                Expanded(
                  child: Text(
                    text,
                    key: const ValueKey('termination-notice-text'),
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                      height: 1.4,
                    ),
                  ),
                ),
                Icon(
                  AppIcons.chevronRight,
                  size: 22.sp,
                  color: AppIconColors.muted,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
