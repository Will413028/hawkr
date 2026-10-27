import 'package:flutter_svg/flutter_svg.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/auth/presentation/auth_screen.dart';
import 'package:hawkr_buyer/shared/widgets/animated_orbs_background.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget buildLogo({required double luminance}) {
    return MaterialApp(
      home: AmbientBrightnessScope(
        luminance: luminance,
        child: const Scaffold(body: AuthFooterBrandLogo(width: 200)),
      ),
    );
  }

  testWidgets('uses deep green footer logo on the bright daytime background', (
    tester,
  ) async {
    await tester.pumpWidget(buildLogo(luminance: 0.9));

    expect(find.byKey(const Key('auth-footer-forest-logo')), findsOneWidget);
    final logo = tester.widget<SvgPicture>(
      find.byKey(const Key('auth-footer-forest-logo')),
    );
    expect(
      logo.colorFilter,
      const ColorFilter.mode(AppColors.brandPrimaryStrong, BlendMode.srcIn),
    );
  });

  testWidgets('keeps the ivory footer logo on the dark nighttime background', (
    tester,
  ) async {
    await tester.pumpWidget(buildLogo(luminance: 0.2));

    expect(find.byKey(const Key('auth-footer-ivory-logo')), findsOneWidget);
    final logo = tester.widget<SvgPicture>(
      find.byKey(const Key('auth-footer-ivory-logo')),
    );
    expect(logo.colorFilter, isNull);
  });

  test('formats the utility version without adding a duplicate separator', () {
    expect(formatAuthVersion('1.0.5(1.0.5)'), 'v1.0.5');
    expect(formatAuthVersion('1.0.5(7)'), 'v1.0.5(7)');
    expect(formatAuthVersion(''), 'v—');
    expect(formatAuthVersion(null), 'v—');
  });

  testWidgets('footer copy switches to contrast-safe day and night colors', (
    tester,
  ) async {
    Future<void> pumpFooter({required double luminance}) {
      return tester.pumpWidget(
        MaterialApp(
          home: AmbientBrightnessScope(
            luminance: luminance,
            child: Scaffold(
              body: AuthFooterCopy(appVersionFuture: Future.value('1.0.5(42)')),
            ),
          ),
        ),
      );
    }

    await pumpFooter(luminance: 0.9);
    await tester.pump();
    expect(
      tester
          .widget<Text>(find.byKey(const ValueKey('auth-footer-legal')))
          .style
          ?.color,
      AppColors.neutralBody,
    );
    expect(find.text('v1.0.5(42)'), findsOneWidget);

    await pumpFooter(luminance: 0.2);
    await tester.pump();
    expect(
      tester
          .widget<Text>(find.byKey(const ValueKey('auth-footer-legal')))
          .style
          ?.color,
      AppColors.neutralCream.withValues(alpha: 0.94),
    );
  });
}
