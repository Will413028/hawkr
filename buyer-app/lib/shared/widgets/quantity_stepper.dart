import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_round_action_button.dart';
import 'package:flutter/material.dart';

/// A reusable quantity stepper with standalone, unframed +/- circular buttons.
///
/// Layout specs (base 414x896):
/// - Button diameter: 32
/// - Accessible action target: 48
/// - Count text minimum width: 32; grows for large values/text
/// - No outer pill/frame
class QuantityStepper extends StatelessWidget {
  const QuantityStepper({
    super.key,
    required this.count,
    required this.onIncrement,
    required this.onDecrement,
    this.minCount = 1,
    this.maxCount,
    this.disabled = false,
    this.reversed = false,
    this.semanticLabel = '數量',
    this.stacked = false,
    this.countTextStyle,
  }) : assert(maxCount == null || maxCount >= minCount);

  final int count;
  final VoidCallback onIncrement;
  final VoidCallback onDecrement;
  final int minCount;
  final int? maxCount;
  final bool disabled;
  final bool reversed;
  final String semanticLabel;

  /// Put the count above the two controls when a large value cannot fit inline.
  /// Digits retain their full text scale and can wrap within the available width.
  final bool stacked;

  /// Optional surface-specific numeral role; retains the shared default when
  /// omitted. Text scaling and 48px action targets are unchanged.
  final TextStyle? countTextStyle;

  @override
  Widget build(BuildContext context) {
    final bool canIncrement =
        !disabled && (maxCount == null || count < maxCount!);
    final bool canDecrement = !disabled && count > minCount;
    final decrementButton = HkRoundActionButton(
      semanticLabel: '減少數量',
      semanticHint: canDecrement
          ? null
          : disabled
          ? '目前無法調整數量'
          : '已達最小數量',
      symbol: HkRoundActionSymbol.minus,
      onPressed: canDecrement ? onDecrement : null,
      visualKey: const ValueKey('quantity-stepper-minus-visual'),
      iconKey: const ValueKey('quantity-stepper-minus-glyph'),
    );
    final incrementButton = HkRoundActionButton(
      semanticLabel: '增加數量',
      semanticHint: canIncrement
          ? null
          : disabled
          ? '目前無法調整數量'
          : '已達最大數量',
      symbol: HkRoundActionSymbol.plus,
      onPressed: canIncrement ? onIncrement : null,
      visualKey: const ValueKey('quantity-stepper-plus-visual'),
      iconKey: const ValueKey('quantity-stepper-plus-glyph'),
    );
    final countLabel = ConstrainedBox(
      key: const ValueKey('quantity-stepper-count'),
      constraints: const BoxConstraints(minWidth: 32),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
        child: Text(
          '$count',
          maxLines: stacked ? null : 1,
          softWrap: stacked,
          textAlign: TextAlign.center,
          style: countTextStyle ?? AppTypography.h1.copyWith(height: 1),
        ),
      ),
    );

    return Semantics(
      container: true,
      explicitChildNodes: true,
      label: semanticLabel,
      value: '$count',
      child: stacked
          ? Column(
              key: const ValueKey('quantity-stepper'),
              mainAxisSize: MainAxisSize.min,
              children: [
                ExcludeSemantics(child: countLabel),
                const SizedBox(height: HkSpace.xs),
                SizedBox(
                  height: HkControl.minTap,
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: reversed
                        ? [incrementButton, decrementButton]
                        : [decrementButton, incrementButton],
                  ),
                ),
              ],
            )
          : SizedBox(
              key: const ValueKey('quantity-stepper'),
              height: HkControl.minTap,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.center,
                children: reversed
                    ? [
                        incrementButton,
                        ExcludeSemantics(child: countLabel),
                        decrementButton,
                      ]
                    : [
                        decrementButton,
                        ExcludeSemantics(child: countLabel),
                        incrementButton,
                      ],
              ),
            ),
    );
  }
}
