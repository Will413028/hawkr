import 'package:flutter/material.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';

enum StickerBadgeShape { circle, label }

/// 貼圖標籤：以色塊、圖示與文字畫出，匯出圖片時隨圖層一起合成。
enum StickerBadge {
  naturalDrop(
    label: '天然',
    icon: AppIcons.waterDropOutlined,
    background: AppColors.info,
    shape: StickerBadgeShape.circle,
  ),
  naturalWave(
    label: '天然',
    icon: AppIcons.waves,
    background: AppColors.info,
    shape: StickerBadgeShape.label,
  ),
  freshPicked(
    label: '新鮮採摘',
    icon: AppIcons.localFloristOutlined,
    background: AppColors.success,
    shape: StickerBadgeShape.label,
  ),
  localDelivery(
    label: '在地直送',
    icon: AppIcons.localShippingOutlined,
    background: AppColors.error,
    shape: StickerBadgeShape.circle,
  ),
  fresh(
    label: '新鮮',
    icon: AppIcons.autoAwesomeOutlined,
    background: AppColors.success,
    shape: StickerBadgeShape.circle,
  ),
  quality(
    label: '優質',
    icon: AppIcons.workspacePremiumOutlined,
    background: AppColors.brandSecondary,
    foreground: AppColors.brandInk,
    shape: StickerBadgeShape.circle,
  ),
  premium(
    label: '頂級',
    icon: AppIcons.starOutline,
    background: AppColors.brandSecondary,
    foreground: AppColors.brandInk,
    shape: StickerBadgeShape.label,
  ),
  limited(
    label: '限量',
    icon: AppIcons.hourglassTop,
    background: AppColors.brandPrimaryStrong,
    shape: StickerBadgeShape.circle,
  );

  const StickerBadge({
    required this.label,
    required this.icon,
    required this.background,
    required this.shape,
    this.foreground = AppColors.neutralWhite,
  });

  final String label;
  final IconData icon;
  final Color background;
  final Color foreground;
  final StickerBadgeShape shape;
}

class StickerBadgeView extends StatelessWidget {
  const StickerBadgeView(this.badge, {super.key, required this.size});

  final StickerBadge badge;
  final double size;

  @override
  Widget build(BuildContext context) {
    return SizedBox.square(
      dimension: size,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: badge.background,
          shape: badge.shape == StickerBadgeShape.circle
              ? BoxShape.circle
              : BoxShape.rectangle,
          borderRadius: badge.shape == StickerBadgeShape.label
              ? BorderRadius.circular(size * 0.22)
              : null,
          border: Border.all(color: AppColors.neutralWhite, width: size * 0.04),
        ),
        child: Center(
          child: SizedBox(
            width: size * 0.76,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(badge.icon, size: size * 0.26, color: badge.foreground),
                Text(
                  badge.label,
                  maxLines: 2,
                  overflow: TextOverflow.clip,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: size * 0.2,
                    fontWeight: FontWeight.w700,
                    color: badge.foreground,
                    height: 1.1,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
