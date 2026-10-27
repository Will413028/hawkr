import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/codemeta/code_meta_display.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('CodeMetaDisplay', () {
    test('resolves payment method from value and code', () {
      expect(
        CodeMetaDisplay.paymentMethod(PaymentMethod.creditCard.value),
        '信用卡',
      );
      expect(
        CodeMetaDisplay.paymentMethod(PaymentMethod.creditCard.code),
        '信用卡',
      );
    });

    test('resolves supported coupon types including legacy aliases', () {
      expect(CodeMetaDisplay.couponType(CouponType.cash.value), '現金折扣');
      expect(CodeMetaDisplay.couponType(CouponType.percent.code), '百分比折扣');
      expect(CodeMetaDisplay.couponType('DISCOUNT'), '百分比折扣');
    });

    test('leaves removed coupon types unchanged', () {
      expect(CodeMetaDisplay.couponType('SHIPPING'), 'SHIPPING');
      expect(CodeMetaDisplay.couponType('FREE_SHIPPING'), 'FREE_SHIPPING');
    });

    test('resolves refund reasons from value and code', () {
      expect(
        CodeMetaDisplay.refundReason(RefundReason.defective.value),
        '商品瑕疵',
      );
      expect(CodeMetaDisplay.refundReason(RefundReason.defective.code), '商品瑕疵');
    });

    test('resolves after sale case status from value and code', () {
      expect(
        CodeMetaDisplay.afterSaleCaseStatus(AfterSaleCaseStatus.pending.value),
        '待處理',
      );
      expect(
        CodeMetaDisplay.afterSaleCaseStatus(
          AfterSaleCaseStatus.returnPending.code,
        ),
        '待退貨',
      );
    });

    test('returns raw value when code meta lookup misses', () {
      expect(CodeMetaDisplay.paymentMethod('SOMETHING_NEW'), 'SOMETHING_NEW');
    });
  });
}
