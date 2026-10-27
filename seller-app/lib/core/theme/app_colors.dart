import 'package:flutter/material.dart';

/// Hawkr 品牌色彩系統。
///
/// 單一來源：官網 `styles/foundation/00-color-config.css`，品牌色票見 `brand/hawkr/README.md`。
class AppColors {
  AppColors._();

  // ── Hawkr 品牌色票（brand/hawkr/README.md） ────────────────
  static const Color brandPrimary = Color(0xFFE4572E);
  static const Color brandPrimaryStrong = Color(0xFFB8441F);
  static const Color brandPrimaryDeep = Color(0xFF8F3315);
  static const Color brandSecondary = Color(0xFFF2A541);
  static const Color brandInk = Color(0xFF1F2933);
  static const Color brandInkOnDark = Color(0xFFF5F7FA);
  static const Color brandPrimarySoft = Color(0xFFFCEBE6);
  static const Color brandSecondarySoft = Color(0xFFFDF1E1);

  // ── Ink ramp：以 brandInk（900）與 brandInkOnDark（50）為兩端的中性藍灰階 ──
  static const Color ink50 = brandInkOnDark;
  static const Color ink100 = Color(0xFFE4E7EB);
  static const Color ink200 = Color(0xFFCBD2D9);
  static const Color ink300 = Color(0xFF9AA5B1);
  static const Color ink400 = Color(0xFF7B8794);
  static const Color ink500 = Color(0xFF616E7C);
  static const Color ink600 = Color(0xFF52606D);
  static const Color ink700 = Color(0xFF3E4C59);
  static const Color ink800 = Color(0xFF323F4B);
  static const Color ink850 = Color(0xFF28333F);
  static const Color ink900 = brandInk;
  static const Color ink950 = Color(0xFF161C23);

  // ── 官網 neutral palette ──────────────────────────────────
  static const Color neutralWhite = Color(0xFFFFFFFF);
  static const Color neutralCream = Color(0xFFFFFDF7);
  static const Color neutralPage = Color(0xFFF8F6EF);
  static const Color neutralMuted = Color(0xFFF5F1E8);
  static const Color neutralPanel = Color(0xFFF2E9D6);
  static const Color neutralWhisper = Color(0xFFF0F1EA);
  static const Color neutralInk = Color(0xFF191C18);
  static const Color neutralBody = Color(0xFF42493F);
  static const Color neutralMutedText = Color(0xFF5A6154);
  static const Color neutral500 = Color(0xFF888888);
  static const Color neutral300 = Color(0xFFC8C8C8);
  static const Color neutralBlack = Color(0xFF000000);

  // ── 品牌／互動語意 ───────────────────────────────────────
  static const Color primary = brandPrimaryStrong;
  static const Color actionPrimary = brandPrimaryStrong;
  static const Color primaryDark = brandPrimaryDeep;
  static const Color primaryLight = brandSecondarySoft;
  static const Color secondary = brandPrimaryStrong;
  static const Color primarySoft = Color(0x1AB8441F);
  static const Color primarySubtle = Color(0x08B8441F);
  static const Color primaryHover = Color(0x12B8441F);
  static const Color primaryActive = Color(0x17B8441F);

  /// 外部支付品牌色；不參與一般 UI 配色。
  static const Color externalVisa = Color(0xFF1A1F71);

  // ── 表面色 Surface ────────────────────────────────────────
  static const Color surfaceCream = neutralCream;
  static const Color surfacePage = neutralPage;
  static const Color surfaceMuted = neutralMuted;
  static const Color surfacePanel = neutralPanel;
  static const Color surfaceGlass = Color(0xF5FFFDF7);

  /// 官網 `<body>` 的單一連續頁面底色。
  static const Color bgApp = surfacePage;
  static const Color bgPrimary = bgApp;

  /// 官網 card surface。
  static const Color surfaceCard = surfaceCream;
  static const Color bgCard = surfaceCard;
  static const Color bgElevated = neutralWhite;

  /// 頂部 chrome 與畫布連續，玻璃層再提供一階明度。
  static const Color topBarBackground = surfacePage;
  static const Color topBarGlass = Color(0xF5F8F6EF);

  /// 浮動 navigation 的不透明 fallback 與 cream glass。
  static const Color navBarBackground = surfaceCream;
  static const Color navBarGlass = Color(0xF0FFFDF7);

  // ── Storefront semantic palette ────────────────────────────
  // 與買家店舖頁共用相同角色，讓賣家自有店舖與公開頁保持一致。
  static const Color storefrontProfileSurface = neutralCream;
  static const Color storefrontControlSurface = Color(0xEBFFFDF7);
  static const Color storefrontPageSurface = neutralPage;
  static const Color storefrontMutedSurface = neutralMuted;
  static const Color storefrontBorder = neutralPanel;

  /// 評分星星統一使用 Logo 的品牌金色，避免落成偏棕的警示色。
  static const Color storefrontRating = brandSecondary;

