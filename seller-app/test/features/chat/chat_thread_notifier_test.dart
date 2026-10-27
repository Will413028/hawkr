import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_thread_notifier.dart';

// Minimal fake ChatRepository — no WS, no network.
// Real ChatRepository constructor requires Dio; we pass a throwaway real Dio
// (never used — every network method is overridden below) and override
// chatRepositoryProvider in each container.
class _FakeRepo extends ChatRepository {
  _FakeRepo.withMessages(this.initialMessages)
    : super(Dio(BaseOptions(baseUrl: 'http://localhost')));

  final List<ChatMessage> initialMessages;
  Map<String, String>? lastSentExtra;
  Completer<Map<String, dynamic>?>? pendingSend;

  final StreamController<ChatMessage> _msgCtrl =
      StreamController<ChatMessage>.broadcast();
  final StreamController<ChatConnectionState> _stateCtrl =
      StreamController<ChatConnectionState>.broadcast();

  @override
  Stream<ChatMessage> get onMessage => _msgCtrl.stream;
  @override
  Stream<ChatConnectionState> get connectionStateStream => _stateCtrl.stream;

  @override
  Future<ChatHistoryResult> getHistoryWithParticipants({
    required String participantId,
    int? beforeId,
    int pageSize = 50,
    String? conversationId,
  }) async => ChatHistoryResult(messages: initialMessages, participants: []);

  @override
  Future<List<ChatMessage>> getHistory({
    required String participantId,
    int? beforeId,
    int pageSize = 50,
    String? conversationId,
  }) async => [];

  @override
  Future<Map<String, dynamic>?> sendMessage({
    required String receiverId,
    required String messageContent,
    String messageType = 'text',
    Map<String, String>? extra,
  }) async {
    lastSentExtra = extra;
    if (pendingSend != null) return pendingSend!.future;
    return {
      'data': {'messageId': 'real_123', 'timestamp': 0},
    };
  }

  @override
  Future<bool> markAllMessagesRead({required String participantId}) async =>
      true;
  @override
  Future<bool> markConversationAsRead(String conversationId) async => true;
  @override
  void sendReadReceipt({required String receiverId}) {}
  @override
  Future<void> connectForChat() async {}
  @override
  Future<void> disconnectFromChat() async {}

  void pushMessage(ChatMessage msg) => _msgCtrl.add(msg);
  void pushState(ChatConnectionState s) => _stateCtrl.add(s);
}

ChatMessage _msg(String id, {String sender = 'buyer1', int seqId = 0}) =>
    ChatMessage(
      id: id,
      senderId: sender,
      receiverId: 'seller1',
      messageContent: 'hello',
      messageType: 'text',
      timestamp: DateTime(2024),
      seqId: seqId,
    );

ProviderContainer _makeContainer(_FakeRepo repo) => ProviderContainer(
  overrides: [chatRepositoryProvider.overrideWithValue(repo)],
);

