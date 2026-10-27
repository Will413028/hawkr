import 'dart:math' as math;

import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:flutter/material.dart';

/// Storefront-specific responsive measurements.
///
/// ScreenUtil is intentionally not used here: navigation targets and card
/// geometry must stay in logical pixels, while the layout reacts to the real
/// viewport width, height and text scale.
@immutable
class MerchantLayoutSpec {
  const MerchantLayoutSpec._({
    required this.viewportWidth,
    required this.viewportHeight,
    required this.textScale,
    required this.textScaler,
  });

  factory MerchantLayoutSpec.of(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    final scaler = MediaQuery.textScalerOf(context);
    return MerchantLayoutSpec._(
      viewportWidth: size.width,
      viewportHeight: size.height,
      textScale: (scaler.scale(14) / 14).clamp(1, 2),
      textScaler: scaler,
    );
  }

  factory MerchantLayoutSpec.forSize(Size size, {double textScale = 1}) {
    final resolvedTextScale = textScale.clamp(1, 2).toDouble();
    return MerchantLayoutSpec._(
      viewportWidth: size.width,
      viewportHeight: size.height,
      textScale: resolvedTextScale,
      textScaler: TextScaler.linear(resolvedTextScale),
    );
  }

  final double viewportWidth;
  final double viewportHeight;
  final double textScale;
  final TextScaler textScaler;

  bool get isCompact => viewportWidth < 360;
  bool get isWide => viewportWidth >= 600;
  bool get isShort => viewportHeight < 560;

  /// The website switches its seller profile to a vertical flow on narrow
  /// screens. The App keeps the requested two-column summary from 360dp up,
  /// but stacks it when accessibility text would make either pane too narrow.
  bool get stacksProfileDetails => isCompact || textScale >= 1.15;

  static const double topBarHeight = 64;
  static const double tabBarHeight = 52;
  static const double storefrontContentMaxWidth = 960;
  static const double profileContentMaxWidth = 720;

  double get pageHorizontalPadding => isCompact ? 12 : (isWide ? 24 : 16);
  double get profileOverlap => isShort ? 20 : (isWide ? 28 : 24);
  double get avatarSize => isCompact ? 80 : (isWide ? 96 : 88);

  double get expandedHeroHeight {
    if (isShort) return 104;
    final maxHeight = isWide ? 148.0 : 124.0;
    return (viewportWidth * 0.28).clamp(104, maxHeight);
  }

  /// Minimum height used by the two seller-summary panes. Phone panes need a
  /// little more room because each metadata item wraps onto its own row. The
  /// rendered row may grow beyond this when text metrics require more space.
  double get profileDetailsPaneHeight => isWide ? 144 : 169;

  /// Minimum height when the about pane shows its accessible read-more target.
  ///
  /// At 1.0x text the content itself is 130dp tall and already includes 8dp
  /// bottom padding, so reserving the general panel's 169dp worst-case height
  /// here would create an oversized gap above the tabs.
  static const double profileDetailsReadMorePaneHeight = 138;

  double profileDetailsPaneHeightFor({
    required bool hasAward,
    required String displayedLocation,
    required String description,
  }) {
    if (profileSummaryNeedsFullHeight(
      hasAward: hasAward,
      displayedLocation: displayedLocation,
    )) {
      return profileDetailsPaneHeight;
    }
    if (aboutSummaryNeedsFullHeight(description: description)) {
      return profileDetailsReadMorePaneHeight;
    }
    // Without the optional award, the location / rating / subscriber summary
    // only occupies three rows. Restore the full pane height only when the
    // general panel really needs a fourth visual row.
    return isWide ? 104 : 120;
  }

