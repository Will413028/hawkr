import 'dart:ui' show CheckedState, Tristate;

import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/glass_tab_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('glass tab bar exposes labeled tabs and changes selection', (
    tester,
  ) async {
    var selectedIndex = 0;
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StatefulBuilder(
            builder: (context, setState) => HkGlassTabBar(
              labels: const ['貼文', '商城', '留言'],
              icons: const [
                HkIconName.post,
                HkIconName.market,
                HkIconName.chat,
              ],
              selectedIndex: selectedIndex,
              onTap: (index) => setState(() => selectedIndex = index),
            ),
          ),
        ),
      ),
    );

    expect(find.bySemanticsLabel('貼文頁籤'), findsOneWidget);
    expect(find.bySemanticsLabel('商城頁籤'), findsOneWidget);
    expect(find.bySemanticsLabel('留言頁籤'), findsOneWidget);
    expect(find.byType(BackdropFilter), findsOneWidget);
    expect(find.byType(HkIcon), findsNWidgets(3));
    expect(tester.getSize(find.byType(HkGlassTabBar)).height, 52);
    for (final label in const ['貼文頁籤', '商城頁籤', '留言頁籤']) {
      final size = tester.getSize(find.bySemanticsLabel(label));
      expect(
        size.height,
        greaterThanOrEqualTo(HkControl.minTap),
        reason: '$label 的實際點擊區高度應至少為 48 logical pixels',
      );
      expect(
        size.width,
        greaterThanOrEqualTo(HkControl.minTap),
        reason: '$label 的實際點擊區寬度應至少為 48 logical pixels',
      );
    }

    final marketTarget = tester.getRect(find.bySemanticsLabel('商城頁籤'));
    await tester.tapAt(Offset(marketTarget.center.dx, marketTarget.top + 1));
    await tester.pumpAndSettle();

    expect(selectedIndex, 1);
    final indicator = tester.widget<HkGlassTabIndicator>(
      find.byType(HkGlassTabIndicator),
    );
    expect(indicator.dragging, isFalse);

    semantics.dispose();
  });

  testWidgets('undersized height is clamped to an accessible tab target', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: HkGlassTabBar(
            labels: const ['小', '中', '大'],
            selectedIndex: 1,
            height: 40,
            onTap: (_) {},
          ),
        ),
      ),
    );

    for (final label in const ['小頁籤', '中頁籤', '大頁籤']) {
      expect(
        tester.getSize(find.bySemanticsLabel(label)).height,
        greaterThanOrEqualTo(HkControl.minTap),
      );
    }
    expect(tester.takeException(), isNull);

    semantics.dispose();
  });

  testWidgets('icon tabs do not overflow on a compact large-text phone', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 720);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: const TextScaler.linear(2)),
          child: child!,
        ),
        home: Scaffold(
          body: HkGlassTabBar(
            labels: const ['貼文', '商城', '留言'],
            icons: const [HkIconName.post, HkIconName.market, HkIconName.chat],
            selectedIndex: 0,
            onTap: (_) {},
          ),
        ),
      ),
    );

    expect(find.text('貼文'), findsOneWidget);
    expect(find.text('商城'), findsOneWidget);
    expect(find.text('留言'), findsOneWidget);
    expect(tester.takeException(), isNull);

    tester.view.physicalSize = const Size(720, 320);
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('value selector exposes selected radio semantics', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: HkGlassTabBar(
              labels: const ['小', '中', '大'],
              selectedIndex: 1,
              mutuallyExclusiveSemantics: true,
              onTap: (_) {},
            ),
          ),
        ),
      );

      final selected = tester
          .getSemantics(find.bySemanticsLabel('中'))
          .getSemanticsData()
          .flagsCollection;
      final unselected = tester
          .getSemantics(find.bySemanticsLabel('小'))
          .getSemanticsData()
          .flagsCollection;
      expect(selected.isInMutuallyExclusiveGroup, isTrue);
      expect(unselected.isInMutuallyExclusiveGroup, isTrue);
      expect(selected.isSelected, Tristate.isTrue);
      expect(unselected.isSelected, Tristate.isFalse);
      expect(selected.isChecked, CheckedState.none);
    } finally {
      semantics.dispose();
    }
  });
}
