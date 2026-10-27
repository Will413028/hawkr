import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_temperature_tag.dart';
import 'package:flutter/material.dart';

import 'package:hawkr_buyer/core/codemeta/code_meta.dart';

// ---------------------------------------------------------------------------
// 溫層標籤 — 冷藏 / 冷凍 / 常溫。
//
// 以彩色外框圓角標籤顯示商品的配送溫度，
// 用於購物車商品行及配送選項中。
// ---------------------------------------------------------------------------

/// 溫層 key → 顯示顏色。
/// key 使用 ShippingTemperature.value (如 '3202') 及中文名稱雙重對應。
/// Web TemperatureTag: 冷藏=#EE964B, 冷凍=#51A2FF, 常溫=#6B6B6B。
final tempColorMap = <String, Color>{
  ShippingTemperature.chilled.value: AppColors.temperatureChilled,
  '冷藏': AppColors.temperatureChilled,
  ShippingTemperature.frozen.value: AppColors.temperatureFrozen,
  '冷凍': AppColors.temperatureFrozen,
  ShippingTemperature.ambient.value: AppColors.temperatureAmbient,
  '常溫': AppColors.temperatureAmbient,
};

/// 溫層 key → 中文標籤。
final tempLabelMap = <String, String>{
  ShippingTemperature.chilled.value: '冷藏',
  ShippingTemperature.frozen.value: '冷凍',
  ShippingTemperature.ambient.value: '常溫',
};

/// 可互動的配送溫層 chip。
///
/// 色票與購物車、商品詳情的溫層標籤共用，但**視覺不共用**：[HkTemperatureTag]
/// 是 18dp 的資訊性標籤，它自己的文件也寫明「選擇溫層請另外做 48dp 控制項」。
/// 先前這裡直接把那顆 18dp 標籤當成控制項的外觀，觸控範圍雖然有 48dp，看起來
/// 卻只有一顆小標籤，使用者會瞄著那顆小的按而覺得難按。
///
/// 因此這裡畫自己的 32dp 膠囊（與 [HkChoiceChip] 同一組 sm 控制項規格），
/// 外層維持 48dp 觸控範圍。
class TemperatureChoiceChip extends StatelessWidget {
  const TemperatureChoiceChip({
    super.key,
    required this.code,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String code;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  Color get temperatureColor =>
      tempColorMap[code] ?? tempColorMap[label] ?? AppColors.temperatureAmbient;

  @override
  Widget build(BuildContext context) {
    final color = temperatureColor;
    return Semantics(
      button: true,
      selected: selected,
      label: '$label配送溫層',
      onTap: onTap,
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(HkRadius.pill),
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                minWidth: HkControl.minTap,
                minHeight: HkControl.minTap,
              ),
              child: Center(
                widthFactor: 1,
                child: AnimatedContainer(
                  duration: HkMotion.duration,
                  curve: HkMotion.emphasized,
                  height: HkControl.compactChoiceHeight,
                  padding: const EdgeInsets.symmetric(
                    horizontal: HkControl.compactChoiceHorizontalPadding,
                  ),
                  decoration: ShapeDecoration(
                    color: selected
                        ? color.withValues(alpha: 0.12)
                        : AppColors.gray100,
                    shape: StadiumBorder(
                      side: BorderSide(
                        color: selected ? color : Colors.transparent,
                      ),
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        label,
                        softWrap: false,
                        style: TextStyle(
                          fontFamily: AppTypography.family,
                          fontSize: HkControl.compactChoiceFontSize,
                          fontWeight: FontWeight.w700,
                          height: 1,
                          color: color,
                        ),
                      ),
                      // 勾選圖示的位置一律保留。先前是選取時才插入，導致每按一次
                      // 膠囊就變寬、整排跟著位移。
                      const SizedBox(width: HkSpace.xs),
                      AnimatedOpacity(
                        duration: HkMotion.duration,
                        curve: HkMotion.emphasized,
                        opacity: selected ? 1 : 0,
                        child: Icon(AppIcons.check, size: 16, color: color),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 根據 SKU 的配送溫層列表，產生一組溫層標籤 widget。
///
/// [skuId] 用於從 [itemDeliveryTemps] 查找對應的溫層列表，
/// 若無資料則回傳空 widget。
Widget buildTemperatureTags(
  int skuId,
  Map<int, List<String>> itemDeliveryTemps,
) {
  final temps = itemDeliveryTemps[skuId];
  if (temps == null || temps.isEmpty) return const SizedBox.shrink();
  return Wrap(
    spacing: HkSpace.xs,
    runSpacing: HkSpace.xs,
    children: [for (final temp in temps) HkTemperatureTag(value: temp)],
  );
}
