import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../dashboard/data/dashboard_chat_unread_provider.dart';
import 'chat_model.dart';
import 'chat_repository.dart';

part 'chat_thread_notifier.g.dart';

/// Key for a ChatThread provider family instance.
/// Equality: both fields must match so full-screen and embedded sharing same
/// peerId+conversationId reuse the same cached notifier.
class ChatThreadArg {
  final String peerId;
  final String? conversationId;

  const ChatThreadArg({required this.peerId, this.conversationId});

  @override
  bool operator ==(Object other) =>
      other is ChatThreadArg &&
      other.peerId == peerId &&
      other.conversationId == conversationId;

  @override
  int get hashCode => Object.hash(peerId, conversationId);
}

@riverpod
class ChatThread extends _$ChatThread {
  // ── Dedup set ─────────────────────────────────────────────────────────────
  // Keys: id (primary) or "${seqId}_${senderId}" (fallback when id is empty).
  // Seeded in build(); cleared on refetchAndReplace().
  final Set<String> _seenIds = {};

  // ── Pagination ────────────────────────────────────────────────────────────
  bool _hasMore = true;
  bool _loadingOlder = false;

  // ── Peer resolution ───────────────────────────────────────────────────────
  // arg.peerId may be '' when screen opened via conversationId-only route.
  // Resolved from participants on first load; used in _getReceiver().
  String? _resolvedPeerId;

  // ── Side-channel: participants from last load ─────────────────────────────
  List<ChatParticipant> _participants = const [];
  List<ChatParticipant> get lastParticipants => _participants;
  bool get hasMoreOlderHistory => _hasMore;

  // ── WS subscriptions ─────────────────────────────────────────────────────
  StreamSubscription<ChatMessage>? _msgSub;
  StreamSubscription<ChatConnectionState>? _connSub;

  // ── Build ─────────────────────────────────────────────────────────────────

  @override
  Future<List<ChatMessage>> build(ChatThreadArg arg) async {
    _resolvedPeerId = arg.peerId.isNotEmpty ? arg.peerId : null;

    final repo = ref.read(chatRepositoryProvider);

    // Cancel previous subscriptions (defensive; Riverpod re-runs build on
    // arg change, not on keepAlive refresh).
    _msgSub?.cancel();
    _connSub?.cancel();

    _msgSub = repo.onMessage.listen(_handleIncoming);

    // skip(1): connectionStateStream is BehaviorSubject (emit-on-subscribe).
    // Ignore the seed; react only to transitions (reconnect→connected).
    _connSub = repo.connectionStateStream.skip(1).listen((s) {
      if (s == ChatConnectionState.connected) {
        refetchAndReplace();
      }
    });

    ref.onDispose(() {
      _msgSub?.cancel();
      _connSub?.cancel();
    });

    return _fetchAndSeed();
  }

  // ── Private helpers ────────────────────────────────────────────────────────

  bool get _useConv => (arg.conversationId ?? '').isNotEmpty;

  String _getReceiver() {
    if (_resolvedPeerId != null && _resolvedPeerId!.isNotEmpty) {
      return _resolvedPeerId!;
    }
    if (arg.peerId.isNotEmpty) return arg.peerId;
    // Fallback for the conversationId-only (push-tap) entry path, before
    // participants resolve _resolvedPeerId: parse the peer (buyer) numeric id
    // out of the roomId — supports both `b{buyer}_s{seller}` (new) and
    // `m{seller}_{buyer}` (legacy) conventions.
    final conv = arg.conversationId ?? '';
    if (conv.contains('_')) {
      final peer = resolveChatPeerId(
        conv,
        ownId: ref.read(chatRepositoryProvider).ownAccountId,
      );
      if (peer.isNotEmpty && peer != conv) return peer;
    }
    return ''; // unresolved — send() must guard
  }

