import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/icons/app_icons.dart';

/// AI 客服頭像：圓形底框內放 Material 圖示。
///
/// 圖示本身是靜態的；[isReplying] 只影響無障礙標籤。
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
    final padding = size * 0.12;
    return Semantics(
      image: true,
      label: isReplying ? 'AI 客服正在回覆' : 'AI 客服',
      child: ExcludeSemantics(
        child: Container(
          width: size,
          height: size,
          padding: EdgeInsets.all(padding),
          decoration: BoxDecoration(
            color: AppColors.primarySoft,
            shape: BoxShape.circle,
            border: showBorder
                ? Border.all(color: AppColors.borderStrong, width: 1.8)
                : null,
          ),
          clipBehavior: Clip.antiAlias,
          child: FittedBox(
            child: Icon(
              AppIcons.smartToyOutlined,
              key: const ValueKey('ai-assistant-avatar-icon'),
              size: size - padding * 2,
              color: AppIconColors.strong,
            ),
          ),
        ),
      ),
    );
  }
}
