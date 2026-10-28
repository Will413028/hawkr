import 'package:flutter/services.dart';

/// Hawkr App 統一顏色 token。
///
/// 單一來源：網頁 `styles/foundation/00-color-config.css`，品牌色票見 `brand/hawkr/README.md`。
/// Widget 不得自行硬編碼品牌色，請依 Surface / Text / Semantic 角色取用。
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

  // ── 網頁 neutral palette ──────────────────────────────────
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
  static const Color primarySoft = Color(0x1AB8441F);
  static const Color primarySubtle = Color(0x08B8441F);
  static const Color primaryHover = Color(0x12B8441F);
  static const Color primaryActive = Color(0x17B8441F);

  // ── Web commerce CTA palette ───────────────────────────────
  // Exact light-mode values from Web ProductPurchaseBox.
  static const Color commerceCartAction = brandPrimaryStrong;
  static const Color commerceCartForeground = neutralCream;
  static const Color commerceCheckoutAction = brandSecondary;
  static const Color commerceCheckoutForeground = brandInk;

  /// 首頁會員卡底色：Ink 10% 疊在網站 page surface 的實際顯示色。
  /// 使用不透明 token，避免不同父層背景導致同一角色漂色。
  static const Color memberCardSurface = Color(0xFFE2E2DC);

  // ── Storefront semantic palette ────────────────────────────
  // These are the website UI-kit primary-300 / primary-200 rungs. They give
  // the seller hero a clear layer without introducing a second brand palette.
  static const Color storefrontHeroStart = Color(0xFFF0A38C);
  static const Color storefrontHeroEnd = Color(0xFFF6C9BC);
  static const Color storefrontHeroGlow = Color(0x1FF2A541);
  static const Color storefrontProfileSurface = neutralCream;

  /// 店舖頁浮動控制項共用表面：返回／購物車／聊天與 Tab 軌道。
  static const Color storefrontControlSurface = Color(0xEBFFFDF7);
  static const Color storefrontTabsSurface = neutralWhite;
  static const Color storefrontPageSurface = neutralPage;
  static const Color storefrontMutedSurface = neutralMuted;
  static const Color storefrontCardSurface = neutralWhite;
  static const Color storefrontBorder = neutralPanel;

  /// 評分星星統一使用 Logo 的品牌金色，避免落成偏棕的警示色。
  static const Color storefrontRating = brandSecondary;
  static const Color secondary = brandPrimaryStrong;

  /// 外部支付品牌色；不參與一般 UI 配色。
  static const Color externalVisa = Color(0xFF1A1F71);

  /// @deprecated 保留向下相容，新代碼請用 [primary]。

  /// @deprecated 保留向下相容，新代碼請用 [primaryDark]。

  // ── 中性色 Neutral aliases ────────────────────────────────
  static const Color gray900 = neutralInk;
  static const Color gray800 = neutralInk;
  static const Color gray700 = neutralBody;
  static const Color gray600 = neutralMutedText;
  static const Color gray500 = neutral500;
  static const Color gray400 = neutralMutedText;
  static const Color gray300 = neutral300;
  static const Color gray200 = neutralPanel;
  static const Color gray100 = neutralWhisper;
  static const Color gray50 = neutralPage;
  static const Color white = neutralWhite;

  // ── 表面色 Surface ────────────────────────────────────────
  static const Color surfaceCream = neutralCream;
  static const Color surfacePage = neutralPage;
  static const Color surfaceMuted = neutralMuted;
  static const Color surfacePanel = neutralPanel;
  static const Color surfaceGlass = Color(0xF5FFFDF7);

  /// 網頁 `<body>` 的單一連續頁面底色。
  static const Color bgPrimary = surfacePage;

  /// 網頁 card surface。
  static const Color bgCard = surfaceCream;
  static const Color bgElevated = neutralWhite;

  /// 網頁頂部 chrome 使用 page surface，而非會員卡灰或品牌深綠。
  static const Color topBarBackground = surfacePage;

  /// 網頁 header `rgba(248, 246, 239, .96)` 的不透明 fallback／玻璃底色。
  static const Color topBarGlass = Color(0xF5F8F6EF);

  /// 網頁浮動 NAV 的 cream glass 不透明 fallback。
  /// 與 page surface 保持一階明度差，避免底部導覽融入內容畫布。
  static const Color navBarBackground = surfaceCream;

  /// 網頁浮動 NAV 的 cream glass surface。
  static const Color navBarGlass = Color(0xF0FFFDF7);

  /// @deprecated 請依元件角色改用 [topBarBackground] 或 [navBarBackground]。
  static const Color chromeBackground = topBarBackground;
  static const Color chromeForeground = neutralInk;
  static const Color chromeMuted = neutralMutedText;

  static const Color bgOverlay = Color(0x6B1F2933);

  // ── 文字色 Text ───────────────────────────────────────────
  static const Color textPrimary = neutralBody;

  /// 網頁次要資訊色。只用於 14px 以上 metadata，不用於主要正文。
  static const Color textSecondary = neutralMutedText;
  static const Color textMuted = neutralMutedText;
  static const Color textHint = neutralMutedText;
  static const Color textDisabled = neutral300;
  static const Color textEmphasis = neutralInk;
  static const Color textInverse = neutralCream;

  /// primary 的淡底（primarySoft／primaryActive／primaryHover）與 surfacePanel 上的強調文字：
  /// primary 在這些底上低於 4.5:1，改用加深一階的 brandPrimaryDeep。
  static const Color textAccentOnTint = brandPrimaryDeep;

  // ── 語意色 Semantic ───────────────────────────────────────
  static const Color success = Color(0xFF2D7F50);
  static const Color successSoft = Color(0x1A2D7F50);
  static const Color warning = Color(0xFFE0C44C);
  static const Color warningSoft = Color(0x29E0C44C);
  static const Color onWarning = brandInk;
  static const Color warningText = brandInk;
  static const Color danger = Color(0xFFEA6021);
  static const Color dangerSoft = Color(0x1AEA6021);

  /// 收藏與按讚的唯一啟用色；使用明亮紅色，避免與警告橘混淆。
  static const Color favoriteActive = Color(0xFFFF3B4F);
  static const Color info = Color(0xFF1E5FAB);
  static const Color infoMid = Color(0xFF6194D0);
  static const Color infoLight = Color(0xFF9FCCDD);
  static const Color infoSoft = Color(0x1A6194D0);

  /// 輔色只當底色或裝飾；上面的字用 brandInk，不得當文字色（白底對比約 2.0:1）。
  static const Color accentFill = brandSecondary;

  // ── 卡片 / 元件 ───────────────────────────────────────────
  static const Color cardBorder = Color(0x141F2933);
  static const Color cardShadow = Color(0x121F2933);
  static const Color borderSubtle = Color(0x141F2933);
  static const Color borderSoft = Color(0x241F2933);
  static const Color borderStrong = Color(0x3D1F2933);
  static const Color borderPrimary = Color(0x66B8441F);

  /// 網頁 form input 的中性邊框，避免米色卡片與米色頁面黏在一起。
  static const Color borderInput = Color(0xFFDED8CB);
  static const Color dragHandle = neutralPanel;
  static const Color dividerLight = neutralPanel;

  // ── 第三方登入 ─────────────────────────────────────────────
  /// LINE 登入按鈕的底色：LINE 品牌規範的 LINE Green，不屬於官網 palette。
  static const Color lineBrand = Color(0xFF06C755);

  // ── System UI Overlay ─────────────────────────────────────
  static const SystemUiOverlayStyle topBarOverlay = SystemUiOverlayStyle(
    statusBarColor: topBarBackground,
    statusBarIconBrightness: Brightness.dark,
    statusBarBrightness: Brightness.light,
  );

  /// 深色品牌 TopBar 專用，確保 iOS 狀態列圖示保持可讀。
  static const SystemUiOverlayStyle brandTopOverlay = SystemUiOverlayStyle(
    statusBarColor: primary,
    statusBarIconBrightness: Brightness.light,
    statusBarBrightness: Brightness.dark,
  );

  /// 首頁會員卡灰 TopBar 專用，狀態列與內容區維持同一塊連續底色。
  static const SystemUiOverlayStyle memberTopOverlay = SystemUiOverlayStyle(
    statusBarColor: memberCardSurface,
    statusBarIconBrightness: Brightness.dark,
    statusBarBrightness: Brightness.light,
  );

  /// 霧面玻璃 TopBar 專用；狀態列保持透明，讓同一塊玻璃表面延伸到安全區。
  static const SystemUiOverlayStyle glassTopOverlay = SystemUiOverlayStyle(
    statusBarColor: Color(0x00000000),
    statusBarIconBrightness: Brightness.dark,
    statusBarBrightness: Brightness.light,
  );
}
