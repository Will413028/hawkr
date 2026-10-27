import 'package:flutter/material.dart';

import 'app_colors.dart';

/// Hawkr App 統一字體階層。
///
/// 設計規範參考：openspec/specs/core-design-tokens/DESIGN_SYSTEM_v1.md
///
/// 設計原則：
/// - 網站字級階梯：12 / 14 / 16 / 18 / 20 / 24 / 30 / 36 / 48
/// - 對齊網站字重：w400 / w500 / w700 / w900；禁用非規格的 w600 / w800
/// - 所有 TextStyle 皆走 token，禁止在 widget 中硬編碼 `fontSize:`
///
/// ```dart
/// Text('我的訂單',  style: AppTypography.h1);
/// Text('NT$ 9,999',style: AppTypography.display);
/// Text('選擇規格',  style: AppTypography.sectionTitle);
/// Text('小蕃茄',    style: AppTypography.productName);
/// Text('NT$ 999',   style: AppTypography.priceTotal);
/// Text('立即下單',  style: AppTypography.buttonLarge);
/// Text('加入購物車',style: AppTypography.buttonMedium);
/// Text('數量',      style: AppTypography.body);
/// Text('配送溫層',  style: AppTypography.meta);
/// Text('暫無評價',  style: AppTypography.metaMuted);
/// ```
class AppTypography {
  AppTypography._();

  // ── Colors ────────────────────────────────────────────────────────────────
  static const Color colorPrimary = AppColors.textPrimary;
  static const Color colorEmphasis = AppColors.textEmphasis;
  static const Color colorPrice = AppColors.primary;
  static const Color colorMuted = AppColors.textSecondary;
  static const Color colorDisabled = AppColors.textDisabled;
  static const Color colorBrand = AppColors.primary;
  static const Color colorDanger = AppColors.danger;
  static const Color colorInverse = AppColors.white;

  // ── Font family ───────────────────────────────────────────────────────────
  /// 不宣告未打包的 Roboto／Inter。iOS 會使用 San Francisco + PingFang TC，
  /// 取得穩定的繁中文字形與 fallback。
  static const String? family = null;
  static const String? emphasisFamily = null;

  /// App-wide Material text roles aligned with the website's mobile scale.
  /// Body copy uses the website 16px base; 12–14px is reserved for metadata
  /// and compact labels, never primary content.
  static TextTheme get textTheme => TextTheme(
    displaySmall: display,
    headlineSmall: h1,
    titleLarge: h1,
    titleMedium: sectionTitle,
    titleSmall: productName,
    bodyLarge: body,
    bodyMedium: body,
    bodySmall: meta,
    labelLarge: buttonMedium,
    labelMedium: meta.copyWith(fontWeight: FontWeight.w700),
    labelSmall: TextStyle(
      fontFamily: family,
      fontSize: 12,
      fontWeight: FontWeight.w700,
      color: colorPrimary,
      height: 16 / 12,
    ),
  );

  // ── XL: Display (24px Bold) ──────────────────────────────────────────────
  /// 大數字、空狀態主標 — 例：合計「NT$ 9,999」
  static TextStyle get display => TextStyle(
    fontFamily: family,
    fontSize: 24,
    fontWeight: FontWeight.w700,
    color: colorEmphasis,
    height: 32 / 24,
  );

  // ── H1 (20px Bold) ───────────────────────────────────────────────────────
  /// 頁面主標題 — 例：「我的訂單」、「商家詳情」
  static TextStyle get h1 => TextStyle(
    fontFamily: family,
    fontSize: 20,
    fontWeight: FontWeight.w700,
    color: colorEmphasis,
    height: 28 / 20,
  );

  // ── L1: Section Title (18px Bold, emphasis) ──────────────────────────────
  /// 區塊標題 — 例：「選擇規格」
  static TextStyle get sectionTitle => TextStyle(
    fontFamily: family,
    fontSize: 18,
    fontWeight: FontWeight.w700,
    color: colorEmphasis,
    height: 28 / 18,
  );

