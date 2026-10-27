import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../core/theme/hk_tokens.dart';

/// 買家主導覽的全螢幕懸浮層。
///
/// 頁面永遠鋪滿 viewport，Navbar 疊在內容上方；捲動內容只透過 MediaQuery
/// 取得固定的底部避讓空間，因此 Navbar 顯示／隱藏時不會改變 viewport 高度，
/// 也不會造成列表跳動。
///
/// 自動隱藏交互對齊 Web `MobileBottomNav`：
/// - 位於頁首時固定顯示。
/// - 使用者向下捲動時收起，向上捲動時顯示。
/// - 路由／分頁切換後保留 900ms settle window；真實手勢會立即接管，
///   程式性捲動仍不會造成抖動。
/// - 隱藏時可從螢幕底部向上滑動喚回。
/// - 系統啟用 reduced motion 時停用自動隱藏。
class BuyerFloatingNavigationLayout extends StatefulWidget {
  const BuyerFloatingNavigationLayout({
    super.key,
    required this.page,
    required this.navigation,
    required this.navigationVisible,
    required this.visibilityResetKey,
    this.reserveNavigationClearance,
  });

  final Widget page;
  final Widget navigation;

  /// 目前路由是否允許顯示全域 Navbar。
  ///
  /// 即使為 false，[navigation] 仍保持 mounted，只透過 Offstage 隱藏，確保
  /// 路由與 dialog 切換時不會建立第二個 Navbar 或遺失拖曳狀態。
  final bool navigationVisible;

  /// 是否在頁面內容底部保留 Navbar 避讓空間。
  ///
  /// Popup 暫時遮住 Navbar 時仍可維持 true，避免背景頁面因 dialog 開關改變
  /// 可用高度；未指定時跟隨 [navigationVisible]。
  final bool? reserveNavigationClearance;

  /// 路由或主分頁識別；變更時強制顯示並重啟 settle window。
  final Object visibilityResetKey;

  @override
  State<BuyerFloatingNavigationLayout> createState() =>
      _BuyerFloatingNavigationLayoutState();
}

