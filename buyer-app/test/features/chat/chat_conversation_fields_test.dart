import 'dart:convert';

import 'package:hawkr_buyer/features/chat/data/chat_conversation_fields.dart';
import 'package:hawkr_buyer/features/chat/presentation/chat_preview.dart';
import 'package:flutter_test/flutter_test.dart';

/// `GET /api/v1/app/chat/conversations` 對 dev 的實打回應
/// （2026-08-06 重打，買家 account 2623900520，逐字貼上）。
///
/// 這份 fixture 的用途是釘住「後端到底回哪些 key」。畫面層曾經對每個值都寫一長串
/// 候選（`participantName ?? merchantName ?? otherParticipantName ?? …`），
/// 其中大多數後端從未回過——`im.proto` 的 `message Conversation` 只有 10 個欄位，
/// 而且**沒有 conversationId**（hawkr_buyer#89）。
///
/// ⚠️ 這份 fixture 自己也曾經寫錯：對象頭像抄成 proto 欄名的 camel 版
/// `targetAvatarKey` / `targetAvatarUrl`，但 Gateway REST 吐的是
/// `avatarKey` / `avatarUrl`（`gateway/internal/handler/chat.go:401-402`）。
/// 已改成實打值，兩個舊名移進 [_deadKeysOnly]。
const String _conversationsBody =
    '{"data":{"conversations":[{'
    '"avatarKey":"public/seller_logo/2685914682/20260805/3f47a6bf-e8f4-4459-8737-91215b0050ea.png",'
    '"avatarUrl":"https://test-media.example.com/i/original/public/seller_logo/2685914682/20260805/3f47a6bf-e8f4-4459-8737-91215b0050ea.png",'
    '"lastMessage":"https://test-media.example.com/i/original/public/chat_message/2685914682/20260719/8641cab7-4ed5-4c21-8a3c-d06ba54dab5b.png",'
    '"lastMessageTime":"2026-07-19T04:25:49Z","lastMessageType":"image",'
    '"logoKey":"public/seller_logo/2685914682/20260805/3f47a6bf-e8f4-4459-8737-91215b0050ea.png",'
    '"logoUrl":"https://test-media.example.com/i/original/public/seller_logo/2685914682/20260805/3f47a6bf-e8f4-4459-8737-91215b0050ea.png",'
    '"storeCode":"2685914682","storeName":"陳記",'
    '"targetNickname":"陳記","targetUserId":"2685914682","unreadCount":"0"}],'
    '"total":"1"},"message":"success","code":0}';

Map<String, dynamic> _liveConversation() {
  final data =
      (jsonDecode(_conversationsBody) as Map<String, dynamic>)['data']
          as Map<String, dynamic>;
  return Map<String, dynamic>.from(
    (data['conversations'] as List).first as Map,
  );
}

/// 只含「畫面層曾經猜過、但後端從未回過」的 key。任何解析器碰到這份資料都必須
/// 回預設值——若某個欄位還讀得出東西，代表那個死 key 還掛在程式碼裡。
const Map<String, dynamic> _deadKeysOnly = {
  'participantName': '不該被讀到的名字',
  'merchantName': '不該被讀到的名字',
  'otherParticipantName': '不該被讀到的名字',
  'store_name': '不該被讀到的名字',
  'conversationId': 'conv-should-not-exist',
  'counterpartId': '9999999999',
  'otherParticipantId': '9999999999',
  'participantId': '9999999999',
  'userId': '9999999999',
  'merchantId': '9999999999',
  'lastContent': '不該被讀到的內容',
  'lastMessageContent': '不該被讀到的內容',
  'content': '不該被讀到的內容',
  'lastTimestamp': 1785911186,
  'lastMessageTimestamp': 1785911186,
  'timestamp': 1785911186,
  'updatedAt': '2026-08-05T06:26:26Z',
  'messageType': 'image',
  'lastType': 'image',
  'type': 'image',
  // proto 欄名 target_avatar_key 的 camel 版。Gateway REST 把它輸出成
  // avatarKey / avatarUrl，這兩個名字從來沒出現在 wire 上。
  'targetAvatarKey': 'dead/target-avatar.png',
  'targetAvatarUrl': 'https://dead.example.test/target-avatar.png',
};

void main() {
  group('conversation 欄位解析對齊 im.proto（#89）', () {
    test('實打回應解得出目標 id / 店名 / 頭像 / 時間 / 未讀', () {
      final c = _liveConversation();

      expect(resolveConversationTargetId(c), '2685914682');
      expect(resolveConversationDisplayName(c), '陳記');
      expect(
        resolveConversationAvatarUrl(c),
        'https://test-media.example.com/i/original/public/seller_logo/2685914682/20260805/3f47a6bf-e8f4-4459-8737-91215b0050ea.png',
      );
      expect(
        resolveConversationLastMessageTime(c)?.toUtc().toIso8601String(),
        '2026-07-19T04:25:49.000Z',
      );
      expect(resolveConversationUnreadCount(c), 0);
      expect(chatConversationPreview(c), '傳送了一張圖片');
    });

    test('實打回應沒有 conversationId，也沒有那四個名稱候選 key', () {
      final c = _liveConversation();
      for (final dead in const [
        'conversationId',
        'store_name',
        'participantName',
        'merchantName',
        'otherParticipantName',
        'counterpartId',
        'otherParticipantId',
        'participantId',
        'userId',
        'lastContent',
        'lastMessageContent',
        'lastTimestamp',
      ]) {
        expect(c.containsKey(dead), isFalse, reason: 'Conversation 不該出現 $dead');
      }
    });

    test('只給死 key 時，每個解析器都必須回預設值', () {
      expect(resolveConversationTargetId(_deadKeysOnly), '');
      expect(resolveConversationDisplayName(_deadKeysOnly), '商家');
      expect(resolveConversationAvatarUrl(_deadKeysOnly), '');
      expect(resolveConversationLastMessageTime(_deadKeysOnly), isNull);
      expect(resolveConversationUnreadCount(_deadKeysOnly), 0);
      expect(chatConversationPreview(_deadKeysOnly), '');
    });

    test('沒有 storeName 時退到 targetNickname（買家對買家 / 查不到店家）', () {
      expect(
        resolveConversationDisplayName(const {
          'storeName': '',
          'targetNickname': '小明',
        }),
        '小明',
      );
      expect(
        resolveConversationDisplayName(const {
          'storeName': '陳記',
          'targetNickname': '小明',
        }),
        '陳記',
      );
    });

    test('unreadCount 是字串（proto string unread_count）也要解得出數字', () {
      expect(resolveConversationUnreadCount(const {'unreadCount': '7'}), 7);
      expect(resolveConversationUnreadCount(const {'unreadCount': 7}), 7);
      expect(resolveConversationUnreadCount(const {}), 0);
    });

    test('lastMessageTime 是 ISO 8601 UTC，轉成本地時間', () {
      final parsed = resolveConversationLastMessageTime(const {
        'lastMessageTime': '2026-07-19T04:25:49Z',
      });
      expect(parsed, isNotNull);
      expect(parsed!.isUtc, isFalse, reason: '顯示端要拿本地時間');
      expect(parsed.toUtc().toIso8601String(), '2026-07-19T04:25:49.000Z');
      expect(
        resolveConversationLastMessageTime(const {'lastMessageTime': ''}),
        isNull,
      );
    });
  });
}
