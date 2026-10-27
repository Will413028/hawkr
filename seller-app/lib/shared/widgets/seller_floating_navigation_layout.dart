import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../core/theme/hk_tokens.dart';

/// 賣家五分頁的真正懸浮導覽層。
///
/// [page] 永遠鋪滿整個 viewport；[navigation] 透過 Stack 疊在頁面上方，
/// 不占用 Scaffold 的 bottomNavigationBar 版位，也不改變頁面高度。
///
/// 自動隱藏交互對齊 Web `MobileBottomNav`：
/// - 頁首固定顯示。
/// - 使用者向下捲動時收起，向上捲動時顯示。
/// - 路由／分頁切換後保留 900ms settle window，避免程式性捲動造成抖動。
/// - 隱藏時可從螢幕底部向上滑動喚回。
/// - 系統啟用 reduced motion 時停用自動隱藏。
///
/// 需要停在導覽上方的固定操作元件，應個別使用
/// [sellerFloatingNavigationClearance]；捲動內容則在自己的 scroll padding
/// 保留同等空間，背景會自然延伸到導覽下方。
class SellerFloatingNavigationLayout extends StatefulWidget {
  const SellerFloatingNavigationLayout({
    super.key,
    required this.page,
    required this.navigation,
    this.visibilityResetKey,
  });

  final Widget page;
  final Widget navigation;

  /// 路由或主分頁識別；變更時強制顯示並重啟 settle window。
  final Object? visibilityResetKey;

  @override
  State<SellerFloatingNavigationLayout> createState() =>
      _SellerFloatingNavigationLayoutState();
}

class _SellerFloatingNavigationLayoutState
    extends State<SellerFloatingNavigationLayout> {
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
  void didUpdateWidget(covariant SellerFloatingNavigationLayout oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.visibilityResetKey == widget.visibilityResetKey) return;
    _showNavigation(restartMinimumDuration: true);
    _restartRouteSettleWindow();
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
    if (_hidden || _reduceMotion || !_canHide || _settlingRoute) return;
    setState(() => _hidden = true);
    _directionTravel = 0;
  }

  bool _handleScrollNotification(ScrollNotification notification) {
    if (notification.metrics.axis != Axis.vertical) return false;

    if (_reduceMotion) {
      _showNavigation(restartMinimumDuration: false);
      return false;
    }

    if (notification.metrics.pixels <= _topThreshold) {
      _showNavigation();
      return false;
    }

    if (notification is ScrollStartNotification) {
      _userScrollActive = notification.dragDetails != null;
    } else if (notification is ScrollUpdateNotification) {
      // 程式性 jumpTo／animateTo 沒有 dragDetails；忽略它們。手指放開後的
      // 慣性捲動仍屬於同一個使用者手勢，持續追蹤到 ScrollEnd。
      if (notification.dragDetails != null) _userScrollActive = true;
      if (!_userScrollActive) return false;
      _consumeUserScrollDelta(notification.scrollDelta ?? 0);
    } else if (notification is OverscrollNotification &&
        notification.dragDetails != null) {
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
    if (!_hidden) return;
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
            key: const ValueKey('seller-floating-navigation-stack'),
            fit: StackFit.expand,
            children: [
              Positioned.fill(
                child: NotificationListener<ScrollNotification>(
                  onNotification: _handleScrollNotification,
                  child: SizedBox.expand(
                    key: const ValueKey('seller-full-page-layer'),
                    child: widget.page,
                  ),
                ),
              ),
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                child: IgnorePointer(
                  ignoring: _hidden,
                  child: ExcludeSemantics(
                    excluding: _hidden,
                    child: AnimatedSlide(
                      key: const ValueKey('seller-bottom-nav-auto-hide-slide'),
                      duration: transitionDuration,
                      curve: HkMotion.ease,
                      offset: _hidden
                          ? const Offset(0, _hiddenSlideOffset)
                          : Offset.zero,
                      child: AnimatedOpacity(
                        key: const ValueKey(
                          'seller-bottom-nav-auto-hide-opacity',
                        ),
                        duration: transitionDuration,
                        curve: HkMotion.ease,
                        opacity: _hidden ? 0 : 1,
                        child: RepaintBoundary(
                          key: const ValueKey(
                            'seller-floating-navigation-layer',
                          ),
                          child: widget.navigation,
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

/// 固定在主分頁底部的操作元件與螢幕底邊之間應保留的距離。
///
/// 包含系統 home indicator、浮動導覽本體，以及導覽上方的呼吸間距。
double sellerFloatingNavigationClearance(BuildContext context) {
  final safeBottom = math.max(
    MediaQuery.viewPaddingOf(context).bottom,
    HkSpace.md,
  );
  return HkControl.navHeight + HkSpace.md + safeBottom;
}
