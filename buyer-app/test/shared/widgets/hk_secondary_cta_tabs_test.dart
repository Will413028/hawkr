import 'dart:ui' show CheckedState, Tristate;

import 'package:hawkr_buyer/shared/widgets/hk_secondary_cta_tabs.dart';
import 'package:flutter/foundation.dart'
    show debugDefaultTargetPlatformOverride;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('iOS tabs expose selected mutually-exclusive semantics', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    var selectedIndex = 0;

    try {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: StatefulBuilder(
              builder: (context, setState) => HkSecondaryCtaTabs(
                labels: const ['小', '中', '大'],
                selectedIndex: selectedIndex,
                itemKeyPrefix: 'text-size',
                onSelected: (index) => setState(() => selectedIndex = index),
              ),
            ),
          ),
        ),
      );

      var small = tester
          .getSemantics(find.bySemanticsLabel('小'))
          .getSemanticsData();
      var large = tester
          .getSemantics(find.bySemanticsLabel('大'))
          .getSemanticsData();
      expect(small.flagsCollection.isSelected, Tristate.isTrue);
      expect(large.flagsCollection.isSelected, Tristate.isFalse);
      expect(small.flagsCollection.isChecked, CheckedState.none);
      expect(large.flagsCollection.isChecked, CheckedState.none);
      expect(small.flagsCollection.isInMutuallyExclusiveGroup, isTrue);
      expect(large.flagsCollection.isInMutuallyExclusiveGroup, isTrue);

      await tester.tap(find.bySemanticsLabel('大'));
      await tester.pumpAndSettle();

      small = tester
          .getSemantics(find.bySemanticsLabel('小'))
          .getSemanticsData();
      large = tester
          .getSemantics(find.bySemanticsLabel('大'))
          .getSemanticsData();
      expect(selectedIndex, 2);
      expect(small.flagsCollection.isSelected, Tristate.isFalse);
      expect(large.flagsCollection.isSelected, Tristate.isTrue);
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      semantics.dispose();
    }
  });
}