  /// @deprecated 請依元件角色改用 [topBarBackground] 或 [navBarBackground]。
  static const Color chromeBackground = topBarBackground;
  static const Color chromeForeground = neutralInk;
  static const Color chromeMuted = neutralMutedText;
  static const Color bgOverlay = Color(0x6B1F2933);
  static const Color bgOverlaySoft = Color(0x2E191C18);

  // ── 舊名稱相容 aliases ────────────────────────────────────
  static const Color cream = neutralCream;
  static const Color red = danger;
  static const Color textDark = neutralInk;

  static const Color white100 = neutralWhite;
  static Color get white70 => white100.withValues(alpha: 0.7);
  static Color get white40 => white100.withValues(alpha: 0.4);

  static const Color gray1 = neutralBody;
  static Color get gray1_80 => gray1.withValues(alpha: 0.8);
  static Color get gray1_50 => gray1.withValues(alpha: 0.5);
  static Color get gray1_30 => gray1.withValues(alpha: 0.3);
  static Color get gray1_20 => gray1.withValues(alpha: 0.2);
  static const Color gray50 = neutralPage;
  static const Color gray100 = neutralWhisper;
  static const Color gray200 = neutralPanel;
  static const Color gray300 = neutral300;
  static const Color gray400 = neutralMutedText;
  static const Color gray500 = neutral500;
  static const Color gray600 = neutralMutedText;
  static const Color gray700 = neutralBody;
  static const Color gray800 = neutralInk;
  static const Color gray900 = neutralInk;

  static const Color black = neutralBlack;
  static Color get black80 => black.withValues(alpha: 0.8);

  // ── 語意色 Semantic ───────────────────────────────────────
  static const Color success = Color(0xFF2D7F50);
  static const Color successSoft = Color(0x1A2D7F50);
  static const Color danger = Color(0xFFEA6021);
  static const Color error = danger;
  static const Color dangerSoft = Color(0x1AEA6021);

  /// 收藏與按讚的唯一啟用色；與 Buyer 共用同一明亮紅色。
  static const Color favoriteActive = Color(0xFFFF3B4F);

  /// danger 的文字與必填標記用深階：danger 本身在淺底上只有 3.1–3.4:1，這一階在 cream 上 5.85:1。
  static const Color dangerStrong = Color(0xFFB33A0B);
  static const Color warning = Color(0xFFE0C44C);
  static const Color warningSoft = Color(0x29E0C44C);
  static const Color onWarning = brandInk;
  static const Color warningText = brandInk;
  static const Color info = Color(0xFF1E5FAB);
  static const Color infoMid = Color(0xFF6194D0);
  static const Color infoLight = Color(0xFF9FCCDD);
  static const Color infoSoft = Color(0x1A6194D0);

  /// 輔色只當底色或裝飾；上面的字用 brandInk，不得當文字色（白底對比約 2.0:1）。
  static const Color accentFill = brandSecondary;
  static Color get disabled => gray1_30;

  // 配送溫層標籤：與 Web TemperatureTag 使用相同的語意色。
  static const Color temperatureAmbient = Color(0xFF6B6B6B);
  static const Color temperatureChilled = Color(0xFFEE964B);
  static const Color temperatureFrozen = Color(0xFF51A2FF);

  // ── 邊框與 elevation ──────────────────────────────────────
  static Color get border => gray1_20;
  static Color get divider => gray1_20;
  static const Color dividerLight = neutralPanel;
  static const Color cardBorder = Color(0x141F2933);
  static const Color cardShadow = Color(0x121F2933);
  static const Color borderSubtle = Color(0x141F2933);
  static const Color borderSoft = Color(0x241F2933);
  static const Color borderStrong = Color(0x3D1F2933);
  static const Color borderPrimary = Color(0x66B8441F);
  static const Color borderInput = Color(0xFFDED8CB);
  static const Color dragHandle = neutralPanel;

  // ── 文字色 Text ───────────────────────────────────────────
  static const Color textPrimary = neutralBody;
  static const Color textEmphasis = neutralInk;

  /// 一般次要文字仍需通過 App 的 WCAG AA；使用官網 body 色。
  static const Color textSecondary = neutralBody;
  static const Color textMuted = neutralMutedText;
  static const Color textHint = neutralMutedText;
  static const Color textDisabled = neutral300;
  static const Color textInverse = neutralCream;

  /// primary 的淡底（primarySoft／primaryActive／primaryHover）與 surfacePanel 上的強調文字：
  /// primary 在這些底上低於 4.5:1，改用加深一階的 brandPrimaryDeep。
  static const Color textAccentOnTint = brandPrimaryDeep;

  // ── 按鈕色 ────────────────────────────────────────────────
  static const Color buttonPrimary = actionPrimary;
  static const Color buttonSecondary = secondary;
  static const Color buttonDanger = error;

  // ── 第三方登入 ─────────────────────────────────────────────
  /// LINE 登入按鈕的底色：LINE 品牌規範的 LINE Green，不屬於官網 palette。
  static const Color lineBrand = Color(0xFF06C755);
}
