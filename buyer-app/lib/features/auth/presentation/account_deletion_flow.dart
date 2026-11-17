import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../data/account_deletion.dart';

/// 刪除帳號的完整流程：確認 → `DeleteAccount` → 依結果登出回首頁。回傳要顯示給使用者的訊息：
/// 取消、帳號已刪除時是 null。[onBusy] 在請求前後通知，供呼叫端停用按鈕。
Future<String?> confirmAndDeleteAccount(
  BuildContext context,
  WidgetRef ref, {
  required ValueChanged<bool> onBusy,
}) async {
  final confirmed = await showCommonDialog(
    context: context,
    title: '刪除帳號',
    content:
        '刪除後帳號資料會被移除，無法復原。還有未完成的訂單時，帳號會在最後一筆訂單完成後刪除，期間無法新增收藏與追蹤。\n\n若你有開設商店，商店也會一起關閉，商店與商品不再公開，賣家身分會移除。有未結訂單時，帳號與商店會在訂單結清後才刪除，期間商店不能自行關閉或換店。',
    confirmText: '確認刪除',
    cancelText: '取消',
    confirmColor: AppColors.danger,
    barrierDismissible: true,
  );
  if (confirmed != true || !context.mounted) return null;

  onBusy(true);
  final AccountDeletionOutcome outcome;
  try {
    outcome = await ref.read(accountDeletionProvider).run();
  } finally {
    if (context.mounted) onBusy(false);
  }
  if (!context.mounted) return null;
  switch (outcome) {
    case AccountDeleted():
      await ref.read(sessionProvider.notifier).logout();
      if (context.mounted) context.go('/');
      return null;
    case AccountDeletionPending():
      return accountPendingOrdersMessage;
    case AccountDeletionFailed(:final message):
      return message;
  }
}
