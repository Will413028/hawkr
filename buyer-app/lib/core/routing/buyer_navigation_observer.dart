import 'package:flutter/material.dart';

/// Tracks modal routes that sit above GoRouter content.
///
/// The global buyer Navbar lives outside the Router's Navigator so it does not
/// participate in page transitions. Consequently, URI-only visibility cannot
/// detect `showDialog` or `showModalBottomSheet`. This observer exposes that
/// overlay state while keeping the single Navbar element mounted offstage.
class BuyerNavigationObserver extends NavigatorObserver with ChangeNotifier {
  final Set<Route<dynamic>> _blockingRoutes = <Route<dynamic>>{};
  bool _notificationScheduled = false;

  bool get hasBlockingRoute => _blockingRoutes.isNotEmpty;

  bool _blocksGlobalNavigation(Route<dynamic>? route) => route is PopupRoute;

  void _scheduleNavigationNotification() {
    if (_notificationScheduled) return;
    _notificationScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _notificationScheduled = false;
      notifyListeners();
    });
  }

  void _add(Route<dynamic>? route) {
    if (!_blocksGlobalNavigation(route)) return;
    _blockingRoutes.add(route!);
  }

  void _remove(Route<dynamic>? route) {
    if (route == null) return;
    _blockingRoutes.remove(route);
  }

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    super.didPush(route, previousRoute);
    _add(route);
    _scheduleNavigationNotification();
  }

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    _remove(route);
    super.didPop(route, previousRoute);
    _scheduleNavigationNotification();
  }

  @override
  void didRemove(Route<dynamic> route, Route<dynamic>? previousRoute) {
    _remove(route);
    super.didRemove(route, previousRoute);
    _scheduleNavigationNotification();
  }

  @override
  void didReplace({Route<dynamic>? newRoute, Route<dynamic>? oldRoute}) {
    _remove(oldRoute);
    _add(newRoute);
    super.didReplace(newRoute: newRoute, oldRoute: oldRoute);
    _scheduleNavigationNotification();
  }
}

final buyerNavigationObserver = BuyerNavigationObserver();
