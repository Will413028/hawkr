import 'dart:convert';

import 'package:hawkr_buyer/shared/utils/avatar_wire.dart';
import 'package:flutter_test/flutter_test.dart';

/// 以下三段都是 2026-08-05 實打回應（本機 P4 gateway 接本機微服務全棧）逐字貼上，
/// 分別對應 `resolveAvatarKey` 的三個真實呼叫點：
/// 討論區（merchant_chat_tab）、商品評論（product_reviews_screen /
/// merchant_item_detail_screen）、貼文留言與會員資料（comments_bottom_sheet）。

/// `GET /api/v1/public/sellers/{id}/forum` 的一筆 record。
const String _forumRecord =
    '{"userName":"王小明",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"content":"P4 討論區驗證留言（買家 2001）","id":"7490620245815251003","sellerId":"9",'
    '"userId":"2001","parentId":"0","replyToId":"0","createdAt":1785903035,"replyCount":0}';

/// `GET /api/v1/public/products/{id}/reviews` 的一筆 record。
const String _reviewRecord =
    '{"reviewId":"7466164685380591689","buyerId":"2001","productId":"99001","username":"王**",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"rating":"4","content":"不錯，會再回購","createdAt":"2026-05-29T16:32:55Z",'
    '"imageKeys":[],"imageUrls":[]}';

/// `GET /api/v1/app/buyers/me` 的 data（comments_bottom_sheet 用它取自己的頭像）。
const String _buyerProfile =
    '{"id":"2001","accountId":"2001","nickname":"王小明",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"level":"bronze","birthDate":"1990-05-20","city":"台北市","job":"",'
    '"email":"buyer1@hawkr.test","phone":"0933111222","gender":"male",'
    '"createdAt":"2026-07-13T17:33:45Z","updatedAt":"2026-08-05T03:37:39Z"}';

const String _expectedKey =
    'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';
const String _expectedUrl =
    'https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';

Map<String, dynamic> _decode(String raw) =>
    jsonDecode(raw) as Map<String, dynamic>;

void main() {
  group('resolveAvatarKey / resolveAvatarUrl（49 §6.1 扁平成對）', () {
    test('討論區 / 商品評論 / 會員資料三種 payload 都取得同一把 key', () {
      for (final entry in {
        '討論區 record': _forumRecord,
        '商品評論 record': _reviewRecord,
        '會員資料 /app/buyers/me': _buyerProfile,
      }.entries) {
        final map = _decode(entry.value);
        expect(resolveAvatarKey(map), _expectedKey, reason: entry.key);
        expect(resolveAvatarUrl(map), _expectedUrl, reason: entry.key);
      }
    });

    test('缺欄位 / 空字串 / null map 一律回 null（不回空字串）', () {
      expect(resolveAvatarKey(null), isNull);
      expect(resolveAvatarKey(const <String, dynamic>{}), isNull);
      expect(resolveAvatarKey(const {'avatarKey': ''}), isNull);
      expect(resolveAvatarKey(const {'avatarKey': '   '}), isNull);
      expect(resolveAvatarUrl(const {'avatarUrl': ''}), isNull);
    });

    test('九個舊名一個都不讀——收斂後沒有相容層', () {
      for (final legacy in const [
        'avatar',
        'object_key',
        'user_avatar_key',
        'user_avatar',
        'userAvatar',
        'userAvatarKey',
        'userAvatarUrl',
        'targetAvatarKey',
        'targetAvatarUrl',
        'buyerAvatarKey',
        'buyerAvatarUrl',
        'senderAvatar',
        'avatarBase64',
        'header',
        'photoUrl',
      ]) {
        expect(
          resolveAvatarKey({legacy: 'legacy/should/not/be/read.png'}),
          isNull,
          reason: '$legacy 不是契約欄位，不得被讀取',
        );
      }
    });

    test('討論區 wire 上不得再出現舊名', () {
      final map = _decode(_forumRecord);
      for (final legacy in const [
        'userAvatarKey',
        'userAvatarUrl',
        'user_avatar_key',
      ]) {
        expect(map.containsKey(legacy), isFalse, reason: '不得再出現 $legacy');
      }
    });
  });
}
