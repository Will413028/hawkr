import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// 訂單模組專用色彩角色。
///
/// 僅由 Hawkr 品牌色票（主色、輔色、墨色）與其既有 surface token 組成，
/// 避免訂單列表、詳情、物流各自引入藍色或其他色票。輔色底上的字與圖示用墨色。
abstract final class OrderColorTokens {
  static const primary = AppColors.brandPrimaryStrong;
  static const progress = AppColors.brandPrimaryStrong;
  static const neutral = AppColors.brandPrimaryStrong;
  static const attention = AppColors.brandPrimaryStrong;
  static const attentionFill = AppColors.brandSecondary;

  static const primarySurface = AppColors.primarySoft;
  static const progressSurface = AppColors.memberCardSurface;
  static const attentionSurface = AppColors.warningSoft;
  static const neutralSurface = AppColors.brandSecondarySoft;

  static const onPrimary = AppColors.bgCard;
  static const onAttention = AppColors.brandInk;

  static Color foregroundFor(Color background) {
    if (background == attentionFill || background == neutralSurface) {
      return onAttention;
    }
    return onPrimary;
  }
}
