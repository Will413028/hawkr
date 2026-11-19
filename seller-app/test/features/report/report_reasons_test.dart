import 'package:hawkr_seller/features/report/data/report_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('reportReasonsFor', () {
    test('USER 不含 FOOD_SAFETY', () {
      final reasons = reportReasonsFor(ReportTargetType.user);
      expect(reasons.contains(ReportReasonCode.foodSafety), isFalse);
      expect(reasons.length, ReportReasonCode.values.length - 1);
    });

    test('其餘對象含全部原因', () {
      for (final t in ReportTargetType.values) {
        if (t == ReportTargetType.user) continue;
        expect(reportReasonsFor(t).length, ReportReasonCode.values.length);
      }
    });
  });
}
