import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../shared/widgets/common_top_bar.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../data/after_sale_model.dart';
import '../../data/after_sale_repository.dart';

/// 售後服務列表頁 — 對齊 Figma node 4661:37209
class AfterSaleListScreen extends ConsumerStatefulWidget {
  const AfterSaleListScreen({super.key});

  @override
  ConsumerState<AfterSaleListScreen> createState() =>
      _AfterSaleListScreenState();
}

class _AfterSaleListScreenState extends ConsumerState<AfterSaleListScreen> {
  List<AfterSaleCase> _cases = [];
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadCases();
  }

  Future<void> _loadCases() async {
    try {
      final result = await ref
          .read(afterSaleRepositoryProvider)
          .getAfterSaleList();
      if (mounted) {
        setState(() {
          _cases = result.records;
          _isLoading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          _buildAppBar(context),
          Expanded(
            child: _isLoading
                ? const TruckLoadingOverlay(message: '載入中...')
                : _cases.isEmpty
                ? const HkEmptyState(
                    key: ValueKey('after-sale-empty-state'),
                    iconKey: ValueKey('after-sale-empty-state-icon'),
                    icon: HkIconName.emptyAfterSale,
                    message: '暫無售後服務',
                  )
                : FruitRefreshIndicator(
                    onRefresh: _loadCases,
                    child: ListView.builder(
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 20.h),
                      itemCount: _cases.length,
                      itemBuilder: (_, i) => Padding(
                        padding: EdgeInsets.only(bottom: 16.h),
                        child: _buildCaseCard(_cases[i]),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildAppBar(BuildContext context) {
    return commonTitleTopBar(
      context,
      title: '售後服務',
      backIcon: AppIcons.chevronLeft,
      onBack: () => context.pop(),
    );
  }

  static final _infoStyle = TextStyle(
    fontSize: 16,
    fontWeight: FontWeight.w400,
    color: AppColors.textPrimary,
    height: 24 / 16,
  );

  Widget _buildCaseCard(AfterSaleCase c) {
    // 申訴 / 平台仲裁中時覆寫顯示，否則維持 raw status（與詳情頁一致）。
    final underAppeal = AfterSaleStatus.isUnderAppeal(c.appealState);
    final statusLabel = AfterSaleStatus.displayLabel(c.status, c.appealState);
    final statusColor = underAppeal
        ? AppColors.warningText
        : _statusColor(c.status);

    return GestureDetector(
      onTap: () => context.push(
        '/settings/after-sale/detail',
        extra: {'caseNo': c.caseNo},
      ),
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(
            color: AppColors.textPrimary.withValues(alpha: 0.2),
          ),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 2,
              offset: const Offset(0, 1),
            ),
          ],
        ),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Padding(
              padding: EdgeInsets.fromLTRB(24.w, 12.h, 48.w, 12.h),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 案件號 + 狀態
                  Row(
                    children: [
                      Expanded(
                        child: Text('案件：${c.caseNo}', style: _infoStyle),
                      ),
                      Container(
                        padding: EdgeInsets.symmetric(
                          horizontal: 8.w,
                          vertical: 4.h,
                        ),
                        decoration: BoxDecoration(
                          color: statusColor.withValues(alpha: 0.15),
                          borderRadius: BorderRadius.circular(999.r),
                        ),
                        child: Text(
                          statusLabel,
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            color: statusColor,
                          ),
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: 6.h),
                  // 訂單號
                  Text('訂單：${c.orderNo}', style: _infoStyle),
                  SizedBox(height: 6.h),
                  // 時間
                  Row(
                    children: [
                      Icon(
                        AppIcons.accessTime,
                        size: 20.sp,
                        color: AppIconColors.defaultColor,
                      ),
                      SizedBox(width: 6.w),
                      Text(_formatTime(c.createdAt), style: _infoStyle),
                    ],
                  ),
                  SizedBox(height: 6.h),
                  // 原因
                  Row(
                    children: [
                      Icon(
                        AppIcons.infoOutline,
                        size: 20.sp,
                        color: AppIconColors.defaultColor,
                      ),
                      SizedBox(width: 6.w),
                      Text(AfterSaleReason.label(c.reason), style: _infoStyle),
                    ],
                  ),
                  if (c.refundableAmount.isNotEmpty &&
                      c.refundableAmount != '0') ...[
                    SizedBox(height: 6.h),
                    Text(
                      '可退金額 NT\$ ${c.refundableAmount}',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: AppColors.primary,
                      ),
                    ),
                  ],
                  // PR #983：status=20 顯示寄回截止日（後端 ISO 8601）
                  if (c.status == AfterSaleStatus.awaitingReturn &&
                      c.shipBackDeadlineAt.isNotEmpty) ...[
                    SizedBox(height: 6.h),
                    Text(
                      '寄回截止：${_formatTime(c.shipBackDeadlineAt)}',
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w400,
                        color: AppColors.textSecondary,
                        height: 20 / 13,
                      ),
                    ),
                  ],
                  SizedBox(height: 8.h),
                  // 按鈕
                  Row(
                    children: [
                      Container(
                        padding: EdgeInsets.symmetric(
                          horizontal: 16.w,
                          vertical: 4.h,
                        ),
                        decoration: BoxDecoration(
                          color: AppColors.primary,
                          borderRadius: BorderRadius.circular(999.r),
                        ),
                        child: Text(
                          '細節',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: AppColors.bgCard,
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            // Red badge
            Positioned(
              top: -1,
              right: 17.w,
              child: Container(
                width: 40.w,
                height: 42.h,
                decoration: BoxDecoration(
                  color: AppColors.danger,
                  borderRadius: const BorderRadius.only(
                    bottomLeft: Radius.circular(32),
                    bottomRight: Radius.circular(32),
                  ),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 3,
                      offset: const Offset(0, 1),
                    ),
                  ],
                ),
                child: Center(
                  child: Icon(
                    AppIcons.campaign,
                    size: 20.sp,
                    color: AppIconColors.onBrand,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Color _statusColor(String status) => switch (status) {
    // 後端 #1241：REOPENED(65) 已淘汰（被拒後改走平台申訴）
    '0' => AppColors.warningText,
    '20' || '25' => AppColors.info,
    '33' => AppColors.textSecondary, // PR #983 待寄回逾時自動關案（灰）
    '40' => AppColors.primary,
    '50' => AppColors.textSecondary,
    '60' => AppColors.danger,
    _ => AppColors.textSecondary,
  };

  String _formatTime(String raw) {
    if (raw.isEmpty) return '';
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}
