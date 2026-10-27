import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_icon.dart';
import 'hk_liquid_glass.dart';

// Shared top bar height (EXCLUDES status bar)
// 對齊網頁 mobile header，固定 logical px，避免不同螢幕高度切換時跳動。
const double kTopBarHeight = HkControl.topBarHeight;

EdgeInsetsGeometry _defaultHorizontalPadding() =>
    const EdgeInsets.symmetric(horizontal: HkSpace.md);

EdgeInsets _safeHorizontalPadding(
  BuildContext context,
  EdgeInsetsGeometry? padding,
) {
  final resolved = (padding ?? _defaultHorizontalPadding()).resolve(
    Directionality.of(context),
  );
  final safe = MediaQuery.paddingOf(context);
  return EdgeInsets.fromLTRB(
    resolved.left + safe.left,
    resolved.top,
    resolved.right + safe.right,
    resolved.bottom,
  );
}

Widget _constrainedChromeChild({required Widget child}) => Center(
  child: ConstrainedBox(
    constraints: const BoxConstraints(maxWidth: HkControl.chromeMaxWidth),
    child: child,
  ),
);

Widget _topBarSurface({
  required BuildContext context,
  required Widget child,
  required Color backgroundColor,
  required BorderRadiusGeometry? borderRadius,
  required List<BoxShadow>? boxShadow,
  required bool showBottomBorder,
  required bool frostedGlass,
}) {
  if (!frostedGlass) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: borderRadius,
        border: showBottomBorder
            ? const Border(bottom: BorderSide(color: AppColors.borderSubtle))
            : null,
        boxShadow: boxShadow,
      ),
      child: child,
    );
  }

  return HkLiquidGlassSurface(
    backgroundColor: backgroundColor,
    surfaceOpacity: HkGlass.topBarSurfaceOpacity,
    blurSigma: HkGlass.topBarBlurSigma,
    borderRadius: borderRadius ?? BorderRadius.zero,
    border: showBottomBorder
        ? const Border(bottom: BorderSide(color: AppColors.borderSubtle))
        : null,
    showBorder: showBottomBorder,
    shadows: boxShadow,
    child: child,
  );
}

