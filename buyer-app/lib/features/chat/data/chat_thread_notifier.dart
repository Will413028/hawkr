import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/error_reason.dart';
import '../../../shared/utils/hk_log.dart';
import 'chat_realtime.dart';
import 'chat_repository.dart';

part 'chat_thread_notifier.g.dart';

/// 一個對話的定位：從對話列表或推播進來有 [conversationId]；從店面或訂單進來只有 [storeId]
/// （對話可能還不存在，第一則訊息送出時由伺服器開）。
class ChatThreadArg {
  const ChatThreadArg({this.storeId, this.conversationId});

  final String? storeId;
  final String? conversationId;

  @override
  bool operator ==(Object other) =>
      other is ChatThreadArg &&
      other.storeId == storeId &&
      other.conversationId == conversationId;

  @override
  int get hashCode => Object.hash(storeId, conversationId);

  @override
  String toString() =>
      'ChatThreadArg(storeId: $storeId, conversationId: $conversationId)';
}

/// 還沒有伺服器訊息的送出：送出中或失敗。重試用同一個 [content]（同一個 request_id）。
class PendingSend {
  const PendingSend({
    required this.content,
    this.previewUrl,
    this.failed = false,
  });

  final OutgoingContent content;

  /// 圖片的本機預覽網址；文字為 null。
  final String? previewUrl;
  final bool failed;

  String get requestId => content.requestId;

  PendingSend copyWith({bool? failed}) => PendingSend(
    content: content,
    previewUrl: previewUrl,
    failed: failed ?? this.failed,
  );
}

enum ChatLoadStatus { loading, loaded, failed }

class ChatThreadState {
  const ChatThreadState({
    this.loadStatus = ChatLoadStatus.loading,
    this.loadError,
    this.conversation,
    this.conversationId,
    this.messages = const [],
    this.pending = const [],
    this.hasMoreOlder = false,
    this.peerReadSeq = Int64.ZERO,
    this.syncError,
    this.peerOnline = false,
    this.readOnlyMessage,
  });

  final ChatLoadStatus loadStatus;
  final Object? loadError;

  /// 列表裡的這個對話（店家卡片）；對話還不存在時為 null。
  final pb.Conversation? conversation;
  final String? conversationId;

  /// 已載入的訊息，seq 由小到大。
  final List<pb.Message> messages;
  final List<PendingSend> pending;
  final bool hasMoreOlder;
  final Int64 peerReadSeq;

  /// 補洞或重新整理失敗的錯誤；下一次成功就清掉。
  final Object? syncError;
  final bool peerOnline;

  /// 對方封鎖或已刪除帳號（`CHAT_BLOCKED`、`RECEIVER_DELETED`）時伺服器給的說明；輸入框改為唯讀。
  final String? readOnlyMessage;

  Int64 get maxSeq => messages.isEmpty ? Int64.ZERO : messages.last.seq;

  ChatThreadState copyWith({
    ChatLoadStatus? loadStatus,
    Object? loadError,
    bool clearLoadError = false,
    pb.Conversation? conversation,
    String? conversationId,
    List<pb.Message>? messages,
    List<PendingSend>? pending,
    bool? hasMoreOlder,
    Int64? peerReadSeq,
    Object? syncError,
    bool clearSyncError = false,
    bool? peerOnline,
    String? readOnlyMessage,
  }) => ChatThreadState(
    loadStatus: loadStatus ?? this.loadStatus,
    loadError: clearLoadError ? null : (loadError ?? this.loadError),
    conversation: conversation ?? this.conversation,
    conversationId: conversationId ?? this.conversationId,
    messages: messages ?? this.messages,
    pending: pending ?? this.pending,
    hasMoreOlder: hasMoreOlder ?? this.hasMoreOlder,
    peerReadSeq: peerReadSeq ?? this.peerReadSeq,
    syncError: clearSyncError ? null : (syncError ?? this.syncError),
    peerOnline: peerOnline ?? this.peerOnline,
    readOnlyMessage: readOnlyMessage ?? this.readOnlyMessage,
  );
}

/// [messages] 併入 [incoming]（同 seq 以 [incoming] 為準），依 seq 由小到大。
List<pb.Message> mergeMessages(
  List<pb.Message> messages,
  Iterable<pb.Message> incoming,
) {
  final bySeq = {for (final m in messages) m.seq: m};
  for (final m in incoming) {
    bySeq[m.seq] = m;
  }
  return bySeq.values.toList()..sort((a, b) => a.seq.compareTo(b.seq));
}

