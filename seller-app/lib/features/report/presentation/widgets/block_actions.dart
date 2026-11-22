import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hawkr_api/errors.dart' show errorText;

import '../../../../core/l10n/ui_locale.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/app_confirm_dialog.dart';
import '../../data/store_block_repository.dart';
import '../../data/store_block_rules.dart';

/// 封鎖確認彈窗；[message] 由呼叫端決定（聊天用 [chatBlockConfirmMessage]，留言與討論區用 [storeBlockConfirmMessage]）。
Future<bool> showBlockConfirmDialog(
  BuildContext context, {
  required String targetName,
  required String message,
}) {
  return showAppConfirmDialog(
    context,
    title: '封鎖「$targetName」？',
    message: message,
    confirmText: '封鎖',
    confirmTextColor: AppColors.danger,
  );
}

const chatBlockConfirmMessage = '封鎖後，對方將無法傳送訊息或留言給你；你可以隨時在「封鎖名單」解除封鎖。';

const storeBlockConfirmMessage =
    '封鎖後，對方將無法在你的貼文與討論區留言、按讚，也無法傳送訊息給你；對方仍看得到你公開的內容，評價不受影響。你可以隨時在「封鎖名單」解除封鎖。';

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
  final name = blockTargetName(buyerName);
  final confirmed = await showBlockConfirmDialog(
    context,
    targetName: name,
    message: storeBlockConfirmMessage,
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
    FloatingToast.success(context, '已封鎖「$name」');
  } on Exception catch (e) {
    if (!context.mounted) return;
    FloatingToast.error(context, errorText(e, defaults: defaults));
  }
}

/// 聊天室的封鎖：確認後以訊息 ID 呼叫 `BlockBuyer`（封鎖該則訊息所屬對話的買家）；訊息已隱藏或抹除仍可當參照。
Future<void> blockBuyerByMessageWithConfirm(
  BuildContext context,
  WidgetRef ref, {
  required String messageId,
  required String buyerName,
}) async {
  final name = blockTargetName(buyerName);
  final confirmed = await showBlockConfirmDialog(
    context,
    targetName: name,
    message: chatBlockConfirmMessage,
  );
  if (!confirmed || !context.mounted) return;
  final repo = ref.read(storeBlockRepositoryProvider);
  final defaults = ref.read(defaultErrorTextProvider);
  try {
    await repo.blockByMessage(messageId);
    if (!context.mounted) return;
    FloatingToast.success(context, '已封鎖「$name」');
  } on Exception catch (e) {
    if (!context.mounted) return;
    FloatingToast.error(context, errorText(e, defaults: defaults));
  }
}
