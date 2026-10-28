import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/product/presentation/product_setup_spec_screen.dart';

void main() {
  group('filterOnboardedShippingOptions', () {
    test('lists every onboarded option the server returned, in order', () {
      final out = filterOnboardedShippingOptions(
        [
          {'shippingMethod': '3001', 'fee': '100'},
          {'shippingMethod': '3002', 'fee': '60'},
        ],
        {'3001', '3002'},
      );
      expect(out.map((o) => o['shippingMethod']), ['3001', '3002']);
    });

    test('does not filter by temperature even if the payload carries one', () {
      final out = filterOnboardedShippingOptions(
        [
          {'shippingMethod': '3001', 'temperature': '3202'},
        ],
        {'3001'},
      );
      expect(out, hasLength(1));
    });

    test('drops methods the seller has not onboarded', () {
      final out = filterOnboardedShippingOptions(
        [
          {'shippingMethod': '3001'},
          {'shippingMethod': '3002'},
        ],
        {'3002'},
      );
      expect(out.single['shippingMethod'], '3002');
    });
  });
}