/// Generic top bar container used inside page bodies (not Scaffold.appBar)
/// It renders the website-aligned cream chrome, accounts for status bar padding, and fixes
/// the inner content height to kTopBarHeight. Provide the row/content via `child`.
Widget commonTopBar({
  required BuildContext context,
  required Widget child,
  Color backgroundColor = AppColors.topBarGlass,
  EdgeInsetsGeometry? horizontalPadding,
  SystemUiOverlayStyle systemOverlayStyle = AppColors.glassTopOverlay,
  BorderRadiusGeometry? borderRadius,
  List<BoxShadow>? boxShadow,
  bool showBottomBorder = true,
  bool frostedGlass = true,
}) {
  return AnnotatedRegion<SystemUiOverlayStyle>(
    value: systemOverlayStyle,
    child: _topBarSurface(
      context: context,
      backgroundColor: backgroundColor,
      borderRadius: borderRadius,
      boxShadow: boxShadow,
      showBottomBorder: showBottomBorder,
      frostedGlass: frostedGlass,
      child: Padding(
        padding: EdgeInsets.only(top: MediaQuery.of(context).padding.top),
        child: SizedBox(
          height: kTopBarHeight,
          child: Padding(
            padding: _safeHorizontalPadding(context, horizontalPadding),
            child: _constrainedChromeChild(
              child: IconTheme(
                data: const IconThemeData(color: AppColors.chromeForeground),
                child: child,
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

/// Build a PreferredSize app bar with fixed content height of kTopBarHeight,
/// including the device status bar padding.
PreferredSize buildCommonAppBar(
  BuildContext context, {
  required Widget child,
  Color backgroundColor = AppColors.topBarGlass,
  EdgeInsetsGeometry? horizontalPadding,
  SystemUiOverlayStyle systemOverlayStyle = AppColors.glassTopOverlay,
  bool frostedGlass = true,
}) {
  final double top = MediaQuery.of(context).padding.top;
  return PreferredSize(
    preferredSize: Size.fromHeight(top + kTopBarHeight),
    child: AnnotatedRegion<SystemUiOverlayStyle>(
      value: systemOverlayStyle,
      child: _topBarSurface(
        context: context,
        backgroundColor: backgroundColor,
        borderRadius: null,
        boxShadow: null,
        showBottomBorder: true,
        frostedGlass: frostedGlass,
        child: Padding(
          padding: EdgeInsets.only(top: top),
          child: SizedBox(
            height: kTopBarHeight,
            child: Padding(
              padding: _safeHorizontalPadding(context, horizontalPadding),
              child: _constrainedChromeChild(
                child: IconTheme(
                  data: const IconThemeData(color: AppColors.chromeForeground),
                  child: child,
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

/// 標題列 Row 的核心內容（48 tap target + 標題 + actions）
/// 給 [buildCommonTitleAppBar] 與 [commonTitleTopBar] 共用，確保兩種容器產出相同視覺
Widget _titleBarRow(
  BuildContext context, {
  required String title,
  required bool showBack,
  required VoidCallback? onBack,
  required List<Widget> actions,
  required Color titleColor,
  required IconData backIcon,
}) {
  return Row(
    children: [
      if (showBack)
        Semantics(
          label: '返回',
          button: true,
          onTap: onBack ?? () => Navigator.maybePop(context),
          child: ExcludeSemantics(
            child: GestureDetector(
              onTap: onBack ?? () => Navigator.maybePop(context),
              behavior: HitTestBehavior.opaque,
              child: SizedBox.square(
                dimension: HkControl.minTap,
                child: Center(
                  child:
                      backIcon == AppIcons.arrowBack ||
                          backIcon == AppIcons.chevronLeft ||
                          backIcon == AppIcons.arrowBackIosNewRounded
                      ? HkIcon(
                          HkIconName.back,
                          color: titleColor,
                          size: HkControl.icon,
                        )
                      : Icon(backIcon, color: titleColor, size: HkControl.icon),
                ),
              ),
            ),
          ),
        ),
      if (showBack) const SizedBox(width: HkSpace.sm),
      Expanded(
        child: Text(
          title,
          textAlign: TextAlign.center,
          style: TextStyle(
            color: titleColor,
            fontSize: 20,
            fontWeight: FontWeight.w700,
          ),
          overflow: TextOverflow.ellipsis,
        ),
      ),
      if (actions.isEmpty)
        SizedBox(width: showBack ? HkControl.minTap + HkSpace.sm : 0)
      else ...[
        const SizedBox(width: HkSpace.sm),
        for (var i = 0; i < actions.length; i++) ...[
          actions[i],
          if (i != actions.length - 1) const SizedBox(width: HkSpace.sm),
        ],
      ],
    ],
  );
}

/// Convenience for a simple centered title with optional back button and actions
PreferredSize buildCommonTitleAppBar(
  BuildContext context, {
  required String title,
  bool showBack = true,
  VoidCallback? onBack,
  List<Widget> actions = const <Widget>[],
  Color backgroundColor = AppColors.topBarGlass,
  Color titleColor = AppColors.chromeForeground,
  IconData backIcon = AppIcons.arrowBack,
}) {
  return buildCommonAppBar(
    context,
    backgroundColor: backgroundColor,
    systemOverlayStyle: AppColors.glassTopOverlay,
    frostedGlass: true,
    child: _titleBarRow(
      context,
      title: title,
      showBack: showBack,
      onBack: onBack,
      actions: actions,
      titleColor: titleColor,
      backIcon: backIcon,
    ),
  );
}

/// Inline (non-Scaffold.appBar) 版本：用在 Column 第一個 child 等不使用 appBar slot 的場合
Widget commonTitleTopBar(
  BuildContext context, {
  required String title,
  bool showBack = true,
  VoidCallback? onBack,
  List<Widget> actions = const <Widget>[],
  Color backgroundColor = AppColors.topBarGlass,
  Color titleColor = AppColors.chromeForeground,
  IconData backIcon = AppIcons.arrowBack,
}) {
  return commonTopBar(
    context: context,
    backgroundColor: backgroundColor,
    systemOverlayStyle: AppColors.glassTopOverlay,
    frostedGlass: true,
    child: _titleBarRow(
      context,
      title: title,
      showBack: showBack,
      onBack: onBack,
      actions: actions,
      titleColor: titleColor,
      backIcon: backIcon,
    ),
  );
}
