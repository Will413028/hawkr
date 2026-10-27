import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 優惠券主操作。以可存取的原生按鈕取代固定尺寸點陣圖片。
class CouponSubmitButton extends StatelessWidget {
  const CouponSubmitButton({
    super.key,
    required this.isComplete,
    required this.saving,
    required this.onTap,
  });

  final bool isComplete;
  final bool saving;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final statusText = isComplete ? '內容已完整，送出後將依選擇對象發放' : '完成名稱、折扣與使用期限後即可送出';
    return Column(
      children: [
        Row(
          children: [
            HkIcon(
              isComplete ? HkIconName.check : HkIconName.info,
              size: 18,
              color: isComplete ? AppIconColors.success : AppIconColors.muted,
            ),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Text(
                statusText,
                style: TextStyle(
                  color: isComplete ? AppColors.success : AppColors.textMuted,
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: HkSpace.md),
        SizedBox(
          width: double.infinity,
          height: 54,
          child: FilledButton.icon(
            onPressed: saving || !isComplete ? null : onTap,
            icon: saving
                ? const SizedBox.square(
                    dimension: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const HkIcon(
                    HkIconName.externalLink,
                    size: 20,
                    color: AppIconColors.onBrand,
                  ),
            label: HkButtonLabel(saving ? '正在建立優惠券…' : '儲存並發放優惠券'),
          ),
        ),
      ],
    );
  }
}
