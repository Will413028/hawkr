import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_temperature_tag.dart';
import 'package:hawkr_seller/shared/widgets/temperature_tag.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'temperature codes, enum names and labels preserve shipping meaning',
    () {
      for (final entry in {
        '3201': '常溫',
        ' ambient ': '常溫',
        '常溫': '常溫',
        '3202': '冷藏',
        'CHILLED': '冷藏',
        '冷藏': '冷藏',
        '3203': '冷凍',
        'FROZEN': '冷凍',
        '冷凍': '冷凍',
        '待確認': '待確認',
      }.entries) {
        expect(TemperatureTag.codeToLabel(entry.key), entry.value);
      }
      expect(TemperatureTag.codeToColor('3201'), AppColors.temperatureAmbient);
      expect(
        TemperatureTag.codeToColor('CHILLED'),
        AppColors.temperatureChilled,
      );
      expect(TemperatureTag.codeToColor('冷凍'), AppColors.temperatureFrozen);
      expect(
        TemperatureTag.codeToColor('future-code'),
        AppColors.temperatureAmbient,
      );
    },
  );

  for (final scale in [0.8, 1.0, 1.4, 2.0]) {
    testWidgets('Web badge keeps readable content at text scale $scale', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      await tester.pumpWidget(
        MaterialApp(
          home: MediaQuery(
            data: MediaQueryData(textScaler: TextScaler.linear(scale)),
            child: const Scaffold(
              body: Center(child: HkTemperatureTag(value: '3202')),
            ),
          ),
        ),
      );
      expect(find.text('冷藏'), findsOneWidget);
      final text = tester.widget<Text>(find.text('冷藏'));
      expect(text.style?.fontSize, 11);
      expect(text.style?.fontWeight, FontWeight.w700);
      expect(text.style?.letterSpacing, 0);
      final pill = tester.widget<Container>(
        find.descendant(
          of: find.byType(HkTemperatureTag),
          matching: find.byType(Container),
        ),
      );
      final decoration = pill.decoration! as BoxDecoration;
      expect(decoration.color, isNull);
      expect(
        decoration.border,
        Border.all(color: AppColors.temperatureChilled),
      );
      expect(decoration.borderRadius, BorderRadius.circular(999));
      expect(
        tester.getSize(find.byType(HkTemperatureTag)).height,
        closeTo(scale <= 1 ? 18 : 16 * scale + 2, 1),
      );
      final node = tester.getSemantics(find.text('冷藏'));
      expect(node.getSemanticsData().flagsCollection.isButton, isFalse);
      expect(node.getSemanticsData().hasAction(SemanticsAction.tap), isFalse);
      expect(tester.takeException(), isNull);
      semantics.dispose();
    });
  }

  testWidgets('empty data hides badge and unknown data remains visible', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              HkTemperatureTag(value: ' '),
              HkTemperatureTag(value: 'UNKNOWN'),
            ],
          ),
        ),
      ),
    );
    expect(find.text('UNKNOWN'), findsOneWidget);
    expect(find.byType(Text), findsOneWidget);
  });

  testWidgets(
    'selection has a 48dp target, one semantic action and disables safely',
    (tester) async {
      final semantics = tester.ensureSemantics();
      var taps = 0;
      Future<void> pump({required bool enabled}) => tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Center(
              child: TemperatureTag.selectable(
                code: '3203',
                isSelected: true,
                onTap: enabled ? () => taps++ : null,
              ),
            ),
          ),
        ),
      );
      await pump(enabled: true);
      final selector = find.byType(TemperatureTag);
      final rect = tester.getRect(selector);
      expect(rect.width, greaterThanOrEqualTo(48));
      expect(rect.height, greaterThanOrEqualTo(48));
      final action = find.bySemanticsLabel('配送溫層 冷凍');
      expect(action, findsOneWidget);
      expect(
        tester.getSemantics(action),
        isSemantics(
          label: '配送溫層 冷凍',
          isButton: true,
          isSelected: true,
          hasEnabledState: true,
          isEnabled: true,
          hasTapAction: true,
        ),
      );
      await tester.tapAt(Offset(rect.center.dx, rect.top + 1));
      expect(taps, 1);
      tester.binding.renderViews.first.owner!.semanticsOwner!.performAction(
        tester.getSemantics(action).id,
        SemanticsAction.tap,
      );
      expect(taps, 2);
      await pump(enabled: false);
      await tester.tap(selector);
      expect(taps, 2);
      expect(
        tester
            .getSemantics(action)
            .getSemanticsData()
            .hasAction(SemanticsAction.tap),
        isFalse,
      );
      semantics.dispose();
    },
  );
}
