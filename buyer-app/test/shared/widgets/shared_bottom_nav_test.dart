import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/shared_bottom_nav.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final viewport in <Size>[
    const Size(320, 844),
    const Size(375, 812),
    const Size(390, 844),
    const Size(768, 1024),
    const Size(844, 390),
  ]) {
    testWidgets('bottom nav matches the web geometry at '
        '${viewport.width.toInt()}x${viewport.height.toInt()}', (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = viewport;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            home: Scaffold(
              body: const SizedBox.expand(),
              bottomNavigationBar: SharedBottomNav(
                currentIndex: 0,
                onDestinationSelected: (_) {},
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(tester.takeException(), isNull);
      for (final label in ['首頁', '市集', '錢包', '訂單', '設定']) {
        expect(find.text(label), findsOneWidget);
      }

      final surface = find.byKey(const ValueKey('buyer-bottom-nav-surface'));
      expect(
        tester.getSize(find.byType(SharedBottomNav)).height,
        lessThanOrEqualTo(HkControl.navHeight + 12),
      );
      final size = tester.getSize(surface);
      expect(size.height, HkControl.navHeight);
      expect(size.width, lessThanOrEqualTo(HkControl.bottomNavMaxWidth));
      expect(size.width, lessThanOrEqualTo(viewport.width - 24));
      expect(
        (tester.widget<Container>(surface).decoration as BoxDecoration)
            .gradient,
        isA<LinearGradient>(),
      );
      expect(
        find.descendant(
          of: find.byType(SharedBottomNav),
          matching: find.byType(BackdropFilter),
        ),
        findsOneWidget,
      );

      for (var index = 0; index < 5; index++) {
        final item = find.byKey(ValueKey('buyer-bottom-nav-item-$index'));
        expect(tester.getSize(item).height, HkControl.navItemHeight);
        expect(tester.getSize(item).width, greaterThanOrEqualTo(48));
      }
    });
  }

  testWidgets('bottom nav remains readable at 1.4x text scale', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 844);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    var selected = -1;
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: const TextScaler.linear(1.4)),
            child: child!,
          ),
          home: Scaffold(
            bottomNavigationBar: SharedBottomNav(
              currentIndex: 0,
              onDestinationSelected: (index) => selected = index,
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(tester.takeException(), isNull);
    await tester.tap(find.text('訂單'));
    expect(selected, 3);
  });

  testWidgets('each destination owns one icon, filled only when selected', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            bottomNavigationBar: SharedBottomNav(
              currentIndex: 0,
              onDestinationRequested: (_) async => true,
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(HkIcon), findsNWidgets(5));
    HkIcon iconAt(int index) => tester.widget<HkIcon>(
      find.descendant(
        of: find.byKey(ValueKey('buyer-bottom-nav-item-$index')),
        matching: find.byType(HkIcon),
      ),
    );
    IconData glyphAt(int index) => tester
        .widget<Icon>(
          find.descendant(
            of: find.byKey(ValueKey('buyer-bottom-nav-item-$index')),
            matching: find.byType(Icon),
          ),
        )
        .icon!;

    expect(iconAt(0).selected, isTrue);
    expect(glyphAt(0), AppIcons.home);
    expect(iconAt(1).selected, isFalse);
    expect(glyphAt(1), AppIcons.categoryOutlined);

    await tester.tap(find.byKey(const ValueKey('buyer-bottom-nav-item-1')));
    await tester.pump();
    expect(iconAt(0).selected, isFalse);
    expect(glyphAt(0), AppIcons.homeOutlined);
    expect(iconAt(1).selected, isTrue);
    expect(glyphAt(1), AppIcons.category);
  });

  testWidgets('dragging the magnetic pill commits the nearest destination', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    var requested = -1;
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            bottomNavigationBar: SharedBottomNav(
              currentIndex: 0,
              onDestinationRequested: (index) async {
                requested = index;
                return true;
              },
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final first = tester.getCenter(
      find.byKey(const ValueKey('buyer-bottom-nav-item-0')),
    );
    final order = tester.getCenter(
      find.byKey(const ValueKey('buyer-bottom-nav-item-3')),
    );
    final gesture = await tester.startGesture(first);
    await gesture.moveBy(const Offset(20, 0));
    await tester.pump();

    final followingIndicator = tester.getRect(
      find.byKey(const ValueKey('buyer-bottom-nav-indicator')),
    );
    final homeItem = tester.getRect(
      find.byKey(const ValueKey('buyer-bottom-nav-item-0')),
    );
    final marketItem = tester.getRect(
      find.byKey(const ValueKey('buyer-bottom-nav-item-1')),
    );
    expect(followingIndicator.center.dx, greaterThan(homeItem.center.dx + 12));
    expect(followingIndicator.center.dx, lessThan(marketItem.center.dx));

    await gesture.moveTo(order);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 180));
    final orderItem = tester.getRect(
      find.byKey(const ValueKey('buyer-bottom-nav-item-3')),
    );
    final preview = tester.getRect(
      find.byKey(const ValueKey('buyer-bottom-nav-indicator')),
    );
    expect(preview.center.dx, closeTo(orderItem.center.dx, 8));

    await gesture.up();
    await tester.pump();
    expect(requested, 3);
    await tester.pump(const Duration(milliseconds: 180));

    final indicator = tester.getRect(
      find.byKey(const ValueKey('buyer-bottom-nav-indicator')),
    );
    // The Web-matched spring is intentionally allowed a small overshoot.
    expect(indicator.center.dx, closeTo(orderItem.center.dx, 8));

    await tester.pump(const Duration(milliseconds: 1));
    await tester.tap(find.byKey(const ValueKey('buyer-bottom-nav-item-1')));
    await tester.pump();
    expect(
      requested,
      1,
      reason: 'a completed drag must not swallow the next tap',
    );
  });
}
