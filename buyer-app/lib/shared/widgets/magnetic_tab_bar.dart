import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/physics.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import 'glass_tab_bar.dart';

typedef MagneticTabIconBuilder = Widget Function(bool selected);

/// A Web-parity tab track with a magnetic, draggable selection pill.
///
/// A short press remains a normal tap. Once the pointer travels 6dp, the pill
/// snaps to the nearest slot and provides selection feedback. Releasing the
/// drag commits the hovered tab, while swiping the matching [TabBarView] keeps
/// the same indicator continuously attached to [controller.animation].
class MagneticTabBar extends StatefulWidget implements PreferredSizeWidget {
  const MagneticTabBar({
    super.key,
    required this.controller,
    required this.labels,
    this.height = 52,
    this.maxWidth = 560,
    this.indicatorKey,
    this.labelKeyPrefix = 'magnetic_tab_label_',
    this.onTabRequested,
    this.iconBuilders,
    this.simpleSelection = false,
    this.showMaterialTapEffect = true,
  }) : assert(
         iconBuilders == null || iconBuilders.length == labels.length,
         'iconBuilders must match labels length',
       );

  final TabController controller;
  final List<String> labels;
  final double height;
  final double maxWidth;
  final Key? indicatorKey;
  final String labelKeyPrefix;
  final FutureOr<bool> Function(int index)? onTabRequested;
  final List<MagneticTabIconBuilder>? iconBuilders;
  final bool simpleSelection;
  final bool showMaterialTapEffect;

  static const transitionDuration = Duration(milliseconds: 280);

  @override
  Size get preferredSize => Size.fromHeight(height);

  @override
  State<MagneticTabBar> createState() => _MagneticTabBarState();
}

