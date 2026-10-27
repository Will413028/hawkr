import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../../shared/widgets/hk_icon.dart';

/// 設定與錢包共用的商家認證旅程。
///
/// 導覽與各步驟主卡共用白色霧面玻璃語言；表單欄位仍保留高不透明白底，
/// 讓 iOS 26 液態玻璃有層級感且不干擾證件資料閱讀。
class CertificationJourneyStepper extends StatelessWidget {
  const CertificationJourneyStepper({super.key, required this.currentStep})
    : assert(currentStep >= 0 && currentStep < stepCount);

  static const int stepCount = 4;

  final int currentStep;

  static const _steps = <({String label, HkIconName icon})>[
    (label: '賣家類型', icon: HkIconName.store),
    (label: '身分資料', icon: HkIconName.shield),
    (label: '收款資料', icon: HkIconName.wallet),
    (label: '送審開通', icon: HkIconName.check),
  ];

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label:
          '認證進度，第 ${currentStep + 1} 步，共 $stepCount 步，'
          '${_steps[currentStep].label}',
      child: ExcludeSemantics(
        child: HkFrostedCardSurface(
          radius: HkRadius.xl,
          blurSigma: HkGlass.topBarBlurSigma,
          shadows: HkShadows.sm,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              HkSpace.sm,
              HkSpace.md,
              HkSpace.sm,
              HkSpace.sm,
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (var index = 0; index < _steps.length; index++) ...[
                  Expanded(
                    child: _JourneyStep(
                      step: _steps[index],
                      index: index,
                      currentStep: currentStep,
                    ),
                  ),
                  if (index < _steps.length - 1)
                    _StepConnector(completed: index < currentStep),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _JourneyStep extends StatelessWidget {
  const _JourneyStep({
    required this.step,
    required this.index,
    required this.currentStep,
  });

  final ({String label, HkIconName icon}) step;
  final int index;
  final int currentStep;

  @override
  Widget build(BuildContext context) {
    final completed = index < currentStep;
    final active = index == currentStep;
    final foreground = completed || active
        ? AppIconColors.onBrand
        : AppIconColors.muted;
    final background = completed
        ? AppColors.brandPrimaryStrong
        : active
        ? AppColors.brandPrimaryStrong
        : AppColors.neutralCream;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        AnimatedContainer(
          duration: MediaQuery.maybeOf(context)?.disableAnimations ?? false
              ? Duration.zero
              : HkMotion.duration,
          curve: HkMotion.ease,
          width: 32,
          height: 32,
          decoration: BoxDecoration(
            color: background,
            shape: BoxShape.circle,
            border: Border.all(
              color: active
                  ? AppColors.brandSecondary
                  : AppColors.neutralWhite.withValues(alpha: 0.88),
              width: active ? 1.5 : 1,
            ),
          ),
          child: Center(
            child: HkIcon(
              completed ? HkIconName.check : step.icon,
              size: 17,
              color: foreground,
            ),
          ),
        ),
        const SizedBox(height: HkSpace.xs),
        Text(
          step.label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          textAlign: TextAlign.center,
          style: AppTypography.caption.copyWith(
            color: active || completed
                ? AppColors.textEmphasis
                : AppColors.textMuted,
            fontWeight: active ? FontWeight.w700 : FontWeight.w500,
          ),
        ),
      ],
    );
  }
}

class _StepConnector extends StatelessWidget {
  const _StepConnector({required this.completed});

  final bool completed;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 15),
      child: AnimatedContainer(
        duration: MediaQuery.maybeOf(context)?.disableAnimations ?? false
            ? Duration.zero
            : HkMotion.duration,
        curve: HkMotion.ease,
        width: HkSpace.sm,
        height: 2,
        decoration: BoxDecoration(
          color: completed
              ? AppColors.brandPrimaryStrong
              : AppColors.cardBorder,
          borderRadius: BorderRadius.circular(HkRadius.pill),
        ),
      ),
    );
  }
}
