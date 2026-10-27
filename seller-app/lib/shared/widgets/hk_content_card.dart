import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// Hawkr Seller 的標準內容卡。
///
/// 長列表、表單與營運資料使用接近不透明的內容層，不在每張卡片重複
/// BackdropFilter。玻璃只保留給導覽與暫時性 controls。
class HkContentCard extends StatelessWidget {
  const HkContentCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(HkSpace.lg),
    this.margin = EdgeInsets.zero,
    this.onTap,
    this.semanticLabel,
    this.backgroundColor = AppColors.surfaceCard,
    this.borderColor = AppColors.cardBorder,
    this.radius = HkRadius.lg,
    this.elevated = false,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;
  final EdgeInsetsGeometry margin;
  final VoidCallback? onTap;
  final String? semanticLabel;
  final Color backgroundColor;
  final Color borderColor;
  final double radius;
  final bool elevated;

  @override
  Widget build(BuildContext context) {
    final borderRadius = BorderRadius.circular(radius);
    Widget content = Padding(padding: padding, child: child);

    if (onTap != null) {
      content = Semantics(
        button: true,
        label: semanticLabel,
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: borderRadius,
            child: content,
          ),
        ),
      );
    }

    return Padding(
      padding: margin,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: backgroundColor,
          borderRadius: borderRadius,
          border: Border.all(color: borderColor),
          boxShadow: elevated ? HkShadows.card : HkShadows.xs,
        ),
        child: ClipRRect(borderRadius: borderRadius, child: content),
      ),
    );
  }
}
