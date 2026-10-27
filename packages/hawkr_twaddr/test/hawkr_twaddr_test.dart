import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:test/test.dart';

void main() {
  test('縣市數量為 22，鄉鎮市區數量為 368', () {
    expect(counties.length, 22);
    expect(towns.length, 368);
  });

  test('依代碼查詢鄉鎮市區與縣市', () {
    final t = townByCode('63000050');
    expect(t, isNotNull);
    expect(countyByCode(t!.countyCode)!.name, '臺北市');
    expect(t.name, '中正區');
    expect(t.postalCode, '100');
    expect(t.countyCode, '63000');

    final c = countyByCode('63000');
    expect(c, isNotNull);
    expect(c!.name, '臺北市');

    expect(townByCode('non-existent'), isNull);
    expect(countyByCode('non-existent'), isNull);
  });

  test('townsByCounty 回傳該縣市之鄉鎮市區', () {
    final tpeTowns = townsByCounty('63000');
    expect(tpeTowns.length, 12);
    expect(tpeTowns.every((t) => t.countyCode == '63000'), isTrue);
  });

  test('townByName 以縣市名與鄉鎮名精確比對，台與臺視為同一字', () {
    expect(townByName('台北市', '中正區')?.code, '63000050');
    expect(townByName('臺北市', '中正區')?.code, '63000050');
    final xiTai = townByName('雲林縣', '台西鄉');
    expect(xiTai?.name, '臺西鄉');
    expect(townByName('雲林縣', '臺西鄉')?.code, xiTai?.code);
    expect(townByName('屏東縣', '霧台鄉')?.name, '霧臺鄉');
    final taitung = townByName('台東縣', '台東市');
    expect(taitung?.name, '臺東市');
    expect(countyByCode(taitung!.countyCode)!.name, '臺東縣');
    expect(postalCodeByTownCode(taitung.code), '950');
  });

  test('townByName 去掉名稱前後的空白', () {
    expect(townByName(' 台北市 ', '\t中正區\n')?.code, '63000050');
    expect(townByName('台東縣', ' 台東市')?.name, '臺東市');
  });

  test('townByName 找不到、縣市與鄉鎮不搭配、空字串都回 null', () {
    expect(townByName('台北市', '台東市'), isNull);
    expect(townByName('台北市', '不存在區'), isNull);
    expect(townByName('不存在縣', '中正區'), isNull);
    expect(townByName('', ''), isNull);
    expect(townByName('台北市', ''), isNull);
  });

  test('townByName 對每個鄉鎮市區都能以其縣市名與鄉鎮名找回自己', () {
    for (final t in towns) {
      final c = countyByCode(t.countyCode)!;
      expect(townByName(c.name, t.name)?.code, t.code);
      expect(
        townByName(
          c.name.replaceAll('臺', '台'),
          t.name.replaceAll('臺', '台'),
        )?.code,
        t.code,
      );
    }
  });

  test('formatAddress 以代碼組出地址，未知代碼只回明細', () {
    expect(formatAddress('63000050', '重慶南路一段 122 號'), '臺北市中正區重慶南路一段 122 號');
    expect(formatAddress('unknown', '某地'), '某地');
    expect(postalCodeByTownCode('63000050'), '100');
    expect(postalCodeByTownCode('unknown'), '');
    expect(countyCodeOfTownCode('63000050'), '63000');
  });

  test('下拉順序：臺北市是第一個縣市，鄉鎮市區依郵遞區號遞增', () {
    expect(counties.first.name, '臺北市');
    for (var i = 1; i < towns.length; i++) {
      final a = towns[i - 1];
      final b = towns[i];
      final cmp = a.postalCode.compareTo(b.postalCode);
      expect(cmp < 0 || (cmp == 0 && a.code.compareTo(b.code) < 0), isTrue);
    }
  });
}