class _MagneticTabBarState extends State<MagneticTabBar>
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
        value: widget.controller.index.toDouble(),
      );

  int? _pointer;
  double _pointerStartX = 0;
  bool _dragging = false;
  bool _pressing = false;
  bool _settling = false;
  bool _swallowTap = false;
  Timer? _tapGuardTimer;
  int _hoverIndex = 0;
  double _trackWidth = 1;

  Animation<double> get _tabAnimation =>
      widget.controller.animation ??
      AlwaysStoppedAnimation(widget.controller.index.toDouble());

  bool get _reduceMotion =>
      MediaQuery.maybeOf(context)?.disableAnimations ?? false;

  @override
  void initState() {
    super.initState();
    _hoverIndex = widget.controller.index;
    widget.controller.animation?.addListener(_handleControllerTick);
  }

  @override
  void didUpdateWidget(covariant MagneticTabBar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller == widget.controller) return;
    oldWidget.controller.animation?.removeListener(_handleControllerTick);
    widget.controller.animation?.addListener(_handleControllerTick);
    _hoverIndex = widget.controller.index;
    _indicatorController.value = widget.controller.index.toDouble();
  }

  void _handleControllerTick() {
    if (!_settling || widget.controller.indexIsChanging) return;
    final value = _tabAnimation.value;
    if ((value - widget.controller.index).abs() > 0.001) return;
    _settling = false;
    _indicatorController.value = value;
  }

  void _springTo(double target) {
    if (_reduceMotion || widget.simpleSelection) {
      _indicatorController.value = target;
      return;
    }
    if ((_indicatorController.value - target).abs() < 0.001) return;
    _indicatorController.animateWith(
      SpringSimulation(_spring, _indicatorController.value, target, 0),
    );
  }

  int _slotFor(double localX) {
    final tabWidth = _trackWidth / widget.labels.length;
    return (localX / tabWidth).floor().clamp(0, widget.labels.length - 1);
  }

  void _handlePointerDown(PointerDownEvent event) {
    if (_pointer != null || widget.controller.indexIsChanging) return;
    _pointer = event.pointer;
    _pointerStartX = event.localPosition.dx;
    _hoverIndex = widget.controller.index;
    _settling = false;
    _indicatorController.value = _tabAnimation.value;
    setState(() => _pressing = true);
  }

  void _handlePointerMove(PointerMoveEvent event) {
    if (_pointer != event.pointer) return;
    if (!_dragging &&
        (event.localPosition.dx - _pointerStartX).abs() < _dragThreshold) {
      return;
    }
    if (!_dragging) setState(() => _dragging = true);
    final next = _slotFor(event.localPosition.dx);
    if (next == _hoverIndex) return;
    _hoverIndex = next;
    if (!_reduceMotion) HapticFeedback.selectionClick();
    _springTo(next.toDouble());
    setState(() {});
  }

  Future<void> _commit(int index) async {
    final allowed = await widget.onTabRequested?.call(index) ?? true;
    if (!mounted) return;
    if (!allowed) {
      _settling = false;
      _springTo(widget.controller.index.toDouble());
      setState(() {});
      return;
    }
    _settling = true;
    _springTo(index.toDouble());
    widget.controller.animateTo(
      index,
      duration: _reduceMotion
          ? Duration.zero
          : MagneticTabBar.transitionDuration,
      curve: Curves.easeOutCubic,
    );
    setState(() {});
  }

  void _handlePointerUp(PointerUpEvent event) {
    if (_pointer != event.pointer) return;
    _pointer = null;
    if (!_dragging) {
      setState(() => _pressing = false);
      return;
    }
    _swallowTap = true;
    _tapGuardTimer?.cancel();
    _tapGuardTimer = Timer(Duration.zero, () => _swallowTap = false);
    final target = _hoverIndex;
    setState(() {
      _dragging = false;
      _pressing = false;
    });
    unawaited(_commit(target));
  }

  void _handlePointerCancel(PointerCancelEvent event) {
    if (_pointer != event.pointer) return;
    _pointer = null;
    if (!_dragging) {
      setState(() => _pressing = false);
      return;
    }
    setState(() {
      _dragging = false;
      _pressing = false;
    });
    _springTo(widget.controller.index.toDouble());
  }

  void _handleTap(int index) {
    if (_dragging || _swallowTap) return;
    unawaited(_commit(index));
  }

  @override
  void dispose() {
    widget.controller.animation?.removeListener(_handleControllerTick);
    _tapGuardTimer?.cancel();
    _indicatorController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final animation = Listenable.merge([_tabAnimation, _indicatorController]);
    return HkGlassTabSurface(
      height: widget.height,
      child: Center(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: widget.maxWidth),
          child: LayoutBuilder(
            builder: (context, constraints) {
              _trackWidth = constraints.maxWidth;
              final tabWidth = constraints.maxWidth / widget.labels.length;
              final trackHeight = constraints.maxHeight;
              final indicatorInset = HkGlass.tabSelectionInset;
              return Listener(
                behavior: HitTestBehavior.opaque,
                onPointerDown: _handlePointerDown,
                onPointerMove: _handlePointerMove,
                onPointerUp: _handlePointerUp,
                onPointerCancel: _handlePointerCancel,
                child: AnimatedBuilder(
                  animation: animation,
                  builder: (context, _) {
                    final highlightedIndex = _dragging
                        ? _hoverIndex
                        : widget.controller.index;
                    final rawPosition = widget.simpleSelection
                        ? highlightedIndex.toDouble()
                        : _dragging || _settling
                        ? _indicatorController.value
                        : _tabAnimation.value;
                    final position = rawPosition.clamp(
                      0.0,
                      (widget.labels.length - 1).toDouble(),
                    );
                    return Stack(
                      children: [
                        Positioned(
                          left: tabWidth * position + indicatorInset,
                          top: indicatorInset,
                          width: math.max(0, tabWidth - indicatorInset * 2),
                          height: trackHeight - indicatorInset * 2,
                          child: AnimatedScale(
                            duration: _reduceMotion || widget.simpleSelection
                                ? Duration.zero
                                : const Duration(milliseconds: 160),
                            curve: Curves.easeOutCubic,
                            scale: widget.simpleSelection
                                ? 1
                                : _dragging
                                ? 1.04
                                : _pressing
                                ? 0.97
                                : 1,
                            child: HkGlassTabIndicator(
                              decorationKey: widget.indicatorKey,
                              dragging: _dragging,
                            ),
                          ),
                        ),
                        Row(
                          children: List.generate(widget.labels.length, (
                            index,
                          ) {
                            final strength = widget.simpleSelection
                                ? (highlightedIndex == index ? 1.0 : 0.0)
                                : (1 - (position - index).abs()).clamp(
                                    0.0,
                                    1.0,
                                  );
                            final selected = widget.controller.index == index;
                            final visuallySelected = widget.simpleSelection
                                ? highlightedIndex == index
                                : selected;
                            final iconBuilders = widget.iconBuilders;
                            return Expanded(
                              child: MouseRegion(
                                cursor: SystemMouseCursors.click,
                                child: Semantics(
                                  button: true,
                                  selected: selected,
                                  label: '${widget.labels[index]}頁籤',
                                  child: Material(
                                    type: MaterialType.transparency,
                                    child: InkWell(
                                      onTap: () => _handleTap(index),
                                      borderRadius: BorderRadius.circular(
                                        HkRadius.pill,
                                      ),
                                      splashFactory:
                                          widget.showMaterialTapEffect
                                          ? null
                                          : NoSplash.splashFactory,
                                      overlayColor: widget.showMaterialTapEffect
                                          ? null
                                          : const WidgetStatePropertyAll(
                                              Colors.transparent,
                                            ),
                                      child: Center(
                                        child: Row(
                                          mainAxisSize: MainAxisSize.min,
                                          mainAxisAlignment:
                                              MainAxisAlignment.center,
                                          children: [
                                            if (iconBuilders != null) ...[
                                              iconBuilders[index](
                                                visuallySelected,
                                              ),
                                              const SizedBox(width: 4),
                                            ],
                                            Text(
                                              widget.labels[index],
                                              key: Key(
                                                '${widget.labelKeyPrefix}$index',
                                              ),
                                              textAlign: TextAlign.center,
                                              style: TextStyle(
                                                color: Color.lerp(
                                                  AppColors.neutralMutedText,
                                                  AppColors.textEmphasis,
                                                  strength,
                                                ),
                                                fontSize: 18,
                                                fontWeight: FontWeight.lerp(
                                                  FontWeight.w500,
                                                  FontWeight.w700,
                                                  strength,
                                                ),
                                                height: 1.15,
                                              ),
                                            ),
                                          ],
                                        ),
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            );
                          }),
                        ),
                      ],
                    );
                  },
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}
