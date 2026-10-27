import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'section_card.dart';
import 'section_header.dart';

/// 優惠券名稱、折扣方式、折扣值與最低消費門檻。
class CouponFormCard extends StatelessWidget {
  const CouponFormCard({
    super.key,
    required this.nameCtrl,
    required this.discountCtrl,
    required this.minPurchaseCtrl,
    required this.discountRule,
    required this.onDiscountRuleChanged,
    this.maxDiscountAmountCtrl,
  });

  final TextEditingController nameCtrl;
  final TextEditingController discountCtrl;
  final TextEditingController minPurchaseCtrl;
  final String discountRule;
  final ValueChanged<String> onDiscountRuleChanged;
  final TextEditingController? maxDiscountAmountCtrl;

  @override
  Widget build(BuildContext context) {
    final isPercent = discountRule == '折數折抵';

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const CouponSectionHeader(
            step: '1',
            icon: HkIconName.coupon,
            title: '設定優惠內容',
            description: '先決定顧客看到的名稱，再設定折扣與使用門檻。',
          ),
          const SizedBox(height: HkSpace.xxl),
          _CouponTextField(
            controller: nameCtrl,
            label: '優惠券名稱',
            hintText: '例如：新鮮水果滿額折',
            textInputAction: TextInputAction.next,
            maxLength: 30,
          ),
          const SizedBox(height: HkSpace.xl),
          const _FieldLabel(label: '折扣方式', supportingText: '選擇固定金額或依訂單比例折抵'),
          const SizedBox(height: HkSpace.sm),
          LayoutBuilder(
            builder: (context, constraints) {
              final compact = constraints.maxWidth < 340;
              final cash = _DiscountRuleButton(
                label: '固定金額',
                description: '直接折抵指定金額',
                icon: HkIconName.wallet,
                isSelected: !isPercent,
                onTap: () => onDiscountRuleChanged('現金折抵'),
              );
              final percent = _DiscountRuleButton(
                label: '百分比折扣',
                description: '依訂單金額計算',
                icon: HkIconName.tag,
                isSelected: isPercent,
                onTap: () => onDiscountRuleChanged('折數折抵'),
              );
              if (compact) {
                return Column(
                  children: [
                    cash,
                    const SizedBox(height: HkSpace.sm),
                    percent,
                  ],
                );
              }
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: cash),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(child: percent),
                ],
              );
            },
          ),
          const SizedBox(height: HkSpace.xl),
          LayoutBuilder(
            builder: (context, constraints) {
              final useColumns = constraints.maxWidth >= 520;
              final discountField = _CouponTextField(
                controller: discountCtrl,
                label: isPercent ? '折扣百分比' : '折扣金額',
                hintText: isPercent ? '例如：10' : '例如：100',
                prefixText: isPercent ? null : 'NT\$ ',
                suffixText: isPercent ? ' %' : null,
                keyboardType: TextInputType.number,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                textInputAction: TextInputAction.next,
              );
              final thresholdField = _CouponTextField(
                controller: minPurchaseCtrl,
                label: '最低消費門檻',
                hintText: '0',
                prefixText: 'NT\$ ',
                keyboardType: TextInputType.number,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                textInputAction: TextInputAction.next,
                helperText: '填 0 代表不限消費金額',
              );
              if (!useColumns) {
                return Column(
                  children: [
                    discountField,
                    const SizedBox(height: HkSpace.xl),
                    thresholdField,
                  ],
                );
              }
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: discountField),
                  const SizedBox(width: HkSpace.md),
                  Expanded(child: thresholdField),
                ],
              );
            },
          ),
          AnimatedSwitcher(
            duration: HkMotion.duration,
            switchInCurve: HkMotion.ease,
            switchOutCurve: HkMotion.ease,
            child: isPercent && maxDiscountAmountCtrl != null
                ? Padding(
                    key: const ValueKey('max-discount'),
                    padding: const EdgeInsets.only(top: HkSpace.xl),
                    child: _CouponTextField(
                      controller: maxDiscountAmountCtrl!,
                      label: '最高折抵上限',
                      hintText: '例如：300',
                      prefixText: 'NT\$ ',
                      keyboardType: TextInputType.number,
                      inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                      helperText: '避免高額訂單折抵超出預算；留空表示不限制',
                    ),
                  )
                : const SizedBox.shrink(key: ValueKey('no-max-discount')),
          ),
        ],
      ),
    );
  }
}

class _FieldLabel extends StatelessWidget {
  const _FieldLabel({required this.label, this.supportingText});

