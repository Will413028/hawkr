import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../icons/app_icon_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_icon.dart';

/// 停權帳號的橫幅，放在共用 Layout 的每頁頂端；點下去開受限狀態頁。
class AccountRestrictedBanner extends StatelessWidget {
  const AccountRestrictedBanner({super.key, required this.onTap});

  static const message = '帳號已停權，部分功能受限';

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.warningSoft,
      child: InkWell(
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
            child: Row(
              children: [
                const HkIcon(
                  HkIconName.warning,
                  size: 20,
                  color: AppIconColors.warningText,
                ),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Text(
                    message,
                    style: AppTypography.meta.copyWith(
                      color: AppColors.warningText,
                    ),
                  ),
                ),
                const HkIcon(
                  HkIconName.chevronRight,
                  size: 20,
                  color: AppIconColors.warningText,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
