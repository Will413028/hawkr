import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_icon.dart';

/// 一列「圖示＋標題＋說明＋箭頭」的入口（設定頁的營運設定清單、開店頁的措施提示卡共用）。
///
/// 整列是一個 button（semantics 的 label 是標題、hint 是說明），最小高度 72。
class OptionRow extends StatelessWidget {
  const OptionRow({
    super.key,
    required this.icon,
    required this.label,
    this.description,
    required this.onTap,
    this.badgeCount = 0,
    this.statusLabel,
  });

  final HkIconName icon;
  final String label;

  /// 標題下的一行說明；沒有時只顯示標題。
  final String? description;
  final VoidCallback onTap;

  /// 大於 0 時在右側顯示未讀數。
  final int badgeCount;

  /// 右側的狀態標籤（例如「需認證」）。
  final String? statusLabel;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      hint: description,
      child: InkWell(
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 72),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: HkSpace.lg,
              vertical: HkSpace.md,
            ),
            child: Row(
              children: [
                DecoratedBox(
                  decoration: const BoxDecoration(
                    color: AppColors.primarySoft,
                    shape: BoxShape.circle,
                  ),
                  child: SizedBox.square(
                    dimension: 40,
                    child: Center(
                      child: HkIcon(
                        icon,
                        size: 22,
                        color: AppIconColors.secondary,
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        label,
                        style: const TextStyle(
                          color: AppColors.textEmphasis,
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      if (description != null) ...[
                        const SizedBox(height: HkSpace.xs),
                        Text(
                          description!,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: AppColors.textSecondary,
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                if (badgeCount > 0) ...[
                  const SizedBox(width: HkSpace.sm),
                  _OptionBadge(count: badgeCount),
                ],
                if (statusLabel != null) ...[
                  const SizedBox(width: HkSpace.sm),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: HkSpace.sm,
                      vertical: HkSpace.xs,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.primarySoft,
                      borderRadius: BorderRadius.circular(HkRadius.pill),
                    ),
                    child: Text(
                      statusLabel!,
                      style: const TextStyle(
                        color: AppColors.textAccentOnTint,
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ],
                const SizedBox(width: HkSpace.sm),
                const HkIcon(
                  HkIconName.chevronRight,
                  size: 20,
                  color: AppIconColors.muted,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _OptionBadge extends StatelessWidget {
  const _OptionBadge({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    final label = count > 99 ? '99+' : count.toString();
    return Semantics(
      label: '$label 則未讀通知',
      child: ExcludeSemantics(
        child: DecoratedBox(
          decoration: const BoxDecoration(
            color: AppColors.error,
            shape: BoxShape.circle,
          ),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minWidth: 24, minHeight: 24),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
              child: Center(
                child: Text(
                  label,
                  style: const TextStyle(
                    color: AppColors.textInverse,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    height: 1,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
