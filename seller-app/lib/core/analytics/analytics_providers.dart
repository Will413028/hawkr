import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../config/firebase_config.dart';
import '../../config/flavor.dart';
import 'analytics_events.dart';
import 'analytics_service.dart';

/// Collected in prod + dev (dev is the test bed for DebugView; stage unused).
/// Per-flavor Firebase projects route each environment to its own GA property,
/// so dev events never pollute prod reports. Mirrors the buyer gate.
bool analyticsEnabledForFlavor(Flavor flavor) =>
    flavor == Flavor.prod || flavor == Flavor.dev;

final analyticsServiceProvider = Provider<AnalyticsService>((ref) {
  return AnalyticsService(
    _analyticsOrNull(),
    enabled: analyticsEnabledForFlavor(F.appFlavor),
  );
});

final analyticsEventsProvider = Provider<AnalyticsEvents>((ref) {
  return AnalyticsEvents(ref.watch(analyticsServiceProvider));
});

/// `null` when Firebase is not initialized: touching [FirebaseAnalytics.instance]
/// would throw `core/no-app`.
FirebaseAnalytics? _analyticsOrNull() =>
    isFirebaseInitialized ? FirebaseAnalytics.instance : null;
