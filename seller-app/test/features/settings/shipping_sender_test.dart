import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/features/settings/data/shipping_sender.dart';
import 'package:hawkr_seller/features/settings/presentation/shipping_settings/data/taiwan_cities_data.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;

import '../../support/my_store_fixtures.dart';

MyStore _store({
  String contactName = '',
  String contactMobile = '',
  String contactPhone = '',
  String? townCode,
  String addressDetail = '',
}) => MyStore(
  myStoreResponse(
    contactName: contactName,
    contactMobile: contactMobile,
    contactPhone: contactPhone,
    townCode: townCode,
    addressDetail: addressDetail,
  ),
);

String _code(String county, String town) =>
    twaddr.townByName(county, town)!.code;

Map<String, dynamic> _homeRow({
  String name = '',
  String phone = '',
  String city = '',
  String district = '',
  String detail = '',
  String status = '4304',
}) => {
  'shippingType': '3652',
  'payuniSubmitStatus': status,
  'senderName': name,
  'senderPhone': phone,
  'senderCity': city,
  'senderDistrict': district,
  'senderAddressDetail': detail,
};

Map<String, dynamic> _cvsRow(
  String goods, {
  String name = '',
  String phone = '',
  String status = '4303',
}) => {
  'shippingType': '3651',
  'onboardingId': 'ob-$goods',
  'payuniSubmitStatus': status,
  'senderName': name,
  'senderPhone': phone,
};

