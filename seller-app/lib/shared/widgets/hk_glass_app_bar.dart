import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_glass_chrome_surface.dart';
import 'hk_icon.dart';

/// 可直接放入 [Scaffold.appBar] 的 iOS 26 功能層頁首。
class HkGlassAppBar extends StatelessWidget implements PreferredSizeWidget {
  const HkGlassAppBar({
    super.key,
    this.title,
    this.subtitle,
    this.leading,
    this.actions,
    this.bottom,
    this.showBackButton = true,
    this.onBackPressed,
    this.centerTitle = true,
    this.backgroundColor = AppColors.topBarGlass,
  });

  final Widget? title;
  final String? subtitle;
  final Widget? leading;
  final List<Widget>? actions;
  final PreferredSizeWidget? bottom;
  final bool showBackButton;
  final VoidCallback? onBackPressed;
  final bool centerTitle;
  final Color backgroundColor;

  // Two lines need their own space at the supported 1.4x text scale. Reserving
  // it in preferredSize as well keeps Scaffold's body below the actual bar.
  double get _toolbarHeight =>
      HkControl.topBarHeight + (subtitle == null ? 0 : HkSpace.lg);

  @override
  Size get preferredSize =>
      Size.fromHeight(_toolbarHeight + (bottom?.preferredSize.height ?? 0));

  @override
  Widget build(BuildContext context) {
    final effectiveTitle = subtitle == null
        ? title
        : Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: centerTitle
                ? CrossAxisAlignment.center
                : CrossAxisAlignment.start,
            children: [
              if (title case final title?)
                DefaultTextStyle.merge(
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  child: title,
                ),
              const SizedBox(height: HkSpace.xs),
              Text(
                subtitle!,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 12,
                ),
              ),
            ],
          );

    return AppBar(
      toolbarHeight: _toolbarHeight,
      backgroundColor: Colors.transparent,
      foregroundColor: AppColors.chromeForeground,
      surfaceTintColor: Colors.transparent,
      shadowColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: centerTitle,
      title: effectiveTitle,
      titleTextStyle: const TextStyle(
        color: AppColors.textEmphasis,
        fontSize: 20,
        fontWeight: FontWeight.w700,
      ),
      leading:
          leading ??
          (showBackButton
              ? IconButton(
                  onPressed:
                      onBackPressed ?? () => Navigator.of(context).maybePop(),
                  tooltip: '返回',
                  icon: const HkIcon(
                    HkIconName.back,
                    color: AppIconColors.defaultColor,
                  ),
                )
              : null),
      automaticallyImplyLeading: false,
      actions: actions,
      bottom: bottom,
      flexibleSpace: HkGlassChromeSurface(
        backgroundColor: backgroundColor,
        child: const SizedBox.expand(),
      ),
    );
  }
}
