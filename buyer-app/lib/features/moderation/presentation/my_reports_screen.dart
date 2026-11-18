import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as pb;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/report_labels.dart';
import 'my_reports_notifier.dart';
import 'paged_section.dart';
import 'paged_section_widgets.dart';

/// 我的檢舉紀錄列表頁（`ListMyReports`，只列社群對象；聊天、評價的檢舉在 A6 前不列）。
/// 伺服器狀態在 [myReportsProvider]；這裡只顯示與發動作。
class MyReportsScreen extends ConsumerWidget {
  const MyReportsScreen({super.key});

  Future<void> _reload(BuildContext context, WidgetRef ref) async {
    final ok = await ref.read(myReportsProvider.notifier).reload();
    if (!ok && context.mounted) {
      showFloatingToast(context, '載入失敗，請稍後再試', isSuccess: false);
    }
  }

  Future<void> _loadMore(BuildContext context, WidgetRef ref) async {
    final ok = await ref.read(myReportsProvider.notifier).loadMore();
    if (!ok && context.mounted) {
      showFloatingToast(context, '載入失敗，請稍後再試', isSuccess: false);
    }
  }

  String _formatTime(pb.ReportRecord r) {
    if (!r.hasCreateTime()) return '';
    final local = DateTime.fromMillisecondsSinceEpoch(
      r.createTime.seconds.toInt() * 1000,
    ).toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final reports = ref.watch(myReportsProvider);
    final locale = ref.watch(uiLocaleProvider);
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          commonTitleTopBar(
            context,
            title: '我的檢舉',
            onBack: () => context.pop(),
          ),
          Expanded(
            child: reports.phase == SectionPhase.loading
                ? const TruckLoadingOverlay(message: '載入中...')
                : reports.phase == SectionPhase.failed
                ? PagedErrorState(onRetry: () => _reload(context, ref))
                : reports.items.isEmpty
                ? const HkEmptyState(
                    key: ValueKey('my-reports-empty-state'),
                    iconKey: ValueKey('my-reports-empty-state-icon'),
                    icon: HkIconName.emptyAfterSale,
                    message: '尚無檢舉紀錄',
                  )
                : FruitRefreshIndicator(
                    onRefresh: () => _reload(context, ref),
                    child: ListView.builder(
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
                      itemCount:
                          reports.items.length + (reports.hasMore ? 1 : 0),
                      itemBuilder: (_, i) {
                        if (i == reports.items.length) {
                          return PagedLoadMoreFooter(
                            moreKey: const ValueKey('my-reports-more'),
                            loading: reports.loadingMore,
                            onMore: () => _loadMore(context, ref),
                          );
                        }
                        return Padding(
                          padding: EdgeInsets.only(bottom: 16.h),
                          child: _buildReportCard(reports.items[i], locale),
                        );
                      },
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildReportCard(pb.ReportRecord r, UiLocale locale) {
    final kind = reportTargetKindLabel(r.target.kind);
    final outcome = reportOutcomeLabel(r.outcome);
    final category = r.category.uiText(locale);
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  kind ?? '',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                ),
              ),
              if (outcome != null)
                Container(
                  padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                  decoration: BoxDecoration(
                    color: AppColors.gray100,
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                  child: Text(
                    outcome,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ),
            ],
          ),
          if (category != null) ...[
            SizedBox(height: 6.h),
            Row(
              children: [
                HkIcon(
                  HkIconName.warning,
                  size: 18.sp,
                  color: AppIconColors.warning,
                ),
                SizedBox(width: 6.w),
                Text(
                  '原因：$category',
                  style: TextStyle(fontSize: 14, color: AppColors.textPrimary),
                ),
              ],
            ),
          ],
          if (r.description.isNotEmpty) ...[
            SizedBox(height: 6.h),
            Text(
              r.description,
              style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          SizedBox(height: 6.h),
          Row(
            children: [
              HkIcon(
                HkIconName.clock,
                size: 16.sp,
                color: AppIconColors.defaultColor,
              ),
              SizedBox(width: 6.w),
              Text(
                _formatTime(r),
                style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
