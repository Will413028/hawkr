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

import 'package:hawkr_api/errors.dart' show errorText;

import '../../../core/networking/suspension_signal.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../seller/data/store_card_provider.dart';
import 'my_reports_notifier.dart';
import 'paged_section.dart';
import 'paged_section_widgets.dart';
import 'report_view_parts.dart';
import 'store_report_detail_screen.dart';

/// 我的檢舉紀錄列表頁（`ListMyReports` 合併社群與店家兩種；店家檢舉可點進詳情）。
/// 伺服器狀態在 [myReportsProvider]；這裡只顯示與發動作。
class MyReportsScreen extends ConsumerWidget {
  const MyReportsScreen({super.key});

  void _toastError(BuildContext context, WidgetRef ref) {
    final error = ref.read(myReportsProvider).error;
    if (error != null && isAccountSuspendedError(error)) return;
    showFloatingToast(context, _errorMessage(ref, error), isSuccess: false);
  }

  String _errorMessage(WidgetRef ref, Object? error) => errorText(
    error ?? StateError('unknown'),
    defaults: ref.read(defaultErrorTextProvider),
  );

  Future<void> _reload(BuildContext context, WidgetRef ref) async {
    for (final r in ref.read(myReportsProvider).items) {
      if (r.hasStore() &&
          ref.read(storeCardProvider(r.store.storeId)).hasError) {
        ref.invalidate(storeCardProvider(r.store.storeId));
      }
    }
    final ok = await ref.read(myReportsProvider.notifier).reload();
    if (!ok && context.mounted) {
      _toastError(context, ref);
    }
  }

  Future<void> _loadMore(BuildContext context, WidgetRef ref) async {
    final ok = await ref.read(myReportsProvider.notifier).loadMore();
    if (!ok && context.mounted) {
      _toastError(context, ref);
    }
  }

  String _formatTime(pb.MyReportItem r) =>
      r.hasCreateTime() ? formatReportTime(r.createTime.toDateTime()) : '';

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final reports = ref.watch(myReportsProvider);
    final visible = reports.items
        .where((r) => r.hasCommunity() || r.hasStore())
        .toList();
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
                ? PagedErrorState(
                    message: _errorMessage(ref, reports.error),
                    onRetry: () => _reload(context, ref),
                  )
                : visible.isEmpty
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
                      itemCount: visible.length + (reports.hasMore ? 1 : 0),
                      itemBuilder: (_, i) {
                        if (i == visible.length) {
                          return PagedLoadMoreFooter(
                            moreKey: const ValueKey('my-reports-more'),
                            loading: reports.loadingMore,
                            onMore: () => _loadMore(context, ref),
                          );
                        }
                        return Padding(
                          padding: EdgeInsets.only(bottom: 16.h),
                          child: _buildReportCard(context, visible[i], locale),
                        );
                      },
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildReportCard(
    BuildContext context,
    pb.MyReportItem r,
    UiLocale locale,
  ) {
    final store = r.hasStore();
    const titleStyle = TextStyle(
      fontSize: 16,
      fontWeight: FontWeight.w700,
      color: AppColors.textPrimary,
    );
    final outcome = store
        ? storeReportOutcomeText(r.store, locale)
        : r.community.outcome.uiText(locale);
    final category = r.category.uiText(locale);
    final card = Ink(
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
                child: store
                    ? StoreNameText(r.store.storeId, style: titleStyle)
                    : Text(
                        r.community.target.kind.uiText(locale) ?? '',
                        style: titleStyle,
                      ),
              ),
              if (outcome != null) ReportOutcomeChip(outcome),
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
              if (store) ...[
                const Spacer(),
                HkIcon(
                  HkIconName.chevronRight,
                  size: 18.sp,
                  color: AppIconColors.defaultColor,
                ),
              ],
            ],
          ),
        ],
      ),
    );
    if (!store) return card;
    return InkWell(
      key: ValueKey('my-report-store-${r.id}'),
      borderRadius: BorderRadius.circular(16.r),
      onTap: () => context.push(storeReportDetailRoute(r.id)),
      child: card,
    );
  }
}
