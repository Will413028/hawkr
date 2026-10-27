import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:ui' show SemanticsAction, Tristate;

import 'package:hawkr_buyer/shared/widgets/quantity_stepper.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('uses canonical 32dp circles inside 48dp labeled actions', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    var count = 2;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StatefulBuilder(
            builder: (context, setState) => Center(
              child: QuantityStepper(
                count: count,
                onIncrement: () => setState(() => count += 1),
                onDecrement: () => setState(() => count -= 1),
              ),
            ),
          ),
        ),
      ),
    );

    final decrement = find.bySemanticsLabel('減少數量');
    final increment = find.bySemanticsLabel('增加數量');
    expect(tester.getSize(decrement), const Size.square(48));
    expect(tester.getSize(increment), const Size.square(48));
    for (final symbol in <String>['minus', 'plus']) {
      expect(
        tester.getSize(find.byKey(ValueKey('quantity-stepper-$symbol-visual'))),
        const Size.square(32),
      );
    }

    final minusGlyph = find.byKey(
      const ValueKey('quantity-stepper-minus-glyph'),
    );
    final plusGlyph = find.byKey(const ValueKey('quantity-stepper-plus-glyph'));
    expect(tester.getSize(minusGlyph), const Size.square(18));
    expect(tester.getSize(plusGlyph), const Size.square(18));
    expect(
      tester
          .widget<Icon>(
            find.descendant(of: minusGlyph, matching: find.byType(Icon)),
          )
          .icon,
      AppIcons.remove,
    );
    expect(
      tester
          .widget<Icon>(
            find.descendant(of: plusGlyph, matching: find.byType(Icon)),
          )
          .icon,
      AppIcons.add,
    );
    expect(find.byKey(const ValueKey('quantity-stepper-frame')), findsNothing);
    final defaultCount = tester.widget<Text>(find.text('2')).style!;
    expect((defaultCount.fontSize, defaultCount.height), (20, 1));

    await tester.tap(increment);
    await tester.pump();
    expect(find.text('3'), findsOneWidget);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('large text and four-digit count grow without wrapping', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 640);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MediaQuery(
        data: const MediaQueryData(textScaler: TextScaler.linear(1.4)),
        child: MaterialApp(
          home: Scaffold(
            body: Center(
              child: QuantityStepper(
                count: 1108,
                maxCount: 9999,
                onIncrement: _noop,
                onDecrement: _noop,
              ),
            ),
          ),
        ),
      ),
    );

    expect(
      tester
          .getSize(find.byKey(const ValueKey('quantity-stepper-count')))
          .width,
      greaterThan(32),
    );
    expect(
      tester.renderObject<RenderParagraph>(find.text('1108')).didExceedMaxLines,
      isFalse,
    );
    expect(
      tester.getSize(find.bySemanticsLabel('增加數量')),
      const Size.square(48),
    );
    expect(
      tester.getSize(
        find.byKey(const ValueKey('quantity-stepper-plus-visual')),
      ),
      const Size.square(32),
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('stacked large count wraps above accessible actions', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      var increments = 0;
      var decrements = 0;
      await tester.pumpWidget(
        MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: const TextScaler.linear(1.4)),
            child: child!,
          ),
          home: Scaffold(
            body: Center(
              child: SizedBox(
                width: 160,
                child: QuantityStepper(
                  count: 999999999,
                  stacked: true,
                  onIncrement: () => increments++,
                  onDecrement: () => decrements++,
                ),
              ),
            ),
          ),
        ),
      );
      final count = find.byKey(const ValueKey('quantity-stepper-count'));
      final minus = find.bySemanticsLabel('減少數量');
      final plus = find.bySemanticsLabel('增加數量');
      expect(tester.getSize(plus), const Size.square(48));
      expect(tester.getSize(minus), const Size.square(48));
      expect(tester.getRect(count).bottom, lessThan(tester.getRect(plus).top));
      expect(tester.getSize(count).width, lessThanOrEqualTo(160));
      expect(
        tester
            .renderObject<RenderParagraph>(find.text('999999999'))
            .didExceedMaxLines,
        isFalse,
      );
      expect(
        tester
            .getSemantics(find.bySemanticsLabel('數量'))
            .getSemanticsData()
            .value,
        '999999999',
      );
      await tester.tap(plus);
      await tester.tap(minus);
      expect(increments, 1);
      expect(decrements, 1);
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('bounds and disabled state block actions with accurate hints', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    var increments = 0;
    var decrements = 0;

    Future<void> pump({required int count, bool disabled = false}) {
      return tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Center(
              child: QuantityStepper(
                count: count,
                minCount: 1,
                maxCount: 3,
                disabled: disabled,
                onIncrement: () => increments += 1,
                onDecrement: () => decrements += 1,
              ),
            ),
          ),
        ),
      );
    }

    await pump(count: 1);
    var minusData = tester
        .getSemantics(find.bySemanticsLabel('減少數量'))
        .getSemanticsData();
    expect(minusData.hint, '已達最小數量');
    expect(minusData.flagsCollection.isEnabled, Tristate.isFalse);
    expect(minusData.hasAction(SemanticsAction.tap), isFalse);
    await tester.tap(find.bySemanticsLabel('減少數量'));
    expect(decrements, 0);

    await pump(count: 3);
    final plusData = tester
        .getSemantics(find.bySemanticsLabel('增加數量'))
        .getSemanticsData();
    expect(plusData.hint, '已達最大數量');
    expect(plusData.flagsCollection.isEnabled, Tristate.isFalse);
    expect(plusData.hasAction(SemanticsAction.tap), isFalse);
    await tester.tap(find.bySemanticsLabel('增加數量'));
    expect(increments, 0);

    await pump(count: 2, disabled: true);
    minusData = tester
        .getSemantics(find.bySemanticsLabel('減少數量'))
        .getSemanticsData();
    expect(minusData.hint, '目前無法調整數量');
    expect(minusData.flagsCollection.isEnabled, Tristate.isFalse);
    semantics.dispose();
  });

  testWidgets('reversed order remains intentional in LTR and RTL', (
    tester,
  ) async {
    Future<void> pump({
      required TextDirection direction,
      required bool reversed,
    }) {
      return tester.pumpWidget(
        MaterialApp(
          home: Directionality(
            textDirection: direction,
            child: Scaffold(
              body: Center(
                child: QuantityStepper(
                  count: 2,
                  reversed: reversed,
                  onIncrement: _noop,
                  onDecrement: _noop,
                ),
              ),
            ),
          ),
        ),
      );
    }

    for (final direction in TextDirection.values) {
      for (final reversed in <bool>[false, true]) {
        await pump(direction: direction, reversed: reversed);
        final minusX = tester.getCenter(find.bySemanticsLabel('減少數量')).dx;
        final plusX = tester.getCenter(find.bySemanticsLabel('增加數量')).dx;
        final minusComesFirst = direction == TextDirection.ltr
            ? minusX < plusX
            : minusX > plusX;
        expect(minusComesFirst, isNot(reversed));
      }
    }
    expect(tester.takeException(), isNull);
  });
}

void _noop() {}
