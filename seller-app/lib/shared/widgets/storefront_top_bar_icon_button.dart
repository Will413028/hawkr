import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_glass_chrome_surface.dart';
import 'hk_icon.dart';

/// 買家店舖與賣家店舖共用的頂部玻璃圖示按鈕。
///
/// 圖示固定為 24×24、視覺控制維持 32×32，但外層觸控區固定為 48×48，
/// 避免不同圖示來源忽大忽小，也不讓小型圓形按鈕犧牲可用性。
class StorefrontTopBarIconButton extends StatelessWidget {
  const StorefrontTopBarIconButton({
    super.key,
    required this.semanticLabel,
    required this.onPressed,
    required this.icon,
    this.iconColor = AppIconColors.active,
  });

  final String semanticLabel;
  final VoidCallback? onPressed;
  final HkIconName icon;
  final Color iconColor;

  static const double tapTargetSize = HkControl.minTap;
  static const double visualSize = 32;
  static const double iconSize = HkControl.icon;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: semanticLabel,
      excludeFromSemantics: true,
      child: Semantics(
        label: semanticLabel,
        button: true,
        enabled: onPressed != null,
        child: SizedBox.square(
          dimension: tapTargetSize,
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: onPressed,
              customBorder: const CircleBorder(),
              child: Center(
                child: SizedBox.square(
                  dimension: visualSize,
                  child: HkGlassChromeSurface(
                    height: visualSize,
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                    backgroundColor: AppColors.neutralWhite,
                    surfaceOpacity: 0.88,
                    showBottomBorder: false,
                    child: Center(
                      child: HkIcon(icon, size: iconSize, color: iconColor),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
