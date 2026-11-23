import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/hk_adaptive_page.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/account_scope.dart';
import '../data/inquiries.dart';
import '../data/inquiry_display.dart';
import 'seller_inquiries_screen.dart';
import '../../../shared/time/taipei_display.dart';

/// 一則詢問的內容（路由 [SellerRoutes.inquiryDetail]，以詢問 id 開啟）：平台的說明、期限、狀態；已回覆的顯示回覆的管道、時間、
/// 內容與附件數（賣家不讀回附件）；伺服器的 `can_reply` 為真才有「回覆」入口。
class SellerInquiryDetailScreen extends ConsumerWidget {
  const SellerInquiryDetailScreen({
    super.key,
    required this.inquiryId,
    this.clock = DateTime.now,
  });

  final String inquiryId;
  final DateTime Function() clock;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    Widget shell(Widget child) => Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: const Text(inquiriesTitle),
        onBackPressed: () => leaveInquiries(context),
      ),
      body: HkAdaptivePage(child: child),
    );
    if (accountId == null) return shell(const TruckLoadingOverlay());
    final provider = inquiryByIdProvider(accountId, inquiryId);
    final async = ref.watch(provider);
    final loaded = async.value;
    if (loaded != null) {
      return shell(_DetailBody(inquiry: loaded, now: clock()));
    }
    if (async.hasError && !async.isLoading) {
      final code = ConnectException.from(async.error!).code;
      final notFound = code == Code.notFound || code == Code.permissionDenied;
      return shell(
        ListView(
          padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
          children: [
            notFound
                ? HkAsyncStatePanel(
                    kind: HkAsyncStateKind.empty,
                    title: inquiryNotFoundTitle,
                    description: inquiryNotFoundDescription,
                    actionLabel: inquiryNotFoundAction,
                    actionIcon: HkIconName.back,
                    onAction: () => context.go(SellerRoutes.inquiries),
                  )
                : HkAsyncStatePanel(
                    kind: HkAsyncStateKind.error,
                    title: inquiriesLoadFailedTitle,
                    description: inquiriesLoadFailedDescription,
                    actionLabel: '重試',
                    onAction: () => ref.invalidate(provider),
                  ),
          ],
        ),
      );
    }
    return shell(const TruckLoadingOverlay());
  }
}

class _DetailBody extends StatelessWidget {
  const _DetailBody({required this.inquiry, required this.now});

  final Inquiry inquiry;
  final DateTime now;

  @override
  Widget build(BuildContext context) {
    final chip = inquiryStateChip(inquiry, now, context.uiLocale);
    final deadline = inquiryDeadlineText(inquiry, context.taipeiDisplay);
    final overdue = isInquiryOverdue(inquiry, now);
    final replyMeta = inquiryReplyMeta(
      inquiry,
      context.uiLocale,
      context.taipeiDisplay,
    );
    final attachments = inquiryAttachmentText(inquiry);
    return ListView(
      padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
      children: [
        HkContentCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Text(
                      inquiry.kind.uiText(context.uiLocale) ?? inquiriesTitle,
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
              const SizedBox(height: HkSpace.xs),
              Text(
                inquiryMeta(inquiry, context.uiLocale, context.taipeiDisplay),
                style: AppTypography.caption.copyWith(
                  color: AppColors.textMuted,
                ),
              ),
              if (inquiry.message.isNotEmpty) ...[
                const SizedBox(height: HkSpace.md),
                Text(
                  inquiry.message,
                  key: const ValueKey('inquiry-message'),
                  style: AppTypography.meta,
                ),
              ],
              if (deadline != null) ...[
                const SizedBox(height: HkSpace.md),
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
        if (inquiry.hasReply()) ...[
          const SizedBox(height: HkSpace.md),
          HkContentCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('你的回覆', style: AppTypography.sectionTitle),
                if (replyMeta != null) ...[
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    replyMeta,
                    key: const ValueKey('inquiry-reply-meta'),
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textMuted,
                    ),
                  ),
                ],
                const SizedBox(height: HkSpace.md),
                Text(
                  inquiry.reply.body,
                  key: const ValueKey('inquiry-reply-body'),
                  style: AppTypography.meta,
                ),
                if (attachments != null) ...[
                  const SizedBox(height: HkSpace.sm),
                  Text(
                    attachments,
                    key: const ValueKey('inquiry-reply-attachments'),
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textMuted,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
        if (inquiry.canReply) ...[
          const SizedBox(height: HkSpace.lg),
          FilledButton(
            key: const ValueKey('inquiry-reply-button'),
            onPressed: () =>
                context.push(SellerRoutes.inquiryReplyFor(inquiry.id)),
            child: const HkButtonLabel('回覆'),
          ),
        ],
      ],
    );
  }
}