@riverpod
class ChatThread extends _$ChatThread {
  StreamSubscription<pb.ChatEvent>? _eventSub;
  Future<void> _tail = Future.value();
  bool _adopting = false;
  bool _loadingOlder = false;
  bool _viewing = false;
  bool _missedWhileLoading = false;
  final _foreignConversations = <String>{};

  @override
  ChatThreadState build(ChatThreadArg arg) {
    final realtime = ref.read(chatRealtimeProvider);
    final repo = ref.read(chatRepositoryProvider);
    realtime.acquire();
    _eventSub = realtime.events.listen(_onEvent);
    ref.onDispose(() {
      unawaited(_eventSub?.cancel());
      realtime.release();
    });
    unawaited(_load(repo));
    return ChatThreadState(conversationId: arg.conversationId);
  }

  ChatRepository get _repo => ref.read(chatRepositoryProvider);

  void _update(ChatThreadState Function(ChatThreadState s) f) {
    if (!ref.mounted) return;
    state = f(state);
  }

  Future<T> _serial<T>(Future<T> Function() task) {
    final result = _tail.then((_) => task());
    _tail = result.then((_) {}, onError: (_) {});
    return result;
  }

  bool _matches(pb.Conversation c) =>
      (arg.conversationId != null && c.id == arg.conversationId) ||
      (arg.storeId != null && c.hasStore() && c.store.id == arg.storeId);

  Future<void> _load(ChatRepository repo) async {
    try {
      final conversations = await repo.listAllConversations();
      pb.Conversation? found;
      for (final c in conversations) {
        if (_matches(c)) {
          found = c;
          break;
        }
      }
      if (found == null) {
        if (arg.conversationId != null && state.conversationId != null) {
          // 指定了對話 ID 卻不在列表裡：讓 ListMessages 回覆（不存在或不是自己的是 not_found）。
          await _loadLatest(arg.conversationId!, null);
          return;
        }
        _update(
          (s) => s.copyWith(
            loadStatus: ChatLoadStatus.loaded,
            clearLoadError: true,
          ),
        );
        return;
      }
      await _loadLatest(found.id, found);
    } catch (e) {
      _update(
        (s) => s.copyWith(loadStatus: ChatLoadStatus.failed, loadError: e),
      );
    }
  }

  Future<void> _loadLatest(String id, pb.Conversation? conversation) async {
    final page = await _repo.listMessagesBefore(id);
    final ordered = page.messages.reversed.toList();
    _update(
      (s) => s.copyWith(
        loadStatus: ChatLoadStatus.loaded,
        clearLoadError: true,
        conversation: conversation,
        conversationId: id,
        messages: mergeMessages(s.messages, ordered),
        hasMoreOlder:
            (s.messages.isEmpty ||
                (ordered.isNotEmpty &&
                    ordered.first.seq <= s.messages.first.seq))
            ? page.hasMore
            : null,
        peerReadSeq: _maxSeq(s.peerReadSeq, page.peerReadSeq),
        clearSyncError: true,
      ),
    );
    ref.read(chatRealtimeProvider).noteSeq(id, state.maxSeq);
    _markReadIfNeeded();
    if (_missedWhileLoading) {
      _missedWhileLoading = false;
      unawaited(_catchUp());
    }
  }

  /// 畫面是不是正在給使用者看（App 在前景且這個路由在最上面）。只有這時才自動標記已讀；
  /// 回到可見時補標記。
  void setViewing(bool viewing) {
    if (_viewing == viewing) return;
    _viewing = viewing;
    if (viewing) _markReadIfNeeded();
  }

  /// 初次載入失敗後的重試。
  Future<void> reload() async {
    _update(
      (s) =>
          s.copyWith(loadStatus: ChatLoadStatus.loading, clearLoadError: true),
    );
    await _load(_repo);
  }

  Int64 _maxSeq(Int64 a, Int64 b) => a >= b ? a : b;

