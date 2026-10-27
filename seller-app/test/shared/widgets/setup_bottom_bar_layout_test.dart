import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/shared/widgets/setup_bottom_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in [320.0, 402.0, 440.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        'setup footer labels and buttons stay intact at $width / $scale',
        (tester) async {
          tester.view
            ..physicalSize = Size(width, 568)
            ..devicePixelRatio = 1;
          addTearDown(tester.view.reset);
          var back = 0;
          var next = 0;
          await tester.pumpWidget(
            ScreenUtilInit(
              designSize: const Size(414, 896),
              enableScaleWH: () => false,
              enableScaleText: () => false,
              builder: (_, _) => MaterialApp(
                theme: AppTheme.light,
                builder: (context, child) => MediaQuery(
                  data: MediaQuery.of(
                    context,
                  ).copyWith(textScaler: TextScaler.linear(scale)),
                  child: child!,
                ),
                home: Scaffold(
                  bottomNavigationBar: SetupBottomBar(
                    onBack: () => back++,
                    onNext: () => next++,
                    nextLabel: '完成且預覽',
                  ),
                ),
              ),
            ),
          );
          expect(tester.takeException(), isNull);
          for (final label in ['上一步', '完成且預覽']) {
            final text = find.text(label);
            expect(
              tester.renderObject<RenderParagraph>(text).didExceedMaxLines,
              isFalse,
            );
            final button = find
                .ancestor(of: text, matching: find.byType(GestureDetector))
                .first;
            final rect = tester.getRect(button);
            expect(rect.width, greaterThanOrEqualTo(48));
            expect(rect.height, greaterThanOrEqualTo(48));
            expect(rect.left, greaterThanOrEqualTo(16));
            expect(rect.right, lessThanOrEqualTo(width - 16));
            await tester.tap(button);
          }
          expect(back, 1);
          expect(next, 1);
        },
      );
    }
  }
}
