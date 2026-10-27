import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';

// TSID 主鍵是 64-bit BIGINT。後端正逐步把推給前端的 id 從 JSON number 改成 string
// （notification #960/#961 + 同期稽核）。買家 app 必須在後端翻字串「前後」都能解析，
// 否則 json_serializable 產出的 `(json['id'] as num)` 對 String 會丟例外、貼文/留言
// 直接炸。本測試鎖住容錯行為：number 與 string 兩種型別都要能解析，且 TSID > 2^53
// （Dart int 為 64-bit，無精度損失）round-trip 精確。
void main() {
  // 2026 年的 TSID（42-bit ms << 22）約 7.5e18，遠超 JS 的 2^53；Dart int 仍精確。
  const tsid = 7234567890123456789;
  const tsidStr = '7234567890123456789';

  group('Post.fromJson TSID 容錯', () {
    test('id 為 JSON number（後端翻字串前的現狀）', () {
      final post = Post.fromJson({'id': tsid, 'sellerId': tsid});
      expect(post.id, tsid);
      expect(post.merchantId, tsid);
    });

    test('id 為 JSON string（後端翻字串後）', () {
      final post = Post.fromJson({'id': tsidStr, 'sellerId': tsidStr});
      expect(post.id, tsid);
      expect(post.merchantId, tsid);
    });

    test('seller_id 缺省維持 null', () {
      final post = Post.fromJson({'id': tsidStr});
      expect(post.merchantId, isNull);
    });
  });

  group('PostComment.fromJson TSID 容錯', () {
    test('id 為 JSON number', () {
      final c = PostComment.fromJson({'id': tsid, 'content': 'hi'});
      expect(c.id, tsid);
    });

    test('id 為 JSON string', () {
      final c = PostComment.fromJson({'id': tsidStr, 'content': 'hi'});
      expect(c.id, tsid);
    });

    test('isMine 缺省為 false，伺服器給 true 才是自己的留言', () {
      expect(
        PostComment.fromJson({'id': tsidStr, 'content': 'hi'}).isMine,
        isFalse,
      );
      expect(
        PostComment.fromJson({
          'id': tsidStr,
          'content': 'hi',
          'isMine': true,
        }).isMine,
        isTrue,
      );
    });
  });
}
