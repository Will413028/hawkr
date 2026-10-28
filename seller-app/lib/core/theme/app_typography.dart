import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'app_colors.dart';

/// Hawkr App 統一字體階層（與 buyer app 同步）。
///
/// 設計原則：
/// - **L1 / L2 / L3** — Bold，用於重點資訊（區塊標題、商品名、按鈕、金額強調）
/// - **M**         — Regular 16sp，一般內容
/// - **S**         — Regular 14sp，輔助文字
/// - **XS**        — Regular 12sp，僅限 Tag 類徽章
/// - 字重只使用網站規格的 w400 / w500 / w700 / w900
///
/// 顏色：主要 `#42493F`（正文）／強調 `#191C18`（標題）／重點 `#B8441F`（價格）
///
/// ```dart
/// Text('選擇規格', style: AppTypography.sectionTitle);
/// Text('小蕃茄',   style: AppTypography.productName);
/// Text('NT$ 999',  style: AppTypography.priceTotal);
/// Text('數量',     style: AppTypography.body);
/// Text('配送方式', style: AppTypography.meta);
/// Text('暫無評價', style: AppTypography.metaMuted);
/// ```
class AppTypography {
  AppTypography._();

  // ── Colors ────────────────────────────────────────────────────────────────
  static const Color colorPrimary = AppColors.textPrimary;
  static const Color colorEmphasis = AppColors.textEmphasis;
  static const Color colorPrice = AppColors.primary;
  static const Color colorMuted = AppColors.textSecondary;
  static const Color colorDisabled = AppColors.textSecondary; // 50% 主色

  // ── Font family ───────────────────────────────────────────────────────────
  static const String family = 'Roboto';

  /// App-wide Material roles aligned with the website mobile typography.
  static TextTheme get textTheme => TextTheme(
    displaySmall: display,
    headlineSmall: h1,
    titleLarge: h1,
    titleMedium: sectionTitle,
    titleSmall: productName,
    bodyLarge: body.copyWith(fontSize: 16.sp, height: 24 / 16),
    bodyMedium: body,
    bodySmall: meta,
    labelLarge: buttonMedium,
    labelMedium: meta.copyWith(fontWeight: FontWeight.w700),
    labelSmall: TextStyle(
      fontFamily: family,
      fontSize: 12.sp,
      fontWeight: FontWeight.w700,
      color: colorPrimary,
      height: 16 / 12,
    ),
  );

  static TextStyle get display => TextStyle(
    fontFamily: family,
    fontSize: 24.sp,
    fontWeight: FontWeight.w700,
    color: colorEmphasis,
    height: 32 / 24,
  );

  static TextStyle get h1 => TextStyle(
    fontFamily: family,
    fontSize: 20.sp,
    fontWeight: FontWeight.w700,
    color: colorEmphasis,
    height: 28 / 20,
  );

  // ── L1: Section Title (16sp Bold, emphasis) ──────────────────────────────
  static TextStyle get sectionTitle => TextStyle(
    fontFamily: family,
    fontSize: 16.sp,
    fontWeight: FontWeight.w700,
    color: colorEmphasis,
    height: 24 / 16,
  );

  // ── L2: Product Name / Button (16sp Bold, primary) ───────────────────────
  static TextStyle get productName => TextStyle(
    fontFamily: family,
    fontSize: 16.sp,
    fontWeight: FontWeight.w700,
    color: colorPrimary,
    height: 24 / 16,
  );

  static TextStyle get bodyBold => productName;

  // ── L3: Price Total Emphasis (18sp Bold, accent) ─────────────────────────
  static TextStyle get priceTotal => TextStyle(
    fontFamily: family,
    fontSize: 18.sp,
    fontWeight: FontWeight.w700,
    color: colorPrice,
  );

  static TextStyle get priceUnit => TextStyle(
    fontFamily: family,
    fontSize: 14.sp,
    fontWeight: FontWeight.w700,
    color: colorPrice,
  );

  // ── M: Body (16sp Regular, primary) ──────────────────────────────────────
  static TextStyle get body => TextStyle(
    fontFamily: family,
    fontSize: 16.sp,
    fontWeight: FontWeight.w400,
    color: colorPrimary,
    height: 24 / 16,
  );

  // ── S: Meta (14sp Regular, primary) ──────────────────────────────────────
  static TextStyle get meta => TextStyle(
    fontFamily: family,
    fontSize: 14.sp,
    fontWeight: FontWeight.w400,
    color: colorPrimary,
    height: 21 / 14,
  );

  static TextStyle get metaMuted => TextStyle(
    fontFamily: family,
    fontSize: 14.sp,
    fontWeight: FontWeight.w400,
    color: colorMuted,
    height: 21 / 14,
  );

  // ── XS: Tag (12sp Regular) ───────────────────────────────────────────────
  static TextStyle tag(Color color) => TextStyle(
    fontFamily: family,
    fontSize: 12.sp,
    fontWeight: FontWeight.w400,
    color: color,
    height: 1.5,
    letterSpacing: 1.55,
  );

  static TextStyle get caption => const TextStyle(
    fontFamily: family,
    fontSize: 12,
    fontWeight: FontWeight.w400,
    color: colorMuted,
  );

  static TextStyle get buttonLarge => TextStyle(
    fontFamily: family,
    fontSize: 16.sp,
    fontWeight: FontWeight.w700,
    height: 24 / 16,
    color: AppColors.white100,
  );

  static TextStyle get buttonMedium => TextStyle(
    fontFamily: family,
    fontSize: 16.sp,
    fontWeight: FontWeight.w700,
    height: 24 / 16,
    color: colorPrimary,
  );
}
