import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../l10n/l10n.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../seller/data/account_scope.dart';
import '../data/earnings.dart';
import 'earnings_entry_row.dart';

/// 錢包的「入帳紀錄」（`ListMyEarnings`，新到舊）：平台轉給店家的款項、退款時收回的款項與補差額。
/// 先顯示第一頁，按「載入更多」讀下一頁；第一頁讀取失敗顯示錯誤與重試，下一頁失敗時已顯示的列保留。沒有登入時不顯示。
class WalletEarningsSection extends ConsumerWidget {
  const WalletEarningsSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    if (accountId == null) return const SizedBox.shrink();
    final provider = myEarningsProvider(accountId);
    final ledger = ref.watch(provider);
    return Column(
      key: const ValueKey('wallet-earnings'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(context.l10n.earningsTitle, style: AppTypography.sectionTitle),
        const SizedBox(height: HkSpace.md),
        HkContentCard(
          child: switch (ledger) {
            AsyncValue(:final value?) => _Ledger(
              ledger: value,
              loadMore: ref.read(provider.notifier).loadMore,
            ),
            AsyncValue(hasError: true) => _Retry(
              message: context.l10n.earningsLoadFailed,
              onRetry: () => ref.invalidate(provider),
            ),
            _ => const Padding(
              padding: EdgeInsets.symmetric(vertical: HkSpace.md),
              child: Center(child: CircularProgressIndicator()),
            ),
          },
        ),
      ],
    );
  }
}

class _Ledger extends StatelessWidget {
  const _Ledger({required this.ledger, required this.loadMore});

  final EarningsLedger ledger;
  final Future<void> Function() loadMore;

  @override
  Widget build(BuildContext context) {
    if (ledger.entries.isEmpty) {
      return Text(
        context.l10n.earningsEmpty,
        style: AppTypography.body.copyWith(color: AppColors.textSecondary),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final (i, entry) in ledger.entries.indexed) ...[
          if (i > 0) const Divider(height: HkSpace.lg),
          EarningsEntryRow(entry: entry, showOrderNo: true),
        ],
        if (ledger.loadMoreError != null)
          _Retry(message: context.l10n.earningsLoadFailed, onRetry: loadMore)
        else if (ledger.hasMore)
          Padding(
            padding: const EdgeInsets.only(top: HkSpace.md),
            child: TextButton(
              onPressed: ledger.loadingMore ? null : loadMore,
              child: HkButtonLabel(context.l10n.commonLoadMore),
            ),
          ),
      ],
    );
  }
}

class _Retry extends StatelessWidget {
  const _Retry({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: HkSpace.sm),
      child: Column(
        children: [
          Text(message, textAlign: TextAlign.center),
          TextButton(
            onPressed: onRetry,
            child: HkButtonLabel(context.l10n.commonRetry),
          ),
        ],
      ),
    );
  }
}
