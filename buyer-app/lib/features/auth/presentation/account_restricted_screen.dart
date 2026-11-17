import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/session/account_restriction.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import 'account_deletion_flow.dart';

/// 受限狀態頁「帳號受限」：列停權的日期、類別與說明，提供刪除帳號與登出。停權不登出，其餘功能照常瀏覽。
class AccountRestrictedScreen extends ConsumerStatefulWidget {
  const AccountRestrictedScreen({super.key});

  static const path = '/account-restricted';
  static const browseNotice = '停權期間可以瀏覽，無法下單、留言、按讚、追蹤或修改資料。';

  @override
  ConsumerState<AccountRestrictedScreen> createState() =>
      _AccountRestrictedScreenState();
}

class _AccountRestrictedScreenState
    extends ConsumerState<AccountRestrictedScreen> {
  bool _busy = false;
  String? _message;

  Future<void> _delete() async {
    if (_busy) return;
    setState(() => _message = null);
    final message = await confirmAndDeleteAccount(
      context,
      ref,
      onBusy: (busy) => setState(() => _busy = busy),
    );
    if (mounted && message != null) setState(() => _message = message);
  }

  Future<void> _signOut() async {
    if (_busy) return;
    await ref.read(sessionProvider.notifier).logout();
    if (mounted) context.go('/');
  }

  @override
  Widget build(BuildContext context) {
    final suspension = ref.watch(accountRestrictionProvider);
    if (suspension == null) {
      // 沒有停權（已解除或已登出）：這頁沒有內容，回首頁。
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && !_busy) context.go('/');
      });
      return const Scaffold(backgroundColor: AppColors.bgPrimary);
    }
    final locale = ref.watch(uiLocaleProvider);
    final category = suspension.category.uiText(locale);
    final suspendedAt = suspension.suspendedAt;
    final reason = suspension.reason.trim();

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(context, title: '帳號受限'),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(HkSpace.lg),
          children: [
            if (suspendedAt != null)
              _InfoRow(label: '停權日期', value: formatTaipeiDate(suspendedAt)),
            if (category != null) _InfoRow(label: '類別', value: category),
            if (reason.isNotEmpty) _InfoRow(label: '說明', value: reason),
            const SizedBox(height: HkSpace.sm),
            Text(
              AccountRestrictedScreen.browseNotice,
              style: AppTypography.body,
            ),
            const SizedBox(height: HkSpace.xxl),
            if (_message != null)
              Padding(
                padding: const EdgeInsets.only(bottom: HkSpace.md),
                child: Semantics(
                  liveRegion: true,
                  child: Text(
                    _message!,
                    style: AppTypography.metaMuted.copyWith(
                      color: AppColors.danger,
                    ),
                  ),
                ),
              ),
            ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: OutlinedButton(
                key: const ValueKey('account-restricted-delete'),
                onPressed: _busy ? null : _delete,
                style: OutlinedButton.styleFrom(
                  foregroundColor: AppColors.danger,
                  side: const BorderSide(color: AppColors.danger),
                  textStyle: AppTypography.buttonLarge,
                ),
                child: HkButtonLabel(_busy ? '刪除帳號中…' : '刪除帳號'),
              ),
            ),
            const SizedBox(height: HkSpace.md),
            ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: OutlinedButton(
                key: const ValueKey('account-restricted-sign-out'),
                onPressed: _busy ? null : _signOut,
                child: const HkButtonLabel('登出'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: HkSpace.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.metaMuted),
          const SizedBox(height: HkSpace.xs),
          Text(value, style: AppTypography.body),
        ],
      ),
    );
  }
}
