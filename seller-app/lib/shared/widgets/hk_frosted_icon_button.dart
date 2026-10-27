import 'package:flutter/material.dart';

import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_frosted_card_surface.dart';

/// Circular frosted-glass action shared by message composers and more menus.
///
/// The visual surface and tap target stay 48 logical pixels with a 24px icon.
/// Callers own validation, submission and focus; loading disables the action
/// while keeping the same circular surface and layout.
class HkFrostedIconButton extends StatelessWidget {
  const HkFrostedIconButton({
    super.key,
    required this.label,
    required this.icon,
    required this.onPressed,
    this.loading = false,
    this.loadingLabel = '處理中',
    this.semanticHint,
  });

  final String label;
  final IconData icon;
  final VoidCallback? onPressed;
  final bool loading;
  final String loadingLabel;
  final String? semanticHint;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null && !loading;
    return Semantics(
      label: loading ? loadingLabel : label,
      hint: semanticHint,
      button: true,
      enabled: enabled,
      onTap: enabled ? onPressed : null,
      child: ExcludeSemantics(
        child: SizedBox.square(
          dimension: HkControl.minTap,
          child: HkFrostedCardSurface(
            radius: HkRadius.pill,
            shadows: HkShadows.sm,
            child: Material(
              color: Colors.transparent,
              shape: const CircleBorder(),
              clipBehavior: Clip.antiAlias,
              child: IconButton(
                tooltip: label,
                onPressed: enabled ? onPressed : null,
                style: IconButton.styleFrom(
                  shape: const CircleBorder(),
                  side: BorderSide.none,
                  padding: EdgeInsets.zero,
                ),
                icon: loading
                    ? SizedBox.square(
                        dimension: HkControl.icon,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          semanticsLabel: loadingLabel,
                        ),
                      )
                    : Icon(
                        icon,
                        color: enabled
                            ? AppIconColors.active
                            : AppIconColors.disabled,
                        size: HkControl.icon,
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
