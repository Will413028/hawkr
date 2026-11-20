import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/hk_adaptive_page.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/account_scope.dart';
import '../data/inquiries.dart';
import '../data/inquiry_display.dart';

const _refreshFailedNotice = '重新整理失敗，請稍後再試';

HkStatusTone inquiryTone(InquiryTone tone) => switch (tone) {
  InquiryTone.success => HkStatusTone.success,
  InquiryTone.danger => HkStatusTone.danger,
  InquiryTone.warning => HkStatusTone.warning,
  InquiryTone.neutral => HkStatusTone.neutral,
};

/// 離開平台詢問頁：有上一頁就返回，否則（網址直接開啟）去設定分頁，也就是常駐入口所在的位置。
void leaveInquiries(BuildContext context) {
  if (context.canPop()) {
    context.pop();
  } else {
    context.go(MainRoutes.settings);
  }
}

/// 平台詢問列表（帳號層級，不需要店家）：讀 `ListMyInquiries` 的全部分頁，待回覆的在前，其餘（已回覆、已撤回）在後。
/// 能不能回覆讀 `can_reply`，逾期以期限判斷，都不自己推導。重讀失敗但有上一次成功的列表時繼續顯示列表。
class SellerInquiriesScreen extends ConsumerWidget {
  const SellerInquiriesScreen({super.key, this.clock = DateTime.now});

  final DateTime Function() clock;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    final inquiries = ref.watch(currentMyInquiriesProvider);
    final list = inquiries?.value;

    Future<void> refresh() async {
      if (accountId == null) return;
      try {
        final provider = myInquiriesProvider(accountId);
        ref.invalidate(provider);
        ref.invalidate(pendingInquiryCountProvider(accountId));
        await ref.read(provider.future);
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, _refreshFailedNotice);
      }
    }

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: const Text(inquiriesTitle),
        onBackPressed: () => leaveInquiries(context),
      ),
      body: HkAdaptivePage(
        child: list != null
            ? FruitRefreshIndicator(
                onRefresh: refresh,
                child: _InquiryList(inquiries: list, clock: clock),
              )
            : inquiries != null && inquiries.hasError && !inquiries.isLoading
            ? ListView(
                padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
                children: [
                  HkAsyncStatePanel(
                    kind: HkAsyncStateKind.error,
                    title: inquiriesLoadFailedTitle,
                    description: inquiriesLoadFailedDescription,
                    actionLabel: '重試',
                    onAction: () =>
                        ref.invalidate(myInquiriesProvider(accountId!)),
                  ),
                ],
              )
            : const TruckLoadingOverlay(),
      ),
    );
  }
}

class _InquiryList extends StatelessWidget {
  const _InquiryList({required this.inquiries, required this.clock});

  final List<Inquiry> inquiries;
  final DateTime Function() clock;

  @override
  Widget build(BuildContext context) {
    final open = [
      for (final i in inquiries)
        if (i.state == InquiryState.INQUIRY_STATE_OPEN) i,
    ];
    final rest = [
      for (final i in inquiries)
        if (i.state != InquiryState.INQUIRY_STATE_OPEN) i,
    ];
    final ordered = [...open, ...rest];
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
      children: [
        if (ordered.isEmpty)
          const HkAsyncStatePanel(
            kind: HkAsyncStateKind.empty,
            title: inquiriesEmptyTitle,
            description: inquiriesEmptyDescription,
          ),
        for (var i = 0; i < ordered.length; i++) ...[
          if (i > 0) const SizedBox(height: HkSpace.md),
          InquiryCard(
            inquiry: ordered[i],
            now: clock(),
            onTap: () =>
                context.push(SellerRoutes.inquiryDetailFor(ordered[i].id)),
          ),
        ],
      ],
    );
  }
}

/// 一則詢問：種類與時間、狀態、平台的說明與期限。
class InquiryCard extends StatelessWidget {
  const InquiryCard({
    super.key,
    required this.inquiry,
    required this.now,
    required this.onTap,
  });

  final Inquiry inquiry;
  final DateTime now;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final chip = inquiryStateChip(inquiry, now, context.uiLocale);
    final deadline = inquiryDeadlineText(inquiry);
    final overdue = isInquiryOverdue(inquiry, now);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(HkRadius.md),
      child: HkContentCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Text(
                    inquiryMeta(inquiry, context.uiLocale),
                    style: AppTypography.sectionTitle,
                  ),
                ),
                if (chip != null) ...[
                  const SizedBox(width: HkSpace.sm),
                  HkStatusBadge(
                    label: chip.label,
                    tone: inquiryTone(chip.tone),
                  ),
                ],
              ],
            ),
            if (inquiry.message.isNotEmpty) ...[
              const SizedBox(height: HkSpace.md),
              Text(
                inquiry.message,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.meta,
              ),
            ],
            if (deadline != null) ...[
              const SizedBox(height: HkSpace.sm),
              Text(
                deadline,
                key: const ValueKey('inquiry-deadline'),
                style: AppTypography.caption.copyWith(
                  color: overdue ? AppColors.danger : AppColors.textMuted,
                  fontWeight: overdue ? FontWeight.w700 : null,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
