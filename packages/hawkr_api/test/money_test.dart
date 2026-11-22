import 'package:hawkr_api/money.dart';
import 'package:test/test.dart';

void main() {
  test('formats with zero decimals and NT\$ prefix', () {
    expect(formatMoney(150), 'NT\$150');
    expect(formatMoney(150.0), 'NT\$150');
    expect(formatMoney(0), 'NT\$0');
  });

  test('rounds fractions instead of printing decimals', () {
    expect(formatMoneyAmount(99.5), '100');
    expect(formatMoneyAmount(99.4), '99');
    expect(formatMoneyAmount(12.34), '12');
  });
}
