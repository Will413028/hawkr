import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/exceptions/app_exception.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/app_confirm_dialog.dart';
import '../../data/block_repository.dart';
import '../report_block_error_dispatch.dart';

/// 封鎖確認彈窗（backend-payuni#645）。
Future<bool> showBlockConfirmDialog(
  BuildContext context, {
  required String targetName,
}) {
  return showAppConfirmDialog(
    context,
    title: '封鎖「$targetName」？',
    message: '封鎖後，對方將無法傳送訊息或留言給你；你可以隨時在「封鎖名單」解除封鎖。',
    confirmText: '封鎖',
    confirmTextColor: AppColors.danger,
  );
}

/// 解除封鎖確認彈窗。
Future<bool> showUnblockConfirmDialog(
  BuildContext context, {
  required String targetName,
}) {
  return showAppConfirmDialog(
    context,
    title: '解除封鎖「$targetName」？',
    message: '解除後對方可以重新傳送訊息或留言給你。',
    confirmText: '解除封鎖',
  );
}

/// 顯示封鎖確認彈窗 → 確認後呼叫封鎖 API 並顯示結果 toast。
/// 供聊天／留言等各檢舉入口共用（backend-payuni#645）。
Future<void> blockUserWithConfirm(
  BuildContext context,
  WidgetRef ref, {
  required String userId,
  required String userName,
}) async {
  final confirmed = await showBlockConfirmDialog(context, targetName: userName);
  if (!confirmed) return;
  if (!context.mounted) return;
  try {
    await ref.read(blockRepositoryProvider).blockUser(userId);
    if (!context.mounted) return;
    FloatingToast.success(context, '已封鎖「$userName」');
  } catch (e) {
    if (!context.mounted) return;
    final handled = handleReportBlockApiError(context, e);
    if (!handled) {
      FloatingToast.error(
        context,
        e is AppException ? e.message : '封鎖失敗，請稍後再試',
      );
    }
  }
}
