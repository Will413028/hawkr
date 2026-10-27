import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';
import 'package:hawkr_buyer/features/auth/presentation/widgets/auth_moon_hero.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

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
    final eveningTop = tester
        .getTopLeft(find.byKey(const Key('auth-moon-visual')))
        .dy;
    final nightOpacity = tester.widget<Opacity>(
      find.byKey(const Key('auth-moon-opacity')),
    );
    expect(nightOpacity.opacity, greaterThanOrEqualTo(0.94));
    expect(find.byKey(const Key('auth-moon-outer-halo')), findsOneWidget);
    expect(find.byKey(const Key('auth-moon-overexposure')), findsOneWidget);
    expect(find.byKey(const Key('auth-moon-rotation')), findsOneWidget);
    expect(find.byType(HkPlaceholder), findsOneWidget);
    expect(find.byType(Image), findsNothing);

    final halo = tester.widget<DecoratedBox>(
      find.byKey(const Key('auth-moon-outer-halo')),
    );
    final haloDecoration = halo.decoration as BoxDecoration;
    expect(haloDecoration.color, isNull);
    expect(haloDecoration.boxShadow, isNull);
    final haloGradient = haloDecoration.gradient as RadialGradient;
    expect(haloGradient.colors.last, Colors.transparent);
    expect(haloGradient.stops, const [0, 0.42, 0.75, 1]);

    await pumpAt(DateTime(2026, 7, 13, 5));
    final dawnTop = tester
        .getTopLeft(find.byKey(const Key('auth-moon-visual')))
        .dy;
    expect(dawnTop, lessThan(eveningTop));
    expect(tester.takeException(), isNull);
  });

  testWidgets('moon surface rotates slowly when motion is enabled', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Center(
          child: AuthMoonHero(size: 200, time: DateTime(2026, 7, 12, 23)),
        ),
      ),
    );

    final rotation = tester.widget<RotationTransition>(
      find.byKey(const Key('auth-moon-rotation')),
    );
    final initialTurns = rotation.turns.value;

    await tester.pump(const Duration(seconds: 1));

    expect(rotation.turns.value, greaterThan(initialTurns));
    expect(rotation.turns.value - initialTurns, closeTo(1 / 400, 0.0001));

    await tester.pumpWidget(const SizedBox.shrink());
  });
}
