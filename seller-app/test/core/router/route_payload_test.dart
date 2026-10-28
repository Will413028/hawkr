import 'dart:typed_data';

import 'package:hawkr_seller/core/router/route_payload.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('route payload adapters', () {
    test('valid typed and legacy order payloads are preserved', () {
      final typed = OrderSummary(
        id: 'typed-1',
        orderSn: 'typed-1',
        title: '待出貨',
        subtitle: '',
        dateTime: '',
        productName: '',
        price: '',
      );

      expect(orderSummaryFromRoutePayload(typed), same(typed));

      final legacy = orderSummaryFromRoutePayload({
        'order_no': 'legacy-2',
        'status': '3',
        'statusSummary': '配送中',
      });
      expect(legacy?.orderSn, 'legacy-2');
      expect(legacy?.statusCode, 3);
      expect(legacy?.title, '配送中');
    });

    test('missing extra can restore an order route from its query ID', () {
      expect(orderSummaryFromRoutePayload(null), isNull);

      final restored = orderSummaryFromRoutePayload(
        null,
        fallbackOrderSn: ' restored-3 ',
      );
      expect(restored?.orderSn, 'restored-3');
    });

    test('malformed maps and nested values never require a force cast', () {
      final payload = stringKeyedRouteMap({
        'title': '有效標題',
        7: 'non-string key',
        'nested': {1: 'ignored', 'name': '保留'},
        'bytes': <Object>[Uint8List(1), 'wrong type'],
        'blocks': [
          {'type': 'paragraph'},
          <Object, Object>{3: 'ignored', 'text': '內容'},
        ],
      });

      expect(payload, isNot(contains(7)));
      expect(routeNestedMap(payload, 'nested'), {'name': '保留'});
      expect(routeTypedList<Uint8List>(payload, 'bytes'), isNull);
      expect(routeMapList(payload, 'blocks'), [
        {'type': 'paragraph'},
        {'text': '內容'},
      ]);
      expect(orderSummaryFromRoutePayload({1: 'only malformed key'}), isNull);
    });
  });
}
