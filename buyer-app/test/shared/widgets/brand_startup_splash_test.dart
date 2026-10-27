import 'dart:async';

import 'package:hawkr_buyer/shared/widgets/brand_startup_splash.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('keeps one splash owner until bootstrap is ready, then fades', (
    tester,
  ) async {
    final ready = Completer<Widget>();
    var completionCount = 0;

    await tester.pumpWidget(
      BrandStartupGate(
        bootstrap: () => ready.future,
        minimumDisplayDuration: Duration.zero,
        fadeDuration: const Duration(milliseconds: 100),
        onComplete: () => completionCount += 1,
      ),
    );

    expect(find.byType(BrandStartupSplash), findsOneWidget);
    expect(find.byKey(const Key('ready-application')), findsNothing);
    expect(completionCount, 0);

    ready.complete(
      const ColoredBox(key: Key('ready-application'), color: Colors.white),
    );
    await tester.pump();
    await tester.pump();

    expect(find.byType(BrandStartupSplash), findsOneWidget);
    expect(find.byKey(const Key('ready-application')), findsOneWidget);
    expect(completionCount, 0);

    // Let the post-frame bootstrap continuation start the fade controller.
    await tester.pump(const Duration(milliseconds: 1));
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump();

    expect(find.byType(BrandStartupSplash), findsNothing);
    expect(find.byKey(const Key('ready-application')), findsOneWidget);
    expect(completionCount, 1);
  });
}
