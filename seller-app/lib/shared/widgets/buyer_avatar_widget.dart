import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import 'optimized_image.dart';
import 'hk_placeholder.dart';

/// 買家大頭貼元件
///
/// 傳入 [imageUrl] 顯示買家頭像，若無頭像則顯示名字首字。
///
/// 使用方式：
/// ```dart
/// BuyerAvatarWidget(size: 80, imageUrl: order.buyerAvatar, name: order.buyerName)
/// BuyerAvatarWidget(size: 40, name: '哭哭饅頭')  // 無圖 → 顯示「哭」
/// ```
class BuyerAvatarWidget extends StatelessWidget {
  const BuyerAvatarWidget({
    super.key,
    this.size = 80,
    this.imageUrl,
    this.name,
    this.borderColor = AppColors.dividerLight,
    this.backgroundColor = AppColors.gray100,
    this.initialColor = AppColors.primary,
  });

  final double size;
  final String? imageUrl;
  final String? name;
  final Color borderColor;
  final Color backgroundColor;
  final Color initialColor;

  @override
  Widget build(BuildContext context) {
    final hasUrl =
        imageUrl != null &&
        imageUrl!.isNotEmpty &&
        imageUrl!.startsWith('http');

    return Container(
      width: size.w,
      height: size.w,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: backgroundColor,
        border: Border.all(color: borderColor, width: 1),
      ),
      clipBehavior: Clip.antiAlias,
      child: hasUrl
          ? ClipOval(
              child: OptimizedNetworkImage(
                imageUrl: imageUrl!,
                scene: ImageScene.avatarLarge,
                fit: BoxFit.cover,
                width: size.w,
                height: size.w,
                placeholder: _buildDefaultAvatar(),
                errorWidget: _buildDefaultAvatar(),
              ),
            )
          : _buildDefaultAvatar(),
    );
  }

  Widget _buildDefaultAvatar() {
    return SizedBox.square(
      dimension: size.w,
      child: const ClipOval(child: HkPlaceholder(HkPlaceholderKind.avatar)),
    );
  }
}
