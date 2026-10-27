import 'package:hawkr_buyer/features/address/data/store_pickup_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('normalizes the OpenAPI camelCase response', () {
    final pickup = StorePickup.fromJson({
      'id': 'pickup-1',
      'receiverName': '王小明',
      'receiverPhone': '0912345678',
      'storeCode': '123456',
      'storeName': '民生門市',
      'storeAddress': '臺北市中山區',
      'isDefault': 'true',
    });

    expect(pickup.id, 'pickup-1');
    expect(pickup.storeCode, '123456');
    expect(pickup.isDefault, isTrue);
    expect(pickup.toJson()['isDefault'], isTrue);
  });

  // /app/store_pickups 的欄名一律 camelCase，沒有相容層。只餵舊的
  // snake_case 名字必須整筆讀不到 —— 誰把 fallback 加回來這條就轉紅。
  test('只餵舊 snake_case 欄名一律讀不到（無相容層）', () {
    final pickup = StorePickup.fromJson({
      'id': 42,
      'receiver_name': '陳小美',
      'receiver_phone': '0987654321',
      'store_code': '654321',
      'store_name': '復興門市',
      'store_address': '新北市板橋區',
      'is_default': '1',
    });

    expect(pickup.id, '42');
    expect(pickup.receiverName, isEmpty);
    expect(pickup.receiverPhone, isEmpty);
    expect(pickup.storeCode, isEmpty);
    expect(pickup.storeName, isEmpty);
    expect(pickup.storeAddress, isEmpty);
    expect(pickup.isDefault, isFalse);
  });
}
