import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';

/// Delivery-temperature display shared with the Web TemperatureTag.
///
/// The 18dp pill grows with platform text scaling. It is informational; use
/// a separate 48dp control when selecting a delivery temperature.
class HkTemperatureTag extends StatelessWidget {
  const HkTemperatureTag({super.key, required this.value});

  final String value;

  static String labelFor(String value) => switch (value.trim().toUpperCase()) {
    '3201' || 'AMBIENT' || '常溫' => '常溫',
    '3202' || 'CHILLED' || '冷藏' => '冷藏',
    '3203' || 'FROZEN' || '冷凍' => '冷凍',
    _ => value,
  };

  static Color colorFor(String value) => switch (labelFor(value)) {
    '冷藏' => AppColors.temperatureChilled,
    '冷凍' => AppColors.temperatureFrozen,
    _ => AppColors.temperatureAmbient,
  };

  @override
  Widget build(BuildContext context) {
    if (value.trim().isEmpty) return const SizedBox.shrink();
    final color = colorFor(value);
    return Container(
      constraints: const BoxConstraints(minHeight: 18),
      padding: const EdgeInsets.symmetric(horizontal: 8),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color),
      ),
      child: Text(
        labelFor(value),
        softWrap: false,
        style: TextStyle(
          fontFamily: AppTypography.family,
          fontSize: 11,
          fontWeight: FontWeight.w700,
          height: 16 / 11,
          letterSpacing: 0,
          color: color,
        ),
      ),
    );
  }
}
