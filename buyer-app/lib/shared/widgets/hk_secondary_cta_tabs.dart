import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';

/// Shared secondary CTA used for compact, mutually exclusive page filters.
class HkSecondaryCtaTabs extends StatefulWidget {
  const HkSecondaryCtaTabs({
    super.key,
    required this.labels,
    required this.selectedIndex,
    required this.onSelected,
    required this.itemKeyPrefix,
    this.trackKey,
  }) : assert(labels.length > 1),
       assert(selectedIndex >= 0 && selectedIndex < labels.length);

  final List<String> labels;
  final int selectedIndex;
  final ValueChanged<int> onSelected;
  final String itemKeyPrefix;
  final Key? trackKey;

  @override
  State<HkSecondaryCtaTabs> createState() => _HkSecondaryCtaTabsState();
}

class _HkSecondaryCtaTabsState extends State<HkSecondaryCtaTabs> {
  int? _pressedIndex;

  @override
  Widget build(BuildContext context) {
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    const trackRadius = BorderRadius.all(Radius.circular(HkRadius.lg));
    return SizedBox(
      height: HkControl.minTap,
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: trackRadius,
          boxShadow: [
            BoxShadow(
              color: AppColors.textEmphasis.withValues(alpha: 0.08),
              blurRadius: 22,
              offset: const Offset(0, 7),
            ),
            BoxShadow(
              color: AppColors.neutralWhite.withValues(alpha: 0.56),
              blurRadius: 2,
              offset: const Offset(0, -1),
            ),
          ],
        ),
        child: ClipRRect(
          borderRadius: trackRadius,
          child: BackdropFilter(
            filter: ImageFilter.blur(
              sigmaX: HkGlass.tabBarBlurSigma,
              sigmaY: HkGlass.tabBarBlurSigma,
            ),
            child: DecoratedBox(
              key: widget.trackKey,
              decoration: BoxDecoration(
                color: Color.alphaBlend(
                  AppColors.brandPrimaryStrong.withValues(alpha: 0.08),
                  AppColors.neutralCream.withValues(alpha: 0.78),
                ),
                borderRadius: trackRadius,
                border: Border.all(
                  color: AppColors.neutralWhite.withValues(alpha: 0.72),
                ),
              ),
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final tabWidth = constraints.maxWidth / widget.labels.length;
                  const indicatorInset = HkGlass.tabSelectionInset;
                  return Stack(
                    children: [
                      AnimatedPositioned(
                        duration: reduceMotion
                            ? Duration.zero
                            : HkMotion.duration,
                        curve: HkMotion.ease,
                        left: tabWidth * widget.selectedIndex + indicatorInset,
                        top: indicatorInset,
                        width: tabWidth - indicatorInset * 2,
                        height: constraints.maxHeight - indicatorInset * 2,
                        child: AnimatedScale(
                          duration: reduceMotion
                              ? Duration.zero
                              : const Duration(milliseconds: 120),
                          curve: Curves.easeOutCubic,
                          scale: _pressedIndex == widget.selectedIndex
                              ? 0.97
                              : 1,
                          child: _MatteGlassIndicator(
                            key: ValueKey('${widget.itemKeyPrefix}-indicator'),
                          ),
                        ),
                      ),
                      Row(
                        children: [
                          for (
                            var index = 0;
                            index < widget.labels.length;
                            index++
                          )
                            Expanded(
                              child: _SecondaryCtaTab(
                                key: ValueKey('${widget.itemKeyPrefix}-$index'),
                                label: widget.labels[index],
                                selected: widget.selectedIndex == index,
                                reduceMotion: reduceMotion,
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
            ),
          ),
        ),
      ),
    );
  }
}

class _MatteGlassIndicator extends StatelessWidget {
  const _MatteGlassIndicator({super.key});

  @override
  Widget build(BuildContext context) {
    const radius = BorderRadius.all(Radius.circular(HkRadius.md));
    return DecoratedBox(
      decoration: BoxDecoration(
        color: Color.alphaBlend(
          AppColors.brandPrimaryStrong.withValues(alpha: 0.07),
          AppColors.neutralWhite.withValues(alpha: 0.78),
        ),
        borderRadius: radius,
        border: Border.all(
          color: AppColors.neutralWhite.withValues(alpha: 0.88),
        ),
        boxShadow: [
          BoxShadow(
            color: AppColors.textEmphasis.withValues(alpha: 0.10),
            blurRadius: 10,
            offset: const Offset(0, 3),
          ),
          BoxShadow(
            color: AppColors.neutralWhite.withValues(alpha: 0.56),
            blurRadius: 2,
            offset: const Offset(-1, -1),
          ),
        ],
      ),
      child: ClipRRect(
        borderRadius: radius,
        child: BackdropFilter(
          filter: ImageFilter.blur(
            sigmaX: HkGlass.tabIndicatorBlurSigma,
            sigmaY: HkGlass.tabIndicatorBlurSigma,
          ),
          child: const SizedBox.expand(),
        ),
      ),
    );
  }
}

class _SecondaryCtaTab extends StatelessWidget {
  const _SecondaryCtaTab({
    super.key,
    required this.label,
    required this.selected,
    required this.reduceMotion,
    required this.onHighlightChanged,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final bool reduceMotion;
  final ValueChanged<bool> onHighlightChanged;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      inMutuallyExclusiveGroup: true,
      label: label,
      excludeSemantics: true,
      child: Material(
        color: Colors.transparent,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.all(Radius.circular(HkRadius.md)),
        ),
        child: InkWell(
          onHighlightChanged: onHighlightChanged,
          onTap: onTap,
          customBorder: const RoundedRectangleBorder(
            borderRadius: BorderRadius.all(Radius.circular(HkRadius.md)),
          ),
          splashFactory: NoSplash.splashFactory,
          overlayColor: const WidgetStatePropertyAll(Colors.transparent),
          child: Center(
            child: AnimatedDefaultTextStyle(
              duration: reduceMotion ? Duration.zero : HkMotion.duration,
              curve: HkMotion.ease,
              style: AppTypography.meta.copyWith(
                color: selected
                    ? AppColors.brandPrimaryStrong
                    : AppColors.neutralMutedText,
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
