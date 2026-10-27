import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/codemeta/shipment_anomaly_presentation.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/empty_state_view.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/shipment_anomaly_model.dart';
import '../data/shipment_anomaly_providers.dart';

/// 配送異常 Dashboard（賣家端）。
///
/// 路徑：`/shipment-anomalies`
/// 對應 backend §5.21 — Gateway `GET /api/v1/app/shipments/anomalies(/stats)`。
class ShipmentAnomalyDashboardScreen extends ConsumerWidget {
  const ShipmentAnomalyDashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(shipmentAnomalyDashboardControllerProvider);

    return Scaffold(
      backgroundColor: AppColors.bgApp,
      appBar: const HkGlassAppBar(title: Text('配送異常')),
      body: asyncState.when(
        loading: () => const TruckLoadingOverlay(message: '載入中...'),
        error: (error, _) => _errorView(context, ref, error),
        data: (state) => _content(context, ref, state),
      ),
    );
  }

  Widget _errorView(BuildContext context, WidgetRef ref, Object error) {
    return Center(
      child: EmptyStateView(
        visual: Icon(
          AppIcons.errorOutline,
          size: 56.sp,
          color: AppIconColors.muted,
        ),
        message: error.toString(),
        spacing: 12.h,
        messageStyle: TextStyle(fontSize: 14.sp, color: AppColors.gray1_80),
      ),
    );
  }

  Widget _content(
    BuildContext context,
    WidgetRef ref,
    ShipmentAnomalyDashboardState state,
  ) {
    return FruitRefreshIndicator(
      onRefresh: () => ref
          .read(shipmentAnomalyDashboardControllerProvider.notifier)
          .refresh(),
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          SliverPadding(
            padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 8.h),
            sliver: SliverToBoxAdapter(child: _statsRow(state.stats)),
          ),
          SliverPadding(
            padding: EdgeInsets.fromLTRB(16.w, 4.h, 16.w, 8.h),
            sliver: SliverToBoxAdapter(
              child: _filterBar(context, ref, state.filter),
            ),
          ),
          if (state.records.isEmpty)
            SliverFillRemaining(
              hasScrollBody: false,
              child: const HkEmptyState(
                key: ValueKey('shipment-anomalies-empty-state'),
                iconKey: ValueKey('shipment-anomalies-empty-state-icon'),
                icon: HkIconName.box,
                message: '目前沒有配送異常',
                description: '需要處理的配送問題會集中顯示在這裡。',
                topPadding: 40,
              ),
            )
          else
            SliverPadding(
              padding: EdgeInsets.fromLTRB(16.w, 4.h, 16.w, 8.h),
              sliver: SliverList.builder(
                itemCount: _groupedItems(state.records).length,
                itemBuilder: (_, i) {
                  final item = _groupedItems(state.records)[i];
                  if (item is _GroupHeader) {
                    return Padding(
                      padding: EdgeInsets.only(
                        top: i == 0 ? 0 : 16.h,
                        bottom: 8.h,
                      ),
                      child: Text(
                        item.title,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textPrimary,
                        ),
                      ),
                    );
                  }
                  final anomaly = (item as _AnomalyRow).anomaly;
                  return Padding(
                    padding: EdgeInsets.only(bottom: 12.h),
                    child: _AnomalyCard(
                      anomaly: anomaly,
                      onAcknowledge: () =>
                          _onAcknowledge(context, ref, anomaly),
                      onIgnore: () => _onIgnore(context, ref, anomaly),
                    ),
                  );
                },
              ),
            ),
          // 議題 4：RETURN_STUCK 在 aftersale_cases 不在 shipments，dashboard 不列出但 FCM 會推
          SliverPadding(
            padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 20.h),
            sliver: SliverToBoxAdapter(
              child: Text(
                '退貨流程的「卡關通知」請至「售後管理」中查看（推播有送，但本頁僅列正向物流異常）。',
                style: TextStyle(
                  fontSize: 12.sp,
                  color: AppColors.gray1_50,
                  height: 1.5,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  // ── 分組（依 anomaly_type） ───────────────────────────────────────────
  List<_Item> _groupedItems(List<ShipmentAnomaly> records) {
    final byType = <String, List<ShipmentAnomaly>>{};
    for (final r in records) {
      byType.putIfAbsent(r.anomalyType, () => []).add(r);
    }
    final out = <_Item>[];
    final keys = byType.keys.toList()..sort();
    for (final k in keys) {
      final label = anomalyTypeLabelOf(
        k,
        backendLabel: byType[k]!.first.anomalyTypeLabel,
      );
      out.add(_GroupHeader('$label (${byType[k]!.length})'));
      for (final a in byType[k]!) {
        out.add(_AnomalyRow(a));
      }
    }
    return out;
  }

  // ── 統計卡 ───────────────────────────────────────────────────────────
  Widget _statsRow(ShipmentAnomalyStats stats) {
    return Row(
      children: [
        Expanded(
          child: _statTile(
            label: '高',
            value: stats.highCount,
            color: AppColors.error,
          ),
        ),
        SizedBox(width: 12.w),
        Expanded(
          child: _statTile(
            label: '中',
            value: stats.mediumCount,
            color: AppColors.warning,
          ),
        ),
        SizedBox(width: 12.w),
        Expanded(
          child: _statTile(
            label: '低',
            value: stats.lowCount,
            color: AppColors.gray1_80,
          ),
        ),
      ],
    );
  }

  Widget _statTile({
    required String label,
    required int value,
    required Color color,
  }) {
    return Container(
      constraints: BoxConstraints(minHeight: 80.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Row(
            children: [
              Container(
                width: 8.w,
                height: 8.w,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              ),
              SizedBox(width: 6.w),
              Text(
                label,
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w400,
                  color: AppColors.textPrimary,
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            '$value',
            style: TextStyle(
              fontSize: 24.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
              height: 1.2,
            ),
          ),
        ],
      ),
    );
  }

  // ── Filter bar（severity / type / showAcknowledged） ────────────────
  Widget _filterBar(
    BuildContext context,
    WidgetRef ref,
    ShipmentAnomalyFilter filter,
  ) {
    final controller = ref.read(
      shipmentAnomalyDashboardControllerProvider.notifier,
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              _FilterChip(
                label: '全部嚴重度',
                selected: filter.severity == null,
                onTap: () {
                  controller.updateFilter(filter.copyWith(clearSeverity: true));
                },
              ),
              SizedBox(width: 8.w),
              _FilterChip(
                label: '高',
                selected: filter.severity == ShipmentAnomalySeverity.high,
                activeColor: AppColors.error,
                onTap: () {
                  controller.updateFilter(
                    filter.copyWith(severity: ShipmentAnomalySeverity.high),
                  );
                },
              ),
              SizedBox(width: 8.w),
              _FilterChip(
                label: '中',
                selected: filter.severity == ShipmentAnomalySeverity.medium,
                activeColor: AppColors.warning,
                onTap: () {
                  controller.updateFilter(
                    filter.copyWith(severity: ShipmentAnomalySeverity.medium),
                  );
                },
              ),
              SizedBox(width: 8.w),
              _FilterChip(
                label: '低',
                selected: filter.severity == ShipmentAnomalySeverity.low,
                onTap: () {
                  controller.updateFilter(
                    filter.copyWith(severity: ShipmentAnomalySeverity.low),
                  );
                },
              ),
            ],
          ),
        ),
        SizedBox(height: 8.h),
        Row(
          children: [
            Expanded(
              child: Text(
                '顯示已處理 / 已忽略',
                style: TextStyle(fontSize: 14.sp, color: AppColors.textPrimary),
              ),
            ),
            Switch.adaptive(
              value: filter.showAcknowledged,
              activeThumbColor: AppColors.primary,
              onChanged: (v) {
                controller.updateFilter(filter.copyWith(showAcknowledged: v));
              },
            ),
          ],
        ),
      ],
    );
  }

  // ── Actions ─────────────────────────────────────────────────────────
  Future<void> _onAcknowledge(
    BuildContext context,
    WidgetRef ref,
    ShipmentAnomaly anomaly,
  ) async {
    final ok = await ref
        .read(shipmentAnomalyDashboardControllerProvider.notifier)
        .acknowledge(anomaly.shipmentId, anomaly.anomalyType);
    if (!context.mounted) return;
    if (ok) {
      FloatingToast.show(context, '已標記為處理中');
    } else {
      FloatingToast.error(context, '操作失敗，請稍後再試');
    }
  }

  Future<void> _onIgnore(
    BuildContext context,
    WidgetRef ref,
    ShipmentAnomaly anomaly,
  ) async {
    final ok = await ref
        .read(shipmentAnomalyDashboardControllerProvider.notifier)
        .markIgnored(anomaly.shipmentId, anomaly.anomalyType);
    if (!context.mounted) return;
    if (ok) {
      FloatingToast.show(context, '已忽略');
    } else {
      FloatingToast.error(context, '操作失敗，請稍後再試');
    }
  }
}

