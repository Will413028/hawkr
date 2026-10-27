import 'dart:collection';
import 'dart:convert';

/// Key embedded into local-notification payloads so the main isolate can
/// correlate a tap with the originating FCM delivery.
const notificationSourceMessageIdKey = '_fcmMessageId';

enum NotificationDestinationKind {
  orders,
  orderDetail,
  afterSaleDetail,
  chat,
  chatRoom,
  coupons,
  socialPost,
  product,
  none,
}

enum NotificationStatusRefresh { none, order, afterSale }

/// A typed, UI-independent navigation target resolved from a push payload.
class NotificationDestination {
  const NotificationDestination(this.kind, {this.referenceId});

  final NotificationDestinationKind kind;
  final String? referenceId;
}

/// The complete action produced by payload resolution.
///
/// [payload] is retained only for the existing order/after-sale refresh
/// notifiers. Navigation code does not parse the untyped map again.
class NotificationAction {
  const NotificationAction({
    required this.destination,
    required this.normalizedType,
    required this.payload,
    this.statusRefresh = NotificationStatusRefresh.none,
  });

  final NotificationDestination destination;
  final String normalizedType;
  final Map<String, dynamic> payload;
  final NotificationStatusRefresh statusRefresh;
}

/// Pure notification contract resolver. Unknown or malformed actionable
/// payloads return null instead of reaching the Navigator.
class NotificationDestinationResolver {
  const NotificationDestinationResolver._();

  static NotificationAction? resolve(Map<String, dynamic> rawPayload) {
    final payload = Map<String, dynamic>.unmodifiable(rawPayload);
    final type = _firstNonEmpty(payload, const [
      'type',
      'refType',
      'eventType',
    ]);
    if (type.isEmpty) return null;

    final normalized = type.toUpperCase();
    final refId = _firstNonEmpty(payload, const ['refId']);

    switch (normalized) {
      case 'ORDER':
      case 'ORDER_PAID':
      case 'ORDER_SHIPPED':
      case 'ORDER_ARRIVED':
      case 'ORDER_CANCELLED':
      case 'ORDER_CREATED':
      case 'ORDER_DELIVERED':
      case 'ORDER_COMPLETED':
      case 'ORDER_STATUS':
        final orderSn = _firstNonEmpty(payload, const [
          'order_id',
          'orderSn',
          'orderNo',
          'refId',
        ]);
        return NotificationAction(
          destination: orderSn.isEmpty
              ? const NotificationDestination(
                  NotificationDestinationKind.orders,
                )
              : NotificationDestination(
                  NotificationDestinationKind.orderDetail,
                  referenceId: orderSn,
                ),
          normalizedType: normalized,
          payload: payload,
          statusRefresh: NotificationStatusRefresh.order,
        );

      case 'AFTERSALE':
      case 'AFTER_SALE':
      case 'AFTER_SALE_STATUS':
      case 'RETURN_SHIPPED':
      case 'REFUND':
      case 'REFUND_REQUESTED':
      case 'REFUND_COMPLETED':
      case 'REFUND_APPROVED':
      case 'REFUND_REJECTED':
      case 'ORDER_REFUNDED':
        final caseNo = _firstNonEmpty(payload, const [
          'caseNo',
          'case_no',
          'refId',
        ]);
        final orderSn = _firstNonEmpty(payload, const [
          'order_id',
          'orderSn',
          'orderNo',
        ]);
        return NotificationAction(
          destination: caseNo.isNotEmpty
              ? NotificationDestination(
                  NotificationDestinationKind.afterSaleDetail,
                  referenceId: caseNo,
                )
              : orderSn.isNotEmpty
              ? NotificationDestination(
                  NotificationDestinationKind.orderDetail,
                  referenceId: orderSn,
                )
              : const NotificationDestination(
                  NotificationDestinationKind.orders,
                ),
          normalizedType: normalized,
          payload: payload,
          statusRefresh: NotificationStatusRefresh.afterSale,
        );

      case 'CHAT':
      case 'CHAT_MESSAGE':
        final conversationId = _firstNonEmpty(payload, const [
          'conversation_id',
          'conversationId',
          'refId',
        ]);
        return NotificationAction(
          destination: conversationId.isEmpty
              ? const NotificationDestination(NotificationDestinationKind.chat)
              : NotificationDestination(
                  NotificationDestinationKind.chatRoom,
                  referenceId: conversationId,
                ),
          normalizedType: normalized,
          payload: payload,
        );

      case 'COUPON_RECEIVED':
      case 'COUPON_EXPIRING':
      case 'COUPON_FREE_SHIPPING':
      case 'PROMOTION':
        return NotificationAction(
          destination: const NotificationDestination(
            NotificationDestinationKind.coupons,
          ),
          normalizedType: normalized,
          payload: payload,
        );

      case 'POST_FOLLOWED_AUTHOR':
      case 'POST_FRIEND_REMIND':
      case 'POST_NEW_COMMENT':
      case 'POST_COMMENT_REPLY':
      case 'POST_LIKE':
      case 'POST_CREATED':
        final postId = refId.isNotEmpty
            ? refId
            : _firstNonEmpty(payload, const ['postId', 'post_id']);
        if (postId.isEmpty) return null;
        return NotificationAction(
          destination: NotificationDestination(
            NotificationDestinationKind.socialPost,
            referenceId: postId,
          ),
          normalizedType: normalized,
          payload: payload,
        );

      case 'RESTOCK_AVAILABLE':
        final productId = _firstNonEmpty(payload, const [
          'product_id',
          'productId',
          'refId',
        ]);
        if (productId.isEmpty) return null;
        return NotificationAction(
          destination: NotificationDestination(
            NotificationDestinationKind.product,
            referenceId: productId,
          ),
          normalizedType: normalized,
          payload: payload,
        );

      case 'SYSTEM_MAINTENANCE':
      case 'TERMS_UPDATE':
      case 'PAYUNI_STATUS':
        return NotificationAction(
          destination: const NotificationDestination(
            NotificationDestinationKind.none,
          ),
          normalizedType: normalized,
          payload: payload,
        );

      default:
        return null;
    }
  }

