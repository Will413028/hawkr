import 'package:fixnum/fixnum.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/features/chat/data/chat_realtime.dart';

import 'chat_fakes.dart';

ChatRealtime _realtime(
  FakeChatRepository repo, {
  Duration Function(Duration)? liveness,
}) => ChatRealtime(
  repo,
  reconnectDelay: (_) => const Duration(milliseconds: 5),
  livenessWindow: liveness,
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('Ready 後是 connected，總未讀進 unreadCount，Unread 事件更新它', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);
    final unread = <int>[];
    rt.unreadCount.listen(unread.add);

    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent(unread: 3));
    await settle();
    repo.lastStream.add(unreadEvent(5));
    await settle();

    expect(rt.currentState, ChatConnectionState.connected);
    expect(unread, [3, 5]);
  });

  test('cursors 最多 50 個，留最近更新的', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);
    for (var i = 0; i < 80; i++) {
      rt.noteSeq('c$i', Int64(i + 1));
    }

    rt.acquire();
    await settle();

    final sent = repo.connectCursors.single;
    expect(sent.length, 50);
    expect(sent.containsKey('c79'), isTrue);
    expect(sent.containsKey('c29'), isFalse);
    expect(sent['c79'], Int64(80));
  });

  test('cursors 只前進，收到的訊息推進它', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);
    rt.noteSeq('a', Int64(5));
    rt.noteSeq('a', Int64(3));
    expect(rt.requestCursors()['a'], Int64(5));

    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent());
    repo.lastStream.add(messageEvent(chatMessage('a', 9)));
    await settle();
    expect(rt.requestCursors()['a'], Int64(9));
  });

  test('伺服器到 max_lifetime 正常結束：立即以最新 cursors 重連', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);

    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent());
    repo.lastStream.add(messageEvent(chatMessage('a', 4)));
    await settle();
    await repo.lastStream.close();
    await settle();

    expect(repo.streams.length, 2);
    expect(repo.connectCursors.last['a'], Int64(4));
  });

  test('暫時性的結束（unavailable、resource_exhausted、一般錯誤）持續重連，不永久停止', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);

    repo.streamError = ConnectException(Code.unavailable, 'down');
    rt.acquire();
    await settle(120);
    expect(repo.streams.length, greaterThan(2));
    expect(rt.currentState, isNot(ChatConnectionState.disconnected));

    repo.streamError = ConnectException(Code.resourceExhausted, 'too many');
    final before = repo.streams.length;
    await settle(120);
    expect(repo.streams.length, greaterThan(before));

    repo.streamError = null;
    await settle(60);
    repo.lastStream.add(readyEvent());
    await settle();
    expect(rt.currentState, ChatConnectionState.connected);
  });

  test('unauthenticated 與 invalid_argument 才停止', () async {
    for (final code in [Code.unauthenticated, Code.invalidArgument]) {
      final repo = FakeChatRepository()
        ..streamError = ConnectException(code, 'x');
      final rt = _realtime(repo);
      rt.acquire();
      await settle(80);
      expect(repo.streams.length, 1, reason: code.name);
      expect(rt.currentState, ChatConnectionState.disconnected);
      rt.dispose();
    }
  });

  test('超過 heartbeat 兩倍沒有任何 frame：當成斷線重連', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(
      repo,
      liveness: (_) => const Duration(milliseconds: 30),
    );
    addTearDown(rt.dispose);

    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent(heartbeatMs: 15));
    await settle(10);
    expect(repo.streams.length, 1);
    await settle(120);

    expect(repo.streams.length, greaterThan(1));
  });

  test('Resync 與 Message 轉給 events，Heartbeat 不轉', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);
    final seen = <pb.ChatEvent_Event>[];
    rt.events.listen((e) => seen.add(e.whichEvent()));

    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent());
    repo.lastStream.add(pb.ChatEvent(heartbeat: pb.Heartbeat()));
    repo.lastStream.add(resyncAllEvent());
    repo.lastStream.add(messageEvent(chatMessage('a', 1)));
    await settle();

    expect(seen, [
      pb.ChatEvent_Event.ready,
      pb.ChatEvent_Event.resync,
      pb.ChatEvent_Event.message,
    ]);
  });

  test('最後一個畫面離開才斷線', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);

    rt.acquire();
    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent());
    await settle();
    rt.release();
    expect(rt.currentState, ChatConnectionState.connected);
    rt.release();
    expect(rt.currentState, ChatConnectionState.disconnected);
  });

  test('還沒 Ready 就正常結束：算失敗，以 backoff 重連，不是緊密迴圈', () async {
    final repo = FakeChatRepository();
    final attempts = <int>[];
    final rt = ChatRealtime(
      repo,
      reconnectDelay: (a) {
        attempts.add(a);
        return const Duration(milliseconds: 5);
      },
    );
    addTearDown(rt.dispose);

    rt.acquire();
    for (var i = 0; i < 3; i++) {
      await settle();
      await repo.lastStream.close();
    }
    await settle();

    expect(attempts.length, greaterThanOrEqualTo(2));
    expect(attempts, [for (var i = 0; i < attempts.length; i++) i]);
  });

  test('session 換了：清掉 cursors 與未讀數，以空 cursors 重連', () async {
    final repo = FakeChatRepository();
    final rt = _realtime(repo);
    addTearDown(rt.dispose);
    final unread = <int>[];
    rt.unreadCount.listen(unread.add);

    rt.acquire();
    await settle();
    repo.lastStream.add(readyEvent(unread: 4));
    repo.lastStream.add(messageEvent(chatMessage('a', 3)));
    await settle();

    rt.resetSession();
    await settle();

    expect(rt.requestCursors(), isEmpty);
    expect(unread.last, 0);
    expect(repo.streams.length, 2);
    expect(repo.connectCursors.last, isEmpty);
  });
}
