import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../../shared/widgets/truck_loading_overlay.dart';
import 'coupon_helpers.dart';

// ---------------------------------------------------------------------------
// 優惠券選擇 BottomSheet。
//
// 以 FutureBuilder 非同步載入可用優惠券列表，
// 使用者點選後 pop 回選中的 coupon Map，呼叫端再套用。
// ---------------------------------------------------------------------------

/// 優惠券選擇彈窗 — 用於 showModalBottomSheet。
///
/// [couponsFuture] 為非同步載入的優惠券列表。
/// [initiallySelectedId] 為目前已選中的優惠券 ID（高亮顯示）。
class CouponSelectionSheet extends StatelessWidget {
  const CouponSelectionSheet({
    super.key,
    required this.couponsFuture,
    required this.initiallySelectedId,
  });

  final Future<List<Map<String, dynamic>>> couponsFuture;
  final String? initiallySelectedId;

  @override
  Widget build(BuildContext context) {
    final maxHeight = MediaQuery.of(context).size.height * 0.75;
    return SafeArea(
      top: false,
      child: Container(
        constraints: BoxConstraints(maxHeight: maxHeight),
        padding: const EdgeInsets.fromLTRB(
          HkSpace.lg,
          HkSpace.lg,
          HkSpace.lg,
          HkSpace.xxl,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 標題列
            Row(
              children: [
                Expanded(child: Text('選擇優惠券', style: CartTypography.section)),
                IconButton(
                  tooltip: '關閉優惠券',
                  onPressed: () => context.pop(),
                  icon: const Icon(AppIcons.close),
                ),
              ],
            ),
            const SizedBox(height: HkSpace.sm),

            // 優惠券列表
            Expanded(
              child: FutureBuilder<List<Map<String, dynamic>>>(
                future: couponsFuture,
                builder: (context, snapshot) {
                  if (snapshot.connectionState == ConnectionState.waiting) {
                    return const TruckLoadingOverlay(message: '載入中...');
                  }
                  if (snapshot.hasError) {
                    return Center(
                      child: Text(
                        '載入優惠券失敗，請稍後再試',
                        style: TextStyle(color: AppColors.danger),
                      ),
                    );
                  }
                  final coupons =
                      snapshot.data ?? const <Map<String, dynamic>>[];
                  if (coupons.isEmpty) {
                    return const HkEmptyState(
                      key: ValueKey('coupon-picker-empty-state'),
                      iconKey: ValueKey('coupon-picker-empty-state-icon'),
                      icon: HkIconName.emptyCoupon,
                      message: '目前沒有可用的優惠券',
                    );
                  }
                  return ListView.separated(
                    padding: const EdgeInsets.only(top: HkSpace.sm),
                    itemCount: coupons.length,
                    separatorBuilder: (_, __) =>
                        const SizedBox(height: HkSpace.md),
                    itemBuilder: (context, index) {
                      final coupon = coupons[index];
                      final isSelected =
                          initiallySelectedId != null &&
                          initiallySelectedId!.isNotEmpty &&
                          initiallySelectedId == couponIdentifier(coupon);
                      return _CouponListTile(
                        coupon: coupon,
                        isSelected: isSelected,
                        onApply: () => context.pop(coupon),
                      );
                    },
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// 單張優惠券卡片
// ---------------------------------------------------------------------------

/// 優惠券列表中的單張卡片 — 顯示名稱、折扣、有效期限、適用商家。
class _CouponListTile extends StatelessWidget {
  const _CouponListTile({
    required this.coupon,
    required this.isSelected,
    required this.onApply,
  });

  final Map<String, dynamic> coupon;
  final bool isSelected;
  final VoidCallback onApply;

  @override
  Widget build(BuildContext context) {
    final title = couponDisplayTitle(coupon);
    final discount = couponDiscountText(coupon);
    final expire = couponExpireText(coupon);
    final merchant = couponMerchantText(coupon);

    return Container(
      padding: const EdgeInsets.all(HkSpace.lg),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(
          color: isSelected ? AppColors.primary : AppColors.gray100,
          width: isSelected ? 1.5 : 1,
        ),
        boxShadow: isSelected
            ? [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 12,
                  offset: const Offset(0, 4),
                ),
              ]
            : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 名稱 + 選中 icon
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(child: Text(title, style: CartTypography.subtitle)),
              if (isSelected)
                const Icon(
                  AppIcons.checkCircle,
                  color: AppIconColors.secondary,
                ),
            ],
          ),
          const SizedBox(height: HkSpace.sm),

          // 折扣描述
          if (discount.isNotEmpty) ...[
            Text(
              discount,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: AppColors.success,
              ),
            ),
            const SizedBox(height: HkSpace.xs),
          ],

          // 適用商家
          if (merchant.isNotEmpty) ...[
            Text(merchant, style: CartTypography.support),
            const SizedBox(height: HkSpace.xs),
          ],

          // 有效期限
          if (expire.isNotEmpty) ...[
            Text(expire, style: CartTypography.supportMuted),
            const SizedBox(height: HkSpace.md),
          ] else
            const SizedBox(height: HkSpace.md),

          // 套用按鈕
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () => onApply(),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.actionPrimary,
                minimumSize: const Size(0, HkControl.minTap),
                padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(HkRadius.md),
                ),
              ),
              child: HkButtonLabel(
                isSelected ? '使用中' : '使用此優惠券',
                style: CartTypography.button.copyWith(
                  color: AppColors.textInverse,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
