import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 購物車的交易備註；控制器由主畫面管理，結帳時保留原始輸入。
class DeliveryNotesSection extends StatelessWidget {
  const DeliveryNotesSection({super.key, required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) => Container(
    margin: const EdgeInsets.symmetric(
      horizontal: HkSpace.lg,
      vertical: HkSpace.sm,
    ),
    padding: const EdgeInsets.all(HkSpace.lg),
    decoration: BoxDecoration(
      color: AppColors.bgCard,
      borderRadius: BorderRadius.circular(HkRadius.lg),
      border: Border.all(color: AppColors.cardBorder),
      boxShadow: HkShadows.card,
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            const HkIcon(HkIconName.edit, size: HkControl.icon),
            const SizedBox(width: HkSpace.sm),
            Text('交易備註', style: CartTypography.section),
          ],
        ),
        const SizedBox(height: HkSpace.md),
        TextField(
          controller: controller,
          decoration: const InputDecoration(hintText: '例如：請放管理室、需要收據、卸貨需協助等'),
          minLines: 2,
          maxLines: 4,
          style: CartTypography.body,
        ),
      ],
    ),
  );
}
