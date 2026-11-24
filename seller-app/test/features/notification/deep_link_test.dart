import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_seller/app.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/notification/data/deep_link.dart';

/// 伺服器給的深連結（路徑加查詢參數）對到現有路由；空字串與對不到的連結不導航。
void main() {
  Future<Map<String, Object?>> pump(WidgetTester tester, String link) async {
    final seen = <String, Object?>{};
    GoRoute route(String path) => GoRoute(
      path: path,
      builder: (context, state) {
        seen[path] = state.extra;
        return Text('page:$path');
      },
    );
    final router = GoRouter(
      navigatorKey: MyApp.navigatorKey,
      routes: [
        GoRoute(path: '/', builder: (_, _) => const Text('home')),
        route(MainRoutes.orders),
        route(MainRoutes.dashboard),
        route(MainRoutes.category),
        route(OrderRoutes.matchedDetail),
        route(ToolRoutes.afterSalesDetail),
        route(SettingsRoutes.chatThread),
        route(PostRoutes.feed),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    NotificationDeepLink.parse(link)?.open();
    await tester.pumpAndSettle();
    return seen;
  }

  testWidgets('訂單詳情帶 orderSn', (tester) async {
    final seen = await pump(tester, '/orders/matched/detail?orderSn=O1');
    expect(seen[OrderRoutes.matchedDetail], {'orderSn': 'O1'});
  });

  testWidgets('售後詳情帶 caseNo', (tester) async {
    final seen = await pump(tester, '/after-sales/detail?caseNo=C9');
    expect(seen[ToolRoutes.afterSalesDetail], 'C9');
  });

  testWidgets('聊天室帶 conversationId', (tester) async {
    final seen = await pump(tester, '/chat/thread?conversationId=7');
    expect(seen[SettingsRoutes.chatThread], {'conversationId': '7'});
  });

  testWidgets('貼文動態帶 postId，缺值時沒有 extra', (tester) async {
    final seen = await pump(tester, '/post/feed?postId=2690000000000002');
    expect(seen[PostRoutes.feed], {'postId': 2690000000000002});
    final none = await pump(tester, '/post/feed');
    expect(none[PostRoutes.feed], isNull);
  });

  testWidgets('缺必要參數、對不到路由或空字串都不導航', (tester) async {
    for (final link in ['/orders/matched/detail', '/unknown/page', '']) {
      final seen = await pump(tester, link);
      expect(seen, isEmpty, reason: link);
      expect(find.text('home'), findsOneWidget, reason: link);
    }
  });

  test('聊天室連結可取出 conversationId，其他連結不行', () {
    expect(
      NotificationDeepLink.parse(
        '/chat/thread?conversationId=7',
      )?.chatConversationId,
      '7',
    );
    expect(
      NotificationDeepLink.parse('/main/orders')?.chatConversationId,
      isNull,
    );
  });
}
