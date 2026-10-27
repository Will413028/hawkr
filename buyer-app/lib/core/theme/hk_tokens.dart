import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';

/// Hawkr 設計 token — 間距 / 圓角 / 控制項 / 陰影 / 動效。
///
/// 對齊網站 design system：
/// - 間距、圓角：styles/foundation/01-tokens-theme.css
/// - 陰影：styles/foundation/01-tokens-theme.css（墨色染色 elevation）
/// - 動效：標準 easing cubic-bezier(0.22, 1, 0.36, 1) @ 200ms
///
/// Flutter 畫面只使用這組離散值，避免同一層級出現 10 / 14 / 18 / 20 等
/// 看似接近、實際不一致的硬編碼數值。
class HkSpace {
  HkSpace._();

  static const double xs = 4;
  static const double sm = 8;
  static const double md = 12;
  static const double lg = 16;
  static const double xl = 20;
  static const double xxl = 24;
  static const double section = 32;
  static const double large = 40;
  static const double control = 48;
  static const double sectionLarge = 64;
}

class HkRadius {
  HkRadius._();

  /// tiny tags、inline labels
  static const double xs = 6;

  /// chips、inputs
  static const double sm = 8;

  /// 卡片、按鈕
  static const double md = 12;

  /// feature panels
  static const double lg = 16;

  /// hero blocks、dialog、bottom sheet
  static const double xl = 24;

  /// full-bleed surfaces
  static const double xxl = 32;

  static const double dialog = xl;
  static const double signature = lg;
  static const double hero = xxl;

  static const double pill = 999;
}

class HkControl {
  HkControl._();

  /// WCAG / platform accessible tap target.
  static const double minTap = 48;
  static const double icon = 24;

  /// 排序／篩選等次要選擇控制的視覺規格。
  ///
  /// 對齊網頁設計系統的 sm 控制項階梯（`styles/components/ui-kit.css` 的
  /// `--hkui-control-h-sm` / `--hkui-control-px-sm` / `--hkui-control-font-sm`，
  /// 註解標明供 chips、表格篩選與緊湊操作共用）。原本的 28/8 比設計系統窄
  /// 三分之一，文字會貼著膠囊邊緣。
  ///
  /// 外層仍需保留 [minTap] 的可點擊範圍，避免為了緊湊視覺犧牲觸控性。
  static const double compactChoiceHeight = 32;
  static const double compactChoiceFontSize = 12;
  static const double compactChoiceHorizontalPadding = 12;

  /// 網頁 mobile header 的固定內容高度（不包含 system status inset）。
  static const double topBarHeight = 56;

  /// 一般 TopBar／篩選區的搜尋與輸入控制，符合最小觸控高度。
  static const double searchFieldHeight = minTap;

  /// 首頁獨立的全寬搜尋欄。
  static const double prominentSearchHeight = 56;

  /// 網頁 mobile floating navigation 的實測高度約 67.5px。
  static const double navHeight = 68;
  static const double navItemHeight = 50;
  static const double navIcon = 30;

  /// 手機以上避免 chrome 內容無限制拉寬。
  static const double chromeMaxWidth = 720;
  static const double bottomNavMaxWidth = 560;
}

/// 只有版面會依寬度切換；字級、圖示與可點擊尺寸維持 logical px。
class HkBreakpoints {
  HkBreakpoints._();

  static const double compact = 360;
  static const double tablet = 600;
  static const double wide = 1024;

  static double pageGutter(double width) {
    if (width >= wide) return HkSpace.section;
    if (width >= tablet) return HkSpace.xxl;
    return HkSpace.lg;
  }

  static double contentMaxWidth(double width) {
    if (width >= wide) return 1120;
    if (width >= tablet) return 720;
    return width;
  }
}

/// 網頁式霧面玻璃效果。
///
/// 數值刻意保持克制：只模糊小面積 chrome，不套在長列表或商品圖片上。
class HkGlass {
  HkGlass._();

  /// Regular Liquid Glass for navigation bars, toolbars and text-heavy
  /// transient surfaces.
  static const double regularBlurSigma = 20;
  static const double regularSurfaceOpacity = 0.74;

  /// Clear Liquid Glass for compact controls floating above photos or video.
  static const double clearBlurSigma = 16;
  static const double clearSurfaceOpacity = 0.56;

  /// Flutter does not currently expose iOS Reduce Transparency. High Contrast
  /// therefore receives a near-opaque, blur-free material as the safe fallback.
  static const double accessibleSurfaceOpacity = 0.96;
  static const double edgeHighlightOpacity = 0.86;
  static const double clearSpecularOpacity = 0.42;

