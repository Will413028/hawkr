import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_card.dart';

void main() {
  group('looksLikeCard', () {
    test('type=card 明示卡片 → true', () {
      expect(looksLikeCard('{"type":"card","title":"x"}'), isTrue);
    });

    test('含 orders 陣列 → true', () {
      expect(looksLikeCard('{"orders":[{"id":1}]}'), isTrue);
    });

    test('回歸：orderId / order_id key（embedded 舊版漏抓）→ true', () {
      // title + orderId / order_id 這組是統一前 embedded 版會漏判的 drift
      expect(looksLikeCard('{"title":"訂單","orderId":"A1"}'), isTrue);
      expect(looksLikeCard('{"title":"訂單","order_id":"A1"}'), isTrue);
    });

    test('回歸：title + desc（embedded 舊版漏抓）→ true', () {
      expect(looksLikeCard('{"title":"商品","desc":"說明"}'), isTrue);
    });

    test('card 包在 card 欄位裡也能解 → true', () {
      expect(looksLikeCard('{"card":{"type":"card","title":"x"}}'), isTrue);
    });

    test('純文字（非 JSON）→ false', () {
      expect(looksLikeCard('請問還有現貨嗎？'), isFalse);
    });

    test('一般 JSON 但無卡片特徵 → false', () {
      expect(looksLikeCard('{"text":"hi"}'), isFalse);
    });

    test('壞掉 / 截斷的 JSON → false（不丟例外）', () {
      expect(looksLikeCard('{"title":"x","order'), isFalse);
    });
  });
}
