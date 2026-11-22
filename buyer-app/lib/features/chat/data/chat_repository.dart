import 'package:fixnum/fixnum.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../../core/networking/connect_provider.dart';

part 'chat_repository.g.dart';

@Riverpod(keepAlive: true)
ChatRepository chatRepository(Ref ref) =>
    ChatRepository(ref.watch(chatAppClientProvider));

/// 一次送出：內容與 `request_id`。重送（同一個畫面上的「重試」）帶同一個 [requestId]，伺服器回第一次寫入的那則訊息。
class OutgoingContent {
  OutgoingContent.text(String this.text, {String? requestId})
    : imageStagedToken = null,
      requestId = requestId ?? const Uuid().v4();

  OutgoingContent.image(String this.imageStagedToken, {String? requestId})
    : text = null,
      requestId = requestId ?? const Uuid().v4();

  final String? text;
  final String? imageStagedToken;
  final String requestId;

  pb.MessageContent toProto() => text != null
      ? pb.MessageContent(text: text)
      : pb.MessageContent(imageStagedToken: imageStagedToken);
}

/// 買家的聊天（`ChatAppService` 買家版方法）。讀取與送出失敗一律丟出，不回空值。
class ChatRepository {
  ChatRepository(this._client);

  final ChatAppServiceClient _client;

  /// 訊息每頁筆數（API 預設 20，上限 100）。
  static const messagePageSize = 50;

  /// 對話每頁筆數（API 預設 20，上限 100）。
  static const conversationPageSize = 100;

  /// 串流 `cursors` 與 `GetPresence` 的對話數上限。
  static const maxCursors = 50;

  /// 讀過的對話的店家卡片（對話 ID 到店家），給沒有列表資料的地方（例如新訊息橫幅）取名稱與頭像。
  final storeCards = <String, StoreCard>{};

  Future<pb.ListConversationsResponse> listConversations({
    String pageToken = '',
  }) async {
    final response = await _client.listConversations(
      pb.ListConversationsRequest(
        pageSize: conversationPageSize,
        pageToken: pageToken,
      ),
    );
    for (final c in response.conversations) {
      if (c.hasStore()) storeCards[c.id] = c.store;
    }
    return response;
  }

  /// 全部對話（逐頁取到沒有下一頁）；任何一頁失敗就丟出。
  Future<List<pb.Conversation>> listAllConversations() async {
    final all = <pb.Conversation>[];
    var token = '';
    do {
      final page = await listConversations(pageToken: token);
      all.addAll(page.conversations);
      token = page.nextPageToken;
    } while (token.isNotEmpty);
    return all;
  }

  /// 新的在前（seq 由大到小）。[beforeSeq] 只列 seq 較小的；不給是從最新開始。
  Future<pb.ListMessagesResponse> listMessagesBefore(
    String conversationId, {
    Int64? beforeSeq,
  }) => _client.listMessages(
    pb.ListMessagesRequest(
      conversationId: conversationId,
      pageSize: messagePageSize,
      beforeSeq: beforeSeq,
    ),
  );

  /// 補洞：seq 大於 [afterSeq] 的訊息，舊的在前。只回一頁，`hasMore` 為真時呼叫端以最大的 seq 繼續。
  Future<pb.ListMessagesResponse> listMessagesAfter(
    String conversationId,
    Int64 afterSeq,
  ) => _client.listMessages(
    pb.ListMessagesRequest(
      conversationId: conversationId,
      pageSize: messagePageSize,
      afterSeq: afterSeq,
    ),
  );

  Future<pb.Message> sendMessage(
    String conversationId,
    OutgoingContent content,
  ) async {
    final response = await _client.sendMessage(
      pb.SendMessageRequest(
        conversationId: conversationId,
        requestId: content.requestId,
        content: content.toProto(),
      ),
    );
    return response.message;
  }

  /// 對一家店送出：沒有對話時伺服器開新對話。
  Future<pb.Message> sendMessageToStore(
    String storeId,
    OutgoingContent content,
  ) async {
    final response = await _client.sendMessageToStore(
      pb.SendMessageToStoreRequest(
        storeId: storeId,
        requestId: content.requestId,
        content: content.toProto(),
      ),
    );
    return response.message;
  }

  Future<void> markRead(String conversationId, Int64 uptoSeq) async {
    await _client.markConversationRead(
      pb.MarkConversationReadRequest(
        conversationId: conversationId,
        uptoSeq: uptoSeq,
      ),
    );
  }

  Future<void> markAllRead() async {
    await _client.markAllRead(pb.MarkAllReadRequest());
  }

  Future<int> getUnreadCount() async {
    final response = await _client.getUnreadCount(pb.GetUnreadCountRequest());
    return response.unreadCount.toInt();
  }

  /// 對方是否在線，每個對話一項；一次最多 [maxCursors] 個。
  Future<Map<String, bool>> getPresence(List<String> conversationIds) async {
    final response = await _client.getPresence(
      pb.GetPresenceRequest(conversationIds: conversationIds),
    );
    return {for (final p in response.presences) p.conversationId: p.online};
  }

  /// 即時事件串流；[cursors] 是每個對話已有的最後一個 seq，最多 [maxCursors] 個。
  Stream<pb.ChatEvent> streamEvents(Map<String, Int64> cursors) => _client
      .streamEvents(pb.StreamEventsRequest(cursors: cursors.entries))
      .map((response) => response.event);
}