// ============================================================
// 分組項目
// ============================================================

sealed class _Item {
  const _Item();
}

class _GroupHeader extends _Item {
  final String title;
  const _GroupHeader(this.title);
}

class _AnomalyRow extends _Item {
  final ShipmentAnomaly anomaly;
  const _AnomalyRow(this.anomaly);
}

// ============================================================
// Anomaly card
// ============================================================

class _AnomalyCard extends StatelessWidget {
  const _AnomalyCard({
    required this.anomaly,
    required this.onAcknowledge,
    required this.onIgnore,
  });

  final ShipmentAnomaly anomaly;
  final VoidCallback onAcknowledge;
  final VoidCallback onIgnore;

  Color get _severityColor {
    switch (anomaly.severity) {
      case ShipmentAnomalySeverity.high:
        return AppColors.error;
      case ShipmentAnomalySeverity.medium:
        return AppColors.warning;
      default:
        return AppColors.gray1_80;
    }
  }

  bool get _alreadyActed => anomaly.ackAction.isNotEmpty;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      padding: EdgeInsets.all(12.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: _severityColor.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(6.r),
                  border: Border.all(color: _severityColor, width: 1),
                ),
                child: Text(
                  anomalySeverityLabelOf(anomaly.severity),
                  style: TextStyle(
                    fontSize: 12.sp,
                    fontWeight: FontWeight.w700,
                    color: _severityColor,
                  ),
                ),
              ),
              SizedBox(width: 8.w),
              Expanded(
                child: Text(
                  '訂單 ${anomaly.orderNo}',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
          SizedBox(height: 8.h),
          Text(
            anomalyTypeLabelOf(
              anomaly.anomalyType,
              backendLabel: anomaly.anomalyTypeLabel,
            ),
            style: TextStyle(fontSize: 14.sp, color: AppColors.textEmphasis),
          ),
          if (anomaly.detectedAt.isNotEmpty) ...[
            SizedBox(height: 4.h),
            Text(
              '偵測時間：${anomaly.detectedAt}',
              style: TextStyle(fontSize: 12.sp, color: AppColors.gray1_50),
            ),
          ],
          if (_alreadyActed) ...[
            SizedBox(height: 4.h),
            Text(
              anomaly.ackAction == ShipmentAnomalyAckAction.acknowledged
                  ? '已標記處理中'
                  : '已忽略',
              style: TextStyle(
                fontSize: 12.sp,
                color: AppColors.primary,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
          SizedBox(height: 10.h),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              OutlinedButton(
                onPressed: _alreadyActed ? null : onIgnore,
                style: OutlinedButton.styleFrom(
                  foregroundColor: AppColors.gray1_80,
                  side: BorderSide(color: AppColors.gray1_20),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                  padding: EdgeInsets.symmetric(
                    horizontal: 16.w,
                    vertical: 8.h,
                  ),
                  textStyle: AppTypography.meta,
                ),
                child: const HkButtonLabel('已忽略'),
              ),
              SizedBox(width: 8.w),
              ElevatedButton(
                onPressed: _alreadyActed ? null : onAcknowledge,
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  foregroundColor: AppColors.bgCard,
                  disabledBackgroundColor: AppColors.disabled,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                  padding: EdgeInsets.symmetric(
                    horizontal: 16.w,
                    vertical: 8.h,
                  ),
                  textStyle: AppTypography.meta,
                ),
                child: const HkButtonLabel('已處理'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _FilterChip extends StatelessWidget {
  const _FilterChip({
    required this.label,
    required this.selected,
    required this.onTap,
    this.activeColor = AppColors.primary,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;
  final Color activeColor;

  @override
  Widget build(BuildContext context) {
    final foreground = selected ? AppColors.bgCard : AppTypography.colorPrimary;
    final borderColor = selected ? activeColor : AppColors.gray1_20;
    return Material(
      color: selected ? activeColor : AppColors.bgCard,
      borderRadius: BorderRadius.circular(999.r),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(999.r),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOut,
          padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(999.r),
            border: Border.all(color: borderColor, width: 1),
            boxShadow: selected
                ? const [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 6,
                      offset: Offset(0, 2),
                    ),
                  ]
                : null,
          ),
          child: Text(
            label,
            style: AppTypography.meta.copyWith(
              color: foreground,
              fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              height: 1.2,
            ),
          ),
        ),
      ),
    );
  }
}
