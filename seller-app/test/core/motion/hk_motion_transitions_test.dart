import 'package:hawkr_seller/core/motion/hk_motion_transitions.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('every platform uses the Cupertino route builder (App 只發 iOS)', () {
    for (final platform in TargetPlatform.values) {
      expect(
        HkPageTransitions.theme.builders[platform],
        isA<CupertinoPageTransitionsBuilder>(),
        reason: platform.name,
      );
    }
  });

  testWidgets(
    'MaterialPageRoute uses native push and edge swipe on the test host',
    (tester) async {
      final navigatorKey = GlobalKey<NavigatorState>();
      late MaterialPageRoute<void> route;
      await tester.pumpWidget(
        MaterialApp(
          navigatorKey: navigatorKey,
          theme: ThemeData(pageTransitionsTheme: HkPageTransitions.theme),
          home: const Scaffold(body: Text('首頁')),
        ),
      );

      route = MaterialPageRoute<void>(
        settings: const RouteSettings(name: '/settings/profile'),
        builder: (_) => const Scaffold(body: Text('個人設定')),
      );
      navigatorKey.currentState!.push(route);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1));

      expect(find.byType(CupertinoPageTransition), findsWidgets);
      expect(
        route.transitionDuration,
        CupertinoRouteTransitionMixin.kTransitionDuration,
      );

      await tester.pumpAndSettle();
      expect(route.popGestureEnabled, isTrue);
      final gesture = await tester.startGesture(const Offset(1, 300));
      await gesture.moveBy(const Offset(600, 0));
      await gesture.up();
      await tester.pumpAndSettle();

      expect(find.text('首頁'), findsOneWidget);
      expect(find.text('個人設定'), findsNothing);
    },
  );

  testWidgets('fullscreen modal delegates to Cupertino transition', (
    tester,
  ) async {
    late Duration resolvedDuration;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) {
            resolvedDuration = HkMotionTransitions.durationOf(
              context,
              presentation: HkModalPresentation.fullscreenDialog,
            );
            return HkMotionTransitions.modal(
              context: context,
              animation: const AlwaysStoppedAnimation<double>(0.5),
              presentation: HkModalPresentation.fullscreenDialog,
              child: const SizedBox(key: ValueKey('fullscreen-modal-child')),
            );
          },
        ),
      ),
    );

    expect(resolvedDuration, CupertinoRouteTransitionMixin.kTransitionDuration);
    expect(find.byType(CupertinoFullscreenDialogTransition), findsOneWidget);
    expect(
      find.byKey(const ValueKey('hk-modal-motion-translate')),
      findsNothing,
    );
  });

  testWidgets('modal motion uses compositor transforms without a fade', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => HkMotionTransitions.modal(
            context: context,
            animation: const AlwaysStoppedAnimation<double>(0.5),
            child: const SizedBox(key: ValueKey('modal-child')),
          ),
        ),
      ),
    );

    expect(
      find.byKey(const ValueKey('hk-modal-motion-translate')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('hk-modal-motion-scale')), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('hk-modal-motion-translate')),
        matching: find.byType(FadeTransition),
      ),
      findsNothing,
    );
  });

  testWidgets('interrupted custom motion reverses without a transform jump', (
    tester,
  ) async {
    final controller = AnimationController(
      vsync: TestVSync(),
      duration: const Duration(seconds: 1),
    )..value = 0.42;
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(platform: TargetPlatform.iOS),
        home: Builder(
          builder: (context) => HkMotionTransitions.modal(
            context: context,
            animation: controller,
            presentation: HkModalPresentation.photoZoom,
            child: const SizedBox(key: ValueKey('interruptible-child')),
          ),
        ),
      ),
    );

    controller.forward();
    await tester.pump();
    final before = tester
        .widget<Transform>(find.byKey(const ValueKey('hk-modal-motion-scale')))
        .transform
        .storage[0];

    controller.reverse();
    await tester.pump();
    final after = tester
        .widget<Transform>(find.byKey(const ValueKey('hk-modal-motion-scale')))
        .transform
        .storage[0];

    expect(after, closeTo(before, 0.000001));
    controller.stop();
    await tester.pump();
  });

  testWidgets('reduced motion removes modal transforms', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: MediaQuery(
          data: const MediaQueryData(disableAnimations: true),
          child: Builder(
            builder: (context) => HkMotionTransitions.modal(
              context: context,
              animation: const AlwaysStoppedAnimation<double>(0),
              child: const SizedBox(key: ValueKey('reduced-modal-child')),
            ),
          ),
        ),
      ),
    );

    expect(find.byKey(const ValueKey('reduced-modal-child')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('hk-modal-motion-translate')),
      findsNothing,
    );
  });
}
