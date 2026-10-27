import 'dart:io';

import 'package:hawkr_seller/features/auth/presentation/widgets/auth_entrance_sequence.dart';
import 'package:flutter/material.dart';
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
    return tester
        .widget<Opacity>(
          find.ancestor(of: transform, matching: find.byType(Opacity)).first,
        )
        .opacity;
  }

  testWidgets('entrance stages share one ordered 840ms timeline', (
    tester,
  ) async {
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

  test('auth motion builders do not allocate unmanaged CurvedAnimation', () {
    for (final path in const [
      'lib/features/auth/presentation/widgets/auth_entrance_sequence.dart',
    ]) {
      expect(
        File(path).readAsStringSync(),
        isNot(contains('CurvedAnimation(')),
        reason: '$path must use the shared managed transition',
      );
    }
  });
}
