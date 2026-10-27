import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_adaptive_page.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_page_section.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/account_scope.dart';
import '../data/measure_display.dart';
import '../data/measures.dart';

const _emptyTitle = '目前沒有措施紀錄';
const _emptyDescription = '保持良好營運狀況，這裡會維持空空如也。';
const _refreshFailedNotice = '重新整理失敗，請稍後再試';

/// 離開措施與申覆列表：有上一頁就返回；列表是唯一一頁（從表單 `go` 回來、或網址直接開啟）時改去設定分頁，
/// 也就是這頁常駐入口所在的位置（和 profile 等沒有上一頁時 `go` 到主分頁的做法一致）。
void leaveAppealList(BuildContext context) {
  if (context.canPop()) {
    context.pop();
  } else {
    context.go(MainRoutes.settings);
  }
}

/// 措施與申覆列表（帳號層級，不需要店家；46 號設計書「賣家能力權限層」§7.2）。
///
/// 讀 `ListMyMeasures` 的全部分頁，分成「需要處理」（可以申覆，或停權還在生效）與「歷史」。能不能申覆讀
/// `can_appeal`、措施的狀態讀 `state`，都是伺服器判斷，這裡不自己推導。申覆是全螢幕表單
/// （[SellerRoutes.measureRequest]，以措施 id 開啟）。重讀失敗但有上一次成功的列表時繼續顯示列表。
class SellerAppealScreen extends ConsumerWidget {
  const SellerAppealScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    final measures = ref.watch(currentMyMeasuresProvider(allMeasureStates));
    final list = measures?.value;

    Future<void> refresh() async {
      if (accountId == null) return;
      try {
        final provider = myMeasuresProvider(accountId, allMeasureStates);
        ref.invalidate(provider);
        await ref.read(provider.future);
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, _refreshFailedNotice);
      }
    }

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: const Text('措施與申覆'),
        onBackPressed: () => leaveAppealList(context),
      ),
      body: HkAdaptivePage(
        child: list != null
            ? FruitRefreshIndicator(
                onRefresh: refresh,
                child: _MeasureList(measures: list),
              )
            // 重試中（error 狀態被 invalidate 後是讀取中）顯示讀取中，不繼續停在錯誤面板。
            : measures != null && measures.hasError && !measures.isLoading
            ? ListView(
                padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
                children: [
                  HkAsyncStatePanel(
                    kind: HkAsyncStateKind.error,
                    title: measuresLoadFailedTitle,
                    description: measuresLoadFailedDescription,
                    actionLabel: '重試',
                    onAction: () => ref.invalidate(
                      myMeasuresProvider(accountId!, allMeasureStates),
                    ),
                  ),
                ],
              )
            : const TruckLoadingOverlay(),
      ),
    );
  }
}

class _MeasureList extends StatelessWidget {
  const _MeasureList({required this.measures});

  final List<Measure> measures;

  @override
  Widget build(BuildContext context) {
    final groups = groupMeasures(measures);
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
      children: [
        if (measures.isEmpty)
          const HkAsyncStatePanel(
            kind: HkAsyncStateKind.empty,
            title: _emptyTitle,
            description: _emptyDescription,
          ),
        if (groups.attention.isNotEmpty)
          _Section(title: '需要處理', measures: groups.attention),
        if (groups.attention.isNotEmpty && groups.history.isNotEmpty)
          const SizedBox(height: HkSpace.section),
        if (groups.history.isNotEmpty)
          _Section(title: '歷史', measures: groups.history),
      ],
    );
  }
}

class _Section extends StatelessWidget {
  const _Section({required this.title, required this.measures});

  final String title;
  final List<Measure> measures;