  final String label;
  final String? supportingText;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(
            color: AppColors.textEmphasis,
            fontSize: 16,
            fontWeight: FontWeight.w700,
          ),
        ),
        if (supportingText != null) ...[
          const SizedBox(height: HkSpace.xs),
          Text(
            supportingText!,
            style: const TextStyle(
              color: AppColors.textMuted,
              fontSize: 12,
              height: 1.4,
            ),
          ),
        ],
      ],
    );
  }
}

class _CouponTextField extends StatelessWidget {
  const _CouponTextField({
    required this.controller,
    required this.label,
    required this.hintText,
    this.prefixText,
    this.suffixText,
    this.helperText,
    this.keyboardType,
    this.inputFormatters,
    this.textInputAction,
    this.maxLength,
  });

  final TextEditingController controller;
  final String label;
  final String hintText;
  final String? prefixText;
  final String? suffixText;
  final String? helperText;
  final TextInputType? keyboardType;
  final List<TextInputFormatter>? inputFormatters;
  final TextInputAction? textInputAction;
  final int? maxLength;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _FieldLabel(label: label),
        const SizedBox(height: HkSpace.sm),
        TextField(
          controller: controller,
          keyboardType: keyboardType,
          inputFormatters: inputFormatters,
          textInputAction: textInputAction,
          maxLength: maxLength,
          style: const TextStyle(
            color: AppColors.textEmphasis,
            fontSize: 16,
            fontWeight: FontWeight.w500,
          ),
          decoration: InputDecoration(
            hintText: hintText,
            counterText: '',
            prefixText: prefixText,
            suffixText: suffixText,
            helperText: helperText,
            helperMaxLines: 2,
            filled: true,
            fillColor: AppColors.surfacePage,
            constraints: const BoxConstraints(minHeight: 52),
            contentPadding: const EdgeInsets.symmetric(
              horizontal: HkSpace.lg,
              vertical: HkSpace.md,
            ),
            hintStyle: const TextStyle(
              color: AppColors.textHint,
              fontSize: 16,
              fontWeight: FontWeight.w400,
            ),
            prefixStyle: const TextStyle(
              color: AppColors.brandPrimaryStrong,
              fontSize: 16,
              fontWeight: FontWeight.w700,
            ),
            suffixStyle: const TextStyle(
              color: AppColors.brandPrimaryStrong,
              fontSize: 16,
              fontWeight: FontWeight.w700,
            ),
            helperStyle: const TextStyle(
              color: AppColors.textMuted,
              fontSize: 12,
              height: 1.4,
            ),
            border: _border(AppColors.borderInput),
            enabledBorder: _border(AppColors.borderInput),
            focusedBorder: _border(AppColors.brandPrimaryStrong, width: 1.5),
          ),
        ),
      ],
    );
  }

  static OutlineInputBorder _border(Color color, {double width = 1}) {
    return OutlineInputBorder(
      borderRadius: BorderRadius.circular(HkRadius.md),
      borderSide: BorderSide(color: color, width: width),
    );
  }
}

class _DiscountRuleButton extends StatelessWidget {
  const _DiscountRuleButton({
    required this.label,
    required this.description,
    required this.icon,
    required this.isSelected,
    required this.onTap,
  });

  final String label;
  final String description;
  final HkIconName icon;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      label: '$label，$description',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: AnimatedContainer(
            duration: HkMotion.duration,
            curve: HkMotion.ease,
            constraints: const BoxConstraints(minHeight: 76),
            padding: const EdgeInsets.all(HkSpace.md),
            decoration: BoxDecoration(
              color: isSelected ? AppColors.primarySoft : AppColors.surfacePage,
              borderRadius: BorderRadius.circular(HkRadius.md),
              border: Border.all(
                color: isSelected
                    ? AppColors.borderPrimary
                    : AppColors.borderInput,
                width: isSelected ? 1.5 : 1,
              ),
            ),
            child: Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: isSelected
                        ? AppColors.brandPrimaryStrong
                        : AppColors.surfaceMuted,
                    borderRadius: BorderRadius.circular(HkRadius.sm),
                  ),
                  child: Center(
                    child: HkIcon(
                      icon,
                      size: 19,
                      color: isSelected
                          ? AppIconColors.onBrand
                          : AppIconColors.muted,
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        label,
                        style: TextStyle(
                          color: isSelected
                              ? AppColors.brandPrimaryStrong
                              : AppColors.textEmphasis,
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        description,
                        maxLines: 2,
                        style: const TextStyle(
                          color: AppColors.textMuted,
                          fontSize: 12,
                          height: 1.3,
                        ),
                      ),
                    ],
                  ),
                ),
                if (isSelected)
                  const HkIcon(
                    HkIconName.check,
                    size: 18,
                    color: AppIconColors.active,
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
