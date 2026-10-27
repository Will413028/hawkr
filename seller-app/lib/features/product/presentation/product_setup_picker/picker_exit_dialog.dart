import 'package:hawkr_seller/shared/widgets/app_confirm_dialog.dart';
import 'package:flutter/material.dart';

/// 商品編輯各步驟共用的離開保護。
Future<void> showProductWizardExitDialog(
  BuildContext context, {
  required VoidCallback onConfirm,
}) async {
  final confirmed = await showAppConfirmDialog(
    context,
    title: '要離開商品編輯嗎？',
    message: '商品尚未完成，目前輸入的內容可能不會保留。',
    cancelText: '繼續編輯',
    confirmText: '離開',
    barrierDismissible: true,
  );
  if (confirmed && context.mounted) onConfirm();
}
