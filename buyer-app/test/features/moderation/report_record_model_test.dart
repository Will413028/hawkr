import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/moderation/data/report_record_model.dart';

void main() {
  group('ReportRecord.fromJson', () {
    test('完整欄位（number 型別的 ms 欄位）', () {
      final r = ReportRecord.fromJson({
        'id': '7234567890123456789',
        'targetType': 'POST',
        'targetId': '111',
        'reasonCode': 'SPAM',
        'description': '廣告內容',
        'status': 'PENDING',
        'resolvedAction': null,
        'resolvedBy': null,
        'resolvedAtMs': null,
        'createTimeMs': 1721800000000,
        'recentActionCount': 2,
      });
      expect(r.id, '7234567890123456789');
      expect(r.targetType, 'POST');
      expect(r.targetId, '111');
      expect(r.reasonCode, 'SPAM');
      expect(r.description, '廣告內容');
      expect(r.status, 'PENDING');
      expect(r.resolvedAtMs, isNull);
      expect(r.createTimeMs, 1721800000000);
      expect(r.recentActionCount, 2);
    });

    test('ms 欄位為 String 時仍能容錯解析', () {
      final r = ReportRecord.fromJson({
        'id': '1',
        'targetType': 'REVIEW',
        'targetId': '2',
        'reasonCode': 'FRAUD',
        'status': 'RESOLVED',
        'resolvedAtMs': '1721800000000',
        'createTimeMs': '1721700000000',
      });
      expect(r.resolvedAtMs, 1721800000000);
      expect(r.createTimeMs, 1721700000000);
    });

    test('缺省欄位維持 null / 0 預設值', () {
      final r = ReportRecord.fromJson({
        'id': '1',
        'targetType': 'USER',
        'targetId': '2',
        'reasonCode': 'OTHER',
        'status': 'PENDING',
      });
      expect(r.description, isNull);
      expect(r.resolvedAction, isNull);
      expect(r.resolvedBy, isNull);
      expect(r.resolvedAtMs, isNull);
      expect(r.createTimeMs, 0);
      expect(r.recentActionCount, 0);
    });
  });
}
