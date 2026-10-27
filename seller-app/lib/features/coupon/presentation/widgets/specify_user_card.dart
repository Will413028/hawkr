import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

import 'section_card.dart';
import 'selected_users_list.dart';

/// 以手機號碼查找並加入指定發放名單。
class CouponSpecifyUserCard extends StatelessWidget {
  const CouponSpecifyUserCard({
    super.key,
    required this.phoneSearchCtrl,
    required this.selectedUsers,
    required this.onAddUser,
    required this.onRemoveUser,
    this.isLoading = false,
    this.errorMessage,
  });

  final TextEditingController phoneSearchCtrl;
  final List<Map<String, String>> selectedUsers;
  final VoidCallback onAddUser;
  final ValueChanged<int> onRemoveUser;
  final bool isLoading;
  final String? errorMessage;

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
                          ? '查找會員並加入這次發放名單'
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
          const Text(
            '輸入會員手機號碼',
            style: TextStyle(
              color: AppColors.textEmphasis,
              fontSize: 14,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: HkSpace.sm),
          LayoutBuilder(
            builder: (context, constraints) {
              final stack = constraints.maxWidth < 320;
              final input = TextField(
                controller: phoneSearchCtrl,
                keyboardType: TextInputType.phone,
                textInputAction: TextInputAction.search,
                onSubmitted: (_) {
                  if (!isLoading) onAddUser();
                },
                decoration: InputDecoration(
                  hintText: '09xx xxx xxx',
                  filled: true,
                  fillColor: AppColors.surfacePage,
                  prefixIcon: const Padding(
                    padding: EdgeInsets.all(14),
                    child: HkIcon(
                      HkIconName.search,
                      size: 20,
                      color: AppIconColors.muted,
                    ),
                  ),
                  border: _border(AppColors.borderInput),
                  enabledBorder: _border(AppColors.borderInput),
                  focusedBorder: _border(
                    AppColors.brandPrimaryStrong,
                    width: 1.5,
                  ),
                ),
              );
              final action = SizedBox(
                height: 52,
                child: FilledButton.icon(
                  onPressed: isLoading ? null : onAddUser,
                  icon: isLoading
                      ? const SizedBox.square(
                          dimension: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const HkIcon(HkIconName.add, size: 19),
                  label: HkButtonLabel(isLoading ? '查詢中' : '加入名單'),
                ),
              );
              if (stack) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    input,
                    const SizedBox(height: HkSpace.sm),
                    action,
                  ],
                );
              }
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: input),
                  const SizedBox(width: HkSpace.sm),
                  action,
                ],
              );
            },
          ),
          if (errorMessage != null) ...[
            const SizedBox(height: HkSpace.sm),
            Semantics(
              liveRegion: true,
              label: errorMessage,
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const HkIcon(
                    HkIconName.warning,
                    size: 18,
                    color: AppIconColors.danger,
                  ),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: Text(
                      errorMessage!,
                      style: const TextStyle(
                        color: AppColors.danger,
                        fontSize: 12,
                        height: 1.4,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
          CouponSelectedUsersList(
            users: selectedUsers,
            onRemove: onRemoveUser,
            idKey: 'accountId',
          ),
        ],
      ),
    );
  }

  static OutlineInputBorder _border(Color color, {double width = 1}) {
    return OutlineInputBorder(
      borderRadius: BorderRadius.circular(HkRadius.md),
      borderSide: BorderSide(color: color, width: width),
    );
  }
}
