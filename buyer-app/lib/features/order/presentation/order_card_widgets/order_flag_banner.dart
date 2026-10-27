import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/order/order_color_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 訂單卡片右上角狀態旗幟 — Figma node 10352:126087（master component，5 種狀態 variant）。
///
/// 規格：32×34、頂部直角、底部 24 圓角（書籤形狀），
/// drop_shadow(0, 1, 1.5, rgba(0,0,0,0.5))，icon 18×18 略偏上。
/// （Figma 原始尺寸 40×42 在實機顯得過大，依使用者偏好降至 32×34）
///
/// 用法：
/// - 用具名 constructor 取得四大主色衍生變體：
///     OrderFlagBanner.pendingPayment()   // 待付款（Gold + $）
///     OrderFlagBanner.shipping()         // 配送中（primary + 卡車）
///     OrderFlagBanner.dispute()          // 爭議/售後（Gold + !）
///     OrderFlagBanner.completed()        // 已完成（primary + ✓）
///     OrderFlagBanner.cancelled()        // 取消/失效（Grain + ⊘）
/// - 若需要客製顏色 / icon，直接用主 constructor 傳 color + iconData。
class OrderFlagBanner extends StatelessWidget {
  const OrderFlagBanner({
    super.key,
    required this.color,
    required this.iconData,
    this.iconColor = AppColors.bgCard,
  });

  /// 待付款（Gold + 錢幣 icon）
  const OrderFlagBanner.pendingPayment({super.key})
    : color = OrderColorTokens.attentionFill,
      iconData = AppIcons.paidOutlined,
      iconColor = OrderColorTokens.onAttention;

  /// 配送中（primary + 卡車 icon）
  const OrderFlagBanner.shipping({super.key})
    : color = OrderColorTokens.progress,
      iconData = AppIcons.localShippingOutlined,
      iconColor = OrderColorTokens.onPrimary;

  /// 爭議 / 售後（Gold + announcement icon）
  const OrderFlagBanner.dispute({super.key})
    : color = OrderColorTokens.attentionFill,
      iconData = AppIcons.campaignOutlined,
      iconColor = OrderColorTokens.onAttention;

  /// 已完成（綠 + verified ✓）— Figma 10092:99190
  const OrderFlagBanner.completed({super.key})
    : color = OrderColorTokens.primary,
      iconData = AppIcons.verifiedOutlined,
      iconColor = OrderColorTokens.onPrimary;

  /// 取消 / 失效（Grain + block ⊘）
  const OrderFlagBanner.cancelled({super.key})
    : color = OrderColorTokens.neutralSurface,
      iconData = AppIcons.blockOutlined,
      iconColor = OrderColorTokens.onAttention;

  final Color color;
  final IconData iconData;
  final Color iconColor;

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
      // Figma icon inset: top 19.05% / bottom 23.81% — 略偏上。
      // 用 Align + offset 模擬：icon 中心約在容器垂直 47% 處。
      child: Align(
        alignment: const Alignment(0, -0.1),
        child: SizedBox(
          width: 18.w,
          height: 18.w,
          child: Icon(iconData, color: iconColor, size: 18.w),
        ),
      ),
    );
  }
}
