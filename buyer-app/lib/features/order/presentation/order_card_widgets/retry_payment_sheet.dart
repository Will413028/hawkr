import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart'
    hide OrderStatus, OrderItemStatus;
import 'package:hawkr_buyer/shared/utils/hk_log.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/swipe_to_pay.dart';
import '../../data/order_model.dart';

/// 再次付款 BottomSheet — 顯示價格區 + 選擇付款方式（Apple Pay / ATM / 信用卡）+ SwipeToPay 確認
///
/// ATM 若為原付款方式則置灰不可選；底部滑動確認後執行付款。
/// 價格區僅顯示 商品總金額 / 運費 / 總付款金額（不顯示折價券，因再次付款不開放重選券）。
void showRetryPaymentSheet(
  BuildContext context, {
  required Order order,
  required String? currentPayType,
  required Future<void> Function(String payType) onPaymentSelected,
}) {
  final isAtmDisabled = currentPayType == PaymentMethod.atm.value;

  // 後端 §3.4.1 金額 gating：依訂單金額判定付款方式可用性。空清單 graceful 視為不限制。
  ({bool eligible, String reason}) amountAvail(String payType) {
    final list = order.availablePaymentMethods;
    if (list.isEmpty) return (eligible: true, reason: '');
    for (final m in list) {
      if (m['paymentType']?.toString() == payType) {
        return (
          eligible: m['eligible'] == true,
          reason: m['ineligibleReason']?.toString() ?? '',
        );
      }
    }
    return (eligible: true, reason: '');
  }

  bool methodDisabled(String payType) =>
      (isAtmDisabled && payType == PaymentMethod.atm.value) ||
      !amountAvail(payType).eligible;

  int selectedIdx = _kRetryPayMethods.indexWhere(
    (m) => m.payType == currentPayType && !methodDisabled(m.payType),
  );
  if (selectedIdx < 0) {
    selectedIdx = _kRetryPayMethods.indexWhere(
      (m) => !methodDisabled(m.payType),
    );
    if (selectedIdx < 0) selectedIdx = 0;
  }

  showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: false,
    backgroundColor: AppColors.bgPrimary,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (_) {
      return StatefulBuilder(
        builder: (ctx, setSB) {
          return SafeArea(
            top: false,
            child: SingleChildScrollView(
              key: const PageStorageKey('retry-payment-sheet-scroll'),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Padding(
                    padding: EdgeInsets.only(
                      top: AppSpacing.md,
                      bottom: AppSpacing.lg,
                    ),
                    child: Container(
                      width: 40.w,
                      height: 4.h,
                      decoration: BoxDecoration(
                        color: AppColors.dragHandle,
                        borderRadius: BorderRadius.circular(2.r),
                      ),
                    ),
                  ),

                  // 價格區（不含折價券，與滑動結帳頁差異點）
                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg),
                    child: _PaymentDetailsCard(order: order),
                  ),
                  SizedBox(height: AppSpacing.lg),

                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg),
                    child: Container(
                      padding: EdgeInsets.all(AppSpacing.lg),
                      decoration: BoxDecoration(
                        color: AppColors.bgCard,
                        borderRadius: BorderRadius.circular(16.r),
                        border: Border.all(
                          color: AppTypography.colorPrimary.withValues(
                            alpha: 0.2,
                          ),
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
                          Row(
                            children: [
                              SizedBox(
                                width: 24.sp,
                                height: 24.sp,
                                child: FittedBox(
                                  child: Icon(
                                    AppIcons.creditCardOutlined,
                                    color: AppIconColors.defaultColor,
                                  ),
                                ),
                              ),
                              SizedBox(width: 6.w),
                              Text(
                                '付款方式',
                                style: AppTypography.sectionTitle.copyWith(
                                  fontSize: 18,
                                  height: 1.23,
                                ),
                              ),
                            ],
                          ),
                          SizedBox(height: AppSpacing.md),
                          ..._kRetryPayMethods.asMap().entries.expand((e) {
                            HKLog.logD(
                              '[OrderCard] retryPayMethod: ${e.value}',
                            );
                            final idx = e.key;
                            final method = e.value;
                            final isSelected = idx == selectedIdx;
                            final amtAvail = amountAvail(method.payType);
                            final isDisabled = methodDisabled(method.payType);
                            final labelColor = isDisabled
                                ? AppTypography.colorDisabled
                                : AppColors.textEmphasis;
                            final disabledReason = !amtAvail.eligible
                                ? (_kIneligibleReasonLabel[amtAvail.reason] ??
                                      '金額不符')
                                : '不可選';
                            void selectMethod() =>
                                setSB(() => selectedIdx = idx);
                            return [
                              Semantics(
                                button: true,
                                selected: isSelected,
                                enabled: !isDisabled,
                                label: isDisabled
                                    ? '${method.label}，$disabledReason'
                                    : method.label,
                                onTap: isDisabled ? null : selectMethod,
                                excludeSemantics: true,
                                child: InkWell(
                                  key: ValueKey(
                                    'retry-payment-method-${method.payType}',
                                  ),
                                  onTap: isDisabled ? null : selectMethod,
                                  child: ConstrainedBox(
                                    constraints: const BoxConstraints(
                                      minHeight: HkControl.minTap,
                                    ),
                                    child: Padding(
                                      padding: EdgeInsets.symmetric(
                                        vertical: AppSpacing.sm,
                                      ),
                                      child: _PaymentMethodContent(
                                        label: method.label,
                                        payType: method.payType,
                                        labelColor: labelColor,
                                        isSelected: isSelected,
                                        isDisabled: isDisabled,
                                        disabledReason: disabledReason,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                              if (method.payType == PaymentMethod.atm.value)
                                Padding(
                                  padding: EdgeInsets.only(
                                    left: 24.w,
                                    bottom: 4.h,
                                  ),
                                  child: Text(
                                    '虛擬帳號只能用網路銀行跟ATM轉帳，不能臨櫃匯款',
                                    style: AppTypography.metaMuted,
                                  ),
                                ),
                              if (idx < _kRetryPayMethods.length - 1)
                                Divider(
                                  height: 1,
                                  color: AppTypography.colorPrimary.withValues(
                                    alpha: 0.1,
                                  ),
                                ),
                            ];
                          }),
                        ],
                      ),
                    ),
                  ),

                  // SwipeToPay
                  Padding(
                    padding: EdgeInsets.fromLTRB(
                      24.w,
                      AppSpacing.lg,
                      24.w,
                      AppSpacing.md,
                    ),
                    child: SwipeToPay(
                      onComplete: () async {
                        Navigator.of(ctx).pop();
                        onPaymentSelected(
                          _kRetryPayMethods[selectedIdx].payType,
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      );
    },
  );
}

const _kRetryPayMethods = [
  (label: 'Apple Pay', payType: '2002'),
  (label: 'ATM 虛擬轉帳', payType: '2004'),
  (label: '信用卡', payType: '2001'),
];

// 付款方式不可用理由碼 → 通用文案（對齊後端 PaymentMethodIneligibleReason；§3.4.1）
const Map<String, String> _kIneligibleReasonLabel = {
  '2101': '金額未達此付款方式下限',
  '2102': '金額超過此付款方式上限',
};

/// 付款詳情卡片 — 商品總金額 / 運費 / 總付款金額。
/// 不顯示折扣券（再次付款流程不允許重選券）。
class _PaymentDetailsCard extends StatelessWidget {
  const _PaymentDetailsCard({required this.order});
  final Order order;

  @override
  Widget build(BuildContext context) {
    final productAmount = order.totalOriginalAmount.isNotEmpty
        ? order.totalOriginalAmount
        : '—';
    final shippingFee = order.shippingFee.isNotEmpty ? order.shippingFee : '—';
    final totalActual = order.totalActualAmount.isNotEmpty
        ? order.totalActualAmount
        : '—';

    return Container(
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 20.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(
          color: AppTypography.colorPrimary.withValues(alpha: 0.2),
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
        children: [
          Row(
            children: [
              HkIcon(
                HkIconName.payment,
                size: 24.sp,
                color: AppIconColors.strong,
              ),
              SizedBox(width: 6.w),
              Text(
                '付款詳情',
                style: AppTypography.sectionTitle.copyWith(
                  fontSize: 18,
                  height: 1.23,
                ),
              ),
            ],
          ),
          SizedBox(height: 10.h),
          _DetailRow(
            key: const ValueKey('retry-payment-product-amount'),
            icon: HkIconName.goods,
            label: '商品總金額',
            value: 'NT\$ $productAmount',
          ),
          SizedBox(height: 10.h),
          _DetailRow(
            key: const ValueKey('retry-payment-shipping-fee'),
            icon: HkIconName.truck,
            label: '運費總金額',
            value: 'NT\$ $shippingFee',
          ),
          SizedBox(height: 10.h),
          const Divider(height: 1, color: AppColors.dividerLight),
          SizedBox(height: 10.h),
          _AdaptiveTextPair(
            key: const ValueKey('retry-payment-total-amount'),
            label: '總付款金額',
            value: 'NT\$ $totalActual',
            gap: 40.w,
            labelStyle: const TextStyle(
              fontSize: 18,
              height: 1.38,
              color: AppColors.textEmphasis,
            ),
            valueStyle: const TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w700,
              height: 1.4,
              color: AppColors.textEmphasis,
            ),
          ),
        ],
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  const _DetailRow({
    super.key,
    required this.icon,
    required this.label,
    required this.value,
  });
  final HkIconName icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    const labelStyle = TextStyle(
      fontSize: 16,
      height: 1.5,
      color: AppTypography.colorPrimary,
    );
    const valueStyle = TextStyle(
      fontSize: 18,
      height: 1.38,
      color: AppTypography.colorPrimary,
    );
    return _AdaptiveTextPair(
      label: label,
      value: value,
      gap: AppSpacing.lg,
      labelStyle: labelStyle,
      valueStyle: valueStyle,
      leading: HkIcon(icon, size: 24.sp, color: AppIconColors.defaultColor),
      leadingGap: 12.w,
    );
  }
}

class _AdaptiveTextPair extends StatelessWidget {
  const _AdaptiveTextPair({
    super.key,
    required this.label,
    required this.value,
    required this.gap,
    required this.labelStyle,
    required this.valueStyle,
    this.leading,
    this.leadingGap = 0,
  });

  final String label;
  final String value;
  final double gap;
  final TextStyle labelStyle;
  final TextStyle valueStyle;
  final Widget? leading;
  final double leadingGap;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final textScaler = MediaQuery.textScalerOf(context);
        final direction = Directionality.of(context);
        final labelWidth = _singleLineTextWidth(
          label,
          labelStyle,
          textScaler,
          direction,
        );
        final valueWidth = _singleLineTextWidth(
          value,
          valueStyle,
          textScaler,
          direction,
        );
        final leadingWidth = leading == null ? 0.0 : 24.sp + leadingGap;
        final fitsInline =
            leadingWidth + labelWidth + gap + valueWidth <=
            constraints.maxWidth;
        final labelRow = Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (leading != null) ...[leading!, SizedBox(width: leadingGap)],
            Flexible(child: Text(label, style: labelStyle)),
          ],
        );

        if (fitsInline) {
          return Row(
            key: ValueKey('retry-payment-info-$label-inline'),
            children: [
              labelRow,
              SizedBox(width: gap),
              Expanded(
                child: Text(
                  value,
                  maxLines: 1,
                  softWrap: false,
                  overflow: TextOverflow.ellipsis,
                  style: valueStyle,
                  textAlign: TextAlign.right,
                ),
              ),
            ],
          );
        }

        return Column(
          key: ValueKey('retry-payment-info-$label-stacked'),
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Align(alignment: Alignment.centerLeft, child: labelRow),
            const SizedBox(height: HkSpace.xs),
            Text(value, style: valueStyle, textAlign: TextAlign.right),
          ],
        );
      },
    );
  }
}

class _PaymentMethodContent extends StatelessWidget {
  const _PaymentMethodContent({
    required this.label,
    required this.payType,
    required this.labelColor,
    required this.isSelected,
    required this.isDisabled,
    required this.disabledReason,
  });

  final String label;
  final String payType;
  final Color labelColor;
  final bool isSelected;
  final bool isDisabled;
  final String disabledReason;

  @override
  Widget build(BuildContext context) {
    final labelStyle = AppTypography.body.copyWith(
      fontSize: 18,
      color: labelColor,
    );
    return LayoutBuilder(
      builder: (context, constraints) {
        final labelWidth = _singleLineTextWidth(
          label,
          labelStyle,
          MediaQuery.textScalerOf(context),
          Directionality.of(context),
        );
        final enlargedText = MediaQuery.textScalerOf(context).scale(18) > 18.01;
        final stackIcon =
            !isDisabled &&
            ((enlargedText && constraints.maxWidth < HkBreakpoints.compact) ||
                16.w +
                        HkSpace.sm +
                        labelWidth +
                        HkSpace.sm +
                        40.w +
                        HkSpace.md >
                    constraints.maxWidth);

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Row(
              children: [
                _RadioCircle(isSelected: isSelected, isDisabled: isDisabled),
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
                if (!isDisabled && !stackIcon) ...[
                  const SizedBox(width: HkSpace.sm),
                  _PayMethodIcon(payType: payType),
                ],
              ],
            ),
            if (isDisabled) ...[
              SizedBox(height: AppSpacing.xs),
              Padding(
                padding: EdgeInsets.only(left: 27.w),
                child: Text(
                  disabledReason,
                  style: AppTypography.meta.copyWith(
                    color: AppTypography.colorDisabled,
                  ),
                ),
              ),
            ] else if (stackIcon) ...[
              SizedBox(height: AppSpacing.xs),
              Align(
                alignment: Alignment.centerRight,
                child: _PayMethodIcon(payType: payType),
              ),
            ],
          ],
        );
      },
    );
  }
}

