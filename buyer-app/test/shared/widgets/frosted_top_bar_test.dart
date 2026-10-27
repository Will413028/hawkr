import 'dart:math' as math;

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/common_top_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    testWidgets('frosted header stays bounded at ${width.toInt()}px', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(width, 844);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              appBar: buildCommonAppBar(
                context,
                backgroundColor: AppColors.memberCardSurface,
                systemOverlayStyle: AppColors.glassTopOverlay,
                frostedGlass: true,
                child: const SizedBox.expand(
                  key: ValueKey('frosted-top-bar-content'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(tester.takeException(), isNull);
      final blur = find.byType(BackdropFilter);
      expect(blur, findsOneWidget);
      expect(
        find.ancestor(of: blur, matching: find.byType(ClipRSuperellipse)),
        findsOneWidget,
      );
      expect(
        find.ancestor(of: blur, matching: find.byType(RepaintBoundary)),
        findsWidgets,
      );
      expect(
        tester.getSize(find.byKey(const ValueKey('frosted-top-bar-content'))),
        Size(
          math.min(width - (HkSpace.md * 2), HkControl.chromeMaxWidth),
          HkControl.topBarHeight,
        ),
      );
      expect(tester.getSize(blur).height, HkControl.topBarHeight);

      final overlay = tester.widget<AnnotatedRegion<SystemUiOverlayStyle>>(
        find.byType(AnnotatedRegion<SystemUiOverlayStyle>),
      );
      expect(overlay.value, AppColors.glassTopOverlay);

      final decorations = tester.widgetList<DecoratedBox>(
        find.descendant(of: blur, matching: find.byType(DecoratedBox)),
      );
      expect(
        decorations.any((widget) {
          final decoration = widget.decoration;
          return decoration is BoxDecoration &&
              ((decoration.color?.a ?? 0) - HkGlass.topBarSurfaceOpacity)
                      .abs() <
                  0.001;
        }),
        isTrue,
      );
      expect(
        decorations.any((widget) {
          final decoration = widget.decoration;
          return decoration is BoxDecoration &&
              decoration.gradient is LinearGradient;
        }),
        isTrue,
      );
    });
  }

  testWidgets('common headers use white frosted glass by default', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            appBar: buildCommonAppBar(context, child: const SizedBox.expand()),
          ),
        ),
      ),
    );

    expect(find.byType(BackdropFilter), findsOneWidget);
  });

  testWidgets('explicit solid headers can opt out of backdrop blur', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            appBar: buildCommonAppBar(
              context,
              frostedGlass: false,
              child: const SizedBox.expand(),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(BackdropFilter), findsNothing);
  });
}
