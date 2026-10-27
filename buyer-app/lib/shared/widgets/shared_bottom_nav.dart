import 'dart:async';
import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/physics.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_icon.dart';
import 'glass_tab_bar.dart';
import 'tab_switch_provider.dart';

/// Web-parity buyer navigation with a magnetic selection pill.
///
/// Taps move the pill optimistically. A pointer movement of 6dp turns the
/// press into a drag: the pill springs to the nearest slot, gives subtle
/// selection feedback, and commits the destination only on release.
class SharedBottomNav extends ConsumerStatefulWidget {
  const SharedBottomNav({
    super.key,
    this.currentIndex = 0,
    this.onDestinationSelected,
    this.onDestinationRequested,
  });

  final int currentIndex;
  final ValueChanged<int>? onDestinationSelected;
  final Future<bool> Function(int index)? onDestinationRequested;

  static const labels = ['首頁', '市集', '錢包', '訂單', '設定'];
  static const icons = [
    HkIconName.home,
    HkIconName.market,
    HkIconName.wallet,
    HkIconName.orders,
    HkIconName.user,
  ];

  @override
  ConsumerState<SharedBottomNav> createState() => _SharedBottomNavState();
}

class _SharedBottomNavState extends ConsumerState<SharedBottomNav>
    with SingleTickerProviderStateMixin {
  static const double _dragThreshold = 6;
  static const SpringDescription _spring = SpringDescription(
    mass: 0.9,
    stiffness: 460,
    damping: 30,
  );

  late final AnimationController _indicatorController =
      AnimationController.unbounded(
        vsync: this,
        value: widget.currentIndex.toDouble(),
      );

  int? _pointer;
  double _pointerStartX = 0;
  double _dragGrabOffsetX = 0;
  double _trackWidth = 1;
  int _hoverIndex = 0;
  int? _pendingIndex;
  bool _dragging = false;
  bool _swallowTap = false;
  Timer? _pendingFallback;
  Timer? _tapGuardTimer;

  bool get _reduceMotion =>
      (MediaQuery.maybeOf(context)?.disableAnimations ?? false) ||
      View.of(context).platformDispatcher.accessibilityFeatures.reduceMotion;

  @override
  void initState() {
    super.initState();
    _hoverIndex = widget.currentIndex;
  }

  @override
  void didUpdateWidget(covariant SharedBottomNav oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.currentIndex == widget.currentIndex) return;
    _pendingFallback?.cancel();
    _pendingIndex = null;
    _hoverIndex = widget.currentIndex;
    _springTo(widget.currentIndex.toDouble());
  }

  void _springTo(double target) {
    if (_reduceMotion) {
      _indicatorController.value = target;
      return;
    }
    if ((_indicatorController.value - target).abs() < 0.001) return;
    _indicatorController.animateWith(
      SpringSimulation(_spring, _indicatorController.value, target, 0),
    );
  }

  double _positionFor(double localX) {
    final itemWidth = _trackWidth / SharedBottomNav.labels.length;
    final indicatorCenter = localX - _dragGrabOffsetX;
    return (indicatorCenter / itemWidth - 0.5).clamp(
      0.0,
      (SharedBottomNav.labels.length - 1).toDouble(),
    );
  }

  void _handlePointerDown(PointerDownEvent event) {
    if (_pointer != null) return;
    _pointer = event.pointer;
    _pointerStartX = event.localPosition.dx;
    _hoverIndex = _pendingIndex ?? widget.currentIndex;
    final itemWidth = _trackWidth / SharedBottomNav.labels.length;
    final indicatorCenter = (_indicatorController.value + 0.5) * itemWidth;
    _dragGrabOffsetX = event.localPosition.dx - indicatorCenter;
  }

  void _handlePointerMove(PointerMoveEvent event) {
    if (_pointer != event.pointer) return;
    if (!_dragging &&
        (event.localPosition.dx - _pointerStartX).abs() < _dragThreshold) {
      return;
    }
    if (!_dragging) {
      _indicatorController.stop();
      setState(() => _dragging = true);
    }
    final position = _positionFor(event.localPosition.dx);
    _indicatorController.value = position;
    final next = position.round().clamp(0, SharedBottomNav.labels.length - 1);
    if (next == _hoverIndex) return;
    if (!_reduceMotion) HapticFeedback.selectionClick();
    setState(() => _hoverIndex = next);
  }

  Future<bool> _requestSelection(int index) async {
    final request = widget.onDestinationRequested;
    if (request != null) return request(index);

    final callback = widget.onDestinationSelected;
    if (callback != null) {
      callback(index);
      return true;
    }
    ref.read(currentTabProvider.notifier).state = index;
    if (!mounted) return false;
    context.go('/');
    return true;
  }

  Future<void> _select(int index) async {
    _pendingFallback?.cancel();
    setState(() => _pendingIndex = index);
    _springTo(index.toDouble());

    final accepted = await _requestSelection(index);
    if (!mounted) return;
    if (!accepted) {
      setState(() => _pendingIndex = null);
      _springTo(widget.currentIndex.toDouble());
      return;
    }

    // Route/PageView resolution normally clears pending in didUpdateWidget.
    // The fallback protects callbacks that accept a request but do not move.
    _pendingFallback = Timer(const Duration(milliseconds: 1200), () {
      if (!mounted || _pendingIndex == null) return;
      setState(() => _pendingIndex = null);
      _springTo(widget.currentIndex.toDouble());
    });
  }

  void _handlePointerUp(PointerUpEvent event) {
    if (_pointer != event.pointer) return;
    _pointer = null;
    if (!_dragging) return;
    _swallowTap = true;
    _tapGuardTimer?.cancel();
    _tapGuardTimer = Timer(Duration.zero, () => _swallowTap = false);
    final target = _hoverIndex;
    setState(() => _dragging = false);
    unawaited(_select(target));
  }

  void _handlePointerCancel(PointerCancelEvent event) {
    if (_pointer != event.pointer) return;
    _pointer = null;
    if (!_dragging) return;
    setState(() => _dragging = false);
    _springTo((_pendingIndex ?? widget.currentIndex).toDouble());
  }

  void _handleTap(int index) {
    if (_dragging || _swallowTap) return;
    unawaited(_select(index));
  }

  @override
  void dispose() {
    _pendingFallback?.cancel();
    _tapGuardTimer?.cancel();
    _indicatorController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final highlightIndex = _dragging
        ? _hoverIndex
        : (_pendingIndex ?? widget.currentIndex);

    return SafeArea(
      top: false,
      minimum: const EdgeInsets.fromLTRB(HkSpace.md, 0, HkSpace.md, HkSpace.md),
      child: Align(
        alignment: Alignment.bottomCenter,
        heightFactor: 1,
        child: ConstrainedBox(
          constraints: const BoxConstraints(
            maxWidth: HkControl.bottomNavMaxWidth,
          ),
          child: RepaintBoundary(
            child: DecoratedBox(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(HkRadius.xl),
                boxShadow: HkShadows.floatingNav,
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(HkRadius.xl),
                child: BackdropFilter(
                  filter: ImageFilter.blur(
                    sigmaX: HkGlass.navBarBlurSigma,
                    sigmaY: HkGlass.navBarBlurSigma,
                  ),
                  child: Container(
                    key: const ValueKey('buyer-bottom-nav-surface'),
                    height: HkControl.navHeight,
                    padding: const EdgeInsets.all(HkSpace.sm),
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                        colors: [
                          AppColors.neutralWhite.withValues(
                            alpha: HkGlass.navBarHighlightOpacity,
                          ),
                          AppColors.navBarBackground.withValues(
                            alpha: HkGlass.navBarSurfaceOpacity,
                          ),
                        ],
                      ),
                      borderRadius: BorderRadius.circular(HkRadius.xl),
                      border: Border.all(
                        color: AppColors.neutralWhite.withValues(
                          alpha: HkGlass.navBarBorderOpacity,
                        ),
                      ),
                    ),
                    child: LayoutBuilder(
                      builder: (context, constraints) {
                        _trackWidth = constraints.maxWidth;
                        final itemWidth =
                            constraints.maxWidth /
                            SharedBottomNav.labels.length;
                        return Listener(
                          behavior: HitTestBehavior.opaque,
                          onPointerDown: _handlePointerDown,
                          onPointerMove: _handlePointerMove,
                          onPointerUp: _handlePointerUp,
                          onPointerCancel: _handlePointerCancel,
                          child: Stack(
                            children: [
                              AnimatedBuilder(
                                animation: _indicatorController,
                                builder: (context, child) {
                                  final position = _indicatorController.value
                                      .clamp(
                                        0.0,
                                        (SharedBottomNav.labels.length - 1)
                                            .toDouble(),
                                      );
                                  return Positioned(
                                    left: itemWidth * position,
                                    top: 0,
                                    width: itemWidth,
                                    height: HkControl.navItemHeight,
                                    child: AnimatedScale(
                                      duration: _reduceMotion
                                          ? Duration.zero
                                          : const Duration(milliseconds: 160),
                                      curve: Curves.easeOutCubic,
                                      scale: _dragging ? 1.05 : 1,
                                      child: HkGlassTabIndicator(
                                        decorationKey: const ValueKey(
                                          'buyer-bottom-nav-indicator',
                                        ),
                                        dragging: _dragging,
                                        borderRadius: BorderRadius.circular(
                                          HkRadius.md,
                                        ),
                                      ),
                                    ),
                                  );
                                },
                              ),
                              Row(
                                children: List.generate(
                                  SharedBottomNav.labels.length,
                                  (index) => Expanded(
                                    child: _BottomNavItem(
                                      key: ValueKey(
                                        'buyer-bottom-nav-item-$index',
                                      ),
                                      label: SharedBottomNav.labels[index],
                                      icon: SharedBottomNav.icons[index],
                                      selected: index == highlightIndex,
                                      onTap: () => _handleTap(index),
                                    ),
                                  ),
                                ),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _BottomNavItem extends StatelessWidget {
  const _BottomNavItem({
    super.key,
    required this.label,
    required this.icon,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final HkIconName icon;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label: label,
      button: true,
      selected: selected,
      onTap: onTap,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: onTap,
          child: SizedBox(
            height: HkControl.navItemHeight,
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                HkIcon(
                  icon,
                  selected: selected,
                  size: HkControl.navIcon,
                  color: selected ? AppIconColors.active : AppIconColors.muted,
                ),
                const SizedBox(height: 2),
                AnimatedDefaultTextStyle(
                  duration: const Duration(milliseconds: 160),
                  curve: Curves.easeOutCubic,
                  style: TextStyle(
                    color: selected
                        ? AppColors.actionPrimary
                        : AppColors.chromeMuted,
                    fontSize: 12,
                    height: 1,
                    fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                  ),
                  child: Text(label, maxLines: 1, overflow: TextOverflow.clip),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
