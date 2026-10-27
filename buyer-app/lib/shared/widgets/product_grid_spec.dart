import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../core/theme/hk_tokens.dart';

/// Shared responsive geometry for buyer-facing product grids.
///
/// The Web catalogue keeps product media at 4:3 and lets the information
/// panel occupy 72 logical pixels at the default text scale. Computing
/// [mainAxisExtent] from the incoming width and bounded accessibility scale
/// avoids ScreenUtil-dependent card heights and keeps every product surface
/// aligned without clipping enlarged text.
@immutable
class ProductGridSpec {
  const ProductGridSpec._({
    required this.viewportWidth,
    required this.columnCount,
    required this.horizontalInset,
    required this.crossAxisSpacing,
    required this.mainAxisSpacing,
    required this.itemWidth,
    required this.mainAxisExtent,
  });

  static const double mediaAspectRatio = 4 / 3;
  static const double bodyExtent = 72;
  static const double bodyTextScaleGrowth = 20;
  static const double maxContentWidth = 1120;
  static const double twoColumnMaxWidth = 680;

  final double viewportWidth;
  final int columnCount;
  final double horizontalInset;
  final double crossAxisSpacing;
  final double mainAxisSpacing;
  final double itemWidth;
  final double mainAxisExtent;

  EdgeInsets get padding => EdgeInsets.symmetric(horizontal: horizontalInset);

  SliverGridDelegateWithFixedCrossAxisCount get delegate =>
      SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: columnCount,
        crossAxisSpacing: crossAxisSpacing,
        mainAxisSpacing: mainAxisSpacing,
        mainAxisExtent: mainAxisExtent,
      );

  factory ProductGridSpec.forWidth(
    double viewportWidth, {
    double textScale = 1,
  }) {
    final safeWidth = math.max(0.0, viewportWidth);
    final safeTextScale = textScale.clamp(1.0, 1.4).toDouble();
    final columnCount = safeWidth <= HkBreakpoints.compact
        ? 1
        : safeWidth >= HkBreakpoints.wide
        ? 4
        : safeWidth > twoColumnMaxWidth
        ? 3
        : 2;
    final baseInset = safeWidth >= HkBreakpoints.wide
        ? HkSpace.section
        : safeWidth >= HkBreakpoints.tablet
        ? HkSpace.xxl
        : safeWidth < HkBreakpoints.compact
        ? HkSpace.md
        : HkSpace.lg;
    final centeredInset = math.max(
      baseInset,
      (safeWidth - maxContentWidth) / 2,
    );
    final spacing = safeWidth < HkBreakpoints.compact
        ? HkSpace.sm
        : safeWidth >= HkBreakpoints.tablet
        ? HkSpace.lg
        : HkSpace.md;
    final usableWidth = math.max(
      0.0,
      safeWidth - centeredInset * 2 - spacing * (columnCount - 1),
    );
    final itemWidth = usableWidth / columnCount;
    final imageExtent = itemWidth / mediaAspectRatio;
    final scaledBodyExtent =
        bodyExtent + (safeTextScale - 1) * bodyTextScaleGrowth;

    return ProductGridSpec._(
      viewportWidth: safeWidth,
      columnCount: columnCount,
      horizontalInset: centeredInset,
      crossAxisSpacing: spacing,
      mainAxisSpacing: spacing,
      itemWidth: itemWidth,
      mainAxisExtent: imageExtent + scaledBodyExtent,
    );
  }
}
