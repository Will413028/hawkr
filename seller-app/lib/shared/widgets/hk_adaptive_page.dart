import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';

/// 將內容約束在可閱讀寬度，並從可用寬度決定 gutter。
///
/// 不判斷手機或平板型號；所有頁面都可直接放入 sliver/list/form。
class HkAdaptivePage extends StatelessWidget {
  const HkAdaptivePage({
    super.key,
    required this.child,
    this.maxWidth,
    this.padding,
    this.alignment = Alignment.topCenter,
  });

  final Widget child;
  final double? maxWidth;
  final EdgeInsetsGeometry? padding;
  final AlignmentGeometry alignment;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        final gutter = HkBreakpoints.pageGutter(width);
        final resolvedMaxWidth =
            maxWidth ?? HkBreakpoints.contentMaxWidth(width);

        return ColoredBox(
          color: AppColors.surfacePage,
          child: Align(
            alignment: alignment,
            child: ConstrainedBox(
              constraints: BoxConstraints(maxWidth: resolvedMaxWidth),
              child: Padding(
                padding: padding ?? EdgeInsets.symmetric(horizontal: gutter),
                child: child,
              ),
            ),
          ),
        );
      },
    );
  }
}
