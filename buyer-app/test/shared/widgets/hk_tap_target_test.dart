import 'dart:ui' show CheckedState, SemanticsAction, Tristate;

import 'package:hawkr_buyer/shared/widgets/hk_tap_target.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('custom icon control keeps a labeled 48dp target', (
    tester,
  ) async {
    var tapped = false;
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Center(
            child: HkTapTarget(
              semanticLabel: '測試操作',
              onTap: () => tapped = true,
              child: const SizedBox.square(
                key: ValueKey('compact-visual'),
                dimension: 18,
              ),
            ),
          ),
        ),
      ),
    );

    final size = tester.getSize(find.byType(HkTapTarget));
    expect(size.width, greaterThanOrEqualTo(48));
    expect(size.height, greaterThanOrEqualTo(48));
    expect(
      tester.getSize(find.byKey(const ValueKey('compact-visual'))),
      const Size.square(18),
    );
    expect(find.bySemanticsLabel('測試操作'), findsOneWidget);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await tester.tap(find.byType(HkTapTarget));
    expect(tapped, isTrue);
    semantics.dispose();
  });

  testWidgets('forwards checkbox, selection, hint and tooltip semantics', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: HkTapTarget(
            key: const ValueKey('selection-target'),
            semanticLabel: '選取通知',
            hint: '加入刪除清單',
            tooltip: '選取',
            checked: true,
            selected: true,
            button: false,
            onTap: () {},
            child: const Icon(Icons.check, size: 16),
          ),
        ),
      ),
    );

    final data = tester
        .getSemantics(find.byKey(const ValueKey('selection-target')))
        .getSemanticsData();
    expect(find.byType(Tooltip), findsOneWidget);
    expect(find.bySemanticsLabel('選取通知'), findsOneWidget);
    expect(data.label, '選取通知');
    expect(data.hint, '加入刪除清單');
    expect(data.tooltip, '選取');
    expect(data.flagsCollection.isChecked, CheckedState.isTrue);
    expect(data.flagsCollection.isSelected, Tristate.isTrue);
    expect(data.flagsCollection.isButton, isFalse);
    expect(data.flagsCollection.isEnabled, Tristate.isTrue);
    expect(data.hasAction(SemanticsAction.tap), isTrue);
    semantics.dispose();
  });

  testWidgets('forwards toggled state and exposes disabled controls', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              HkTapTarget(
                key: ValueKey('toggled-target'),
                semanticLabel: '刪除模式',
                toggled: true,
                onTap: _noop,
                child: Icon(Icons.delete, size: 16),
              ),
              HkTapTarget(
                key: ValueKey('disabled-target'),
                semanticLabel: '送出留言',
                child: Icon(Icons.send, size: 16),
              ),
            ],
          ),
        ),
      ),
    );

    final toggled = tester
        .getSemantics(find.byKey(const ValueKey('toggled-target')))
        .getSemanticsData();
    expect(toggled.flagsCollection.isToggled, Tristate.isTrue);
    expect(toggled.flagsCollection.isButton, isTrue);
    expect(toggled.flagsCollection.isEnabled, Tristate.isTrue);

    final disabled = tester
        .getSemantics(find.byKey(const ValueKey('disabled-target')))
        .getSemanticsData();
    expect(disabled.flagsCollection.isEnabled, Tristate.isFalse);
    expect(disabled.hasAction(SemanticsAction.tap), isFalse);
    semantics.dispose();
  });
}

void _noop() {}
