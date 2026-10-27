import 'package:hawkr_seller/features/photo_editor/data/ai_image_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('quotaExceededFrom', () {
    test('HTTP 429 + code 4291 → 例外，訊息優先取後端 messageZh', () {
      final ex = quotaExceededFrom(429, {
        'code': 4291,
        'message': 'quota full',
        'messageZh': '本週額度已滿',
      });
      expect(ex, isA<AiQuotaExceededException>());
      expect(ex!.message, '本週額度已滿');
    });

    test('code 4291 但無中文訊息 → fallback 文案', () {
      final ex = quotaExceededFrom(429, {'code': 4291});
      expect(ex, isNotNull);
      expect(ex!.message, '本週 AI 美化次數已用完，下週一重置');
    });

    // #2272：Gateway 限流器也回 HTTP 429，envelope code 是 429。用 status 判會
    // 把「請求太頻繁」誤鎖成「本週已用完」並觸發重抓風格。
    test('HTTP 429 但 code 429（限流）→ null，不得誤判為額度用盡', () {
      expect(
        quotaExceededFrom(429, {'code': 429, 'message': 'rate limit exceeded'}),
        isNull,
      );
    });

    test('正常回應（200 / code 0）→ null，不誤判為額度用盡', () {
      expect(quotaExceededFrom(200, {'code': 0}), isNull);
      expect(quotaExceededFrom(200, {'code': 0, 'data': {}}), isNull);
    });
  });

  group('rateLimitedFrom', () {
    test('HTTP 429 + code 429 → 限流例外，用自家中文文案不透傳英文', () {
      final ex = rateLimitedFrom(429, {
        'code': 429,
        'message': 'rate limit exceeded',
      });
      expect(ex, isA<AiTransformException>());
      expect(ex!.message, '操作太頻繁，請稍候幾秒再試');
      expect(ex.retryable, isTrue);
    });

    test('HTTP 429 + code 4291（額度用完）→ null，交給 quotaExceededFrom', () {
      expect(rateLimitedFrom(429, {'code': 4291}), isNull);
    });

    test('非 429 → null', () {
      expect(rateLimitedFrom(409, {'code': 9250}), isNull);
      expect(rateLimitedFrom(200, {'code': 0}), isNull);
    });
  });

  group('transformedImageBase64 只認契約鍵 imageBase64', () {
    test('camelCase 契約鍵讀得到', () {
      expect(transformedImageBase64({'imageBase64': 'QUJD'}), 'QUJD');
    });

    test('只有舊 snake 鍵 image_base64 → null（不得有 fallback）', () {
      expect(transformedImageBase64({'image_base64': 'QUJD'}), isNull);
    });

    test('空字串 / 非字串 / 非 Map → null', () {
      expect(transformedImageBase64({'imageBase64': ''}), isNull);
      expect(transformedImageBase64({'imageBase64': 123}), isNull);
      expect(transformedImageBase64(null), isNull);
    });
  });
}
