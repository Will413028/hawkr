import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_placeholder.dart';

typedef _Expected = ({Color background, Color foreground, IconData icon});

const Map<HkPlaceholderKind, _Expected> _expected = {
  HkPlaceholderKind.avatar: (
    background: AppColors.brandPrimarySoft,
    foreground: AppColors.brandPrimary,
    icon: AppIcons.personOutline,
  ),
  HkPlaceholderKind.storeCover: (
    background: AppColors.brandPrimaryStrong,
    foreground: AppColors.brandPrimarySoft,
    icon: AppIcons.storefrontOutlined,
  ),
  HkPlaceholderKind.photo: (
    background: AppColors.brandPrimarySoft,
    foreground: AppColors.brandPrimary,
    icon: AppIcons.imageOutlined,
  ),
  HkPlaceholderKind.hero: (
    background: AppColors.brandSecondarySoft,
    foreground: AppColors.brandSecondary,
    icon: AppIcons.nightlightOutlined,
  ),
};

Future<void> _pump(
  WidgetTester tester,
  HkPlaceholderKind kind, {
  double width = 120,
  double height = 80,
}) {
  return tester.pumpWidget(
    MaterialApp(
      home: Center(
        child: SizedBox(
          width: width,
          height: height,
          child: HkPlaceholder(kind),
        ),
      ),
    ),
  );
}

void main() {
  test('every kind has an expectation', () {
    expect(_expected.keys.toSet(), HkPlaceholderKind.values.toSet());
  });

  for (final entry in _expected.entries) {
    final kind = entry.key;
    final expected = entry.value;

    group(kind.name, () {
      testWidgets('draws one Material icon and no image', (tester) async {
        await _pump(tester, kind);

        final root = find.byType(HkPlaceholder);
        final icon = find.descendant(of: root, matching: find.byType(Icon));
        expect(icon, findsOneWidget);
        expect(tester.widget<Icon>(icon).icon, expected.icon);
        expect(tester.widget<Icon>(icon).color, expected.foreground);
        expect(
          find.descendant(of: root, matching: find.byType(Image)),
          findsNothing,
        );
        expect(
          find.descendant(of: root, matching: find.byType(SvgPicture)),
          findsNothing,
        );
      });

      testWidgets('paints the kind background token', (tester) async {
        await _pump(tester, kind);

        final fill = find.descendant(
          of: find.byType(HkPlaceholder),
          matching: find.byType(ColoredBox),
        );
        expect(
          tester.widget<ColoredBox>(fill.first).color,
          expected.background,
        );
      });

      testWidgets('fills the box it is given', (tester) async {
        await _pump(tester, kind);
        expect(tester.getSize(find.byType(HkPlaceholder)), const Size(120, 80));

        await _pump(tester, kind, width: 48, height: 48);
        expect(tester.getSize(find.byType(HkPlaceholder)), const Size(48, 48));
      });

      testWidgets('scales the icon to the shorter side', (tester) async {
        await _pump(tester, kind);
        final icon = find.descendant(
          of: find.byType(HkPlaceholder),
          matching: find.byType(Icon),
        );
        expect(tester.widget<Icon>(icon).size, 80 * 0.4);

        await _pump(tester, kind, width: 48, height: 48);
        expect(tester.widget<Icon>(icon).size, 48 * 0.4);
      });

      testWidgets('stays decorative', (tester) async {
        await _pump(tester, kind);
        expect(
          find.descendant(
            of: find.byType(HkPlaceholder),
            matching: find.byType(ExcludeSemantics),
          ),
          findsWidgets,
        );
      });
    });
  }

  group('without a height limit from the parent', () {
    for (final (name, build) in <(String, Widget Function(Widget))>[
      ('Column', (child) => Column(children: [child])),
      ('ListView', (child) => ListView(children: [child])),
    ]) {
      testWidgets('lays out inside a $name', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: build(
                const SizedBox(
                  width: 120,
                  child: HkPlaceholder(HkPlaceholderKind.photo),
                ),
              ),
            ),
          ),
        );
        expect(tester.takeException(), isNull);
        expect(find.byType(Icon), findsOneWidget);
      });
    }
  });
}
