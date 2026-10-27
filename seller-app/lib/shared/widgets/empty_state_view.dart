import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';

/// 通用的空状态组件，用于展示“暂无数据”类页面内容。
///
/// 调用方可自行传入图标、插图与文案，让同一套布局同时适用于
/// icon 风格与插图风格的空状态。
class EmptyStateView extends StatelessWidget {
  const EmptyStateView({
    super.key,
    required this.visual,
    required this.message,
    this.messageStyle,
    this.spacing = 12,
    this.visualFirst = true,
    this.mainAxisSize = MainAxisSize.min,
    this.padding,
  });

  /// 适用于需要直接传入 IconButton 的场景。
  ///
  /// 例如空状态里需要放一个可点击的刷新按钮或跳转按钮时，可直接使用此构造。
  factory EmptyStateView.iconButton({
    Key? key,
    required IconButton iconButton,
    required String message,
    TextStyle? messageStyle,
    double spacing = 12,
    bool visualFirst = true,
    MainAxisSize mainAxisSize = MainAxisSize.min,
    EdgeInsetsGeometry? padding,
  }) {
    return EmptyStateView(
      key: key,
      visual: iconButton,
      message: message,
      messageStyle: messageStyle,
      spacing: spacing,
      visualFirst: visualFirst,
      mainAxisSize: mainAxisSize,
      padding: padding,
    );
  }

  /// 适用于使用本地资产图片作为空状态视觉元素的场景。
  ///
  /// 若页面需要更复杂的图片形式，例如网络图、文件图或自定义裁切，
  /// 仍可继续通过 `visual` 直接传入任意 Widget。
  factory EmptyStateView.assetImage({
    Key? key,
    required String assetName,
    required String message,
    TextStyle? messageStyle,
    double spacing = 12,
    bool visualFirst = true,
    MainAxisSize mainAxisSize = MainAxisSize.min,
    EdgeInsetsGeometry? padding,
    double? imageWidth,
    double? imageHeight,
    BoxFit fit = BoxFit.contain,
    double imageOpacity = 1,
  }) {
    assert(
      imageOpacity >= 0 && imageOpacity <= 1,
      'imageOpacity 必须介于 0 到 1 之间。',
    );

    final imageWidget = Image.asset(
      assetName,
      width: imageWidth,
      height: imageHeight,
      fit: fit,
    );

    return EmptyStateView(
      key: key,
      visual: imageOpacity == 1
          ? imageWidget
          : Opacity(opacity: imageOpacity, child: imageWidget),
      message: message,
      messageStyle: messageStyle,
      spacing: spacing,
      visualFirst: visualFirst,
      mainAxisSize: mainAxisSize,
      padding: padding,
    );
  }

  /// 与文案一起展示的视觉元素，例如 Icon 或 Image。
  final Widget visual;

  /// 展示给用户的主要空状态文案。
  final String message;

  /// 允许页面按需覆盖空状态文案样式。
  final TextStyle? messageStyle;

  /// 视觉元素与文案之间的垂直间距。
  final double spacing;

  /// 控制视觉元素是否显示在文案前面。
  final bool visualFirst;

  /// 允许页面控制内容区 Column 的包裹方式。
  final MainAxisSize mainAxisSize;

  /// 允许页面在外层追加内边距，便于微调间距。
  final EdgeInsetsGeometry? padding;

  @override
  Widget build(BuildContext context) {
    final messageWidget = Text(
      message,
      style:
          messageStyle ??
          TextStyle(fontSize: 16, color: AppColors.textSecondary),
      textAlign: TextAlign.center,
    );

    final children = visualFirst
        ? <Widget>[visual, SizedBox(height: spacing), messageWidget]
        : <Widget>[messageWidget, SizedBox(height: spacing), visual];

    Widget content = Column(
      mainAxisSize: mainAxisSize,
      mainAxisAlignment: MainAxisAlignment.center,
      children: children,
    );

    if (padding != null) {
      content = Padding(padding: padding!, child: content);
    }

    return Center(child: content);
  }
}
