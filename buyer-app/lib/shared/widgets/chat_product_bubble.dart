import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import 'app_cached_image.dart';
import '../utils/price_display.dart';

/// 聊天訊息中的商品卡片泡泡
///
/// 精確對齊 Figma：
/// - 送出（isSelf=true）: node `10022:101654`
///   bg primary, border #56565620, rounded 14, shadow 0 1 2 10%
///   image 50×50 rounded 8 + border 2px #F3F3F5
///   name 14sp bold #F3F3F5, price 16sp bold #F3F3F5
///
/// - 收到（isSelf=false）: node `10022:101644`
///   bg white, border #56565620, rounded 14, shadow 0 1 2 10%
///   image 50×50 rounded 8 (no border)
///   name 14sp bold #565656, price 16sp bold #EE964B (orange)
class ChatProductBubble extends StatelessWidget {
  const ChatProductBubble({
    super.key,
    required this.messageContent,
    required this.isSelf,
    this.onTap,
  });

  final String messageContent;
  final bool isSelf;
  final VoidCallback? onTap;

  /// 嘗試解析 messageContent JSON 為商品資料。
  static Map<String, dynamic>? tryParseProduct(String content) {
    try {
      final parsed = json.decode(content);
      if (parsed is Map<String, dynamic>) {
        final type = (parsed['messageType'] ?? parsed['type'] ?? '').toString();
        if (type == 'product') return parsed;
        // 有 productName 或 skuName 的也算
        if (parsed.containsKey('productName') ||
            parsed.containsKey('skuName')) {
          return parsed;
        }
      }
    } catch (_) {}
    return null;
  }

  // ── 顏色 ──
  static const _greenBg = AppColors.primary;
  static const _selfText = AppColors.gray100; // --輸入框底色
  static const _recvName = AppColors.textPrimary; // --brand/primary/gray1/100
  static const _recvPrice = AppColors.warning; // --brand/sencoy/orange/100
  static const _borderColor = AppColors.cardBorder; // --外框線
  static const _imageBg = AppColors.gray100;
  static const _imageBorder = AppColors.gray100; // --輸入框底色

  @override
  Widget build(BuildContext context) {
    final product = tryParseProduct(messageContent);
    if (product == null) return const SizedBox.shrink();

    final name =
        (product['productName'] ?? product['skuName'] ?? product['name'] ?? '')
            .toString();
    // 判準走 PriceDisplay：`"0"` = 無價格，不能印成 `NT$ 0`。
    final price = PriceDisplay.formatPrice(
      product['price'] ?? product['unitPrice'] ?? product['amount'],
      prefix: 'NT\$ ',
    );
    final imageUrl =
        (product['imageKey'] ?? product['imageUrl'] ?? product['image'] ?? '')
            .toString();

    final nameColor = isSelf ? _selfText : _recvName;
    final priceColor = isSelf ? _selfText : _recvPrice;

    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 300.w,
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: isSelf ? _greenBg : AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(color: _borderColor),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 2,
              offset: Offset(0, 1.h),
            ),
          ],
        ),
        child: Row(
          children: [
            // Image 50×50, rounded 8
            Container(
              width: 50.w,
              height: 50.w,
              decoration: BoxDecoration(
                color: _imageBg,
                borderRadius: BorderRadius.circular(8.r),
                border: isSelf
                    ? Border.all(color: _imageBorder, width: 2)
                    : null,
              ),
              clipBehavior: Clip.antiAlias,
              child: imageUrl.isNotEmpty
                  ? AppCachedImage(
                      imageUrl: imageUrl,
                      scene: ImageScene.productThumbnail,
                      fit: BoxFit.cover,
                      width: 50.w,
                      height: 50.w,
                      errorWidget: Icon(
                        AppIcons.imageOutlined,
                        size: 24.sp,
                        color: AppIconColors.disabled,
                      ),
                    )
                  : Icon(
                      AppIcons.imageOutlined,
                      size: 24.sp,
                      color: AppIconColors.disabled,
                    ),
            ),
            SizedBox(width: 12.w),
            // Name + Price
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  if (name.isNotEmpty)
                    Text(
                      name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                        color: nameColor,
                        height: 20 / 14,
                      ),
                    ),
                  SizedBox(height: 4.h),
                  if (price.isNotEmpty)
                    Text(
                      price,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: priceColor,
                        height: 24 / 16,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
