import 'dart:async';

import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../dashboard/data/dashboard_chat_unread_provider.dart';
import 'chat_model.dart';
import 'chat_repository.dart';
import 'chat_session.dart';
import 'chat_stream_client.dart';

part 'chat_thread_notifier.g.dart';

/// 一個對話頁的 provider key：[peerId] 是買家 ID（從客戶或訂單進來時給），[conversationId] 是從列表或推播進來時給；
/// 至少給一個。
class ChatThreadArg {
  const ChatThreadArg({this.peerId = '', this.conversationId});

  final String peerId;
  final String? conversationId;

  @override
  bool operator ==(Object other) =>
      other is ChatThreadArg &&
      other.peerId == peerId &&
      other.conversationId == conversationId;

  @override
  int get hashCode => Object.hash(peerId, conversationId);
}

/// 對話頁的資料。
class ChatThreadData {
  const ChatThreadData({
    required this.messages,
    required this.conversationId,
    required this.hasMoreOlder,
    required this.peerReadSeq,
    this.buyer,
  });

  /// 依 seq 由小到大，本機送出中與失敗的訊息排在最後。
  final List<ChatMessage> messages;

  /// 對話 ID；買家與這家店還沒有對話時是 null，第一則訊息送出後才有。
  final String? conversationId;
  final bool hasMoreOlder;
  final int peerReadSeq;

  /// 對方買家的名稱與頭像（取自對話列表）；找不到對話時是 null。
  final ChatBuyerInfo? buyer;

  /// 從已載入的最舊一則起、seq 連續到哪裡；中間有缺口時停在缺口前，補洞從這裡往後讀。
  int get contiguousLastSeq {
    var last = 0;
    for (final m in messages) {
      if (m.seq <= 0) continue;
      if (last == 0 || m.seq == last + 1) {
        last = m.seq;
      } else if (m.seq > last + 1) {
        break;
      }
    }
    return last;
  }

  /// 已確定在伺服器上的最後一個 seq。
  int get lastSeq {
    var s = 0;
    for (final m in messages) {
      if (m.seq > s) s = m.seq;
    }
    return s;
  }

  ChatThreadData copyWith({
    List<ChatMessage>? messages,
    String? conversationId,
    bool? hasMoreOlder,
    int? peerReadSeq,
  }) => ChatThreadData(
    messages: messages ?? this.messages,
    conversationId: conversationId ?? this.conversationId,
    hasMoreOlder: hasMoreOlder ?? this.hasMoreOlder,
    peerReadSeq: peerReadSeq ?? this.peerReadSeq,
    buyer: buyer,
  );
}

class _SendJob {
  const _SendJob({this.text, this.imageStagedToken});
  final String? text;
  final String? imageStagedToken;
}

/// 讀取失敗立刻顯示錯誤與重試，不走 Riverpod 的自動重試（它會讓畫面一直停在載入中）。
Duration? _noAutoRetry(int retryCount, Object error) => null;

@Riverpod(retry: _noAutoRetry)
class ChatThread extends _$ChatThread {
  static const _uuid = Uuid();

  final Map<String, _SendJob> _jobs = {};
  bool _loadingOlder = false;
  bool _fillingGap = false;
  StreamSubscription<ChatStreamEvent>? _sub;

  ChatRepository get _repo => ref.read(chatRepositoryProvider);
  ChatSession get _session => ref.read(chatSessionProvider);

  @override
  Future<ChatThreadData> build(ChatThreadArg arg) async {
    unawaited(_sub?.cancel());
    _sub = _session.events.listen(_onEvent);
    ref.onDispose(() => _sub?.cancel());

    if (arg.peerId.isEmpty && (arg.conversationId ?? '').isEmpty) {
      throw ArgumentError('chat thread needs a peerId or a conversationId');
    }
    // 契約沒有依買家讀對話的方法：先看列表載入時記下的，沒有再逐頁找。
    var conv = _session.conversationFor(
      conversationId: arg.conversationId,
      buyerId: arg.peerId,
    );
    conv ??= (arg.conversationId ?? '').isNotEmpty
        ? await _repo.findConversation(conversationId: arg.conversationId)
        : await _repo.findConversation(buyerId: arg.peerId);
    final convId = conv?.id ?? arg.conversationId;
    if (convId == null || convId.isEmpty) {
      return const ChatThreadData(
        messages: [],
        conversationId: null,
        hasMoreOlder: false,
        peerReadSeq: 0,
      );
    }
    final page = await _repo.listOlder(convId);
    final data = ChatThreadData(
      messages: page.messages,
      conversationId: convId,
      hasMoreOlder: page.hasMore,
      peerReadSeq: page.peerReadSeq,
      buyer: conv?.buyer,
    );
    _session.client.noteSeq(convId, data.lastSeq);
    // build 回傳之後 state 才有值，所以下一輪事件迴圈再標已讀。
    unawaited(Future<void>(markRead));
    return data;
  }