class _BuyerFloatingNavigationLayoutState
    extends State<BuyerFloatingNavigationLayout> {
  static const double _topThreshold = 56;
  static const double _directionDelta = 4;
  static const double _bottomRevealBand = 72;
  static const double _bottomRevealDistance = 24;
  static const double _hiddenSlideOffset = 1.35;
  static const Duration _minimumVisibleDuration = Duration(milliseconds: 160);
  static const Duration _routeSettleDuration = Duration(milliseconds: 900);
  static const Duration _transitionDuration = Duration(milliseconds: 240);

  bool _hidden = false;
  bool _canHide = false;
  bool _settlingRoute = true;
  bool _reduceMotion = false;
  bool _userScrollActive = false;
  double _directionTravel = 0;
  int? _bottomEdgePointer;
  double _bottomEdgeStartY = 0;
  Timer? _minimumVisibleTimer;
  Timer? _routeSettleTimer;

  @override
  void initState() {
    super.initState();
    _restartVisibilityGuards();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    if (_reduceMotion == reduceMotion) return;
    _reduceMotion = reduceMotion;
    if (_reduceMotion) {
      _hidden = false;
      _directionTravel = 0;
    }
  }

  @override
  void didUpdateWidget(covariant BuyerFloatingNavigationLayout oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.visibilityResetKey != widget.visibilityResetKey ||
        (!oldWidget.navigationVisible && widget.navigationVisible)) {
      _showNavigation(restartMinimumDuration: true);
      _restartRouteSettleWindow();
    } else if (!widget.navigationVisible) {
      _hidden = false;
      _directionTravel = 0;
    }
  }

  void _restartVisibilityGuards() {
    _restartMinimumVisibleWindow();
    _restartRouteSettleWindow();
  }

  void _restartMinimumVisibleWindow() {
    _minimumVisibleTimer?.cancel();
    _canHide = false;
    _minimumVisibleTimer = Timer(_minimumVisibleDuration, () {
      if (!mounted) return;
      _canHide = true;
    });
  }

  void _restartRouteSettleWindow() {
    _routeSettleTimer?.cancel();
    _settlingRoute = true;
    _userScrollActive = false;
    _routeSettleTimer = Timer(_routeSettleDuration, () {
      if (!mounted) return;
      _settlingRoute = false;
    });
  }

  void _showNavigation({bool restartMinimumDuration = true}) {
    if (_hidden && mounted) setState(() => _hidden = false);
    _directionTravel = 0;
    if (restartMinimumDuration) _restartMinimumVisibleWindow();
  }

  void _hideNavigation() {
    if (_hidden ||
        _reduceMotion ||
        !widget.navigationVisible ||
        !_canHide ||
        _settlingRoute) {
      return;
    }
    setState(() => _hidden = true);
    _directionTravel = 0;
  }

  void _beginUserScroll() {
    _userScrollActive = true;
    if (!_settlingRoute) return;
    _routeSettleTimer?.cancel();
    _settlingRoute = false;
  }

  bool _handleScrollNotification(ScrollNotification notification) {
    if (!widget.navigationVisible ||
        notification.metrics.axis != Axis.vertical) {
      return false;
    }

    if (_reduceMotion) {
      _showNavigation(restartMinimumDuration: false);
      return false;
    }

    // A fast real-device flick can leave the top threshold during ballistic
    // scrolling, where ScrollUpdateNotification.dragDetails is null. Capture
    // ownership at ScrollStart so that continuation still moves the Navbar.
    if (notification is ScrollStartNotification) {
      if (notification.dragDetails != null) {
        _beginUserScroll();
      } else {
        _userScrollActive = false;
      }
    }

    if (notification.metrics.pixels <= _topThreshold) {
      _showNavigation(restartMinimumDuration: false);
      if (notification is ScrollEndNotification) {
        _userScrollActive = false;
        _directionTravel = 0;
      }
      return false;
    }

    if (notification is ScrollUpdateNotification) {
      // 程式性 jumpTo／animateTo 不具有 dragDetails；忽略它們，避免頁面還原
      // 捲動位置時把 Navbar 誤判成使用者下滑。手指放開後的慣性捲動仍屬於
      // 同一個使用者手勢，持續追蹤到 ScrollEnd。
      if (notification.dragDetails != null) _beginUserScroll();
      if (!_userScrollActive) return false;
      _consumeUserScrollDelta(notification.scrollDelta ?? 0);
    } else if (notification is OverscrollNotification &&
        notification.dragDetails != null) {
      _beginUserScroll();
      _consumeUserScrollDelta(notification.overscroll);
    } else if (notification is ScrollEndNotification) {
      _userScrollActive = false;
      _directionTravel = 0;
    }
    return false;
  }

  void _consumeUserScrollDelta(double delta) {
    if (delta == 0) return;
    if (_directionTravel == 0 || _directionTravel.sign == delta.sign) {
      _directionTravel += delta;
    } else {
      _directionTravel = delta;
    }

    if (_directionTravel > _directionDelta) {
      _hideNavigation();
    } else if (_directionTravel < -_directionDelta) {
      _showNavigation();
    }
  }

  void _handlePointerDown(PointerDownEvent event, double viewportHeight) {
    if (!_hidden || !widget.navigationVisible) return;
    if (event.localPosition.dy < viewportHeight - _bottomRevealBand) return;
    _bottomEdgePointer = event.pointer;
    _bottomEdgeStartY = event.localPosition.dy;
  }

  void _handlePointerMove(PointerMoveEvent event) {
    if (_bottomEdgePointer != event.pointer) return;
    if (_bottomEdgeStartY - event.localPosition.dy < _bottomRevealDistance) {
      return;
    }
    _bottomEdgePointer = null;
    _showNavigation();
  }

  void _clearBottomEdgePointer(PointerEvent event) {
    if (_bottomEdgePointer == event.pointer) _bottomEdgePointer = null;
  }

  @override
  void dispose() {
    _minimumVisibleTimer?.cancel();
    _routeSettleTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final mediaQuery = MediaQuery.of(context);
    final reservesNavigation =
        widget.reserveNavigationClearance ?? widget.navigationVisible;
    final pageBottomPadding = reservesNavigation
        ? buyerFloatingNavigationClearance(context)
        : mediaQuery.padding.bottom;
    final pageMediaQuery = mediaQuery.copyWith(
      padding: mediaQuery.padding.copyWith(
        bottom: math.max(mediaQuery.padding.bottom, pageBottomPadding),
      ),
    );
    final transitionDuration = _reduceMotion
        ? Duration.zero
        : _transitionDuration;

    return LayoutBuilder(
      builder: (context, constraints) {
        return Listener(
          behavior: HitTestBehavior.translucent,
          onPointerDown: (event) =>
              _handlePointerDown(event, constraints.maxHeight),
          onPointerMove: _handlePointerMove,
          onPointerUp: _clearBottomEdgePointer,
          onPointerCancel: _clearBottomEdgePointer,
          child: Stack(
            key: const ValueKey('buyer-floating-navigation-stack'),
            fit: StackFit.expand,
            children: [
              Positioned.fill(
                child: NotificationListener<ScrollNotification>(
                  onNotification: _handleScrollNotification,
                  child: MediaQuery(
                    data: pageMediaQuery,
                    child: SizedBox.expand(
                      key: const ValueKey('buyer-full-page-layer'),
                      child: widget.page,
                    ),
                  ),
                ),
              ),
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                child: Offstage(
                  offstage: !widget.navigationVisible,
                  child: IgnorePointer(
                    ignoring: _hidden,
                    child: ExcludeSemantics(
                      excluding: _hidden,
                      child: AnimatedSlide(
                        key: const ValueKey('buyer-bottom-nav-auto-hide-slide'),
                        duration: transitionDuration,
                        curve: HkMotion.ease,
                        offset: _hidden
                            ? const Offset(0, _hiddenSlideOffset)
                            : Offset.zero,
                        child: AnimatedOpacity(
                          key: const ValueKey(
                            'buyer-bottom-nav-auto-hide-opacity',
                          ),
                          duration: transitionDuration,
                          curve: HkMotion.ease,
                          opacity: _hidden ? 0 : 1,
                          child: RepaintBoundary(
                            key: const ValueKey(
                              'buyer-floating-navigation-layer',
                            ),
                            child: widget.navigation,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// 捲動內容需保留的固定底部空間。
///
/// 包含系統 home indicator、Navbar 本體與其上方 12pt 呼吸距離；Navbar
/// 自動隱藏時仍維持相同 inset，避免列表高度與捲動位置跳動。
double buyerFloatingNavigationClearance(BuildContext context) {
  final safeBottom = math.max(
    MediaQuery.viewPaddingOf(context).bottom,
    HkSpace.md,
  );
  return HkControl.navHeight + HkSpace.md + safeBottom;
}
