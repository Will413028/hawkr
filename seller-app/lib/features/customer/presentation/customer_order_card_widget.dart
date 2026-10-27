import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';

import '../data/customer_model.dart';

/// 訂單卡片元件 — Figma: 5324:61151
///
/// 顯示：訂單編號、日期、購買品項、金額、評分、評論
/// 點擊進入訂單詳情頁
///
/// Layout (p-16, column gap-6):
/// - Row1: #orderId (16sp Bold #565656) + date (14sp Regular gray80) — spaceBetween, h-24, w-293
/// - Row2: items (16sp Regular #565656 w-215) + $amount (16sp Bold primary) — spaceBetween, h-24, w-293
/// - Row3: star icons (16px, gap-4)
/// - Row4: review text (14sp Regular #565656, h-20)
/// - Shield badge: top-right, 40w x 42h, rounded-bl/br-30
class CustomerOrderCardWidget extends StatelessWidget {
  final OrderRecord order;
  final String customerName;
  final VoidCallback? onTap;

  const CustomerOrderCardWidget({
    super.key,
    required this.order,
    required this.customerName,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final (badgeColor, badgeIcon) = _badgeConfig(order.status);

    return GestureDetector(
      onTap:
          onTap ??
          () => context.push(
            '/customer/order-detail',
            extra: {'order': order, 'customerName': customerName},
          ),
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          // Card body
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(16.w),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(16.r),
              border: Border.all(
                color: AppColors.textPrimary.withValues(alpha: 0.2),
              ),
              boxShadow: const [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 2,
                  offset: Offset(0, 1),
                ),
              ],
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Row 1: #orderId + date — w-293 (card minus badge area)
                Padding(
                  padding: EdgeInsets.only(right: 48.w),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Flexible(
                        child: Text(
                          '#${order.orderId}',
                          style: TextStyle(
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                            height: 1.5, // lineHeight 24
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      SizedBox(width: 8.w),
                      Text(
                        order.date,
                        style: TextStyle(
                          fontSize: 14.sp,
                          fontWeight: FontWeight.w400,
                          color: AppColors.textPrimary.withValues(alpha: 0.8),
                          height: 1.43, // lineHeight 20
                        ),
                      ),
                    ],
                  ),
                ),
                SizedBox(height: 6.h),
                // Row 2: items + $amount — w-293
                Padding(
                  padding: EdgeInsets.only(right: 48.w),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Flexible(
                        child: Text(
                          order.items,
                          style: TextStyle(
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w400,
                            color: AppColors.textPrimary,
                            height: 1.5,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      SizedBox(width: 8.w),
                      Text(
                        '\$${_fmt(order.amount)}',
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.primary,
                          height: 1.5,
                        ),
                      ),
                    ],
                  ),
                ),
                SizedBox(height: 6.h),
                // Row 3: star rating — full width w-338
                _StarRating(rating: order.rating),
                SizedBox(height: 6.h),
                // Row 4: review text — full width w-338
                SizedBox(
                  height: 20.h,
                  child: Text(
                    order.review.isNotEmpty ? order.review : '尚無評論',
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w400,
                      color: order.review.isNotEmpty
                          ? AppColors.textPrimary
                          : AppColors.textPrimary.withValues(alpha: 0.4),
                      height: 1.43,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
          // Shield badge — top-right
          Positioned(
            top: -5.h,
            right: 16.w,
            child: _ShieldBadge(color: badgeColor, icon: badgeIcon),
          ),
        ],
      ),
    );
  }

  static (Color, IconData) _badgeConfig(OrderStatus status) {
    return switch (status) {
      OrderStatus.delivering => (
        AppColors.warning,
        AppIcons.localShippingOutlined,
      ),
      OrderStatus.completed => (
        AppColors.brandPrimaryStrong,
        AppIcons.verifiedOutlined,
      ),
      OrderStatus.cancelled => (
        AppColors.textSecondary,
        AppIcons.cancelOutlined,
      ),
      OrderStatus.issue => (AppColors.error, AppIcons.campaignOutlined),
    };
  }
}

/// Star rating row — 16px star icons, gap-4, filled = golden, empty = grey outline
class _StarRating extends StatelessWidget {
  final int rating;

  const _StarRating({required this.rating});

  @override
  Widget build(BuildContext context) {
    // Clamp rating 0-5, show up to 5 stars
    final filled = rating.clamp(0, 5);

    return Row(
      children: List.generate(5, (i) {
        return Padding(
          padding: EdgeInsets.only(right: i < 4 ? 4.w : 0),
          child: Icon(
            i < filled ? AppIcons.star : AppIcons.starBorder,
            size: 16.sp,
            color: i < filled
                ? AppIconColors.gold
                : AppIconColors.defaultColor.withValues(alpha: 0.3),
          ),
        );
      }),
    );
  }
}

/// Shield badge — Figma: 40w x 42h, rounded-bl-30 rounded-br-30
/// Shadow: 0 1 3 rgba(0,0,0,0.5)
class _ShieldBadge extends StatelessWidget {
  final Color color;
  final IconData icon;

  const _ShieldBadge({required this.color, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 40.w,
      height: 42.h,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.only(
          bottomLeft: Radius.circular(32.r),
          bottomRight: Radius.circular(32.r),
        ),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 3,
            offset: Offset(0, 1),
          ),
        ],
      ),
      child: Center(
        child: Padding(
          padding: EdgeInsets.only(top: 4.h),
          child: Icon(icon, size: 20.sp, color: AppIconColors.onBrand),
        ),
      ),
    );
  }
}

String _fmt(int number) {
  final str = number.toString();
  final buffer = StringBuffer();
  int count = 0;
  for (int i = str.length - 1; i >= 0; i--) {
    buffer.write(str[i]);
    count++;
    if (count % 3 == 0 && i != 0) {
      buffer.write(',');
    }
  }
  return buffer.toString().split('').reversed.join();
}
