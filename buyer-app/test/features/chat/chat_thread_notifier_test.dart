import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/chat/data/chat_realtime.dart';
import 'package:hawkr_buyer/features/chat/data/chat_repository.dart';
import 'package:hawkr_buyer/features/chat/data/chat_thread_notifier.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'chat_fakes.dart';

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

const _byConversation = ChatThreadArg(conversationId: 'c1');
const _byStore = ChatThreadArg(storeId: '7');

({ProviderContainer container, FakeChatRepository repo}) _setup({
  ChatThreadArg arg = _byConversation,
  bool viewing = true,
  void Function(FakeChatRepository repo)? seed,
}) {
  final repo = FakeChatRepository();
  repo.conversations = [chatConversation('c1')];
  repo.addMessages('c1', [for (var i = 1; i <= 5; i++) chatMessage('c1', i)]);
  seed?.call(repo);
  final container = ProviderContainer(
    overrides: [
      chatRepositoryProvider.overrideWithValue(repo),
      sessionProvider.overrideWith(_GuestSession.new),
    ],
  );
  addTearDown(container.dispose);
  container.listen(chatThreadProvider(arg), (_, _) {});
  container.read(chatThreadProvider(arg).notifier).setViewing(viewing);
  return (container: container, repo: repo);
}

ChatThreadState _state(ProviderContainer c, ChatThreadArg arg) =>
    c.read(chatThreadProvider(arg));

ConnectException _reasonError(String reason) => ConnectException(
  Code.failedPrecondition,
  'blocked',
  details: [
    ErrorDetail(
      'google.rpc.ErrorInfo',
      (ErrorInfo()
            ..reason = reason
            ..domain = 'hawkr')
          .writeToBuffer(),
    ),
    ErrorDetail(
      'google.rpc.LocalizedMessage',
      (LocalizedMessage()
            ..locale = 'zh-TW'
            ..message = '對方已封鎖，無法傳送訊息')
          .writeToBuffer(),
    ),
  ],
);

