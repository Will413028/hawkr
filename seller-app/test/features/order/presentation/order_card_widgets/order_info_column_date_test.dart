import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/order/presentation/order_card_widgets/order_info_column.dart';

void main() {
  test('ISO with offset shows Taipei time; empty stays empty', () {
    expect(formatOrderInfoDate('2026-07-30T16:30:00Z'), '2026/07/31 00:30');
    expect(formatOrderInfoDate(''), '');
  });

  test('unparseable text has no formatted date (so no Taipei label)', () {
    expect(tryFormatOrderInfoDate('soon'), isNull);
    expect(tryFormatOrderInfoDate(''), isNull);
    expect(formatOrderInfoDate('soon'), 'soon');
    expect(formatOrderInfoDate('ts=1785429000'), 'ts=1785429000');
  });

  test('a bare 10-digit seconds string is epoch, not a year-171238 date', () {
    expect(formatOrderInfoDate('1785429000'), '2026/07/31 00:30');
    expect(formatOrderInfoDate('1785429000000'), '2026/07/31 00:30');
  });

  test('slash dates parse as Taipei wall clock', () {
    expect(formatOrderInfoDate('2026/07/31 00:30'), '2026/07/31 00:30');
  });
}
