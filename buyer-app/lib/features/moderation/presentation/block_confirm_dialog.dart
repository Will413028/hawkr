import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../data/block_repository.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 封鎖使用者確認彈窗。
/// 使用：`await showBlockConfirmDialog(context, userId: '123', userLabel: '王小明')`
class BlockConfirmDialog extends ConsumerStatefulWidget {
  const BlockConfirmDialog({
    super.key,
    required this.userId,
    required this.userLabel,
  });

  final String userId;
  final String userLabel;

  @override
  ConsumerState<BlockConfirmDialog> createState() => _BlockConfirmDialogState();
}

class _BlockConfirmDialogState extends ConsumerState<BlockConfirmDialog> {
  bool _submitting = false;

  Future<void> _confirmBlock() async {
    setState(() => _submitting = true);
    try {
      await ref.read(blockRepositoryProvider).blockUser(widget.userId);
      if (!mounted) return;
      Navigator.of(context).pop(true);
      showFloatingToast(context, '已封鎖「${widget.userLabel}」', isSuccess: true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      showFloatingToast(
        context,
        e.toString().replaceAll('Exception: ', ''),
        isSuccess: false,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16.r)),
      title: Text(
        '封鎖「${widget.userLabel}」',
        style: TextStyle(
          fontSize: 18,
          fontWeight: FontWeight.w700,
          color: AppColors.textEmphasis,
        ),
      ),
      content: Text(
        '封鎖後對方將無法傳送訊息給您，您也無法傳送訊息給對方。可隨時在「封鎖名單」解除。',
        style: TextStyle(
          fontSize: 14,
          color: AppColors.textSecondary,
          height: 1.4,
        ),
      ),
      actions: [
        TextButton(
          onPressed: _submitting
              ? null
              : () => Navigator.of(context).pop(false),
          child: HkButtonLabel(
            '取消',
            style: TextStyle(color: AppColors.textSecondary),
          ),
        ),
        TextButton(
          onPressed: _submitting ? null : _confirmBlock,
          child: _submitting
              ? SizedBox(
                  width: 16.w,
                  height: 16.w,
                  child: const HkBrandLoader(
                    tone: HkBrandLoaderTone.mono,
                    color: AppColors.danger,
                  ),
                )
              : HkButtonLabel(
                  '封鎖',
                  style: TextStyle(
                    color: AppColors.danger,
                    fontWeight: FontWeight.w700,
                  ),
                ),
        ),
      ],
    );
  }
}

/// 顯示封鎖確認彈窗。回傳 true 表示已成功封鎖。
Future<bool?> showBlockConfirmDialog(
  BuildContext context, {
  required String userId,
  required String userLabel,
}) {
  return showDialog<bool>(
    context: context,
    builder: (_) => BlockConfirmDialog(userId: userId, userLabel: userLabel),
  );
}
