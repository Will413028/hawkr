import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/moderation/data/report_block_codes.dart';

void main() {
  group('reportBlockErrorMessage', () {
    test('9202（int）回傳封鎖提示', () {
      expect(
        reportBlockErrorMessage(ReportBlockCode.blockedMessageNotAllowed),
        '對方已封鎖您，或您已封鎖對方，無法傳送訊息',
      );
    });

    test('9210（int）回傳內容審核提示', () {
      expect(
        reportBlockErrorMessage(ReportBlockCode.moderationRejected),
        '訊息內容包含不當字詞，請修改後重試',
      );
    });

    test('9202（String，手寫 HTTP 路徑常見型別）同樣命中', () {
      expect(reportBlockErrorMessage('9202'), isNotNull);
    });

    test('9210（num，非 int 的數值型別）同樣命中', () {
      expect(reportBlockErrorMessage(9210.0), isNotNull);
    });

    test('未知 code 回傳 null，讓呼叫端 fallback', () {
      expect(reportBlockErrorMessage(4090), isNull);
      expect(reportBlockErrorMessage(null), isNull);
      expect(reportBlockErrorMessage('not-a-number'), isNull);
    });
  });
}
