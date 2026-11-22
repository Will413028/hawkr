import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_session.dart';
import 'package:hawkr_seller/features/chat/data/chat_stream_client.dart';
import 'package:hawkr_seller/features/chat/data/chat_thread_notifier.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import 'support/chat_fakes.dart';

class _Harness {
  _Harness(this.repo) {
    client = ChatStreamClient(
      open: (cursors) {
        opens.add(Map.of(cursors));
        final c = StreamController<pb.ChatEvent>();
        streams.add(c);
        return c.stream;
      },
    );
    session = ChatSession(repo, () => UiLocale.zhTW, client: client);
    container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(repo),
        chatSessionProvider.overrideWithValue(session),
      ],
    );
    container.listen(chatThreadProvider(arg), (_, _) {});
  }

  final FakeChatRepository repo;
  late final ChatStreamClient client;
  late final ChatSession session;
  late final ProviderContainer container;
  final opens = <Map<String, int>>[];
  final streams = <StreamController<pb.ChatEvent>>[];
  final arg = const ChatThreadArg(peerId: 'b1');

  ChatThread get notifier => container.read(chatThreadProvider(arg).notifier);
  AsyncValue<ChatThreadData> get state =>
      container.read(chatThreadProvider(arg));

  Future<void> start() async {
    session.start();
    await Future<void>.delayed(Duration.zero);
    streams.last.add(readyFrame());
    await Future<void>.delayed(Duration.zero);
  }

  Future<void> pump() => Future<void>.delayed(Duration.zero);
}

