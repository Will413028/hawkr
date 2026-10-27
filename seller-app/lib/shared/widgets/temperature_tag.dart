import 'package:flutter/material.dart';

import 'hk_icon.dart';
import 'hk_tap_target.dart';
import 'hk_temperature_tag.dart';

/// 相容既有 code／label API，所有顯示共用 Web 對齊的 [HkTemperatureTag]。
/// 選擇模式另外保留 48dp 點擊範圍與勾選狀態，不放大標籤本身。
class TemperatureTag extends StatelessWidget {
  const TemperatureTag({super.key, this.code, this.label})
    : isSelectable = false,
      isSelected = false,
      onTap = null;

  const TemperatureTag.selectable({
    super.key,
    this.code,
    this.label,
    required this.isSelected,
    this.onTap,
  }) : isSelectable = true;

  final String? code;
  final String? label;
  final bool isSelectable;
  final bool isSelected;
  final VoidCallback? onTap;

  String get _value => label?.trim().isNotEmpty == true ? label! : code ?? '';

  @override
  Widget build(BuildContext context) {
    final tag = HkTemperatureTag(value: _value);
    if (!isSelectable) return tag;
    return IntrinsicWidth(
      child: HkTapTarget(
        semanticLabel: '配送溫層 ${codeToLabel(_value)}',
        selected: isSelected,
        onTap: onTap,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            tag,
            if (isSelected) ...[
              const SizedBox(width: 4),
              HkIcon(HkIconName.check, size: 16, color: codeToColor(_value)),
            ],
          ],
        ),
      ),
    );
  }

  static String codeToLabel(String code) => HkTemperatureTag.labelFor(code);
  static Color codeToColor(String codeOrLabel) =>
      HkTemperatureTag.colorFor(codeOrLabel);
}
