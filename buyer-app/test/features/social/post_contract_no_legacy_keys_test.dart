import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 契約收斂回歸鎖（backend PR #2235）。
///
/// 後端把貼文 JSON 的欄名一律收斂成 camelCase，並**移除**了單數的
/// `imageKey` / `imageUrl` —— 貼文圖只剩 `imageKeys` / `imageUrls` 兩個陣列。
///
/// 單數欄是「貼文編輯清空 DB 圖片」的直接成因：前端讀單數欄回送，複數欄那份
/// 就被當成使用者刪掉了。收斂是一次切，**不留相容層**。
///
/// 本檔的 fixture 全部只給舊欄名，斷言結果為空 —— 任何 `新 ?? 舊` 的
/// fallback 復活都會讓這裡轉紅。
void main() {
  group('Post 只餵舊 wire 名一律讀不到（無相容層）', () {
    test('snake_case 欄名全部讀不到，計數欄退回預設值', () {
      final p = Post.fromJson({
        'id': '7490317712170336958',
        'content': 'x',
        'seller_id': '2685914682',
        'seller_name': '陳記',
        'like_count': 12,
        'comment_count': 34,
        'view_count': 56,
        'share_count': 78,
        'is_liked': true,
        'created_at': 1785830906,
        'store_code': '2685914682',
      });

      expect(p.merchantId, isNull);
      expect(p.merchantName, isNull);
      expect(p.likeCount, 0);
      expect(p.commentCount, 0);
      expect(p.viewCount, 0);
      expect(p.shareCount, 0);
      expect(p.liked, isFalse);
      expect(p.createdAt, isNull);
    });

    test('單數 imageKey / imageUrl 已被移除，不得再被讀成封面圖', () {
      final p = Post.fromJson({
        'id': '1',
        'content': 'x',
        'imageKey': 'public/post/legacy-single.png',
        'imageUrl': 'https://test-media.example.com/legacy-single.png',
      });

      expect(p.imageKeys, isEmpty);
      expect(p.imageUrls, isEmpty);
      expect(p.imageUrl, isEmpty, reason: '封面圖只能由複數欄導出');
    });

    test('無圖時後端回 []（非缺欄），封面圖為空字串不是 null', () {
      final p = Post.fromJson({
        'id': '1',
        'content': 'x',
        'imageKeys': <String>[],
        'imageUrls': <String>[],
      });

      expect(p.imageUrl, '');
    });
  });

  group('Post 讀新契約', () {
    test('camelCase 欄名全數命中，封面圖取 imageUrls 第一張', () {
      final p = Post.fromJson({
        'id': '7490317712170336958',
        'content': 'C6 契約驗證貼文',
        'sellerId': '2685914682',
        'sellerName': '陳記',
        'logoUrl': 'https://test-media.example.com/logo.png',
        'imageKeys': ['public/post/2685914682/20260804/a.png'],
        'imageUrls': ['https://test-media.example.com/i/original/a.png'],
        'likeCount': 12,
        'commentCount': 34,
        'viewCount': 56,
        'shareCount': 78,
        'isLiked': true,
        'createdAt': 1785830906,
      });

      expect(p.merchantId, 2685914682);
      expect(p.merchantName, '陳記');
      expect(p.likeCount, 12);
      expect(p.commentCount, 34);
      expect(p.viewCount, 56);
      expect(p.shareCount, 78);
      expect(p.liked, isTrue);
      expect(p.createdAt, 1785830906);
      expect(p.imageUrl, 'https://test-media.example.com/i/original/a.png');
    });

    test('只有 imageKeys（無 imageUrls）時封面圖退用裸 key', () {
      final p = Post.fromJson({
        'id': '1',
        'content': 'x',
        'imageKeys': ['public/post/a.png'],
      });

      expect(p.imageUrl, 'public/post/a.png');
    });
  });
}
