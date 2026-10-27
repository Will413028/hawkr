import 'package:hawkr_buyer/features/moderation/data/report_target_type.dart';
import 'package:hawkr_buyer/features/moderation/presentation/report_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/production_layout_test_harness.dart';

void main() {
  testWidgets(
    'other reason and keyboard keep report CTA reachable at 320dp and 1.4x',
    (tester) async {
      await pumpProductionLayoutTestApp(
        tester,
        size: const Size(320, 568),
        systemTextScale: 2,
        viewPadding: const EdgeInsets.only(top: 24, bottom: 16),
        viewInsets: const EdgeInsets.only(bottom: 260),
        home: ProviderScope(
          child: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                onPressed: () => showReportSheet(
                  context,
                  targetType: ReportTargetType.post,
                  targetId: 'post-1',
                ),
                child: const Text('開啟檢舉'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('開啟檢舉'));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);

      final reasonsScroll = find.byKey(const ValueKey('report-reasons-scroll'));
      await tester.scrollUntilVisible(
        find.text('其他（請說明）'),
        120,
        scrollable: find.descendant(
          of: reasonsScroll,
          matching: find.byType(Scrollable),
        ),
      );
      await tester.tap(find.text('其他（請說明）'));
      await tester.pumpAndSettle();
      expect(find.byType(TextField), findsOneWidget);

      await tester.enterText(find.byType(TextField), '這是一段完整的補充說明');
      await tester.pump();

      final submit = find.byKey(const ValueKey('report-submit'));
      expect(submit, findsOneWidget);
      expect(tester.getSize(submit).height, greaterThanOrEqualTo(48));
      expect(submit.hitTestable(), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}
