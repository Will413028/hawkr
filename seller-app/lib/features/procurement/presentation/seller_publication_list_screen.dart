import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/router/app_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../data/seller_quote_model.dart';
import 'seller_publication_list_controller.dart';

class SellerPublicationListScreen extends ConsumerWidget {
  const SellerPublicationListScreen({super.key, this.now});

  @visibleForTesting
  final DateTime? now;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncRows = ref.watch(sellerPublicationListProvider);

    return Scaffold(
      backgroundColor: AppColors.bgApp,
      appBar: const HkGlassAppBar(title: Text('待回覆詢價')),
      body: SafeArea(
        child: asyncRows.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _LoadFailure(
            message: error is Exception ? '$error' : '載入詢價清單失敗',
            onRetry: () => ref.invalidate(sellerPublicationListProvider),
          ),
          data: (rows) => _PublicationList(
            rows: rows,
            now: now ?? DateTime.now(),
            onRefresh: () => ref.refresh(sellerPublicationListProvider.future),
          ),
        ),
      ),
    );
  }
}

String _summaryLine(List<SellerPublicationListItem> rows) {
  final pending = rows.where((row) => !row.hasSubmittedResponse).length;
  if (pending == 0) return '共 ${rows.length} 則，都已經回過價了。';
  return '共 ${rows.length} 則，其中 $pending 則還沒回價。';
}

class _PublicationList extends StatelessWidget {
  const _PublicationList({
    required this.rows,
    required this.now,
    required this.onRefresh,
  });

  final List<SellerPublicationListItem> rows;
  final DateTime now;
  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context) {
    return FruitRefreshIndicator(
      onRefresh: onRefresh,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (rows.isEmpty)
            const SliverFillRemaining(
              hasScrollBody: false,
              child: HkEmptyState(
                key: ValueKey('seller-publications-empty-state'),
                iconKey: ValueKey('seller-publications-empty-state-icon'),
                icon: HkIconName.document,
                message: '目前沒有等你回價的詢價',
                description: '買家發出詢價並邀請你回價時，會出現在這裡。下拉可以重新整理。',
                topPadding: 40,
              ),
            )
          else ...[
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.lg,
                HkSpace.lg,
                HkSpace.lg,
                HkSpace.sm,
              ),
              sliver: SliverToBoxAdapter(
                child: Text(_summaryLine(rows), style: AppTypography.metaMuted),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.lg,
                0,
                HkSpace.lg,
                HkSpace.xxl,
              ),
              sliver: SliverList.builder(
                itemCount: rows.length,
                itemBuilder: (_, index) => Padding(
                  padding: const EdgeInsets.only(bottom: HkSpace.md),
                  child: _PublicationCard(row: rows[index], now: now),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _PublicationCard extends StatelessWidget {
  const _PublicationCard({required this.row, required this.now});

  final SellerPublicationListItem row;
  final DateTime now;

  @override
  Widget build(BuildContext context) {
    final payload = row.sellerFacingPayload;
    final publicationId = row.publication.publicationId;
    final title = _title(payload);
    final deadline = classifyResponseDeadline(
      payload.responseDeadline,
      now: now,
    );

    return HkContentCard(
      semanticLabel: title,
      onTap: publicationId.isEmpty
          ? null
          : () => context.push(
              '${ProcurementRoutes.sellerQuote}'
              '?publicationId=${Uri.encodeComponent(publicationId)}',
            ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.sectionTitle,
                ),
              ),
              if (publicationId.isNotEmpty) ...[
                const SizedBox(width: HkSpace.sm),
                const HkIcon(
                  HkIconName.chevronRight,
                  size: 20,
                  color: AppIconColors.muted,
                ),
              ],
            ],
          ),
          const SizedBox(height: HkSpace.sm),
          Wrap(
            spacing: HkSpace.sm,
            runSpacing: HkSpace.xs,
            children: [
              if (deadline.isKnown) _DeadlineBadge(deadline: deadline),
              _ResponseStatusBadge(row: row),
              if (row.hasDraft) const _DraftBadge(),
            ],
          ),
          for (final line in _metaLines(payload)) ...[
            const SizedBox(height: HkSpace.xs),
            Text(line, style: AppTypography.metaMuted),
          ],
          if (publicationId.isEmpty) ...[
            const SizedBox(height: HkSpace.sm),
            Text(
              '這則詢價缺少識別碼，暫時打不開，請聯絡客服。',
              style: AppTypography.metaMuted.copyWith(color: AppColors.danger),
            ),
          ],
        ],
      ),
    );
  }

  static String _title(SellerFacingPayload payload) {
    final label = payload.itemLabel.trim();
    if (label.isNotEmpty) return label;
    final item = payload.item.trim();
    if (item.isNotEmpty) return item;
    return '未填品項的詢價';
  }

  static List<String> _metaLines(SellerFacingPayload payload) {
    final lines = <String>[];

    final quantity = payload.quantity.trim();
    final unit = payload.unit.trim();
    final amount = [quantity, unit].where((v) => v.isNotEmpty).join(' ');
    if (amount.isNotEmpty) lines.add('需求數量 $amount');

    final city = payload.deliveryCity.trim();
    final deliveryDate = payload.deliveryDate.trim();
    if (city.isNotEmpty) lines.add('交貨縣市 $city');
    if (deliveryDate.isNotEmpty) lines.add('希望交貨日 $deliveryDate');

    final deadline = payload.responseDeadline.trim();
    if (deadline.isNotEmpty) lines.add('回覆期限 $deadline');

    return lines;
  }
}

