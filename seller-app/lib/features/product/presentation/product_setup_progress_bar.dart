import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

/// 商品編輯共用步驟列：類別 → 資訊 → 圖片 → 認證 → 規格。
class ProductSetupProgressBar extends StatelessWidget {
  const ProductSetupProgressBar({super.key, required this.currentStep})
    : assert(currentStep >= 1 && currentStep <= 5);

  final int currentStep;

  static const _steps = <(String, HkIconName)>[
    ('類別', HkIconName.market),
    ('資訊', HkIconName.edit),
    ('圖片', HkIconName.photoAdd),
    ('認證', HkIconName.tag),
    ('規格', HkIconName.goods),
  ];

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: const BoxDecoration(
        color: AppColors.surfaceCream,
        border: Border(bottom: BorderSide(color: AppColors.borderSubtle)),
      ),
      child: Row(
        children: List.generate(_steps.length * 2 - 1, (index) {
          if (index.isOdd) {
            final connectorNumber = (index + 1) ~/ 2;
            return Expanded(
              child: Container(
                height: 2,
                margin: const EdgeInsets.only(bottom: 24),
                color: currentStep > connectorNumber
                    ? AppColors.brandPrimaryStrong
                    : AppColors.borderStrong,
              ),
            );
          }

          final stepIndex = index ~/ 2;
          return _Step(
            number: stepIndex + 1,
            currentStep: currentStep,
            label: _steps[stepIndex].$1,
            icon: _steps[stepIndex].$2,
          );
        }),
      ),
    );
  }
}

class _Step extends StatelessWidget {
  const _Step({
    required this.number,
    required this.currentStep,
    required this.label,
    required this.icon,
  });

  final int number;
  final int currentStep;
  final String label;
  final HkIconName icon;

  @override
  Widget build(BuildContext context) {
    final isCompleted = currentStep > number;
    final isActive = currentStep == number;
    final stateLabel = isCompleted
        ? '已完成'
        : isActive
        ? '目前步驟'
        : '尚未開始';

    return Semantics(
      label: '步驟 $number，共 5 步，$label，$stateLabel',
      selected: isActive,
      child: SizedBox(
        width: 44,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                color: isActive
                    ? AppColors.brandPrimaryStrong
                    : isCompleted
                    ? AppColors.successSoft
                    : AppColors.surfaceMuted,
                shape: BoxShape.circle,
                border: Border.all(
                  color: isActive
                      ? AppColors.brandPrimaryStrong
                      : isCompleted
                      ? AppColors.success
                      : AppColors.borderStrong,
                ),
              ),
              child: Center(
                child: HkIcon(
                  isCompleted ? HkIconName.check : icon,
                  size: 19,
                  color: isActive
                      ? AppIconColors.onBrand
                      : isCompleted
                      ? AppIconColors.success
                      : AppIconColors.muted,
                ),
              ),
            ),
            const SizedBox(height: 8),
            Text(
              label,
              maxLines: 1,
              style: TextStyle(
                fontSize: 12,
                fontWeight: isActive ? FontWeight.w700 : FontWeight.w500,
                color: isActive ? AppColors.textEmphasis : AppColors.textMuted,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
