import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../data/coupon_model.dart';
import '../data/coupon_repository.dart';
import 'coupon_card_widget.dart';
import 'widgets/date_formatter.dart';

/// 優惠券唯讀詳情頁。
///
/// 後端未提供「更新優惠券模板」端點（`/app/coupon_templates/{id}` 僅支援 GET），
/// 因此本頁為唯讀：顯示券內容與已領取 / 已使用統計，不提供就地編輯。
/// 如需調整內容，賣家應作廢舊券後重新建立。
class CouponDetailScreen extends HookConsumerWidget {
  const CouponDetailScreen({super.key, required this.coupon});

  final Coupon coupon;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // 先用列表帶入的 coupon 立即渲染，同時抓詳情補上已領取 / 已使用統計。
    final detailFuture = useMemoized(
      () =>
          ref.read(couponRepositoryProvider).getCouponDetail(coupon.templateId),
      [coupon.templateId],
    );
    final snapshot = useFuture(detailFuture);
    final detail = snapshot.data;

    // 詳情回來後以 template 為準（欄位較完整），否則沿用列表資料。
    final c = detail?.template ?? coupon;

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: const HkGlassAppBar(title: Text('優惠券詳情')),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(20.w, 8.h, 20.w, 32.h),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── 券卡（唯讀，沿用列表卡片外觀）──
              CouponCardWidget(coupon: c),
              SizedBox(height: 24.h),

              // ── 領用統計 ──
              Row(
                children: [
                  _StatCard(
                    label: '發行上限',
                    value: _countText(c.totalIssueLimit),
                  ),
                  SizedBox(width: 12.w),
                  _StatCard(
                    label: '已領取',
                    value: detail == null ? '—' : '${detail.reservedCount}',
                  ),
                  SizedBox(width: 12.w),
                  _StatCard(
                    label: '已使用',
                    value: detail == null ? '—' : '${detail.usedCount}',
                  ),
                ],
              ),
              SizedBox(height: 20.h),

              // ── 明細 ──
              _InfoPanel(coupon: c),
              SizedBox(height: 20.h),

              // ── 唯讀說明 ──
              Container(
                width: double.infinity,
                padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(12.r),
                  border: Border.all(
                    color: AppColors.ink300.withValues(alpha: 0.2),
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      AppIcons.infoOutline,
                      size: 18.sp,
                      color: AppIconColors.secondary,
                    ),
                    SizedBox(width: 8.w),
                    Expanded(
                      child: Text(
                        '優惠券建立後無法修改內容。如需調整，請刪除此券後重新建立。',
                        style: TextStyle(
                          fontSize: 14.sp,
                          height: 1.5,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// 數量欄位：空 / '0' / 無法解析時視為「不限」。
  static String _countText(String? raw) {
    final n = int.tryParse(raw ?? '');
    if (n == null || n <= 0) return '不限';
    return '$n';
  }
}

/// 領用統計卡片（等寬三欄）。
class _StatCard extends StatelessWidget {
  const _StatCard({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Container(
        padding: EdgeInsets.symmetric(vertical: 16.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(12.r),
          border: Border.all(color: const Color(0x2242493F)),
        ),
        child: Column(
          children: [
            Text(
              value,
              style: TextStyle(
                fontSize: 20.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.brandPrimaryStrong,
              ),
            ),
            SizedBox(height: 4.h),
            Text(
              label,
              style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            ),
          ],
        ),
      ),
    );
  }
}

/// 優惠券明細面板。
class _InfoPanel extends StatelessWidget {
  const _InfoPanel({required this.coupon});

  final Coupon coupon;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: const Color(0x2242493F)),
      ),
      child: Column(
        children: [
          _InfoRow(
            label: '優惠類型',
            value: CodeMetaDisplay.couponType(coupon.type),
          ),
          _InfoRow(label: '折抵內容', value: _discountText(coupon)),
          _InfoRow(label: '使用門檻', value: _thresholdText(coupon)),
          _InfoRow(label: '有效期間', value: _periodText(coupon)),
          _InfoRow(label: '每人限領', value: _perAccountText(coupon)),
          _InfoRow(
            label: '狀態',
            value: _statusText(coupon.status),
            isLast: true,
          ),
        ],
      ),
    );
  }

  static String _discountText(Coupon c) {
    switch (CodeMetaDisplay.couponTypeMeta(c.type)) {
      case CouponType.cash:
        return '折抵 \$${c.discountAmount ?? '0'}';
      case CouponType.percent:
        final pct = c.discountPercent ?? '0';
        final max = c.maxDiscountAmount;
        return (max != null && max.isNotEmpty && max != '0')
            ? '$pct% 折抵（最高折 \$$max）'
            : '$pct% 折抵';
      case null:
        return '—';
    }
  }

  static String _thresholdText(Coupon c) {
    final t = c.thresholdAmount;
    if (t == null || t.isEmpty || t == '0') return '無門檻';
    return '消費滿 \$$t';
  }

  static String _periodText(Coupon c) {
    final start = DateFormatter.formatToYYYYMMDD(c.startAt);
    final end = DateFormatter.formatToYYYYMMDD(c.endAt);
    if (start.isEmpty && end.isEmpty) return '—';
    if (start.isEmpty) return '至 $end';
    if (end.isEmpty) return '$start 起';
    return '$start ~ $end';
  }

  static String _perAccountText(Coupon c) {
    final n = int.tryParse(c.usageLimitPerAccount ?? '');
    if (n == null || n <= 0) return '不限';
    return '$n 張';
  }

  static String _statusText(String status) {
    return switch (status.toUpperCase()) {
      'UNUSED' => '進行中',
      'LOCKED' => '已鎖定',
      'USED' => '已使用',
      'EXPIRED' => '已過期',
      'REVOKED' => '已收回',
      'ANNUL' => '已作廢',
      _ => status.isEmpty ? '—' : status,
    };
  }
}

/// 明細單列（label 左、value 右）。
class _InfoRow extends StatelessWidget {
  const _InfoRow({
    required this.label,
    required this.value,
    this.isLast = false,
  });

  final String label;
  final String value;
  final bool isLast;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(vertical: 16.h),
      decoration: BoxDecoration(
        border: isLast
            ? null
            : const Border(bottom: BorderSide(color: Color(0x1142493F))),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: TextStyle(fontSize: 16.sp, color: AppColors.textSecondary),
          ),
          SizedBox(width: 16.w),
          Expanded(
            child: Text(
              value,
              textAlign: TextAlign.right,
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.w500,
                color: AppColors.textPrimary,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