  // ── L2: Product Name / Bold Body (16px Bold) ─────────────────────────────
  /// 商品名稱 · 主要按鈕文字 — 例：「小蕃茄」、「繼續購買」
  static TextStyle get productName => TextStyle(
    fontFamily: family,
    fontSize: 16,
    fontWeight: FontWeight.w700,
    color: colorPrimary,
    height: 24 / 16,
  );

  /// L2 別名（語意同 productName）— 用於一般 Bold 正文
  static TextStyle get bodyBold => productName;

  // ── L3: Price Total Emphasis (18px Bold, accent) ─────────────────────────
  /// 合計金額強調 — 例：「NT$ 999」
  static TextStyle get priceTotal => TextStyle(
    fontFamily: family,
    fontSize: 18,
    fontWeight: FontWeight.w700,
    color: colorPrice,
  );

  /// 「NT$」前綴（搭配 priceTotal）
  static TextStyle get priceUnit => TextStyle(
    fontFamily: family,
    fontSize: 14,
    fontWeight: FontWeight.w700,
    color: colorPrice,
  );

  // ── M: Body (16px Regular, primary) ──────────────────────────────────────
  /// 一般正文 · 標籤 — 例：「數量」、「庫存：50 件」、規格選項
  static TextStyle get body => TextStyle(
    fontFamily: family,
    fontSize: 16,
    fontWeight: FontWeight.w400,
    color: colorPrimary,
    height: 24 / 16,
  );

  // ── S: Meta (14px Regular, primary) ──────────────────────────────────────
  /// 次要文字 — 例：「配送溫層」、「4.8 (326則評價)」
  static TextStyle get meta => TextStyle(
    fontFamily: family,
    fontSize: 14,
    fontWeight: FontWeight.w400,
    color: colorPrimary,
    height: 21 / 14,
  );

  /// 弱化次要文字 — 例：「單價 NT$ 999 × 6」、「暫無評價」
  static TextStyle get metaMuted => TextStyle(
    fontFamily: family,
    fontSize: 14,
    fontWeight: FontWeight.w400,
    color: colorMuted,
    height: 21 / 14,
  );

  // ── XS: Tag (12px Regular) ───────────────────────────────────────────────
  /// 徽章類標籤 — 例：冷藏 / 冷凍 / 常溫。需搭配對應顏色（由呼叫方傳入）。
  static TextStyle tag(Color color) => TextStyle(
    fontFamily: family,
    fontSize: 12,
    fontWeight: FontWeight.w400,
    color: color,
    height: 1.5,
    letterSpacing: 1.55,
  );

  // ── Caption (12 Regular) ──────────────────────────────────────────────────
  /// 極小說明文字 — 例：完整地址、7-11 門市名稱、付款期限提示
  /// 12px is the hard floor for compact supporting information.
  static TextStyle get caption => const TextStyle(
    fontFamily: family,
    fontSize: 12,
    fontWeight: FontWeight.w400,
    color: colorMuted,
  );

  // ── 按鈕文字 ─────────────────────────────────────────────────────────────
  /// 主 CTA 按鈕 — 例：「立即下單」、「結帳」
  static TextStyle get buttonLarge => TextStyle(
    fontFamily: family,
    fontSize: 16,
    fontWeight: FontWeight.w700,
    height: 24 / 16,
    color: colorInverse,
  );

  /// 次要按鈕 — 例：「加入購物車」、「再次購買」
  static TextStyle get buttonMedium => TextStyle(
    fontFamily: family,
    fontSize: 16,
    fontWeight: FontWeight.w700,
    height: 24 / 16,
    color: colorPrimary,
  );

  /// Pill 按鈕 — 例：「細節」「再次付款」
  static TextStyle get buttonSmall => TextStyle(
    fontFamily: emphasisFamily,
    fontSize: 14,
    fontWeight: FontWeight.w400,
    height: 1.33,
    color: AppColors.gray100,
  );
}