  /// Whether the summary needs the original four-row height. Besides the
  /// optional award, a location that wraps at the current pane width also
  /// becomes a fourth visual row and must keep the full height.
  bool profileSummaryNeedsFullHeight({
    required bool hasAward,
    required String displayedLocation,
  }) {
    if (hasAward) return true;

    final contentWidth = math.min(
      profileContentMaxWidth,
      math.max(0.0, viewportWidth - pageHorizontalPadding * 2),
    );
    final paneWidth = math.max(0.0, (contentWidth - HkSpace.sm) / 2);
    final locationTextWidth = math.max(
      0.0,
      paneWidth - HkSpace.md * 2 - 16 - HkSpace.xs,
    );
    final painter = TextPainter(
      text: TextSpan(text: displayedLocation, style: AppTypography.meta),
      maxLines: 1,
      textDirection: TextDirection.ltr,
      textScaler: textScaler,
    )..layout(maxWidth: locationTextWidth);
    final wraps = painter.didExceedMaxLines;
    painter.dispose();
    return wraps;
  }

  /// The about pane adds a 48px accessible "read more" target when its copy
  /// exceeds two lines. The compact three-row profile height cannot contain
  /// that extra row, so the pane switches to its dedicated read-more height.
  bool aboutSummaryNeedsFullHeight({required String description}) {
    final resolvedDescription = description.trim();
    if (resolvedDescription.isEmpty) return false;

    final contentWidth = math.min(
      profileContentMaxWidth,
      math.max(0.0, viewportWidth - pageHorizontalPadding * 2),
    );
    final paneWidth = math.max(0.0, (contentWidth - HkSpace.sm) / 2);
    final textWidth = math.max(1.0, paneWidth - HkSpace.md * 2);
    final painter = TextPainter(
      text: TextSpan(
        text: resolvedDescription,
        style: AppTypography.metaMuted.copyWith(height: 1.5),
      ),
      maxLines: 2,
      textDirection: TextDirection.ltr,
      textScaler: textScaler,
    )..layout(maxWidth: textWidth);
    final needsReadMore = painter.didExceedMaxLines;
    painter.dispose();
    return needsReadMore;
  }

  double get estimatedProfileHeight {
    final scaleDelta = math.max(0, textScale - 1);
    // Avatar overlap followed by the 2dp gap before profile details.
    // The store name now lives inside the general-information pane.
    const identity = 28.0;
    if (!stacksProfileDetails) {
      return identity + profileDetailsPaneHeight;
    }

    // Stacked cards must reserve their natural text height because the
    // FlexibleSpaceBar clips anything outside expandedHeight.
    final generalPanel = 124.0 + scaleDelta * 124;
    final aboutPanel = 148.0 + scaleDelta * 64;
    return identity + generalPanel + 4 + aboutPanel;
  }

  /// SliverAppBar's [expandedHeight] includes the pinned tab bar but excludes
  /// the system status-bar inset.
  double get expandedAppBarHeight =>
      expandedHeroHeight + estimatedProfileHeight + tabBarHeight;

  double expandedAppBarHeightFor({
    required bool hasAward,
    required String displayedLocation,
    required String description,
  }) {
    if (stacksProfileDetails) return expandedAppBarHeight;
    final compactProfileHeight =
        28.0 +
        profileDetailsPaneHeightFor(
          hasAward: hasAward,
          displayedLocation: displayedLocation,
          description: description,
        );
    return expandedHeroHeight + compactProfileHeight + tabBarHeight;
  }

  int postColumnCount(double availableWidth) {
    // [availableWidth] excludes the page gutters: a 360dp viewport yields
    // roughly 328dp here, while a 320dp viewport yields roughly 296dp.
    if (availableWidth < 320) return 2;
    if (availableWidth < 600) return 3;
    if (availableWidth < 840) return 4;
    return 5;
  }

  static double centeredSideInset(
    double viewportWidth, {
    required double baseInset,
    double maxContentWidth = storefrontContentMaxWidth,
  }) {
    if (viewportWidth <= maxContentWidth + baseInset * 2) return baseInset;
    return (viewportWidth - maxContentWidth) / 2;
  }
}