ChatConversation _conv({String id = 'c1', String buyer = 'b1'}) =>
    ChatConversation.fromProto(protoConversation(id, buyer, name: '小明'));

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late FakeChatRepository repo;
  setUp(() {
    repo = FakeChatRepository()
      ..conversations = [_conv()]
      ..history = [clientMessage(1), clientMessage(2, store: true)];
  });

  test(
    'resolves the conversation by buyer id and loads history oldest first',
    () async {
      final h = _Harness(repo);
      await h.start();
      final d = await h.container.read(chatThreadProvider(h.arg).future);
      expect(d.conversationId, 'c1');
      expect(d.messages.map((m) => m.seq), [1, 2]);
      expect(d.buyer?.name, '小明');
      expect(h.client.cursors['c1'], 2);
    },
  );

  test('opens by buyer through the by-buyer lookup, not a list scan', () async {
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    expect(repo.findCalls, 1);
  });

  test(
    'no conversation yet: empty thread, first send uses SendMessageToBuyer',
    () async {
      repo.conversations = [];
      final h = _Harness(repo);
      await h.start();
      final d = await h.container.read(chatThreadProvider(h.arg).future);
      expect(d.conversationId, isNull);
      expect(d.messages, isEmpty);
      await h.notifier.sendText('hi');
      expect(repo.sends.single.buyerId, 'b1');
      expect(repo.sends.single.convId, isNull);
      expect(h.state.value!.conversationId, 'new-conv');
      expect(h.state.value!.messages.single.sendState, ChatSendState.sent);
    },
  );

  test('read failure is an error, not an empty thread', () async {
    repo.conversations = [_conv()];
    repo.listOlderError = ConnectException(Code.unavailable, 'down');
    final h = _Harness(repo);
    await h.start();
    await expectLater(
      h.container.read(chatThreadProvider(h.arg).future),
      throwsA(isA<ConnectException>()),
    );
    expect(h.state.hasError, isTrue);
  });

  test(
    'stream message with updated=true replaces the same seq with a tombstone',
    () async {
      final h = _Harness(repo);
      await h.start();
      await h.container.read(chatThreadProvider(h.arg).future);
      h.streams.last.add(
        messageFrame(protoMessage(1, hidden: true), updated: true),
      );
      await h.pump();
      final msgs = h.state.value!.messages;
      expect(msgs, hasLength(2));
      expect(msgs.first.hidden, isTrue);
      expect(msgs.first.text, isNull);
    },
  );

  test(
    'new peer message is appended and marks the conversation read',
    () async {
      final h = _Harness(repo);
      await h.start();
      await h.container.read(chatThreadProvider(h.arg).future);
      repo.marks.clear();
      h.streams.last.add(messageFrame(protoMessage(3)));
      await h.pump();
      expect(h.state.value!.messages.map((m) => m.seq), [1, 2, 3]);
      expect(repo.marks, contains(('c1', 3)));
    },
  );

  test('a seq gap triggers a fill after the last known seq', () async {
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    repo.afterResult = [clientMessage(3), clientMessage(4)];
    h.streams.last.add(messageFrame(protoMessage(5)));
    await h.pump();
    await h.pump();
    expect(repo.afterCalls, isNotEmpty);
    expect(repo.afterCalls.first, ('c1', 2));
    expect(h.state.value!.messages.map((m) => m.seq), [1, 2, 3, 4, 5]);
  });

  test(
    'resync for this conversation fills the gap; for another one it does not',
    () async {
      final h = _Harness(repo);
      await h.start();
      await h.container.read(chatThreadProvider(h.arg).future);
      h.streams.last.add(
        pb.ChatEvent(resync: pb.Resync(conversationId: 'other')),
      );
      await h.pump();
      expect(repo.afterCalls, isEmpty);
      repo.afterResult = [clientMessage(3)];
      h.streams.last.add(pb.ChatEvent(resync: pb.Resync(all: true)));
      await h.pump();
      await h.pump();
      expect(repo.afterCalls.single, ('c1', 2));
      expect(h.state.value!.messages.map((m) => m.seq), [1, 2, 3]);
    },
  );

  test('peer read event advances the read cursor', () async {
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    h.streams.last.add(
      pb.ChatEvent(
        peerRead: pb.PeerRead(conversationId: 'c1', peerReadSeq: i64(2)),
      ),
    );
    await h.pump();
    expect(h.state.value!.peerReadSeq, 2);
  });

  test(
    'send failure keeps the message as failed and retry reuses the request id',
    () async {
      final h = _Harness(repo);
      await h.start();
      await h.container.read(chatThreadProvider(h.arg).future);
      repo.sendError = ConnectException(Code.unavailable, 'down');
      await expectLater(
        h.notifier.sendText('hello'),
        throwsA(isA<ConnectException>()),
      );
      final failed = h.state.value!.messages.last;
      expect(failed.sendState, ChatSendState.failed);
      final rid = failed.requestId!;
      repo.sendError = null;
      await h.notifier.retry(rid);
      expect(repo.sends.map((s) => s.requestId), [rid, rid]);
      expect(h.state.value!.messages.where((m) => m.isPending), isEmpty);
      expect(h.state.value!.messages.last.fromStore, isTrue);
    },
  );

  test('separate sends use different request ids', () async {
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    await h.notifier.sendText('a');
    await h.notifier.sendText('b');
    expect(repo.sends[0].requestId, isNot(repo.sends[1].requestId));
    expect(repo.sends[0].requestId, matches(RegExp(r'^[0-9a-f-]{36}$')));
  });

  test('markRead pushes the cursor to the last peer message seq', () async {
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    await h.pump();
    expect(repo.marks, contains(('c1', 1)));
  });

  test('loadOlder asks for seq below the oldest and prepends', () async {
    repo.history = [clientMessage(5), clientMessage(6)];
    repo.hasMore = true;
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    repo.history = [clientMessage(3), clientMessage(4), clientMessage(5)];
    repo.hasMore = false;
    expect(await h.notifier.loadOlder(), isTrue);
    expect(repo.beforeCalls.last, 5);
    expect(h.state.value!.messages.map((m) => m.seq), [3, 4, 5, 6]);
    expect(h.state.value!.hasMoreOlder, isFalse);
  });

  test('a thread opened before the conversation exists adopts the first '
      'incoming message of its buyer', () async {
    repo.conversations = [];
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    repo.conversations = [_conv(id: 'c7')];
    h.streams.last.add(messageFrame(protoMessage(1, conv: 'c7')));
    await h.pump();
    await h.pump();
    final d = h.state.value!;
    expect(d.conversationId, 'c7');
    expect(d.messages.map((m) => m.seq), [1]);
    expect(d.buyer?.name, '小明');
  });

  test('a message of another buyer is not adopted', () async {
    repo.conversations = [];
    final h = _Harness(repo);
    await h.start();
    await h.container.read(chatThreadProvider(h.arg).future);
    repo.conversations = [_conv(id: 'c8', buyer: 'other')];
    h.streams.last.add(messageFrame(protoMessage(1, conv: 'c8')));
    await h.pump();
    await h.pump();
    expect(h.state.value!.conversationId, isNull);
    expect(h.state.value!.messages, isEmpty);
  });

  test(
    "a message of a conversation other than the buyer's is not adopted",
    () async {
      repo.conversations = [];
      final h = _Harness(repo);
      await h.start();
      await h.container.read(chatThreadProvider(h.arg).future);
      repo.conversations = [_conv(id: 'c9')];
      h.streams.last.add(messageFrame(protoMessage(1, conv: 'c-elsewhere')));
      await h.pump();
      await h.pump();
      expect(h.state.value!.conversationId, isNull);
      expect(h.state.value!.messages, isEmpty);
    },
  );
}
