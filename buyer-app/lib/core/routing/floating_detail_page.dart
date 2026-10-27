import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';

import '../motion/hk_motion_transitions.dart';
import '../theme/app_colors.dart';

/// Marker exposed to detail content that may otherwise veto system back.
abstract interface class FloatingDetailBackGestureRoute {}

/// 貼文、商品與訂單詳情共用的非不透明 drill-down route。
///
/// 前一頁保持 mounted 並透過設計系統遮罩顯示在後方；詳情畫面本身必須使用
/// 透明 Scaffold，僅繪製前景卡片，避免再次蓋掉原頁。
Page<T> floatingDetailPage<T>({
  required BuildContext context,
  required LocalKey key,
  required Widget child,
  String barrierLabel = '關閉詳情',
}) {
  return _CupertinoFloatingDetailPage<T>(
    key: key,
    child: child,
    barrierColor: AppColors.bgOverlay,
    barrierLabel: barrierLabel,
    reduceMotion: HkMotionTransitions.reduceMotionOf(context),
  );
}

/// A transparent Cupertino drill-down page.
///
/// [CustomTransitionPage] can draw a Cupertino-looking slide, but its route
/// does not install Flutter's interactive leading-edge back gesture. Using the
/// Cupertino route mixin here preserves that native gesture while keeping the
/// previous page visible beneath the floating detail surface.
class _CupertinoFloatingDetailPage<T> extends Page<T> {
  const _CupertinoFloatingDetailPage({
    required super.key,
    required this.child,
    required this.barrierColor,
    required this.barrierLabel,
    required this.reduceMotion,
  });

  final Widget child;
  final Color barrierColor;
  final String barrierLabel;
  final bool reduceMotion;

  @override
  Route<T> createRoute(BuildContext context) {
    return _CupertinoFloatingDetailRoute<T>(page: this);
  }
}

class _CupertinoFloatingDetailRoute<T> extends PageRoute<T>
    with CupertinoRouteTransitionMixin<T>
    implements FloatingDetailBackGestureRoute {
  _CupertinoFloatingDetailRoute({required _CupertinoFloatingDetailPage<T> page})
    : super(settings: page, fullscreenDialog: false);

  _CupertinoFloatingDetailPage<T> get _page =>
      settings as _CupertinoFloatingDetailPage<T>;
  @override
  Widget buildContent(BuildContext context) => _page.child;

  @override
  String? get title => null;

  @override
  bool get maintainState => true;

  @override
  bool get opaque => false;

  @override
  bool get barrierDismissible => false;

  @override
  Color get barrierColor => _page.barrierColor;

  @override
  String get barrierLabel => _page.barrierLabel;

  @override
  Duration get transitionDuration => _page.reduceMotion
      ? Duration.zero
      : CupertinoRouteTransitionMixin.kTransitionDuration;

  @override
  Duration get reverseTransitionDuration => transitionDuration;

  @override
  DelegatedTransitionBuilder? get delegatedTransition =>
      CupertinoPageTransition.delegatedTransition;
}
