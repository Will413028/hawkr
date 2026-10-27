import 'package:flutter/widgets.dart';

import 'app_text_size.dart';

/// Global device adaptation policy.
///
/// Layout dimensions stay in logical pixels while the OS text preference is
/// preserved inside a tested range. The in-app Small / Standard / Large choice
/// adds a modest adjustment without flattening nonlinear Dynamic Type behavior.
abstract final class AppMediaQueryPolicy {
  static const double minTextScale = 0.8;
  static const double maxTextScale = 1.4;

  static MediaQueryData apply(
    MediaQueryData data, {
    AppTextSize textSize = AppTextSize.standard,
  }) {
    return data.copyWith(
      textScaler: _AppTextScaler(
        platformScaler: data.textScaler,
        adjustment: textSize.scaleAdjustment,
      ),
    );
  }
}

final class _AppTextScaler extends TextScaler {
  const _AppTextScaler({
    required this.platformScaler,
    required this.adjustment,
  });

  final TextScaler platformScaler;
  final double adjustment;

  @override
  double scale(double fontSize) {
    if (fontSize <= 0) return platformScaler.scale(fontSize);
    final adjusted = platformScaler.scale(fontSize) + fontSize * adjustment;
    return adjusted.clamp(
      fontSize * AppMediaQueryPolicy.minTextScale,
      fontSize * AppMediaQueryPolicy.maxTextScale,
    );
  }

  @override
  double get textScaleFactor => scale(14) / 14;

  @override
  bool operator ==(Object other) =>
      other is _AppTextScaler &&
      other.platformScaler == platformScaler &&
      other.adjustment == adjustment;

  @override
  int get hashCode => Object.hash(platformScaler, adjustment);
}
