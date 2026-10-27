import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/analytics/ga4_item.dart';
import 'package:hawkr_buyer/features/product/data/product_model.dart';
import 'package:hawkr_buyer/features/cart/data/cart_model.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';

void main() {
  group('GA4Item.toAnalyticsEventItem', () {
    test('maps fields and stringifies the snowflake id', () {
      final item = const GA4Item(
        itemId: '4521000123',
        itemName: '有機高麗菜',
        price: 120,
        quantity: 2,
        itemVariant: '大份',
        itemBrand: '青山農場',
        index: 3,
      ).toAnalyticsEventItem();

      expect(item.itemId, '4521000123');
      expect(item.itemName, '有機高麗菜');
      expect(item.price, 120.0);
      expect(item.quantity, 2);
      expect(item.itemVariant, '大份');
      expect(item.itemBrand, '青山農場');
      expect(item.affiliation, '青山農場');
      expect(item.index, 3);
    });
  });

  group('productToGA4Item', () {
    test('uses effective (sale) price and seller as brand', () {
      final p = const Product(
        skuId: 4521000123,
        title: '有機高麗菜',
        price: 120,
        image: '',
        sellerId: 99,
        merchantName: '青山農場',
      );
      final g = productToGA4Item(p, index: 0);
      expect(g.itemId, '4521000123');
      expect(g.itemName, '有機高麗菜');
      expect(g.price, 120);
      expect(g.itemBrand, '青山農場');
      expect(g.index, 0);
    });
  });

  group('cartItemToGA4Item', () {
    test('maps quantity, variant and brand', () {
      final c = const CartItem(
        id: 1,
        skuId: 4521000123,
        count: 3,
        price: 120,
        title: '有機高麗菜',
        sellerId: 99,
        merchantName: '青山農場',
        specName: '大份',
      );
      final g = cartItemToGA4Item(c);
      expect(g.itemId, '4521000123');
      expect(g.quantity, 3);
      expect(g.price, 120);
      expect(g.itemVariant, '大份');
      expect(g.itemBrand, '青山農場');
    });
  });

  group('orderItemToGA4Item', () {
    test('parses string price and quantity', () {
      final o = const OrderItem(
        skuId: '4521000123',
        productName: '有機高麗菜',
        quantity: '2',
        unitPrice: '120',
      );
      final g = orderItemToGA4Item(o);
      expect(g.itemId, '4521000123');
      expect(g.itemName, '有機高麗菜');
      expect(g.price, 120);
      expect(g.quantity, 2);
    });
  });

  group('sumValue', () {
    test('sums price * quantity', () {
      const items = [
        GA4Item(itemId: 'a', itemName: 'A', price: 100, quantity: 2),
        GA4Item(itemId: 'b', itemName: 'B', price: 50, quantity: 1),
      ];
      expect(sumValue(items), 250);
    });

    test('treats missing quantity as 1 and missing price as 0', () {
      const items = [
        GA4Item(itemId: 'a', itemName: 'A', price: 100),
        GA4Item(itemId: 'b', itemName: 'B'),
      ];
      expect(sumValue(items), 100);
    });
  });
}
