import 'package:flutter/material.dart';

import '../../../../shared/widgets/hk_temperature_tag.dart';

/// 顯示商品實際回傳的配送溫層；缺值不推定為常溫。
class ProductTemperatureTags extends StatelessWidget {
  const ProductTemperatureTags({super.key, required this.deliveryTemp});

  final Object? deliveryTemp;

  @override
  Widget build(BuildContext context) {
    final raw = deliveryTemp;
    final values = (raw is List ? raw : [raw])
        .where((value) => value is String || value is num)
        .expand((value) => value.toString().split(','))
        .map((value) => value.trim())
        .where((value) => value.isNotEmpty)
        .map(HkTemperatureTag.labelFor)
        .toSet();
    if (values.isEmpty) return const Text('尚未指定');
    return Wrap(
      spacing: 4,
      runSpacing: 4,
      children: [for (final value in values) HkTemperatureTag(value: value)],
    );
  }
}
