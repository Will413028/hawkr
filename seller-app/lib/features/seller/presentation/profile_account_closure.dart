import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'
    show SellerCapability;
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_page_section.dart';
import '../../auth/presentation/auth_controller.dart';
import '../data/account_deletion.dart';
import '../data/current_store.dart';
import '../data/store_closing.dart';
import '../data/version_conflict.dart';

/// 設定頁的「關閉商店」。呼叫端只在 `ACCOUNT_CLOSE` 能力存在時顯示（停權中也可以關店，等待刪除時沒有）。
/// 成功後店家狀態重讀，路由把賣家導去開店頁。
class ProfileCloseStoreSection extends ConsumerStatefulWidget {
  const ProfileCloseStoreSection({super.key});

  @override
  ConsumerState<ProfileCloseStoreSection> createState() =>
      _ProfileCloseStoreSectionState();
}

class _ProfileCloseStoreSectionState
    extends ConsumerState<ProfileCloseStoreSection> {
  bool _busy = false;

  Future<void> _close() async {
    if (_busy) return;
    final confirmed = await showAppConfirmDialog(
      context,
      title: '關閉商店',
      message:
          '關閉後商店與商品不再公開，logo 與封面會清除，賣家身分會移除；帳號與買家身分會保留。這個動作無法復原，之後要再販售需重新申請開店。有未結訂單時無法關閉。',
      confirmText: '確認關閉',
      confirmTextColor: AppColors.danger,
    );
    if (!confirmed || !mounted) return;
    setState(() => _busy = true);
    try {
      final outcome = await ref.read(storeClosingProvider).close();
      if (!mounted) return;
      if (outcome case StoreClosingFailed(:final message)) {
        FloatingToast.error(context, message);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return HkPageSection(
      title: '關閉商店',
      description: '關閉後商店與商品不再公開，賣家身分會移除；帳號與買家身分會保留。',
      child: HkContentCard(
        child: SizedBox(
          width: double.infinity,
          child: TextButton.icon(
            onPressed: _busy ? null : _close,
            icon: const HkIcon(HkIconName.warning, color: AppIconColors.danger),
            label: HkButtonLabel(
              _busy ? '關閉商店中…' : '關閉商店',
              style: const TextStyle(color: AppColors.danger),
            ),
          ),
        ),
      ),
    );
  }
}

/// 設定頁的「刪除帳號」（Apple Guideline 5.1.1(v) 要求 App 內提供帳號刪除）。順序見 [AccountDeletion]。
class ProfileDeleteAccountSection extends ConsumerStatefulWidget {
  const ProfileDeleteAccountSection({super.key, this.onGoToCloseStore});

  /// 帳號還有商店時，「前往關閉商店」要做的事。
  final VoidCallback? onGoToCloseStore;

  @override
  ConsumerState<ProfileDeleteAccountSection> createState() =>
      _ProfileDeleteAccountSectionState();
}

class _ProfileDeleteAccountSectionState
    extends ConsumerState<ProfileDeleteAccountSection> {
  bool _busy = false;

  Future<void> _delete() async {
    if (_busy) return;
    final confirmed = await showAppConfirmDialog(
      context,
      title: '刪除帳號',
      message: '刪除後帳號資料會被移除，無法復原。還有未完成的訂單時，帳號會在最後一筆訂單完成後刪除，期間無法新增收藏與追蹤。',
      confirmText: '確認刪除',
      confirmTextColor: AppColors.danger,
    );
    if (!confirmed || !mounted) return;
    setState(() => _busy = true);
    final AccountDeletionOutcome outcome;
    try {
      outcome = await ref.read(accountDeletionProvider).run();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (!mounted) return;
    switch (outcome) {
      case AccountDeleted():
        FloatingToast.success(context, '帳號已刪除');
        await ref.read(sessionProvider.notifier).logout();
        if (!mounted) return;
        context.go('/auth');
      case AccountDeletionPending():
        FloatingToast.success(context, accountPendingOrdersMessage);
        await ref.read(currentStoreProvider.notifier).refresh();
      case AccountHasOpenStore():
        final refreshed = await ref
            .read(currentStoreProvider.notifier)
            .refresh();
        if (!mounted) return;
        if (!refreshed) {
          FloatingToast.error(context, storeStateUnavailableNotice);
          return;
        }
        final canClose =
            ref
                .read(myStoreSnapshotProvider)
                ?.can(SellerCapability.SELLER_CAPABILITY_ACCOUNT_CLOSE) ??
            false;
        if (!canClose) {
          FloatingToast.error(context, accountHasOpenStoreMessage);
          return;
        }
        final goToClose = await showAppConfirmDialog(
          context,
          title: '刪除帳號',
          message: accountHasOpenStoreMessage,
          confirmText: '前往關閉商店',
        );
        if (goToClose && mounted) widget.onGoToCloseStore?.call();
      case AccountDeletionFailed(:final message):
        FloatingToast.error(context, message);
    }
  }

  @override
  Widget build(BuildContext context) {
    return HkPageSection(
      title: '刪除帳號',
      description: '刪除後帳號資料會被移除，無法復原。',
      child: HkContentCard(
        child: SizedBox(
          width: double.infinity,
          child: TextButton.icon(
            onPressed: _busy ? null : _delete,
            icon: const HkIcon(HkIconName.warning, color: AppIconColors.danger),
            label: HkButtonLabel(
              _busy ? '刪除帳號中…' : '刪除帳號',
              style: const TextStyle(color: AppColors.danger),
            ),
          ),
        ),
      ),
    );
  }
}
