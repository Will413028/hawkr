import 'dart:async';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/utils/require_login.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/tab_switch_provider.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../category/presentation/category_screen.dart';
import '../../order/presentation/order_list_controller.dart';
import '../../order/presentation/order_list_screen.dart';
import '../../settings/presentation/settings_screen.dart';
import '../../wallet/presentation/wallet_screen.dart';
import 'home_screen.dart';

class MainNavigationScreen extends ConsumerStatefulWidget {
  const MainNavigationScreen({
    super.key,
    this.initialIndex,
    this.marketKeyword = '',
    this.marketSort = '',
    this.walletTab = 0,
    this.orderTab = 0,
    this.orderPendingAction = false,
  });
  final int? initialIndex;
  final String marketKeyword;
  final String marketSort;
  final int walletTab;
  final int orderTab;
  final bool orderPendingAction;

  @override
  ConsumerState<MainNavigationScreen> createState() =>
      _MainNavigationScreenState();
}

class _MainNavigationScreenState extends ConsumerState<MainNavigationScreen> {
  static const int _pagesCount = 5;

  late int _currentIndex;
  late final PageController _pageController;
  final Map<int, PreferredSizeWidget> _topBarCache = {};

  final _homeKey = GlobalKey();
  final _categoryKey = GlobalKey();
  final _walletKey = GlobalKey();
  final _orderKey = GlobalKey();
  final _settingsKey = GlobalKey();

  late final List<Widget> _pages;

  Future<void> _switchToTab(int mainTab, {int walletTab = 0}) async {
    if (mainTab == 2) {
      _pages[2] = WalletScreen(
        key: ValueKey('wallet-$walletTab'),
        showBottomNav: false,
        showTopBar: false,
        initialTab: walletTab,
      );
    }
    await _requestMainTab(mainTab);
  }

  @override
  void initState() {
    super.initState();
    _currentIndex =
        widget.initialIndex?.clamp(0, _pagesCount - 1) ??
        ref.read(currentTabProvider).clamp(0, _pagesCount - 1);
    _pageController = PageController(initialPage: _currentIndex);
    _pages = [
      HomeScreen(key: _homeKey, onSwitchTab: _switchToTab),
      CategoryScreen(
        key: _categoryKey,
        initialKeyword: widget.marketKeyword,
        initialSort: widget.marketSort,
      ),
      WalletScreen(
        key: _walletKey,
        showBottomNav: false,
        showTopBar: false,
        initialTab: widget.walletTab,
      ),
      OrderListScreen(
        key: _orderKey,
        initialTabIndex: widget.orderTab,
        initialPendingAction: widget.orderPendingAction,
      ),
      SettingsScreen(
        key: _settingsKey,
        showBottomNav: false,
        showTopBar: false,
      ),
    ];
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      setState(() {});
    });
  }

  @override
  void didUpdateWidget(covariant MainNavigationScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    final requestedTab = widget.initialIndex?.clamp(0, _pagesCount - 1);
    final oldRequestedTab = oldWidget.initialIndex?.clamp(0, _pagesCount - 1);
    final explicitTabChange =
        requestedTab != null && requestedTab != oldRequestedTab;
    final walletTabChanged = widget.walletTab != oldWidget.walletTab;
    final marketIntentChanged =
        widget.marketKeyword != oldWidget.marketKeyword ||
        widget.marketSort != oldWidget.marketSort;
    if (explicitTabChange) {
      _currentIndex = requestedTab;
      Future(() {
        ref.read(currentTabProvider.notifier).state = _currentIndex;
      });
    }
    if (walletTabChanged) {
      _pages[2] = WalletScreen(
        key: ValueKey('wallet-${widget.walletTab}'),
        showBottomNav: false,
        showTopBar: false,
        initialTab: widget.walletTab,
      );
    }
    if (marketIntentChanged) {
      _pages[1] = CategoryScreen(
        key: _categoryKey,
        initialKeyword: widget.marketKeyword,
        initialSort: widget.marketSort,
      );
    }
    if (explicitTabChange || walletTabChanged || marketIntentChanged) {
      setState(() {});
      if (explicitTabChange) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted || !_pageController.hasClients) return;
          _pageController.jumpToPage(_currentIndex);
        });
      }
    }
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  Future<bool> _requestMainTab(int index) async {
    const protectedTabs = {2, 3, 4};
    if (protectedTabs.contains(index)) {
      if (!await requireLogin(context, ref)) return false;
      if (!mounted) return false;
    }
    if (index == 3) {
      ref.read(orderListControllerProvider.notifier).refresh();
    }

    final routeUri = GoRouterState.of(context).uri;
    final routeTab = int.tryParse(routeUri.queryParameters['tab'] ?? '');
    if (routeUri.path == '/' && routeTab != null && routeTab != index) {
      context.go('/?tab=$index');
      return true;
    }

    if (!_pageController.hasClients) {
      setState(() => _currentIndex = index);
      ref.read(currentTabProvider.notifier).state = index;
      return true;
    }
    _pageController.jumpToPage(index);
    return true;
  }

  void _handlePageChanged(int index) {
    if (_currentIndex == index) return;
    setState(() => _currentIndex = index);
    ref.read(currentTabProvider.notifier).state = index;

    final uri = GoRouterState.of(context).uri;
    if (uri.path == '/' && uri.queryParameters['tab'] != '$index') {
      context.go('/?tab=$index');
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<int>(tabSwitchRequestProvider, (prev, next) {
      if (next >= 0 && next < _pages.length) {
        unawaited(_requestMainTab(next));
        ref.read(tabSwitchRequestProvider.notifier).state = -1;
      }
    });

    return Scaffold(
      appBar: _buildSharedTopBar(context),
      body: PageView(
        controller: _pageController,
        physics: const NeverScrollableScrollPhysics(),
        allowImplicitScrolling: true,
        onPageChanged: _handlePageChanged,
        children: _pages
            .map((page) => _KeepAliveMainTab(child: page))
            .toList(growable: false),
      ),
    );
  }

  PreferredSizeWidget _buildSharedTopBar(BuildContext context) {
    final List<State?> states = [
      _homeKey.currentState,
      _categoryKey.currentState,
      _walletKey.currentState,
      _orderKey.currentState,
      _settingsKey.currentState,
    ];
    for (int i = 0; i < states.length; i++) {
      final s = states[i];
      if (s is TopBarProvider) {
        _topBarCache[i] = (s as TopBarProvider).buildTopBar(context);
      }
    }
    return _topBarCache[_currentIndex] ??
        buildCommonAppBar(context, child: const SizedBox.shrink());
  }
}

class _KeepAliveMainTab extends StatefulWidget {
  const _KeepAliveMainTab({required this.child});

  final Widget child;

  @override
  State<_KeepAliveMainTab> createState() => _KeepAliveMainTabState();
}

class _KeepAliveMainTabState extends State<_KeepAliveMainTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;

  @override
  Widget build(BuildContext context) {
    super.build(context);
    return widget.child;
  }
}
