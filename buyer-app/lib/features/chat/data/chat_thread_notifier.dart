import 'dart:async';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'chat_model.dart';
import 'chat_repository.dart'; // PR1: renamed from chat_service.dart

part 'chat_thread_notifier.g.dart';

/// Identifies a chat thread by peer + optional conversation id.
/// buyer: peerId = merchantId. seller: peerId = customer accountId.
class ChatThreadArg {
  const ChatThreadArg({required this.peerId, this.conversationId});
  final String peerId;
  final String? conversationId;

  @override
  bool operator ==(Object other) =>
      other is ChatThreadArg &&
      other.peerId == peerId &&
      other.conversationId == conversationId;

  @override
  int get hashCode => Object.hash(peerId, conversationId);

  @override
  String toString() => 'ChatThreadArg(peerId: $peerId, conv: $conversationId)';
}

@riverpod
class ChatThread extends _$ChatThread {
  // ── dedup state ──────────────────────────────────────────────────────────
  final Set<String> _knownIds = {};

  // ── subscriptions ────────────────────────────────────────────────────────
  StreamSubscription<ChatMessage>? _msgSub;
  StreamSubscription<ChatConnectionState>? _connSub;

  // ── build (initial load + stream subscriptions) ──────────────────────────
  @override
  Future<List<ChatMessage>> build(ChatThreadArg arg) async {
    final repo = ref.read(chatRepositoryProvider);

    // Subscribe once; ref.onDispose tears down on autoDispose.
    _msgSub = repo.onMessage.listen(_onFrame);

    // skip(1): connectionStateStream is a BehaviorSubject (emit-on-subscribe).
    // We only want to react to *transitions*, not the seeded value.
    _connSub = repo.connectionStateStream.skip(1).listen((s) {
      if (s == ChatConnectionState.connected) {
        refetchAndReplace();
      }
    });

    ref.onDispose(() {
      _msgSub?.cancel();
      _connSub?.cancel();
    });

    final result = await repo.getHistoryWithParticipants(
      participantId: arg.peerId,
      conversationId: null, // 強制 participantId（1:1 買賣家唯一定位，沿用舊行為）
    );
    final sorted = List<ChatMessage>.from(result.messages)
      ..sort((a, b) => a.timestamp.compareTo(b.timestamp));

    for (final m in sorted) {
      if (m.id.isNotEmpty) _knownIds.add(m.id);
    }

    return sorted;
  }

  // ── public API ────────────────────────────────────────────────────────────

  /// Full refetch + REPLACE. Reconnect path.
  /// Error surfaces as AsyncError — no swallowing.
  Future<void> refetchAndReplace() async {
    // riverpod 3.x auto-applies copyWithPrevious on manual `state =`, so this
    // AsyncLoading RETAINS the current data (state.value stays non-null). The
    // screen keeps showing the list during refetch — see buildMessageList's
    // `isLoading && msgs.isEmpty` guard — and this AsyncLoading transition is
    // also the signal chat_thread_screen's seenIncomingIds-seed listener gates on.
    state = const AsyncLoading();
    final repo = ref.read(chatRepositoryProvider);
    try {
      final result = await repo.getHistoryWithParticipants(
        participantId: arg.peerId,
        conversationId: null, // 強制 participantId（1:1 買賣家唯一定位，沿用舊行為）
      );
      final sorted = List<ChatMessage>.from(result.messages)
        ..sort((a, b) => a.timestamp.compareTo(b.timestamp));
      // Clear dedup set after full reload.
      _knownIds
        ..clear()
        ..addAll(sorted.where((m) => m.id.isNotEmpty).map((m) => m.id));
      state = AsyncData(sorted);
    } catch (e, st) {
      state = AsyncError(e, st);
    }
  }

