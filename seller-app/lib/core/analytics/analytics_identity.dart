import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/seller/data/current_store.dart';
import '../session/session_provider.dart';
import 'analytics_constants.dart';
import 'analytics_providers.dart';
import 'analytics_service.dart';

/// Reflect the session into GA4 identity. user_id=accountId (cross-app stitch),
/// seller_id + role='seller' as user properties. Clears user_id on logout.
Future<void> applyIdentity(
  AnalyticsService svc,
  SessionModel? session, {
  String? storeId,
}) async {
  if (session != null && session.isLoggedIn) {
    await svc.setUserId(session.accountId?.toString());
    await svc.setUserProperty(kAnalyticsSellerIdProperty, storeId);
    await svc.setUserProperty(kAnalyticsRoleProperty, 'seller');
  } else {
    await svc.setUserId(null);
  }
}

/// Subscribe sessionProvider and sellerStoreIdProvider → applyIdentity. Call once at startup from a
/// ConsumerState initState via ref.listenManual. sessionProvider exposes
/// SessionModel synchronously (not AsyncValue), so [next] is used directly.
void registerAnalyticsIdentity(WidgetRef ref) {
  final svc = ref.read(analyticsServiceProvider);
  ref.listenManual(
    sessionProvider,
    (prev, next) =>
        applyIdentity(svc, next, storeId: ref.read(sellerStoreIdProvider)),
    fireImmediately: true,
  );
  ref.listenManual(
    sellerStoreIdProvider,
    (prev, next) =>
        applyIdentity(svc, ref.read(sessionProvider), storeId: next),
  );
}
