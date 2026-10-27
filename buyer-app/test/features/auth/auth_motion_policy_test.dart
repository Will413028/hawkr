import 'dart:io';

import 'package:hawkr_buyer/features/auth/presentation/widgets/auth_entrance_sequence.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget buildSequence({bool disableAnimations = false, bool ticker = true}) {
    return MaterialApp(
      home: MediaQuery(
        data: MediaQueryData(disableAnimations: disableAnimations),
        child: TickerMode(
          enabled: ticker,
          child: const AuthEntranceSequence(
            child: Column(
              children: [
                AuthEntranceItem(
                  stage: AuthEntranceStage.wordmark,
                  child: SizedBox(key: ValueKey('wordmark-child')),
                ),
                AuthEntranceItem(
                  stage: AuthEntranceStage.google,
                  child: SizedBox(key: ValueKey('google-child')),
                ),
                AuthEntranceItem(
                  stage: AuthEntranceStage.footer,
                  child: SizedBox(key: ValueKey('footer-child')),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  double opacityFor(WidgetTester tester, AuthEntranceStage stage) {
    final transform = find.byKey(ValueKey('auth-entrance-${stage.name}'));
    final opacity = find.ancestor(
      of: transform,
      matching: find.byType(Opacity),
    );
    return tester.widget<Opacity>(opacity.first).opacity;
  }

  testWidgets('entrance stages use one ordered 840ms timeline', (tester) async {
    await tester.pumpWidget(buildSequence());
    expect(opacityFor(tester, AuthEntranceStage.wordmark), 0);
    expect(opacityFor(tester, AuthEntranceStage.google), 0);

    await tester.pump(const Duration(milliseconds: 300));
    expect(opacityFor(tester, AuthEntranceStage.wordmark), 1);
    expect(opacityFor(tester, AuthEntranceStage.google), 0);

    await tester.pump(const Duration(milliseconds: 540));
    expect(opacityFor(tester, AuthEntranceStage.google), 1);
    expect(opacityFor(tester, AuthEntranceStage.footer), 1);
    expect(tester.takeException(), isNull);

    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('reduced motion and disabled TickerMode resolve immediately', (
    tester,
  ) async {
    for (final configuration in const [
      (disableAnimations: true, ticker: true),
      (disableAnimations: false, ticker: false),
    ]) {
      await tester.pumpWidget(
        buildSequence(
          disableAnimations: configuration.disableAnimations,
          ticker: configuration.ticker,
        ),
      );
      await tester.pumpAndSettle();
      expect(opacityFor(tester, AuthEntranceStage.wordmark), 1);
      expect(opacityFor(tester, AuthEntranceStage.google), 1);
      expect(opacityFor(tester, AuthEntranceStage.footer), 1);
      expect(tester.binding.hasScheduledFrame, isFalse);
      await tester.pumpWidget(const SizedBox.shrink());
      expect(tester.takeException(), isNull);
    }
  });

  testWidgets('hidden entrance stages ignore taps until visible', (
    tester,
  ) async {
    var taps = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: AuthEntranceSequence(
          child: AuthEntranceItem(
            stage: AuthEntranceStage.google,
            child: TextButton(
              key: const ValueKey('delayed-auth-action'),
              onPressed: () => taps += 1,
              child: const Text('繼續'),
            ),
          ),
        ),
      ),
    );

    final action = find.byKey(const ValueKey('delayed-auth-action'));
    await tester.tap(action, warnIfMissed: false);
    expect(taps, 0);

    await tester.pump(const Duration(milliseconds: 500));
    await tester.tap(action);
    expect(taps, 1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('hidden entrance stages exclude keyboard focus until visible', (
    tester,
  ) async {
    var activations = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: AuthEntranceSequence(
          child: AuthEntranceItem(
            stage: AuthEntranceStage.google,
            child: TextButton(
              key: const ValueKey('delayed-keyboard-auth-action'),
              onPressed: () => activations += 1,
              child: const Text('繼續'),
            ),
          ),
        ),
      ),
    );

    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(activations, 0);

    await tester.pump(const Duration(milliseconds: 500));
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(activations, 1);
    expect(tester.takeException(), isNull);
  });

  test('auth motion builders do not allocate unmanaged CurvedAnimation', () {
    for (final path in const [
      'lib/features/auth/presentation/widgets/auth_entrance_sequence.dart',
    ]) {
      expect(
        File(path).readAsStringSync(),
        isNot(contains('CurvedAnimation(')),
        reason: '$path must use the existing listenable without status leaks',
      );
    }
  });
}
