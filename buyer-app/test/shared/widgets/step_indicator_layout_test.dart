import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/shared/widgets/step_indicator.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  for (final scenario
      in <({Size size, double width, double scale, AppTextSize textSize})>[
        (
          size: const Size(320, 568),
          width: 208,
          scale: 1,
          textSize: AppTextSize.large,
        ),
        (
          size: const Size(320, 568),
          width: 208,
          scale: 2,
          textSize: AppTextSize.standard,
        ),
        (
          size: const Size(390, 844),
          width: 264,
          scale: 2,
          textSize: AppTextSize.standard,
        ),
        (
          size: const Size(768, 1024),
          width: 560,
          scale: 2,
          textSize: AppTextSize.standard,
        ),
      ]) {
    testWidgets(
      'four-step progress fits ${scenario.width.toInt()}dp at large text',
      (tester) async {
        await pumpProductionLayoutTestApp(
          tester,
          size: scenario.size,
          systemTextScale: scenario.scale,
          textSize: scenario.textSize,
          home: Scaffold(
            body: Center(
              child: SizedBox(
                key: const ValueKey('step-indicator-bounds'),
                width: scenario.width,
                child: const StepIndicator(
                  steps: StepIndicator.afterSaleStepsRejected,
                  currentStep: 3,
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        final bounds = tester.getRect(
          find.byKey(const ValueKey('step-indicator-bounds')),
        );
        for (final step in StepIndicator.afterSaleStepsRejected) {
          final label = find.text(step.label);
          expect(label, findsOneWidget);
          final rect = tester.getRect(label);
          expect(rect.left, greaterThanOrEqualTo(bounds.left - 0.01));
          expect(rect.right, lessThanOrEqualTo(bounds.right + 0.01));
          expect(rect.top, greaterThanOrEqualTo(bounds.top - 0.01));
          expect(rect.bottom, lessThanOrEqualTo(bounds.bottom + 0.01));
          expect(
            tester.renderObject<RenderParagraph>(label).didExceedMaxLines,
            isFalse,
          );
        }
      },
    );
  }
}
