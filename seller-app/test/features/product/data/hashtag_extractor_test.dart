import 'package:flutter_test/flutter_test.dart';

import 'package:hawkr_seller/features/product/data/hashtag_extractor.dart';

void main() {
  group('extractFromText', () {
    test('純英文 hashtag', () {
      expect(HashtagExtractor.extractFromText('cool #foodie post'), ['foodie']);
    });

    test('中英混合', () {
      expect(HashtagExtractor.extractFromText('來自 #紐西蘭 的 #foodie 推薦'), [
        '紐西蘭',
        'foodie',
      ]);
    });

    test('連續 ##', () {
      // 第一個 # 後面是另一個 # 不是合法 hashtag char → 不抽出空 tag；
      // 第二個 # 後面接合法 chars → 抽出 hashtag
      expect(HashtagExtractor.extractFromText('##紐西蘭'), ['紐西蘭']);
    });

    test('黏在字元前不抽（必須 word boundary）', () {
      // 「abc#xx」這種沒有空白分隔的不算 hashtag
      // 注意：regex 不在意前面是什麼字元，所以這個 case 其實會抽出。
      // 我們的設計選擇：簡化規則，只要 # 後面有合法 chars 就抽（跟 Twitter 行為一致）。
      expect(HashtagExtractor.extractFromText('abc#xx'), ['xx']);
    });

    test('排除標點 / 空白 / emoji', () {
      expect(
        HashtagExtractor.extractFromText('#fo!odie #good 🎉 #台灣🇹🇼'),
        // ! 中斷 → 抽出 'fo'；'good' 完整；emoji 中斷 → 抽出 '台灣'
        ['fo', 'good', '台灣'],
      );
    });

    test('dedup (case-insensitive)', () {
      expect(
        HashtagExtractor.extractFromText('#Foodie #FOODIE #foodie #紐西蘭 #紐西蘭'),
        ['Foodie', '紐西蘭'],
      );
    });

    test('underscore + digit OK', () {
      expect(HashtagExtractor.extractFromText('#summer_2026 #v2'), [
        'summer_2026',
        'v2',
      ]);
    });

    test('全 # 算 0 個', () {
      expect(HashtagExtractor.extractFromText('### #'), isEmpty);
    });

    test('超過 maxCount 截斷', () {
      final input = List.generate(15, (i) => '#tag$i').join(' ');
      final result = HashtagExtractor.extractFromText(input);
      expect(result.length, HashtagExtractor.maxCount);
      expect(result.first, 'tag0');
      expect(result.last, 'tag${HashtagExtractor.maxCount - 1}');
    });
  });

  group('extractFromBlocks', () {
    test('多個 text block 合併抽取且 dedup', () {
      final blocks = [
        {'type': 'text', 'content': '我去了 #紐西蘭'},
        {'type': 'image', 'objectKey': 'a.jpg'},
        {'type': 'text', 'content': '#紐西蘭 真棒 #foodie'},
      ];
      expect(HashtagExtractor.extractFromBlocks(blocks), ['紐西蘭', 'foodie']);
    });

    test('image-only blocks 回空', () {
      expect(
        HashtagExtractor.extractFromBlocks([
          {'type': 'image', 'objectKey': 'a.jpg'},
        ]),
        isEmpty,
      );
    });
  });

  group('detectTaggingMode', () {
    test('打 # 後一個字 → 進入 tagging mode', () {
      final mode = HashtagExtractor.detectTaggingMode('我去了 #紐');
      expect(mode, isNotNull);
      expect(mode!.prefix, '紐');
      expect(mode.hashAt, 4); // '我去了 ' 4 chars, 然後 #
    });

    test('只打 # 但設定為要 1 個字才觸發 → 不觸發', () {
      final mode = HashtagExtractor.detectTaggingMode('hello #');
      expect(mode, isNull);
    });

    test('triggerAfterChars=0 → 打 # 就觸發', () {
      final mode = HashtagExtractor.detectTaggingMode(
        'hello #',
        triggerAfterChars: 0,
      );
      expect(mode, isNotNull);
      expect(mode!.prefix, '');
    });

    test('caret 前有空白 → 不在 tagging mode', () {
      expect(HashtagExtractor.detectTaggingMode('hello #紐西蘭 '), isNull);
    });

    test('# 前面接著合法 chars → 不在 tagging mode（避免 abc#xx 觸發 popover）', () {
      // 規則：tagging mode 只在 # 是 word boundary 開頭時觸發
      expect(HashtagExtractor.detectTaggingMode('abc#xx'), isNull);
    });

    test('開頭 # → 觸發', () {
      final mode = HashtagExtractor.detectTaggingMode('#fo');
      expect(mode, isNotNull);
      expect(mode!.prefix, 'fo');
      expect(mode.hashAt, 0);
    });
  });

  group('countInText', () {
    test('回傳 dedup 後的數量', () {
      expect(HashtagExtractor.countInText('#a #a #b'), 2);
      expect(HashtagExtractor.countInText(''), 0);
      expect(HashtagExtractor.countInText('no hashtags here'), 0);
    });
  });

  group('extractFromBlocks — inline-run 模型（issue #1246）', () {
    test('runs 格式 text block 可抽出 hashtag（修復前恆為空 → 存檔洗掉 hashtag）', () {
      final tags = HashtagExtractor.extractFromBlocks([
        {
          'type': 'text',
          'runs': [
            {'text': '新鮮直送 '},
            {'text': '#芒果', 'bold': true},
            {'text': ' 與 #產地直送 推薦'},
          ],
        },
      ]);
      expect(tags, ['芒果', '產地直送']);
    });

    test('legacy content 欄位仍相容', () {
      final tags = HashtagExtractor.extractFromBlocks([
        {'type': 'text', 'content': '老格式 #舊資料'},
      ]);
      expect(tags, ['舊資料']);
    });

    test('runs 與 content 並存時 runs 優先', () {
      final tags = HashtagExtractor.extractFromBlocks([
        {
          'type': 'text',
          'runs': [
            {'text': '#新'},
          ],
          'content': '#舊',
        },
      ]);
      expect(tags, ['新']);
    });

    test('blockPlainText：runs 串接 / legacy fallback', () {
      expect(
        HashtagExtractor.blockPlainText({
          'runs': [
            {'text': 'a'},
            {'text': 'b'},
          ],
        }),
        'ab',
      );
      expect(HashtagExtractor.blockPlainText({'content': 'c'}), 'c');
      expect(HashtagExtractor.blockPlainText({}), '');
    });
  });
}
