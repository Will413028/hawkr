import 'package:hawkr_buyer/features/notification/data/push/destination.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('NotificationDestinationResolver', () {
    test('resolves typed destinations without Navigator state', () {
      final order = NotificationDestinationResolver.resolve({
        'type': 'ORDER_SHIPPED',
        'orderSn': 'ORD-42',
      });
      final chat = NotificationDestinationResolver.resolve({
        'eventType': 'chat_message',
        'conversation_id': 'chat-9',
      });

      expect(order?.destination.kind, NotificationDestinationKind.orderDetail);
      expect(order?.destination.referenceId, 'ORD-42');
      expect(order?.statusRefresh, NotificationStatusRefresh.order);
      expect(chat?.destination.kind, NotificationDestinationKind.chatRoom);
      expect(chat?.destination.referenceId, 'chat-9');
    });

    test('unknown and malformed actionable payloads are ignored', () {
      expect(
        NotificationDestinationResolver.resolve({'type': 'NOT_A_REAL_TYPE'}),
        isNull,
      );
      expect(
        NotificationDestinationResolver.resolve({'type': 'POST_NEW_COMMENT'}),
        isNull,
      );
      expect(
        NotificationDestinationResolver.resolve({
          'type': 'RESTOCK_AVAILABLE',
          'productId': '  ',
        }),
        isNull,
      );
      expect(NotificationDestinationResolver.resolve(const {}), isNull);
    });
  });

  group('NotificationNavigationCoordinator', () {
    test(
      'cold-start payload queues, then delivers once when auth/router are ready',
      () {
        var ready = false;
        final delivered = <NotificationAction>[];
        final coordinator = NotificationNavigationCoordinator(
          isReady: () => ready,
          execute: (action) {
            delivered.add(action);
            return true;
          },
        );

        final fcmPayload = <String, dynamic>{
          'type': 'ORDER_PAID',
          'orderSn': 'ORD-1',
        };
        final localPayload = <String, dynamic>{
          ...fcmPayload,
          notificationSourceMessageIdKey: 'fcm-123',
        };

        expect(
          coordinator.enqueue(fcmPayload, sourceMessageId: 'fcm-123'),
          NotificationEnqueueResult.queued,
        );
        // The same delivery can arrive from onMessageOpenedApp and the local
        // notification callback while the first request is still pending.
        expect(
          coordinator.enqueue(fcmPayload, sourceMessageId: 'fcm-123'),
          NotificationEnqueueResult.duplicate,
        );
        expect(
          coordinator.enqueue(localPayload),
          NotificationEnqueueResult.duplicate,
        );
        expect(delivered, isEmpty);

        ready = true;
        expect(coordinator.flush(), 1);
        expect(delivered, hasLength(1));
        expect(
          delivered.single.destination.kind,
          NotificationDestinationKind.orderDetail,
        );

        expect(
          coordinator.enqueue(localPayload),
          NotificationEnqueueResult.duplicate,
        );
        expect(delivered, hasLength(1));
      },
    );

    test('canonical payload identity is independent of map key order', () {
      final first = NotificationMessageIdentity.fromPayload({
        'type': 'CHAT_MESSAGE',
        'refId': 'conversation-1',
        'nested': {'b': 2, 'a': 1},
      });
      final second = NotificationMessageIdentity.fromPayload({
        'nested': {'a': 1, 'b': 2},
        'refId': 'conversation-1',
        'type': 'CHAT_MESSAGE',
      });

      expect(first, second);
    });

    test('pending queue and delivered identity history are bounded', () {
      var ready = false;
      final delivered = <String?>[];
      final coordinator = NotificationNavigationCoordinator(
        isReady: () => ready,
        execute: (action) {
          delivered.add(action.destination.referenceId);
          return true;
        },
        maxPending: 2,
        maxDeliveredIdentities: 2,
      );

      for (var index = 1; index <= 3; index++) {
        coordinator.enqueue({
          'type': 'ORDER',
          'orderSn': 'order-$index',
          'message_id': 'message-$index',
        });
      }
      expect(coordinator.pendingCount, 2);

      ready = true;
      expect(coordinator.flush(), 2);
      expect(delivered, ['order-2', 'order-3']);
      expect(coordinator.deliveredIdentityCount, 2);

      expect(
        coordinator.enqueue({
          'type': 'ORDER',
          'orderSn': 'order-4',
          'message_id': 'message-4',
        }),
        NotificationEnqueueResult.delivered,
      );
      expect(coordinator.deliveredIdentityCount, 2);
    });
  });
}
