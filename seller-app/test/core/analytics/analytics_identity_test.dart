import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:hawkr_seller/core/analytics/analytics_service.dart';
import 'package:hawkr_seller/core/analytics/analytics_identity.dart';
import 'package:hawkr_seller/core/session/session_model.dart';

class MockFirebaseAnalytics extends Mock implements FirebaseAnalytics {}

void main() {
  late MockFirebaseAnalytics fa;
  late AnalyticsService svc;
  setUp(() {
    fa = MockFirebaseAnalytics();
    when(() => fa.setUserId(id: any(named: 'id'))).thenAnswer((_) async {});
    when(
      () => fa.setUserProperty(
        name: any(named: 'name'),
        value: any(named: 'value'),
      ),
    ).thenAnswer((_) async {});
    svc = AnalyticsService(fa, enabled: true);
  });

  test(
    'applyIdentity sets user_id + seller_id + role when logged in',
    () async {
      await applyIdentity(
        svc,
        const SessionModel(accountId: 'acc-42', isLoggedIn: true),
        storeId: '99',
      );
      verify(() => fa.setUserId(id: 'acc-42')).called(1);
      verify(
        () => fa.setUserProperty(name: 'seller_id', value: '99'),
      ).called(1);
      verify(() => fa.setUserProperty(name: 'role', value: 'seller')).called(1);
    },
  );

  test('applyIdentity clears user_id when logged out', () async {
    await applyIdentity(
      svc,
      const SessionModel(),
    ); // isLoggedIn defaults to false
    verify(() => fa.setUserId(id: null)).called(1);
    verifyNever(
      () => fa.setUserProperty(
        name: 'seller_id',
        value: any(named: 'value'),
      ),
    );
    verifyNever(
      () => fa.setUserProperty(
        name: 'role',
        value: any(named: 'value'),
      ),
    );
  });
}
