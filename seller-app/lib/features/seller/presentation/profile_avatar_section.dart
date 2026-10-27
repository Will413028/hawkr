import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';

class ProfileAvatarCircle extends StatelessWidget {
  const ProfileAvatarCircle({super.key, required this.child, this.size = 118});

  final Widget child;
  final double size;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: const BoxDecoration(
        shape: BoxShape.circle,
        color: AppColors.surfaceCard,
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 20,
            offset: Offset(0, 8),
          ),
        ],
      ),
      child: SizedBox.square(
        dimension: size,
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.sm),
          child: ClipOval(child: child),
        ),
      ),
    );
  }
}
