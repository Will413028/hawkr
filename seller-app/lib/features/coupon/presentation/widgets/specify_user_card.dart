import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

import 'section_card.dart';
import 'selected_users_list.dart';

/// 從客戶名單挑選並加入指定發放名單。
class CouponSpecifyUserCard extends StatelessWidget {
  const CouponSpecifyUserCard({
    super.key,
    required this.selectedUsers,
    required this.onPickUsers,
    required this.onRemoveUser,
  });

  final List<Map<String, String>> selectedUsers;
  final VoidCallback onPickUsers;
  final ValueChanged<int> onRemoveUser;

  @override
  Widget build(BuildContext context) {
    final selectedCount = selectedUsers.length;

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(HkRadius.md),
                ),
                child: const Center(
                  child: HkIcon(
                    HkIconName.user,
                    size: 20,
                    color: AppIconColors.active,
                  ),
                ),
              ),
              const SizedBox(width: HkSpace.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      '建立指定名單',
                      style: TextStyle(
                        color: AppColors.textEmphasis,
                        fontSize: 16,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: HkSpace.xs),
                    Text(
                      selectedCount == 0
                          ? context.l10n.couponPickFromCustomersHint
                          : '已選擇 $selectedCount 位會員',
                      style: const TextStyle(
                        color: AppColors.textMuted,
                        fontSize: 14,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: HkSpace.xl),
          SizedBox(
            width: double.infinity,
            height: 52,
            child: FilledButton.icon(
              key: const ValueKey('coupon-pick-customers'),
              onPressed: onPickUsers,
              icon: const HkIcon(HkIconName.add, size: 19),
              label: HkButtonLabel(context.l10n.couponPickFromCustomers),
            ),
          ),
          CouponSelectedUsersList(
            users: selectedUsers,
            onRemove: onRemoveUser,
            idKey: 'accountId',
          ),
        ],
      ),
    );
  }
}
