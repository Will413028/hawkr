import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../../shared/widgets/hk_icon.dart';

/// 店舖三分頁的透明內容層。
///
/// 連續漸層由 PageScreen 的頁面級 SellerGradientCanvas 統一繪製，這裡不再
/// 重啟漸層，避免 Tab、內容與底部導覽之間形成可見色塊。
class StorefrontTabCanvas extends StatelessWidget {
  const StorefrontTabCanvas({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      key: const ValueKey('storefront-transparent-tab-layer'),
      color: Colors.transparent,
      child: child,
    );
  }
}

/// 三個分頁統一使用的白色霧面玻璃空狀態。
class StorefrontGlassEmptyPanel extends StatelessWidget {
  const StorefrontGlassEmptyPanel({
    super.key,
    required this.icon,
    required this.message,
    this.description,
  });

  final HkIconName icon;
  final String message;
  final String? description;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
      child: HkFrostedCardSurface(
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: HkSpace.xxl,
            vertical: HkSpace.section,
          ),
          child: HkEmptyState(
            icon: icon,
            message: message,
            description: description,
            topPadding: 0,
          ),
        ),
      ),
    );
  }
}

/// 店舖分頁的主要新增入口；使用霧面玻璃膠囊，避免與底部導覽搶層級。
class StorefrontGlassActionButton extends StatelessWidget {
  const StorefrontGlassActionButton({
    super.key,
    required this.icon,
    required this.label,
    required this.onTap,
  });

  final HkIconName icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      excludeSemantics: true,
      child: HkFrostedCardSurface(
        radius: HkRadius.pill,
        shadows: HkShadows.lg,
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(HkRadius.pill),
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.xl,
                  vertical: HkSpace.md,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    HkIcon(
                      icon,
                      size: HkControl.icon,
                      color: AppIconColors.active,
                    ),
                    const SizedBox(width: HkSpace.sm),
                    Text(
                      label,
                      style: const TextStyle(
                        color: AppColors.textEmphasis,
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
