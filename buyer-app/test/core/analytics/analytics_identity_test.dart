import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:hawkr_buyer/core/analytics/analytics_service.dart';
import 'package:hawkr_buyer/core/analytics/analytics_identity.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';

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

  test('applyIdentity sets user_id + role on a logged-in session', () async {
    await applyIdentity(
      svc,
      const SessionModel(userId: '4521000123', nickname: 'A'),
    );
    verify(() => fa.setUserId(id: '4521000123')).called(1);
    verify(() => fa.setUserProperty(name: 'role', value: 'buyer')).called(1);
  });

  test('applyIdentity clears user_id on logout (null session)', () async {
    await applyIdentity(svc, null);
    verify(() => fa.setUserId(id: null)).called(1);
  });
}
