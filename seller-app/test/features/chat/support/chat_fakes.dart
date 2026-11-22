import 'dart:async';

import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.client.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_session.dart';
import 'package:hawkr_seller/features/chat/data/chat_stream_client.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as wkt;

/// int64 欄位的值（生成碼用 `Int64`，App 不直接依賴 fixnum；欄位預設值加上去取得）。
final _zero = pb.GetUnreadCountResponse().unreadCount;
dynamic i64(int v) => _zero + v;

/// 沒有任何方法的 client：腳本化的 repository 覆寫每個方法，不會真的呼叫它。
ChatAppServiceClient idleChatClient() =>
    ChatAppServiceClient(FakeTransportBuilder().build());

pb.Message protoMessage(
  int seq, {
  String conv = 'c1',
  bool store = false,
  String? text,
  bool hidden = false,
  pb.MessageKind kind = pb.MessageKind.MESSAGE_KIND_TEXT,
}) => pb.Message(
  id: 'm$seq',
  conversationId: conv,
  seq: i64(seq),
  sender: store ? pb.ChatParty.CHAT_PARTY_STORE : pb.ChatParty.CHAT_PARTY_BUYER,
  kind: kind,
  text: hidden ? null : (text ?? 'hello $seq'),
  hidden: hidden,
);

ChatMessage clientMessage(int seq, {String conv = 'c1', bool store = false}) =>
    ChatMessage.fromProto(protoMessage(seq, conv: conv, store: store));

pb.ChatEvent readyFrame({int heartbeatSeconds = 10, int unread = 0}) =>
    pb.ChatEvent(
      ready: pb.Ready(
        unreadCount: i64(unread),
        heartbeatInterval: wkt.Duration(seconds: i64(heartbeatSeconds)),
      ),
    );

pb.ChatEvent heartbeatFrame() => pb.ChatEvent(heartbeat: pb.Heartbeat());

pb.ChatEvent messageFrame(pb.Message m, {bool updated = false}) => pb.ChatEvent(
  message: pb.MessageEvent(message: m, updated: updated),
);

pb.Conversation protoConversation(
  String id,
  String buyerId, {
  String? name,
  int unread = 0,
}) => pb.Conversation(
  id: id,
  buyer: pb.ChatBuyer(id: buyerId, name: name),
  unreadCount: i64(unread),
);

/// 腳本化的 repository：不碰網路，記下每次呼叫。
class FakeChatRepository extends ChatRepository {
  FakeChatRepository() : super(idleChatClient());

  List<ChatConversation> conversations = [];
  Object? listConversationsError;

  /// 設了就依 page token 回頁（第一頁的 token 是空字串）；[pageGates] 對應的 Completer 完成前該頁不回。
  Map<String, ConversationPage>? pages;
  final pageGates = <String, Completer<void>>{};
  Object? listOlderError;
  List<ChatMessage> history = [];
  bool hasMore = false;
  int peerRead = 0;

  final sends =
      <({String? convId, String? buyerId, String requestId, String? text})>[];
  final marks = <(String, int)>[];
  final afterCalls = <(String, int)>[];
  final beforeCalls = <int?>[];
  List<ChatMessage> afterResult = [];
  Object? sendError;
  int findCalls = 0;

  @override
  Future<ConversationPage> listConversations({String pageToken = ''}) async {
    final err = listConversationsError;
    if (err != null) throw err;
    final byToken = pages;
    if (byToken != null) {
      await pageGates[pageToken]?.future;
      return byToken[pageToken]!;
    }
    return ConversationPage(items: conversations, nextPageToken: '');
  }

  @override
  Future<ChatConversation?> findConversation({
    String? buyerId,
    String? conversationId,
  }) async {
    findCalls++;
    final err = listConversationsError;
    if (err != null) throw err;
    for (final c in conversations) {
      if (buyerId != null ? c.buyer.id == buyerId : c.id == conversationId) {
        return c;
      }
    }
    return null;
  }

  @override
  Future<MessagePage> listOlder(String conversationId, {int? beforeSeq}) async {
    beforeCalls.add(beforeSeq);
    final err = listOlderError;
    if (err != null) throw err;
    final list = beforeSeq == null
        ? history
        : history.where((m) => m.seq < beforeSeq).toList();
    return MessagePage(messages: list, hasMore: hasMore, peerReadSeq: peerRead);
  }

  @override
  Future<MessagePage> listAfter(String conversationId, int afterSeq) async {
    afterCalls.add((conversationId, afterSeq));
    return MessagePage(
      messages: afterResult,
      hasMore: false,
      peerReadSeq: peerRead,
    );
  }

  @override
  Future<ChatMessage> send(
    String conversationId, {
    required String requestId,
    String? text,
    String? imageStagedToken,
  }) async {
    sends.add((
      convId: conversationId,
      buyerId: null,
      requestId: requestId,
      text: text,
    ));
    final err = sendError;
    if (err != null) throw err;
    return clientMessage(
      history.length + 100,
      conv: conversationId,
      store: true,
    );
  }

  @override
  Future<ChatMessage> sendToBuyer(
    String buyerId, {
    required String requestId,
    String? text,
    String? imageStagedToken,
  }) async {
    sends.add((
      convId: null,
      buyerId: buyerId,
      requestId: requestId,
      text: text,
    ));
    final err = sendError;
    if (err != null) throw err;
    return clientMessage(1, conv: 'new-conv', store: true);
  }

  @override
  Future<void> markRead(String conversationId, int uptoSeq) async {
    marks.add((conversationId, uptoSeq));
  }

  int unread = 0;

  /// 事件串流的腳本；沒設就是一條永不出事件也不結束的串流。
  Stream<pb.ChatEvent> Function(Map<String, int> cursors)? onStream;

  @override
  Stream<pb.ChatEvent> stream(Map<String, int> cursors) =>
      onStream?.call(cursors) ?? StreamController<pb.ChatEvent>().stream;
  int unreadReads = 0;

  @override
  Future<int> unreadCount() async {
    unreadReads++;
    return unread;
  }

  @override
  Future<Map<String, bool>> presence(List<String> ids) async => {
    for (final id in ids) id: true,
  };
}

/// 沒有啟動串流的 session（列表與對話頁測試只需要它的資料入口）。
ChatSession idleSession(ChatRepository repo) => ChatSession(
  repo,
  () => UiLocale.zhTW,
  client: ChatStreamClient(open: (_) => const Stream<pb.ChatEvent>.empty()),
);
