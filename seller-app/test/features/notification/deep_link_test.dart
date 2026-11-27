import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_seller/app.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/chat/data/active_chat_gate.dart';
import 'package:hawkr_seller/features/notification/data/deep_link.dart';

/// 伺服器給的深連結是 website 路徑（`deep-links.md`），App 把 STORE 受眾的列對到自己的畫面；
/// 語系前綴 `/en` 先去掉，空字串、外部連結與不認得的路徑落在收件匣。
void main() {
  final extras = <String, Object?>{};

  Future<void> open(WidgetTester tester, String? link) async {
    extras.clear();
    GoRoute route(String path) => GoRoute(
      path: path,
      builder: (context, state) {
        extras[state.uri.toString()] = state.extra;
        return Text('page:${state.uri}');
      },
    );
    final router = GoRouter(
      navigatorKey: MyApp.navigatorKey,
      routes: [
        GoRoute(path: '/', builder: (_, _) => const Text('home')),
        route(MainRoutes.dashboard),
        route(MainRoutes.orders),
        route(MainRoutes.wallet),
        route(MainRoutes.category),
        route(OrderRoutes.matchedDetail),
        route(SettingsRoutes.chatThread),
        route(SettingsRoutes.notifications),
        route(SellerRoutes.suspensionNotice),
        route(SellerRoutes.certificationUpload),
        route(SellerRoutes.measureRequest),
        route(SellerRoutes.inquiryDetail),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    NotificationDeepLink.parse(link).open();
    await tester.pumpAndSettle();
  }

  final storeRows = <String, String>{
    '/seller-center/messages/7': SettingsRoutes.chatThread,
    '/seller-center/orders/O1': OrderRoutes.matchedDetail,
    '/seller-center/wallet': MainRoutes.wallet,
    '/seller-center/suspended': SellerRoutes.suspensionNotice,
    '/seller/onboarding': SellerRoutes.certificationUpload,
    '/seller/measures/M1': '/seller/measures/M1',
    '/seller/measures/M1/appeal': '/seller/measures/M1',
    '/seller/measures/M1/remediation': '/seller/measures/M1?mode=remediation',
    '/seller/inquiries/I1': '/seller/inquiries/I1',
    '/sellers/S1/messages/T1': MainRoutes.category,
    '/seller-center/notifications': SettingsRoutes.notifications,
  };

  for (final prefix in ['', '/en']) {
    for (final row in storeRows.entries) {
      testWidgets('$prefix${row.key} opens ${row.value}', (tester) async {
        await open(tester, '$prefix${row.key}');
        expect(find.text('page:${row.value}'), findsOneWidget);
      });
    }
  }

  testWidgets('order and chat load by the id in the path', (tester) async {
    await open(tester, '/en/seller-center/orders/O1');
    expect(extras[OrderRoutes.matchedDetail], {'orderSn': 'O1'});
    await open(tester, '/seller-center/messages/7');
    expect(extras[SettingsRoutes.chatThread], {'participantId': '7'});
  });

  for (final link in [
    '/account/orders/O1',
    '/sellers/S1/reviews',
    '/seller/measures',
    '/seller-center/orders',
    '/unknown/page',
    '/',
    '/en',
    '',
    null,
  ]) {
    testWidgets('unknown or empty "$link" opens the inbox', (tester) async {
      await open(tester, link);
      expect(find.text('page:${SettingsRoutes.notifications}'), findsOneWidget);
    });
  }

  for (final link in [
    'https://evil.example/seller-center/wallet',
    '//evil.example/seller-center/wallet',
    'javascript:alert(1)',
    'seller-center/wallet',
  ]) {
    testWidgets('external or malformed "$link" is rejected to the inbox', (
      tester,
    ) async {
      await open(tester, link);
      expect(find.text('page:${SettingsRoutes.notifications}'), findsOneWidget);
      expect(find.text('page:${MainRoutes.wallet}'), findsNothing);
    });
  }

  test('only the seller chat path yields the buyer id', () {
    expect(
      NotificationDeepLink.parse('/seller-center/messages/7').chatBuyerId,
      '7',
    );
    expect(
      NotificationDeepLink.parse('/en/seller-center/messages/7').chatBuyerId,
      '7',
    );
    expect(
      NotificationDeepLink.parse('/account/messages/7').chatBuyerId,
      isNull,
    );
    expect(
      NotificationDeepLink.parse('/seller-center/wallet').chatBuyerId,
      isNull,
    );
  });

  test('foreground banner is suppressed only for the open thread buyer', () {
    final link = NotificationDeepLink.parse('/seller-center/messages/X');
    expect(link.isOpenIn(const ActiveChat(peerId: 'X')), isTrue);
    expect(link.isOpenIn(const ActiveChat(peerId: 'Y')), isFalse);
    expect(link.isOpenIn(const ActiveChat(conversationId: 'X')), isFalse);
    expect(link.isOpenIn(const ActiveChat()), isFalse);
    expect(
      NotificationDeepLink.parse(
        '/seller-center/wallet',
      ).isOpenIn(const ActiveChat(peerId: 'X')),
      isFalse,
    );
  });

  test('inbox-bound links are flagged so the inbox does not reopen itself', () {
    expect(NotificationDeepLink.parse('').opensInbox, isTrue);
    expect(NotificationDeepLink.parse('').isEmpty, isTrue);
    expect(NotificationDeepLink.parse('https://x.example/').opensInbox, isTrue);
    expect(NotificationDeepLink.parse('https://x.example/').isEmpty, isFalse);
    expect(
      NotificationDeepLink.parse('/seller-center/wallet').opensInbox,
      isFalse,
    );
  });
}
