import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';

void main() {
  testWidgets('renders every semantic icon without errors', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Wrap(
          children: [for (final name in HkIconName.values) HkIcon(name)],
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    expect(find.byType(Icon), findsNWidgets(HkIconName.values.length));
  });

  testWidgets('selected swaps to the filled glyph where one exists', (
    tester,
  ) async {
    Future<IconData?> glyph(HkIconName name, {required bool selected}) async {
      await tester.pumpWidget(
        MaterialApp(home: HkIcon(name, selected: selected)),
      );
      return tester.widget<Icon>(find.byType(Icon)).icon;
    }

    expect(
      await glyph(HkIconName.home, selected: false),
      AppIcons.homeOutlined,
    );
    expect(await glyph(HkIconName.home, selected: true), AppIcons.home);
    expect(
      await glyph(HkIconName.chat, selected: false),
      AppIcons.chatBubbleOutline,
    );
    expect(await glyph(HkIconName.chat, selected: true), AppIcons.chatBubble);
    expect(
      await glyph(HkIconName.search, selected: true),
      await glyph(HkIconName.search, selected: false),
    );
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
