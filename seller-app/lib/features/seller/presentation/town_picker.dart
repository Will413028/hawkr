import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;
import 'package:flutter/material.dart';

import '../../../shared/widgets/app_dropdown.dart';

/// 縣市與鄉鎮市區的選擇（內政部代碼表）。選了縣市還沒選鄉鎮市區時 [townCode] 是空字串。
class TownPicker extends StatelessWidget {
  const TownPicker({
    super.key,
    required this.countyCode,
    required this.townCode,
    required this.onChanged,
    this.gap = 12,
  });

  final String countyCode;
  final String townCode;
  final double gap;

  /// 縣市或鄉鎮市區改變；換縣市時 townCode 清成空字串。
  final void Function(String countyCode, String townCode) onChanged;

  @override
  Widget build(BuildContext context) {
    final counties = twaddr.counties;
    final towns = countyCode.isEmpty
        ? const <twaddr.Town>[]
        : twaddr.townsByCounty(countyCode);
    return Row(
      children: [
        Expanded(
          child: AppDropdown(
            items: [for (final c in counties) c.name],
            selectedItem: twaddr.countyByCode(countyCode)?.name,
            placeholder: '選擇縣市',
            showSearch: true,
            onSelected: (name) {
              final county = counties.firstWhere((c) => c.name == name);
              onChanged(county.code, '');
            },
          ),
        ),
        SizedBox(width: gap),
        Expanded(
          child: AppDropdown(
            items: [for (final t in towns) t.name],
            selectedItem: twaddr.townByCode(townCode)?.name,
            placeholder: '選擇行政區',
            onSelected: (name) {
              final town = towns.firstWhere((t) => t.name == name);
              onChanged(countyCode, town.code);
            },
          ),
        ),
      ],
    );
  }
}