  void _markReadIfNeeded() {
    final id = state.conversationId;
    if (!_viewing || id == null || state.maxSeq == Int64.ZERO) return;
    final upto = state.maxSeq;
    unawaited(
      _repo.markRead(id, upto).catchError((Object e) {
        HKLog.logD('[ChatThread] markRead failed: $e');
      }),
    );
  }

  // ── 即時事件 ────────────────────────────────────────────────────────────

  void _onEvent(pb.ChatEvent event) {
    switch (event.whichEvent()) {
      case pb.ChatEvent_Event.message:
        _onMessage(event.message);
      case pb.ChatEvent_Event.peerRead:
        if (event.peerRead.conversationId == state.conversationId) {
          _update(
            (s) => s.copyWith(
              peerReadSeq: _maxSeq(s.peerReadSeq, event.peerRead.peerReadSeq),
            ),
          );
        }
      case pb.ChatEvent_Event.resync:
        final r = event.resync;
        final mine =
            r.whichScope() == pb.Resync_Scope.all ||
            (r.whichScope() == pb.Resync_Scope.conversationId &&
                r.conversationId == state.conversationId);
        if (!mine && state.conversationId != null) return;
        _foreignConversations.clear();
        unawaited(_catchUp());
      case pb.ChatEvent_Event.ready:
        unawaited(_catchUp());
      case pb.ChatEvent_Event.unread:
      case pb.ChatEvent_Event.heartbeat:
      case pb.ChatEvent_Event.notSet:
        break;
    }
  }

  void _onMessage(pb.MessageEvent event) {
    final m = event.message;
    final current = state.conversationId;
    if (current == null) {
      unawaited(_adopt(m.conversationId));
      return;
    }
    if (m.conversationId != current) return;
    if (state.loadStatus != ChatLoadStatus.loaded) {
      _missedWhileLoading = true;
      return;
    }
    if (state.messages.any((x) => x.seq == m.seq)) {
      _update((s) => s.copyWith(messages: mergeMessages(s.messages, [m])));
      return;
    }
    // 比已載入的視窗更舊：不放進來（會變成第一則而讓載入更舊跳過中間），往上捲時由分頁讀到。
    if (state.messages.isNotEmpty && m.seq < state.messages.first.seq) return;
    if (state.messages.isNotEmpty && m.seq > state.maxSeq + 1) {
      // 序號有缺口：先補洞（補回來的頁含這則訊息）。
      unawaited(_catchUp());
      return;
    }
    _update((s) => s.copyWith(messages: mergeMessages(s.messages, [m])));
    if (m.sender == pb.ChatParty.CHAT_PARTY_STORE && !event.updated) {
      _markReadIfNeeded();
    }
  }

  /// 開在對話還不存在的串流上，收到某個對話的訊息：確認它是不是這家店的對話，是就載入。
  Future<void> _adopt(String conversationId) async {
    if (arg.storeId == null || _adopting) return;
    if (_foreignConversations.contains(conversationId)) return;
    _adopting = true;
    try {
      final conversations = await _repo.listAllConversations();
      for (final c in conversations) {
        if (c.id == conversationId && _matches(c)) {
          await _serial(() => _loadLatest(c.id, c));
          return;
        }
      }
      _foreignConversations.add(conversationId);
    } catch (e) {
      _update((s) => s.copyWith(syncError: e));
    } finally {
      _adopting = false;
    }
  }

  /// 以 after_seq 補到最新（讀完所有頁）；失敗記在 [ChatThreadState.syncError]。
  Future<void> _catchUp() async {
    try {
      await _serial(_fillGap);
    } catch (e) {
      _update((s) => s.copyWith(syncError: e));
    }
  }

  Future<void> _fillGap() async {
    final id = state.conversationId;
    if (id == null) {
      final storeId = arg.storeId;
      if (storeId == null || state.loadStatus != ChatLoadStatus.loaded) return;
      final conversations = await _repo.listAllConversations();
      for (final c in conversations) {
        if (_matches(c)) {
          await _loadLatest(c.id, c);
          return;
        }
      }
      return;
    }
    if (state.messages.isEmpty) {
      await _loadLatest(id, state.conversation);
      return;
    }
    var after = state.maxSeq;
    while (true) {
      final page = await _repo.listMessagesAfter(id, after);
      _update(
        (s) => s.copyWith(
          messages: mergeMessages(s.messages, page.messages),
          peerReadSeq: _maxSeq(s.peerReadSeq, page.peerReadSeq),
          clearSyncError: true,
        ),
      );
      if (!page.hasMore || page.messages.isEmpty) break;
      after = state.maxSeq;
    }
    ref.read(chatRealtimeProvider).noteSeq(id, state.maxSeq);
    _markReadIfNeeded();
  }

