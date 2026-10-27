import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// 網頁同源的語意顏色，供所有 in-app 圖示使用。
///
/// Feature code must select a role from this class instead of choosing a raw
/// color.
abstract final class AppIconColors {
  /// Default icon on page/card surfaces (`--theme-text-body`).
  static const defaultColor = AppColors.textPrimary;

  /// Strong icon / heading companion (`--theme-text-heading`).
  static const strong = AppColors.textDark;

  /// Supporting, inactive navigation and tertiary actions.
  static const muted = AppColors.chromeMuted;

  /// Active navigation, selected controls and primary action affordances.
  static const active = AppColors.brandPrimaryStrong;

  /// Secondary brand icon (`--color-brand-primary-strong`).
  static const secondary = AppColors.brandPrimaryStrong;

  /// 網頁 `--icon-secondary`: primary 55%。
  static const secondarySoft = Color(0x8CE4572E);

  /// 網頁 `--icon-secondary-active`: Gold 70%。
  static const secondaryActive = Color(0xB3F2A541);

  /// Icon placed on primary-strong or dark media.
  static const onBrand = AppColors.surfaceCard;

  /// Icon placed on the secondary (accent) fill.
  static const onAccent = AppColors.brandInk;

  /// Disabled/non-actionable icon.
  static const disabled = AppColors.textDisabled;

  static const success = AppColors.success;

  /// Warning glyph on a light surface; the yellow fill itself is a background.
  static const warning = AppColors.warningText;
  static const warningText = AppColors.warningText;
  static const danger = AppColors.error;
  static const favorite = AppColors.favoriteActive;
  static const info = AppColors.info;

  /// Logo gold used by branded highlights.
  static const gold = AppColors.brandSecondary;

  /// Rating stars use the exact Logo gold rather than warning/ink tones.
  static const rating = AppColors.storefrontRating;
}
