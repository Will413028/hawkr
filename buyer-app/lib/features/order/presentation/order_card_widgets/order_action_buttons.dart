import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/app_spacing.dart';
import '../../data/after_sale_order_lookup.dart';
import '../../data/order_model.dart';
import '../../order_color_tokens.dart';
import '../order_detail_transition.dart';
import 'pill_button.dart';

/// 訂單卡片操作按鈕列 — 根據訂單狀態動態顯示不同按鈕組合
/// 待付款: 細節 + 匯款資訊(ATM) + 再次付款
/// 已送達: 細節 + 確認收貨/查看售後/填寫退款資訊
/// 已完成: 細節 + 商品評分
/// Figma: 按鈕 height 32, fontSize 12, 水平排列靠右, spacing 8
class OrderActionButtons extends ConsumerWidget {
  const OrderActionButtons({
    super.key,
    required this.order,
    required this.isPendingPayment,
    required this.isDelivered,
    required this.isAtmPayment,
    required this.detailLoading,
    required this.isPayLoading,
    required this.isConfirmLoading,
    required this.onShowAtmInfo,
    required this.onRetryPayment,
    required this.onOpenRefundPage,
    required this.onConfirmReceipt,
    required this.onReview,
  });

  final Order order;
  final bool isPendingPayment;
  final bool isDelivered;
  final bool isAtmPayment;
  final bool detailLoading;
  final bool isPayLoading;
  final bool isConfirmLoading;
  final VoidCallback onShowAtmInfo;
  final VoidCallback onRetryPayment;
  final VoidCallback onOpenRefundPage;
  final VoidCallback onConfirmReceipt;
  final VoidCallback onReview;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Wrap(
      spacing: AppSpacing.sm,
      runSpacing: AppSpacing.xs,
      alignment: WrapAlignment.end,
      children: [
        PillButton(
          label: '細節',
          color: OrderColorTokens.primary,
          onTap: () => context.push(
            '/orders/detail',
            extra: {
              'orderSn': order.orderSn,
              'heroTag': orderDetailHeroTag(order.orderSn),
            },
          ),
        ),
        if (isPendingPayment) ...[
          if (isAtmPayment && !detailLoading)
            PillButton(
              label: '匯款資訊',
              color: OrderColorTokens.attentionFill,
              onTap: onShowAtmInfo,
            ),
          PillButton(
            label: isPayLoading ? '處理中...' : '再次付款',
            color: OrderColorTokens.attentionFill,
            onTap: isPayLoading ? null : onRetryPayment,
          ),
        ],
        if (isDelivered)
          Builder(
            builder: (ctx) {
              final lookupMap = ref.watch(afterSaleOrderLookupProvider);
              final snapshot = lookupMap.value?[order.orderSn];

              if (snapshot != null) {
                if (snapshot.needRefundPage) {
                  return PillButton(
                    label: isPayLoading ? '處理中...' : '填寫退款資訊',
                    color: OrderColorTokens.attentionFill,
                    onTap: isPayLoading ? null : onOpenRefundPage,
                  );
                }
                return PillButton(
                  label: '查看售後',
                  color: OrderColorTokens.primary,
                  onTap: () => context.push(
                    '/settings/after-sale/detail',
                    extra: {'orderSn': order.orderSn},
                  ),
                );
              }
              return PillButton(
                label: isConfirmLoading ? '處理中...' : '確認收貨',
                color: OrderColorTokens.attentionFill,
                onTap: isConfirmLoading ? null : onConfirmReceipt,
              );
            },
          ),
        if (order.status == '50' && !order.isReviewed && order.canReview)
          PillButton(
            label: '商品評分',
            color: OrderColorTokens.attentionFill,
            onTap: onReview,
          ),
        // 已取消訂單 + ATM 需補填退款資訊（Figma 10258:117571 情境 A-ATM）
        if (order.status == '80' && order.needRefundPage)
          PillButton(
            label: isPayLoading ? '處理中...' : '填寫退款資訊',
            color: OrderColorTokens.attentionFill,
            onTap: isPayLoading ? null : onOpenRefundPage,
          ),
      ],
    );
  }
}
