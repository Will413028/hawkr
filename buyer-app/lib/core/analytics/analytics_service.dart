import 'package:firebase_analytics/firebase_analytics.dart';

/// The sole FirebaseAnalytics sink. Gated by [enabled] (prod-only by flavor),
/// mirroring web's `isEnabled()` gate on the measurement id.
class AnalyticsService {
  AnalyticsService(FirebaseAnalytics? fa, {required bool enabled})
    : _fa = fa,
      enabled = enabled && fa != null;

  /// `null` when Firebase is not configured; the service is then disabled.
  final FirebaseAnalytics? _fa;
  final bool enabled;

  /// Exposed for the navigator observer (screen_view).
  FirebaseAnalytics? get instance => _fa;

  Future<void> logEvent(String name, Map<String, Object?> params) async {
    final fa = _fa;
    if (!enabled || fa == null) return;
    await fa.logEvent(name: name, parameters: _clean(params));
  }

  /// Logs a GA4 ecommerce event whose payload includes an items array.
  /// Generic logEvent cannot carry nested item lists, so ecommerce events go
  /// through FirebaseAnalytics' typed methods via [logEcommerce].
  Future<void> logEcommerce(
    Future<void> Function(FirebaseAnalytics fa) call,
  ) async {
    final fa = _fa;
    if (!enabled || fa == null) return;
    await call(fa);
  }

  Future<void> setUserId(String? id) async {
    final fa = _fa;
    if (!enabled || fa == null) return;
    await fa.setUserId(id: id);
  }

  Future<void> setUserProperty(String name, String? value) async {
    final fa = _fa;
    if (!enabled || fa == null) return;
    await fa.setUserProperty(name: name, value: value);
  }

  Map<String, Object> _clean(Map<String, Object?> params) {
    final out = <String, Object>{};
    params.forEach((k, v) {
      if (v != null) out[k] = v;
    });
    return out;
  }
}
