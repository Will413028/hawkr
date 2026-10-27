import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// 頁面區塊的單一標題階層。
class HkPageSection extends StatelessWidget {
  const HkPageSection({
    super.key,
    required this.title,
    this.child,
    this.description,
    this.trailing,
    this.spacing = HkSpace.md,
  });

  final String title;
  final String? description;
  final Widget? trailing;
  final Widget? child;
  final double spacing;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      explicitChildNodes: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Semantics(
                      header: true,
                      child: Text(
                        title,
                        style: const TextStyle(
                          color: AppColors.textEmphasis,
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                          height: 1.5,
                        ),
                      ),
                    ),
                    if (description case final description?) ...[
                      const SizedBox(height: HkSpace.xs),
                      Text(
                        description,
                        style: const TextStyle(
                          color: AppColors.textSecondary,
                          fontSize: 12,
                          fontWeight: FontWeight.w400,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              if (trailing case final trailing?) ...[
                const SizedBox(width: HkSpace.sm),
                trailing,
              ],
            ],
          ),
          if (child case final child?) ...[SizedBox(height: spacing), child],
        ],
      ),
    );
  }
}
