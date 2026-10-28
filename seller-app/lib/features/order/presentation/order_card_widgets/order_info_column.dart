import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/codemeta/code_meta_display.dart';
import '../../../../core/codemeta/shipping_presentation.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../data/order_model.dart';

/// 卡片右側資訊欄（買家名稱、時間、地址、配送方式）。
/// 單號與按鈕群已搬至 [OrderBottomRow]。
class OrderInfoColumn extends ConsumerWidget {
  const OrderInfoColumn({super.key, required this.order});

  final OrderSummary order;

  String _formatDate(String raw) {
    if (raw.isEmpty) return '';
    DateTime? dt = DateTime.tryParse(raw);
    if (dt == null) {
      final m = RegExp(r'\d{10,13}').firstMatch(raw);
      if (m != null) {
        final n = int.tryParse(m.group(0)!);
        if (n != null) {
          dt = (m.group(0)!.length >= 13)
              ? DateTime.fromMillisecondsSinceEpoch(n)
              : DateTime.fromMillisecondsSinceEpoch(n * 1000);
        }
      }
    }
    if (dt == null) return raw;
    final local = dt.toLocal();
    String pad(int v) => v.toString().padLeft(2, '0');
    return '${local.year}/${pad(local.month)}/${pad(local.day)} ${pad(local.hour)}:${pad(local.minute)}';
  }

  String _formatShortDate(String raw) {
    if (raw.isEmpty) return '';
    final formatted = _formatDate(raw);
    final match = RegExp(
      r'^\d{4}/(\d{2}/\d{2} \d{2}:\d{2})$',
    ).firstMatch(formatted);
    return match?.group(1) ?? formatted;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final displayAddress = order.shippingAddress.isNotEmpty
        ? order.shippingAddress
        : order.address;
    final displayTime = _formatDate(order.dateTime);
    final shipping = shippingPresentationOf(order.shippingMethod);
    final shippingLabel =
        shipping == ShippingPresentation.cvsStorePickup &&
            order.pickupStoreName.isNotEmpty
        ? '7-11${order.pickupStoreName}'
        : CodeMetaDisplay.shippingMethod(order.shippingMethod);
    final buyerName = order.recipientName.isNotEmpty
        ? order.recipientName
        : order.buyerName.isNotEmpty
        ? order.buyerName
        : '買家';
    final deadlineText = _formatShortDate(order.autoCancelAt);
    final isShipmentDeadline = order.statusCode == 10 || order.statusCode == 20;

    final nameStyle = AppTypography.body.copyWith(height: 24 / 14);
    final timeStyle = AppTypography.meta.copyWith(
      color: AppColors.textSecondary,
      height: 1.67,
    );
    final buyer = Text(
      buyerName,
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
      style: nameStyle,
    );
    final time = Text(displayTime, style: timeStyle);

    return LayoutBuilder(
      builder: (context, constraints) {
        double textWidth(String text, TextStyle style) {
          final painter = TextPainter(
            text: TextSpan(text: text, style: style),
            textDirection: Directionality.of(context),
            textScaler: MediaQuery.textScalerOf(context),
          )..layout();
          final width = painter.width;
          painter.dispose();
          return width;
        }

        final inlineTime =
            displayTime.isNotEmpty &&
            textWidth(buyerName, nameStyle) +
                    8.w +
                    textWidth(displayTime, timeStyle) <=
                constraints.maxWidth;

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (inlineTime)
              Row(
                children: [
                  Expanded(child: buyer),
                  SizedBox(width: 8.w),
                  time,
                ],
              )
            else ...[
              buyer,
              if (displayTime.isNotEmpty) time,
            ],

            // 地址使用自己的完整寬度，不能被長門市名擠掉。
            if (displayAddress.isNotEmpty)
              Text(
                displayAddress,
                style: AppTypography.meta.copyWith(
                  color: AppColors.textSecondary,
                  height: 1.67,
                ),
              ),

            // ③ 期限提示：獨立一行，避免與下單時間塞在同一列。
            if (deadlineText.isNotEmpty)
              Padding(
                padding: EdgeInsets.only(top: 4.h),
                child: Row(
                  children: [
                    HkIcon(
                      isShipmentDeadline
                          ? HkIconName.clock
                          : HkIconName.warning,
                      size: 13.sp,
                      color: isShipmentDeadline
                          ? AppIconColors.warningText
                          : AppIconColors.danger,
                    ),
                    SizedBox(width: 4.w),
                    Flexible(
                      child: Text(
                        isShipmentDeadline
                            ? '出貨期限 $deadlineText'
                            : '取消期限 $deadlineText',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.meta.copyWith(
                          color: isShipmentDeadline
                              ? AppColors.warningText
                              : AppColors.error,
                          height: 20 / 12,
                        ),
                      ),
                    ),
                  ],
                ),
              ),

            // 配送品牌與門市保留原圖示；空間不足時自然換行。
            if (order.shippingMethod.isNotEmpty)
              Wrap(
                spacing: 8.w,
                runSpacing: 4.h,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  if (order.shippingMethod.isNotEmpty)
                    ConstrainedBox(
                      constraints: BoxConstraints(
                        maxWidth: constraints.maxWidth,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Container(
                            width: 12.w,
                            height: 12.w,
                            decoration: BoxDecoration(
                              borderRadius: BorderRadius.circular(6.r),
                              border: Border.all(
                                color: AppColors.textPrimary,
                                width: 0.5,
                              ),
                            ),
                            child: HkIcon(
                              shipping == ShippingPresentation.cvsStorePickup
                                  ? HkIconName.store
                                  : HkIconName.truck,
                              size: 9.sp,
                              color: AppIconColors.defaultColor,
                            ),
                          ),
                          SizedBox(width: 4.w),
                          Flexible(
                            child: Text(
                              shippingLabel,
                              style: AppTypography.meta.copyWith(
                                color: AppColors.textPrimary,
                                height: 2,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
          ],
        );
      },
    );
  }
}
