import 'package:hawkr_buyer/features/notification/data/push/destination.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('NotificationDestinationResolver', () {
    test('hands the server deep link to the router without parsing it', () {
      final action = NotificationDestinationResolver.resolve({
        notificationDeepLinkKey: ' /orders/ORD-42 ',
        'type': 'ignored',
      });

      expect(action?.deepLink, '/orders/ORD-42');
    });

    test('missing, empty and non-app deep links are ignored', () {
      expect(NotificationDestinationResolver.resolve(const {}), isNull);
      expect(
        NotificationDestinationResolver.resolve({notificationDeepLinkKey: ''}),
        isNull,
      );
      expect(
        NotificationDestinationResolver.resolve({
          notificationDeepLinkKey: 'https://example.com/x',
        }),
        isNull,
      );
      expect(
        NotificationDestinationResolver.resolve({
          notificationDeepLinkKey: '//example.com/x',
        }),
        isNull,
      );
    });
  });

  group('NotificationNavigationCoordinator', () {
    test('cold-start payload queues, then delivers once when ready', () {
      var ready = false;
      final delivered = <NotificationAction>[];
      final coordinator = NotificationNavigationCoordinator(
        isReady: () => ready,
        execute: (action) {
          delivered.add(action);
          return true;
        },
      );

      final payload = <String, dynamic>{notificationDeepLinkKey: '/orders/1'};

      expect(
        coordinator.enqueue(payload, sourceMessageId: 'fcm-123'),
        NotificationEnqueueResult.queued,
      );
      expect(
        coordinator.enqueue(payload, sourceMessageId: 'fcm-123'),
        NotificationEnqueueResult.duplicate,
      );
      expect(delivered, isEmpty);

      ready = true;
      expect(coordinator.flush(), 1);
      expect(delivered.single.deepLink, '/orders/1');
      expect(
        coordinator.enqueue(payload, sourceMessageId: 'fcm-123'),
        NotificationEnqueueResult.duplicate,
      );
      expect(delivered, hasLength(1));
    });

    test('canonical payload identity is independent of map key order', () {
      final first = NotificationMessageIdentity.fromPayload({
        notificationDeepLinkKey: '/chat/1',
        'nested': {'b': 2, 'a': 1},
      });
      final second = NotificationMessageIdentity.fromPayload({
        'nested': {'a': 1, 'b': 2},
        notificationDeepLinkKey: '/chat/1',
      });

      expect(first, second);
    });

    test('pending queue and delivered identity history are bounded', () {
      var ready = false;
      final delivered = <String>[];
      final coordinator = NotificationNavigationCoordinator(
        isReady: () => ready,
        execute: (action) {
          delivered.add(action.deepLink);
          return true;
        },
        maxPending: 2,
        maxDeliveredIdentities: 2,
      );

      for (var index = 1; index <= 3; index++) {
        coordinator.enqueue({
          notificationDeepLinkKey: '/orders/$index',
          'message_id': 'message-$index',
        });
      }
      expect(coordinator.pendingCount, 2);

      ready = true;
      expect(coordinator.flush(), 2);
      expect(delivered, ['/orders/2', '/orders/3']);
      expect(coordinator.deliveredIdentityCount, 2);

      expect(
        coordinator.enqueue({
          notificationDeepLinkKey: '/orders/4',
          'message_id': 'message-4',
        }),
        NotificationEnqueueResult.delivered,
      );
      expect(coordinator.deliveredIdentityCount, 2);
    });
  });
}
