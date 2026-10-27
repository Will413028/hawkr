import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_liquid_glass.dart';
import 'package:hawkr_buyer/shared/widgets/glass_tab_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('uses white glass for the track and selected tab', (
    tester,
  ) async {
    await tester.pumpWidget(const MaterialApp(home: _GlassTabHarness()));

    final indicatorFinder = find.byKey(const Key('test-glass-indicator'));
    final initialLeft = tester.getTopLeft(indicatorFinder).dx;
    final decoration =
        tester.widget<DecoratedBox>(indicatorFinder).decoration
            as BoxDecoration;
    final track = tester.widget<HkLiquidGlassSurface>(
      find.descendant(
        of: find.byType(HkGlassTabSurface),
        matching: find.byType(HkLiquidGlassSurface),
      ),
    );

    expect(find.byType(BackdropFilter), findsOneWidget);
    expect(track.backgroundColor, AppColors.navBarGlass);
    expect(track.surfaceOpacity, HkGlass.navBarSurfaceOpacity);
    expect(track.blurSigma, HkGlass.tabBarBlurSigma);
    expect(decoration.color, isNull);
    expect(decoration.gradient, isA<LinearGradient>());
    expect(decoration.color, isNot(AppColors.primarySoft));
    expect(decoration.border, isNotNull);
    expect(decoration.boxShadow, isNotEmpty);

    final press = await tester.startGesture(tester.getCenter(find.text('訂閱')));
    await tester.pump();
    expect(
      tester
          .widget<AnimatedScale>(
            find.descendant(
              of: find.byType(HkGlassTabBar),
              matching: find.byType(AnimatedScale),
            ),
          )
          .scale,
      0.97,
    );
    await press.up();
    await tester.pumpAndSettle();

    await tester.tap(find.text('收藏'));
    await tester.pumpAndSettle();

    expect(tester.getTopLeft(indicatorFinder).dx, greaterThan(initialLeft));
    final selectedLabelStyle = tester
        .widgetList<AnimatedDefaultTextStyle>(
          find.descendant(
            of: find.byType(HkGlassTabBar),
            matching: find.byType(AnimatedDefaultTextStyle),
          ),
        )
        .map((widget) => widget.style)
        .singleWhere((style) => style.color == AppColors.textEmphasis);
    expect(selectedLabelStyle.color, AppColors.textEmphasis);
    expect(selectedLabelStyle.fontWeight, FontWeight.w700);
  });

  testWidgets('keeps every tab hit target at least 48dp', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: _GlassTabHarness()));

    expect(tester.getSize(find.byType(HkGlassTabBar)).height, 52);
    expect(
      tester.getSize(find.byType(HkLiquidGlassSurface)).height,
      52 - HkGlass.tabVerticalInset * 2,
    );

    for (var index = 0; index < 3; index += 1) {
      final target = find.ancestor(
        of: find.byKey(Key('glass_tab_label_$index')),
        matching: find.byType(InkWell),
      );
      final size = tester.getSize(target);
      expect(size.width, greaterThanOrEqualTo(HkControl.minTap));
      expect(size.height, greaterThanOrEqualTo(HkControl.minTap));
    }

    final indicatorFinder = find.byKey(const Key('test-glass-indicator'));
    final initialLeft = tester.getTopLeft(indicatorFinder).dx;
    final secondTabRect = tester.getRect(
      find.ancestor(
        of: find.byKey(const Key('glass_tab_label_1')),
        matching: find.byType(InkWell),
      ),
    );
    await tester.tapAt(Offset(secondTabRect.center.dx, secondTabRect.top + 1));
    await tester.pumpAndSettle();

    expect(tester.getTopLeft(indicatorFinder).dx, greaterThan(initialLeft));
    expect(tester.takeException(), isNull);
  });

  testWidgets('clamps a short requested height to the 48dp tap minimum', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(home: _GlassTabHarness(height: 40)),
    );

    expect(tester.getSize(find.byType(HkGlassTabBar)).height, HkControl.minTap);
    for (var index = 0; index < 3; index += 1) {
      expect(
        tester
            .getSize(
              find.ancestor(
                of: find.byKey(Key('glass_tab_label_$index')),
                matching: find.byType(InkWell),
              ),
            )
            .height,
        HkControl.minTap,
      );
    }
    expect(tester.takeException(), isNull);
  });
}

class _GlassTabHarness extends StatefulWidget {
  const _GlassTabHarness({this.height = 52});

  final double height;

  @override
  State<_GlassTabHarness> createState() => _GlassTabHarnessState();
}

class _GlassTabHarnessState extends State<_GlassTabHarness> {
  int selectedIndex = 0;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Align(
        alignment: Alignment.topCenter,
        child: SizedBox(
          width: 390,
          child: HkGlassTabBar(
            labels: const ['訂閱', '收藏', '瀏覽'],
            selectedIndex: selectedIndex,
            onTap: (index) => setState(() => selectedIndex = index),
            indicatorKey: const Key('test-glass-indicator'),
            height: widget.height,
          ),
        ),
      ),
    );
  }
}
