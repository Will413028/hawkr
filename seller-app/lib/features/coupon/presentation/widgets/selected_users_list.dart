import 'package:hawkr_seller/config/image_config.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/optimized_image.dart';
import 'package:flutter/material.dart';

/// 已選會員列表，供新增與進階優惠券流程共用。
class CouponSelectedUsersList extends StatelessWidget {
  const CouponSelectedUsersList({
    super.key,
    required this.users,
    required this.onRemove,
    this.idKey = 'id',
    this.themeColor = AppColors.primary,
  });

  final List<Map<String, String>> users;
  final ValueChanged<int> onRemove;
  final String idKey;
  final Color themeColor;

  @override
  Widget build(BuildContext context) {
    if (users.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: HkSpace.xl),
        Text(
          '已選擇 ${users.length} 位會員',
          style: const TextStyle(
            color: AppColors.textEmphasis,
            fontSize: 14,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: HkSpace.sm),
        ...users.asMap().entries.map((entry) {
          final index = entry.key;
          final user = entry.value;
          final id = user[idKey] ?? user['id'] ?? '';
          final phone = user['phone'] ?? '';
          final name = user['name'] ?? '會員';
          final avatarUrl = user['avatarUrl'] ?? '';
          return Container(
            margin: const EdgeInsets.only(bottom: HkSpace.sm),
            padding: const EdgeInsets.fromLTRB(
              HkSpace.md,
              HkSpace.sm,
              HkSpace.xs,
              HkSpace.sm,
            ),
            decoration: BoxDecoration(
              color: AppColors.surfacePage,
              borderRadius: BorderRadius.circular(HkRadius.md),
              border: Border.all(color: AppColors.cardBorder),
            ),
            child: Row(
              children: [
                _MemberAvatar(
                  name: name,
                  avatarUrl: avatarUrl,
                  themeColor: themeColor,
                ),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: AppColors.textEmphasis,
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      if (id.isNotEmpty || phone.isNotEmpty) ...[
                        const SizedBox(height: 2),
                        Text(
                          [
                            if (id.isNotEmpty) 'ID $id',
                            if (phone.isNotEmpty) phone,
                          ].join(' · '),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: AppColors.textMuted,
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                Semantics(
                  button: true,
                  label: '從名單移除 $name',
                  child: IconButton(
                    tooltip: '移除會員',
                    onPressed: () => onRemove(index),
                    icon: const HkIcon(
                      HkIconName.close,
                      size: 18,
                      color: AppIconColors.muted,
                    ),
                  ),
                ),
              ],
            ),
          );
        }),
      ],
    );
  }
}

class _MemberAvatar extends StatelessWidget {
  const _MemberAvatar({
    required this.name,
    required this.avatarUrl,
    required this.themeColor,
  });

  final String name;
  final String avatarUrl;
  final Color themeColor;

  @override
  Widget build(BuildContext context) {
    final initial = Center(
      child: Text(
        name.isNotEmpty ? name[0] : '?',
        style: TextStyle(
          color: themeColor,
          fontSize: 16,
          fontWeight: FontWeight.w900,
        ),
      ),
    );
    return Container(
      width: 40,
      height: 40,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: themeColor.withValues(alpha: 0.10),
        shape: BoxShape.circle,
      ),
      child: avatarUrl.startsWith('http')
          ? OptimizedNetworkImage(
              imageUrl: avatarUrl,
              scene: ImageScene.avatarSmall,
              fit: BoxFit.cover,
              width: 40,
              height: 40,
              placeholder: initial,
              errorWidget: initial,
            )
          : initial,
    );
  }
}
