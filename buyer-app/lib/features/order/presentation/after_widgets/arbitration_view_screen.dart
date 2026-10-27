import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/common_top_bar.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../data/after_sale_repository.dart';
import '../../data/appeal_data.dart';
import 'package:hawkr_buyer/shared/widgets/fruit_refresh_indicator.dart';

/// PR #983：仲裁進度查詢頁
///
/// 後端：`GET /app/after_sales/{caseNo}/arbitration`
///
/// 顯示（state 為 CodeMeta `AfterSaleAppealState` 的 Value）：
/// - state badge（'10' 申訴中 / '20' 平台調查中 用 warning 色；'30' 已裁決用 success 色）
/// - timeline（沿用詳情頁 section 卡片 + 條目樣式）
/// - decision（state='30' 才顯示，含 outcome / refundAmount / reason / decidedAt）
class ArbitrationViewScreen extends HookConsumerWidget {
  const ArbitrationViewScreen({super.key, required this.caseNo});

  final String caseNo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final loading = useState(true);
    final error = useState<String?>(null);
    final data = useState<ArbitrationData?>(null);

    Future<void> load() async {
      loading.value = true;
      error.value = null;
      try {
        final repo = ref.read(afterSaleRepositoryProvider);
        final result = await repo.getArbitration(caseNo);
        if (!context.mounted) return;
        data.value = result;
      } catch (e) {
        if (!context.mounted) return;
        error.value = e.toString().replaceAll('Exception: ', '');
      } finally {
        if (context.mounted) loading.value = false;
      }
    }