class _ResponseStatusBadge extends StatelessWidget {
  const _ResponseStatusBadge({required this.row});

  final SellerPublicationListItem row;

  @override
  Widget build(BuildContext context) {
    final (label, tone, icon) = switch (row.latestResponseStatusValue) {
      ProcurementSellerResponseStatus.active => (
        '已回價',
        HkStatusTone.success,
        HkIconName.check,
      ),
      ProcurementSellerResponseStatus.superseded => (
        '已被新版取代',
        HkStatusTone.neutral,
        HkIconName.swap,
      ),
      ProcurementSellerResponseStatus.withdrawn => (
        '已撤回',
        HkStatusTone.neutral,
        HkIconName.block,
      ),
      ProcurementSellerResponseStatus.expired => (
        '報價已過期',
        HkStatusTone.warning,
        HkIconName.clock,
      ),
      null when row.hasSubmittedResponse => (
        '已回價（請更新 App 看狀態）',
        HkStatusTone.info,
        HkIconName.info,
      ),
      null => ('還沒回價', HkStatusTone.info, HkIconName.edit),
    };

    return HkStatusBadge(
      key: const ValueKey('seller-publication-response-status-badge'),
      label: label,
      tone: tone,
      icon: icon,
    );
  }
}

class _DraftBadge extends StatelessWidget {
  const _DraftBadge();

  @override
  Widget build(BuildContext context) {
    return const HkStatusBadge(
      key: ValueKey('seller-publication-draft-badge'),
      label: '草稿未送出',
      tone: HkStatusTone.warning,
      icon: HkIconName.document,
    );
  }
}

class _DeadlineBadge extends StatelessWidget {
  const _DeadlineBadge({required this.deadline});

  final SellerPublicationDeadline deadline;

  @override
  Widget build(BuildContext context) {
    final (label, tone) = switch (deadline.state) {
      SellerPublicationDeadlineState.overdue => ('已過回覆期限', HkStatusTone.danger),
      SellerPublicationDeadlineState.dueToday => ('今天截止', HkStatusTone.warning),
      SellerPublicationDeadlineState.upcoming => (
        '剩 ${deadline.daysLeft} 天',
        deadline.daysLeft <= 3 ? HkStatusTone.warning : HkStatusTone.info,
      ),
      SellerPublicationDeadlineState.unknown => ('', HkStatusTone.neutral),
    };
    if (label.isEmpty) return const SizedBox.shrink();

    return HkStatusBadge(
      key: const ValueKey('seller-publication-deadline-badge'),
      label: label,
      tone: tone,
      icon: HkIconName.clock,
    );
  }
}

class _LoadFailure extends StatelessWidget {
  const _LoadFailure({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              message,
              style: AppTypography.body,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: HkSpace.lg),
            FilledButton(
              onPressed: onRetry,
              child: const HkButtonLabel('重新載入'),
            ),
          ],
        ),
      ),
    );
  }
}
