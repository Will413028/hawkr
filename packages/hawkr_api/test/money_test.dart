import 'package:hawkr_api/money.dart';
import 'package:test/test.dart';

void main() {
  test('formats with zero decimals, thousands and NT\$ prefix', () {
    expect(formatMoney(150), 'NT\$ 150');
    expect(formatMoney(150.0), 'NT\$ 150');
    expect(formatMoney(0), 'NT\$ 0');
    expect(formatMoney(1234567), 'NT\$ 1,234,567');
    expect(formatMoney(-1234), 'NT\$ -1,234');
  });

  test('rounds fractions instead of printing decimals', () {
    expect(formatMoneyAmount(99.5), '100');
    expect(formatMoneyAmount(99.4), '99');
    expect(formatMoneyAmount(12.34), '12');
    expect(formatMoneyAmount(1234.5), '1,235');
  });

  test('total is the rounded unit price times quantity', () {
    expect(moneyTotal(99.5, 3), 300);
    expect(moneyTotal(99.4, 3), 297);
  });

  test('discount compares and computes on rounded prices', () {
    expect(moneyDiscountPercent(100.4, 99.6), isNull);
    expect(moneyDiscountPercent(320, 240), 25);
    expect(moneyDiscountPercent(100.4, 50.4), 50);
    expect(moneyDiscountPercent(100, 0), isNull);
  });
}
