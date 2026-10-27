import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_icon.dart';
import 'glass_tab_bar.dart';

/// 底部導航列索引
enum NavIndex {
  dashboard(0),
  category(1),
  wallet(2),
  order(3),
  setting(4);

  final int value;
  const NavIndex(this.value);
}

/// 賣家端 iOS 26 Liquid Glass 浮動底部導覽。
///
/// 提供統一的底部導航列樣式，可用於：
/// 1. 主導航頁面（MainNavigationScreen）- 使用 onTap callback
/// 2. 子頁面 - 自動導航回主頁面對應 tab
///
/// 使用範例：
/// ```dart
/// // 子頁面使用（自動導航）
/// Scaffold(
///   bottomNavigationBar: AppBottomNavigationBar(
///     currentIndex: NavIndex.wallet,
///   ),
/// )
///
/// // 主導航頁面使用（自定義 onTap）
/// Scaffold(
///   bottomNavigationBar: AppBottomNavigationBar(
///     currentIndex: NavIndex.dashboard,
///     onTap: (index) => setState(() => _currentIndex = index),
///   ),
/// )
/// ```
class AppBottomNavigationBar extends StatefulWidget {
  /// 當前選中的索引
  final NavIndex currentIndex;

  /// 點擊回調（如果為 null，則使用預設導航行為）
  final ValueChanged<int>? onTap;

  /// 背景顏色
  final Color backgroundColor;

  /// 是否顯示標籤文字
  final bool showLabels;

  const AppBottomNavigationBar({
    super.key,
    required this.currentIndex,
    this.onTap,
    this.backgroundColor = AppColors.navBarBackground,
    this.showLabels = true,
  });

  static const _icons = [
    HkIconName.home,
    HkIconName.market,
    HkIconName.wallet,
    HkIconName.box,
    HkIconName.user,
  ];

  @override
  State<AppBottomNavigationBar> createState() => _AppBottomNavigationBarState();
}

class _AppBottomNavigationBarState extends State<AppBottomNavigationBar> {
  static const int _itemCount = 5;

  bool _dragging = false;
  int? _dragIndex;
  int? _pendingIndex;

  int get _highlightIndex => _dragging
      ? (_dragIndex ?? widget.currentIndex.value)
      : (_pendingIndex ?? widget.currentIndex.value);

  bool get _reduceMotion =>
      (MediaQuery.maybeOf(context)?.disableAnimations ?? false) ||
      View.of(context).platformDispatcher.accessibilityFeatures.reduceMotion;