void main() {
  group('黑貓寄件人來源', () {
    final store = _store(
      contactName: '王店長',
      contactMobile: '0912345678',
      contactPhone: '02-12345678',
      townCode: _code('臺東縣', '臺東市'),
      addressDetail: '中華路一段 1 號',
    );

    test('沒有紀錄時整組取店家，臺換成物流名稱表的台', () {
      final sender = homeSender(null, store);
      expect(sender.name, '王店長');
      expect(sender.phone, '0912345678');
      expect(sender.city, '台東縣');
      expect(sender.district, '台東市');
      expect(sender.addressDetail, '中華路一段 1 號');
    });

    test('有紀錄就整組取紀錄，失敗狀態也算，店家的值不混進來', () {
      final sender = homeSender(
        _homeRow(name: '舊寄件人', city: '高雄市', status: '4304'),
        store,
      );
      expect(sender.name, '舊寄件人');
      expect(sender.phone, '');
      expect(sender.city, '高雄市');
      expect(sender.district, '');
      expect(sender.addressDetail, '');
    });

    test('紀錄的寄件人值全空時視為沒有紀錄', () {
      final sender = homeSender(_homeRow(), store);
      expect(sender.name, '王店長');
      expect(sender.city, '台東縣');
    });

    test('店家缺的值留空，不退回商店電話', () {
      final sender = homeSender(null, _store(contactPhone: '02-12345678'));
      expect(sender.isEmpty, isTrue);
      expect(sender.phone, '');
    });

    test('店家只有鄉鎮沒有明細時明細留空', () {
      final sender = homeSender(null, _store(townCode: _code('雲林縣', '台西鄉')));
      expect(sender.city, '雲林縣');
      expect(sender.district, '台西鄉');
      expect(sender.addressDetail, '');
    });

    test('店家還沒讀到時是空的', () {
      expect(homeSender(null, null).isEmpty, isTrue);
    });

    test('紀錄裡的臺字也轉成物流名稱表的寫法', () {
      final sender = homeSender(
        _homeRow(name: '甲', city: '雲林縣', district: '臺西鄉'),
        null,
      );
      expect(sender.district, '台西鄉');
    });
  });

  group('7-11 寄件人來源', () {
    final store = _store(
      contactName: '王店長',
      contactMobile: '0912345678',
      townCode: _code('臺北市', '中正區'),
    );

    test('沒有紀錄時取店家的姓名與手機', () {
      final sender = cvsSender(null, store);
      expect(sender.name, '王店長');
      expect(sender.phone, '0912345678');
    });

    test('參考紀錄的寄件人值全空時用店家，不去翻另一筆紀錄', () {
      final sender = cvsSender(_cvsRow('3201'), store);
      expect(sender.name, '王店長');
    });

    test('失敗狀態的紀錄照用，不改用店家', () {
      final row = _cvsRow('3201', name: '紀錄人', status: '4304');
      expect(cvsSender(row, store).name, '紀錄人');
    });
  });

  group('已建店（toggle）', () {
    test('有 onboardingId 且 4303 才算', () {
      expect(isShippingRowToggleable(_cvsRow('3201')), isTrue);
      expect(isShippingRowToggleable(_cvsRow('3201', status: '4302')), isFalse);
      expect(isShippingRowToggleable(_cvsRow('3201', status: '4304')), isFalse);
      final noId = _cvsRow('3201')..remove('onboardingId');
      expect(isShippingRowToggleable(noId), isFalse);
    });

    test('4304 有寄件人姓名不算已建店', () {
      expect(
        isShippingRowToggleable(_cvsRow('3201', name: '甲', status: '4304')),
        isFalse,
      );
    });
  });

  group('鎖定', () {
    test('只有 4302 與 4303 鎖定', () {
      for (final status in ['4302', '4303']) {
        expect(
          isShippingRowLocked(_homeRow(name: '甲', status: status)),
          isTrue,
        );
      }
      for (final status in ['', '4301', '4304']) {
        expect(
          isShippingRowLocked(_homeRow(name: '甲', status: status)),
          isFalse,
        );
      }
    });

    test('有寄件人姓名但沒有狀態的紀錄不鎖定', () {
      final row = _homeRow(name: '甲', status: '');
      row['enabled'] = 'true';
      row['payuniSubmitted'] = true;
      expect(isShippingRowLocked(row), isFalse);
    });
  });

  group('郵遞區號由所選的區推導', () {
    test('一般縣市', () {
      expect(postalCodeOfSelection('台北市', '中正區'), '100');
    });

    test('台西鄉、霧台鄉、台東市等含台字的名稱', () {
      expect(postalCodeOfSelection('雲林縣', '台西鄉'), '638');
      expect(postalCodeOfSelection('屏東縣', '霧台鄉'), '902');
      expect(postalCodeOfSelection('台東縣', '台東市'), '950');
    });

    test('區對不到代碼時是 null', () {
      expect(postalCodeOfSelection('台北市', '不存在區'), isNull);
      expect(postalCodeOfSelection('台北市', '台東市'), isNull);
      expect(postalCodeOfSelection('台北市', null), isNull);
      expect(postalCodeOfSelection(null, '中正區'), isNull);
    });
  });

  group('物流名稱表與 twaddr', () {
    test('名稱表每個（縣市、區）都對得到鄉鎮，twaddr 每個鄉鎮轉成物流寫法後都在名稱表內', () {
      var pairs = 0;
      for (final MapEntry(key: city, value: districts)
          in taiwanCities.entries) {
        for (final district in districts) {
          pairs++;
          expect(
            twaddr.townByName(city, district),
            isNotNull,
            reason: '$city$district',
          );
        }
      }
      expect(taiwanCities.length, 22);
      expect(pairs, 368);
      for (final town in twaddr.towns) {
        final county = twaddr.countyByCode(town.countyCode)!;
        expect(
          taiwanCities[toShippingName(county.name)],
          contains(toShippingName(town.name)),
          reason: '${county.name}${town.name}',
        );
      }
    });

    test('台／臺的轉換只有一個入口', () {
      expect(toShippingName('臺東縣臺東市'), '台東縣台東市');
      expect(toShippingName('霧臺鄉'), '霧台鄉');
    });
  });

  group('姓名顯示長度', () {
    test('中文算 2、英文算 1，超過上限才提示', () {
      String? err(String v) =>
          senderNameLengthError(v, maxDisplayLength: 10, fieldLabel: '退貨人姓名');
      expect(err('一二三四五'), isNull);
      expect(err('一二三四五六'), contains('退貨人姓名長度過長'));
      expect(err('abcdefghij'), isNull);
      expect(err('abcdefghijk'), isNotNull);
    });

    test('空白不在這裡提示（必填另外檢查）', () {
      expect(
        senderNameLengthError('  ', maxDisplayLength: 10, fieldLabel: '退貨人姓名'),
        isNull,
      );
    });
  });
}
