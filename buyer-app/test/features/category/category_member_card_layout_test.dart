import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/features/category/presentation/category_landing_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  for (final scenario in <({Size size, double scale, AppTextSize textSize})>[
    (size: const Size(320, 568), scale: 1, textSize: AppTextSize.large),
    (size: const Size(320, 568), scale: 2, textSize: AppTextSize.standard),
    (size: const Size(390, 844), scale: 1, textSize: AppTextSize.large),
    (size: const Size(768, 1024), scale: 2, textSize: AppTextSize.standard),
  ]) {
    testWidgets(
      'member summary fits ${scenario.size.width.toInt()}dp at effective '
      '${scenario.textSize == AppTextSize.large ? '1.15' : '1.4'}x',
      (tester) async {
        await pumpProductionLayoutTestApp(
          tester,
          size: scenario.size,
          systemTextScale: scenario.scale,
          textSize: scenario.textSize,
          home: Scaffold(
            body: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 28),
              child: SizedBox(
                height: 70,
                child: CategoryMemberCardSummary(
                  merchantAvatars: List.filled(10, ''),
                  memberCardCount: 999999,
                ),
              ),
            ),
          ),
        );

        expect(tester.takeException(), isNull);
        final summary = tester.getRect(
          find.byKey(const ValueKey('category-member-card-summary')),
        );
        expect(summary.left, greaterThanOrEqualTo(0));
        expect(summary.right, lessThanOrEqualTo(scenario.size.width));

        final avatarStack = tester.getRect(
          find.byKey(const ValueKey('category-member-avatar-stack')),
        );
        expect(summary.contains(avatarStack.topLeft), isTrue);
        expect(summary.contains(avatarStack.bottomRight), isTrue);
        if (scenario.size.width == 320) {
          expect(avatarStack.width, lessThan(292));
        }
      },
    );
  }
}
