import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:hawkr_seller/core/analytics/analytics_service.dart';
import 'package:hawkr_seller/core/analytics/analytics_events.dart';

class MockFirebaseAnalytics extends Mock implements FirebaseAnalytics {}

void main() {
  late MockFirebaseAnalytics fa;
  late AnalyticsEvents ev;
  setUp(() {
    fa = MockFirebaseAnalytics();
    when(
      () => fa.logEvent(
        name: any(named: 'name'),
        parameters: any(named: 'parameters'),
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
    ev = AnalyticsEvents(AnalyticsService(fa, enabled: true));
  });

  test('shipOrder sends order_no + shipping_method', () async {
    await ev.shipOrder('ORD-1', shippingMethod: '3002');
    verify(
      () => fa.logEvent(
        name: 'ship_order',
        parameters: {'order_no': 'ORD-1', 'shipping_method': '3002'},
      ),
    ).called(1);
  });

  test('cancelOrder with reason sends order_no + cancel_reason', () async {
    await ev.cancelOrder('ORD-1', cancelReason: '9201');
    verify(
      () => fa.logEvent(
        name: 'cancel_order',
        parameters: {'order_no': 'ORD-1', 'cancel_reason': '9201'},
      ),
    ).called(1);
  });

  test('cancelOrder without reason drops cancel_reason', () async {
    await ev.cancelOrder('ORD-2');
    verify(
      () =>
          fa.logEvent(name: 'cancel_order', parameters: {'order_no': 'ORD-2'}),
    ).called(1);
  });

  test('setProductStatus sends product_id + status', () async {
    await ev.setProductStatus('P1', 'listed');
    verify(
      () => fa.logEvent(
        name: 'set_product_status',
        parameters: {'product_id': 'P1', 'status': 'listed'},
      ),
    ).called(1);
  });

  test('login sets method + login_method property', () async {
    await ev.login('apple');
    verify(() => fa.logLogin(loginMethod: 'apple')).called(1);
    verify(
      () => fa.setUserProperty(name: 'login_method', value: 'apple'),
    ).called(1);
  });
}
