import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:hawkr_buyer/core/analytics/analytics_service.dart';

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
    when(() => fa.setUserId(id: any(named: 'id'))).thenAnswer((_) async {});
    when(
      () => fa.setUserProperty(
        name: any(named: 'name'),
        value: any(named: 'value'),
      ),
    ).thenAnswer((_) async {});
  });

  test('enabled service forwards logEvent', () async {
    final svc = AnalyticsService(fa, enabled: true);
    await svc.logEvent('view_item', {'currency': 'TWD'});
    verify(
      () => fa.logEvent(
        name: 'view_item',
        parameters: any(named: 'parameters'),
      ),
    ).called(1);
  });

  test('disabled service is a no-op', () async {
    final svc = AnalyticsService(fa, enabled: false);
    await svc.logEvent('view_item', {'currency': 'TWD'});
    await svc.setUserId('123');
    verifyNever(
      () => fa.logEvent(
        name: any(named: 'name'),
        parameters: any(named: 'parameters'),
      ),
    );
    verifyNever(() => fa.setUserId(id: any(named: 'id')));
  });

  test('logEvent drops null-valued params', () async {
    final svc = AnalyticsService(fa, enabled: true);
    await svc.logEvent('select_item', {'item_list_id': 'cat-1', 'x': null});
    final captured =
        verify(
              () => fa.logEvent(
                name: 'select_item',
                parameters: captureAny(named: 'parameters'),
              ),
            ).captured.single
            as Map;
    expect(captured.containsKey('x'), isFalse);
    expect(captured['item_list_id'], 'cat-1');
  });
}
