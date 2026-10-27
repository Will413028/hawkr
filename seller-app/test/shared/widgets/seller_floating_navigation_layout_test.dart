import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/seller_floating_navigation_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('page fills the viewport behind the floating Navbar', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      const _TestApp(
        page: ColoredBox(key: ValueKey('page-background'), color: Colors.green),
      ),
    );

    expect(
      tester.getSize(find.byKey(const ValueKey('seller-full-page-layer'))),
      const Size(390, 844),
    );
    expect(
      tester.getSize(find.byKey(const ValueKey('page-background'))),
      const Size(390, 844),
    );
    expect(
      find.byKey(const ValueKey('seller-floating-navigation-layer')),
      findsOneWidget,
    );
  });

  testWidgets(
    'scroll down hides, scroll up and a bottom-edge swipe reveal the Navbar',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(const _TestApp(page: _ScrollablePage()));
      await tester.pump(const Duration(milliseconds: 901));

      final scrollable = find.byKey(const ValueKey('auto-hide-scrollable'));
      await tester.timedDrag(
        scrollable,
        const Offset(0, -220),
        const Duration(milliseconds: 320),
      );
      await tester.pump();

      expect(_slide(tester).offset.dy, greaterThan(1));
      expect(_opacity(tester).opacity, 0);

      await tester.drag(scrollable, const Offset(0, 80));
      await tester.pump();

      expect(_slide(tester).offset, Offset.zero);
      expect(_opacity(tester).opacity, 1);

      await tester.pump(const Duration(milliseconds: 161));
      await tester.drag(scrollable, const Offset(0, -100));
      await tester.pump();
      expect(_slide(tester).offset.dy, greaterThan(1));

      final revealGesture = await tester.startGesture(const Offset(195, 838));
      await revealGesture.moveBy(const Offset(0, -40));
      await tester.pump();

      expect(_slide(tester).offset, Offset.zero);
      expect(_opacity(tester).opacity, 1);
      await revealGesture.up();
    },
  );

  testWidgets('route or tab change restores the Navbar without rebuilding it', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    var resetKey = 'dashboard';
    late StateSetter rebuild;
    await tester.pumpWidget(
      StatefulBuilder(
        builder: (context, setState) {
          rebuild = setState;
          return _TestApp(resetKey: resetKey, page: const _ScrollablePage());
        },
      ),
    );
    await tester.pump(const Duration(milliseconds: 901));
    await tester.timedDrag(
      find.byKey(const ValueKey('auto-hide-scrollable')),
      const Offset(0, -220),
      const Duration(milliseconds: 320),
    );
    await tester.pump();
    expect(_slide(tester).offset.dy, greaterThan(1));

    final originalNavigation = tester.element(
      find.byKey(const ValueKey('test-navigation')),
    );
    rebuild(() => resetKey = 'orders');
    await tester.pump();

    expect(_slide(tester).offset, Offset.zero);
    expect(
      identical(
        originalNavigation,
        tester.element(find.byKey(const ValueKey('test-navigation'))),
      ),
      isTrue,
    );
  });

  testWidgets('reduced motion keeps the Navbar visible while scrolling', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      const _TestApp(disableAnimations: true, page: _ScrollablePage()),
    );
    await tester.pump(const Duration(milliseconds: 901));
    await tester.drag(
      find.byKey(const ValueKey('auto-hide-scrollable')),
      const Offset(0, -220),
    );
    await tester.pump();

    expect(_slide(tester).offset, Offset.zero);
    expect(_slide(tester).duration, Duration.zero);
    expect(_opacity(tester).opacity, 1);
  });
}

AnimatedSlide _slide(WidgetTester tester) => tester.widget<AnimatedSlide>(
  find.byKey(const ValueKey('seller-bottom-nav-auto-hide-slide')),
);

AnimatedOpacity _opacity(WidgetTester tester) => tester.widget<AnimatedOpacity>(
  find.byKey(const ValueKey('seller-bottom-nav-auto-hide-opacity')),
);

class _TestApp extends StatelessWidget {
  const _TestApp({
    required this.page,
    this.resetKey = 'dashboard',
    this.disableAnimations = false,
  });

  final Widget page;
  final String resetKey;
  final bool disableAnimations;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(
          context,
        ).copyWith(disableAnimations: disableAnimations),
        child: child!,
      ),
      home: SellerFloatingNavigationLayout(
        visibilityResetKey: resetKey,
        page: page,
        navigation: const SizedBox(
          key: ValueKey('test-navigation'),
          height: HkControl.navHeight,
          child: ColoredBox(color: Colors.white),
        ),
      ),
    );
  }
}

class _ScrollablePage extends StatelessWidget {
  const _ScrollablePage();

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      key: const ValueKey('auto-hide-scrollable'),
      itemExtent: 80,
      itemCount: 40,
      itemBuilder: (context, index) => Text('item $index'),
    );
  }
}
