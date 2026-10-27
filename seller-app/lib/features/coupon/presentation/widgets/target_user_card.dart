import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

import 'section_card.dart';
import 'section_header.dart';

/// 優惠券發放對象。
///
/// 四個選項直接可見，取代舊版下拉選單，讓商家在送出前能一眼確認範圍。
class CouponTargetUserCard extends StatelessWidget {
  const CouponTargetUserCard({
    super.key,
    required this.targetUserType,
    required this.onTypeSelected,
  });

  final String targetUserType;
  final ValueChanged<String> onTypeSelected;

  static const _options = [
    _TargetOption(
      value: '全部會員',
      title: '全部會員',
      description: '發給所有買家會員',
      icon: HkIconName.users,
    ),
    _TargetOption(
      value: '消費過會員',
      title: '消費過會員',
      description: '只發給曾下單顧客',
      icon: HkIconName.history,
    ),
    _TargetOption(
      value: '指定會員',
      title: '指定會員',
      description: '用手機或 ID 選擇',
      icon: HkIconName.user,
    ),
    _TargetOption(
      value: 'TAG 標籤',
      title: 'TAG 標籤',
      description: '依客戶標籤發放',
      icon: HkIconName.tag,
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const CouponSectionHeader(
            step: '3',
            icon: HkIconName.users,
            title: '選擇發放對象',
            description: '設定誰會收到這張優惠券；指定會員與標籤可再細選。',
          ),
          const SizedBox(height: HkSpace.xxl),
          LayoutBuilder(
            builder: (context, constraints) {
              const spacing = HkSpace.sm;
              final columns = constraints.maxWidth < 250 ? 1 : 2;
              final width =
                  (constraints.maxWidth - (columns - 1) * spacing) / columns;
              return Wrap(
                spacing: spacing,
                runSpacing: spacing,
                children: _options
                    .map(
                      (option) => SizedBox(
                        width: width,
                        child: _TargetOptionButton(
                          option: option,
                          isSelected: option.value == targetUserType,
                          onTap: () => onTypeSelected(option.value),
                        ),
                      ),
                    )
                    .toList(),
              );
            },
          ),
        ],
      ),
    );
  }
}

class _TargetOption {
  const _TargetOption({
    required this.value,
    required this.title,
    required this.description,
    required this.icon,
  });

  final String value;
  final String title;
  final String description;
  final HkIconName icon;
}

class _TargetOptionButton extends StatelessWidget {
  const _TargetOptionButton({
    required this.option,
    required this.isSelected,
    required this.onTap,
  });

  final _TargetOption option;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      label: '${option.title}，${option.description}',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: AnimatedContainer(
            duration: HkMotion.duration,
            curve: HkMotion.ease,
            constraints: const BoxConstraints(minHeight: 96),
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
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: 34,
                      height: 34,
                      decoration: BoxDecoration(
                        color: isSelected
                            ? AppColors.brandPrimaryStrong
                            : AppColors.surfaceMuted,
                        borderRadius: BorderRadius.circular(HkRadius.sm),
                      ),
                      child: Center(
                        child: HkIcon(
                          option.icon,
                          size: 18,
                          color: isSelected
                              ? AppIconColors.onBrand
                              : AppIconColors.muted,
                        ),
                      ),
                    ),
                    const Spacer(),
                    HkIcon(
                      isSelected ? HkIconName.check : HkIconName.chevronRight,
                      size: 18,
                      color: isSelected
                          ? AppIconColors.active
                          : AppIconColors.muted,
                    ),
                  ],
                ),
                const SizedBox(height: HkSpace.sm),
                Text(
                  option.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    color: isSelected
                        ? AppColors.brandPrimaryStrong
                        : AppColors.textEmphasis,
                    fontSize: 14,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  option.description,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 12,
                    height: 1.3,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
