import 'package:hawkr_seller/app.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/notification/data/push/router.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// 貼文互動推播開到那一篇：`refId` 帶成 `/post/feed` 的 `postId`，缺值或不是數字時沒有 extra（停在第一篇）。
void main() {
  Future<List<Object?>> pumpAndNavigate(
    WidgetTester tester,
    Map<String, dynamic> data,
  ) async {
    final extras = <Object?>[];
    final router = GoRouter(
      navigatorKey: MyApp.navigatorKey,
      routes: [
        GoRoute(path: '/', builder: (_, _) => const Text('home')),
        GoRoute(
          path: PostRoutes.feed,
          builder: (context, state) {
            extras.add(state.extra);
            return const Text('feed');
          },
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    NotificationRouter.navigate(data);
    await tester.pumpAndSettle();
    expect(find.text('feed'), findsOneWidget);
    return extras;
  }

  for (final type in [
    'POST_LIKE',
    'POST_COMMENT',
    'POST_NEW_COMMENT',
    'POST_COMMENT_REPLY',
    'COMMENT_REPLY',
    'COMMENT_LIKED',
  ]) {
    testWidgets('$type 帶 refId 開到那一篇', (tester) async {
      final extras = await pumpAndNavigate(tester, {
        'type': type,
        'refId': '2690000000000002',
      });
      expect(extras.last, {'postId': 2690000000000002});
    });
  }

  testWidgets('refId 缺值時沒有 extra，停在第一篇', (tester) async {
    final extras = await pumpAndNavigate(tester, {'type': 'POST_LIKE'});
    expect(extras.last, isNull);
  });
}
