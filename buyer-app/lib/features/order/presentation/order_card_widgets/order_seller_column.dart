import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../config/image_config.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../data/order_model.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

/// 訂單卡片左側 — 賣家圓形頭像。
class OrderSellerColumn extends StatelessWidget {
  const OrderSellerColumn({super.key, required this.order});
  final Order order;

  static final _avatarSize = 48.w;

  @override
  Widget build(BuildContext context) {
    return ClipOval(
      child: order.sellerAvatarUrl.isNotEmpty
          ? OssImage(
              imageSource: order.sellerAvatarUrl,
              scene: ImageScene.merchantAvatar,
              width: _avatarSize,
              height: _avatarSize,
              fit: BoxFit.cover,
            )
          : SizedBox(
              width: _avatarSize,
              height: _avatarSize,
              child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
            ),
    );
  }
}
