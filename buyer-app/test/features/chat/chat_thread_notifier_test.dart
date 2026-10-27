// test/features/chat/chat_thread_notifier_test.dart
import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:hawkr_buyer/features/chat/data/chat_model.dart';
import 'package:hawkr_buyer/features/chat/data/chat_repository.dart';
import 'package:hawkr_buyer/features/chat/data/chat_thread_notifier.dart';

@GenerateMocks([ChatRepository])
import 'chat_thread_notifier_test.mocks.dart';

// Helper: build a ChatMessage with sensible defaults
ChatMessage _msg({
  String id = 'm1',
  String senderId = 'peer42',
  String receiverId = 'me',
  String content = 'hello',
  String type = 'text',
  int seqId = 1,
  bool read = false,
}) => ChatMessage(
  id: id,
  senderId: senderId,
  receiverId: receiverId,
  messageContent: content,
  messageType: type,
  timestamp: DateTime(2024, 1, 1, 12, 0),
  seqId: seqId,
  read: read,
);

const _arg = ChatThreadArg(peerId: 'peer42', conversationId: 'conv1');

ProviderContainer _makeContainer(MockChatRepository repo) => ProviderContainer(
  overrides: [chatRepositoryProvider.overrideWithValue(repo)],
);

void main() {
  late MockChatRepository repo;
  late StreamController<ChatMessage> msgCtrl;
  late StreamController<ChatConnectionState> stateCtrl;

  setUp(() {
    repo = MockChatRepository();
    msgCtrl = StreamController<ChatMessage>.broadcast();
    stateCtrl = StreamController<ChatConnectionState>();
    when(repo.onMessage).thenAnswer((_) => msgCtrl.stream);
    when(
      repo.connectionStateStream,
    ).thenAnswer((_) => stateCtrl.stream.asBroadcastStream());
    when(
      repo.getHistoryWithParticipants(
        participantId: anyNamed('participantId'),
        pageSize: anyNamed('pageSize'),
        conversationId: anyNamed('conversationId'),
      ),
    ).thenAnswer(
      (_) async =>
          (messages: <ChatMessage>[], participants: <Map<String, dynamic>>[]),
    );
  });

  tearDown(() {
    msgCtrl.close();
    stateCtrl.close();
  });

  group('applyIncoming — dedup', () {
    test('same id inserted only once', () async {
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);
      final n = c.read(chatThreadProvider(_arg).notifier);

      n.applyIncoming(_msg(id: 'x1', seqId: 1));
      n.applyIncoming(_msg(id: 'x1', seqId: 1)); // duplicate

      expect(
        c
            .read(chatThreadProvider(_arg))
            .value!
            .where((m) => m.id == 'x1')
            .length,
        1,
      );
    });

    test('fallback dedup by seqId+senderId when id is empty', () async {
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);
      final n = c.read(chatThreadProvider(_arg).notifier);

      n.applyIncoming(_msg(id: '', seqId: 7, senderId: 'peer42'));
      n.applyIncoming(_msg(id: '', seqId: 7, senderId: 'peer42'));

      expect(
        c
            .read(chatThreadProvider(_arg))
            .value!
            .where((m) => m.seqId == 7 && m.senderId == 'peer42')
            .length,
        1,
      );
    });

    test('empty-content non-text control frame is skipped', () async {
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);
      final n = c.read(chatThreadProvider(_arg).notifier);

      n.applyIncoming(_msg(id: 'ctrl1', content: '', type: 'sync_done'));

      expect(
        c.read(chatThreadProvider(_arg)).value!.where((m) => m.id == 'ctrl1'),
        isEmpty,
      );
    });

    test(
      'empty-content text frame IS kept (empty text bubble allowed)',
      () async {
        final c = _makeContainer(repo);
        addTearDown(c.dispose);
        await c.read(chatThreadProvider(_arg).future);
        final n = c.read(chatThreadProvider(_arg).notifier);

        n.applyIncoming(_msg(id: 't1', content: '', type: 'text'));

        expect(
          c
              .read(chatThreadProvider(_arg))
              .value!
              .where((m) => m.id == 't1')
              .length,
          1,
        );
      },
    );
  });

  group('markMyMessagesRead', () {
    test('flips only messages NOT from peer (my sent messages)', () async {
      when(
        repo.getHistoryWithParticipants(
          participantId: anyNamed('participantId'),
          pageSize: anyNamed('pageSize'),
          conversationId: anyNamed('conversationId'),
        ),
      ).thenAnswer(
        (_) async => (
          messages: [
            _msg(id: 'mine', senderId: 'me', receiverId: 'peer42', read: false),
            _msg(
              id: 'theirs',
              senderId: 'peer42',
              receiverId: 'me',
              read: false,
            ),
          ],
          participants: <Map<String, dynamic>>[],
        ),
      );

      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);

      c.read(chatThreadProvider(_arg).notifier).markMyMessagesRead();

      final msgs = c.read(chatThreadProvider(_arg)).value!;
      expect(msgs.firstWhere((m) => m.id == 'mine').read, isTrue);
      expect(msgs.firstWhere((m) => m.id == 'theirs').read, isFalse);
    });

    test('already-read my messages are not duplicated in list', () async {
      when(
        repo.getHistoryWithParticipants(
          participantId: anyNamed('participantId'),
          pageSize: anyNamed('pageSize'),
          conversationId: anyNamed('conversationId'),
        ),
      ).thenAnswer(
        (_) async => (
          messages: [_msg(id: 'm1', senderId: 'me', read: true)],
          participants: <Map<String, dynamic>>[],
        ),
      );
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);

      c.read(chatThreadProvider(_arg).notifier).markMyMessagesRead();

      expect(c.read(chatThreadProvider(_arg)).value!.length, 1);
    });
  });

  group('optimistic send lifecycle', () {
    test(
      'addOptimistic then removeOptimistic leaves no ghost message',
      () async {
        final c = _makeContainer(repo);
        addTearDown(c.dispose);
        await c.read(chatThreadProvider(_arg).future);
        final n = c.read(chatThreadProvider(_arg).notifier);

        n.addOptimistic(_msg(id: 'temp_001'));
        n.removeOptimistic('temp_001');

        expect(
          c
              .read(chatThreadProvider(_arg))
              .value!
              .where((m) => m.id == 'temp_001'),
          isEmpty,
        );
      },
    );

    test('confirmOptimistic replaces temp id with real id in-place', () async {
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);
      final n = c.read(chatThreadProvider(_arg).notifier);

      n.addOptimistic(_msg(id: 'temp_001', content: 'hi'));
      n.confirmOptimistic('temp_001', 'server_001');

      final msgs = c.read(chatThreadProvider(_arg)).value!;
      expect(msgs.any((m) => m.id == 'server_001'), isTrue);
      expect(msgs.any((m) => m.id == 'temp_001'), isFalse);
      expect(msgs.firstWhere((m) => m.id == 'server_001').messageContent, 'hi');
    });
  });

  group('refetchAndReplace', () {
    test('error from repo surfaces as AsyncError (not swallowed)', () async {
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);

      when(
        repo.getHistoryWithParticipants(
          participantId: anyNamed('participantId'),
          pageSize: anyNamed('pageSize'),
          conversationId: anyNamed('conversationId'),
        ),
      ).thenThrow(Exception('network error'));

      await c.read(chatThreadProvider(_arg).notifier).refetchAndReplace();

      expect(c.read(chatThreadProvider(_arg)).hasError, isTrue);
    });

    test('after successful refetch old dedup set is cleared', () async {
      when(
        repo.getHistoryWithParticipants(
          participantId: anyNamed('participantId'),
          pageSize: anyNamed('pageSize'),
          conversationId: anyNamed('conversationId'),
        ),
      ).thenAnswer(
        (_) async => (
          messages: [_msg(id: 'a1')],
          participants: <Map<String, dynamic>>[],
        ),
      );
      final c = _makeContainer(repo);
      addTearDown(c.dispose);
      await c.read(chatThreadProvider(_arg).future);
      final n = c.read(chatThreadProvider(_arg).notifier);

      // refetch returns same message — should not duplicate
      await n.refetchAndReplace();
      await n.refetchAndReplace();

      expect(
        c
            .read(chatThreadProvider(_arg))
            .value!
            .where((m) => m.id == 'a1')
            .length,
        1,
      );
    });
  });
}