  // ── 使用者操作 ──────────────────────────────────────────────────────────

  /// 下拉重新整理：補到最新，不丟掉已載入的舊訊息；失敗丟出。
  Future<void> refresh() async {
    await _serial(_fillGap);
  }

  /// 載入更舊的一頁；回傳是否還有更舊的。失敗丟出（不改狀態）。
  Future<bool> loadOlder() {
    // 已經有一頁在讀就不再排隊：往上捲時捲動事件很密。
    if (_loadingOlder) return Future.value(state.hasMoreOlder);
    _loadingOlder = true;
    return _serial(() async {
      final id = state.conversationId;
      if (id == null || !state.hasMoreOlder || state.messages.isEmpty) {
        return false;
      }
      final page = await _repo.listMessagesBefore(
        id,
        beforeSeq: state.messages.first.seq,
      );
      _update(
        (s) => s.copyWith(
          messages: mergeMessages(s.messages, page.messages),
          hasMoreOlder: page.hasMore,
        ),
      );
      return state.hasMoreOlder;
    }).whenComplete(() => _loadingOlder = false);
  }

  Future<void> pollPresence() async {
    final id = state.conversationId;
    if (id == null) return;
    final presence = await _repo.getPresence([id]);
    _update((s) => s.copyWith(peerOnline: presence[id] ?? false));
  }

  Future<void> sendText(String text) => _send(OutgoingContent.text(text));

  Future<void> sendImage(String stagedToken, String previewUrl) =>
      _send(OutgoingContent.image(stagedToken), previewUrl: previewUrl);

  /// 重送失敗的那一則：同一個 request_id，伺服器已寫入就回第一次的訊息。
  Future<void> retry(String requestId) {
    final p = state.pending.where((x) => x.requestId == requestId).firstOrNull;
    if (p == null) return Future.value();
    return _send(p.content, previewUrl: p.previewUrl);
  }

  void discard(String requestId) {
    _update(
      (s) => s.copyWith(
        pending: [
          for (final p in s.pending)
            if (p.requestId != requestId) p,
        ],
      ),
    );
  }

  Future<void> _send(OutgoingContent content, {String? previewUrl}) async {
    final entry = PendingSend(content: content, previewUrl: previewUrl);
    _update(
      (s) => s.copyWith(
        pending: [
          for (final p in s.pending)
            if (p.requestId != content.requestId) p,
          entry,
        ],
      ),
    );
    try {
      final id = state.conversationId;
      final sent = id != null
          ? await _repo.sendMessage(id, content)
          : await _repo.sendMessageToStore(arg.storeId!, content);
      final hole =
          state.messages.isNotEmpty &&
          sent.conversationId == state.conversationId &&
          sent.seq > state.maxSeq + 1;
      if (hole) {
        // 自己的訊息跳過了別人的訊息：先從原本的最後一個 seq 補到最新，不把游標推過空洞。
        await _serial(_fillGap);
      }
      _update(
        (s) => s.copyWith(
          conversationId: sent.conversationId,
          messages: mergeMessages(s.messages, [sent]),
          pending: [
            for (final p in s.pending)
              if (p.requestId != content.requestId) p,
          ],
        ),
      );
      ref.read(chatRealtimeProvider).noteSeq(sent.conversationId, state.maxSeq);
    } catch (e) {
      final reason = errorReasonOf(e);
      if (reason == ErrorReason.CHAT_BLOCKED ||
          reason == ErrorReason.RECEIVER_DELETED) {
        final message = errorText(
          e,
          defaults: ref.read(defaultErrorTextProvider),
        );
        _update(
          (s) => s.copyWith(
            readOnlyMessage: message,
            pending: [
              for (final p in s.pending)
                if (p.requestId != content.requestId) p,
            ],
          ),
        );
      } else {
        _update(
          (s) => s.copyWith(
            pending: [
              for (final p in s.pending)
                p.requestId == content.requestId ? p.copyWith(failed: true) : p,
            ],
          ),
        );
      }
      rethrow;
    }
  }
}
