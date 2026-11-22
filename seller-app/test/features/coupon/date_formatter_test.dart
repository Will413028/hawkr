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
}