  String? get conversationId => state.value?.conversationId;

  // ── 事件 ───────────────────────────────────────────────────────────────

  void _onEvent(ChatStreamEvent e) {
    final data = state.value;
    final conv = data?.conversationId;
    if (data == null) return;
    if (conv == null) {
      // 對話還不存在時開著這頁：買家（或另一台裝置）送出第一則訊息後對話才出現，認得是這個買家的就接上。
      if (e case ChatMessageArrived(:final message)) {
        unawaited(_adoptConversation(message));
      }
      return;
    }
    switch (e) {
      case ChatMessageArrived(:final message, :final updated):
        if (message.conversationId != conv) return;
        _applyMessage(message);
        if (!updated && !message.fromStore) markRead();
        _checkGap(message);
      case ChatPeerRead(:final conversationId, :final peerReadSeq):
        if (conversationId != conv) return;
        if (peerReadSeq > data.peerReadSeq) {
          state = AsyncData(data.copyWith(peerReadSeq: peerReadSeq));
        }
      case ChatResync(:final conversationId):
        if (conversationId == null || conversationId == conv) {
          unawaited(fillGap());
        }
      case ChatReady():
      case ChatUnreadChanged():
        break;
    }
  }

  bool _adopting = false;

  Future<void> _adoptConversation(ChatMessage m) async {
    if (_adopting || arg.peerId.isEmpty) return;
    _adopting = true;
    try {
      final found = await _repo.findConversation(buyerId: arg.peerId);
      final cur = state.value;
      if (!ref.mounted ||
          found == null ||
          found.id != m.conversationId ||
          cur == null ||
          cur.conversationId != null) {
        return;
      }
      state = AsyncData(
        ChatThreadData(
          messages: cur.messages,
          conversationId: found.id,
          hasMoreOlder: cur.hasMoreOlder,
          peerReadSeq: cur.peerReadSeq,
          buyer: found.buyer,
        ),
      );
      _applyMessage(m);
      if (!m.fromStore) markRead();
      _session.client.noteSeq(found.id, m.seq);
      _checkGap(m);
    } on Object {
      // 認不出來就維持空對話；之後的事件會再試。
    } finally {
      _adopting = false;
    }
  }

  void _checkGap(ChatMessage m) {
    final data = state.value;
    if (data == null || m.seq <= 0) return;
    if (m.seq > data.contiguousLastSeq + 1 && data.contiguousLastSeq > 0) {
      unawaited(fillGap());
    }
  }

  /// 補 seq 缺口：讀 `after_seq` 之後的訊息併進來（重連、Resync 與偵測到序號跳號時）。
  Future<void> fillGap() async {
    final data = state.value;
    final conv = data?.conversationId;
    if (data == null || conv == null || _fillingGap) return;
    _fillingGap = true;
    try {
      final page = await _repo.listAfter(conv, data.contiguousLastSeq);
      if (!ref.mounted) return;
      for (final m in page.messages) {
        _applyMessage(m);
      }
      final cur = state.value;
      if (cur != null && page.peerReadSeq > cur.peerReadSeq) {
        state = AsyncData(cur.copyWith(peerReadSeq: page.peerReadSeq));
      }
      _session.client.noteSeq(conv, state.value?.contiguousLastSeq ?? 0);
    } on Object {
      // 補洞失敗不蓋掉畫面上已有的訊息；下一次事件或重連會再補。
    } finally {
      _fillingGap = false;
    }
  }

  /// 併入一則伺服器的訊息：同 id 或同 seq 取代（可見度改變），沒有就依 seq 插入。送出中的訊息在送出回應時依
  /// requestId 移除，所以自己的訊息先經串流回來也不會重複。
  void _applyMessage(ChatMessage m) {
    final data = state.value;
    if (data == null) return;
    final list = [...data.messages];
    final byId = list.indexWhere((x) => x.id == m.id);
    final bySeq = m.seq > 0 ? list.indexWhere((x) => x.seq == m.seq) : -1;
    if (byId >= 0) {
      list[byId] = m;
    } else if (bySeq >= 0) {
      list[bySeq] = m;
    } else {
      list.add(m);
    }
    state = AsyncData(data.copyWith(messages: _sorted(list)));
  }

  List<ChatMessage> _sorted(List<ChatMessage> list) {
    final sent = list.where((m) => m.seq > 0).toList()
      ..sort((a, b) => a.seq.compareTo(b.seq));
    final local = list.where((m) => m.seq == 0).toList();
    return [...sent, ...local];
  }

  // ── 公開操作 ───────────────────────────────────────────────────────────