  static String _firstNonEmpty(
    Map<String, dynamic> payload,
    List<String> keys,
  ) {
    for (final key in keys) {
      final value = payload[key]?.toString().trim();
      if (value != null && value.isNotEmpty) return value;
    }
    return '';
  }
}

/// Stable identity shared by FCM, background-open and local-notification paths.
class NotificationMessageIdentity {
  const NotificationMessageIdentity._();

  static String fromPayload(
    Map<String, dynamic> payload, {
    String? sourceMessageId,
  }) {
    final explicit = _firstNonEmpty([
      sourceMessageId,
      payload[notificationSourceMessageIdKey],
      payload['message_id'],
      payload['messageId'],
      payload['notification_id'],
      payload['notificationId'],
      payload['event_id'],
      payload['eventId'],
    ]);
    if (explicit.isNotEmpty) return 'message:$explicit';

    final canonical = _canonicalize(payload);
    final encoded = jsonEncode(canonical);
    return 'payload:${_fnv1a32(encoded)}';
  }

  static String _firstNonEmpty(Iterable<Object?> values) {
    for (final value in values) {
      final normalized = value?.toString().trim();
      if (normalized != null && normalized.isNotEmpty) return normalized;
    }
    return '';
  }

  static Object? _canonicalize(Object? value) {
    if (value is Map) {
      final entries = value.entries.toList()
        ..sort((a, b) => a.key.toString().compareTo(b.key.toString()));
      return <String, Object?>{
        for (final entry in entries)
          entry.key.toString(): _canonicalize(entry.value),
      };
    }
    if (value is Iterable) {
      return value.map<Object?>(_canonicalize).toList(growable: false);
    }
    if (value == null || value is num || value is bool || value is String) {
      return value;
    }
    return value.toString();
  }