  Future<List<ChatMessage>> _fetchAndSeed() async {
    final repo = ref.read(chatRepositoryProvider);
    final result = await repo.getHistoryWithParticipants(
      participantId: _useConv ? '' : arg.peerId,
      conversationId: arg.conversationId,
      pageSize: 50,
    );

    _participants = result.participants;

    // Resolve peerId from participants when arg.peerId was empty.
    if (_resolvedPeerId == null || _resolvedPeerId!.isEmpty) {
      for (final p in result.participants) {
        if (p.role == '1' || p.role == 'buyer') {
          _resolvedPeerId = p.userId;
          break;
        }
      }
    }

    final msgs = result.messages
      ..sort((a, b) => a.timestamp.compareTo(b.timestamp));

    _hasMore = msgs.length >= 50;
    _seenIds.clear();
    for (final m in msgs) {
      if (m.id.isNotEmpty) _seenIds.add(m.id);
    }

    _markRead();

    return msgs;
  }

  void _markRead() {
    final repo = ref.read(chatRepositoryProvider);
    final pid = _getReceiver();
    if (pid.isNotEmpty) {
      repo
          .markAllMessagesRead(participantId: pid)
          .catchError((_) => false)
          .whenComplete(() => ref.invalidate(dashboardChatUnreadProvider));
      repo.sendReadReceipt(receiverId: pid);
    }
    final convId = arg.conversationId;
    if (convId != null && convId.isNotEmpty) {
      repo
          .markConversationAsRead(convId)
          .catchError((_) => false)
          .whenComplete(() => ref.invalidate(dashboardChatUnreadProvider));
    }
  }

  void _handleIncoming(ChatMessage msg) {
    // Sync-done signal: gap-fill acknowledgement — no UI needed.
    if (msg.messageType == 'sync_done') return;

    // Read receipt: peer read our messages → flip own msgs to read.
    if (msg.messageType == 'read_receipt') {
      if (msg.senderId == _getReceiver()) markMyMessagesRead();
      return;
    }

    // Typing signals: widget displays these via its own repo.onMessage sub.
    if (msg.messageType == 'typing' ||
        msg.messageType == 'start_typing' ||
        msg.messageType == 'stop_typing') {
      return;
    }

    // Filter to this conversation's messages only.
    if (!_isRelevantMessage(msg)) return;

    applyIncoming(msg);

    // Peer sending us a message implies they've seen our previous ones.
    if (msg.senderId == _getReceiver()) markMyMessagesRead();
  }

  bool _isRelevantMessage(ChatMessage msg) {
    final peer = _getReceiver();
    if (peer.isEmpty) return false;
    return msg.senderId == peer || msg.receiverId == peer;
  }

  // ── Public API (canonical contract) ───────────────────────────────────────

  /// Full refetch + REPLACE. Called on WS reconnect (transition to connected).
  /// Expresses loading/error via AsyncValue — no silent swallow.
  Future<void> refetchAndReplace() async {
    state = const AsyncLoading();
    try {
      final msgs = await _fetchAndSeed();
      state = AsyncData(msgs);
    } catch (e, st) {
      state = AsyncError(e, st);
    }
  }

  /// Prepend older page. Returns false when no more pages available.
  Future<bool> loadOlder() async {
    if (!_hasMore) return false;
    if (_loadingOlder) return false;
    final current = state.value ?? [];
    if (current.isEmpty) return false;

    _loadingOlder = true;
    try {
      final oldestId = int.tryParse(current.first.id);
      final older = await ref
          .read(chatRepositoryProvider)
          .getHistory(
            participantId: _useConv ? '' : arg.peerId,
            conversationId: arg.conversationId,
            beforeId: oldestId,
            pageSize: 50,
          );
      if (older.isEmpty) {
        _hasMore = false;
        return false;
      }
      older.sort((a, b) => a.timestamp.compareTo(b.timestamp));
      final existingIds = current.map((m) => m.id).toSet();
      final deduped = older.where((m) => existingIds.add(m.id)).toList();
      if (deduped.isNotEmpty) {
        for (final m in deduped) {
          if (m.id.isNotEmpty) _seenIds.add(m.id);
        }
        state = AsyncData([...deduped, ...current]);
      }
      if (older.length < 50) _hasMore = false;
      return deduped.isNotEmpty;
    } catch (_) {
      return false;
    } finally {
      _loadingOlder = false;
    }
  }

