import 'dart:collection';
import 'dart:convert';

import 'deep_link.dart';

/// FCM `data` 裡的深連結鍵。伺服器組好的 website 路徑（`InboxItem.deep_link` 同一個值，可帶 `/en` 前綴），
/// 對應到 App 畫面由 [resolveNotificationLocation] 負責。
const notificationDeepLinkKey = 'deep_link';

/// FCM `data` 裡的收件匣通知 ID 與狀態重讀提示（逗號分隔的 proto enum 名稱）。
const notificationInboxItemIdKey = 'inbox_item_id';
const notificationStateRefreshHintsKey = 'state_refresh_hints';

/// 要交給路由的目的地。
class NotificationAction {
  const NotificationAction({required this.deepLink, required this.payload});

  /// 空字串表示沒有目的地（開收件匣）。
  final String deepLink;
  final Map<String, dynamic> payload;
}

/// 推播 payload 的解析：深連結缺或空就開收件匣，不是 App 內路徑回 null，不會到 Navigator。
class NotificationDestinationResolver {
  const NotificationDestinationResolver._();

  static NotificationAction? resolve(Map<String, dynamic> rawPayload) {
    final payload = Map<String, dynamic>.unmodifiable(rawPayload);
    final link = payload[notificationDeepLinkKey]?.toString().trim() ?? '';
    if (resolveNotificationLocation(link) == null) return null;
    return NotificationAction(deepLink: link, payload: payload);
  }

  static bool isAppDeepLink(String link) =>
      resolveNotificationLocation(link) != null;
}

/// Stable identity shared by the foreground and background-open paths.
class NotificationMessageIdentity {
  const NotificationMessageIdentity._();

  static String fromPayload(
    Map<String, dynamic> payload, {
    String? sourceMessageId,
  }) {
    final explicit = _firstNonEmpty([
      sourceMessageId,
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
