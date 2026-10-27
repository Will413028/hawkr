import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_preview.dart';

void main() {
  group('derivePreview', () {
    test('回歸：後端截斷過的卡片 JSON 不可洩漏成原文', () {
      // 對應線上 bug：lastMsg 是被截斷、json.decode 會失敗的卡片 payload
      final preview = derivePreview(
        '{"messageType":"product","productId":"859783856743286973","product',
        '',
      );
      expect(preview, '傳送了一張卡片');
      expect(preview.contains('{'), isFalse);
    });

    test('完整 product 卡片 JSON → 商品卡片字樣', () {
      final preview = derivePreview(
        '{"messageType":"product","productId":"859783856743286973","productName":"池上米"}',
        '',
      );
      expect(preview, '傳送了一張商品卡片');
    });

    test('order 卡片 JSON → 訂單卡片字樣', () {
      expect(
        derivePreview('{"messageType":"order","orderSn":"A123"}', ''),
        '傳送了一張訂單卡片',
      );
    });

    test('type 參數為 image → 圖片字樣', () {
      expect(derivePreview('', 'image'), '傳送了一張圖片');
    });

    test('純文字訊息維持原文', () {
      expect(derivePreview('請問還有現貨嗎？', 'text'), '請問還有現貨嗎？');
    });

    test('任何 { 開頭且無法辨識的 payload 一律不顯示原文', () {
      final preview = derivePreview('{"foo":"bar","baz":123', '');
      expect(preview.startsWith('{'), isFalse);
    });
  });

  group('cardLabelForType（送出卡片時 client 端算 preview 用）', () {
    test('product → 商品卡片字樣', () {
      expect(cardLabelForType('product'), '傳送了一張商品卡片');
    });

    test('order_prev → 訂單卡片字樣', () {
      expect(cardLabelForType('order_prev'), '傳送了一張訂單卡片');
    });

    test('text → null（非卡片型別）', () {
      expect(cardLabelForType('text'), isNull);
    });
  });
}
