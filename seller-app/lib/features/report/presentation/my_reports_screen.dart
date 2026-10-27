import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/report_model.dart';
import 'report_list_controller.dart';

/// 我的檢舉列表頁（backend-payuni#645）。
class MyReportsScreen extends HookConsumerWidget {
  const MyReportsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(myReportsControllerProvider);
    final scrollController = useScrollController();

    useEffect(() {
      void onScroll() {
        if (scrollController.position.pixels >=
            scrollController.position.maxScrollExtent - 200) {
          // loadMore 失敗只跳 toast、保留既有清單（不整頁切成 error 狀態）
          ref.read(myReportsControllerProvider.notifier).loadMore().catchError((
            Object e,
          ) {
            if (!context.mounted) return;
            FloatingToast.error(
              context,
              e is AppException ? e.message : '載入更多失敗，請稍後再試',
            );
          });
        }
      }

      scrollController.addListener(onScroll);
      return () => scrollController.removeListener(onScroll);
    }, [scrollController]);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('我的檢舉'),
        onBackPressed: () => context.pop(),
      ),
      body: asyncState.when(
        loading: () => const TruckLoadingOverlay(),
        error: (error, _) => _buildErrorState(context, ref, error),
        data: (state) => FruitRefreshIndicator(
          // silentRefresh 失敗只跳 toast、保留既有清單（不整頁切成 error 狀態）
          onRefresh: () => ref
              .read(myReportsControllerProvider.notifier)
              .silentRefresh()
              .catchError((Object e) {
                if (!context.mounted) return;
                FloatingToast.error(
                  context,
                  e is AppException ? e.message : '刷新失敗，請稍後再試',
                );
              }),
          child: state.reports.isEmpty
              ? _buildEmptyState()
              : ListView.builder(
                  physics: const AlwaysScrollableScrollPhysics(),
                  controller: scrollController,
                  padding: EdgeInsets.symmetric(
                    horizontal: 16.w,
                    vertical: 12.h,
                  ),
                  itemCount:
                      state.reports.length + (state.isLoadingMore ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (index == state.reports.length) {
                      return Padding(
                        padding: EdgeInsets.all(16.w),
                        child: const Center(
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                      );
                    }
                    return _ReportCard(report: state.reports[index]);
                  },
                ),
        ),
      ),
    );
  }

  Widget _buildErrorState(BuildContext context, WidgetRef ref, Object error) {
    final message = error is AppException ? error.message : '載入失敗，請稍後再試';
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(AppIcons.errorOutline, size: 64.sp, color: AppIconColors.muted),
          SizedBox(height: 16.h),
          Text(
            message,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            textAlign: TextAlign.center,
          ),
          SizedBox(height: 16.h),
          ElevatedButton(
            onPressed: () =>
                ref.read(myReportsControllerProvider.notifier).refresh(),
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.primary),
            child: const HkButtonLabel(
              '重試',
              style: TextStyle(color: AppColors.bgCard),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      children: const [
        HkEmptyState(
          key: ValueKey('seller-my-reports-empty-state'),
          iconKey: ValueKey('seller-my-reports-empty-state-icon'),
          icon: HkIconName.document,
          message: '尚無檢舉紀錄',
          description: '你送出的檢舉會顯示在這裡，方便追蹤處理進度。',
          topPadding: 88,
        ),
      ],
    );
  }
}

class _ReportCard extends StatelessWidget {
  const _ReportCard({required this.report});

  final ReportRecord report;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: EdgeInsets.only(bottom: 12.h),
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.dividerLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  '${reportTargetTypeLabel(report.targetType)}・${reportReasonLabel(report.reasonCode)}',
                  style: TextStyle(
                    fontSize: 16.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ),
              SizedBox(width: 8.w),
              _StatusPill(status: report.status),
            ],
          ),
          if (report.description.isNotEmpty) ...[
            SizedBox(height: 6.h),
            Text(
              report.description,
              style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            ),
          ],
          SizedBox(height: 8.h),
          Text(
            _formatMs(report.createTimeMs),
            style: TextStyle(fontSize: 12.sp, color: AppColors.textSecondary),
          ),
        ],
      ),
    );
  }

  String _formatMs(int ms) {
    if (ms <= 0) return '';
    final dt = DateTime.fromMillisecondsSinceEpoch(ms).toLocal();
    final mm = dt.month.toString().padLeft(2, '0');
    final dd = dt.day.toString().padLeft(2, '0');
    return '${dt.year}/$mm/$dd';
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: AppColors.primarySoft,
        borderRadius: BorderRadius.circular(999.r),
      ),
      child: Text(
        reportStatusLabel(status),
        style: TextStyle(
          fontSize: 12.sp,
          fontWeight: FontWeight.w700,
          color: AppColors.textAccentOnTint,
        ),
      ),
    );
  }
}
