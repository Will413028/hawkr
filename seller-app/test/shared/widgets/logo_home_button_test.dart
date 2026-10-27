import 'package:hawkr_seller/shared/widgets/logo_home_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('uses the untinted four-color Hawkr mark', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: LogoHomeButton())),
    );

    final mark = tester.widget<SvgPicture>(
      find.byKey(const ValueKey('hawkr_four_color_logo')),
    );

    expect(
      (mark.bytesLoader as SvgAssetLoader).assetName,
      'assets/brand/logo-four-color.svg',
    );
    expect(mark.colorFilter, isNull);
    expect(
      find.ancestor(
        of: find.byKey(const ValueKey('hawkr_four_color_logo')),
        matching: find.byType(DecoratedBox),
      ),
      findsNothing,
      reason: '品牌 SVG 周圍不應再包白色圓形底',
    );
    expect(find.bySemanticsLabel('返回總覽'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
