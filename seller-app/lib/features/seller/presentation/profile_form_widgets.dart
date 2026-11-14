import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_page_section.dart';

/// 設定頁基本資料區。保留原本 rows API，讓資料與儲存流程不受 UI 重做影響。
class ProfileInfoSection extends StatelessWidget {
  const ProfileInfoSection({
    super.key,
    required this.title,
    required this.rows,
    this.description = '點選欄位即可編輯；儲存後會同步到買家看到的店舖資訊。',
  });

  final String title;
  final List<Widget> rows;
  final String? description;

  @override
  Widget build(BuildContext context) {
    return HkPageSection(
      title: title,
      description: description,
      child: HkContentCard(
        padding: EdgeInsets.zero,
        child: Column(
          children: [
            for (var index = 0; index < rows.length; index++) ...[
              rows[index],
              if (index != rows.length - 1)
                const Divider(
                  height: 1,
                  indent: HkSpace.lg,
                  endIndent: HkSpace.lg,
                  color: AppColors.dividerLight,
                ),
            ],
          ],
        ),
      ),
    );
  }
}

class ProfileEditableInfoRow extends StatelessWidget {
  const ProfileEditableInfoRow({
    super.key,
    required this.label,
    this.controllers = const [],
    this.value,
    this.placeholder,
    this.editable = true,
    this.maxLines = 1,
    this.showTrailingIcon = true,
    this.onTap,
  });

  final String label;
  final List<TextEditingController> controllers;
  final String? value;
  final String? placeholder;
  final bool editable;
  final int maxLines;
  final bool showTrailingIcon;

  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final rawValue =
        value ?? (controllers.isNotEmpty ? controllers.first.text.trim() : '');
    final isEmpty = rawValue.isEmpty;
    final displayValue = isEmpty ? (placeholder ?? '尚未設定') : rawValue;
    final enabled = editable && onTap != null;

    return Semantics(
      button: enabled,
      label: '$label：$displayValue',
      hint: enabled ? '點兩下編輯' : null,
      child: InkWell(
        onTap: enabled ? onTap : null,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 64),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: HkSpace.lg,
              vertical: HkSpace.md,
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                SizedBox(
                  width: 96,
                  child: Text(
                    label,
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        displayValue,
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: isEmpty
                              ? FontWeight.w400
                              : FontWeight.w700,
                          color: isEmpty
                              ? AppColors.textMuted
                              : AppColors.textEmphasis,
                        ),
                        maxLines: maxLines,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                if (showTrailingIcon && enabled) ...[
                  const SizedBox(width: HkSpace.sm),
                  const HkIcon(
                    HkIconName.edit,
                    size: 20,
                    color: AppIconColors.muted,
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class ProfileAddressRow extends StatelessWidget {
  const ProfileAddressRow({
    super.key,
    this.label = '店家地址',
    required this.value,
    required this.onTap,
  });

  final String label;
  final String value;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return ProfileEditableInfoRow(
      label: label,
      value: value,
      placeholder: '請設定縣市、區域與詳細地址',
      maxLines: 2,
      onTap: onTap,
    );
  }
}