  /// Prepend older page; returns false when no more messages.
  Future<bool> loadOlder() async {
    final current = state.value;
    if (current == null || current.isEmpty) return false;

    // Find oldest loaded message with a real id for cursor.
    final oldestId = current
        .firstWhere((m) => m.id.isNotEmpty, orElse: () => current.first)
        .id;

    final repo = ref.read(chatRepositoryProvider);
    final batch = await repo.getHistory(
      participantId: arg.peerId,
      conversationId: null, // 強制 participantId（1:1 買賣家唯一定位，沿用舊行為）
      beforeId: oldestId.isEmpty ? null : oldestId,
      pageSize: 50,
    );
    if (batch.isEmpty) return false;

    final sorted = List<ChatMessage>.from(batch)
      ..sort((a, b) => a.timestamp.compareTo(b.timestamp));

    final toInsert = <ChatMessage>[];
    for (final m in sorted) {
      if (m.id.isEmpty) continue;
      if (_knownIds.contains(m.id)) continue;
      _knownIds.add(m.id);
      toInsert.add(m);
    }
    if (toInsert.isEmpty) return false;
    state = AsyncData([...toInsert, ...current]);
    return true;
  }

  /// Single dedup impl: key on id; fallback (seqId, senderId).
  /// Skips empty-content non-text control frames.
  void applyIncoming(ChatMessage msg) {
    if (msg.messageContent.isEmpty && msg.messageType != 'text') return;

    if (msg.id.isNotEmpty) {
      if (_knownIds.contains(msg.id)) return;
      _knownIds.add(msg.id);
    } else if (msg.seqId > 0) {
      final fallback = '${msg.seqId}_${msg.senderId}';
      if (_knownIds.contains(fallback)) return;
      _knownIds.add(fallback);
    }

    final current = state.value ?? [];
    state = AsyncData([...current, msg]);
  }

  void addOptimistic(ChatMessage msg) {
    final current = state.value ?? [];
    state = AsyncData([...current, msg]);
  }

  void confirmOptimistic(String tempId, String realId) {
    final current = state.value;
    if (current == null) return;
    _knownIds.add(realId);
    state = AsyncData([
      for (final m in current) m.id == tempId ? m.copyWith(id: realId) : m,
    ]);
  }

  void removeOptimistic(String tempId) {
    final current = state.value;
    if (current == null) return;
    state = AsyncData(current.where((m) => m.id != tempId).toList());
  }

  /// Peer sent read_receipt → flip my outgoing messages to read=true (copyWith).
  /// "My messages" = senderId != arg.peerId (buyer context: not from merchant).
  void markMyMessagesRead() {
    final current = state.value;
    if (current == null) return;
    state = AsyncData([
      for (final m in current)
        (m.senderId != arg.peerId && !m.read) ? m.copyWith(read: true) : m,
    ]);
  }

  /// Send a message. Optimistic insert → API call → confirm or remove on failure.
  Future<void> send({
    required String content,
    String messageType = 'text',
    String? optimisticTempId,
  }) async {
    final tempId =
        optimisticTempId ?? 'temp_${DateTime.now().millisecondsSinceEpoch}';
    final optimistic = ChatMessage(
      id: tempId,
      senderId:
          '', // unknown until server confirms; isSentBy check uses receiverId
      receiverId: arg.peerId,
      messageContent: content,
      messageType: messageType,
      timestamp: DateTime.now(),
    );
    addOptimistic(optimistic);

    try {
      final repo = ref.read(chatRepositoryProvider);
      final resp = await repo.sendMessage(
        receiverId: arg.peerId,
        messageContent: content,
        messageType: messageType,
      );
      final realId = (resp?['data']?['messageId'] ?? '').toString();
      if (realId.isNotEmpty && realId != 'null') {
        confirmOptimistic(tempId, realId);
      }
    } catch (e) {
      // Ghost message fix: remove optimistic bubble on send failure.
      removeOptimistic(tempId);
      rethrow;
    }
  }

  // ── private WS frame router ───────────────────────────────────────────────

  void _onFrame(ChatMessage msg) {
    // read_receipt from peer → flip my messages read.
    if (msg.messageType == 'read_receipt') {
      if (msg.senderId == arg.peerId) markMyMessagesRead();
      return;
    }
    // typing frames → ephemeral UI concern, handled by screen Hook.
    if (msg.messageType == 'typing' ||
        msg.messageType == 'start_typing' ||
        msg.messageType == 'stop_typing') {
      return;
    }
    // control signals — skip.
    if (msg.messageType == 'sync_done' || msg.messageType == 'unread_changed') {
      return;
    }
    // Filter to this conversation only.
    if (msg.senderId != arg.peerId && msg.receiverId != arg.peerId) return;

    applyIncoming(msg);

    // Peer (merchant) sending us a message implies they've seen our previous ones.
    if (msg.senderId == arg.peerId) markMyMessagesRead();
  }
}
