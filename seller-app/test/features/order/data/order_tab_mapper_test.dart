import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/data/order_tab_mapper.dart';
import 'package:flutter_test/flutter_test.dart';

OrderSummary _buildOrder({
  required String id,
  int? statusCode,
  String subtitle = '',
  String shipmentStatus = '',
  String shippingMethod = '',
}) {
  return OrderSummary(
    id: id,
    orderSn: id,
    statusCode: statusCode,
    title: 'order-$id',
    subtitle: subtitle,
    dateTime: '',
    productName: 'demo',
    price: '99',
    shipmentStatus: shipmentStatus,
    shippingMethod: shippingMethod,
  );
}

void main() {
  group('tabForOrder', () {
    test('maps pending shipment statuses to 待出貨 tab', () {
      final order = _buildOrder(id: 'pending', statusCode: 10);

      expect(tabForOrder(order), 0);
    });

    test('maps in-transit statuses to 配送中 tab', () {
      final order = _buildOrder(id: 'transit', statusCode: 30);

      expect(tabForOrder(order), 1);
    });

    test('maps closed statuses to 已結束 tab', () {
      final completedOrder = _buildOrder(id: 'completed', statusCode: 50);
      expect(tabForOrder(completedOrder), 2);
    });

    test('已取消(80) 也落在第三桶 —— 這正是它不能叫「已完成」的原因', () {
      final cancelledOrder = _buildOrder(id: 'cancelled', statusCode: 80);
      expect(tabForOrder(cancelledOrder), 2);
    });

    test('maps delivered status to 配送中 tab', () {
      final deliveredOrder = _buildOrder(id: 'delivered', statusCode: 40);
      expect(tabForOrder(deliveredOrder), 1);
    });

    test('uses tcat called shipment status as 配送中 override', () {
      final order = _buildOrder(
        id: 'tcat',
        statusCode: 20,
        shipmentStatus: ShippingStatus.tcatCalled.value,
        shippingMethod: ShippingMethod.taqbin.value,
      );

      expect(tabForOrder(order), 1);
    });

    test('returns -1 for unpaid orders', () {
      final order = _buildOrder(id: 'unpaid', statusCode: 0);

      expect(tabForOrder(order), -1);
    });
  });

  // 桶的成員與桶的名字要對得上。第三桶含 80 已取消 / 70 全額退款，所以不能叫
  // 「已完成」—— 那會讓賣家把它讀成成交數（dev 實測同店 81 筆時顯示 75，實際成交
  // 只有 19 筆）。「已完成」是 CodeMeta OrderStatus 50 單一狀態的 Label。
  // 對應的另兩支：backend OrderServiceSellerStatusCountsTest、
  // web tests/seller-order-tab-labels.test.ts。
  group('sellerOrderTabLabels', () {
    test('第三桶不得叫「已完成」——它含已取消/全額退款', () {
      expect(completedOrderStatuses, contains(80));
      expect(completedOrderStatuses, contains(70));
      expect(sellerOrderTabLabels[2], isNot('已完成'));
      expect(sellerOrderTabLabels[2], '已結束');
    });

    test('三個 tab 各有標籤且互不重複', () {
      expect(sellerOrderTabLabels, hasLength(3));
      expect(sellerOrderTabLabels.every((l) => l.isNotEmpty), isTrue);
      expect(sellerOrderTabLabels.toSet(), hasLength(3));
    });
  });
}
