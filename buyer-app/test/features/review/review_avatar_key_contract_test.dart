import 'dart:convert';

import 'package:hawkr_buyer/features/review/data/review_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// `GET /api/v1/public/products/{id}/reviews` 的一筆 record（實打回應，
/// 2026-08-05 本機 P4 gateway 接本機微服務全棧，逐字貼上）。
/// 49 §6.6 收斂後評論者頭像叫 `avatarKey`＋`avatarUrl`（舊 `userAvatarKey`/`userAvatarUrl`）。
const String _reviewRecord =
    '{"reviewId":"7466164685380591689","buyerId":"2001","productId":"99001","username":"王**",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"rating":"4","content":"不錯，會再回購","createdAt":"2026-05-29T16:32:55Z",'
    '"imageKeys":[],"imageUrls":[]}';

const String _expectedKey =
    'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';

Map<String, dynamic> _decode(String raw) =>
    jsonDecode(raw) as Map<String, dynamic>;

void main() {
  group('Review 頭像契約（49 §6.6）', () {
    test('review record 的 avatarKey 進 Review.avatarKey', () {
      final map = _decode(_reviewRecord);
      // Review model 的 id/skuId/rating 是 int，公開評論端點回的是 TSID 字串，
      // 這裡只驗頭像欄位的解析，其餘欄位補成 model 期待的型別。
      final r = Review.fromJson({
        'id': 1,
        'skuId': 1,
        'rating': 4,
        'content': map['content'],
        'username': map['username'],
        'avatarKey': map['avatarKey'],
        'createdAt': map['createdAt'],
      });

      expect(r.avatarKey, _expectedKey);
    });

    test('舊名 userAvatar / userAvatarKey / userAvatarUrl 已不再被讀取', () {
      final r = Review.fromJson({
        'id': 1,
        'skuId': 1,
        'rating': 5,
        'content': 'x',
        'userAvatar': 'legacy/should/not/be/read.png',
        'userAvatarKey': 'legacy/should/not/be/read.png',
        'userAvatarUrl': 'legacy/should/not/be/read.png',
      });

      expect(r.avatarKey, isNull);
    });

    test('wire 上不得再出現舊名', () {
      final map = _decode(_reviewRecord);
      expect(map['avatarKey'], _expectedKey);
      for (final legacy in const [
        'userAvatarKey',
        'userAvatarUrl',
        'userAvatar',
        'buyerAvatarKey',
        'buyerAvatarUrl',
        'user_avatar_key',
      ]) {
        expect(map.containsKey(legacy), isFalse, reason: '不得再出現舊名 $legacy');
      }
    });
  });
}