  @override
  Widget build(BuildContext context) {
    return HkPageSection(
      title: title,
      child: Column(
        children: [
          for (var i = 0; i < measures.length; i++) ...[
            if (i > 0) const SizedBox(height: HkSpace.md),
            MeasureCard(
              measure: measures[i],
              onAppeal: () =>
                  context.push(SellerRoutes.measureRequestFor(measures[i].id)),
              onRemediate: () => context.push(
                SellerRoutes.measureRemediationFor(measures[i].id),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// 一筆措施：依據、嚴重程度與措施、措施時間、平台的說明、措施目前的影響、請求狀態與說明。
class MeasureCard extends StatelessWidget {
  const MeasureCard({
    super.key,
    required this.measure,
    required this.onAppeal,
    required this.onRemediate,
  });

  final Measure measure;
  final VoidCallback onAppeal;

  /// 「已改善，請求解除」（RISK 依據、生效中的措施才有；能不能按讀 `can_request_remediation`）。
  final VoidCallback onRemediate;

  @override
  Widget build(BuildContext context) {
    final l = context.uiLocale;
    final meta = measureMeta(measure, l);
    final effect = measureEffect(measure, l);
    final chip = requestChip(measure, l);
    final stateBadge = stateChip(measure, l);
    final appeal = appealAction(measure);
    final appealNote = appealNoteText(measure, l);
    final note = requestNote(measure, l);
    final remediation = remediationAction(measure);
    final decided = formatTaipeiTimestamp(
      measure.hasCreateTime() ? measure.createTime : null,
    );
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  measureTitle(measure, l),
                  style: AppTypography.sectionTitle,
                ),
              ),
              for (final badge in [?stateBadge, ?chip]) ...[
                const SizedBox(width: HkSpace.sm),
                HkStatusBadge(
                  label: badge.label,
                  tone: switch (badge.tone) {
                    MeasureChipTone.success => HkStatusTone.success,
                    MeasureChipTone.danger => HkStatusTone.danger,
                    MeasureChipTone.warning => HkStatusTone.warning,
                    MeasureChipTone.neutral => HkStatusTone.neutral,
                  },
                ),
              ],
            ],
          ),
          if (meta != null) ...[
            const SizedBox(height: HkSpace.xs),
            Text(
              meta,
              style: AppTypography.caption.copyWith(color: AppColors.textMuted),
            ),
          ],
          if (decided.isNotEmpty) ...[
            const SizedBox(height: HkSpace.xs),
            Text(
              '措施時間：$decided',
              style: AppTypography.caption.copyWith(color: AppColors.textMuted),
            ),
          ],
          if (measure.statement.isNotEmpty) ...[
            const SizedBox(height: HkSpace.md),
            Text(measure.statement, style: AppTypography.meta),
          ],
          if (effect != null) ...[
            const SizedBox(height: HkSpace.md),
            Text(
              effect.text,
              key: const ValueKey('measure-effect'),
              style: AppTypography.meta.copyWith(
                color: effect.active
                    ? AppColors.danger
                    : AppColors.textSecondary,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
          if (note != null) ...[
            const SizedBox(height: HkSpace.sm),
            Text(
              note,
              style: AppTypography.caption.copyWith(color: AppColors.textMuted),
            ),
          ],
          // 申覆入口一律顯示（與 website 相同）：不能申覆時反灰並附伺服器給的原因。
          const SizedBox(height: HkSpace.md),
          Align(
            alignment: Alignment.centerRight,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                if (appealNote != null) ...[
                  Text(
                    appealNote,
                    key: const ValueKey('measure-appeal-note'),
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textMuted,
                    ),
                  ),
                  const SizedBox(height: HkSpace.xs),
                ],
                OutlinedButton(
                  key: const ValueKey('measure-appeal-button'),
                  onPressed: appeal.enabled ? onAppeal : null,
                  child: const HkButtonLabel('提出申覆'),
                ),
              ],
            ),
          ),
          if (remediation != null) ...[
            const SizedBox(height: HkSpace.md),
            Align(
              alignment: Alignment.centerRight,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  OutlinedButton(
                    key: const ValueKey('measure-remediation-button'),
                    onPressed: remediation.enabled ? onRemediate : null,
                    child: const HkButtonLabel('已改善，請求解除'),
                  ),
                  if (remediation.reason != null) ...[
                    const SizedBox(height: HkSpace.xs),
                    Text(
                      remediation.reason!,
                      key: const ValueKey('measure-remediation-reason'),
                      style: AppTypography.caption.copyWith(
                        color: AppColors.textMuted,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
