import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';

/// Coupon-specific quiet tabs.
///
/// The wallet's primary member-card/coupon switch already owns the glass
/// surface. This second level stays visually lighter while preserving an
/// equal 48dp touch target, haptics, semantics and reduced-motion behavior.
class CouponStatusTabs extends StatefulWidget {
  const CouponStatusTabs({
    super.key,
    required this.selectedIndex,
    required this.onSelected,
  });

  static const labels = ['可使用', '已使用', '已過期'];

  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  State<CouponStatusTabs> createState() => _CouponStatusTabsState();
}

class _CouponStatusTabsState extends State<CouponStatusTabs> {
  static const double _indicatorHeight = 2;

  int? _pressedIndex;

  @override
  Widget build(BuildContext context) {
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    final duration = reduceMotion ? Duration.zero : HkMotion.duration;

    return SizedBox(
      key: const ValueKey('coupon-status-tabs'),
      height: HkControl.minTap,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final tabWidth =
              constraints.maxWidth / CouponStatusTabs.labels.length;
          final indicatorWidth = (tabWidth - HkSpace.section)
              .clamp(HkSpace.xxl, tabWidth)
              .toDouble();
          final indicatorLeft =
              tabWidth * widget.selectedIndex + (tabWidth - indicatorWidth) / 2;

          return Stack(
            children: [
              const Positioned.fill(
                child: DecoratedBox(
                  key: ValueKey('coupon-status-track'),
                  decoration: BoxDecoration(
                    border: Border(
                      bottom: BorderSide(color: AppColors.borderSubtle),
                    ),
                  ),
                ),
              ),
              AnimatedPositioned(
                key: const ValueKey('coupon-status-tab-indicator'),
                duration: duration,
                curve: HkMotion.ease,
                left: indicatorLeft,
                bottom: 0,
                width: indicatorWidth,
                height: _indicatorHeight,
                child: const DecoratedBox(
                  decoration: BoxDecoration(
                    color: AppColors.brandPrimaryStrong,
                    borderRadius: BorderRadius.all(
                      Radius.circular(HkRadius.pill),
                    ),
                  ),
                ),
              ),
              Row(
                children: [
                  for (
                    var index = 0;
                    index < CouponStatusTabs.labels.length;
                    index++
                  )
                    Expanded(
                      child: _CouponStatusTab(
                        key: ValueKey('coupon-status-tab-$index'),
                        label: CouponStatusTabs.labels[index],
                        selected: widget.selectedIndex == index,
                        pressed: _pressedIndex == index,
                        reduceMotion: reduceMotion,
                        onHighlightChanged: (highlighted) {
                          setState(() {
                            _pressedIndex = highlighted ? index : null;
                          });
                        },
                        onTap: () {
                          if (index != widget.selectedIndex && !reduceMotion) {
                            HapticFeedback.selectionClick();
                          }
                          widget.onSelected(index);
                        },
                      ),
                    ),
                ],
              ),
            ],
          );
        },
      ),
    );
  }
}

class _CouponStatusTab extends StatelessWidget {
  const _CouponStatusTab({
    super.key,
    required this.label,
    required this.selected,
    required this.pressed,
    required this.reduceMotion,
    required this.onHighlightChanged,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final bool pressed;
  final bool reduceMotion;
  final ValueChanged<bool> onHighlightChanged;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final duration = reduceMotion ? Duration.zero : HkMotion.duration;
    final backgroundColor = pressed
        ? AppColors.primaryHover
        : selected
        ? AppColors.primarySubtle
        : Colors.transparent;

    return Semantics(
      button: true,
      selected: selected,
      inMutuallyExclusiveGroup: true,
      label: label,
      excludeSemantics: true,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          onHighlightChanged: onHighlightChanged,
          splashFactory: NoSplash.splashFactory,
          overlayColor: const WidgetStatePropertyAll(Colors.transparent),
          child: AnimatedContainer(
            duration: duration,
            curve: HkMotion.ease,
            margin: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
            decoration: BoxDecoration(
              color: backgroundColor,
              borderRadius: BorderRadius.circular(HkRadius.md),
            ),
            alignment: Alignment.center,
            child: AnimatedDefaultTextStyle(
              duration: duration,
              curve: HkMotion.ease,
              style: AppTypography.meta.copyWith(
                color: selected
                    ? AppColors.brandPrimaryStrong
                    : AppColors.textPrimary,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              ),
              child: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis),
            ),
          ),
        ),
      ),
    );
  }
}
