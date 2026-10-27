import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_choice_chip.dart';

void main() {
  Future<void> pump(WidgetTester tester, {required bool selected}) {
    return tester.pumpWidget(
      MaterialApp(
        home: Center(
          child: HkChoiceChip(
            label: '已過期',
            selected: selected,
            icon: AppIcons.checkCircleOutline,
            selectedIcon: AppIcons.checkCircle,
            onTap: () {},
          ),
        ),
      ),
    );
  }

  testWidgets('unselected chip shows the outlined icon', (tester) async {
    await pump(tester, selected: false);

    expect(
      tester.widget<Icon>(find.byType(Icon)).icon,
      AppIcons.checkCircleOutline,
    );
    expect(find.text('已過期'), findsOneWidget);
  });

  testWidgets('selected chip swaps to the filled icon', (tester) async {
    await pump(tester, selected: true);

    expect(tester.widget<Icon>(find.byType(Icon)).icon, AppIcons.checkCircle);
    expect(
      tester.getSemantics(find.byType(HkChoiceChip)),
      matchesSemantics(
        isButton: true,
        isSelected: true,
        hasSelectedState: true,
        label: '已過期',
        hasTapAction: true,
        hasFocusAction: true,
        isFocusable: true,
      ),
    );
  });

  testWidgets('chip without an icon draws only its label', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Center(
          child: HkChoiceChip(label: '全部', selected: false, onTap: () {}),
        ),
      ),
    );

    expect(find.byType(Icon), findsNothing);
  });
}