  @override
  void didUpdateWidget(covariant AppBottomNavigationBar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.currentIndex != widget.currentIndex) {
      _pendingIndex = null;
      if (!_dragging) _dragIndex = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final effectiveOnTap =
        widget.onTap ?? (index) => _defaultOnTap(context, index);
    return ColoredBox(
      color: Colors.transparent,
      child: SafeArea(
        top: false,
        minimum: const EdgeInsets.fromLTRB(
          HkSpace.md,
          0,
          HkSpace.md,
          HkSpace.md,
        ),
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
                      key: const ValueKey('seller-bottom-nav-surface'),
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
                            widget.backgroundColor.withValues(
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
                          final itemWidth = constraints.maxWidth / _itemCount;
                          final highlightIndex = _highlightIndex;
                          return GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onHorizontalDragStart: (details) {
                              final index = _indexForPosition(
                                details.localPosition.dx,
                                constraints.maxWidth,
                              );
                              setState(() {
                                _dragging = true;
                                _dragIndex = index;
                              });
                              _selectionFeedbackIfNeeded(
                                from: widget.currentIndex.value,
                                to: index,
                              );
                            },
                            onHorizontalDragUpdate: (details) {
                              final index = _indexForPosition(
                                details.localPosition.dx,
                                constraints.maxWidth,
                              );
                              final previous =
                                  _dragIndex ?? widget.currentIndex.value;
                              if (index == previous) return;
                              setState(() {
                                _dragIndex = index;
                              });
                              _selectionFeedbackIfNeeded(
                                from: previous,
                                to: index,
                              );
                            },
                            onHorizontalDragEnd: (_) {
                              final target =
                                  _dragIndex ?? widget.currentIndex.value;
                              setState(() {
                                _dragging = false;
                                _dragIndex = null;
                                _pendingIndex =
                                    target == widget.currentIndex.value
                                    ? null
                                    : target;
                              });
                              if (target != widget.currentIndex.value) {
                                effectiveOnTap(target);
                              }
                            },
                            onHorizontalDragCancel: () {
                              setState(() {
                                _dragging = false;
                                _dragIndex = null;
                              });
                            },
                            child: Stack(
                              clipBehavior: Clip.none,
                              children: [
                                AnimatedPositioned(
                                  key: const ValueKey(
                                    'seller-bottom-nav-indicator-position',
                                  ),
                                  duration: _reduceMotion
                                      ? Duration.zero
                                      : const Duration(milliseconds: 180),
                                  curve: HkMotion.ease,
                                  left: itemWidth * highlightIndex,
                                  top: 0,
                                  width: itemWidth,
                                  height: HkControl.navItemHeight,
                                  child: AnimatedScale(
                                    duration: _reduceMotion
                                        ? Duration.zero
                                        : const Duration(milliseconds: 140),
                                    curve: Curves.easeOutCubic,
                                    scale: _dragging ? 1.04 : 1,
                                    child: HkGlassTabIndicator(
                                      decorationKey: const ValueKey(
                                        'seller-bottom-nav-indicator',
                                      ),
                                      dragging: _dragging,
                                      borderRadius: const BorderRadius.all(
                                        Radius.circular(HkRadius.md),
                                      ),
                                    ),
                                  ),
                                ),
                                Row(
                                  children: List.generate(_itemCount, (i) {
                                    const labels = [
                                      '總覽',
                                      '商店',
                                      '錢包',
                                      '訂單',
                                      '設定',
                                    ];
                                    return Expanded(
                                      child: _NavItem(
                                        label: labels[i],
                                        icon: AppBottomNavigationBar._icons[i],
                                        highlighted: i == highlightIndex,
                                        current: i == widget.currentIndex.value,
                                        showLabel: widget.showLabels,
                                        onTap: () {
                                          _selectionFeedbackIfNeeded(
                                            from: widget.currentIndex.value,
                                            to: i,
                                          );
                                          setState(() {
                                            _pendingIndex =
                                                i == widget.currentIndex.value
                                                ? null
                                                : i;
                                          });
                                          effectiveOnTap(i);
                                        },
                                      ),
                                    );
                                  }),
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
      ),
    );
  }

  int _indexForPosition(double x, double width) {
    if (width <= 0) return widget.currentIndex.value;
    final normalized = (x / width).clamp(0.0, 0.999999);
    return (normalized * _itemCount).floor();
  }

  void _selectionFeedbackIfNeeded({required int from, required int to}) {
    if (from == to || _reduceMotion) return;
    HapticFeedback.selectionClick();
  }

  /// 預設點擊行為：導航到主頁面對應 tab
  void _defaultOnTap(BuildContext context, int index) {
    // 如果點擊的是當前 tab，不做任何事
    if (index == widget.currentIndex.value) return;

    const paths = [
      '/main/dashboard',
      '/main/category',
      '/main/wallet',
      '/main/orders',
      '/main/settings',
    ];
    context.go(paths[index]);
  }
}

/// 導覽項目；選取時畫 filled 圖示，未選取畫 outlined 圖示。
class _NavItem extends StatelessWidget {
  const _NavItem({
    required this.label,
    required this.icon,
    required this.highlighted,
    required this.current,
    required this.showLabel,
    required this.onTap,
  });

  final String label;
  final HkIconName icon;
  final bool highlighted;
  final bool current;
  final bool showLabel;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label: label,
      button: true,
      selected: current,
      onTap: onTap,
      child: ExcludeSemantics(
        child: Material(
          type: MaterialType.transparency,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(HkRadius.md),
            splashFactory: NoSplash.splashFactory,
            overlayColor: WidgetStateProperty.resolveWith(
              (states) => states.contains(WidgetState.pressed)
                  ? AppColors.primaryHover
                  : Colors.transparent,
            ),
            child: SizedBox(
              height: HkControl.navItemHeight,
              child: Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    HkIcon(
                      icon,
                      selected: highlighted,
                      size: showLabel ? 27 : HkControl.navIcon,
                      color: highlighted
                          ? AppIconColors.active
                          : AppIconColors.defaultColor,
                    ),
                    if (showLabel) ...[
                      const SizedBox(height: 1),
                      Text(
                        label,
                        maxLines: 1,
                        style: TextStyle(
                          color: highlighted
                              ? AppColors.brandPrimaryStrong
                              : AppColors.textSecondary,
                          fontSize: 12,
                          fontWeight: highlighted
                              ? FontWeight.w700
                              : FontWeight.w500,
                          height: 1,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
