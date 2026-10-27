import 'package:hawkr_seller/features/product/data/product_certification_meta.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('product certification meta', () {
    test('maps certification index to value string', () {
      expect(certificationValueForIndex(0), '3701');
      expect(certificationValueForIndex(5), '3706');
    });

    test('maps legacy code to latest value string', () {
      expect(certificationValueForRaw('ORGANIC'), '3701');
      expect(certificationValueForRaw('gmp'), '3706');
    });

    test('maps value string to localized label', () {
      expect(certificationLabelForRaw('3701'), '有機農產品標章');
      expect(certificationLabelForRaw('3705'), 'ISO 認證');
    });

    test('builds certification input payload with value and certNo', () {
      expect(certificationInputForSelection(2, ' CAS-123 '), {
        'name': '3703',
        'certNo': 'CAS-123',
      });
    });
  });
}
