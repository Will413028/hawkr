import 'dart:io';
import 'dart:ui' show PointerDeviceKind, SemanticsAction;

import 'package:hawkr_buyer/features/auth/presentation/widgets/auth_back_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('auth back visual is wrapped by the shared accessible tap target', () {
    final source = File(
      'lib/features/auth/presentation/widgets/auth_back_button.dart',
    ).readAsStringSync();
    final authBackButton = source.substring(
      source.indexOf('class AuthBackButton'),
      source.length,
    );

    expect(authBackButton, contains('HkTapTarget('));
    expect(authBackButton, contains("semanticLabel: '返回繼續瀏覽'"));
    expect(authBackButton, isNot(contains('GestureDetector(')));
    expect(authBackButton, contains('FocusableActionDetector('));
    expect(authBackButton, contains('MouseRegion('));
  });

  testWidgets(
    'auth back exposes 48dp semantics and pointer-keyboard feedback',
    (tester) async {
      final semantics = tester.ensureSemantics();
      var activations = 0;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Align(
              alignment: Alignment.topLeft,
              child: AuthBackButton(onTap: () => activations += 1),
            ),
          ),
        ),
      );

      final button = find.byKey(const ValueKey('auth-back-button'));
      expect(tester.getSize(button), const Size(48, 48));
      expect(
        tester.getSize(find.byKey(const ValueKey('auth-back-visual'))),
        const Size(36, 36),
      );
      final data = tester.getSemantics(button).getSemanticsData();
      expect(data.label, '返回繼續瀏覽');
      expect(data.hasAction(SemanticsAction.tap), isTrue);

      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(location: const Offset(200, 200));
      await mouse.moveTo(tester.getCenter(button));
      await tester.pump(const Duration(milliseconds: 160));
      expect(
        tester
            .widget<AnimatedScale>(
              find.byKey(const ValueKey('auth-back-hover-motion')),
            )
            .scale,
        greaterThan(1),
      );

      final touch = await tester.startGesture(tester.getCenter(button));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 70));
      final press = tester.widget<Transform>(
        find.byKey(const ValueKey('auth-back-press-scale')),
      );
      expect(press.transform.storage[0], lessThan(1));
      await touch.cancel();
      await tester.pump(const Duration(milliseconds: 180));

      await mouse.moveTo(const Offset(200, 200));
      await tester.pump(const Duration(milliseconds: 160));
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(
        find.byKey(const ValueKey('auth-back-focus-ring')),
        findsOneWidget,
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(activations, 1);

      await tester.tap(button);
      await tester.pump();
      expect(activations, 2);
      await mouse.removePointer();
      semantics.dispose();
      await tester.pumpWidget(const SizedBox.shrink());
      expect(tester.takeException(), isNull);
    },
  );
}
