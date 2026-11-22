import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/images.dart';

/// 訊息的送出狀態；只有本機送出中或失敗的訊息不是 [sent]。
enum ChatSendState { sent, sending, failed }

/// 一則聊天訊息（`chat.v1alpha1.Message` 的 client 形狀）。隱藏或抹除的訊息是墓碑：[hidden] 或 [erased]
/// 為真，沒有 [text] 與 [imageUri]。
class ChatMessage {
  const ChatMessage({
    required this.id,
    required this.conversationId,
    required this.seq,
    required this.fromStore,
    required this.kind,
    required this.createTime,
    this.text,
    this.imageUri,
    this.hidden = false,
    this.erased = false,
    this.sendState = ChatSendState.sent,
    this.requestId,
  });

  factory ChatMessage.fromProto(pb.Message m) => ChatMessage(
    id: m.id,
    conversationId: m.conversationId,
    seq: m.seq.toInt(),
    fromStore: m.sender == pb.ChatParty.CHAT_PARTY_STORE,
    kind: m.kind,
    text: m.hasText() ? m.text : null,
    imageUri: m.hasImage() && m.image.uriTemplate.isNotEmpty
        ? m.image.uri(ImagePreset.card)
        : null,
    hidden: m.hidden,
    erased: m.erased,
    createTime: m.hasCreateTime() ? m.createTime.toDateTime() : DateTime.now(),
  );

  /// 本機送出中的訊息：還沒有伺服器的 id 與 seq，以 [requestId] 對回伺服器的訊息。
  factory ChatMessage.pending({
    required String requestId,
    required String conversationId,
    required pb.MessageKind kind,
    String? text,
    String? imageUri,
  }) => ChatMessage(
    id: 'pending-$requestId',
    conversationId: conversationId,
    seq: 0,
    fromStore: true,
    kind: kind,
    text: text,
    imageUri: imageUri,
    createTime: DateTime.now(),
    sendState: ChatSendState.sending,
    requestId: requestId,
  );

  final String id;
  final String conversationId;

  /// 對話裡的序號（從 1 起連號）；本機送出中的訊息是 0。
  final int seq;

  /// 寄件方是這家店（呼叫者）。
  final bool fromStore;
  final pb.MessageKind kind;
  final String? text;
  final String? imageUri;
  final bool hidden;
  final bool erased;
  final DateTime createTime;
  final ChatSendState sendState;
  final String? requestId;

  bool get isTombstone => hidden || erased;
  bool get isPending => sendState != ChatSendState.sent;

  ChatMessage copyWith({
    String? conversationId,
    ChatSendState? sendState,
    String? id,
    int? seq,
  }) => ChatMessage(
    id: id ?? this.id,
    conversationId: conversationId ?? this.conversationId,
    seq: seq ?? this.seq,
    fromStore: fromStore,
    kind: kind,
    text: text,
    imageUri: imageUri,
    hidden: hidden,
    erased: erased,
    createTime: createTime,
    sendState: sendState ?? this.sendState,
    requestId: requestId,
  );
}

/// 對話列表上最後一則訊息的結構化預覽；顯示句子由 [previewText] 取自共用文案目錄。
class ChatPreview {
  const ChatPreview({
    required this.kind,
    required this.hidden,
    required this.erased,
    required this.fromStore,
    this.text,
  });

  factory ChatPreview.fromProto(pb.Preview p) => ChatPreview(
    kind: p.kind,
    text: p.hasText() ? p.text : null,
    hidden: p.hidden,
    erased: p.erased,
    fromStore: p.sender == pb.ChatParty.CHAT_PARTY_STORE,
  );

  final pb.MessageKind kind;
  final String? text;
  final bool hidden;
  final bool erased;
  final bool fromStore;
}

/// 店家的對話裡的買家；已刪除的買家沒有名稱與頭像。
class ChatBuyerInfo {
  const ChatBuyerInfo({
    required this.id,
    required this.deleted,
    this.name,
    this.avatarUri,
  });

  factory ChatBuyerInfo.fromProto(pb.ChatBuyer b) => ChatBuyerInfo(
    id: b.id,
    deleted: b.deleted,
    name: b.hasName() && b.name.trim().isNotEmpty ? b.name.trim() : null,
    avatarUri: b.hasAvatar() && b.avatar.uriTemplate.isNotEmpty
        ? b.avatar.uri(ImagePreset.avatar)
        : null,
  );

  final String id;
  final bool deleted;
  final String? name;
  final String? avatarUri;
}

/// 對話列表的一列（店家版）。
class ChatConversation {
  const ChatConversation({
    required this.id,
    required this.buyer,
    required this.unreadCount,
    required this.peerReadSeq,
    this.lastMessageTime,
    this.preview,
  });

  factory ChatConversation.fromProto(pb.Conversation c) => ChatConversation(
    id: c.id,
    buyer: ChatBuyerInfo.fromProto(c.buyer),
    unreadCount: c.unreadCount.toInt(),
    peerReadSeq: c.peerReadSeq.toInt(),
    lastMessageTime: c.hasLastMessageTime()
        ? c.lastMessageTime.toDateTime()
        : null,
    preview: c.hasPreview() ? ChatPreview.fromProto(c.preview) : null,
  );

  final String id;
  final ChatBuyerInfo buyer;
  final int unreadCount;
  final int peerReadSeq;
  final DateTime? lastMessageTime;
  final ChatPreview? preview;

  ChatConversation copyWith({int? unreadCount, int? peerReadSeq}) =>
      ChatConversation(
        id: id,
        buyer: buyer,
        unreadCount: unreadCount ?? this.unreadCount,
        peerReadSeq: peerReadSeq ?? this.peerReadSeq,
        lastMessageTime: lastMessageTime,
        preview: preview,
      );
}

class ConversationPage {
  const ConversationPage({required this.items, required this.nextPageToken});

  final List<ChatConversation> items;
  final String nextPageToken;

  bool get hasMore => nextPageToken.isNotEmpty;
}

class MessagePage {
  const MessagePage({
    required this.messages,
    required this.hasMore,
    required this.peerReadSeq,
  });

  /// 依 seq 由小到大。
  final List<ChatMessage> messages;
  final bool hasMore;
  final int peerReadSeq;
}