  static String _fnv1a32(String value) {
    const offsetBasis = 0x811c9dc5;
    const prime = 0x01000193;
    const mask = 0xffffffff;
    var hash = offsetBasis;
    for (final byte in utf8.encode(value)) {
      hash ^= byte;
      hash = (hash * prime) & mask;
    }
    return hash.toRadixString(16).padLeft(8, '0');
  }
}

enum NotificationEnqueueResult { delivered, queued, duplicate, ignored }

class _QueuedNotificationAction {
  const _QueuedNotificationAction({
    required this.identity,
    required this.action,
  });

  final String identity;
  final NotificationAction action;
}

/// In-memory, bounded delivery gate for notification navigation.
///
/// The coordinator is intentionally free of Flutter/Firebase dependencies so
/// cold-start ordering, dedupe, and malformed payload behavior are hermetic.
class NotificationNavigationCoordinator {
  NotificationNavigationCoordinator({
    required bool Function() isReady,
    required bool Function(NotificationAction action) execute,
    this.maxPending = 20,
    this.maxDeliveredIdentities = 100,
  }) : assert(maxPending > 0),
       assert(maxDeliveredIdentities > 0),
       _isReady = isReady,
       _execute = execute;

  final bool Function() _isReady;
  final bool Function(NotificationAction action) _execute;
  final int maxPending;
  final int maxDeliveredIdentities;

  final Queue<_QueuedNotificationAction> _pending = Queue();
  final Set<String> _pendingIdentities = <String>{};
  final LinkedHashSet<String> _deliveredIdentities = LinkedHashSet();

  int get pendingCount => _pending.length;
  int get deliveredIdentityCount => _deliveredIdentities.length;

  NotificationEnqueueResult enqueue(
    Map<String, dynamic> payload, {
    String? sourceMessageId,
  }) {
    final action = NotificationDestinationResolver.resolve(payload);
    if (action == null) return NotificationEnqueueResult.ignored;

    final identity = NotificationMessageIdentity.fromPayload(
      payload,
      sourceMessageId: sourceMessageId,
    );
    if (_pendingIdentities.contains(identity) ||
        _deliveredIdentities.contains(identity)) {
      return NotificationEnqueueResult.duplicate;
    }

    final queued = _QueuedNotificationAction(
      identity: identity,
      action: action,
    );
    if (_isReady() && _deliver(queued)) {
      return NotificationEnqueueResult.delivered;
    }

    if (_pending.length == maxPending) {
      final evicted = _pending.removeFirst();
      _pendingIdentities.remove(evicted.identity);
    }
    _pending.addLast(queued);
    _pendingIdentities.add(identity);
    return NotificationEnqueueResult.queued;
  }

  int flush() {
    if (!_isReady() || _pending.isEmpty) return 0;

    var delivered = 0;
    final attemptCount = _pending.length;
    for (var index = 0; index < attemptCount; index++) {
      final queued = _pending.removeFirst();
      _pendingIdentities.remove(queued.identity);
      if (_deliver(queued)) {
        delivered++;
      } else {
        _pending.addLast(queued);
        _pendingIdentities.add(queued.identity);
      }
    }
    return delivered;
  }

  bool _deliver(_QueuedNotificationAction queued) {
    try {
      if (!_execute(queued.action)) return false;
      _rememberDelivered(queued.identity);
      return true;
    } catch (_) {
      return false;
    }
  }

  void _rememberDelivered(String identity) {
    _deliveredIdentities.add(identity);
    while (_deliveredIdentities.length > maxDeliveredIdentities) {
      _deliveredIdentities.remove(_deliveredIdentities.first);
    }
  }

  void reset() {
    _pending.clear();
    _pendingIdentities.clear();
    _deliveredIdentities.clear();
  }
}
