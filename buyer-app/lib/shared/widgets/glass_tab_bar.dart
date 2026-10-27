import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/hk_tokens.dart';
import 'hk_liquid_glass.dart';

/// Hawkr 共用的浮動 Liquid Glass Tab 軌道。
class HkGlassTabSurface extends StatelessWidget {
  const HkGlassTabSurface({
    super.key,
    required this.child,
    this.height,
    this.borderRadius = const BorderRadius.all(Radius.circular(HkRadius.pill)),
  });

  final Widget child;
  final double? height;
  final BorderRadius borderRadius;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: height,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkGlass.tabHorizontalInset,
          vertical: HkGlass.tabVerticalInset,
        ),
        child: HkLiquidGlassSurface(
          variant: HkLiquidGlassVariant.regular,
          backgroundColor: AppColors.navBarGlass,
          surfaceOpacity: HkGlass.navBarSurfaceOpacity,
          blurSigma: HkGlass.tabBarBlurSigma,
          borderRadius: borderRadius,
          border: Border.all(color: AppColors.borderSubtle),
          shadows: [
            BoxShadow(
              color: AppColors.textEmphasis.withValues(alpha: 0.10),
              blurRadius: 18,
              offset: const Offset(0, 5),
            ),
            BoxShadow(
              color: AppColors.neutralWhite.withValues(alpha: 0.56),
              blurRadius: 2,
              offset: const Offset(0, -1),
            ),
          ],
          child: child,
        ),
      ),
    );
  }
}

/// 在同一塊玻璃容器裡滑動的 Liquid Glass 選取 thumb。
class HkGlassTabIndicator extends StatelessWidget {
  const HkGlassTabIndicator({
    super.key,
    this.decorationKey,
    this.dragging = false,
    this.borderRadius = const BorderRadius.all(Radius.circular(16)),
  });

  final Key? decorationKey;
  final bool dragging;
  final BorderRadius borderRadius;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      key: decorationKey,
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            AppColors.neutralWhite.withValues(
              alpha: HkGlass.tabHighlightOpacity,
            ),
            AppColors.neutralWhite.withValues(
              alpha: HkGlass.tabIndicatorSurfaceOpacity,
            ),
            AppColors.neutralWhite.withValues(alpha: 0.68),
          ],
          stops: const [0, 0.58, 1],
        ),
        borderRadius: borderRadius,
        border: Border.all(
          color: AppColors.neutralWhite.withValues(alpha: 0.96),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.textEmphasis.withValues(
              alpha: dragging ? 0.16 : 0.10,
            ),
            blurRadius: dragging ? 16 : 10,
            offset: Offset(0, dragging ? 5 : 3),
          ),
          BoxShadow(
            color: AppColors.neutralWhite.withValues(alpha: 0.72),
            blurRadius: 2,
            offset: const Offset(-1, -1),
          ),
        ],
      ),
      child: const SizedBox.expand(),
    );
  }
}

/// 等寬文字 Tab，供不需要 [TabController] 的頁面共用。
class HkGlassTabBar extends StatefulWidget {
  const HkGlassTabBar({
    super.key,
    required this.labels,
    required this.selectedIndex,
    required this.onTap,
    this.height = 52,
    this.maxWidth = 560,
    this.indicatorKey,
    this.labelKeyPrefix = 'glass_tab_label_',
  }) : assert(labels.length > 1),
       assert(selectedIndex >= 0 && selectedIndex < labels.length);

  final List<String> labels;
  final int selectedIndex;
  final ValueChanged<int> onTap;
  final double height;
  final double maxWidth;
  final Key? indicatorKey;
  final String labelKeyPrefix;

  @override
  State<HkGlassTabBar> createState() => _HkGlassTabBarState();
}

class _HkGlassTabBarState extends State<HkGlassTabBar> {
  int? _pressedIndex;

  @override
  Widget build(BuildContext context) {
    final reduceMotion =
        (MediaQuery.maybeOf(context)?.disableAnimations ?? false) ||
        View.of(context).platformDispatcher.accessibilityFeatures.reduceMotion;
    // Keep the inset glass artwork unchanged while the transparent interaction
    // layer below owns the full accessible tap height.
    final effectiveHeight = widget.height < HkControl.minTap
        ? HkControl.minTap
        : widget.height;
    return SizedBox(
      height: effectiveHeight,
      child: Stack(
        fit: StackFit.expand,
        children: [
          HkGlassTabSurface(
            height: effectiveHeight,
            child: Center(
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: widget.maxWidth),
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final tabWidth =
                        constraints.maxWidth / widget.labels.length;
                    final trackHeight = constraints.maxHeight;
                    final indicatorInset = HkGlass.tabSelectionInset;
                    return Stack(
                      children: [
                        AnimatedPositioned(
                          duration: reduceMotion
                              ? Duration.zero
                              : HkMotion.duration,
                          curve: HkMotion.ease,
                          left:
                              tabWidth * widget.selectedIndex + indicatorInset,
                          top: indicatorInset,
                          width: tabWidth - indicatorInset * 2,
                          height: trackHeight - indicatorInset * 2,
                          child: AnimatedScale(
                            duration: reduceMotion
                                ? Duration.zero
                                : const Duration(milliseconds: 120),
                            curve: Curves.easeOutCubic,
                            scale: _pressedIndex == widget.selectedIndex
                                ? 0.97
                                : 1,
                            child: HkGlassTabIndicator(
                              decorationKey: widget.indicatorKey,
                            ),
                          ),
                        ),
                      ],
                    );
                  },
                ),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: HkGlass.tabHorizontalInset,
            ),
            child: Center(
              child: ConstrainedBox(
                constraints: BoxConstraints(
                  maxWidth: widget.maxWidth,
                  minHeight: effectiveHeight,
                ),
                child: Row(
                  children: List.generate(widget.labels.length, (index) {
                    final selected = index == widget.selectedIndex;
                    return Expanded(
                      child: SizedBox(
                        height: effectiveHeight,
                        child: Semantics(
                          button: true,
                          selected: selected,
                          label: '${widget.labels[index]}頁籤',
                          child: Material(
                            type: MaterialType.transparency,
                            child: InkWell(
                              onHighlightChanged: (highlighted) {
                                setState(() {
                                  _pressedIndex = highlighted ? index : null;
                                });
                              },
                              onTap: () {
                                if (index != widget.selectedIndex &&
                                    !reduceMotion) {
                                  HapticFeedback.selectionClick();
                                }
                                widget.onTap(index);
                              },
                              borderRadius: BorderRadius.circular(
                                HkRadius.pill,
                              ),
                              splashFactory: NoSplash.splashFactory,
                              overlayColor: const WidgetStatePropertyAll(
                                Colors.transparent,
                              ),
                              child: Center(
                                child: AnimatedDefaultTextStyle(
                                  duration: reduceMotion
                                      ? Duration.zero
                                      : HkMotion.duration,
                                  curve: HkMotion.ease,
                                  style: TextStyle(
                                    color: selected
                                        ? AppColors.textEmphasis
                                        : AppColors.neutralMutedText,
                                    fontSize: 16,
                                    fontWeight: selected
                                        ? FontWeight.w700
                                        : FontWeight.w500,
                                    height: 1.15,
                                  ),
                                  child: Text(
                                    widget.labels[index],
                                    key: Key('${widget.labelKeyPrefix}$index'),
                                    textAlign: TextAlign.center,
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    );
                  }),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
