import 'dart:convert';

import 'package:hawkr_buyer/features/chat/data/chat_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 49 §6.6/§6.7 S4 收斂後，WebSocket `type=chat` frame 的發送者頭像叫 `avatarKey`
/// （im-service `domain.Message.SenderAvatar` 的 json tag），值是裸 objectKey。
///
/// 下面兩段 fixture 是實打回應，不是手寫的 wire shape：
/// 2026-08-05 於本機 P4 gateway（接本機微服務全棧）連 `ws://…/api/v1/app/chat/ws`
/// 收到的原始 frame，逐字貼上。
const String _wsChatFrameWithAvatar =
    '{"roomId":"b2001_s9","type":"chat","content":"P4 WS：買家→賣家，senderAvatar 應為 avatarKey",'
    '"from":"2001","to":"9","messageId":"7490620918531261488","seqId":"3",'
    '"timestamp":1785903195984,"msgType":1,"senderName":"王小明",'
    '"avatarKey":"public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png"}';

/// 同一支端點、發送者沒有頭像時的實際 frame —— `avatarKey` 帶 `omitempty`，整個
/// key 不出現（不是空字串）。
const String _wsChatFrameWithoutAvatar =
    '{"roomId":"b2001_s9","type":"chat","content":"P4 WS 契約驗證訊息（賣家→買家）",'
    '"from":"9","to":"2001","messageId":"7490620444813215113","seqId":"1",'
    '"timestamp":1785903083041,"msgType":1,"senderName":"翠綠有機農場"}';

Map<String, dynamic> _decode(String raw) =>
    jsonDecode(raw) as Map<String, dynamic>;

void main() {
  group('ChatMessage 頭像契約（49 §6.7 S4）', () {
    test('WS chat frame 的 avatarKey 進 senderAvatar', () {
      final m = ChatMessage.fromJson(_decode(_wsChatFrameWithAvatar));

      expect(
        m.senderAvatar,
        'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png',
      );
      expect(m.senderName, '王小明');
      expect(m.senderId, '2001');
      expect(m.seqId, 3);
    });

    test('沒有 avatarKey 的 frame（omitempty）→ senderAvatar 為 null', () {
      final m = ChatMessage.fromJson(_decode(_wsChatFrameWithoutAvatar));

      expect(m.senderAvatar, isNull);
      expect(m.senderName, '翠綠有機農場');
    });

    test('舊名一律不再被讀取——收斂成單一 avatarKey，沒有相容層', () {
      // 這些 key 後端已物理移除。留任何一個 fallback，都會在未來某個同名欄位
      // 出現時搶先命中並蓋掉正確值；本 case 就是防止 fallback 被加回來。
      for (final legacy in const [
        'senderAvatar',
        'sender_avatar',
        'senderAvatarUrl',
        'sender_avatar_url',
        'fromAvatar',
        'from_avatar',
        'avatarUrl',
        'avatar_url',
      ]) {
        final m = ChatMessage.fromJson({
          'id': 'x',
          'fromId': 'u1',
          'toId': 'u2',
          'content': '',
          'timestamp': 0,
          legacy: 'legacy/should/not/be/read.png',
        });
        expect(
          m.senderAvatar,
          isNull,
          reason: '$legacy 不是契約欄位，不得被 ChatMessage.fromJson 讀取',
        );
      }
    });
  });
}
