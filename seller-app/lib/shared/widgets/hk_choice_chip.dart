import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';

/// Hawkr 統一的緊湊型篩選／選擇控制。
///
/// 膠囊本體保持精簡，外層觸控範圍固定為 48pt，兼顧資訊密度與可用性。
class HkChoiceChip extends StatelessWidget {
  const HkChoiceChip({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
    this.icon,
    this.selectedIcon,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  /// Leading glyph; [selectedIcon] replaces it while selected so the state
  /// shows in the shape and not only in the fill.
  final IconData? icon;
  final IconData? selectedIcon;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      child: SizedBox(
        height: HkControl.minTap,
        child: Center(
          widthFactor: 1,
          child: Material(
            color: selected ? AppColors.actionPrimary : Colors.transparent,
            shape: StadiumBorder(
              side: BorderSide(
                color: selected
                    ? AppColors.actionPrimary
                    : AppColors.borderSoft,
              ),
            ),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: onTap,
              child: SizedBox(
                height: HkControl.compactChoiceHeight,
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: HkControl.compactChoiceHorizontalPadding,
                  ),
                  child: Center(
                    widthFactor: 1,
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if (icon != null) ...[
                          ExcludeSemantics(
                            child: Icon(
                              selected ? selectedIcon ?? icon : icon,
                              size: HkControl.compactChoiceFontSize + 4,
                              color: selected
                                  ? AppIconColors.onBrand
                                  : AppIconColors.muted,
                            ),
                          ),
                          const SizedBox(width: 4),
                        ],
                        Flexible(child: _label()),
                      ],
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

  Widget _label() {
    return Text(
      label,
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(
        fontSize: HkControl.compactChoiceFontSize,
        height: 1,
        fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
        color: selected ? AppColors.textInverse : AppColors.textSecondary,
      ),
    );
  }
}
