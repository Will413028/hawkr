import 'dart:async';
import 'dart:math' as math;

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/chat/data/chat_stream_client.dart';

import '../../support/connect_errors.dart';
import 'support/chat_fakes.dart';

/// 每次開串流記下 cursors，把串流交給測試控制。
class _Opener {
  final calls = <Map<String, int>>[];
  final controllers = <StreamController<pb.ChatEvent>>[];

  Stream<pb.ChatEvent> call(Map<String, int> cursors) {
    calls.add(Map.of(cursors));
    final c = StreamController<pb.ChatEvent>();
    controllers.add(c);
    return c.stream;
  }

  StreamController<pb.ChatEvent> get last => controllers.last;
}

class _FixedRandom implements math.Random {
  _FixedRandom(this.value);
  final double value;
  @override
  bool nextBool() => false;
  @override
  double nextDouble() => value;
  @override
  int nextInt(int max) => 0;
}

void main() {
  late _Opener opener;
  late List<ChatStreamEvent> events;
  late List<ChatStreamStatus> statuses;

  ChatStreamClient build({double jitter = 1.0}) {
    final c = ChatStreamClient(open: opener.call, random: _FixedRandom(jitter));
    c.events.listen(events.add);
    c.statuses.listen(statuses.add);
    return c;
  }

  setUp(() {
    opener = _Opener();
    events = [];
    statuses = [];
  });

  testWidgets('opens with the cursors it holds and reports Ready', (
    tester,
  ) async {
    final client = build()..noteSeq('c1', 5);
    client.start();
    await tester.pump();
    expect(opener.calls.single, {'c1': 5});
    opener.last.add(readyFrame(unread: 3));
    await tester.pump();
    expect((events.single as ChatReady).unreadCount, 3);
    expect(client.status, isA<ChatStreamLive>());
    client.stop();
  });

  testWidgets(
    'a normal end (max lifetime) reconnects at once with the latest seq',
    (tester) async {
      final client = build();
      client.start();
      await tester.pump();
      opener.last.add(readyFrame());
      opener.last.add(messageFrame(protoMessage(7)));
      await tester.pump();
      unawaited(opener.last.close());
      await tester.pump();
      expect(opener.calls, hasLength(2));
      expect(opener.calls.last, {'c1': 7});
      client.stop();
    },
  );

  testWidgets('no frame for twice the heartbeat interval counts as dropped', (
    tester,
  ) async {
    final client = build(jitter: 0);
    client.start();
    await tester.pump();
    opener.last.add(readyFrame(heartbeatSeconds: 10));
    await tester.pump();
    await tester.pump(const Duration(seconds: 19));
    expect(opener.calls, hasLength(1));
    await tester.pump(const Duration(seconds: 2));
    await tester.pump();
    expect(opener.calls, hasLength(2));
    client.stop();
  });

  testWidgets('a heartbeat keeps the liveness timer alive', (tester) async {
    final client = build(jitter: 0);
    client.start();
    await tester.pump();
    opener.last.add(readyFrame(heartbeatSeconds: 10));
    for (var i = 0; i < 5; i++) {
      await tester.pump(const Duration(seconds: 15));
      opener.last.add(heartbeatFrame());
      await tester.pump();
    }
    expect(opener.calls, hasLength(1));
    client.stop();
  });

  testWidgets('unavailable backs off with Full Jitter and doubles each time', (
    tester,
  ) async {
    final client = build(jitter: 1.0);
    client.start();
    await tester.pump();
    opener.last.addError(ConnectException(Code.unavailable, 'down'));
    await tester.pump();
    var s = statuses.whereType<ChatStreamReconnecting>().last;
    expect(s.delay, const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
    await tester.pump();
    expect(opener.calls, hasLength(2));
    opener.last.addError(ConnectException(Code.unavailable, 'down'));
    await tester.pump();
    s = statuses.whereType<ChatStreamReconnecting>().last;
    expect(s.delay, const Duration(seconds: 2));
    client.stop();
  });

  testWidgets('Full Jitter with a zero draw reconnects without waiting', (
    tester,
  ) async {
    final client = build(jitter: 0);
    client.start();
    await tester.pump();
    opener.last.addError(ConnectException(Code.unavailable, 'down'));
    await tester.pump();
    await tester.pump(Duration.zero);
    await tester.pump();
    expect(opener.calls, hasLength(2));
    client.stop();
  });

  testWidgets('backoff is capped', (tester) async {
    final client = build(jitter: 1.0);
    client.start();
    for (var i = 0; i < 10; i++) {
      await tester.pump();
      opener.last.addError(ConnectException(Code.unavailable, 'down'));
      await tester.pump();
      await tester.pump(const Duration(minutes: 2));
    }
    final s = statuses.whereType<ChatStreamReconnecting>().last;
    expect(s.delay, lessThanOrEqualTo(const Duration(seconds: 30)));
    client.stop();
  });

  testWidgets('resource_exhausted waits at least the long backoff', (
    tester,
  ) async {
    final client = build(jitter: 0);
    client.start();
    await tester.pump();
    opener.last.addError(ConnectException(Code.resourceExhausted, 'cap'));
    await tester.pump();
    final s = statuses.whereType<ChatStreamReconnecting>().last;
    expect(s.delay, greaterThanOrEqualTo(const Duration(minutes: 1)));
    client.stop();
  });

  for (final c in [
    (Code.unauthenticated, ChatStreamStopReason.unauthenticated),
    (Code.notFound, ChatStreamStopReason.noStore),
    (Code.invalidArgument, ChatStreamStopReason.invalidRequest),
  ]) {
    testWidgets('${c.$1} stops without reconnecting', (tester) async {
      final client = build(jitter: 0);
      client.start();
      await tester.pump();
      opener.last.addError(ConnectException(c.$1, 'x'));
      await tester.pump();
      await tester.pump(const Duration(minutes: 5));
      expect(opener.calls, hasLength(1));
      expect((client.status as ChatStreamStopped).reason, c.$2);
      expect(client.isRunning, isFalse);
    });
  }

  testWidgets(
    'failed_precondition with STORE_CLOSED stops; other reasons retry',
    (tester) async {
      final client = build(jitter: 0);
      client.start();
      await tester.pump();
      opener.last.addError(reasonError(Code.failedPrecondition, 'OTHER'));
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      await tester.pump();
      expect(opener.calls, hasLength(2));
      opener.last.addError(
        reasonError(Code.failedPrecondition, 'STORE_CLOSED'),
      );
      await tester.pump();
      await tester.pump(const Duration(minutes: 5));
      expect(opener.calls, hasLength(2));
      expect(
        (client.status as ChatStreamStopped).reason,
        ChatStreamStopReason.storeClosed,
      );
    },
  );

  testWidgets('reconnectNow skips the backoff wait', (tester) async {
    final client = build(jitter: 1.0);
    client.start();
    await tester.pump();
    opener.last.addError(ConnectException(Code.unavailable, 'down'));
    await tester.pump();
    expect(opener.calls, hasLength(1));
    client.reconnectNow();
    await tester.pump();
    expect(opener.calls, hasLength(2));
    client.stop();
  });

  testWidgets('stop ends the loop and does not reconnect', (tester) async {
    final client = build();
    client.start();
    await tester.pump();
    client.stop();
    await tester.pump(const Duration(minutes: 5));
    expect(opener.calls, hasLength(1));
    expect(
      (client.status as ChatStreamStopped).reason,
      ChatStreamStopReason.stopped,
    );
  });

  testWidgets('maps frames: updated message, unread, peer read and resync', (
    tester,
  ) async {
    final client = build();
    client.start();
    await tester.pump();
    final c = opener.last;
    c.add(readyFrame());
    c.add(messageFrame(protoMessage(3, hidden: true), updated: true));
    c.add(pb.ChatEvent(unread: pb.UnreadChanged(unreadCount: i64(4))));
    c.add(
      pb.ChatEvent(
        peerRead: pb.PeerRead(conversationId: 'c1', peerReadSeq: i64(3)),
      ),
    );
    c.add(pb.ChatEvent(resync: pb.Resync(conversationId: 'c1')));
    c.add(pb.ChatEvent(resync: pb.Resync(all: true)));
    await tester.pump();
    final m = events.whereType<ChatMessageArrived>().single;
    expect(m.updated, isTrue);
    expect(m.message.hidden, isTrue);
    expect(events.whereType<ChatUnreadChanged>().single.unreadCount, 4);
    expect(events.whereType<ChatPeerRead>().single.peerReadSeq, 3);
    final resyncs = events.whereType<ChatResync>().toList();
    expect(resyncs.first.conversationId, 'c1');
    expect(resyncs.last.conversationId, isNull);
    client.stop();
  });

  test('noteSeq only moves forward', () {
    final client = build()
      ..noteSeq('c1', 9)
      ..noteSeq('c1', 4);
    expect(client.cursors['c1'], 9);
  });
}
