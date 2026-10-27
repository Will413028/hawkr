import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/chat/presentation/chat_preview.dart';

void main() {
  // fixture 的 key 用 `lastMessage` / `lastMessageType`——`GET /app/chat/conversations`
  // 實際回的就是這兩個（im.proto `Conversation.last_message` /
  // `last_message_type`）。原本寫的 `lastContent` 是後端從未回過的名字，測試因此
  // 跟著假契約一起綠（hawkr_buyer#89）。
  group('chatConversationPreview / derivePreview', () {
    test('回歸：後端截斷過的 product 卡片 JSON 不可洩漏成原文', () {
      // decode 會失敗 → 落到 fallback，仍以卡片字樣呈現、不顯示原文
      final preview = chatConversationPreview({
        'lastMessage':
            '{"messageType":"product","productId":"859783856743286973","product',
      });
      expect(preview, '傳送了一張卡片');
      expect(preview.contains('{'), isFalse);
    });

    test('完整 product 卡片 JSON → 商品卡片（與 seller 對齊）', () {
      final preview = chatConversationPreview({
        'lastMessage':
            '{"messageType":"product","productId":"8597","productName":"池上米"}',
      });
      expect(preview, '傳送了一張商品卡片');
    });

    test('order 卡片 JSON → 訂單卡片（與 seller 對齊）', () {
      expect(
        derivePreview('{"messageType":"order","orderSn":"A123"}', ''),
        '傳送了一張訂單卡片',
      );
    });

    test('conversation-level type = product → 商品卡片', () {
      expect(
        chatConversationPreview({
          'lastMessageType': 'product',
          'lastMessage': '',
        }),
        '傳送了一張商品卡片',
      );
    });

    test('JSON text 欄位照常取出', () {
      expect(
        chatConversationPreview({'lastMessage': '{"text":"還有現貨嗎？"}'}),
        '還有現貨嗎？',
      );
    });

    test('JSON content 欄位也取出（seller 格式）', () {
      expect(derivePreview('{"content":"你好"}', ''), '你好');
    });

    test('↩︎ 回覆格式 → 只顯示內容', () {
      expect(derivePreview('↩︎ 引用預覽\n真正的回覆', ''), '真正的回覆');
    });

    test('「」回覆格式（seller）→ 只顯示內容', () {
      expect(derivePreview('「引用預覽」\n真正的回覆', ''), '真正的回覆');
    });

    test('純文字訊息維持原文', () {
      expect(chatConversationPreview({'lastMessage': '請問還有現貨嗎？'}), '請問還有現貨嗎？');
    });

    test('任何 { 開頭且無法辨識的 payload 一律不顯示原文', () {
      final preview = chatConversationPreview({
        'lastMessage': '{"foo":"bar","baz":123',
      });
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
