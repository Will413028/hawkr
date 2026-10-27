import 'package:firebase_analytics/firebase_analytics.dart';

/// The sole FirebaseAnalytics sink, flavor-gated (prod only). No-op when disabled.
class AnalyticsService {
  AnalyticsService(FirebaseAnalytics? fa, {required bool enabled})
    : _fa = fa,
      enabled = enabled && fa != null;

  /// `null` when Firebase is not configured; the service is then disabled.
  final FirebaseAnalytics? _fa;
  final bool enabled;

  FirebaseAnalytics? get instance => _fa; // for the navigator observer

  Future<void> logEvent(String name, Map<String, Object?> params) async {
    final fa = _fa;
    if (!enabled || fa == null) return;
    await fa.logEvent(name: name, parameters: _clean(params));
  }

  Future<void> logLogin(String method) async {
    final fa = _fa;
    if (!enabled || fa == null) return;
    await fa.logLogin(loginMethod: method);
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

  Map<String, Object> _clean(Map<String, Object?> p) {
    final out = <String, Object>{};
    p.forEach((k, v) {
      if (v != null) out[k] = v;
    });
    return out;
  }
}
