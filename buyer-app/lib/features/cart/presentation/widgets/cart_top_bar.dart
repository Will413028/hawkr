import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/common_top_bar.dart';
import 'package:flutter/material.dart';

import '../cart_typography.dart';

/// Keeps the cart's 22px title clear of the existing 48px navigation actions.
class CartTopBar extends StatelessWidget {
  const CartTopBar({
    super.key,
    required this.onBack,
    required this.chatAction,
    this.onClear,
  });

  final VoidCallback onBack;
  final VoidCallback? onClear;
  final Widget chatAction;

  @override
  Widget build(BuildContext context) => commonTopBar(
    context: context,
    horizontalPadding: const EdgeInsets.symmetric(horizontal: HkSpace.sm),
    child: NavigationToolbar(
      centerMiddle: true,
      middleSpacing: HkSpace.xs,
      leading: IconButton(
        icon: Icon(
          AppIcons.arrowBackIosNew,
          color: AppIconColors.strong,
          size: HkControl.icon,
        ),
        tooltip: '返回',
        onPressed: onBack,
      ),
      middle: Text(
        '購物車',
        style: CartTypography.navigationTitle.copyWith(
          color: AppColors.chromeForeground,
        ),
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (onClear != null)
            IconButton(
              icon: const Icon(AppIcons.deleteOutline, size: HkControl.icon),
              tooltip: '清空購物車',
              onPressed: onClear,
            ),
          chatAction,
        ],
      ),
    ),
  );
}
