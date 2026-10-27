import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:hawkr_buyer/core/analytics/analytics_service.dart';
import 'package:hawkr_buyer/core/analytics/analytics_events.dart';
import 'package:hawkr_buyer/core/analytics/ga4_item.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';

class MockFirebaseAnalytics extends Mock implements FirebaseAnalytics {}

void main() {
  late MockFirebaseAnalytics fa;
  late AnalyticsEvents events;

  setUp(() {
    fa = MockFirebaseAnalytics();
    when(
      () => fa.logViewItem(
        currency: any(named: 'currency'),
        value: any(named: 'value'),
        items: any(named: 'items'),
      ),
    ).thenAnswer((_) async {});
    when(
      () => fa.logPurchase(
        currency: any(named: 'currency'),
        value: any(named: 'value'),
        transactionId: any(named: 'transactionId'),
        shipping: any(named: 'shipping'),
        items: any(named: 'items'),
      ),
    ).thenAnswer((_) async {});
    when(
      () => fa.logLogin(loginMethod: any(named: 'loginMethod')),
    ).thenAnswer((_) async {});
    when(
      () => fa.setUserProperty(
        name: any(named: 'name'),
        value: any(named: 'value'),
      ),
    ).thenAnswer((_) async {});
    events = AnalyticsEvents(AnalyticsService(fa, enabled: true));
  });

  test('trackViewItem sends currency, value and one item', () async {
    const item = GA4Item(itemId: '1', itemName: 'A', price: 120);
    await events.trackViewItem(item);
    verify(
      () => fa.logViewItem(
        currency: 'TWD',
        value: 120,
        items: any(named: 'items', that: hasLength(1)),
      ),
    ).called(1);
  });

  test('trackPurchase derives value/shipping from the order', () async {
    const order = Order(
      orderSn: 'ORD-1',
      totalActualAmount: '350',
      shippingFee: '60',
      items: [
        OrderItem(
          skuId: '1',
          productName: 'A',
          quantity: '2',
          unitPrice: '120',
        ),
        OrderItem(skuId: '2', productName: 'B', quantity: '1', unitPrice: '50'),
      ],
    );
    await events.trackPurchase(order);
    verify(
      () => fa.logPurchase(
        currency: 'TWD',
        value: 350,
        transactionId: 'ORD-1',
        shipping: 60,
        items: any(named: 'items', that: hasLength(2)),
      ),
    ).called(1);
  });

  test('trackLogin sets method and login_method user property', () async {
    await events.trackLogin('google');
    verify(() => fa.logLogin(loginMethod: 'google')).called(1);
    verify(
      () => fa.setUserProperty(name: 'login_method', value: 'google'),
    ).called(1);
  });
}
