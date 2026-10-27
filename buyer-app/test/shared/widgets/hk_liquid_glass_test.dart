import 'dart:io';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_liquid_glass.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('regular glass uses a bounded, continuously clipped blur', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Center(
            child: HkLiquidGlassSurface(
              surfaceKey: ValueKey('glass-surface'),
              width: 180,
              height: 72,
              child: Text('玻璃工具列'),
            ),
          ),
        ),
      ),
    );

    final filter = tester.widget<BackdropFilter>(find.byType(BackdropFilter));
    final decoration =
        tester
                .widget<DecoratedBox>(
                  find.byKey(const ValueKey('glass-surface')),
                )
                .decoration
            as BoxDecoration;

    expect(filter.enabled, isTrue);
    expect(filter.filter, isNull);
    expect(filter.filterConfig, isNotNull);
    expect(find.byType(ClipRSuperellipse), findsOneWidget);
    expect(find.byType(RepaintBoundary), findsWidgets);
    expect(decoration.color?.a, closeTo(HkGlass.regularSurfaceOpacity, 0.001));
    expect(decoration.gradient, isA<LinearGradient>());

    final source = File(
      'lib/shared/widgets/hk_liquid_glass.dart',
    ).readAsStringSync();
    expect(source, contains('ImageFilterConfig.blur('));
    expect(source, contains('bounded: true'));
  });

  testWidgets('opt-in glass precomposes its material beneath the highlight', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: HkLiquidGlassSurface(
            variant: HkLiquidGlassVariant.clear,
            surfaceOpacity: HkGlass.regularSurfaceOpacity,
            paintSurfaceColorBeneathGradient: true,
            surfaceKey: ValueKey('layered-glass'),
            child: Icon(Icons.favorite),
          ),
        ),
      ),
    );

    final decoration =
        tester
                .widget<DecoratedBox>(
                  find.byKey(const ValueKey('layered-glass')),
                )
                .decoration
            as BoxDecoration;
    final gradient = decoration.gradient! as LinearGradient;
    final baseFill = AppColors.neutralWhite.withValues(
      alpha: HkGlass.regularSurfaceOpacity,
    );

    expect(decoration.color, isNull);
    expect(gradient.colors, <Color>[
      Color.alphaBlend(
        AppColors.neutralWhite.withValues(alpha: HkGlass.clearSpecularOpacity),
        baseFill,
      ),
      Color.alphaBlend(
        AppColors.neutralWhite.withValues(alpha: 0.08),
        baseFill,
      ),
      Color.alphaBlend(
        AppColors.actionPrimary.withValues(alpha: 0.025),
        baseFill,
      ),
    ]);
    expect(gradient.colors.first.a, closeTo(0.8492, 0.001));
    expect(gradient.colors.last.a, greaterThan(0.74));
  });

  testWidgets(
    'high contrast removes blur and uses an opaque readable surface',
    (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: MediaQuery(
            data: const MediaQueryData(highContrast: true),
            child: const Scaffold(
              body: HkLiquidGlassSurface(
                paintSurfaceColorBeneathGradient: true,
                surfaceKey: ValueKey('accessible-glass'),
                child: Text('內容'),
              ),
            ),
          ),
        ),
      );

      final filter = tester.widget<BackdropFilter>(find.byType(BackdropFilter));
      final decoration =
          tester
                  .widget<DecoratedBox>(
                    find.byKey(const ValueKey('accessible-glass')),
                  )
                  .decoration
              as BoxDecoration;
      final border = decoration.border! as Border;

      expect(filter.enabled, isFalse);
      expect(
        decoration.color?.a,
        closeTo(HkGlass.accessibleSurfaceOpacity, 0.001),
      );
      expect(decoration.gradient, isNull);
      expect(border.top.color, AppColors.borderStrong);
    },
  );

  testWidgets(
    'high contrast strengthens a supplied border without adding new edges',
    (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: MediaQuery(
            data: const MediaQueryData(highContrast: true),
            child: const Scaffold(
              body: HkLiquidGlassSurface(
                surfaceKey: ValueKey('custom-border-glass'),
                border: Border(
                  bottom: BorderSide(color: AppColors.borderSubtle, width: 0.5),
                ),
                child: Text('內容'),
              ),
            ),
          ),
        ),
      );

      final decoration =
          tester
                  .widget<DecoratedBox>(
                    find.byKey(const ValueKey('custom-border-glass')),
                  )
                  .decoration
              as BoxDecoration;
      final border = decoration.border! as Border;

      expect(border.top, BorderSide.none);
      expect(border.bottom.color, AppColors.borderStrong);
      expect(border.bottom.width, 1);
    },
  );

  testWidgets('media glass action keeps 48pt target and 44pt lens', (
    tester,
  ) async {
    var taps = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Center(
            child: HkLiquidGlassIconButton(
              semanticLabel: '收藏商品',
              onPressed: () => taps++,
              surfaceKey: const ValueKey('glass-lens'),
              child: const Icon(Icons.favorite_border, size: 24),
            ),
          ),
        ),
      ),
    );

    expect(
      tester.getSize(find.bySemanticsLabel('收藏商品')),
      const Size.square(HkControl.minTap),
    );
    expect(
      tester.getSize(find.byKey(const ValueKey('glass-lens'))),
      const Size.square(44),
    );

    final gesture = await tester.startGesture(
      tester.getCenter(find.bySemanticsLabel('收藏商品')),
    );
    await tester.pump();
    expect(
      tester
          .widget<AnimatedScale>(
            find.byKey(const ValueKey('hk-liquid-glass-control-scale')),
          )
          .scale,
      0.94,
    );
    await gesture.up();
    await tester.pumpAndSettle();
    expect(taps, 1);
  });

  testWidgets('media glass action snaps when animations are disabled', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: const MediaQueryData(disableAnimations: true),
          child: Scaffold(
            body: HkLiquidGlassIconButton(
              semanticLabel: '分享商品',
              onPressed: _noop,
              child: const Icon(Icons.share, size: 24),
            ),
          ),
        ),
      ),
    );

    final gesture = await tester.startGesture(
      tester.getCenter(find.bySemanticsLabel('分享商品')),
    );
    await tester.pump();
    final scale = tester.widget<AnimatedScale>(
      find.byKey(const ValueKey('hk-liquid-glass-control-scale')),
    );
    expect(scale.scale, 1);
    expect(scale.duration, Duration.zero);
    await gesture.up();
  });
}

void _noop() {}
