import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_chrome_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/storefront_top_bar_icon_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('storefront top action uses a brighter white glass bubble', (
    tester,
  ) async {
    var tapped = false;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StorefrontTopBarIconButton(
            semanticLabel: '聊天',
            onPressed: () => tapped = true,
            icon: HkIconName.chat,
          ),
        ),
      ),
    );

    final surface = tester.widget<HkGlassChromeSurface>(
      find.byType(HkGlassChromeSurface),
    );
    expect(surface.backgroundColor, AppColors.neutralWhite);
    expect(surface.surfaceOpacity, 0.88);
    expect(surface.height, StorefrontTopBarIconButton.visualSize);
    expect(
      tester.getSize(find.byType(StorefrontTopBarIconButton)),
      const Size.square(HkControl.minTap),
    );
    expect(tester.widget<HkIcon>(find.byType(HkIcon)).size, HkControl.icon);

    await tester.tap(find.bySemanticsLabel('聊天'));
    expect(tapped, isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets('storefront actions share a 24dp icon size', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Row(
            children: [
              StorefrontTopBarIconButton(
                semanticLabel: '通知中心',
                onPressed: () {},
                icon: HkIconName.notification,
              ),
              StorefrontTopBarIconButton(
                semanticLabel: '聊天',
                onPressed: () {},
                icon: HkIconName.chat,
              ),
            ],
          ),
        ),
      ),
    );

    expect(StorefrontTopBarIconButton.iconSize, HkControl.icon);
    expect(find.byType(HkIcon), findsNWidgets(2));
    for (final icon in tester.widgetList<HkIcon>(find.byType(HkIcon))) {
      expect(icon.size, HkControl.icon);
    }
    for (final label in ['通知中心', '聊天']) {
      expect(
        tester.getSize(find.bySemanticsLabel(label)),
        const Size.square(HkControl.minTap),
      );
    }
    expect(tester.takeException(), isNull);
  });
}
