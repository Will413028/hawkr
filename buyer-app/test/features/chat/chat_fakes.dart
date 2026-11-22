import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/features/chat/data/chat_repository.dart';

pb.Message chatMessage(
  String conversationId,
  int seq, {
  String text = '',
  pb.ChatParty sender = pb.ChatParty.CHAT_PARTY_STORE,
  pb.MessageKind kind = pb.MessageKind.MESSAGE_KIND_TEXT,
  bool hidden = false,
  bool erased = false,
}) => pb.Message(
  id: 'msg-$conversationId-$seq',
  conversationId: conversationId,
  seq: Int64(seq),
  sender: sender,
  kind: kind,
  text: kind == pb.MessageKind.MESSAGE_KIND_TEXT && !hidden && !erased
      ? (text.isEmpty ? 'm$seq' : text)
      : null,
  hidden: hidden,
  erased: erased,
);

pb.Conversation chatConversation(
  String id, {
  String storeId = '7',
  String storeName = '好店',
  int unread = 0,
}) => pb.Conversation(
  id: id,
  store: StoreCard(id: storeId, name: storeName),
  unreadCount: Int64(unread),
);

pb.ChatEvent messageEvent(pb.Message m, {bool updated = false}) => pb.ChatEvent(
  message: pb.MessageEvent(message: m, updated: updated),
);

pb.ChatEvent readyEvent({int unread = 0, int heartbeatMs = 15000}) =>
    pb.ChatEvent()..mergeFromProto3Json({
      'ready': {
        'unreadCount': '$unread',
        'heartbeatInterval': '${heartbeatMs / 1000}s',
      },
    });

pb.ChatEvent unreadEvent(int n) =>
    pb.ChatEvent(unread: pb.UnreadChanged(unreadCount: Int64(n)));

pb.ChatEvent peerReadEvent(String conversationId, int seq) => pb.ChatEvent(
  peerRead: pb.PeerRead(
    conversationId: conversationId,
    peerReadSeq: Int64(seq),
  ),
);

pb.ChatEvent resyncAllEvent() => pb.ChatEvent(resync: pb.Resync(all: true));

/// 假的聊天後端：對話與訊息放在記憶體，`ListMessages` 以 [pageSize] 分頁，送出以 request_id 冪等，
/// 串流每次連線給一個可手動推事件的 controller。
class FakeChatRepository extends ChatRepository {
  FakeChatRepository({this.pageSize = 2})
    : super(ChatAppServiceClient(FakeTransportBuilder().build()));

  final int pageSize;
  List<pb.Conversation> conversations = [];
  final Map<String, List<pb.Message>> messages = {};
  final Map<String, Int64> peerRead = {};

  /// 每次 `ListMessages` 的請求（before 或 after）。
  final List<({String conversationId, Int64? before, Int64? after})> listCalls =
      [];
  int listConversationCalls = 0;
  final List<({String conversationId, String requestId})> sends = [];
  final Map<String, pb.Message> _byRequestId = {};
  final List<(String, Int64)> reads = [];

  /// 下一次送出（依序消耗）要丟的錯誤。
  final List<Object> sendErrors = [];

  /// 讓 `ListMessages` 失敗。
  Object? listMessagesError;

  /// 讓 after 方向的 `ListMessages` 先等這個 completer。
  Completer<void>? holdAfter;

  final List<Map<String, Int64>> connectCursors = [];
  final List<StreamController<pb.ChatEvent>> streams = [];
  Object? streamError;

  /// 讓 `listAllConversations` 先等這個 completer（模擬初次載入還沒完成）。
  Completer<void>? holdConversations;

  @override
  Future<List<pb.Conversation>> listAllConversations() async {
    listConversationCalls++;
    if (holdConversations != null) await holdConversations!.future;
    return List.of(conversations);
  }

  void addMessages(String id, Iterable<pb.Message> ms) =>
      messages.putIfAbsent(id, () => []).addAll(ms);

