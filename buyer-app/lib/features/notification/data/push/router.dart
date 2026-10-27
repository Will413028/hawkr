import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routing/app_router.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../order/data/after_sale_status_notifier.dart';
import '../../../order/data/order_status_notifier.dart';
import '../../../product/presentation/merchant_item_detail_screen.dart';
import 'destination.dart';

/// Notification navigation entry point.
///
/// Payload parsing and delivery ordering live in pure, testable classes. This
/// adapter is responsible only for checking app readiness and translating a
/// typed destination into the existing GoRouter routes.
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

    final destination = action.destination;
    final referenceId = destination.referenceId;
    HKLog.logI(
      '[NotificationRouter] ${action.normalizedType} '
      '→ ${destination.kind.name} ($referenceId)',
    );

    switch (destination.kind) {
      case NotificationDestinationKind.orders:
        context.go('/?tab=3');
      case NotificationDestinationKind.orderDetail:
        if (referenceId == null || referenceId.isEmpty) return false;
        context.push('/orders/detail', extra: {'orderSn': referenceId});
      case NotificationDestinationKind.afterSaleDetail:
        if (referenceId == null || referenceId.isEmpty) return false;
        context.push(
          '/settings/after-sale/detail',
          extra: {'caseNo': referenceId},
        );
      case NotificationDestinationKind.chat:
        context.push('/chat');
      case NotificationDestinationKind.chatRoom:
        if (referenceId == null || referenceId.isEmpty) return false;
        context.push(
          '/chat/room',
          extra: {'conversationId': referenceId, 'merchantId': ''},
        );
      case NotificationDestinationKind.coupons:
        context.push('/wallet/coupons');
      case NotificationDestinationKind.socialPost:
        if (referenceId == null || referenceId.isEmpty) return false;
        context.push('/social/post/$referenceId');
      case NotificationDestinationKind.product:
        if (referenceId == null || referenceId.isEmpty) return false;
        context.push(
          '/merchant-product',
          extra: MerchantItemDetailScreen(skuId: referenceId),
        );
      case NotificationDestinationKind.none:
        break;
    }

    _dispatchStatusRefresh(action);
    return true;
  }

  static void _dispatchStatusRefresh(NotificationAction action) {
    switch (action.statusRefresh) {
      case NotificationStatusRefresh.none:
        return;
      case NotificationStatusRefresh.order:
        WidgetsBinding.instance.addPostFrameCallback((_) {
          OrderStatusNotifier.instance.handlePushNotification(action.payload);
        });
      case NotificationStatusRefresh.afterSale:
        WidgetsBinding.instance.addPostFrameCallback((_) {
          AfterSaleStatusNotifier.instance.handlePushNotification(
            action.payload,
          );
        });
    }
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
