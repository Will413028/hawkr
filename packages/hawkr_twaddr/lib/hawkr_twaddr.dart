/// 台灣鄉鎮市區代碼與查詢。資料表由 backend/internal/platform/twaddr/generate.go 生成到
/// `src/twaddr_data.g.dart`（順序是郵遞區號），這裡只放手寫的查詢。
library;

import 'src/town.dart';
import 'src/twaddr_data.g.dart';

export 'src/town.dart';
export 'src/twaddr_data.g.dart';

final Map<String, Town> _townByCode = {for (final t in towns) t.code: t};

final Map<String, County> _countyByCode = {for (final c in counties) c.code: c};

final Map<String, List<Town>> _townsByCounty = () {
  final map = <String, List<Town>>{};
  for (final t in towns) {
    map.putIfAbsent(t.countyCode, () => []).add(t);
  }
  return map;
}();

final Map<String, Town> _townByName = () {
  final map = <String, Town>{};
  for (final c in counties) {
    for (final t in _townsByCounty[c.code] ?? const <Town>[]) {
      map['${c.name}\u0000${t.name}'] = t;
    }
  }
  return map;
}();

/// 以 8 碼代碼找鄉鎮市區；不是現行的代碼回 null。
Town? townByCode(String code) => _townByCode[code];

/// 以 5 碼代碼找縣市；不是現行的代碼回 null。
County? countyByCode(String code) => _countyByCode[code];

/// 指定縣市代碼下的鄉鎮市區（郵遞區號順序）。
List<Town> townsByCounty(String countyCode) =>
    List.unmodifiable(_townsByCounty[countyCode] ?? const <Town>[]);

/// 以縣市名稱與鄉鎮市區名稱精確找鄉鎮市區；兩個名稱先去掉前後空白，「台」換成「臺」
/// （內政部的寫法），找不到回 null。
Town? townByName(String countyName, String townName) =>
    _townByName['${_toTai(countyName)}\u0000${_toTai(townName)}'];

String _toTai(String name) => name.trim().replaceAll('台', '臺');

/// 縣市代碼是鄉鎮市區代碼的前 5 碼。
String countyCodeOfTownCode(String code) =>
    code.length < 5 ? code : code.substring(0, 5);

/// 以 8 碼鄉鎮市區代碼找郵遞區號；不認得的代碼回空字串。
String postalCodeByTownCode(String code) => _townByCode[code]?.postalCode ?? '';

/// 縣市＋鄉鎮市區＋明細；不認得的代碼只回明細。
String formatAddress(String townCode, String detail) {
  final town = _townByCode[townCode];
  if (town == null) return detail;
  final county = _countyByCode[town.countyCode];
  return '${county?.name ?? ''}${town.name}$detail';
}
