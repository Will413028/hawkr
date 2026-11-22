import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/order/presentation/order_card_widgets/order_info_column.dart';

void main() {
  test('seconds and milliseconds epoch show the same Taipei time', () {
    // 2026-07-30T16:30:00Z is 2026/07/31 00:30 in Taipei.
    const seconds = 'ts=1785429000';
    const millis = 'ts=1785429000000';
    expect(formatOrderInfoDate(seconds), '2026/07/31 00:30');
    expect(formatOrderInfoDate(millis), '2026/07/31 00:30');
  });

  test('ISO with offset shows Taipei time; empty stays empty', () {
    expect(formatOrderInfoDate('2026-07-30T16:30:00Z'), '2026/07/31 00:30');
    expect(formatOrderInfoDate(''), '');
  });
}
