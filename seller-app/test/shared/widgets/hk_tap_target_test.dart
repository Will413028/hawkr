import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('custom icon control keeps one labeled 48dp button target', (
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
              hint: '執行測試操作',
              tooltip: '測試操作',
              onTap: () => tapped = true,
              child: const SizedBox.square(dimension: 18),
            ),
          ),
        ),
      ),
    );

    final size = tester.getSize(find.byType(HkTapTarget));
    expect(size.width, greaterThanOrEqualTo(48));
    expect(size.height, greaterThanOrEqualTo(48));
    expect(find.bySemanticsLabel('測試操作'), findsOneWidget);
    expect(find.byTooltip('測試操作'), findsOneWidget);
    expect(
      tester.getSemantics(find.bySemanticsLabel('測試操作')),
      isSemantics(
        label: '測試操作',
        hint: '執行測試操作',
        tooltip: '測試操作',
        isButton: true,
        hasEnabledState: true,
        isEnabled: true,
        hasTapAction: true,
      ),
    );
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

    await tester.tap(find.byType(HkTapTarget));
    expect(tapped, isTrue);
    semantics.dispose();
  });

  testWidgets(
    'checkbox target exposes checked state without button semantics',
    (tester) async {
      final semantics = tester.ensureSemantics();

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Center(
              child: HkTapTarget(
                semanticLabel: '選取通知',
                checked: true,
                button: false,
                onTap: () {},
                child: const SizedBox.square(dimension: 22),
              ),
            ),
          ),
        ),
      );

      expect(find.bySemanticsLabel('選取通知'), findsOneWidget);
      expect(
        tester.getSemantics(find.bySemanticsLabel('選取通知')),
        isSemantics(
          label: '選取通知',
          hasCheckedState: true,
          isChecked: true,
          isButton: false,
          hasEnabledState: true,
          isEnabled: true,
          hasTapAction: true,
        ),
      );
      semantics.dispose();
    },
  );

  testWidgets('toggle, selection, and disabled state remain explicit', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Center(
            child: HkTapTarget(
              semanticLabel: '粗體',
              toggled: true,
              selected: true,
              child: SizedBox.square(dimension: 24),
            ),
          ),
        ),
      ),
    );

    expect(
      tester.getSemantics(find.bySemanticsLabel('粗體')),
      isSemantics(
        label: '粗體',
        hasToggledState: true,
        isToggled: true,
        hasSelectedState: true,
        isSelected: true,
        isButton: true,
        hasEnabledState: true,
        isEnabled: false,
        hasTapAction: false,
      ),
    );
    semantics.dispose();
  });
}
