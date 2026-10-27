import 'dart:ui' show SemanticsAction, SemanticsActionEvent, Tristate;

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/cart/presentation/widgets/temperature_tags.dart';
import 'package:hawkr_buyer/shared/widgets/hk_temperature_tag.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'known API values and labels share the Web colors; unknown stays raw',
    () {
      for (final value in ['3201', 'ambient', '常溫']) {
        expect(HkTemperatureTag.labelFor(value), '常溫');
        expect(HkTemperatureTag.colorFor(value), AppColors.temperatureAmbient);
      }
      for (final value in ['3202', 'CHILLED', '冷藏']) {
        expect(HkTemperatureTag.labelFor(value), '冷藏');
        expect(HkTemperatureTag.colorFor(value), AppColors.temperatureChilled);
      }
      for (final value in ['3203', 'FROZEN', '冷凍']) {
        expect(HkTemperatureTag.labelFor(value), '冷凍');
        expect(HkTemperatureTag.colorFor(value), AppColors.temperatureFrozen);
      }
      expect(HkTemperatureTag.labelFor('未提供的溫層'), '未提供的溫層');
      expect(HkTemperatureTag.colorFor('9999'), AppColors.temperatureAmbient);
    },
  );

  for (final scale in [1.0, 1.4, 2.0]) {
    testWidgets('tags grow at $scale text scale without becoming buttons', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(320, 600);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.light,
          home: Scaffold(
            body: MediaQuery(
              data: MediaQueryData(textScaler: TextScaler.linear(scale)),
              child: const Wrap(
                spacing: 4,
                runSpacing: 4,
                children: [
                  HkTemperatureTag(value: '3201'),
                  HkTemperatureTag(value: '3202'),
                  HkTemperatureTag(value: '3203'),
                  HkTemperatureTag(value: '未知'),
                  HkTemperatureTag(value: ' '),
                ],
              ),
            ),
          ),
        ),
      );
      final text = tester.widget<Text>(find.text('冷藏'));
      expect(text.style!.fontSize, 11);
      expect(text.style!.fontWeight, FontWeight.w700);
      expect(text.style!.letterSpacing, 0);
      final node = tester.getSemantics(find.text('冷藏'));
      expect(node.getSemanticsData().flagsCollection.isButton, isFalse);
      expect(node.getSemanticsData().hasAction(SemanticsAction.tap), isFalse);
      expect(
        tester.getSize(find.widgetWithText(HkTemperatureTag, '冷藏')).height,
        closeTo(16 * scale + 2, 1),
      );
      final box = tester.widget<Container>(
        find.descendant(
          of: find.widgetWithText(HkTemperatureTag, '冷藏'),
          matching: find.byType(Container),
        ),
      );
      final decoration = box.decoration! as BoxDecoration;
      expect(decoration.color, isNull);
      expect(
        (decoration.border! as Border).top,
        const BorderSide(color: AppColors.temperatureChilled),
      );
      expect(find.text(' '), findsNothing);
      expect(tester.takeException(), isNull);
      semantics.dispose();
    });
  }

  testWidgets('cart tags use provided SKU data without a default temperature', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              buildTemperatureTags(1, {
                1: ['3202', '9999'],
              }),
              buildTemperatureTags(2, {
                1: ['3203'],
              }),
            ],
          ),
        ),
      ),
    );
    expect(find.byType(HkTemperatureTag), findsNWidgets(2));
    expect(find.text('冷藏'), findsOneWidget);
    expect(find.text('9999'), findsOneWidget);
    expect(find.text('冷凍'), findsNothing);
  });

  testWidgets(
    'temperature choice uses the full 48dp hit area and one semantic action',
    (tester) async {
      var taps = 0;
      final semantics = tester.ensureSemantics();
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.light,
          home: Scaffold(
            body: Align(
              alignment: Alignment.topLeft,
              child: TemperatureChoiceChip(
                code: '3202',
                label: '冷藏',
                selected: true,
                onTap: () => taps++,
              ),
            ),
          ),
        ),
      );
      final chip = find.byType(TemperatureChoiceChip);
      final rect = tester.getRect(chip);
      expect(rect.height, greaterThanOrEqualTo(48));
      expect(rect.width, greaterThanOrEqualTo(48));
      for (final point in [
        rect.topLeft + const Offset(2, 2),
        rect.center,
        rect.bottomRight - const Offset(2, 2),
      ]) {
        await tester.tapAt(point);
        await tester.pump();
      }
      expect(taps, 3);
      final node = tester.getSemantics(find.bySemanticsLabel('冷藏配送溫層'));
      expect(
        node.getSemanticsData().flagsCollection.isSelected == Tristate.isTrue,
        isTrue,
      );
      tester.binding.performSemanticsAction(
        SemanticsActionEvent(
          type: SemanticsAction.tap,
          nodeId: node.id,
          viewId: tester.view.viewId,
        ),
      );
      await tester.pump();
      expect(taps, 4);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(taps, 5);
      semantics.dispose();
    },
  );
}
