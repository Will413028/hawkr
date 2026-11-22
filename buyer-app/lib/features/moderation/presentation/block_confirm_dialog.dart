import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'package:hawkr_api/errors.dart' show errorText;
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../data/block_repository.dart';
import '../data/block_target.dart';
import '../data/social_blocker.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 封鎖確認彈窗。說明文字與送到哪條 API 都由 [target] 的型別決定：社群對象送 `BuyerAppService`，
/// 聊天（[LegacyBlockTarget]）送舊 REST。
/// 使用：`await showBlockConfirmDialog(context, target: const StoreBlockTarget('7'), userLabel: '好店')`
class BlockConfirmDialog extends ConsumerStatefulWidget {
  const BlockConfirmDialog({
    super.key,
    required this.target,
    required this.userLabel,
  });

  final BlockTarget target;
  final String userLabel;

  @override
  ConsumerState<BlockConfirmDialog> createState() => _BlockConfirmDialogState();
}

class _BlockConfirmDialogState extends ConsumerState<BlockConfirmDialog> {
  bool _submitting = false;

  Future<void> _confirmBlock() async {
    setState(() => _submitting = true);
    try {
      switch (widget.target) {
        case final SocialBlockTarget target:
          await ref.read(socialBlockerProvider).block(target);
        case LegacyBlockTarget(:final userId):
          await ref.read(blockRepositoryProvider).blockUser(userId);
      }
      if (!mounted) return;
      Navigator.of(context).pop(true);
      showFloatingToast(context, '已封鎖「${widget.userLabel}」', isSuccess: true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      // 停權由 transport 的 interceptor 提示，這裡不再疊一則失敗訊息。
      if (isAccountSuspendedError(e)) return;
      showFloatingToast(
        context,
        widget.target is SocialBlockTarget
            ? errorText(e, defaults: ref.read(defaultErrorTextProvider))
            : e.toString().replaceAll('Exception: ', ''),
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
        widget.target.confirmText,
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
  required BlockTarget target,
  required String userLabel,
}) {
  return showDialog<bool>(
    context: context,
    builder: (_) => BlockConfirmDialog(target: target, userLabel: userLabel),
  );
}
