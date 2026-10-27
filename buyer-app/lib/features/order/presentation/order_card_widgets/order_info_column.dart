import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    hide OrderStatus, OrderItemStatus;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/utils/date_format_utils.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/hk_tap_target.dart';
import '../../data/order_model.dart';
import '../../order_color_tokens.dart';

// 文字色：基底 #565656，最淺不低於 80% 透明度
// 80% 在白底實際渲染 ≈ #797979（偏淡），改用 100% / 90% 讓視覺貼近實際 #565656
const Color _kTextPrimary = AppColors.textPrimary; // 100% #565656
const Color _kTextMuted = AppColors.textPrimary; // 100% #565656（避免在白底渲染過淡）

/// 訂單卡片收合資訊區。
///
/// 收合狀態只保留店家、完整配送地址，以及待付款時真正需要注意的倒數。
/// 下單時間、訂單編號與操作移至展開區，物流商則集中在物流進度列，避免
/// 同一行同時競爭地址寬度。
class OrderInfoColumn extends StatelessWidget {
  const OrderInfoColumn({
    super.key,
    required this.order,
    required this.isPendingPayment,
  });

  final Order order;
  final bool isPendingPayment;

  String _buildFullAddress() {
    // 7-11 店到店訂單：優先顯示門市名稱（pickupStoreName）
    if (order.shippingMethod == ShippingMethod.sevenElevenStorePickup.value &&
        order.pickupStoreName.isNotEmpty) {
      final name = order.pickupStoreName;
      return name.startsWith('7-') ? name : '7-11 $name';
    }
    if (order.shippingAddress.isNotEmpty) return order.shippingAddress;
    final parts = <String>[];
    if (order.sellerCity.isNotEmpty) parts.add(order.sellerCity);
    if (order.sellerDistrict.isNotEmpty) parts.add(order.sellerDistrict);
    return parts.join('');
  }

  @override
  Widget build(BuildContext context) {
    final address = _buildFullAddress();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 賣家名稱（截斷）— 14sp Regular #565656 lineHeight 24
        Text(
          order.sellerName.isNotEmpty ? order.sellerName : '賣家',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: AppTypography.body.copyWith(
            color: _kTextPrimary,
            height: 24 / 14,
          ),
        ),

        if (isPendingPayment)
          Builder(
            builder: (_) {
              // B-185：用 paymentDeadline 算實際倒數，不再用 statusTime。
              final countdown = order.paymentCountdownLabel;
              final hasDeadline = countdown.isNotEmpty;
              final urgent = order.paymentDeadlineUrgent;
              final expired = order.paymentDeadlineExpired;
              final text = hasDeadline ? countdown : '請儘速完成付款';
              final color = (urgent || expired)
                  ? OrderColorTokens.attention
                  : _kTextMuted;
              return Text(
                text,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.meta.copyWith(
                  color: color,
                  fontWeight: (urgent || expired)
                      ? FontWeight.w700
                      : FontWeight.w400,
                ),
              );
            },
          ),
        if (address.isNotEmpty) ...[
          SizedBox(height: AppSpacing.xs),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              HkIcon(
                HkIconName.location,
                size: 16.sp,
                color: AppIconColors.muted,
              ),
              SizedBox(width: AppSpacing.xs),
              Expanded(
                child: Text(
                  address,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.meta.copyWith(
                    color: _kTextMuted,
                    height: 1.45,
                  ),
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }
}

/// 訂單卡片展開後的訂單識別與操作列。
class OrderBottomRow extends StatelessWidget {
  const OrderBottomRow({
    super.key,
    required this.order,
    required this.actionButtons,
  });

  final Order order;
  final Widget actionButtons;

  @override
  Widget build(BuildContext context) {
    final orderTime = DateFormatUtils.formatDateTime(order.createTime);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Container(
              width: 40.w,
              height: 40.w,
              alignment: Alignment.center,
              decoration: const BoxDecoration(
                color: AppColors.primarySoft,
                shape: BoxShape.circle,
              ),
              child: HkIcon(
                HkIconName.document,
                size: 20.sp,
                color: AppIconColors.active,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('訂單編號', style: AppTypography.caption),
                  Text(
                    order.orderSn.isEmpty ? '—' : order.orderSn,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.meta.copyWith(
                      color: AppColors.textEmphasis,
                      fontWeight: FontWeight.w700,
                      fontFeatures: const [FontFeature.tabularFigures()],
                    ),
                  ),
                  if (orderTime.isNotEmpty)
                    Text('下單 $orderTime', style: AppTypography.caption),
                ],
              ),
            ),
            HkTapTarget(
              semanticLabel: '複製訂單編號',
              onTap: order.orderSn.isEmpty
                  ? null
                  : () {
                      Clipboard.setData(ClipboardData(text: order.orderSn));
                      showFloatingToast(context, '已複製訂單編號');
                    },
              child: Container(
                width: 40,
                height: 40,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: AppColors.surfaceMuted,
                  borderRadius: BorderRadius.circular(12.r),
                ),
                child: HkIcon(
                  HkIconName.copy,
                  size: 20.sp,
                  color: AppIconColors.active,
                ),
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        Align(alignment: Alignment.centerRight, child: actionButtons),
      ],
    );
  }
}
