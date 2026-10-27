import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';

import '../../../../core/theme/app_colors.dart';

/// 賣家 AI 助手專用的頭像。
class AiAssistantAvatar extends StatelessWidget {
  const AiAssistantAvatar({
    super.key,
    required this.size,
    required this.isReplying,
    this.showBorder = true,
  });

  final double size;
  final bool isReplying;
  final bool showBorder;

  @override
  Widget build(BuildContext context) {
    final padding = size * 0.10;
    return Semantics(
      image: true,
      label: isReplying ? 'AI 營運助手正在回覆' : 'AI 營運助手',
      child: ExcludeSemantics(
        child: Container(
          width: size,
          height: size,
          padding: EdgeInsets.all(padding),
          decoration: BoxDecoration(
            color: AppColors.neutralWhite.withValues(alpha: 0.76),
            shape: BoxShape.circle,
            border: showBorder
                ? Border.all(color: AppColors.borderStrong, width: 1.5)
                : null,
          ),
          clipBehavior: Clip.antiAlias,
          child: Icon(
            AppIcons.autoAwesome,
            key: const ValueKey('seller-ai-assistant-avatar-icon'),
            size: size - padding * 2,
            color: AppIconColors.active,
          ),
        ),
      ),
    );
  }
}
