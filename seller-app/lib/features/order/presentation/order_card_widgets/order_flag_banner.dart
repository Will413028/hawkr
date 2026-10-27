import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../data/order_model.dart';
import '../../data/seller_after_sale_lookup.dart';

/// 訂單卡片右上角狀態旗幟（書籤形狀）— Figma node 10352:126087（master component，5 種狀態 variant），對齊 buyer App 同款元件。
///
/// 規格：32×34、頂部直角、底部 24 圓角，
/// drop_shadow(0, 1, 1.5, rgba(0,0,0,0.5))，icon 18×18 略偏上。
/// （Figma 原始尺寸 40×42 在實機顯得過大，依使用者偏好降至 32×34）
///
/// 用法：
/// - 已知訂單時用 `OrderFlagBanner.from(order: ..., hasAtmPending: ...)`
///   會自動根據 status + 售後 lookup 解析出對應 variant
/// - 已知 variant（譬如售後 detail page 一定是 dispute）直接用具名建構式：
///     OrderFlagBanner.pendingPayment()
///     OrderFlagBanner.shipping()
///     OrderFlagBanner.dispute()
///     OrderFlagBanner.completed()
///     OrderFlagBanner.cancelled()
class OrderFlagBanner extends StatelessWidget {
  /// 待付款（紅 + 錢幣）
  const OrderFlagBanner.pendingPayment({super.key})
    : color = AppColors.error,
      icon = AppIcons.paymentsOutlined,
      iconColor = AppColors.bgCard;

  /// 配送中（金 + 卡車）— 淺金底需搭深色 icon 才看得見
  const OrderFlagBanner.shipping({super.key})
    : color = AppColors.warning,
      icon = AppIcons.localShippingOutlined,
      iconColor = AppColors.textEmphasis;

  /// 爭議 / 售後（紅 + announcement）
  const OrderFlagBanner.dispute({super.key})
    : color = AppColors.error,
      icon = AppIcons.campaignOutlined,
      iconColor = AppColors.bgCard;

  /// 已完成（綠 + ✓）
  const OrderFlagBanner.completed({super.key})
    : color = AppColors.brandPrimaryStrong,
      icon = AppIcons.verified,
      iconColor = AppColors.bgCard;

  /// 取消 / 失效（灰 + ⊘）
  const OrderFlagBanner.cancelled({super.key})
    : color = AppColors.textPrimary,
      icon = AppIcons.block,
      iconColor = AppColors.bgCard;

  final Color color;
  final IconData icon;
  final Color iconColor;

  /// 自動根據訂單狀態解析 banner（含售後 case 偵測）。
  /// 回傳 widget — 若沒對應 banner 則回 SizedBox.shrink。
  static Widget from({
    required OrderSummary order,
    bool hasAtmPending = false,
  }) {
    return _AutoOrderFlagBanner(order: order, hasAtmPending: hasAtmPending);
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 32.w,
      height: 34.w,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.only(
          bottomLeft: Radius.circular(24.r),
          bottomRight: Radius.circular(24.r),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 1.5,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      // icon 略偏上（對齊 Figma：top 19% / bottom 24%）
      child: Align(
        alignment: const Alignment(0, -0.1),
        child: SizedBox(
          width: 18.w,
          height: 18.w,
          child: FittedBox(child: Icon(icon, color: iconColor)),
        ),
      ),
    );
  }
}

/// 內部 helper：根據 OrderSummary status + 售後 lookup 自動解析 banner。
class _AutoOrderFlagBanner extends ConsumerWidget {
  const _AutoOrderFlagBanner({
    required this.order,
    required this.hasAtmPending,
  });

  final OrderSummary order;
  final bool hasAtmPending;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final lookup = ref.watch(sellerAfterSaleLookupProvider);
    final hasAfterSaleCase = lookup.value?.containsKey(order.orderSn) ?? false;

    final code = order.statusCode;
    if (code == 0 || hasAtmPending) {
      return const OrderFlagBanner.pendingPayment();
    }
    switch (code) {
      case 50:
        return const OrderFlagBanner.completed();
      case 80:
        return const OrderFlagBanner.cancelled();
      case 60:
      case 70:
        return const OrderFlagBanner.dispute();
      case 40:
        return hasAfterSaleCase
            ? const OrderFlagBanner.dispute()
            : const OrderFlagBanner.shipping();
      case 10:
      case 20:
      case 30:
      case 32:
      case 35:
        return hasAfterSaleCase
            ? const OrderFlagBanner.dispute()
            : const OrderFlagBanner.shipping();
      default:
        return const SizedBox.shrink();
    }
  }
}
