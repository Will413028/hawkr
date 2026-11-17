import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import 'optimized_image.dart';

/// 導購商品連結卡片 (根據 Figma 4075:59542 設計)
///
/// 顯示商品的 [name]、[price]（新台幣整數）與 [imageUrl]（空字串是沒有圖）；name 為空時回傳空 Widget。
class LinkedProductCard extends StatelessWidget {
  const LinkedProductCard({
    super.key,
    required this.name,
    required this.price,
    this.imageUrl = '',
    this.onTap,
    this.inset = true,
  });

  final String name;
  final int price;
  final String imageUrl;
  final VoidCallback? onTap;

  /// Keeps the legacy outer margin and standalone card chrome by default.
  ///
  /// Set to false when the product belongs to a parent's single continuous
  /// surface. The embedded variant removes its own background, border and
  /// shadow as well as the legacy outer margin.
  final bool inset;

  @override
  Widget build(BuildContext context) {
    if (name.isEmpty) return const SizedBox.shrink();

    final embedded = !inset;
    final radius = BorderRadius.circular(HkRadius.lg);
    final thumbnailSize = embedded ? HkControl.minTap : 50.0;
    final trailingSize = embedded ? HkSpace.large : 36.0;
    final trailingIconSize = embedded ? HkControl.icon : 20.0;

    Widget content = ConstrainedBox(
      constraints: BoxConstraints(minHeight: embedded ? HkControl.minTap : 74),
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: embedded ? 0 : HkSpace.lg,
          vertical: embedded ? HkSpace.sm : HkSpace.xs,
        ),
        child: Row(
          children: [
            Container(
              width: thumbnailSize,
              height: thumbnailSize,
              decoration: BoxDecoration(
                color: AppColors.gray100,
                borderRadius: BorderRadius.circular(HkRadius.sm),
              ),
              clipBehavior: Clip.antiAlias,
              child: imageUrl.isNotEmpty
                  ? OptimizedNetworkImage(
                      imageUrl: imageUrl,
                      scene: ImageScene.productThumbnail,
                      width: thumbnailSize,
                      height: thumbnailSize,
                      fit: BoxFit.cover,
                      errorWidget: const Icon(
                        AppIcons.imageOutlined,
                        color: AppIconColors.muted,
                      ),
                    )
                  : const Icon(
                      AppIcons.imageOutlined,
                      color: AppIconColors.muted,
                    ),
            ),
            const SizedBox(width: HkSpace.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(
                    name,
                    style: TextStyle(
                      fontFamily: 'Roboto',
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                    maxLines: embedded ? 2 : 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    'NT\$ $price',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontFamily: 'Roboto',
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.warning,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            SizedBox.square(
              dimension: trailingSize,
              child: DecoratedBox(
                decoration: const BoxDecoration(
                  color: AppColors.primary,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  AppIcons.storefrontOutlined,
                  size: trailingIconSize,
                  color: AppIconColors.onBrand,
                ),
              ),
            ),
          ],
        ),
      ),
    );

    if (onTap != null) {
      content = Semantics(
        container: true,
        button: true,
        enabled: true,
        label: '查看商品，$name，NT\$ $price',
        onTap: onTap,
        child: ExcludeSemantics(
          child: Material(
            type: MaterialType.transparency,
            child: InkWell(
              onTap: onTap,
              excludeFromSemantics: true,
              borderRadius: radius,
              child: content,
            ),
          ),
        ),
      );
    }

    if (!embedded) {
      content = DecoratedBox(
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: radius,
          border: Border.all(
            color: AppColors.textPrimary.withValues(alpha: 0.2),
          ),
          boxShadow: const [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 3,
              offset: Offset(0, 1),
            ),
          ],
        ),
        child: ClipRRect(borderRadius: radius, child: content),
      );
    }

    // ── 買家版關聯商品卡片樣式（hawkr_buyer post_detail_screen.buildProductCard）──
    if (!inset) return content;
    return Padding(
      padding: const EdgeInsets.fromLTRB(HkSpace.lg, HkSpace.md, HkSpace.lg, 0),
      child: content,
    );
  }
}
