const _certificationValueByIndex = <String>[
  '3701', // ORGANIC
  '3702', // TAP
  '3703', // CAS
  '3704', // HACCP
  '3705', // ISO
  '3706', // GMP
];

const _certificationLabelByValue = <String, String>{
  '3701': '有機農產品標章',
  '3702': '產銷履歷標章TAP',
  '3703': 'CAS 優良農產品',
  '3704': 'HACCP 認證',
  '3705': 'ISO 認證',
  '3706': 'GMP 認證',
};

const _certificationValueByLegacyCode = <String, String>{
  'ORGANIC': '3701',
  'TAP': '3702',
  'CAS': '3703',
  'HACCP': '3704',
  'ISO': '3705',
  'GMP': '3706',
};

String certificationValueForIndex(int index) {
  if (index < 0 || index >= _certificationValueByIndex.length) {
    return _certificationValueByIndex.first;
  }
  return _certificationValueByIndex[index];
}

String certificationValueForRaw(String raw) {
  final normalized = raw.trim();
  if (normalized.isEmpty) return _certificationValueByIndex.first;
  if (_certificationLabelByValue.containsKey(normalized)) return normalized;

  final legacy =
      _certificationValueByLegacyCode[normalized.toUpperCase().trim()];
  if (legacy != null) return legacy;

  return normalized;
}

String certificationLabelForRaw(String raw) {
  final value = certificationValueForRaw(raw);
  return _certificationLabelByValue[value] ?? raw.trim();
}

Map<String, dynamic> certificationInputForSelection(int index, String certNo) {
  return {'name': certificationValueForIndex(index), 'certNo': certNo.trim()};
}