List<int> _seqs(ChatThreadState s) => [
  for (final m in s.messages) m.seq.toInt(),
];

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('初次載入：最新一頁由舊到新、hasMore、已讀游標與標記已讀', () async {
    final t = _setup(seed: (r) => r.peerRead['c1'] = Int64(2));
    await settle();

    final s = _state(t.container, _byConversation);
    expect(s.loadStatus, ChatLoadStatus.loaded);
    expect(_seqs(s), [4, 5]);
    expect(s.hasMoreOlder, isTrue);
    expect(s.peerReadSeq, Int64(2));
    expect(t.repo.reads.last, ('c1', Int64(5)));
  });

  test('載入更舊：以目前最小的 seq 當 before_seq，載完 hasMore 變 false', () async {
    final t = _setup();
    await settle();
    final notifier = t.container.read(
      chatThreadProvider(_byConversation).notifier,
    );

    expect(await notifier.loadOlder(), isTrue);
    expect(await notifier.loadOlder(), isFalse);

    expect(_seqs(_state(t.container, _byConversation)), [1, 2, 3, 4, 5]);
    final befores = [
      for (final c in t.repo.listCalls)
        if (c.before != null) c.before,
    ];
    expect(befores, [Int64(4), Int64(2)]);
  });

  test('重新整理不丟掉已載入的舊頁，只補到最新', () async {
    final t = _setup();
    await settle();
    final notifier = t.container.read(
      chatThreadProvider(_byConversation).notifier,
    );
    await notifier.loadOlder();
    t.repo.addMessages('c1', [chatMessage('c1', 6)]);

    await notifier.refresh();

    expect(_seqs(_state(t.container, _byConversation)), [2, 3, 4, 5, 6]);
  });

  test('重新整理與載入更舊同時發生：依序執行，沒有重複也沒有空洞', () async {
    final t = _setup();
    await settle();
    final notifier = t.container.read(
      chatThreadProvider(_byConversation).notifier,
    );
    t.repo.addMessages('c1', [chatMessage('c1', 6), chatMessage('c1', 7)]);

    await Future.wait([notifier.refresh(), notifier.loadOlder()]);

    expect(_seqs(_state(t.container, _byConversation)), [2, 3, 4, 5, 6, 7]);
  });

  test('斷線補洞：序號跳號時以 after_seq 讀完所有頁', () async {
    final t = _setup();
    await settle();
    t.repo.addMessages('c1', [
      for (var i = 6; i <= 11; i++) chatMessage('c1', i),
    ]);

    t.repo.lastStream.add(readyEvent());
    t.repo.lastStream.add(messageEvent(chatMessage('c1', 11)));
    await settle(80);

    expect(_seqs(_state(t.container, _byConversation)), [
      4,
      5,
      6,
      7,
      8,
      9,
      10,
      11,
    ]);
    final afters = [
      for (final c in t.repo.listCalls)
        if (c.after != null) c.after,
    ];
    expect(afters.length, greaterThanOrEqualTo(3));
  });

  test('補洞失敗：錯誤留在 syncError，不吞掉；下一次重新整理成功就清掉', () async {
    final t = _setup();
    await settle();
    final notifier = t.container.read(
      chatThreadProvider(_byConversation).notifier,
    );
    t.repo.addMessages('c1', [
      for (var i = 6; i <= 9; i++) chatMessage('c1', i),
    ]);

    t.repo.listMessagesError = ConnectException(Code.unavailable, 'down');
    t.repo.lastStream.add(messageEvent(chatMessage('c1', 9)));
    await settle(60);
    expect(_state(t.container, _byConversation).syncError, isNotNull);
    expect(_seqs(_state(t.container, _byConversation)), [4, 5]);

    t.repo.listMessagesError = null;
    await notifier.refresh();
    expect(_state(t.container, _byConversation).syncError, isNull);
    expect(_seqs(_state(t.container, _byConversation)), [4, 5, 6, 7, 8, 9]);
  });

  test('串流重連的 Ready 觸發補洞', () async {
    final t = _setup();
    await settle();
    t.repo.addMessages('c1', [chatMessage('c1', 6)]);

    t.repo.lastStream.add(readyEvent());
    await settle(60);

    expect(_seqs(_state(t.container, _byConversation)), [4, 5, 6]);
  });

  test('可見度改變（updated）以 seq 取代手上那則', () async {
    final t = _setup();
    await settle();

    t.repo.lastStream.add(
      messageEvent(chatMessage('c1', 5, hidden: true), updated: true),
    );
    await settle();

    final m = _state(t.container, _byConversation).messages.last;
    expect(m.hidden, isTrue);
    expect(_seqs(_state(t.container, _byConversation)), [4, 5]);
  });

  test('對方已讀游標前進', () async {
    final t = _setup();
    await settle();

    t.repo.lastStream.add(peerReadEvent('c1', 4));
    await settle();

    expect(_state(t.container, _byConversation).peerReadSeq, Int64(4));
  });

  test('對話還不存在時開的聊天室：店家先寄來的第一則訊息不會被丟掉', () async {
    final t = _setup(
      arg: _byStore,
      seed: (r) {
        r.conversations = [];
        r.messages.clear();
      },
    );
    await settle();
    expect(_state(t.container, _byStore).conversationId, isNull);
    expect(_state(t.container, _byStore).loadStatus, ChatLoadStatus.loaded);

    t.repo.conversations = [chatConversation('conv-x', storeId: '7')];
    t.repo.addMessages('conv-x', [chatMessage('conv-x', 1, text: '嗨')]);
    t.repo.lastStream.add(messageEvent(chatMessage('conv-x', 1, text: '嗨')));
    await settle(60);

    final s = _state(t.container, _byStore);
    expect(s.conversationId, 'conv-x');
    expect(s.messages.single.text, '嗨');
  });

  test('別家店的對話事件不會被當成這個聊天室的', () async {
    final t = _setup(
      arg: _byStore,
      seed: (r) {
        r.conversations = [chatConversation('other', storeId: '99')];
        r.messages.clear();
        r.addMessages('other', [chatMessage('other', 1)]);
      },
    );
    await settle();

    t.repo.lastStream.add(messageEvent(chatMessage('other', 1)));
    await settle(60);

    expect(_state(t.container, _byStore).conversationId, isNull);
    expect(_state(t.container, _byStore).messages, isEmpty);
  });

  test('第一則訊息送出時伺服器開對話，之後的送出用對話 ID', () async {
    final t = _setup(
      arg: _byStore,
      seed: (r) {
        r.conversations = [];
        r.messages.clear();
      },
    );
    await settle();
    final notifier = t.container.read(chatThreadProvider(_byStore).notifier);

    await notifier.sendText('哈囉');
    await notifier.sendText('在嗎');

    expect(t.repo.sends.first.conversationId, 'store:7');
    expect(t.repo.sends.last.conversationId, 'conv-new');
    final s = _state(t.container, _byStore);
    expect(s.conversationId, 'conv-new');
    expect(_seqs(s), [1, 2]);
    expect(s.pending, isEmpty);
  });

  test('初次載入還沒完成就送出：不取消載入，歷史與新訊息都在', () async {
    final hold = Completer<void>();
    final t = _setup(
      arg: _byStore,
      seed: (r) {
        r.conversations = [chatConversation('c1', storeId: '7')];
        r.holdConversations = hold;
      },
    );
    await settle();
    final notifier = t.container.read(chatThreadProvider(_byStore).notifier);

    final sending = notifier.sendText('先送');
    await settle();
    hold.complete();
    await sending;
    await settle(60);

    final s = _state(t.container, _byStore);
    expect(s.loadStatus, ChatLoadStatus.loaded);
    expect(s.messages.map((m) => m.text), containsAll(['m5', '先送']));
  });

  test('送出失敗：保留為失敗的待送，重試用同一個 request_id，伺服器只寫一則', () async {
    final t = _setup();
    await settle();
    final notifier = t.container.read(
      chatThreadProvider(_byConversation).notifier,
    );
    t.repo.sendErrors.add(ConnectException(Code.unavailable, 'down'));

    await expectLater(
      notifier.sendText('你好'),
      throwsA(isA<ConnectException>()),
    );
    var s = _state(t.container, _byConversation);
    expect(s.pending.single.failed, isTrue);
    final requestId = s.pending.single.requestId;

    await notifier.retry(requestId);
    s = _state(t.container, _byConversation);
    expect(s.pending, isEmpty);
    expect(t.repo.sends.map((e) => e.requestId).toSet(), {requestId});
    expect(s.messages.where((m) => m.text == '你好').length, 1);
  });

  test('同一個 request_id 再送一次拿到第一次的訊息（冪等），不會多一則', () async {
    final t = _setup();
    await settle();
    final content = OutgoingContent.text('重複');
    final a = await t.repo.sendMessage('c1', content);
    final b = await t.repo.sendMessage('c1', content);
    expect(b.id, a.id);
    expect(t.repo.messages['c1']!.length, 6);
  });

  test('放棄失敗的待送', () async {
    final t = _setup();
    await settle();
    final notifier = t.container.read(
      chatThreadProvider(_byConversation).notifier,
    );
    t.repo.sendErrors.add(ConnectException(Code.unavailable, 'down'));
    await expectLater(notifier.sendText('x'), throwsA(anything));

    notifier.discard(
      _state(t.container, _byConversation).pending.single.requestId,
    );

    expect(_state(t.container, _byConversation).pending, isEmpty);
  });

  test('對方封鎖或已刪除（CHAT_BLOCKED、RECEIVER_DELETED）：輸入框改唯讀，顯示伺服器的說明', () async {
    for (final reason in ['CHAT_BLOCKED', 'RECEIVER_DELETED']) {
      final t = _setup();
      await settle();
      final notifier = t.container.read(
        chatThreadProvider(_byConversation).notifier,
      );
      t.repo.sendErrors.add(_reasonError(reason));

      await expectLater(notifier.sendText('x'), throwsA(anything));

      final s = _state(t.container, _byConversation);
      expect(s.readOnlyMessage, '對方已封鎖，無法傳送訊息', reason: reason);
      expect(s.pending, isEmpty, reason: reason);
    }
  });

  test('初次載入失敗：狀態是 failed，reload 可重試', () async {
    final t = _setup();
    t.repo.listMessagesError = ConnectException(Code.unavailable, 'down');
    t.container.invalidate(chatThreadProvider(_byConversation));
    await settle();
    expect(
      _state(t.container, _byConversation).loadStatus,
      ChatLoadStatus.failed,
    );

    t.repo.listMessagesError = null;
    await t.container
        .read(chatThreadProvider(_byConversation).notifier)
        .reload();
    expect(
      _state(t.container, _byConversation).loadStatus,
      ChatLoadStatus.loaded,
    );
  });

  ChatThread notifierOf(ProviderContainer c) =>
      c.read(chatThreadProvider(_byConversation).notifier);

  test('載入更舊同時被叫很多次：只讀一頁，不排隊', () async {
    final t = _setup();
    await settle();
    final notifier = notifierOf(t.container);

    await Future.wait([
      notifier.loadOlder(),
      notifier.loadOlder(),
      notifier.loadOlder(),
    ]);

    final befores = [
      for (final c in t.repo.listCalls)
        if (c.before != null) c.before,
    ];
    expect(befores, [Int64(4)]);
    expect(_seqs(_state(t.container, _byConversation)), [2, 3, 4, 5]);
    await notifier.loadOlder();
    expect(_seqs(_state(t.container, _byConversation)), [1, 2, 3, 4, 5]);
  });

  test('自己送的訊息 seq 跳過別人的訊息：先補洞，游標不越過空洞', () async {
    final t = _setup();
    await settle();
    t.repo.addMessages('c1', [chatMessage('c1', 6), chatMessage('c1', 7)]);

    await notifierOf(t.container).sendText('我的');

    expect(_seqs(_state(t.container, _byConversation)), [4, 5, 6, 7, 8]);
    expect(t.repo.listCalls.where((c) => c.after == Int64(5)), isNotEmpty);
    final rt = t.container.read(chatRealtimeProvider);
    expect(rt.requestCursors()['c1'], Int64(8));
  });

  test('補洞失敗：送出回報失敗，訊息與游標都停在洞之前', () async {
    final t = _setup();
    await settle();
    t.repo.addMessages('c1', [chatMessage('c1', 6)]);
    t.repo.listMessagesError = StateError('offline');

    await expectLater(
      notifierOf(t.container).sendText('我的'),
      throwsA(isA<StateError>()),
    );

    final s = _state(t.container, _byConversation);
    expect(_seqs(s), [4, 5]);
    expect(s.pending.single.failed, isTrue);
    expect(
      t.container.read(chatRealtimeProvider).requestCursors()['c1'],
      Int64(5),
    );
  });

  test('比已載入視窗更舊的 updated 事件不放進來，載入更舊仍連續', () async {
    final t = _setup();
    await settle();

    t.repo.lastStream.add(
      messageEvent(chatMessage('c1', 1, hidden: true), updated: true),
    );
    await settle();
    expect(_seqs(_state(t.container, _byConversation)), [4, 5]);

    await notifierOf(t.container).loadOlder();
    expect(_seqs(_state(t.container, _byConversation)), [2, 3, 4, 5]);
  });

  test('載入期間收到的事件不丟：載入完成後補一次', () async {
    final hold = Completer<void>();
    final holdAfter = Completer<void>();
    final t = _setup(
      seed: (r) {
        r.holdConversations = hold;
        r.holdAfter = holdAfter;
      },
    );
    await settle();
    t.repo.lastStream.add(messageEvent(chatMessage('c1', 6)));
    await settle();

    hold.complete();
    await settle(60);
    expect(
      t.repo.listCalls.where((c) => c.after == Int64(5)),
      isNotEmpty,
      reason: '載入完成後以 after_seq 補一次',
    );
    t.repo.addMessages('c1', [chatMessage('c1', 6)]);
    holdAfter.complete();
    await settle(60);

    expect(_seqs(_state(t.container, _byConversation)), [4, 5, 6]);
  });

  test('畫面不可見時不自動標記已讀，回到可見才補標記', () async {
    final t = _setup(viewing: false);
    await settle();
    t.repo.lastStream.add(messageEvent(chatMessage('c1', 6)));
    await settle();
    expect(t.repo.reads, isEmpty);

    notifierOf(t.container).setViewing(true);
    await settle();

    expect(t.repo.reads.last, ('c1', Int64(6)));
  });
}
