import 'package:hawkr_buyer/core/routing/app_router.dart';
import 'package:hawkr_buyer/features/notification/data/push/destination.dart';
import 'package:hawkr_buyer/features/notification/data/push/router.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  testWidgets(
    'cold-start notification waits for session and root Navigator readiness',
    (tester) async {
      var sessionReady = false;
      NotificationRouter.debugReset();
      NotificationRouter.configureSessionReadiness(() => sessionReady);
      addTearDown(NotificationRouter.debugReset);

      expect(
        NotificationRouter.navigate({
          notificationDeepLinkKey: '/orders/detail?orderSn=ORD-COLD',
        }, messageId: 'cold-message-1'),
        NotificationEnqueueResult.queued,
      );
      expect(NotificationRouter.debugPendingCount, 1);

      final router = GoRouter(
        navigatorKey: rootNavigatorKey,
        initialLocation: '/',
        routes: [
          GoRoute(
            path: '/',
            builder: (_, __) => const Scaffold(body: Text('home')),
          ),
          GoRoute(
            path: '/orders/detail',
            builder: (_, state) => Scaffold(
              body: Text('order:${state.uri.queryParameters['orderSn']}'),
            ),
          ),
        ],
      );
      addTearDown(router.dispose);

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pump();
      expect(find.text('home'), findsOneWidget);
      expect(find.text('order:ORD-COLD'), findsNothing);

      sessionReady = true;
      expect(NotificationRouter.flushPending(), 1);
      await tester.pumpAndSettle();
      expect(find.text('order:ORD-COLD'), findsOneWidget);

      expect(
        NotificationRouter.navigate({
          notificationDeepLinkKey: '/orders/detail?orderSn=ORD-COLD',
        }, messageId: 'cold-message-1'),
        NotificationEnqueueResult.duplicate,
      );
      await tester.pump();
      expect(find.text('order:ORD-COLD'), findsOneWidget);

      await tester.pumpWidget(const SizedBox.shrink());
    },
  );
}
