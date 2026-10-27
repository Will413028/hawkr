import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:flutter/material.dart';

import 'hk_button_label.dart';

/// 統一樣式的對話框 widget
/// 與驗證碼頁的錯誤提示 dialog 樣式一致
class CommonDialog extends StatelessWidget {
  const CommonDialog({
    super.key,
    this.title,
    required this.content,
    this.confirmText = '確定',
    this.cancelText,
    this.onConfirm,
    this.onCancel,
    this.confirmColor = AppColors.textEmphasis,
    this.cancelColor = AppColors.textSecondary,
    this.showDivider = true,
  });

  final String? title;
  final String content;
  final String confirmText;
  final String? cancelText;
  final VoidCallback? onConfirm;
  final VoidCallback? onCancel;
  final Color confirmColor;
  final Color cancelColor;
  final bool showDivider;

  @override
  Widget build(BuildContext context) {
    final hasCancel = cancelText != null && cancelText!.isNotEmpty;
    final availableHeight =
        (MediaQuery.sizeOf(context).height -
                MediaQuery.paddingOf(context).vertical -
                HkSpace.section * 2)
            .clamp(HkControl.minTap * 2, double.infinity)
            .toDouble();

    return Dialog(
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(HkRadius.lg),
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(HkRadius.lg),
        child: ConstrainedBox(
          constraints: BoxConstraints(maxHeight: availableHeight),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Flexible(
                fit: FlexFit.loose,
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(HkSpace.xxl),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (title != null && title!.isNotEmpty) ...[
                        Text(
                          title!,
                          textAlign: TextAlign.center,
                          style: AppTypography.sectionTitle,
                        ),
                        const SizedBox(height: HkSpace.lg),
                      ],
                      Text(
                        content,
                        textAlign: TextAlign.center,
                        style: AppTypography.body.copyWith(
                          fontWeight: FontWeight.w500,
                          color: AppColors.textEmphasis,
                          height: 1.4,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              // 分隔線
              if (showDivider)
                const Divider(height: 1, color: AppColors.gray300),
              // 按鈕區
              if (hasCancel)
                // 雙按鈕（取消 + 確定）
                IntrinsicHeight(
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Expanded(
                        child: TextButton(
                          style: TextButton.styleFrom(
                            minimumSize: const Size(0, HkControl.minTap),
                            padding: const EdgeInsets.symmetric(
                              horizontal: HkSpace.sm,
                            ),
                            backgroundColor: AppColors.bgCard,
                            shape: const RoundedRectangleBorder(
                              borderRadius: BorderRadius.only(
                                bottomLeft: Radius.circular(HkRadius.lg),
                              ),
                            ),
                          ),
                          onPressed: () {
                            Navigator.of(context).pop(false);
                            onCancel?.call();
                          },
                          child: HkButtonLabel(
                            cancelText!,
                            style: AppTypography.sectionTitle.copyWith(
                              color: cancelColor,
                            ),
                          ),
                        ),
                      ),
                      const VerticalDivider(
                        width: 1,
                        thickness: 1,
                        color: AppColors.gray300,
                      ),
                      Expanded(
                        child: TextButton(
                          style: TextButton.styleFrom(
                            minimumSize: const Size(0, HkControl.minTap),
                            padding: const EdgeInsets.symmetric(
                              horizontal: HkSpace.sm,
                            ),
                            backgroundColor: AppColors.bgCard,
                            shape: const RoundedRectangleBorder(
                              borderRadius: BorderRadius.only(
                                bottomRight: Radius.circular(HkRadius.lg),
                              ),
                            ),
                          ),
                          onPressed: () {
                            Navigator.of(context).pop(true);
                            onConfirm?.call();
                          },
                          child: HkButtonLabel(
                            confirmText,
                            style: AppTypography.sectionTitle.copyWith(
                              color: confirmColor,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                )
              else
                // 單按鈕（確定）
                SizedBox(
                  width: double.infinity,
                  child: TextButton(
                    style: TextButton.styleFrom(
                      minimumSize: const Size(0, HkControl.minTap),
                      padding: const EdgeInsets.symmetric(
                        horizontal: HkSpace.sm,
                      ),
                      backgroundColor: AppColors.bgCard,
                      shape: const RoundedRectangleBorder(
                        borderRadius: BorderRadius.only(
                          bottomLeft: Radius.circular(HkRadius.lg),
                          bottomRight: Radius.circular(HkRadius.lg),
                        ),
                      ),
                    ),
                    onPressed: () {
                      Navigator.of(context).pop(true);
                      onConfirm?.call();
                    },
                    child: HkButtonLabel(
                      confirmText,
                      style: AppTypography.sectionTitle.copyWith(
                        color: confirmColor,
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 顯示統一樣式的對話框
///
/// 用法：
/// ```dart
/// await showCommonDialog(
///   context: context,
///   content: '操作成功',
/// );
/// ```
///
/// 帶確認/取消按鈕：
/// ```dart
/// final confirmed = await showCommonDialog(
///   context: context,
///   title: '確認刪除',
///   content: '確定要刪除此項目嗎？',
///   confirmText: '刪除',
///   cancelText: '取消',
///   confirmColor: AppColors.danger,
/// );
/// if (confirmed == true) { ... }
/// ```
Future<bool?> showCommonDialog({
  required BuildContext context,
  String? title,
  required String content,
  String confirmText = '確定',
  String? cancelText,
  VoidCallback? onConfirm,
  VoidCallback? onCancel,
  Color confirmColor = AppColors.textEmphasis,
  Color cancelColor = AppColors.textSecondary,
  bool barrierDismissible = false,
  bool showDivider = true,
}) {
  return showDialog<bool>(
    context: context,
    barrierDismissible: barrierDismissible,
    builder: (context) => CommonDialog(
      title: title,
      content: content,
      confirmText: confirmText,
      cancelText: cancelText,
      onConfirm: onConfirm,
      onCancel: onCancel,
      confirmColor: confirmColor,
      cancelColor: cancelColor,
      showDivider: showDivider,
    ),
  );
}
