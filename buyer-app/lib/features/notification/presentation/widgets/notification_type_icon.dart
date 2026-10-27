import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/icons/app_icons.dart';

/// 通知類別。決定 [NotificationTypeIcon] 的圖示與圓底色。
enum NotificationIconKind {
  order(AppIcons.shoppingBagOutlined, AppColors.warning),
  refund(AppIcons.assignmentReturnOutlined, AppColors.danger),
  chat(AppIcons.chatBubbleOutline, AppColors.textPrimary),
  post(AppIcons.descriptionOutlined, AppColors.primary),
  message(AppIcons.emailOutlined, AppColors.primary),
  user(AppIcons.personOutline, AppColors.primary),
  coupon(AppIcons.confirmationNumberOutlined, AppColors.danger),
  system(AppIcons.notificationsOutlined, AppColors.textPrimary);

  const NotificationIconKind(this.icon, this.color);

  final IconData icon;
  final Color color;
}

/// 通知類別的圓形圖示：彩色圓底加白色 Material 圖示。
class NotificationTypeIcon extends StatelessWidget {
  const NotificationTypeIcon({super.key, required this.kind, this.size = 32});

  final NotificationIconKind kind;
  final double size;

  @override
  Widget build(BuildContext context) {
    return ExcludeSemantics(
      child: SizedBox.square(
        dimension: size,
        child: DecoratedBox(
          decoration: BoxDecoration(shape: BoxShape.circle, color: kind.color),
          child: Icon(
            kind.icon,
            size: size * 0.55,
            color: AppIconColors.onBrand,
          ),
        ),
      ),
    );
  }
}