  static const double topBarBlurSigma = 14;
  static const double topBarSurfaceOpacity = 0.74;
  static const double topBarHighlightStartOpacity = 0.20;
  static const double topBarHighlightEndOpacity = 0.04;
  static const double navBarBlurSigma = 18;
  static const double navBarHighlightOpacity = 0.72;
  static const double navBarSurfaceOpacity = 0.56;
  static const double navBarBorderOpacity = 0.88;

  /// Dialog route background blur; paired with a light scrim, never opaque.
  static const double dialogBlurSigma = 16;

  /// Tab 軌道與選取膠囊共用的 iOS 式 Liquid Glass 層。
  ///
  /// 選取狀態只提高白色表面的不透明度與陰影，不使用品牌色填滿，
  /// 避免看起來像 Flutter 預設 Material control。
  static const double tabBarBlurSigma = 20;
  static const double tabIndicatorBlurSigma = 14;
  static const double tabIndicatorSurfaceOpacity = 0.82;
  static const double tabHighlightOpacity = 0.88;
  static const double tabHorizontalInset = 8;
  static const double tabVerticalInset = 4;
  static const double tabSelectionInset = 3;
}

/// 墨色染色 elevation 階梯（非黑灰陰影）。
class HkShadows {
  HkShadows._();

  static const Color _tint = AppColors.brandInk;
  static const Color _ink = AppColors.textEmphasis;

  /// input / hover indicator（= shadow-xs）
  static List<BoxShadow> get xs => [
    BoxShadow(
      color: _ink.withValues(alpha: 0.05),
      offset: const Offset(0, 1),
      blurRadius: 2,
    ),
  ];

  /// chip / tooltip（= shadow-sm）
  static List<BoxShadow> get sm => [
    BoxShadow(
      color: _ink.withValues(alpha: 0.06),
      offset: const Offset(0, 2),
      blurRadius: 6,
    ),
  ];

  /// 卡片標準（= 網站 shadow-md）
  static List<BoxShadow> get card => [
    BoxShadow(
      color: _tint.withValues(alpha: 0.07),
      offset: const Offset(0, 12),
      blurRadius: 30,
    ),
    BoxShadow(
      color: _ink.withValues(alpha: 0.04),
      offset: const Offset(0, 2),
      blurRadius: 8,
    ),
  ];

  /// hover / 抬升強調（= 網站 shadow-lg）
  static List<BoxShadow> get lg => [
    BoxShadow(
      color: _tint.withValues(alpha: 0.10),
      offset: const Offset(0, 18),
      blurRadius: 42,
    ),
    BoxShadow(
      color: _ink.withValues(alpha: 0.06),
      offset: const Offset(0, 5),
      blurRadius: 14,
    ),
  ];

  /// 浮動底部導覽；比一般卡片多一階垂直分離感，但不使用厚重黑影。
  static List<BoxShadow> get floatingNav => [
    BoxShadow(
      color: _tint.withValues(alpha: 0.13),
      offset: const Offset(0, 20),
      blurRadius: 48,
    ),
    BoxShadow(
      color: _ink.withValues(alpha: 0.10),
      offset: const Offset(0, 7),
      blurRadius: 18,
    ),
  ];

  /// popover / large dropdown（= shadow-xl）
  static List<BoxShadow> get xl => [
    BoxShadow(
      color: _ink.withValues(alpha: 0.14),
      offset: const Offset(0, 12),
      blurRadius: 28,
    ),
    BoxShadow(
      color: _ink.withValues(alpha: 0.06),
      offset: const Offset(0, 4),
      blurRadius: 10,
    ),
  ];

  /// Modal / Dialog（= 網站 shadow-2xl）
  static List<BoxShadow> get modal => [
    BoxShadow(
      color: _ink.withValues(alpha: 0.20),
      offset: const Offset(0, 24),
      blurRadius: 60,
    ),
    BoxShadow(
      color: _ink.withValues(alpha: 0.08),
      offset: const Offset(0, 8),
      blurRadius: 18,
    ),
  ];
}

/// 動效常數 — bounce 只用於 delight，禁用於導航。
class HkMotion {
  HkMotion._();

  /// 標準 easing（= 網站 cubic-bezier(0.22, 1, 0.36, 1)）
  static const Curve ease = Cubic(0.22, 1, 0.36, 1);

  /// 頁面與浮層 easing（= 網站 --ease-emphasized）
  static const Curve emphasized = Cubic(0.16, 1, 0.3, 1);

  /// 標準時長
  static const Duration duration = Duration(milliseconds: 200);
}
