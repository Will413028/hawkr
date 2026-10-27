import 'dart:io';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('website brand literals stay centralized in AppColors', () {
    final violations = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      if (entity.path.endsWith('core/theme/app_colors.dart')) continue;
      if (entity.path.endsWith('core/theme/hk_tokens.dart')) continue;
      if (RegExp(
        r'0xFF(?:E4572E|B8441F|8F3315|F2A541|1F2933|F5F7FA|FCEBE6|FDF1E1|F8F6EF|FFFDF7)',
        caseSensitive: false,
      ).hasMatch(entity.readAsStringSync())) {
        violations.add(entity.path);
      }
    }
    expect(
      violations,
      isEmpty,
      reason:
          'Use AppColors semantic roles instead of brand literals: $violations',
    );
  });

  test('theme contains the Hawkr website palette and roles', () {
    final source = File('lib/core/theme/app_colors.dart').readAsStringSync();
    for (final token in <String>[
      'brandPrimary = Color(0xFFE4572E)',
      'brandPrimaryStrong = Color(0xFFB8441F)',
      'brandPrimaryDeep = Color(0xFF8F3315)',
      'brandSecondary = Color(0xFFF2A541)',
      'brandInk = Color(0xFF1F2933)',
      'brandInkOnDark = Color(0xFFF5F7FA)',
      'brandPrimarySoft = Color(0xFFFCEBE6)',
      'brandSecondarySoft = Color(0xFFFDF1E1)',
      'neutralCream = Color(0xFFFFFDF7)',
      'neutralPage = Color(0xFFF8F6EF)',
      'neutralMuted = Color(0xFFF5F1E8)',
      'neutralInk = Color(0xFF191C18)',
      'neutralBody = Color(0xFF42493F)',
      'neutralMutedText = Color(0xFF5A6154)',
      'bgPrimary = surfacePage',
      'bgCard = surfaceCream',
      'topBarBackground = surfacePage',
      'navBarBackground = surfaceCream',
      'textSecondary = neutralMutedText',
      'textHint = neutralMutedText',
      'borderInput = Color(0xFFDED8CB)',
      'primary = brandPrimaryStrong',
      'memberCardSurface = Color(0xFFE2E2DC)',
      'storefrontHeroStart = Color(0xFFF0A38C)',
      'storefrontHeroEnd = Color(0xFFF6C9BC)',
      'storefrontRating = brandSecondary',
      'secondary = brandPrimaryStrong',
      'cardShadow = Color(0x121F2933)',
    ]) {
      expect(source, contains(token));
    }
  });

  test(
    'theme has no opaque colors outside website-derived surfaces and brands',
    () {
      final source = File('lib/core/theme/app_colors.dart').readAsStringSync();
      final colors = RegExp(r'Color\(0xFF([0-9A-Fa-f]{6})\)')
          .allMatches(source)
          .map((match) => match.group(1)!.toUpperCase())
          .toSet();
      const allowed = <String>{
        '000000',
        // LINE 登入按鈕（LINE 的品牌色）。
        '06C755',
        '191C18',
        '1A1F71',
        '1E5FAB',
        '1F2933',
        '2D7F50',
        '42493F',
        '51A2FF',
        '5A6154',
        '6194D0',
        '6B6B6B',
        '888888',
        '8F3315',
        '9FCCDD',
        'B8441F',
        'C8C8C8',
        'DED8CB',
        'E0C44C',
        'E2E2DC',
        'E4572E',
        'EA6021',
        'EE964B',
        'F0A38C',
        'F0F1EA',
        'F2A541',
        'F2E9D6',
        'F5F1E8',
        'F5F7FA',
        'F6C9BC',
        'F8F6EF',
        'FCEBE6',
        'FDF1E1',
        'FF3B4F',
        'FFFDF7',
        'FFFFFF',
        // ink 色階（brandInk 與 brandInkOnDark 之間的中性藍灰）。
        '161C23',
        '28333F',
        '323F4B',
        '3E4C59',
        '52606D',
        '616E7C',
        '7B8794',
        '9AA5B1',
        'CBD2D9',
        'E4E7EB',
      };
      expect(colors.difference(allowed), isEmpty);
    },
  );

  test('website chrome stays visibly layered over the page canvas', () {
    expect(AppColors.bgPrimary, AppColors.neutralPage);
    expect(AppColors.navBarBackground, AppColors.neutralCream);
    expect(AppColors.topBarBackground, AppColors.neutralPage);
    expect(AppColors.bgCard, AppColors.neutralCream);
    expect(AppColors.bgPrimary, isNot(AppColors.bgCard));
    expect(AppColors.bgPrimary, isNot(AppColors.navBarBackground));
    expect(AppColors.topBarGlass.a, lessThan(1));
    expect(AppColors.navBarGlass.a, lessThan(1));
  });

  test('rating stars use the Hawkr secondary color', () {
    expect(AppColors.storefrontRating, AppColors.brandSecondary);
    expect(AppIconColors.rating, AppColors.brandSecondary);
    expect(AppIconColors.gold, AppColors.brandSecondary);
  });

  test('text and input roles match the website semantic token layer', () {
    expect(AppColors.textEmphasis, AppColors.neutralInk);
    expect(AppColors.textPrimary, AppColors.neutralBody);
    expect(AppColors.textSecondary, AppColors.neutralMutedText);
    expect(AppColors.textHint, AppColors.neutralMutedText);
    expect(AppColors.borderInput, const Color(0xFFDED8CB));
    expect(AppColors.borderSubtle.a, closeTo(0.08, 0.002));
    expect(AppColors.borderStrong.a, closeTo(0.24, 0.001));
  });

  test('brand top overlay keeps light system icons on the brand surface', () {
    expect(AppColors.brandTopOverlay.statusBarColor, AppColors.primary);
    expect(AppColors.brandTopOverlay.statusBarIconBrightness, Brightness.light);
    expect(AppColors.brandTopOverlay.statusBarBrightness, Brightness.dark);
  });

  test('home top overlay uses the website page-surface glass', () {
    expect(
      AppColors.memberTopOverlay.statusBarColor,
      AppColors.memberCardSurface,
    );
    expect(AppColors.memberTopOverlay.statusBarIconBrightness, Brightness.dark);
    expect(AppColors.glassTopOverlay.statusBarColor?.a, 0);
    expect(AppColors.glassTopOverlay.statusBarIconBrightness, Brightness.dark);

    final mainTopBar = File(
      'lib/shared/widgets/main_top_bar.dart',
    ).readAsStringSync();
    expect(mainTopBar, contains('backgroundColor: AppColors.topBarBackground'));
    expect(
      mainTopBar,
      contains('systemOverlayStyle: AppColors.glassTopOverlay'),
    );
    expect(mainTopBar, contains('frostedGlass: true'));
  });
}
