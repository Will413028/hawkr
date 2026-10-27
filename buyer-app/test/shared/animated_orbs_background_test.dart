import 'package:hawkr_buyer/shared/widgets/animated_orbs_background.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<void> pumpBackground(WidgetTester tester, DateTime time) {
    return tester.pumpWidget(
      MaterialApp(
        home: SizedBox.expand(
          child: AnimatedOrbsBackground(
            currentTime: time,
            enableAnimation: false,
            leafCount: 2,
            child: const SizedBox.expand(),
          ),
        ),
      ),
    );
  }

  testWidgets('falling leaves are not rendered during full night', (
    tester,
  ) async {
    await pumpBackground(tester, DateTime(2026, 7, 12, 12));
    expect(find.byKey(const ValueKey('seasonal-leaves-layer')), findsOneWidget);

    await pumpBackground(tester, DateTime(2026, 7, 12, 22));
    expect(find.byKey(const ValueKey('seasonal-leaves-layer')), findsNothing);
    expect(tester.takeException(), isNull);
  });

  group('elapsed-time motion', () {
    test('preserves one legacy 60 Hz motion step', () {
      const legacyFallPerFrame = 0.0005;
      const legacyRotationPerFrame = 0.006;
      const legacyWindPerFrame = 0.0004;
      const legacyWindTargetPerFrame = 0.0008;
      const dtSeconds = 1 / 60;

      final wind = advanceAnimatedOrbsWind(
        velocityPerSecond: legacyWindPerFrame * 60,
        targetVelocityPerSecond: legacyWindTargetPerFrame * 60,
        dtSeconds: dtSeconds,
      );
      final expectedWindPerFrame =
          legacyWindPerFrame +
          (legacyWindTargetPerFrame - legacyWindPerFrame) * 0.02;
      expect(wind.velocityPerSecond, closeTo(expectedWindPerFrame * 60, 1e-12));
      expect(wind.displacement, closeTo(expectedWindPerFrame, 1e-12));

      final leaf = advanceAnimatedOrbsLeaf(
        x: 0.25,
        y: 0.5,
        rotation: 1,
        fallSpeedPerSecond: legacyFallPerFrame * 60,
        rotationSpeedPerSecond: legacyRotationPerFrame * 60,
        gravityX: 0.2,
        gravityY: 0.9,
        windDisplacement: wind.displacement,
        dtSeconds: dtSeconds,
      );
      expect(
        leaf.x,
        closeTo(0.25 + legacyFallPerFrame * 0.2 + expectedWindPerFrame, 1e-12),
      );
      expect(leaf.y, closeTo(0.5 + legacyFallPerFrame * 0.9, 1e-12));
      expect(leaf.rotation, closeTo(1 + legacyRotationPerFrame, 1e-12));
    });

    test('60 Hz and 120 Hz reach the same wall-clock motion state', () {
      final at60Hz = _simulateMotion(refreshRate: 60);
      final at120Hz = _simulateMotion(refreshRate: 120);

      expect(at60Hz.elapsedSeconds, closeTo(4, 1e-12));
      expect(at120Hz.elapsedSeconds, closeTo(4, 1e-12));
      expect(at120Hz.x, closeTo(at60Hz.x, 1e-10));
      expect(at120Hz.y, closeTo(at60Hz.y, 1e-10));
      expect(at120Hz.rotation, closeTo(at60Hz.rotation, 1e-10));
      expect(
        at120Hz.windVelocityPerSecond,
        closeTo(at60Hz.windVelocityPerSecond, 1e-10),
      );
      expect(
        at120Hz.gustCooldownSeconds,
        closeTo(at60Hz.gustCooldownSeconds, 1e-10),
      );
    });

    test('starts a fresh timeline and clamps a long resume delta', () {
      expect(animatedOrbsFrameDeltaSeconds(null, Duration.zero), 0);
      expect(
        animatedOrbsFrameDeltaSeconds(
          const Duration(milliseconds: 20),
          const Duration(milliseconds: 10),
        ),
        0,
      );
      expect(
        animatedOrbsFrameDeltaSeconds(
          const Duration(milliseconds: 16),
          const Duration(seconds: 5),
        ),
        closeTo(animatedOrbsMaxFrameDeltaSeconds, 1e-12),
      );
    });

    test('keeps the existing gust interval in wall-clock seconds', () {
      expect(animatedOrbsGustIntervalSeconds(0), 5);
      expect(animatedOrbsGustIntervalSeconds(0.5), 9.5);
      expect(animatedOrbsGustIntervalSeconds(1), 14);
    });
  });
}

({
  double x,
  double y,
  double rotation,
  double elapsedSeconds,
  double windVelocityPerSecond,
  double gustCooldownSeconds,
})
_simulateMotion({required int refreshRate}) {
  const durationSeconds = 4;
  var previousElapsed = Duration.zero;
  var x = 0.2;
  var y = 0.3;
  var rotation = 0.4;
  var elapsedSeconds = 0.0;
  var gustCooldownSeconds = 9.0;
  var windVelocityPerSecond = 0.01;

  for (var frame = 1; frame <= refreshRate * durationSeconds; frame++) {
    final elapsed = Duration(
      microseconds: (frame * Duration.microsecondsPerSecond / refreshRate)
          .round(),
    );
    final dtSeconds = animatedOrbsFrameDeltaSeconds(previousElapsed, elapsed);
    previousElapsed = elapsed;

    final wind = advanceAnimatedOrbsWind(
      velocityPerSecond: windVelocityPerSecond,
      targetVelocityPerSecond: 0.04,
      dtSeconds: dtSeconds,
    );
    windVelocityPerSecond = wind.velocityPerSecond;
    final leaf = advanceAnimatedOrbsLeaf(
      x: x,
      y: y,
      rotation: rotation,
      fallSpeedPerSecond: 0.018,
      rotationSpeedPerSecond: 0.42,
      gravityX: 0.15,
      gravityY: 0.98,
      windDisplacement: wind.displacement,
      dtSeconds: dtSeconds,
    );
    x = leaf.x;
    y = leaf.y;
    rotation = leaf.rotation;
    elapsedSeconds += dtSeconds;
    gustCooldownSeconds -= dtSeconds;
  }

  return (
    x: x,
    y: y,
    rotation: rotation,
    elapsedSeconds: elapsedSeconds,
    windVelocityPerSecond: windVelocityPerSecond,
    gustCooldownSeconds: gustCooldownSeconds,
  );
}