  /// Dedup + insert one incoming WS message.
  /// Primary key: id; fallback: (seqId, senderId) when id is empty.
  /// Skips empty control frames (empty content + non-text type).
  void applyIncoming(ChatMessage msg) {
    if (msg.messageContent.isEmpty && msg.messageType != 'text') return;

    if (msg.id.isNotEmpty) {
      if (!_seenIds.add(msg.id)) return;
    } else if (msg.seqId > 0) {
      final key = '${msg.seqId}_${msg.senderId}';
      if (!_seenIds.add(key)) return;
    }

    final current = state.value ?? [];
    state = AsyncData([...current, msg]);

    // Mark conversation read on every incoming peer message.
    final convId = arg.conversationId;
    if (convId != null && convId.isNotEmpty) {
      ref
          .read(chatRepositoryProvider)
          .markConversationAsRead(convId)
          .catchError((_) => false)
          .whenComplete(() => ref.invalidate(dashboardChatUnreadProvider));
    }
    if (msg.senderId == _getReceiver() && _getReceiver().isNotEmpty) {
      ref
          .read(chatRepositoryProvider)
          .sendReadReceipt(receiverId: _getReceiver());
    }
  }

  void addOptimistic(ChatMessage msg) {
    _seenIds.add(msg.id);
    final current = state.value ?? [];
    state = AsyncData([...current, msg]);
  }

  void confirmOptimistic(String tempId, String realId) {
    _seenIds
      ..remove(tempId)
      ..add(realId);
    final current = state.value ?? [];
    state = AsyncData(
      current.map((m) => m.id == tempId ? m.copyWith(id: realId) : m).toList(),
    );
  }

  void removeOptimistic(String tempId) {
    _seenIds.remove(tempId);
    final current = state.value ?? [];
    state = AsyncData(current.where((m) => m.id != tempId).toList());
  }

  /// Peer read-receipt → flip all my unread messages to read=true.
  void markMyMessagesRead() {
    final current = state.value;
    if (current == null) return;
    final peer = _getReceiver();
    final updated = current
        .map((m) => m.senderId != peer && !m.read ? m.copyWith(read: true) : m)
        .toList();
    state = AsyncData(updated);
  }

  /// Send a message. Adds optimistic entry immediately; confirms or removes
  /// on completion. Rethrows on failure so the widget can show a toast.
  Future<void> send({
    required String content,
    String messageType = 'text',
    String? optimisticTempId,
    Map<String, String>? extra,
  }) async {
    final recv = _getReceiver();
    if (recv.isEmpty) {
      // Receiver unresolved (no participants + no parseable conversationId).
      // Throw BEFORE the optimistic bubble so the screen's catch shows a toast
      // instead of silently sending with an empty receiverId.
      throw StateError('無法解析聊天對象，請稍候再試');
    }
    final tempId =
        optimisticTempId ?? 'temp_${DateTime.now().millisecondsSinceEpoch}';

    addOptimistic(
      ChatMessage(
        id: tempId,
        senderId: '',
        receiverId: recv,
        messageContent: content,
        messageType: messageType,
        timestamp: DateTime.now(),
      ),
    );

    try {
      final result = await ref
          .read(chatRepositoryProvider)
          .sendMessage(
            receiverId: recv,
            messageContent: content,
            messageType: messageType,
            extra: extra,
          );
      if (!ref.mounted) return;
      final data = result?['data'] as Map<String, dynamic>?;
      final realId = (data?['messageId'] ?? '').toString();
      if (realId.isNotEmpty) {
        confirmOptimistic(tempId, realId);
      } else {
        removeOptimistic(tempId);
      }
    } catch (e) {
      if (ref.mounted) removeOptimistic(tempId);
      rethrow;
    }
  }
}
