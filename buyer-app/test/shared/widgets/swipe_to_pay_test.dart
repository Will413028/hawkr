import 'dart:async';
import 'dart:ui' show SemanticsAction, Tristate;

import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/shared/widgets/swipe_to_pay.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('keeps one-line copy, a 48dp target and accessible action', (
    tester,
  ) async {
    var completions = 0;
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      MaterialApp(
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: const TextScaler.linear(1.4)),
          child: child!,
        ),
        home: Scaffold(
          body: Center(
            child: SizedBox(
              width: 240,
              child: SwipeToPay(onComplete: () => completions++),
            ),
          ),
        ),
      ),
    );

    expect(tester.takeException(), isNull);
    expect(tester.getSize(find.byType(SwipeToPay)).height, 48);
    final label = tester.widget<Text>(
      find.descendant(
        of: find.byType(HkButtonLabel),
        matching: find.byType(Text),
      ),
    );
    expect(label.maxLines, 1);
    expect(label.softWrap, isFalse);
    expect(label.overflow, TextOverflow.ellipsis);

    final data = tester
        .getSemantics(find.bySemanticsLabel('向右滑結帳'))
        .getSemanticsData();
    expect(data.flagsCollection.isButton, isTrue);
    expect(data.flagsCollection.isEnabled, Tristate.isTrue);
    expect(data.hasAction(SemanticsAction.tap), isTrue);

    await tester.drag(find.byType(SwipeToPay), const Offset(300, 0));
    await tester.pump();
    expect(completions, 1);

    // A validation failure can leave the sheet open; the control must reset.
    await tester.drag(find.byType(SwipeToPay), const Offset(300, 0));
    await tester.pump();
    expect(completions, 2);
    semantics.dispose();
  });

  testWidgets('disabled swipe has no semantics action', (tester) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: SwipeToPay())),
    );

    final data = tester
        .getSemantics(find.bySemanticsLabel('向右滑結帳'))
        .getSemanticsData();
    expect(data.flagsCollection.isEnabled, Tristate.isFalse);
    expect(data.hasAction(SemanticsAction.tap), isFalse);
    semantics.dispose();
  });

  testWidgets('pending checkout cannot be triggered a second time', (
    tester,
  ) async {
    final pending = Completer<void>();
    var completions = 0;
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 240,
            child: SwipeToPay(
              onComplete: () {
                completions++;
                return pending.future;
              },
            ),
          ),
        ),
      ),
    );

    await tester.drag(find.byType(SwipeToPay), const Offset(300, 0));
    await tester.pump();
    expect(completions, 1);

    final busyData = tester
        .getSemantics(find.bySemanticsLabel('正在建立訂單'))
        .getSemanticsData();
    expect(busyData.flagsCollection.isEnabled, Tristate.isFalse);
    expect(busyData.hasAction(SemanticsAction.tap), isFalse);

    await tester.drag(find.byType(SwipeToPay), const Offset(300, 0));
    await tester.pump();
    expect(completions, 1);

    pending.complete();
    await tester.pumpAndSettle();
    expect(find.bySemanticsLabel('向右滑結帳'), findsOneWidget);
    semantics.dispose();
  });
}
