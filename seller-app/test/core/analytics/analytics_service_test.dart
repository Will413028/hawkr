import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:hawkr_seller/core/analytics/analytics_service.dart';

class MockFirebaseAnalytics extends Mock implements FirebaseAnalytics {}

void main() {
  late MockFirebaseAnalytics fa;
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
    when(() => fa.setUserId(id: any(named: 'id'))).thenAnswer((_) async {});
    when(
      () => fa.setUserProperty(
        name: any(named: 'name'),
        value: any(named: 'value'),
      ),
    ).thenAnswer((_) async {});
  });

  test('enabled forwards logEvent', () async {
    await AnalyticsService(
      fa,
      enabled: true,
    ).logEvent('ship_order', {'order_no': 'A1'});
    verify(
      () => fa.logEvent(
        name: 'ship_order',
        parameters: any(named: 'parameters'),
      ),
    ).called(1);
  });

  test('disabled is a no-op', () async {
    final s = AnalyticsService(fa, enabled: false);
    await s.logEvent('ship_order', {'order_no': 'A1'});
    await s.logLogin('google');
    await s.setUserId('1');
    verifyNever(
      () => fa.logEvent(
        name: any(named: 'name'),
        parameters: any(named: 'parameters'),
      ),
    );
    verifyNever(() => fa.logLogin(loginMethod: any(named: 'loginMethod')));
    verifyNever(() => fa.setUserId(id: any(named: 'id')));
  });

  test('logEvent drops null params', () async {
    await AnalyticsService(
      fa,
      enabled: true,
    ).logEvent('e', {'a': '1', 'b': null});
    final captured =
        verify(
              () => fa.logEvent(
                name: 'e',
                parameters: captureAny(named: 'parameters'),
              ),
            ).captured.single
            as Map;
    expect(captured.containsKey('b'), isFalse);
    expect(captured['a'], '1');
  });
}
