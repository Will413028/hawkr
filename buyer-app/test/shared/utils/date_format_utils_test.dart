import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/shared/utils/date_format_utils.dart';

void main() {
  group('DateFormatUtils.formatDateTime', () {
    test('UTC 時間點顯示成臺北時間，不跟著裝置時區（ADR D5）', () {
      expect(
        DateFormatUtils.formatDateTime('2026-04-10T08:30:00Z'),
        '2026/04/10 16:30',
      );
    });

    test('UTC 16:00 後臺北已跨日', () {
      expect(
        DateFormatUtils.formatDateTime('2026-04-10T16:00:00Z'),
        '2026/04/11 00:00',
      );
    });

    test('沒有時區標記的字串已是臺北牆上時間，原樣顯示', () {
      expect(
        DateFormatUtils.formatDateTime('2026-04-10 08:30:00'),
        '2026/04/10 08:30',
      );
    });

    test('解析失敗原樣回傳，空字串回空', () {
      expect(DateFormatUtils.formatDateTime('not-a-date'), 'not-a-date');
      expect(DateFormatUtils.formatDateTime(''), '');
    });
  });
}