void main() {
  for (final failure in [false, true]) {
    test(
      'send ${failure ? 'failure' : 'success'} after thread disposal is safe',
      () async {
        final repo = _FakeRepo.withMessages([]);
        final container = _makeContainer(repo);
        const arg = ChatThreadArg(peerId: 'buyer1');
        final listener = container.listen(chatThreadProvider(arg), (_, _) {});
        await container.read(chatThreadProvider(arg).future);
        repo.pendingSend = Completer<Map<String, dynamic>?>();
        final pending = container
            .read(chatThreadProvider(arg).notifier)
            .send(content: '離頁前送出');
        listener.close();
        container.dispose();
        if (failure) {
          final expectation = expectLater(pending, throwsA(isA<StateError>()));
          repo.pendingSend!.completeError(StateError('offline'));
          await expectation;
        } else {
          repo.pendingSend!.complete({
            'data': {'messageId': 'real_123'},
          });
          await pending;
        }
      },
    );
  }

  group('ChatThreadArg equality', () {
    test('same peerId + conversationId are equal', () {
      const a = ChatThreadArg(peerId: '1', conversationId: 'c1');
      const b = ChatThreadArg(peerId: '1', conversationId: 'c1');
      expect(a, equals(b));
      expect(a.hashCode, equals(b.hashCode));
    });
    test('different conversationId are not equal', () {
      const a = ChatThreadArg(peerId: '1', conversationId: 'c1');
      const b = ChatThreadArg(peerId: '1', conversationId: 'c2');
      expect(a, isNot(equals(b)));
    });
  });

  group('ChatThread initial load', () {
    test(
      'build() returns sorted messages from getHistoryWithParticipants',
      () async {
        final m1 = _msg('1').copyWith(timestamp: DateTime(2024, 1, 1));
        final m2 = _msg('2').copyWith(timestamp: DateTime(2024, 1, 2));
        final repo = _FakeRepo.withMessages([m2, m1]); // reversed
        final c = _makeContainer(repo);
        addTearDown(c.dispose);
        const arg = ChatThreadArg(peerId: 'buyer1');
        final state = await c.read(chatThreadProvider(arg).future);
        expect(state.first.id, '1'); // sorted asc
        expect(state.last.id, '2');
      },
    );
  });

  group('applyIncoming dedup', () {
    test('duplicate by id is ignored', () async {
      final repo = _FakeRepo.withMessages([_msg('a')]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      final notifier = c.read(chatThreadProvider(arg).notifier);
      notifier.applyIncoming(_msg('a')); // duplicate id
      expect(c.read(chatThreadProvider(arg)).value?.length, 1);
    });

    test('new id is appended', () async {
      final repo = _FakeRepo.withMessages([_msg('a')]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      final notifier = c.read(chatThreadProvider(arg).notifier);
      notifier.applyIncoming(_msg('b'));
      expect(c.read(chatThreadProvider(arg)).value?.length, 2);
      expect(c.read(chatThreadProvider(arg)).value?.last.id, 'b');
    });

    test('seqId fallback dedup when id is empty', () async {
      final repo = _FakeRepo.withMessages([]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      final notifier = c.read(chatThreadProvider(arg).notifier);
      final noId = ChatMessage(
        id: '',
        senderId: 'buyer1',
        receiverId: 'seller1',
        messageContent: 'x',
        messageType: 'text',
        timestamp: DateTime(2024),
        seqId: 5,
      );
      notifier.applyIncoming(noId);
      notifier.applyIncoming(noId); // second call: same (seqId, senderId) → dup
      expect(c.read(chatThreadProvider(arg)).value?.length, 1);
    });

    test('empty control frame is skipped', () async {
      final repo = _FakeRepo.withMessages([]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      final notifier = c.read(chatThreadProvider(arg).notifier);
      final control = ChatMessage(
        id: 'ctrl1',
        senderId: 'buyer1',
        receiverId: 'seller1',
        messageContent: '',
        messageType: 'read_receipt',
        timestamp: DateTime(2024),
      );
      notifier.applyIncoming(control);
      expect(c.read(chatThreadProvider(arg)).value?.isEmpty, true);
    });
  });

  group('loadOlder', () {
    test('returns false when no more history', () async {
      final repo = _FakeRepo.withMessages([]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      // hasMoreOlderHistory starts false when initial page < 50
      final result = await c.read(chatThreadProvider(arg).notifier).loadOlder();
      expect(result, false);
    });
  });

  group('send optimistic', () {
    test('adds temp message then confirms with real id', () async {
      final repo = _FakeRepo.withMessages([]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      final notifier = c.read(chatThreadProvider(arg).notifier);
      final sendFuture = notifier.send(content: 'hi');
      // During send: temp message present
      final duringState = c.read(chatThreadProvider(arg)).value ?? [];
      expect(duringState.any((m) => m.id.startsWith('temp_')), true);
      await sendFuture;
      // After send: temp replaced with real id
      final afterState = c.read(chatThreadProvider(arg)).value ?? [];
      expect(afterState.any((m) => m.id.startsWith('temp_')), false);
      expect(afterState.any((m) => m.id == 'real_123'), true);
    });

    test('send() 把 extra 透傳給 repository.sendMessage', () async {
      final repo = _FakeRepo.withMessages([]);
      final container = _makeContainer(repo);
      addTearDown(container.dispose);

      const arg = ChatThreadArg(peerId: 'seller1', conversationId: 'c1');
      await container
          .read(chatThreadProvider(arg).notifier)
          .send(
            content: '{"messageType":"product"}',
            messageType: 'product',
            extra: {'preview': '傳送了一張商品卡片'},
          );

      expect(repo.lastSentExtra, {'preview': '傳送了一張商品卡片'});
    });
  });

  group('_getReceiver conversationId fallback (push-tap path)', () {
    test(
      'send resolves buyer from conversationId when participants empty',
      () async {
        // peerId '' + participants [] → _resolvedPeerId can't resolve.
        // conversationId '123_m456' → buyer is the non-'m' part '123'.
        final repo = _FakeRepo.withMessages([]);
        final c = _makeContainer(repo);
        addTearDown(c.dispose);
        const arg = ChatThreadArg(peerId: '', conversationId: '123_m456');
        await c.read(chatThreadProvider(arg).future);
        final notifier = c.read(chatThreadProvider(arg).notifier);
        final sendFuture = notifier.send(content: 'hi');
        // Optimistic bubble carries the resolved receiver (not empty).
        final during = c.read(chatThreadProvider(arg)).value ?? [];
        expect(during.length, 1);
        expect(during.last.receiverId, '123');
        await sendFuture;
        // receiverId preserved through confirmOptimistic.
        final after = c.read(chatThreadProvider(arg)).value ?? [];
        expect(after.single.receiverId, '123');
      },
    );

    test(
      'send throws StateError and inserts no bubble when receiver unresolved',
      () async {
        // peerId '' + conversationId '' + participants [] → no receiver.
        final repo = _FakeRepo.withMessages([]);
        final c = _makeContainer(repo);
        addTearDown(c.dispose);
        const arg = ChatThreadArg(peerId: '', conversationId: '');
        await c.read(chatThreadProvider(arg).future);
        final notifier = c.read(chatThreadProvider(arg).notifier);
        await expectLater(
          () => notifier.send(content: 'x'),
          throwsA(isA<StateError>()),
        );
        // No ghost optimistic bubble inserted.
        expect(c.read(chatThreadProvider(arg)).value?.isEmpty, true);
      },
    );
  });

  group('markMyMessagesRead', () {
    test('flips my unread messages to read', () async {
      final myMsg = _msg('m1', sender: 'seller1').copyWith(read: false);
      final repo = _FakeRepo.withMessages([myMsg]);
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      const arg = ChatThreadArg(peerId: 'buyer1');
      await c.read(chatThreadProvider(arg).future);
      final notifier = c.read(chatThreadProvider(arg).notifier);
      notifier.markMyMessagesRead();
      final msgs = c.read(chatThreadProvider(arg)).value ?? [];
      expect(msgs.first.read, true);
    });
  });
}
