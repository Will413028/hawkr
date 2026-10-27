import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/routing/buyer_navigation_observer.dart';
import '../../core/theme/app_colors.dart';
import '../utils/require_login.dart';
import 'buyer_floating_navigation_layout.dart';
import 'shared_bottom_nav.dart';
import 'tab_switch_provider.dart';

/// 全站唯一的買家導覽框架。
///
/// Navbar 位於 GoRouter Navigator 外層，因此頁面 push、pop 或 replace 時只會
/// 切換 [child]，不會建立另一條 Navbar 或把既有 Navbar 跟著頁面轉場滑走。
class BuyerNavigationFrame extends ConsumerWidget {
  const BuyerNavigationFrame({
    super.key,
    required this.router,
    required this.navigatorKey,
    required this.navigationObserver,
    required this.child,
  });

  final GoRouter router;
  final GlobalKey<NavigatorState> navigatorKey;
  final BuyerNavigationObserver navigationObserver;
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final rememberedTab = ref
        .watch(currentTabProvider)
        .clamp(0, SharedBottomNav.labels.length - 1);

    return AnimatedBuilder(
      animation: Listenable.merge([
        router.routeInformationProvider,
        // The observer also notifies after imperative push/pop operations,
        // whose top route is not reliably represented by RouteInformation.
        navigationObserver,
      ]),
      child: child,
      builder: (context, child) {
        final uri = _activeUri;
        final routeTab = _tabForRoute(uri);
        final currentTab = routeTab ?? rememberedTab;

        if (routeTab != null && routeTab != rememberedTab) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (ref.read(currentTabProvider) == routeTab) return;
            ref.read(currentTabProvider.notifier).state = routeTab;
          });
        }

        final routeShowsNavigation = _showsGlobalNavigation(uri);
        final showsNavigation =
            routeShowsNavigation && !navigationObserver.hasBlockingRoute;

        return Scaffold(
          key: const ValueKey('buyer-global-navigation-frame'),
          backgroundColor: AppColors.bgPrimary,
          // 外層導覽 Scaffold 不參與鍵盤縮放，交給各頁自己的 Scaffold 處理一次，
          // 避免與內層頁面 Scaffold 疊加造成鍵盤 inset 被算兩次（輸入框被頂高）。
          resizeToAvoidBottomInset: false,
          body: BuyerFloatingNavigationLayout(
            page: child!,
            navigationVisible: showsNavigation,
            reserveNavigationClearance: routeShowsNavigation,
            visibilityResetKey: '${uri.toString()}#$currentTab',
            navigation: SharedBottomNav(
              key: const ValueKey('buyer-global-bottom-nav'),
              currentIndex: currentTab,
              onDestinationRequested: (index) =>
                  _requestDestination(ref, index: index),
            ),
          ),
        );
      },
    );
  }

  Future<bool> _requestDestination(WidgetRef ref, {required int index}) async {
    const protectedTabs = {2, 3, 4};
    if (protectedTabs.contains(index)) {
      final navigationContext = navigatorKey.currentContext;
      if (navigationContext == null) return false;
      if (!await requireLogin(navigationContext, ref)) return false;
    }

    final uri = _activeUri;

    if (uri.path == '/') {
      // MainNavigationScreen owns the PageView transition. It commits
      // currentTabProvider from onPageChanged only after the content moves, so
      // the Navbar can never get ahead of the visible page.
      ref.read(tabSwitchRequestProvider.notifier).state = index;
    } else {
      // 從貼文、商家或其他子頁回主分頁時，只替換 Navigator 內容；
      // 本 Frame 與 Navbar 元件會原地保留。
      router.go('/?tab=$index');
    }
    return true;
  }

  Uri get _activeUri {
    if (router.routerDelegate.currentConfiguration.isEmpty) {
      return router.routeInformationProvider.value.uri;
    }
    // GoRouter's RouteMatchList.uri intentionally excludes imperative
    // context.push() matches. Delegate.state resolves the top match instead.
    return router.routerDelegate.state.uri;
  }
}

bool _showsGlobalNavigation(Uri uri) {
  const routesThatPreviouslyOwnedNavbar = {
    '/',
    '/marketplace',
    '/search/results',
    '/category-landing',
    '/wallet/coupon-detail',
    '/settings/help',
    '/settings/notifications',
    '/orders/reserve',
    '/orders/payment-details',
    '/orders/tracking',
    '/orders/dispute',
    '/orders/payment',
  };
  return routesThatPreviouslyOwnedNavbar.contains(uri.path);
}

int? _tabForRoute(Uri uri) {
  final path = uri.path;
  if (path == '/') {
    return int.tryParse(
      uri.queryParameters['tab'] ?? '',
    )?.clamp(0, SharedBottomNav.labels.length - 1);
  }
  if (path.startsWith('/search')) return 0;
  if (path == '/marketplace' || path.startsWith('/category')) return 1;
  if (path.startsWith('/wallet') || path.startsWith('/member-cards')) return 2;
  if (path.startsWith('/orders')) return 3;
  if (path.startsWith('/settings')) return 4;
  return null;
}