  /// 重新載入（錯誤畫面的「重試」）。
  Future<void> reload() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() => build(arg));
  }

  /// 往舊的方向載入一頁；回傳是否多了訊息。
  Future<bool> loadOlder() async {
    final data = state.value;
    final conv = data?.conversationId;
    if (data == null || conv == null || !data.hasMoreOlder || _loadingOlder) {
      return false;
    }
    final first = data.messages.where((m) => m.seq > 0).firstOrNull;
    if (first == null) return false;
    _loadingOlder = true;
    try {
      final page = await _repo.listOlder(conv, beforeSeq: first.seq);
      if (!ref.mounted) return false;
      final cur = state.value ?? data;
      final ids = cur.messages.map((m) => m.id).toSet();
      final older = page.messages.where((m) => ids.add(m.id)).toList();
      state = AsyncData(
        cur.copyWith(
          messages: _sorted([...older, ...cur.messages]),
          hasMoreOlder: page.hasMore,
        ),
      );
      return older.isNotEmpty;
    } on Object {
      return false;
    } finally {
      _loadingOlder = false;
    }
  }

  /// 把這一方的已讀游標推到目前最後一則買家訊息。
  void markRead() {
    final data = state.value;
    final conv = data?.conversationId;
    if (data == null || conv == null) return;
    final lastPeer = data.messages
        .where((m) => !m.fromStore && m.seq > 0)
        .map((m) => m.seq)
        .fold<int>(0, (a, b) => a > b ? a : b);
    if (lastPeer == 0) return;
    unawaited(
      _repo.markRead(conv, lastPeer).catchError((Object _) {}).whenComplete(() {
        if (ref.mounted) ref.invalidate(dashboardChatUnreadProvider);
      }),
    );
  }

  /// 送出文字。送出中的訊息立刻出現在畫面上；失敗時留在畫面上標成失敗，並把錯誤丟給呼叫端顯示。
  Future<void> sendText(String text) => _send(
    _SendJob(text: text),
    pending: (rid, conv) => ChatMessage.pending(
      requestId: rid,
      conversationId: conv,
      kind: pb.MessageKind.MESSAGE_KIND_TEXT,
      text: text,
    ),
  );

  /// 送出圖片：[stagedToken] 是 media 上傳完成後的 staged token，[previewUri] 是送出中顯示的本機預覽。
  Future<void> sendImage(String stagedToken, {String? previewUri}) => _send(
    _SendJob(imageStagedToken: stagedToken),
    pending: (rid, conv) => ChatMessage.pending(
      requestId: rid,
      conversationId: conv,
      kind: pb.MessageKind.MESSAGE_KIND_IMAGE,
      imageUri: previewUri,
    ),
  );

  /// 重送一則失敗的訊息：帶同一個 request_id，伺服器已收過就回第一次的那則。
  Future<void> retry(String requestId) async {
    final data = state.value;
    final job = _jobs[requestId];
    if (data == null || job == null) return;
    _setSendState(requestId, ChatSendState.sending);
    await _dispatch(requestId, job);
  }

  Future<void> _send(
    _SendJob job, {
    required ChatMessage Function(String requestId, String conv) pending,
  }) async {
    final data = state.value;
    if (data == null) throw StateError('chat thread is not loaded');
    final rid = _uuid.v4();
    _jobs[rid] = job;
    state = AsyncData(
      data.copyWith(
        messages: [...data.messages, pending(rid, data.conversationId ?? '')],
      ),
    );
    await _dispatch(rid, job);
  }

  Future<void> _dispatch(String requestId, _SendJob job) async {
    final conv = state.value?.conversationId;
    try {
      final sent = (conv == null || conv.isEmpty)
          ? await _repo.sendToBuyer(
              arg.peerId,
              requestId: requestId,
              text: job.text,
              imageStagedToken: job.imageStagedToken,
            )
          : await _repo.send(
              conv,
              requestId: requestId,
              text: job.text,
              imageStagedToken: job.imageStagedToken,
            );
      if (!ref.mounted) return;
      _jobs.remove(requestId);
      final cur = state.value!;
      final list = cur.messages.where((m) => m.requestId != requestId).toList();
      state = AsyncData(
        cur.copyWith(
          messages: list,
          conversationId: cur.conversationId ?? sent.conversationId,
        ),
      );
      _applyMessage(sent);
      _session.client.noteSeq(sent.conversationId, sent.seq);
    } on Object {
      if (ref.mounted) _setSendState(requestId, ChatSendState.failed);
      rethrow;
    }
  }

  void _setSendState(String requestId, ChatSendState s) {
    final data = state.value;
    if (data == null) return;
    state = AsyncData(
      data.copyWith(
        messages: [
          for (final m in data.messages)
            m.requestId == requestId ? m.copyWith(sendState: s) : m,
        ],
      ),
    );
  }

  /// 放棄一則失敗的訊息。
  void discard(String requestId) {
    _jobs.remove(requestId);
    final data = state.value;
    if (data == null) return;
    state = AsyncData(
      data.copyWith(
        messages: data.messages.where((m) => m.requestId != requestId).toList(),
      ),
    );
  }
}
