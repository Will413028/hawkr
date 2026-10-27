import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

void main() {
  final expected = <HkPlaceholderKind, ({IconData icon, Color bg, Color fg})>{
    HkPlaceholderKind.buyerAvatar: (
      icon: AppIcons.personOutline,
      bg: AppColors.brandPrimarySoft,
      fg: AppColors.brandPrimary,
    ),
    HkPlaceholderKind.merchantAvatar: (
      icon: AppIcons.storefrontOutlined,
      bg: AppColors.brandSecondarySoft,
      fg: AppColors.brandSecondary,
    ),
    HkPlaceholderKind.storeCover: (
      icon: AppIcons.storefrontOutlined,
      bg: AppColors.brandSecondarySoft,
      fg: AppColors.brandSecondary,
    ),
    HkPlaceholderKind.photo: (
      icon: AppIcons.imageOutlined,
      bg: AppColors.brandPrimarySoft,
      fg: AppColors.brandPrimary,
    ),
    HkPlaceholderKind.hero: (
      icon: AppIcons.nightlightOutlined,
      bg: AppColors.brandSecondarySoft,
      fg: AppColors.brandSecondary,
    ),
  };

  test('every kind has an expectation', () {
    expect(expected.keys.toSet(), HkPlaceholderKind.values.toSet());
  });

  for (final entry in expected.entries) {
    testWidgets('${entry.key.name} draws a token-colored icon and fills', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Center(
            child: SizedBox(
              width: 120,
              height: 80,
              child: HkPlaceholder(entry.key),
            ),
          ),
        ),
      );

      final root = find.byType(HkPlaceholder);
      expect(tester.getSize(root), const Size(120, 80));
      expect(
        find.descendant(of: root, matching: find.byType(Image)),
        findsNothing,
      );
      expect(
        find.descendant(of: root, matching: find.byType(SvgPicture)),
        findsNothing,
      );

      final icon = tester.widget<Icon>(
        find.descendant(of: root, matching: find.byType(Icon)),
      );
      expect(icon.icon, entry.value.icon);
      expect(icon.color, entry.value.fg);
      expect(icon.size, closeTo(80 * 0.4, 0.001));

      final fill = tester.widget<ColoredBox>(
        find.descendant(of: root, matching: find.byType(ColoredBox)).first,
      );
      expect(fill.color, entry.value.bg);
      expect(tester.getSize(find.byWidget(fill)), const Size(120, 80));
    });
  }

  testWidgets('icon scales with the shorter side', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Center(
          child: SizedBox(
            width: 50,
            height: 200,
            child: HkPlaceholder(HkPlaceholderKind.photo),
          ),
        ),
      ),
    );
    expect(tester.widget<Icon>(find.byType(Icon)).size, closeTo(20, 0.001));
  });

  testWidgets('is decorative: no semantics exposed', (tester) async {
    final handle = tester.ensureSemantics();
    await tester.pumpWidget(
      const MaterialApp(
        home: SizedBox(
          width: 40,
          height: 40,
          child: HkPlaceholder(HkPlaceholderKind.buyerAvatar),
        ),
      ),
    );
    expect(
      find.descendant(
        of: find.byType(HkPlaceholder),
        matching: find.byType(ExcludeSemantics),
      ),
      findsWidgets,
    );
    handle.dispose();
  });

  for (final axis in [Axis.vertical, Axis.horizontal]) {
    testWidgets('does not throw when unbounded along $axis', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: SizedBox(
            width: 100,
            height: 100,
            child: axis == Axis.vertical
                ? const Column(
                    children: [HkPlaceholder(HkPlaceholderKind.photo)],
                  )
                : const SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    child: Row(
                      children: [HkPlaceholder(HkPlaceholderKind.photo)],
                    ),
                  ),
          ),
        ),
      );
      expect(tester.takeException(), isNull);
      expect(find.byType(Icon), findsOneWidget);
    });
  }

  testWidgets('does not throw inside a ListView', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: SizedBox(
          width: 100,
          height: 300,
          child: ListView(
            children: const [HkPlaceholder(HkPlaceholderKind.photo)],
          ),
        ),
      ),
    );
    expect(tester.takeException(), isNull);
    expect(find.byType(Icon), findsOneWidget);
  });
}
