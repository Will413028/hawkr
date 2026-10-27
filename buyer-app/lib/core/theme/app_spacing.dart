import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_tokens.dart';

/// Hawkr App 統一間距 token。
///
/// 設計規範參考：openspec/specs/core-design-tokens/DESIGN_SYSTEM_v1.md
///
/// Figma 設計稿基準：iPhone 11/XR (414 x 896)
///
/// 4px 網格制：所有間距、padding、margin、gap **必須是 4 的倍數**，且只能來自下表 token。
/// 禁用值：6、10、14、18、20、28、36、40、44。
///
/// 使用方式：
/// ```dart
/// SizedBox(height: AppSpacing.md)        // 12.w
/// EdgeInsets.all(AppSpacing.lg)          // 16.w
/// AppSpacing.page                        // 預設頁面 padding
/// SizedBox(height: AppSpacing.buttonHeightLg) // 主 CTA 高度 48
/// ```
class AppSpacing {
  AppSpacing._();

  // ── 固定間距 (用於 SizedBox / Padding) ─────────────────────
  /// 同組元素間最小距（icon 與 text）
  static double get xs => 4.w;

  /// 同組元素內距
  static double get sm => 8.w;

  /// 卡片內小間距
  static double get md => 12.w;

  /// 預設頁面 padding、卡片 padding
  static double get lg => 16.w;

  /// 群組間距（區塊與區塊）
  static double get xl => 24.w;

  /// 頁面段落分隔
  static double get xxl => 32.w;

  /// 大區塊分隔、空狀態垂直間距
  static double get xxxl => 48.w;

  // ── 頁面標準水平 Padding ─────────────────────────────────
  static EdgeInsets get page => EdgeInsets.symmetric(horizontal: 16.w);
  static EdgeInsets get pageVertical => EdgeInsets.symmetric(vertical: 16.h);
  static EdgeInsets get pageBoth =>
      EdgeInsets.symmetric(horizontal: 16.w, vertical: 16.h);

  // ── 卡片內距 ───────────────────────────────────────────
  static EdgeInsets get card => EdgeInsets.all(16.w);
  static EdgeInsets get cardCompact =>
      EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h);

  // ── 觸控目標 Touch Target ──────────────────────────────
  /// Apple HIG 強制下限。所有可點擊元件實際尺寸必須 ≥ 此值。
  static const double minTouchTarget = HkControl.minTap;

  /// Material 標準建議
  static const double recommendedTouchTarget = 48;

  // ── 元件高度 Component Heights ─────────────────────────
  /// Pill / chip 按鈕高度
  static double get buttonHeightSm => 36.h;

  /// 一般按鈕最小高度（≥ minTouchTarget）
  static double get buttonHeightMd => 44.h;

  /// 主 CTA 按鈕高度
  static double get buttonHeightLg => 48.h;

  /// 底部固定 CTA bar
  static double get buttonHeightXl => 56.h;

  /// TextField 統一高度
  static double get inputHeight => HkControl.searchFieldHeight;

  /// Top bar 高度
  static double get appBarHeight => HkControl.topBarHeight;

  /// Bottom navigation 高度
  static double get bottomTabHeight => HkControl.navHeight;
}
