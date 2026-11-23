import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/coupon/presentation/widgets/date_formatter.dart';

void main() {
  test('formatToYYYYMMDD：UTC 16:00 後是臺北隔天（不跟著裝置時區）', () {
    expect(
      DateFormatter.formatToYYYYMMDD('2026-04-10T16:00:00Z'),
      '2026/04/11',
    );
    expect(
      DateFormatter.formatToYYYYMMDD('2026-04-10T15:59:00Z'),
      '2026/04/10',
    );
  });

  test('formatToYYYYMMDD：沒有時區標記的日期原樣顯示', () {
    expect(DateFormatter.formatToYYYYMMDD('2026-04-10'), '2026/04/10');
  });

  test('isExpired：以臺北的日曆日比較', () {
    expect(DateFormatter.isExpired('2000-01-01T00:00:00Z'), isTrue);
    expect(DateFormatter.isExpired('2999-01-01T00:00:00Z'), isFalse);
  });

  test('tryFormatToYYYYMMDD：解析失敗或空值是 null，formatToYYYYMMDD 保留原文', () {
    expect(
      DateFormatter.tryFormatToYYYYMMDD('2026-07-30T17:00:00Z'),
      '2026/07/31',
    );
    expect(DateFormatter.tryFormatToYYYYMMDD('not a date'), isNull);
    expect(DateFormatter.tryFormatToYYYYMMDD(''), isNull);
    expect(DateFormatter.tryFormatToYYYYMMDD(null), isNull);
    expect(DateFormatter.formatToYYYYMMDD('not a date'), 'not a date');
    expect(DateFormatter.formatToYYYYMMDD(null), '');
  });
}
