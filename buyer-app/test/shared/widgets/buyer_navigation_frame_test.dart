import 'package:hawkr_buyer/core/routing/buyer_navigation_observer.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/shared/widgets/buyer_navigation_frame.dart';
import 'package:hawkr_buyer/shared/widgets/shared_bottom_nav.dart';
import 'package:hawkr_buyer/shared/widgets/tab_switch_provider.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('global Navbar keeps one element and preserves old visibility', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final navigatorKey = GlobalKey<NavigatorState>();
    final navigationObserver = BuyerNavigationObserver();
    final container = ProviderContainer(
      overrides: [sessionProvider.overrideWith(_ActiveSession.new)],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);
    container.read(currentTabProvider.notifier).state = 4;
    final router = GoRouter(
      navigatorKey: navigatorKey,
      initialLocation: '/marketplace',
      observers: [navigationObserver],
      routes: [
        GoRoute(
          path: '/',
          pageBuilder: (_, state) => NoTransitionPage<void>(
            key: state.pageKey,
            child: ColoredBox(
              color: Colors.white,
              child: Center(
                child: Text(
                  state.uri.queryParameters['tab'] == '3' ? '訂單分頁' : '首頁內容',
                ),
              ),
            ),
          ),
        ),
        GoRoute(
          path: '/social/posts',
          pageBuilder: (_, state) => NoTransitionPage<void>(
            key: state.pageKey,
            child: const ColoredBox(
              color: Colors.white,
              child: Center(child: Text('貼文預覽')),
            ),
          ),
        ),
        GoRoute(
          path: '/marketplace',
          builder: (_, __) => const ColoredBox(
            color: Colors.white,
            child: Center(child: Text('Hawkr 市集')),
          ),
        ),
        GoRoute(
          path: '/settings/help',
          builder: (_, __) => const ColoredBox(
            color: Colors.white,
            child: Center(child: Text('幫助中心')),
          ),
        ),
        GoRoute(
          path: '/settings/notifications',
          builder: (_, __) => const ColoredBox(
            color: Colors.white,
            child: Center(child: Text('通知設定')),
          ),
        ),
        GoRoute(
          path: '/notifications',
          builder: (_, __) => const ColoredBox(
            color: Colors.white,
            child: Center(child: Text('通知中心')),
          ),
        ),
        GoRoute(
          path: '/orders/detail',
          builder: (_, __) => const ColoredBox(
            color: Colors.white,
            child: Center(child: Text('訂單細節')),
          ),
        ),
        GoRoute(
          path: '/login',
          builder: (_, __) => const ColoredBox(
            color: Colors.white,
            child: Center(child: Text('登入')),
          ),
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: router,
          builder: (context, child) => BuyerNavigationFrame(
            router: router,
            navigatorKey: navigatorKey,
            navigationObserver: navigationObserver,
            child: child!,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Hawkr 市集'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsOneWidget);
    expect(_selectedTab('市集'), findsOneWidget);
    expect(_selectedTab('設定'), findsNothing);
    expect(container.read(currentTabProvider), 1);
    final originalNavbar = tester.element(
      find.byKey(const ValueKey('buyer-global-bottom-nav')),
    );

    router.push('/settings/help');
    await tester.pumpAndSettle();

    expect(find.text('幫助中心'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('buyer-bottom-nav-item-3')));
    await tester.pumpAndSettle();

    expect(find.text('訂單分頁'), findsOneWidget);
    expect(find.text('幫助中心'), findsNothing);

    router.push('/settings/notifications');
    await tester.pumpAndSettle();

    expect(find.text('通知設定'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsOneWidget);
    expect(router.routerDelegate.state.uri.path, '/settings/notifications');

    await tester.tap(find.byKey(const ValueKey('buyer-bottom-nav-item-3')));
    await tester.pumpAndSettle();

    expect(router.routerDelegate.state.uri.path, '/');
    expect(router.routerDelegate.state.uri.queryParameters['tab'], '3');
    expect(find.text('訂單分頁'), findsOneWidget);
    expect(find.text('通知設定'), findsNothing);

    router.go('/');
    await tester.pumpAndSettle();
    router.push('/notifications');
    await tester.pumpAndSettle();
    expect(find.text('通知中心'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsNothing);

    router.push('/orders/detail');
    await tester.pumpAndSettle();
    expect(find.text('訂單細節'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsNothing);

    router.go('/social/posts');
    await tester.pumpAndSettle();

    expect(find.text('貼文預覽'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsNothing);
    expect(
      identical(
        originalNavbar,
        tester.element(
          find.byKey(
            const ValueKey('buyer-global-bottom-nav'),
            skipOffstage: false,
          ),
        ),
      ),
      isTrue,
      reason: '原本無 Navbar 的頁面只隱藏，全域元件不重建',
    );

    router.go('/');
    await tester.pumpAndSettle();

    expect(find.text('首頁內容'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsOneWidget);
    expect(
      identical(
        originalNavbar,
        tester.element(find.byKey(const ValueKey('buyer-global-bottom-nav'))),
      ),
      isTrue,
      reason: '路由切換只更換內容，Navbar Element 不可重建',
    );

    showDialog<void>(
      context: navigatorKey.currentContext!,
      builder: (_) => const AlertDialog(content: Text('原本無 Navbar 的彈窗')),
    );
    await tester.pumpAndSettle();

    expect(find.text('原本無 Navbar 的彈窗'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsNothing);
    expect(
      identical(
        originalNavbar,
        tester.element(
          find.byKey(
            const ValueKey('buyer-global-bottom-nav'),
            skipOffstage: false,
          ),
        ),
      ),
      isTrue,
      reason: '彈窗只隱藏 Navbar，不應建立第二個 Navbar',
    );

    navigatorKey.currentState!.pop();
    await tester.pumpAndSettle();
    expect(find.byType(SharedBottomNav), findsOneWidget);
    expect(
      identical(
        originalNavbar,
        tester.element(find.byKey(const ValueKey('buyer-global-bottom-nav'))),
      ),
      isTrue,
    );

    router.go('/login');
    await tester.pumpAndSettle();
    expect(find.text('登入'), findsOneWidget);
    expect(find.byType(SharedBottomNav), findsNothing);
  });

  testWidgets(
    'root Navbar waits for PageView before committing the selected tab',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      final navigatorKey = GlobalKey<NavigatorState>();
      final navigationObserver = BuyerNavigationObserver();
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final router = GoRouter(
        navigatorKey: navigatorKey,
        initialLocation: '/',
        observers: [navigationObserver],
        routes: [
          GoRoute(
            path: '/',
            builder: (_, __) => const ColoredBox(
              color: Colors.white,
              child: Center(child: Text('目前首頁內容')),
            ),
          ),
        ],
      );
      addTearDown(router.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(
            routerConfig: router,
            builder: (context, child) => BuyerNavigationFrame(
              router: router,
              navigatorKey: navigatorKey,
              navigationObserver: navigationObserver,
              child: child!,
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(container.read(currentTabProvider), 0);
      await tester.tap(find.byKey(const ValueKey('buyer-bottom-nav-item-1')));
      await tester.pump();

      expect(container.read(tabSwitchRequestProvider), 1);
      expect(
        container.read(currentTabProvider),
        0,
        reason: 'PageView must confirm the move before Navbar state commits',
      );

      await tester.pump(const Duration(milliseconds: 1201));
      expect(_selectedTab('首頁'), findsOneWidget);
      expect(_selectedTab('市集'), findsNothing);
    },
  );

  testWidgets('route scroll notifications drive the global auto-hide Navbar', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final navigatorKey = GlobalKey<NavigatorState>();
    final navigationObserver = BuyerNavigationObserver();
    final router = GoRouter(
      navigatorKey: navigatorKey,
      observers: [navigationObserver],
      routes: [
        GoRoute(
          path: '/',
          builder: (_, state) => PageView(
            physics: const NeverScrollableScrollPhysics(),
            children: [
              ListView.builder(
                key: const ValueKey('framed-scrollable'),
                itemExtent: 80,
                itemCount: 40,
                itemBuilder: (_, index) => Text('route item $index'),
              ),
            ],
          ),
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp.router(
          routerConfig: router,
          builder: (context, child) => BuyerNavigationFrame(
            router: router,
            navigatorKey: navigatorKey,
            navigationObserver: navigationObserver,
            child: child!,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.pump(const Duration(milliseconds: 901));

    await tester.timedDrag(
      find.byKey(const ValueKey('framed-scrollable')),
      const Offset(0, -220),
      const Duration(milliseconds: 320),
    );
    await tester.pump();

    final hiddenSlide = tester.widget<AnimatedSlide>(
      find.byKey(const ValueKey('buyer-bottom-nav-auto-hide-slide')),
    );
    expect(hiddenSlide.offset.dy, greaterThan(1));

    router.go('/?tab=1');
    await tester.pump();

    final shownSlide = tester.widget<AnimatedSlide>(
      find.byKey(const ValueKey('buyer-bottom-nav-auto-hide-slide')),
    );
    expect(shownSlide.offset, Offset.zero);
    expect(find.byType(SharedBottomNav), findsOneWidget);
  });
}

Finder _selectedTab(String label) => find.byWidgetPredicate(
  (widget) =>
      widget is Semantics &&
      widget.properties.label == label &&
      widget.properties.selected == true,
);

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer');
}