double _singleLineTextWidth(
  String text,
  TextStyle style,
  TextScaler textScaler,
  TextDirection direction,
) {
  final painter = TextPainter(
    text: TextSpan(text: text, style: style),
    maxLines: 1,
    textScaler: textScaler,
    textDirection: direction,
  )..layout();
  return painter.width;
}

/// 付款方式選項前方的自定義 Radio 圓圈（16×16）
class _RadioCircle extends StatelessWidget {
  const _RadioCircle({required this.isSelected, required this.isDisabled});
  final bool isSelected;
  final bool isDisabled;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 16.w,
      height: 16.w,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: AppColors.bgCard,
        border: Border.all(
          color: AppColors.textEmphasis.withValues(
            alpha: isDisabled ? 0.08 : 0.2,
          ),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: isSelected
          ? Center(
              child: Container(
                width: 8.w,
                height: 8.w,
                decoration: const BoxDecoration(
                  shape: BoxShape.circle,
                  color: AppColors.primary,
                ),
              ),
            )
          : null,
    );
  }
}

/// 付款方式圖示 — Apple Pay / ATM / 信用卡（ATM 以文字呈現）
class _PayMethodIcon extends StatelessWidget {
  const _PayMethodIcon({required this.payType});
  final String payType;

  @override
  Widget build(BuildContext context) {
    if (payType == PaymentMethod.applePay.value) {
      return SizedBox(
        width: 40.w,
        height: 30.h,
        child: FittedBox(
          child: Icon(
            AppIcons.contactlessOutlined,
            color: AppIconColors.strong,
          ),
        ),
      );
    }
    if (payType == PaymentMethod.atm.value) {
      return Text(
        'ATM',
        style: AppTypography.sectionTitle.copyWith(
          color: AppColors.primary,
          fontSize: 20,
        ),
      );
    }
    if (payType == PaymentMethod.creditCard.value) {
      return SizedBox(
        width: 40.w,
        height: 30.h,
        child: FittedBox(
          child: Icon(AppIcons.creditCardOutlined, color: AppIconColors.active),
        ),
      );
    }
    return const SizedBox.shrink();
  }
}
