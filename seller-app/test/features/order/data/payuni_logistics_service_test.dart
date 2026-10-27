import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/order/data/payuni_logistics_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('LogisticsInfo display labels', () {
    test('maps payuni goods and ship types to code meta labels', () {
      final info = LogisticsInfo.fromMap({
        'goodsType': 1,
        'shipType': 1,
        'serviceType': 1,
        'shipStatus': 31,
      });

      expect(info.goodsTypeText, ShippingTemperature.ambient.label);
      expect(info.shipTypeText, ShippingMethod.sevenElevenStorePickup.label);
      expect(info.shipStatusLabel, ShippingStatus.inTransit.label);
    });

    test(
      'falls back to payuni-specific status text when code meta has no label',
      () {
        final info = LogisticsInfo.fromMap({
          'goodsType': 2,
          'shipType': 2,
          'serviceType': 3,
          'shipStatus': 30,
        });

        expect(info.goodsTypeText, ShippingTemperature.frozen.label);
        expect(info.shipTypeText, ShippingMethod.taqbin.label);
        expect(info.shipStatusLabel, '已收件');
      },
    );
  });
}
