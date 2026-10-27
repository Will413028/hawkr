import 'package:hawkr_seller/features/auth/presentation/widgets/auth_moon_hero.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/widgets/hk_placeholder.dart';

void main() {
  test('moon fades with dusk and dawn while rising through the night', () {
    final day = AuthMoonTimeState.at(DateTime(2026, 7, 12, 12));
    final dusk = AuthMoonTimeState.at(DateTime(2026, 7, 12, 20));
    final night = AuthMoonTimeState.at(DateTime(2026, 7, 12, 23));
    final dawn = AuthMoonTimeState.at(DateTime(2026, 7, 13, 6));

    expect(day.opacity, 0);
    expect(day.isVisible, isFalse);
    expect(dusk.opacity, closeTo(0.5, 0.001));
    expect(night.opacity, 1);
    expect(dawn.opacity, closeTo(0.5, 0.001));
    expect(dusk.riseProgress, lessThan(night.riseProgress));
    expect(night.riseProgress, lessThan(dawn.riseProgress));
  });

  testWidgets('moon keeps its slot by day and renders above it at night', (
    tester,
  ) async {
    Future<void> pumpAt(DateTime time) {
      return tester.pumpWidget(
        MaterialApp(
          home: MediaQuery(
            data: const MediaQueryData(disableAnimations: true),
            child: Center(child: AuthMoonHero(size: 200, time: time)),
          ),
        ),
      );
    }

    await pumpAt(DateTime(2026, 7, 12, 12));
    expect(find.byKey(const Key('auth-moon-slot')), findsOneWidget);
    expect(find.byKey(const Key('auth-moon-visual')), findsNothing);

    await pumpAt(DateTime(2026, 7, 12, 21));
    expect(find.byKey(const Key('auth-moon-visual')), findsOneWidget);
    expect(find.byKey(const Key('auth-moon-outer-halo')), findsOneWidget);
    expect(find.byKey(const Key('auth-moon-overexposure')), findsOneWidget);
    expect(find.byKey(const Key('auth-moon-rotation')), findsOneWidget);
    expect(find.byType(HkPlaceholder), findsOneWidget);
    expect(find.byType(Image), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
