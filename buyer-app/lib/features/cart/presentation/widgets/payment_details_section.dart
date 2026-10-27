import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

// ---------------------------------------------------------------------------
// 付款詳情區塊。
//
// 白色圓角卡片，展示商品總額、運費、折扣券、總付款金額。
// 包含「選擇優惠券」按鈕，點擊後由外部處理彈出優惠券選擇器。
// ---------------------------------------------------------------------------

/// 購物車底部的付款詳情摘要卡片。
///
/// [productTotal] 商品總金額。
/// [shippingTotal] 運費總金額。
/// [couponDiscount] 已選優惠券折扣金額。
/// [grandTotal] 最終總付款金額。
/// [canSelectCoupon] 是否可以選擇優惠券（需單一商家 + 有配送選項）。
/// [onSelectCoupon] 點擊「選擇優惠券」的回調。
class PaymentDetailsSection extends StatelessWidget {
  const PaymentDetailsSection({
    super.key,
    required this.productTotal,
    required this.shippingTotal,
    required this.couponDiscount,
    required this.grandTotal,
    required this.canSelectCoupon,
    this.onSelectCoupon,
  });

  final double productTotal;
  final double shippingTotal;
  final double couponDiscount;
  final double grandTotal;
  final bool canSelectCoupon;
  final VoidCallback? onSelectCoupon;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(
        horizontal: HkSpace.lg,
        vertical: HkSpace.sm,
      ),
      padding: const EdgeInsets.all(HkSpace.lg),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.lg),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: HkShadows.card,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          // 標題
          Row(
            children: [
              HkIcon(
                HkIconName.payment,
                size: HkControl.icon,
                color: AppIconColors.strong,
              ),
              const SizedBox(width: HkSpace.sm),
              Text('付款詳情', style: CartTypography.section),
            ],
          ),
          const SizedBox(height: HkSpace.md),

          // 商品總金額
          _paymentRow(
            context,
            HkIconName.goods,
            '商品總金額',
            'NT\$ ${productTotal.toStringAsFixed(0)}',
          ),
          const SizedBox(height: HkSpace.md),

          // 運費總金額
          _paymentRow(
            context,
            HkIconName.truck,
            '運費總金額',
            'NT\$ ${shippingTotal.toStringAsFixed(0)}',
          ),
          const SizedBox(height: HkSpace.md),

          // 折扣券列
          _couponRow(context),
          const SizedBox(height: HkSpace.md),

          // 分隔線
          const Divider(height: 1, color: AppColors.dividerLight),
          const SizedBox(height: HkSpace.md),

          // 總付款金額
          _totalRow(context),
        ],
      ),
    );
  }

  /// 單行付款明細：icon + 標籤 + 金額。
  Widget _paymentRow(
    BuildContext context,
    HkIconName icon,
    String label,
    String amount,
  ) {
    final labelStyle = CartTypography.body;
    final amountStyle = CartTypography.amount.copyWith(
      color: AppColors.textEmphasis,
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final requiredWidth =
            HkControl.icon +
            HkSpace.sm +
            _singleLineWidth(context, label, labelStyle) +
            HkSpace.sm +
            _singleLineWidth(context, amount, amountStyle);
        // A narrow layout stays stacked even while browser fallback fonts load.
        final stack =
            constraints.maxWidth <
                280 * MediaQuery.textScalerOf(context).scale(16) / 16 ||
            requiredWidth > constraints.maxWidth;
        final labelGroup = Row(
          children: [
            HkIcon(
              icon,
              size: HkControl.icon,
              color: AppIconColors.defaultColor,
            ),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Text(
                label,
                maxLines: 1,
                softWrap: false,
                overflow: TextOverflow.ellipsis,
                style: labelStyle,
              ),
            ),
          ],
        );
        final amountText = Text(
          amount,
          maxLines: 1,
          softWrap: false,
          style: amountStyle,
          textAlign: TextAlign.right,
        );

        if (stack) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              labelGroup,
              const SizedBox(height: HkSpace.xs),
              Align(alignment: Alignment.centerRight, child: amountText),
            ],
          );
        }

        return Row(
          children: [
            Expanded(child: labelGroup),
            const SizedBox(width: HkSpace.sm),
            amountText,
          ],
        );
      },
    );
  }

  Widget _couponRow(BuildContext context) {
    final labelStyle = CartTypography.body;
    final discountStyle = CartTypography.subtitle.copyWith(
      color: AppColors.danger,
    );
    const buttonLabel = '選擇優惠券';
    final buttonStyle = CartTypography.button;
    final discount = '-NT\$ ${couponDiscount.toStringAsFixed(0)}';

    Widget couponButton() => OutlinedButton(
      onPressed: canSelectCoupon ? onSelectCoupon : null,
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(0, HkControl.minTap),
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
        foregroundColor: AppColors.actionPrimary,
        textStyle: buttonStyle,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
        ),
      ),
      child: const HkButtonLabel(buttonLabel),
    );

    final labelGroup = Row(
      children: [
        HkIcon(
          HkIconName.tag,
          size: HkControl.icon,
          color: AppIconColors.defaultColor,
        ),
        const SizedBox(width: HkSpace.md),
        Expanded(
          child: Text(
            '折扣券',
            maxLines: 1,
            softWrap: false,
            overflow: TextOverflow.ellipsis,
            style: labelStyle,
          ),
        ),
      ],
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final requiredWidth =
            HkControl.icon +
            HkSpace.md +
            _singleLineWidth(context, '折扣券', labelStyle) +
            HkSpace.sm +
            (couponDiscount > 0
                ? _singleLineWidth(context, discount, discountStyle) +
                      HkSpace.sm
                : 0) +
            _singleLineWidth(context, buttonLabel, buttonStyle) +
            HkSpace.md * 2;
        // A narrow layout stays stacked even while browser fallback fonts load.
        final stack =
            constraints.maxWidth <
                280 * MediaQuery.textScalerOf(context).scale(16) / 16 ||
            requiredWidth > constraints.maxWidth;
        final actions = Wrap(
          alignment: WrapAlignment.end,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: HkSpace.sm,
          runSpacing: HkSpace.xs,
          children: [
            if (couponDiscount > 0)
              Text(
                discount,
                maxLines: 1,
                softWrap: false,
                style: discountStyle,
              ),
            couponButton(),
          ],
        );

        if (stack) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              labelGroup,
              const SizedBox(height: HkSpace.xs),
              actions,
            ],
          );
        }

        return Row(
          children: [
            Expanded(child: labelGroup),
            const SizedBox(width: HkSpace.sm),
            actions,
          ],
        );
      },
    );
  }

  Widget _totalRow(BuildContext context) {
    final labelStyle = CartTypography.subtitle;
    final amountStyle = CartTypography.total.copyWith(
      color: AppColors.brandPrimaryStrong,
    );
    final amount = 'NT\$ ${grandTotal.toStringAsFixed(0)}';

    return LayoutBuilder(
      builder: (context, constraints) {
        final stack =
            constraints.maxWidth <
                280 * MediaQuery.textScalerOf(context).scale(16) / 16 ||
            _singleLineWidth(context, '總付款金額', labelStyle) +
                    HkSpace.sm +
                    _singleLineWidth(context, amount, amountStyle) >
                constraints.maxWidth;
        final label = Text(
          '總付款金額',
          maxLines: 1,
          softWrap: false,
          style: labelStyle,
        );
        final value = Text(
          amount,
          maxLines: 1,
          softWrap: false,
          style: amountStyle,
          textAlign: TextAlign.right,
        );

        if (stack) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              label,
              const SizedBox(height: HkSpace.xs),
              Align(alignment: Alignment.centerRight, child: value),
            ],
          );
        }
        return Row(
          children: [
            Expanded(child: label),
            const SizedBox(width: HkSpace.sm),
            value,
          ],
        );
      },
    );
  }

  double _singleLineWidth(BuildContext context, String value, TextStyle style) {
    final painter = TextPainter(
      text: TextSpan(text: value, style: style),
      maxLines: 1,
      textDirection: Directionality.of(context),
      textScaler: MediaQuery.textScalerOf(context),
    )..layout();
    final width = painter.width;
    painter.dispose();
    return width;
  }
}
