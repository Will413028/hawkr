import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_chrome_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_liquid_glass.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('delegates bounded chrome geometry to Liquid Glass', (
    tester,
  ) async {
    const surfaceKey = ValueKey('bounded-chrome-surface');
    const radius = BorderRadius.all(Radius.circular(20));

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: SizedBox(
              width: 320,
              child: HkGlassChromeSurface(
                height: 48,
                padding: EdgeInsets.symmetric(horizontal: 12),
                backgroundColor: AppColors.neutralWhite,
                surfaceOpacity: 0.88,
                borderRadius: radius,
                surfaceKey: surfaceKey,
                child: Text('導覽'),
              ),
            ),
          ),
        ),
      ),
    );

    final liquid = tester.widget<HkLiquidGlassSurface>(
      find.byType(HkLiquidGlassSurface),
    );
    expect(liquid.variant, HkLiquidGlassVariant.regular);
    expect(liquid.backgroundColor, AppColors.neutralWhite);
    expect(liquid.surfaceOpacity, 0.88);
    expect(liquid.blurSigma, HkGlass.topBarBlurSigma);
    expect(liquid.borderRadius, radius);
    expect(liquid.showBorder, isTrue);
    expect(liquid.width, double.infinity);
    expect(liquid.height, 48);
    expect(tester.getSize(find.byKey(surfaceKey)), const Size(320, 48));
    expect(
      tester.widget<BackdropFilter>(find.byType(BackdropFilter)).enabled,
      isTrue,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('keeps compact chrome shrink-wrapped in an unbounded row', (
    tester,
  ) async {
    const surfaceKey = ValueKey('compact-chrome-surface');

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                HkGlassChromeSurface(
                  height: 48,
                  showBottomBorder: false,
                  surfaceKey: surfaceKey,
                  child: SizedBox(width: 80),
                ),
              ],
            ),
          ),
        ),
      ),
    );

    final liquid = tester.widget<HkLiquidGlassSurface>(
      find.byType(HkLiquidGlassSurface),
    );
    expect(liquid.width, isNull);
    expect(liquid.showBorder, isFalse);
    expect(tester.getSize(find.byKey(surfaceKey)), const Size(80, 48));
    expect(tester.takeException(), isNull);
  });

  testWidgets('high contrast disables blur and strengthens the bottom edge', (
    tester,
  ) async {
    const surfaceKey = ValueKey('accessible-chrome-surface');

    await tester.pumpWidget(
      MaterialApp(
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(context).copyWith(highContrast: true),
          child: child!,
        ),
        home: const Scaffold(
          body: SizedBox(
            width: 320,
            child: HkGlassChromeSurface(
              height: 56,
              surfaceKey: surfaceKey,
              child: Text('高對比導覽'),
            ),
          ),
        ),
      ),
    );

    expect(
      tester.widget<BackdropFilter>(find.byType(BackdropFilter)).enabled,
      isFalse,
    );

    final decoration =
        tester.widget<DecoratedBox>(find.byKey(surfaceKey)).decoration
            as BoxDecoration;
    expect(decoration.gradient, isNull);
    expect(
      decoration.color?.a,
      closeTo(HkGlass.accessibleSurfaceOpacity, 0.001),
    );
    final border = decoration.border! as Border;
    expect(border.bottom.color, AppColors.borderStrong);
    expect(tester.takeException(), isNull);
  });

  testWidgets('high contrast overrides a supplied weak glass border', (
    tester,
  ) async {
    const surfaceKey = ValueKey('custom-border-glass');

    await tester.pumpWidget(
      MaterialApp(
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(context).copyWith(highContrast: true),
          child: child!,
        ),
        home: const Scaffold(
          body: HkLiquidGlassSurface(
            surfaceKey: surfaceKey,
            border: Border(
              bottom: BorderSide(color: AppColors.borderSubtle, width: 0.5),
            ),
            child: Text('高對比內容'),
          ),
        ),
      ),
    );

    final decoration =
        tester.widget<DecoratedBox>(find.byKey(surfaceKey)).decoration
            as BoxDecoration;
    final border = decoration.border! as Border;
    expect(border.top, BorderSide.none);
    expect(border.bottom.color, AppColors.borderStrong);
    expect(border.bottom.width, 1);
  });
}
