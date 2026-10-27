import 'dart:convert';

import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 貼文留言的頭像欄位在 49 §6.6 收斂後一律是扁平成對的 `avatarKey`＋`avatarUrl`
/// （舊：讀取端 `userAvatarKey`/`userAvatarUrl`、寫入端 snake `user_avatar_key`）。
///
/// 三段 fixture 皆為實打回應（2026-08-05，本機 P4 gateway 接本機微服務全棧），
/// 逐字貼上，未經改寫。

/// `GET /api/v1/public/posts/{id}/comments` 的一筆 record。
const String _restCommentRecord =
    '{"userName":"王小明",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"content":"P4 契約驗證留言（買家 2001）","id":"7490620122015620422",'
    '"postId":"7490620040132062876","parentId":"0","replyToId":"0",'
    '"createdAt":1785903006,"likeCount":0,"role":1,"isLiked":false,"isMine":false}';

/// `POST /api/v1/app/posts/{id}/comments` 的 data —— 寫入回應現在與讀取端同形狀
/// （這支以前是 `response.OK(c, resp.Data)` 直吐 proto，欄位叫 `user_avatar_key`）。
const String _writeCommentResponse =
    '{"userName":"王小明",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"content":"P4 契約驗證留言（買家 2001）","id":"7490620122015620422",'
    '"postId":"7490620040132062876","parentId":"0","replyToId":"0",'
    '"createdAt":1785903006,"likeCount":0,"role":1,"isLiked":false,"isMine":false}';

/// SSE `comment.created` 的 `comment`（41 號）。S4 通道不經 Gateway 組裝，
/// **只有 `avatarKey`、沒有 `avatarUrl`**。
const String _sseCommentPayload =
    '{"commentId":"7490620122015620422","postId":"7490620040132062876","userId":"2001",'
    '"userName":"王小明",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png",'
    '"role":1,"content":"P4 契約驗證留言（買家 2001）","parentId":"0","replyToId":"0",'
    '"replyToName":"","likeCount":0,"replyCount":0,'
    '"createdAt":"2026-08-05T04:10:06Z","updatedAt":"2026-08-05T04:10:06Z","isMine":false}';

const String _expectedKey =
    'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';
const String _expectedUrl =
    'https://test-media.example.com/i/original/public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';

Map<String, dynamic> _decode(String raw) =>
    jsonDecode(raw) as Map<String, dynamic>;

void main() {
  group('PostComment 頭像契約（49 §6.6）', () {
    test('REST 留言 record 的 avatarUrl 進 PostComment.avatarUrl', () {
      final c = PostComment.fromJson(_decode(_restCommentRecord));

      expect(c.avatarUrl, _expectedUrl);
      expect(c.userName, '王小明');
      expect(c.liked, isFalse);
      expect(c.likeCount, 0);
    });

    test('舊名 userAvatarUrl 已不再被 PostComment 讀取', () {
      final c = PostComment.fromJson({
        'id': '1',
        'content': 'x',
        'userAvatarUrl': 'legacy/should/not/be/read.png',
        'userAvatarKey': 'legacy/should/not/be/read.png',
      });

      expect(c.avatarUrl, isNull);
    });

    // 契約收斂後留言欄名一律 camelCase，且**沒有相容層**。只餵舊名必須整筆
    // 讀不到 —— 誰把 `user_name` / `is_liked` / `like_count` 的 fallback 加回來，
    // 這條就轉紅。
    test('只餵舊 snake_case 欄名一律讀不到（無相容層）', () {
      final c = PostComment.fromJson({
        'id': '1',
        'content': 'x',
        'user_name': '不該被讀到',
        'is_mine': true,
        'is_liked': true,
        'like_count': 42,
        'created_at': 1785903006,
      });

      expect(c.userName, isNull);
      expect(c.isMine, isFalse);
      expect(c.liked, isFalse);
      expect(c.likeCount, 0);
      expect(c.createdAt, isNull);
    });
  });

  group('留言 map 的頭像鍵在三個來源同名（49 §6.6 收斂結果）', () {
    // comments_bottom_sheet 的 _pickAvatarFromMap 之所以能收斂成單一 key，
    // 前提就是這三個來源同名。這個 case 把那個前提釘住：任何一邊被改回去，
    // 收斂後的讀法會靜默讀到 null（頭像變佔位圖），而這裡會先轉紅。
    test('REST 讀取／REST 寫入／SSE 三者都用 avatarKey', () {
      for (final entry in {
        'REST 讀取': _restCommentRecord,
        'REST 寫入回應': _writeCommentResponse,
        'SSE comment.created': _sseCommentPayload,
      }.entries) {
        final map = _decode(entry.value);
        expect(
          map['avatarKey'],
          _expectedKey,
          reason: '${entry.key} 必須以 avatarKey 帶裸 objectKey',
        );
        for (final legacy in const [
          'userAvatarKey',
          'user_avatar_key',
          'user_avatar',
          'userAvatar',
          'userAvatarUrl',
          'avatar',
        ]) {
          expect(
            map.containsKey(legacy),
            isFalse,
            reason: '${entry.key} 不得再出現舊名 $legacy',
          );
        }
      }
    });

    test('SSE（S4）只帶 key，不帶 avatarUrl；REST 兩支成對', () {
      expect(_decode(_sseCommentPayload).containsKey('avatarUrl'), isFalse);
      expect(_decode(_restCommentRecord)['avatarUrl'], _expectedUrl);
      expect(_decode(_writeCommentResponse)['avatarUrl'], _expectedUrl);
    });
  });
}
