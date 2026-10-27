import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_model.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';

/// 訂單／售後的對外契約鎖。
///
/// 欄名權威＝gateway 的 `OrderSummaryResponse` / `OrderDetailResponse` /
/// `OrderItemDetailResponse` / `AfterSaleCase*Response`，全部 camelCase。
/// 每個 group 都成對：一個「新契約讀得到」＋一個「只給舊 snake 讀不到」——
/// 後者是回歸鎖，任何雙讀 fallback 復活都會讓它轉紅。
void main() {
  group('Order backend contract', () {
    test('parses review and deadline fields from the camelCase contract', () {
      final order = Order.fromJson({
        'orderNo': 'HK202605070001',
        'status': '50',
        'canReview': false,
        'reviewDeadline': '2026-05-31T15:59:59Z',
        'processingDeadline': '2026-05-10T15:59:59Z',
        'autoCompleteAt': '2026-05-11T15:59:59Z',
        'review': {'rating': '4.0', 'content': '很新鮮'},
      });

      expect(order.orderSn, 'HK202605070001');
      expect(order.canReview, isFalse);
      expect(order.reviewDeadline, '2026-05-31T15:59:59Z');
      expect(order.processingDeadline, '2026-05-10T15:59:59Z');
      expect(order.autoCompleteAt, '2026-05-11T15:59:59Z');
      // reviewRating / reviewContent 沒有頂層欄位，只能由巢狀 review 平鋪而來。
      expect(order.reviewRating, 4);
      expect(order.reviewContent, '很新鮮');
    });

    test('legacy snake_case order fields are not read back', () {
      final order = Order.fromJson({
        'orderNo': 'HK202605070002',
        'status': '50',
        'is_reviewed': true,
        'need_refund_page': true,
        'review_deadline': '2026-05-31T15:59:59Z',
        'processing_deadline': '2026-05-10T15:59:59Z',
        'auto_complete_at': '2026-05-11T15:59:59Z',
        'total_actual_amount': '1234',
        'shipping_address': '台北市大安區',
        'recipient_name': '王小明',
      });

      // isReviewed / needRefundPage 的 @Default 都是 false ——
      // 舊 snake 欄位若被讀回來就會變 true，這兩條才有鑑別力。
      expect(order.isReviewed, isFalse);
      expect(order.needRefundPage, isFalse);
      expect(order.reviewDeadline, '');
      expect(order.processingDeadline, '');
      expect(order.autoCompleteAt, '');
      expect(order.totalActualAmount, '');
      expect(order.shippingAddress, '');
      expect(order.recipientName, '');
    });

    test('parses cancellation reason fields from the camelCase contract', () {
      final order = Order.fromJson({
        'orderNo': 'HK202605070003',
        'status': '80',
        'cancelReason': '9102',
        'cancelRemark': '系統自動取消：賣家逾期未處理訂單',
        'cancelByRole': 'SYSTEM',
      });

      expect(order.cancelReason, '9102');
      expect(order.cancelRemark, '系統自動取消：賣家逾期未處理訂單');
      expect(order.cancelByRole, 'SYSTEM');
    });

    test('legacy snake_case cancellation fields are not read back', () {
      final order = Order.fromJson({
        'orderNo': 'HK202605070004',
        'status': '80',
        'cancel_reason': '9102',
        'cancel_remark': '系統自動取消：賣家逾期未處理訂單',
        'cancel_by_role': 'SYSTEM',
      });

      expect(order.cancelReason, '');
      expect(order.cancelRemark, '');
      expect(order.cancelByRole, '');
    });

    test('order item falls back from imageUrl to the bare imageKey', () {
      // 成對輸出，兩個都在：顯示直接用 URL。
      final paired = OrderItem.fromJson({
        'productName': '台灣紅豆',
        'imageKey': 'products/red-bean.jpg',
        'imageUrl': 'https://media.example/products/red-bean.jpg',
      });
      // 只有裸 key：顯示端 buildImageUrl dual-mode 吃得下，補進 imageUrl。
      final keyOnly = OrderItem.fromJson({
        'productName': '蟹味棒',
        'imageKey': 'products/crab-stick.jpg',
      });

      expect(paired.imageUrl, 'https://media.example/products/red-bean.jpg');
      expect(keyOnly.imageUrl, 'products/crab-stick.jpg');
    });

    test('legacy order item image aliases are not read back', () {
      final legacy = OrderItem.fromJson({
        'productName': '台灣紅豆',
        'image_url': 'products/red-bean.jpg',
        'image_key': 'products/red-bean.jpg',
        'productImageUrl': 'products/crab-stick.jpg',
        'productImageKey': 'products/crab-stick.jpg',
        'mainImageUrl': 'products/main.jpg',
        'thumbnailUrl': 'products/thumb.jpg',
      });

      expect(legacy.imageUrl, '');
    });
  });

  group('After sale backend contract', () {
    test('parses refund task fields from the camelCase contract', () {
      final afterSale = AfterSaleCase.fromJson({
        'caseNo': 'AS202605070001',
        'orderNo': 'HK202605070001',
        'refundTaskStatus': 'AWAITING_BUYER_REFUND_PAGE',
        'refundFailureMessage': '銀行帳號格式錯誤',
        'estimatedRefundDate': '2026-05-12',
        'bankDate': '2026-05-13',
        'providerStatus': 'S000',
        'providerMessage': 'success',
      });

      expect(afterSale.refundTaskStatus, 'AWAITING_BUYER_REFUND_PAGE');
      expect(afterSale.refundFailureMessage, '銀行帳號格式錯誤');
      expect(afterSale.estimatedRefundDate, '2026-05-12');
      expect(afterSale.bankDate, '2026-05-13');
      expect(afterSale.providerStatus, 'S000');
      expect(afterSale.providerMessage, 'success');
    });

    test('legacy snake_case refund task fields are not read back', () {
      final afterSale = AfterSaleCase.fromJson({
        'caseNo': 'AS202605070003',
        'orderNo': 'HK202605070003',
        'refund_task_status': 'AWAITING_BUYER_REFUND_PAGE',
        'refund_failure_message': '銀行帳號格式錯誤',
        'estimated_refund_date': '2026-05-12',
        'bank_date': '2026-05-13',
        'provider_status': 'S000',
        'provider_message': 'success',
      });

      expect(afterSale.refundTaskStatus, '');
      expect(afterSale.refundFailureMessage, '');
      expect(afterSale.estimatedRefundDate, '');
      expect(afterSale.bankDate, '');
      expect(afterSale.providerStatus, '');
      expect(afterSale.providerMessage, '');
    });

    test('shows 7-11 return code only for approved 7-11 return cases', () {
      final afterSale = AfterSaleCase.fromJson({
        'caseNo': 'AS202605070002',
        'orderNo': 'HK202605070002',
        'status': '20',
        'returnShippingMethod': ReturnShippingMethod.sevenEleven,
        'caseResolutionType': ResolutionType.approved,
        'partnerId': 'ABC',
        'odno': '12345678',
        'validationNo': '9999',
      });

      expect(afterSale.shouldShow711ReturnCode, isTrue);
      expect(afterSale.formatted711Code, 'ABC 12345678 9999');
    });
  });
}
