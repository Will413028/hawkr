import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// Bottom-sheet drag affordance rendered inside the sheet surface.
class HkSheetDragHandle extends StatelessWidget {
  const HkSheetDragHandle({super.key});

  @override
  Widget build(BuildContext context) {
    return ExcludeSemantics(
      child: Padding(
        padding: const EdgeInsets.only(top: HkSpace.sm, bottom: HkSpace.xs),
        child: Center(
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: AppColors.dragHandle,
              borderRadius: BorderRadius.circular(HkRadius.pill),
            ),
            child: const SizedBox(width: 32, height: 4),
          ),
        ),
      ),
    );
  }
}
