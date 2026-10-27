import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../session/session_model.dart';
import '../session/session_provider.dart';
import 'analytics_constants.dart';
import 'analytics_providers.dart';
import 'analytics_service.dart';

/// Pure side-effect: reflect the current session into GA4 identity.
/// Mirrors web `useAnalyticsIdentity` (set user_id + role on login; clear on
/// logout). `role` is fixed "buyer" for this app.
Future<void> applyIdentity(AnalyticsService svc, SessionModel? session) async {
  if (hasUsableSession(session)) {
    final activeSession = session!;
    await svc.setUserId(activeSession.userId);
    await svc.setUserProperty(kAnalyticsRoleProperty, 'buyer');
  } else {
    await svc.setUserId(null);
  }
}

/// Wires sessionProvider → applyIdentity. Call once during app startup from a
/// ConsumerState's initState (uses listenManual since we're outside build).
void registerAnalyticsIdentity(WidgetRef ref) {
  final svc = ref.read(analyticsServiceProvider);
  ref.listenManual<AsyncValue<SessionModel?>>(sessionProvider, (prev, next) {
    applyIdentity(svc, next.value);
  }, fireImmediately: true);
}
