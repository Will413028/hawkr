import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import 'seller_avatar_widget.dart';

/// 店舖品牌 Hero 共用的大頭貼細框。
///
/// 外徑由使用端決定；[ringWidth] 只保留足以分隔封面與頭像的細邊，
/// 避免設定頁與店舖頁各自疊加 margin / border 形成過厚白圈。
class SellerIdentityAvatarFrame extends StatelessWidget {
  const SellerIdentityAvatarFrame({
    super.key,
    this.size = 88,
    this.ringWidth = 2,
    this.imageUrl,
    this.frameKey,
  }) : assert(size > ringWidth * 2);

  final double size;
  final double ringWidth;
  final String? imageUrl;
  final Key? frameKey;

  @override
  Widget build(BuildContext context) {
    final imageSize = size - ringWidth * 2;

    return SizedBox.square(
      key: frameKey,
      dimension: size,
      child: DecoratedBox(
        decoration: const BoxDecoration(
          color: AppColors.surfaceCard,
          shape: BoxShape.circle,
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 8,
              offset: Offset(0, 4),
            ),
          ],
        ),
        child: Padding(
          padding: EdgeInsets.all(ringWidth),
          child: ClipOval(
            child: SellerAvatarWidget(
              size: imageSize,
              isCircle: true,
              imageUrl: imageUrl,
            ),
          ),
        ),
      ),
    );
  }
}
