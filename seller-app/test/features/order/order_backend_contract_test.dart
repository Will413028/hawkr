import 'package:hawkr_seller/features/order/data/after_sale_model.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Order backend contract', () {
    test(
      'maps processingDeadline into the existing auto-cancel display slot',
      () {
        final order = OrderSummary.fromJson({
          'orderNo': 'HK202605070001',
          'status': '20',
          'productName': '高山高麗菜',
          'processingDeadline': '2026-05-10T15:59:59Z',
        });

        expect(order.id, 'HK202605070001');
        expect(order.statusCode, 20);
        expect(order.autoCancelAt, '2026-05-10T15:59:59Z');
      },
    );

    test('keeps cancellation and 7-11 label fields from backend response', () {
      // Gateway 的 json tag 一律 camelCase（`handler/order.go`）。
      // 這個 fixture 原本用 snake_case（validation_no / cancel_reason / …），
      // 驗到的是 App 自己那層 snake fallback 而不是後端契約——後端從來
      // 沒有回過那些鍵。改成契約實際形狀後，這條才真的在守契約。
      final order = OrderSummary.fromJson({
        'orderNo': 'HK202605070002',
        'status': '80',
        'odno': '12345678',
        'validationNo': '9999',
        'cancelReason': 'PROCESSING_TIMEOUT',
        'cancelRemark': '系統自動取消：逾期未出貨',
        'cancelByRole': 'SYSTEM',
      });

      expect(order.odno, '12345678');
      expect(order.validationNo, '9999');
      expect(order.cancelReason, 'PROCESSING_TIMEOUT');
      expect(order.cancelRemark, '系統自動取消：逾期未出貨');
      expect(order.cancelByRole, 'SYSTEM');
    });

    test('snake_case 變體不再被接受（後端不回那些鍵）', () {
      final order = OrderSummary.fromJson({
        'orderNo': 'HK202605070003',
        'status': '80',
        'validation_no': '9999',
        'cancel_reason': 'PROCESSING_TIMEOUT',
        'cancel_by_role': 'SYSTEM',
        'shipping_status': 'SHIPPED',
        'recipient_name': '王小明',
      });

      expect(order.validationNo, isEmpty);
      expect(order.cancelReason, isEmpty);
      expect(order.cancelByRole, isEmpty);
      expect(order.shippingStatus, isEmpty);
      expect(order.recipientName, isEmpty);
    });
  });

  group('After sale backend contract', () {
    test('parses refund task fields from snake case', () {
      final afterSale = AfterSaleCase.fromJson({
        'caseNo': 'AS202605070001',
        'orderNo': 'HK202605070001',
        'refund_task_status': 'DONE',
        'refund_failure_message': '',
        'estimated_refund_date': '2026-05-12',
        'bank_date': '2026-05-13',
        'provider_status': 'S000',
        'provider_message': 'success',
      });

      expect(afterSale.refundTaskStatus, 'DONE');
      expect(afterSale.refundFailureMessage, '');
      expect(afterSale.estimatedRefundDate, '2026-05-12');
      expect(afterSale.bankDate, '2026-05-13');
      expect(afterSale.providerStatus, 'S000');
      expect(afterSale.providerMessage, 'success');
    });
  });
}
