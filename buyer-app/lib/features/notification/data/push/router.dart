import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routing/app_router.dart';
import '../../../../shared/utils/hk_log.dart';
import 'deep_link.dart';
import 'destination.dart';

/// Notification navigation entry point.
///
/// Payload parsing and delivery ordering live in pure, testable classes. This
/// adapter is responsible only for checking app readiness and handing the server's
/// deep link to GoRouter.
class NotificationRouter {
  NotificationRouter._();

  static bool Function() _sessionReady = _notConfigured;

  static final NotificationNavigationCoordinator _coordinator =
      NotificationNavigationCoordinator(
        isReady: _isNavigationReady,
        execute: _execute,
      );

  /// Called once by application bootstrap. "Ready" means session restoration
  /// has resolved (authenticated or guest), not that a user must be signed in.
  /// GoRouter remains the SSOT for login redirects and continue URLs.
  static void configureSessionReadiness(bool Function() isSessionReady) {
    _sessionReady = isSessionReady;
    _scheduleFlush();
  }

  /// Resolve and deliver a push/local payload, or queue it until both session
  /// restoration and the root Navigator are ready.
  static NotificationEnqueueResult navigate(
    Map<String, dynamic> data, {
    String? messageId,
  }) {
    final result = _coordinator.enqueue(data, sourceMessageId: messageId);
    switch (result) {
      case NotificationEnqueueResult.delivered:
        HKLog.logD('[NotificationRouter] delivered');
      case NotificationEnqueueResult.queued:
        HKLog.logD('[NotificationRouter] queued until app is ready');
        _scheduleFlush();
      case NotificationEnqueueResult.duplicate:
        HKLog.logD('[NotificationRouter] duplicate delivery ignored');
      case NotificationEnqueueResult.ignored:
        HKLog.logW('[NotificationRouter] unknown or malformed payload ignored');
    }
    return result;
  }

  /// Flushes cold-start actions after the app's first routed frame, and can be
  /// called safely on lifecycle/session transitions.
  static int flushPending() => _coordinator.flush();

  static bool _isNavigationReady() {
    if (!_sessionReady()) return false;
    final navigator = rootNavigatorKey.currentState;
    return navigator != null &&
        navigator.mounted &&
        rootNavigatorKey.currentContext != null;
  }

  static bool _execute(NotificationAction action) {
    final context = rootNavigatorKey.currentContext;
    if (context == null) return false;
    HKLog.logI('[NotificationRouter] → ${action.deepLink}');
    return pushDeepLink(context, action.deepLink);
  }

  /// 把伺服器給的深連結交給 GoRouter；不是 App 內路徑就不動作並回 false。
  static bool pushDeepLink(BuildContext context, String deepLink) {
    final location = resolveNotificationLocation(deepLink);
    if (location == null) return false;
    context.push(location);
    return true;
  }

  static void _scheduleFlush() {
    final binding = WidgetsBinding.instance;
    binding.addPostFrameCallback((_) => flushPending());
  }

  static bool _notConfigured() => false;

  @visibleForTesting
  static int get debugPendingCount => _coordinator.pendingCount;

  @visibleForTesting
  static void debugReset() {
    _coordinator.reset();
    _sessionReady = _notConfigured;
  }
}
