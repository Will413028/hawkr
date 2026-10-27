import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/analytics/purchase_dedup.dart';

void main() {
  test('markPurchaseTracked returns true only the first time per orderNo', () {
    expect(markPurchaseTracked('ORD-1'), isTrue);
    expect(markPurchaseTracked('ORD-1'), isFalse);
    expect(markPurchaseTracked('ORD-2'), isTrue);
  });
}