  Int64 _peer(String id) => peerRead[id] ?? Int64.ZERO;

  @override
  Future<pb.ListMessagesResponse> listMessagesBefore(
    String conversationId, {
    Int64? beforeSeq,
  }) async {
    listCalls.add((
      conversationId: conversationId,
      before: beforeSeq,
      after: null,
    ));
    if (listMessagesError != null) throw listMessagesError!;
    final all = messages[conversationId] ?? const <pb.Message>[];
    final older = [
      for (final m in all)
        if (beforeSeq == null || m.seq < beforeSeq) m,
    ];
    final page = older.reversed.take(pageSize).toList();
    return pb.ListMessagesResponse(
      messages: page,
      hasMore: older.length > page.length,
      peerReadSeq: _peer(conversationId),
    );
  }

  @override
  Future<pb.ListMessagesResponse> listMessagesAfter(
    String conversationId,
    Int64 afterSeq,
  ) async {
    listCalls.add((
      conversationId: conversationId,
      before: null,
      after: afterSeq,
    ));
    if (holdAfter != null) await holdAfter!.future;
    if (listMessagesError != null) throw listMessagesError!;
    final all = messages[conversationId] ?? const <pb.Message>[];
    final newer = [
      for (final m in all)
        if (m.seq > afterSeq) m,
    ];
    final page = newer.take(pageSize).toList();
    return pb.ListMessagesResponse(
      messages: page,
      hasMore: newer.length > page.length,
      peerReadSeq: _peer(conversationId),
    );
  }

  pb.Message _store(String conversationId, OutgoingContent content) {
    final existing = _byRequestId[content.requestId];
    if (existing != null) return existing;
    final list = messages.putIfAbsent(conversationId, () => []);
    final m = chatMessage(
      conversationId,
      list.length + 1,
      text: content.text ?? '',
      sender: pb.ChatParty.CHAT_PARTY_BUYER,
      kind: content.text != null
          ? pb.MessageKind.MESSAGE_KIND_TEXT
          : pb.MessageKind.MESSAGE_KIND_IMAGE,
    );
    list.add(m);
    _byRequestId[content.requestId] = m;
    return m;
  }

  @override
  Future<pb.Message> sendMessage(
    String conversationId,
    OutgoingContent content,
  ) async {
    sends.add((conversationId: conversationId, requestId: content.requestId));
    if (sendErrors.isNotEmpty) throw sendErrors.removeAt(0);
    return _store(conversationId, content);
  }

  @override
  Future<pb.Message> sendMessageToStore(
    String storeId,
    OutgoingContent content,
  ) async {
    sends.add((conversationId: 'store:$storeId', requestId: content.requestId));
    if (sendErrors.isNotEmpty) throw sendErrors.removeAt(0);
    var conv = conversations.where((c) => c.store.id == storeId).firstOrNull;
    if (conv == null) {
      conv = chatConversation('conv-new', storeId: storeId);
      conversations = [...conversations, conv];
    }
    return _store(conv.id, content);
  }

  @override
  Future<void> markRead(String conversationId, Int64 uptoSeq) async {
    reads.add((conversationId, uptoSeq));
  }

  @override
  Future<int> getUnreadCount() async => 0;

  @override
  Future<Map<String, bool>> getPresence(List<String> conversationIds) async => {
    for (final id in conversationIds) id: true,
  };

  @override
  Stream<pb.ChatEvent> streamEvents(Map<String, Int64> cursors) {
    connectCursors.add(Map.of(cursors));
    final c = StreamController<pb.ChatEvent>();
    streams.add(c);
    if (streamError != null) {
      final e = streamError!;
      scheduleMicrotask(() {
        c.addError(e);
        c.close();
      });
    }
    return c.stream;
  }

  StreamController<pb.ChatEvent> get lastStream => streams.last;
}

Future<void> settle([int ms = 30]) =>
    Future<void>.delayed(Duration(milliseconds: ms));
