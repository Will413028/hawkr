import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../address/data/address_model.dart';

// ---------------------------------------------------------------------------
// 收件地址選擇底部彈窗。
//
// 以 Radio List 形式展示使用者的所有收件地址，
// 底部提供「新增收件地址」按鈕。選取後回傳地址 ID。
// ---------------------------------------------------------------------------

/// 顯示收件地址選擇器。
///
/// [context] 顯示彈窗的上下文。
/// [userAddresses] 使用者的地址列表。
/// [selectedAddressId] 當前已選的地址 ID。
/// [onAddAddress] 點擊「新增收件地址」後的回調。
///
/// 回傳選取的地址 ID，或 null（取消選擇）。
Future<int?> showAddressPickerSheet({
  required BuildContext context,
  required List<Address> userAddresses,
  required int? selectedAddressId,
  required VoidCallback onAddAddress,
}) async {
  if (userAddresses.isEmpty) {
    onAddAddress();
    return null;
  }

  return showModalBottomSheet<int>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(HkRadius.lg)),
    ),
    builder: (ctx) {
      int? localSelected = selectedAddressId;
      return StatefulBuilder(
        builder: (ctx2, setSB) {
          final media = MediaQuery.of(ctx2);
          final availableHeight =
              media.size.height - media.padding.top - media.padding.bottom;
          return ConstrainedBox(
            constraints: BoxConstraints(maxHeight: availableHeight * 0.9),
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.lg),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // 標題
                  Text('選擇收件地址', style: CartTypography.section),
                  const SizedBox(height: HkSpace.md),

                  Flexible(
                    child: ListView.builder(
                      key: const ValueKey('checkout-address-picker-scroll'),
                      shrinkWrap: true,
                      itemCount: userAddresses.length,
                      itemBuilder: (context, index) {
                        final a = userAddresses[index];
                        final id = a.id;
                        final isSelected = id == localSelected;
                        final fullAddress = a.fullAddress;
                        final name = a.receiverName;
                        final phone = a.receiverPhone;

                        return InkWell(
                          onTap: () {
                            setSB(() => localSelected = id);
                            Navigator.pop(ctx2, id);
                          },
                          child: Container(
                            constraints: const BoxConstraints(
                              minHeight: HkControl.minTap,
                            ),
                            padding: const EdgeInsets.symmetric(
                              vertical: HkSpace.sm,
                            ),
                            child: Row(
                              children: [
                                Icon(
                                  isSelected
                                      ? AppIcons.radioButtonChecked
                                      : AppIcons.radioButtonUnchecked,
                                  size: HkControl.icon,
                                  color: isSelected
                                      ? AppIconColors.secondary
                                      : AppIconColors.disabled,
                                ),
                                const SizedBox(width: HkSpace.sm),
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        fullAddress,
                                        style: CartTypography.subtitle,
                                        maxLines: 2,
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                      if (name.isNotEmpty || phone.isNotEmpty)
                                        Text(
                                          '$name  $phone',
                                          style: CartTypography.supportMuted,
                                        ),
                                    ],
                                  ),
                                ),
                              ],
                            ),
                          ),
                        );
                      },
                    ),
                  ),
                  const SizedBox(height: HkSpace.md),

                  // 新增地址按鈕
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      key: const ValueKey('checkout-address-picker-add'),
                      icon: const Icon(AppIcons.add),
                      label: const HkButtonLabel('新增收件地址'),
                      style: OutlinedButton.styleFrom(
                        textStyle: CartTypography.button,
                        foregroundColor: AppColors.primary,
                        minimumSize: const Size(0, HkControl.minTap),
                        side: const BorderSide(color: AppColors.primary),
                      ),
                      onPressed: () {
                        Navigator.pop(ctx2);
                        onAddAddress();
                      },
                    ),
                  ),
                  const SizedBox(height: HkSpace.lg),
                ],
              ),
            ),
          );
        },
      );
    },
  );
}
