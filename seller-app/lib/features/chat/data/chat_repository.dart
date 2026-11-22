import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import 'chat_model.dart';

part 'chat_repository.g.dart';

/// 生成碼的 int64 欄位是 `Int64`；值由欄位的預設值（0）加上去取得，seller-app 不必直接依賴 fixnum。
final _zero = pb.GetUnreadCountResponse().unreadCount;

@riverpod
ChatRepository chatRepository(Ref ref) =>
    ChatRepository(ref.watch(chatClientProvider));

/// 店家側的聊天（`ChatAppService` 的 `*Store*` 方法與對話上的方法）。店家由伺服器依呼叫者此刻的店推導，
/// 請求不帶 store_id。錯誤原樣丟出（[ConnectException]），顯示文字由呼叫端取 LocalizedMessage。
class ChatRepository {
  ChatRepository(this._chat);

  /// 對話列表與訊息每頁筆數（API 預設 20，上限 100）。
  static const conversationPageSize = 30;
  static const messagePageSize = 30;

  final ChatAppServiceClient _chat;

  Future<ConversationPage> listConversations({String pageToken = ''}) async {
    final res = await _chat.listStoreConversations(
      pb.ListStoreConversationsRequest(
        pageSize: conversationPageSize,
        pageToken: pageToken,
      ),
    );
    return ConversationPage(
      items: res.conversations.map(ChatConversation.fromProto).toList(),
      nextPageToken: res.nextPageToken,
    );
  }

  /// 找一個對話：以買家 ID 或對話 ID 比對；沒有時是 null。契約沒有依買家或 ID 讀單一對話的方法，所以逐頁翻對話列表。
  Future<ChatConversation?> findConversation({
    String? buyerId,
    String? conversationId,
  }) async {
    assert((buyerId == null) != (conversationId == null));
    var token = '';
    do {
      final page = await listConversations(pageToken: token);
      for (final c in page.items) {
        if (buyerId != null ? c.buyer.id == buyerId : c.id == conversationId) {
          return c;
        }
      }
      token = page.nextPageToken;
    } while (token.isNotEmpty);
    return null;
  }

  /// 往舊的方向載入：[beforeSeq] 不給是從最新開始；回傳依 seq 由小到大。
  Future<MessagePage> listOlder(String conversationId, {int? beforeSeq}) async {
    final res = await _chat.listMessages(
      pb.ListMessagesRequest(
        conversationId: conversationId,
        pageSize: messagePageSize,
        beforeSeq: beforeSeq == null ? null : _zero + beforeSeq,
      ),
    );
    final list = res.messages.map(ChatMessage.fromProto).toList()
      ..sort((a, b) => a.seq.compareTo(b.seq));
    return MessagePage(
      messages: list,
      hasMore: res.hasMore,
      peerReadSeq: res.peerReadSeq.toInt(),
    );
  }

  /// 斷線補洞：seq 大於 [afterSeq] 的訊息，一頁一頁讀到沒有更多為止；回傳依 seq 由小到大。
  Future<MessagePage> listAfter(String conversationId, int afterSeq) async {
    final all = <ChatMessage>[];
    var cursor = afterSeq;
    var peerRead = 0;
    while (true) {
      final res = await _chat.listMessages(
        pb.ListMessagesRequest(
          conversationId: conversationId,
          pageSize: 100,
          afterSeq: _zero + cursor,
        ),
      );
      peerRead = res.peerReadSeq.toInt();
      final page = res.messages.map(ChatMessage.fromProto).toList()
        ..sort((a, b) => a.seq.compareTo(b.seq));
      all.addAll(page);
      if (!res.hasMore || page.isEmpty) break;
      cursor = page.last.seq;
    }
    return MessagePage(messages: all, hasMore: false, peerReadSeq: peerRead);
  }

  /// 對既有的對話送出。[requestId] 是這一次送出嘗試的 UUIDv4；重試帶同一個值，伺服器回第一次的訊息。
  Future<ChatMessage> send(
    String conversationId, {
    required String requestId,
    String? text,
    String? imageStagedToken,
  }) async {
    final res = await _chat.sendMessage(
      pb.SendMessageRequest(
        conversationId: conversationId,
        requestId: requestId,
        content: _content(text, imageStagedToken),
      ),
    );
    return ChatMessage.fromProto(res.message);
  }

  /// 還沒有對話時對買家 [buyerId] 送出（開新對話，有就接續）。
  Future<ChatMessage> sendToBuyer(
    String buyerId, {
    required String requestId,
    String? text,
    String? imageStagedToken,
  }) async {
    final res = await _chat.sendMessageToBuyer(
      pb.SendMessageToBuyerRequest(
        buyerId: buyerId,
        requestId: requestId,
        content: _content(text, imageStagedToken),
      ),
    );
    return ChatMessage.fromProto(res.message);
  }

  pb.MessageContent _content(String? text, String? imageStagedToken) {
    assert((text == null) != (imageStagedToken == null));
    return text != null
        ? pb.MessageContent(text: text)
        : pb.MessageContent(imageStagedToken: imageStagedToken);
  }

  /// 把這一方的已讀游標推到 [uptoSeq]。
  Future<void> markRead(String conversationId, int uptoSeq) async {
    await _chat.markConversationRead(
      pb.MarkConversationReadRequest(
        conversationId: conversationId,
        uptoSeq: _zero + uptoSeq,
      ),
    );
  }

  Future<void> markAllRead() async {
    await _chat.markAllStoreRead(pb.MarkAllStoreReadRequest());
  }

  Future<int> unreadCount() async {
    final res = await _chat.getStoreUnreadCount(
      pb.GetStoreUnreadCountRequest(),
    );
    return res.unreadCount.toInt();
  }

  /// 對方此刻是否在線（一次最多 50 個對話）：對話 ID 到 online。
  Future<Map<String, bool>> presence(List<String> conversationIds) async {
    if (conversationIds.isEmpty) return const {};
    final res = await _chat.getStorePresence(
      pb.GetStorePresenceRequest(conversationIds: conversationIds),
    );
    return {for (final p in res.presences) p.conversationId: p.online};
  }

  /// 店家的即時事件流（server streaming）。
  Stream<pb.ChatEvent> stream(Map<String, int> cursors) => _chat
      .streamStoreEvents(
        pb.StreamStoreEventsRequest(
          cursors: [
            for (final e in cursors.entries) MapEntry(e.key, _zero + e.value),
          ],
        ),
      )
      .map((r) => r.event);
}
