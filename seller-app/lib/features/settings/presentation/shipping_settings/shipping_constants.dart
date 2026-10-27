import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';

/// 物流設定頁面顏色常量
/// 定義頁面中使用的所有顏色值
class ShippingColors {
  // 主要顏色
  static const Color taqbinBg = AppColors.primarySoft; // 黑貓宅急便背景色
  static const Color cvsBg = AppColors.brandSecondarySoft; // 7-11背景色
  static const Color drawerBg =
      AppColors.cardBorder; // 抽屜背景色 rgba(86,86,86,0.2)
  static const Color checkGreen = AppColors.brandPrimaryStrong; // 勾選框綠色
  static const Color gray100 = AppColors.textPrimary; // 主要灰色
  static const Color borderColor = AppColors.cardBorder; // 邊框顏色
  static const Color green = AppColors.primary; // 主題綠色

  // 其他顏色
  static const Color white = AppColors.bgCard;
  static const Color black = AppColors.textEmphasis;
  static const Color transparent = Colors.transparent;
  static const Color scaffoldBg = AppColors.bgPrimary; // 頁面背景色

  // 陰影顏色
  static const Color shadowColor = AppColors.cardShadow;
  static const Color lightShadow = AppColors.cardShadow;

  // 下拉選單相關顏色
  static const Color dropdownHighlight = AppColors.warning; // 下拉選單高亮色
  static const Color dropdownSelectedBg = AppColors.primarySoft; // 下拉選單選中背景
}