    useEffect(() {
      load();
      return null;
    }, [caseNo]);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          _buildAppBar(context),
          Expanded(
            child: loading.value
                ? const TruckLoadingOverlay(message: '載入中...')
                : error.value != null
                ? _buildErrorState(error.value!, load)
                : data.value == null
                ? const HkEmptyState(
                    key: ValueKey('arbitration-empty-state'),
                    iconKey: ValueKey('arbitration-empty-state-icon'),
                    icon: HkIconName.emptyAfterSale,
                    message: '查無申訴紀錄',
                  )
                : FruitRefreshIndicator(
                    onRefresh: load,
                    child: SingleChildScrollView(
                      physics: const AlwaysScrollableScrollPhysics(),
                      padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 32.h),
                      child: _buildContent(data.value!),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildContent(ArbitrationData d) {
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.18),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8,
            offset: Offset(0, 3.h),
          ),
        ],
      ),
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildStateBadge(d.state),
            SizedBox(height: AppSpacing.md),
            if (d.timeline.isNotEmpty) ...[
              _buildSectionCard(_buildTimelineSection(d.timeline)),
              SizedBox(height: AppSpacing.sm),
            ],
            if (d.state == ArbitrationState.resolved && d.decision != null)
              _buildSectionCard(_buildDecisionSection(d.decision!)),
          ],
        ),
      ),
    );
  }

  Widget _buildStateBadge(String state) {
    final isResolved = state == ArbitrationState.resolved;
    final color = isResolved ? AppColors.success : AppColors.warning;
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12.h),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: color.withValues(alpha: 0.24)),
      ),
      child: Row(
        children: [
          Icon(
            isResolved
                ? AppIcons.taskAltOutlined
                : AppIcons.hourglassTopOutlined,
            color: color,
            size: 20.w,
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              ArbitrationState.label(state),
              style: AppTypography.bodyBold.copyWith(color: color),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTimelineSection(List<AppealEvent> events) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.historyToggleOffOutlined, '申訴進度'),
        SizedBox(height: AppSpacing.md),
        for (var i = 0; i < events.length; i++) ...[
          if (i > 0) SizedBox(height: AppSpacing.sm),
          _timelineRow(events[i], isLast: i == events.length - 1),
        ],
      ],
    );
  }

  Widget _timelineRow(AppealEvent e, {required bool isLast}) {
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Column(
            children: [
              Container(
                width: 10.w,
                height: 10.w,
                margin: EdgeInsets.only(top: 4.h),
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: AppColors.primary,
                ),
              ),
              if (!isLast)
                Expanded(
                  child: Container(
                    width: 2.w,
                    color: AppColors.primary.withValues(alpha: 0.3),
                  ),
                ),
            ],
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  e.label.isNotEmpty ? e.label : _eventTypeLabel(e.eventType),
                  style: AppTypography.bodyBold,
                ),
                if (e.timestamp.isNotEmpty) ...[
                  SizedBox(height: 2.h),
                  Text(
                    _formatTime(e.timestamp),
                    style: AppTypography.meta.copyWith(
                      color: AppTypography.colorDisabled,
                    ),
                  ),
                ],
                if (e.description.isNotEmpty) ...[
                  SizedBox(height: AppSpacing.xs),
                  Text(
                    e.description,
                    style: AppTypography.meta.copyWith(
                      color: AppTypography.colorPrimary.withValues(alpha: 0.82),
                    ),
                  ),
                ],
                SizedBox(height: AppSpacing.sm),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDecisionSection(PlatformDecision d) {
    final outcomeColor = switch (d.outcome) {
      DecisionOutcome.fullRefund => AppColors.success,
      DecisionOutcome.partialRefund => AppColors.warning,
      DecisionOutcome.rejectAppeal => AppColors.danger,
      _ => AppColors.primary,
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _sectionTitle(AppIcons.gavelOutlined, '平台裁決'),
        SizedBox(height: AppSpacing.md),
        Container(
          width: double.infinity,
          padding: EdgeInsets.all(AppSpacing.sm),
          decoration: BoxDecoration(
            color: AppColors.bgPrimary,
            borderRadius: BorderRadius.circular(8.r),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                DecisionOutcome.label(d.outcome),
                style: AppTypography.bodyBold.copyWith(color: outcomeColor),
              ),
              SizedBox(height: AppSpacing.xs),
              if (d.outcome == DecisionOutcome.partialRefund &&
                  d.refundAmount.isNotEmpty)
                _miniLine(
                  '退款金額',
                  'NT\$ ${d.refundAmount}',
                  color: AppColors.warning,
                ),
              if (d.reason.isNotEmpty) _miniLine('裁決說明', d.reason),
              if (d.decidedAt.isNotEmpty)
                _miniLine('裁決時間', _formatTime(d.decidedAt)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildSectionCard(Widget child) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.15),
        ),
      ),
      child: child,
    );
  }

  Widget _sectionTitle(IconData icon, String label) {
    return Row(
      children: [
        Icon(icon, size: 16.w, color: AppIconColors.defaultColor),
        SizedBox(width: AppSpacing.xs),
        Text(label, style: AppTypography.bodyBold),
      ],
    );
  }

  Widget _miniLine(String label, String value, {Color? color}) {
    return Padding(
      padding: EdgeInsets.only(top: 4.h),
      child: Text.rich(
        TextSpan(
          children: [
            TextSpan(
              text: '$label：',
              style: AppTypography.meta.copyWith(
                color: AppTypography.colorDisabled,
              ),
            ),
            TextSpan(
              text: value,
              style: AppTypography.meta.copyWith(
                color: color ?? AppTypography.colorPrimary,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildErrorState(String msg, Future<void> Function() reload) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            msg,
            style: AppTypography.body.copyWith(color: AppColors.danger),
          ),
          SizedBox(height: AppSpacing.sm),
          OutlinedButton(
            onPressed: reload,
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(0, HkControl.minTap),
            ),
            child: const HkButtonLabel('重試'),
          ),
        ],
      ),
    );
  }

  Widget _buildAppBar(BuildContext context) {
    return commonTitleTopBar(
      context,
      title: '仲裁進度',
      backIcon: AppIcons.chevronLeft,
      onBack: () => context.pop(),
    );
  }

  String _eventTypeLabel(String type) => switch (type) {
    AppealEventType.appealSubmitted => '已提交申訴',
    AppealEventType.decisionMade => '平台裁決',
    _ => type,
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
