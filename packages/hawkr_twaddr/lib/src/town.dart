/// Town 是一個鄉鎮市區。名稱照內政部的寫法（臺）。
class Town {
  final String code;
  final String name;
  final String countyCode;
  final String postalCode;

  const Town({
    required this.code,
    required this.name,
    required this.countyCode,
    required this.postalCode,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Town &&
          code == other.code &&
          name == other.name &&
          countyCode == other.countyCode &&
          postalCode == other.postalCode;

  @override
  int get hashCode => Object.hash(code, name, countyCode, postalCode);

  @override
  String toString() => 'Town($code, $name, $postalCode)';
}

/// County 是一個縣市。
class County {
  final String code;
  final String name;

  const County({required this.code, required this.name});

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is County && code == other.code && name == other.name;

  @override
  int get hashCode => Object.hash(code, name);

  @override
  String toString() => 'County($code, $name)';
}
