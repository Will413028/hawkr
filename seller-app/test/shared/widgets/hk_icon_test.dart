import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';

void main() {
  testWidgets('renders the text-size semantic icon', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: HkIcon(HkIconName.textSize)),
    );

    expect(find.byType(Icon), findsOneWidget);
  });

  testWidgets('renders every semantic icon through the shared registry', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Wrap(
          children: [for (final name in HkIconName.values) HkIcon(name)],
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    expect(
      find.byType(Icon, skipOffstage: false),
      findsNWidgets(HkIconName.values.length),
    );
    final icons = tester.widgetList<Icon>(
      find.byType(Icon, skipOffstage: false),
    );
    expect(
      icons.every((icon) => icon.icon?.fontFamily == 'MaterialIcons'),
      isTrue,
    );
    expect(icons.where((icon) => icon.icon == AppIcons.close), hasLength(1));
    expect(
      icons.where((icon) => icon.icon == AppIcons.textFields),
      hasLength(1),
    );
    expect(
      icons.where((icon) => icon.icon == AppIcons.moreHoriz),
      hasLength(1),
    );
  });

  testWidgets('selected draws a different filled glyph for navigation names', (
    tester,
  ) async {
    const navigation = [
      HkIconName.home,
      HkIconName.market,
      HkIconName.wallet,
      HkIconName.box,
      HkIconName.user,
    ];
    IconData glyph(HkIconName name, {required bool selected}) {
      return tester
          .widget<Icon>(
            find.descendant(
              of: find.byKey(ValueKey('${name.name}-$selected')),
              matching: find.byType(Icon),
            ),
          )
          .icon!;
    }

    await tester.pumpWidget(
      MaterialApp(
        home: Wrap(
          children: [
            for (final name in navigation) ...[
              HkIcon(name, key: ValueKey('${name.name}-false')),
              HkIcon(name, selected: true, key: ValueKey('${name.name}-true')),
            ],
          ],
        ),
      ),
    );

    for (final name in navigation) {
      expect(
        glyph(name, selected: true),
        isNot(glyph(name, selected: false)),
        reason: '${name.name} must differ by shape, not only by color',
      );
    }
  });

  testWidgets('inherits IconTheme size and exposes an optional label', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      const MaterialApp(
        home: Center(
          child: IconTheme(
            data: IconThemeData(size: 32, color: Colors.red),
            child: HkIcon(HkIconName.cart, semanticLabel: '購物車'),
          ),
        ),
      ),
    );

    expect(tester.getSize(find.byType(Icon)), const Size(32, 32));
    expect(find.bySemanticsLabel('購物車'), findsOneWidget);
    semantics.dispose();
  });
}
