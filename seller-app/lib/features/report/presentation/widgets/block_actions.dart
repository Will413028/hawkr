import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hawkr_api/errors.dart' show errorText;

import '../../../../core/exceptions/app_exception.dart';
import '../../../../core/l10n/ui_locale.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/app_confirm_dialog.dart';
import '../../data/block_repository.dart';
import '../../data/store_block_repository.dart';
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

const storeBlockConfirmMessage =
    '封鎖後，對方將無法在你的貼文與討論區留言、按讚，也無法傳送訊息給你；對方仍看得到你公開的內容，評價不受影響。你可以隨時在「封鎖名單」解除封鎖。';

/// 店家封鎖買家（貼文留言、討論區留言入口）的確認彈窗。
Future<bool> showStoreBlockConfirmDialog(
  BuildContext context, {
  required String targetName,
}) {
  return showAppConfirmDialog(
    context,
    title: '封鎖「$targetName」？',
    message: storeBlockConfirmMessage,
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

/// 以留言封鎖買家：確認後呼叫 `BlockBuyer`，[commentId] 與 [forumEntryId] 恰好給一個。
Future<void> blockBuyerWithConfirm(
  BuildContext context,
  WidgetRef ref, {
  required String buyerName,
  String? commentId,
  String? forumEntryId,
}) async {
  assert((commentId == null) != (forumEntryId == null));
  final confirmed = await showStoreBlockConfirmDialog(
    context,
    targetName: buyerName,
  );
  if (!confirmed || !context.mounted) return;
  final repo = ref.read(storeBlockRepositoryProvider);
  final defaults = ref.read(defaultErrorTextProvider);
  try {
    if (commentId != null) {
      await repo.blockByComment(commentId);
    } else {
      await repo.blockByForumEntry(forumEntryId!);
    }
    if (!context.mounted) return;
    FloatingToast.success(context, '已封鎖「$buyerName」');
  } on Exception catch (e) {
    if (!context.mounted) return;
    FloatingToast.error(context, errorText(e, defaults: defaults));
  }
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
