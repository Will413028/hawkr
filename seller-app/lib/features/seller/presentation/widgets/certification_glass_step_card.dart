import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../../shared/widgets/hk_icon.dart';

/// 商家認證各資料步驟共用的 iOS 26 白色霧面玻璃主卡。
///
/// 將標題、圖示、說明與表單容器收斂成同一層級，避免個人與企業
/// 流程在不同步驟出現間距、圓角與資訊階層漂移。
class CertificationGlassStepCard extends StatelessWidget {
  const CertificationGlassStepCard({
    super.key,
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.child,
  });

  final String title;
  final String subtitle;
  final HkIconName icon;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final inputBorder = OutlineInputBorder(
      borderRadius: BorderRadius.circular(HkRadius.md),
      borderSide: BorderSide(
        color: AppColors.brandPrimaryStrong.withValues(alpha: 0.14),
      ),
    );
    final theme = Theme.of(context);

    return Semantics(
      container: true,
      label: '$title，$subtitle',
      child: HkFrostedCardSurface(
        radius: HkRadius.xl,
        blurSigma: HkGlass.dialogBlurSigma,
        shadows: HkShadows.lg,
        child: Theme(
          data: theme.copyWith(
            inputDecorationTheme: theme.inputDecorationTheme.copyWith(
              filled: true,
              fillColor: AppColors.neutralWhite.withValues(alpha: 0.72),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: HkSpace.lg,
                vertical: HkSpace.md,
              ),
              border: inputBorder,
              enabledBorder: inputBorder,
              focusedBorder: inputBorder.copyWith(
                borderSide: const BorderSide(
                  color: AppColors.brandPrimaryStrong,
                  width: 1.5,
                ),
              ),
              errorBorder: inputBorder.copyWith(
                borderSide: const BorderSide(color: AppColors.dangerStrong),
              ),
              focusedErrorBorder: inputBorder.copyWith(
                borderSide: const BorderSide(
                  color: AppColors.dangerStrong,
                  width: 1.5,
                ),
              ),
            ),
          ),
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.xxl),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: HkSpace.control,
                      height: HkSpace.control,
                      decoration: BoxDecoration(
                        color: AppColors.neutralWhite.withValues(alpha: 0.68),
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: AppColors.brandSecondary.withValues(
                            alpha: 0.72,
                          ),
                        ),
                        boxShadow: HkShadows.sm,
                      ),
                      child: Center(
                        child: HkIcon(
                          icon,
                          size: HkControl.icon,
                          color: AppIconColors.active,
                        ),
                      ),
                    ),
                    const SizedBox(width: HkSpace.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(title, style: AppTypography.h1),
                          const SizedBox(height: HkSpace.xs),
                          Text(subtitle, style: AppTypography.metaMuted),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: HkSpace.xl),
                Divider(
                  height: 1,
                  color: AppColors.brandPrimaryStrong.withValues(alpha: 0.10),
                ),
                const SizedBox(height: HkSpace.xl),
                child,
              ],
            ),
          ),
        ),
      ),
    );
  }
}
