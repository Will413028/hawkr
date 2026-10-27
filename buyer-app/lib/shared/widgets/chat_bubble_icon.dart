import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import '../widgets/hk_icon.dart';

/// 統一聊天氣泡 icon。
class ChatBubbleIcon extends StatelessWidget {
  const ChatBubbleIcon({
    super.key,
    this.size = 24,
    this.color = AppIconColors.defaultColor,
  });

  final double size;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return HkIcon(HkIconName.chat, size: size, color: color);
  }
}
