import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:flutter/material.dart';

/// Cart-only semantic roles. 22/28 follows Material 3 titleLarge and 24/32
/// follows headlineSmall, adapted to Hawkr's platform font and colors.
/// TextScaler remains owned by the existing AppMediaQueryPolicy / AppTextSize.
/// This does not change the global type ramp or temperature badge styling.
abstract final class CartTypography {
  static TextStyle get section => AppTypography.sectionTitle.copyWith(
    fontSize: 22,
    height: 28 / 22,
    fontWeight: FontWeight.w600,
  );
  static TextStyle get navigationTitle => section;
  static TextStyle get subtitle => AppTypography.productName.copyWith(
    fontSize: 16,
    height: 24 / 16,
    fontWeight: FontWeight.w600,
  );
  static TextStyle get body => AppTypography.body.copyWith(
    fontSize: 16,
    height: 24 / 16,
    fontWeight: FontWeight.w400,
  );
  static TextStyle get support => AppTypography.meta.copyWith(
    fontSize: 14,
    height: 20 / 14,
    fontWeight: FontWeight.w400,
  );
  static TextStyle get supportMuted =>
      support.copyWith(color: AppTypography.colorMuted);
  static TextStyle get amount => subtitle;
  static TextStyle get total => AppTypography.display.copyWith(
    fontSize: 24,
    height: 32 / 24,
    fontWeight: FontWeight.w600,
  );
  static TextStyle get button => AppTypography.buttonMedium.copyWith(
    fontSize: 16,
    height: 24 / 16,
    fontWeight: FontWeight.w600,
  );
}
