import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../social/data/social_repository.dart';
import '../data/report_record_model.dart';
import '../data/report_target_type.dart';

/// 我的檢舉紀錄列表頁
class MyReportsScreen extends ConsumerStatefulWidget {
  const MyReportsScreen({super.key});

  @override
  ConsumerState<MyReportsScreen> createState() => _MyReportsScreenState();
}

class _MyReportsScreenState extends ConsumerState<MyReportsScreen> {
  List<ReportRecord> _reports = [];
  bool _isLoading = true;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _loadReports();
  }

  Future<void> _loadReports() async {
    try {
      final result = await ref.read(socialRepositoryProvider).getMyReports();
      if (mounted) {
        setState(() {
          _reports = result;
          _isLoading = false;
          _errorMessage = null;
        });
      }
    } catch (e) {
      if (!mounted) return;
      final message = e is AppException ? e.message : '載入失敗，請稍後再試';
      // 已有清單時（下拉刷新失敗）不整頁蓋掉，保留已顯示的資料改用 toast
      // 提示；只有初次載入（清單還是空的）才用整頁錯誤狀態接管。
      if (_reports.isEmpty) {
        setState(() {
          _isLoading = false;
          _errorMessage = message;
        });
      } else {
        setState(() => _isLoading = false);
        showFloatingToast(context, message, isSuccess: false);
      }
    }
  }

  String _formatTime(int ms) {
    if (ms <= 0) return '';
    final local = DateTime.fromMillisecondsSinceEpoch(ms).toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }

  /// 讀取失敗狀態——與「真的沒資料」的空狀態分開顯示，避免使用者誤以為
  /// 沒有檢舉紀錄；提供重試按鈕重新觸發 [_loadReports]。
  Widget _buildErrorState(String message) {
    return Center(
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 32.w),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              AppIcons.errorOutline,
              color: AppIconColors.danger,
              size: 48.sp,
            ),
            SizedBox(height: 12.h),
            Text(
              message,
              textAlign: TextAlign.center,
              style: TextStyle(color: AppColors.textPrimary, fontSize: 14),
            ),
            SizedBox(height: 12.h),
            TextButton(
              onPressed: _loadReports,
              child: const HkButtonLabel('重試'),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
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
            child: _isLoading
                ? const TruckLoadingOverlay(message: '載入中...')
                : _errorMessage != null
                ? _buildErrorState(_errorMessage!)
                : _reports.isEmpty
                ? const HkEmptyState(
                    key: ValueKey('my-reports-empty-state'),
                    iconKey: ValueKey('my-reports-empty-state-icon'),
                    icon: HkIconName.emptyAfterSale,
                    message: '尚無檢舉紀錄',
                  )
                : FruitRefreshIndicator(
                    onRefresh: _loadReports,
                    child: ListView.builder(
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
                      itemCount: _reports.length,
                      itemBuilder: (_, i) => Padding(
                        padding: EdgeInsets.only(bottom: 16.h),
                        child: _buildReportCard(_reports[i]),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildReportCard(ReportRecord r) {
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
                  reportTargetTypeLabel(r.targetType),
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                ),
              ),
              Container(
                padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: AppColors.gray100,
                  borderRadius: BorderRadius.circular(999.r),
                ),
                child: Text(
                  reportStatusLabel(r.status),
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textSecondary,
                  ),
                ),
              ),
            ],
          ),
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
                '原因：${reportReasonLabel(r.reasonCode)}',
                style: TextStyle(fontSize: 14, color: AppColors.textPrimary),
              ),
            ],
          ),
          if ((r.description ?? '').isNotEmpty) ...[
            SizedBox(height: 6.h),
            Text(
              r.description!,
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
                _formatTime(r.createTimeMs),
                style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
