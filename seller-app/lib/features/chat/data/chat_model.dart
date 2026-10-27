import 'package:freezed_annotation/freezed_annotation.dart';

part 'chat_model.freezed.dart';

int _parseSeqId(dynamic v) {
  if (v == null) return 0;
  if (v is int) return v;
  if (v is num) return v.toInt();
  return int.tryParse(v.toString()) ?? 0;
}

@freezed
abstract class ChatMessage with _$ChatMessage {
  const ChatMessage._(); // enables instance methods

  const factory ChatMessage({
    required String id,
    required String senderId,
    required String receiverId,
    required String messageContent,
    required String messageType,
    required DateTime timestamp,
    String? status,
    String? conversationId,
    @Default(false) bool deleted,
    @Default(false) bool read,
    @Default(0) int seqId,
    String? senderName, // merged from buyer; populated by WS frames
    String? senderAvatar, // 裸 objectKey（WS `avatarKey`），顯示前必過 buildImageUrl
  }) = _ChatMessage;

  factory ChatMessage.fromJson(Map<String, dynamic> m) {
    DateTime parseTs(dynamic v) {
      if (v == null) return DateTime.now();
      if (v is int) return DateTime.fromMillisecondsSinceEpoch(v);
      final s = v.toString();
      return DateTime.tryParse(s) ?? DateTime.now();
    }

    String? firstNonEmpty(List<String> keys) {
      for (final k in keys) {
        final v = m[k]?.toString().trim();
        if (v != null && v.isNotEmpty) return v;
      }
      return null;
    }

    return ChatMessage(
      // HTTP: id, WebSocket: messageId
      id: (m['id'] ?? m['messageId'] ?? '').toString(),
      // HTTP: fromUserId, WebSocket: from
      senderId: (m['fromUserId'] ?? m['from'] ?? '').toString(),
      // HTTP: toUserId, WebSocket: to
      receiverId: (m['toUserId'] ?? m['to'] ?? '').toString(),
      messageContent: (m['content'] ?? '').toString(),
      messageType: (m['type'] ?? 'text').toString(),
      timestamp: parseTs(m['timestamp']),
      status: m['status']?.toString(),
      conversationId: m['conversationId']?.toString(),
      deleted: (m['deleted'] == true || m['deleted'] == 'true'),
      read: (m['read'] == true || m['read'] == 'true' || m['isRead'] == true),
      seqId: _parseSeqId(m['seqId'] ?? m['seq_id']),
      senderName: firstNonEmpty([
        'senderName',
        'sender_name',
        'fromName',
        'from_name',
        'displayName',
      ]),
      // 發送者頭像的唯一契約欄位＝`avatarKey`（裸 objectKey，49 §6.7 S4；
      // im-service `domain.Message.SenderAvatar` 的 json tag）。舊名
      // `senderAvatar` / `sender_avatar` / `senderAvatarUrl` / `fromAvatar` /
      // `avatarUrl` 一律不存在——FCM 那條路（仍是 `sender_avatar`）不經過本模型，
      // 見 push/manager.dart 自帶的 key list。
      senderAvatar: firstNonEmpty(['avatarKey']),
    );
  }

  /// True when this message was sent by [myId].
  bool isSentBy(String myId) => senderId == myId;
}

/// 聊天參與者資訊（由 chat history API 回傳）
class ChatParticipant {
  /// 該參與者的 chat wire 身份（買家 buyer_id／賣家 store_id，皆為 CURRENT stint
  /// 代理鍵），與 [ChatMessage.senderId]/[ChatMessage.receiverId] 同一個 id 空間。
  /// **不是登入 accountId** —— 後端 2026-08-07 起把這個欄位的 wire key 由
  /// `accountId` 正名為 `userId`，舊名謊報內容物。
  final String userId;
  final String role;
  final String displayName;
  final String avatarUrl;

  const ChatParticipant({
    required this.userId,
    required this.role,
    required this.displayName,
    required this.avatarUrl,
  });

  factory ChatParticipant.fromJson(Map<String, dynamic> m) {
    return ChatParticipant(
      userId: (m['userId'] ?? '').toString(),
      role: (m['role'] ?? '').toString(),
      displayName: (m['displayName'] ?? '').toString(),
      avatarUrl: (m['avatarUrl'] ?? '').toString(),
    );
  }
}

/// chat history API 的完整回傳結果
class ChatHistoryResult {
  final List<ChatMessage> messages;
  final List<ChatParticipant> participants;

  const ChatHistoryResult({required this.messages, required this.participants});
}
