import 'package:freezed_annotation/freezed_annotation.dart';

part 'chat_model.freezed.dart';
// NOTE: No 'chat_model.g.dart' part — fromJson is hand-written, not codegen.

@freezed
abstract class ChatMessage with _$ChatMessage {
  const ChatMessage._(); // enables custom methods in freezed class (freezed 3.x 要求 abstract)

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
    String? senderName,
    String? senderAvatar,
  }) = _ChatMessage;

  /// Hand-written normalizer — preserves buyer's existing key-alias logic.
  /// Canonical contract: id|messageId, fromId|from, content, msgType int 2→image,
  /// seqId int/string tolerant, senderName key list, senderAvatar ← `avatarKey`.
  ///
  /// 頭像只讀 `avatarKey`：WS frame（im-service `domain.Message`）與 REST 歷史
  /// （`/app/chat/messages`，本來就不帶發送者頭像）是本 factory 的唯二輸入源，
  /// 49 §6.7 S4 收斂後該欄一律叫 `avatarKey`（裸 objectKey）。FCM push 走
  /// `features/notification/data/push/manager.dart` 自己的 key list，不經這裡。
  factory ChatMessage.fromJson(Map<String, dynamic> m) {
    DateTime parseTs(dynamic v) {
      if (v == null) return DateTime.now();
      if (v is int) return DateTime.fromMillisecondsSinceEpoch(v);
      return DateTime.tryParse(v.toString()) ?? DateTime.now();
    }

    String resolvedType = (m['type'] ?? m['messageType'] ?? 'chat').toString();
    final msgTypeInt = m['msgType'];
    if (msgTypeInt == 2) {
      resolvedType = 'image';
    } else if (resolvedType == 'MESSAGE_TYPE_IMAGE') {
      resolvedType = 'image';
    } else if (resolvedType == 'MESSAGE_TYPE_TEXT') {
      resolvedType = 'text';
    }

    String? readString(List<String> keys) {
      for (final key in keys) {
        final value = m[key]?.toString().trim();
        if (value != null && value.isNotEmpty) return value;
      }
      return null;
    }

    return ChatMessage(
      id: (m['id'] ?? m['messageId'] ?? '').toString(),
      senderId:
          (m['fromUserId'] ?? m['fromId'] ?? m['senderId'] ?? m['from'] ?? '')
              .toString(),
      receiverId:
          (m['toUserId'] ?? m['toId'] ?? m['receiverId'] ?? m['to'] ?? '')
              .toString(),
      messageContent: (m['content'] ?? m['messageContent'] ?? '').toString(),
      messageType: resolvedType,
      timestamp: parseTs(m['timestamp'] ?? m['time'] ?? m['createdAt']),
      read: m['isRead'] == true || m['read'] == true || m['status'] == 'read',
      conversationId: m['conversationId']?.toString(),
      seqId: int.tryParse((m['seqId'] ?? m['seq_id'] ?? '0').toString()) ?? 0,
      senderName: readString([
        'senderName',
        'sender_name',
        'fromName',
        'from_name',
        'displayName',
      ]),
      senderAvatar: readString(['avatarKey']),
    );
  }

  bool isSentBy(String myId) => senderId == myId;
}
