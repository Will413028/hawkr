import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_round_action_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('locks the round action visual and accessible target sizes', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    var tapped = false;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Center(
            child: HkRoundActionButton(
              key: const ValueKey('target'),
              semanticLabel: '增加數量',
              symbol: HkRoundActionSymbol.plus,
              onPressed: () => tapped = true,
              visualKey: const ValueKey('visual'),
              iconKey: const ValueKey('glyph'),
            ),
          ),
        ),
      ),
    );

    expect(
      tester.getSize(find.byKey(const ValueKey('target'))),
      const Size.square(48),
    );
    expect(
      tester.getSize(find.byKey(const ValueKey('visual'))),
      const Size.square(32),
    );
    final decoration =
        tester
                .widget<DecoratedBox>(find.byKey(const ValueKey('visual')))
                .decoration
            as BoxDecoration;
    expect(decoration.shape, BoxShape.circle);
    expect(decoration.border, isNull);
    expect(decoration.color, AppColors.primary);

    final glyph = find.byKey(const ValueKey('glyph'));
    expect(tester.getSize(glyph), const Size.square(18));
    final icon = tester.widget<Icon>(
      find.descendant(of: glyph, matching: find.byType(Icon)),
    );
    expect(icon.icon, AppIcons.add);
    expect(icon.size, 16);
    await tester.tap(find.bySemanticsLabel('增加數量'));
    expect(tapped, isTrue);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    semantics.dispose();
  });

  testWidgets('disabled minus keeps shape and exposes no tap action', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Center(
            child: HkRoundActionButton(
              semanticLabel: '減少數量',
              semanticHint: '已達最小數量',
              symbol: HkRoundActionSymbol.minus,
              onPressed: null,
              visualKey: ValueKey('visual'),
              iconKey: ValueKey('glyph'),
            ),
          ),
        ),
      ),
    );

    expect(
      tester.getSemantics(find.bySemanticsLabel('減少數量')),
      isSemantics(
        label: '減少數量',
        hint: '已達最小數量',
        isButton: true,
        hasEnabledState: true,
        isEnabled: false,
        hasTapAction: false,
      ),
    );
    final decoration =
        tester
                .widget<DecoratedBox>(find.byKey(const ValueKey('visual')))
                .decoration
            as BoxDecoration;
    expect(decoration.shape, BoxShape.circle);
    expect(decoration.border, isNull);
    expect(decoration.color, AppColors.gray300);
    final glyph = find.byKey(const ValueKey('glyph'));
    expect(tester.getSize(glyph), const Size.square(18));
    final icon = tester.widget<Icon>(
      find.descendant(of: glyph, matching: find.byType(Icon)),
    );
    expect(icon.icon, AppIcons.remove);
    expect(icon.size, 16);
    semantics.dispose();
  });
}
