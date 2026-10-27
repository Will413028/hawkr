import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/category_top_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('branded category top bar keeps a distinct dark surface', (
    tester,
  ) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => const MaterialApp(
          home: Scaffold(
            body: CategoryTopBar(
              backgroundColor: AppColors.primary,
              systemOverlayStyle: AppColors.brandTopOverlay,
              showBottomBorder: false,
              borderRadius: BorderRadius.only(
                bottomLeft: Radius.circular(24),
                bottomRight: Radius.circular(24),
              ),
              boxShadow: [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 12,
                  offset: Offset(0, 4),
                ),
              ],
              middle: CategorySearchField(),
            ),
          ),
        ),
      ),
    );

    final topBar = find.byType(CategoryTopBar);
    final overlay = tester.widget<AnnotatedRegion<SystemUiOverlayStyle>>(
      find.descendant(
        of: topBar,
        matching: find.byType(AnnotatedRegion<SystemUiOverlayStyle>),
      ),
    );
    expect(overlay.value, AppColors.brandTopOverlay);

    final brandedSurfaces = tester
        .widgetList<DecoratedBox>(
          find.descendant(of: topBar, matching: find.byType(DecoratedBox)),
        )
        .where((surface) {
          final decoration = surface.decoration;
          return decoration is BoxDecoration &&
              decoration.color == AppColors.primary;
        })
        .toList();
    expect(brandedSurfaces, hasLength(1));

    final decoration = brandedSurfaces.single.decoration as BoxDecoration;
    expect(decoration.border, isNull);
    expect(
      decoration.borderRadius,
      const BorderRadius.only(
        bottomLeft: Radius.circular(24),
        bottomRight: Radius.circular(24),
      ),
    );
    expect(decoration.boxShadow, isNotEmpty);
  });

  testWidgets('category top bar uses the website page-surface glass', (
    tester,
  ) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => const MaterialApp(
          home: Scaffold(body: CategoryTopBar(middle: CategorySearchField())),
        ),
      ),
    );

    final topBar = find.byType(CategoryTopBar);
    final overlayFinder = find.descendant(
      of: topBar,
      matching: find.byType(AnnotatedRegion<SystemUiOverlayStyle>),
    );
    final overlay = tester.widget<AnnotatedRegion<SystemUiOverlayStyle>>(
      overlayFinder,
    );
    expect(overlay.value, AppColors.topBarOverlay);
    expect(
      find.descendant(of: topBar, matching: find.byType(BackdropFilter)),
      findsOneWidget,
    );

    final glassSurfaces = tester
        .widgetList<DecoratedBox>(
          find.descendant(of: topBar, matching: find.byType(DecoratedBox)),
        )
        .where((surface) {
          final decoration = surface.decoration;
          return decoration is BoxDecoration &&
              decoration.color ==
                  AppColors.topBarBackground.withValues(
                    alpha: HkGlass.topBarSurfaceOpacity,
                  );
        })
        .toList();
    expect(glassSurfaces, hasLength(1));

    final decoration = glassSurfaces.single.decoration as BoxDecoration;
    final border = decoration.border! as Border;
    expect(border.bottom.color, AppColors.borderSubtle);

    final chromeIconThemes = tester
        .widgetList<IconTheme>(
          find.descendant(of: topBar, matching: find.byType(IconTheme)),
        )
        .where((theme) => theme.data.color == AppColors.chromeForeground);
    expect(chromeIconThemes, isNotEmpty);
  });

  for (final width in <double>[320, 390, 768]) {
    testWidgets('top chrome geometry stays stable at ${width.toInt()}px', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(width, 844);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => const MaterialApp(
            home: Scaffold(
              body: CategoryTopBar(
                leading: SizedBox.square(dimension: HkControl.minTap),
                middle: CategorySearchField(),
                actions: [
                  SizedBox.square(dimension: HkControl.minTap),
                  SizedBox.square(dimension: HkControl.minTap),
                ],
              ),
            ),
          ),
        ),
      );

      expect(tester.takeException(), isNull);
      expect(
        tester.getSize(find.byType(CategoryTopBar)).height,
        HkControl.topBarHeight,
      );
      expect(
        tester.getSize(find.byType(CategorySearchField)).height,
        HkControl.minTap,
      );
    });
  }
}
