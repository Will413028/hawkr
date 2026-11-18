import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/social/presentation/comments_notifier.dart'
    show CommentsPhase;
import 'package:hawkr_buyer/features/social/presentation/forum_notifier.dart';
import 'package:hawkr_buyer/features/social/presentation/write_result.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'social_fakes.dart';

final _forum = forumProvider('7');

ProviderContainer _container(FakeSocialPublic public, FakeSocialApp app) {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(app.client()),
      sessionProvider.overrideWith(SwitchableSession.new),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

SwitchableSession _session(ProviderContainer c) =>
    c.read(sessionProvider.notifier) as SwitchableSession;

Future<void> _settle() async {
  for (var i = 0; i < 8; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

Future<ProviderContainer> _open(
  FakeSocialPublic public, {
  FakeSocialApp? app,
}) async {
  final container = _container(public, app ?? FakeSocialApp());
  await container.read(sessionProvider.future);
  container.listen(_forum, (_, _) {});
  await _settle();
  return container;
}

List<String> _rootIds(ProviderContainer c) => c.read(_forum).tree.rootIds;

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  group('讀取', () {
    test('R1-1 換觀看者：重讀，前一位的項目、權限、身分與回覆一併清掉', () async {
      var calls = 0;
      final public = FakeSocialPublic(
        forumResponder: (req) {
          if (req.hasParentId()) {
            return pb.ListForumEntriesResponse(entries: [fakeForumEntry('r1')]);
          }
          final first = ++calls == 1;
          return pb.ListForumEntriesResponse(
            entries: [
              first
                  ? fakeForumEntry('old', replyCount: 1)
                  : fakeForumEntry('new'),
            ],
            viewerCanComment: first,
            viewerActor: first ? storeActor('好店') : null,
          );
        },
      );
      final c = await _open(public);
      expect(c.read(_forum).tree.replyIds['old'], ['r1']);
      expect(c.read(_forum).viewerCanComment, isTrue);

      _session(c).signIn();
      await _settle();

      expect(_rootIds(c), ['new']);
      expect(c.read(_forum).tree.replyIds, isEmpty);
      expect(c.read(_forum).viewerCanComment, isFalse);
      expect(c.read(_forum).viewerActor, isNull);
    });

    test('R1-2 換觀看者前發出的讀取與回覆晚回來：丟棄', () async {
      final oldPage = Completer<pb.ListForumEntriesResponse>();
      var calls = 0;
      final public = FakeSocialPublic(
        forumResponder: (_) {
          if (++calls == 1) return oldPage.future;
          return pb.ListForumEntriesResponse(entries: [fakeForumEntry('new')]);
        },
      );
      final c = await _open(public);
      _session(c).signIn();
      await _settle();
      oldPage.complete(
        pb.ListForumEntriesResponse(entries: [fakeForumEntry('old')]),
      );
      await _settle();

      expect(_rootIds(c), ['new']);
    });

    test('重新整理之前發出的回覆讀取晚回來：丟棄', () async {
      final gate = Completer<pb.ListForumEntriesResponse>();
      var roots = 0;
      final public = FakeSocialPublic(
        forumResponder: (req) {
          if (req.hasParentId()) return gate.future;
          return pb.ListForumEntriesResponse(
            entries: [fakeForumEntry('a', replyCount: ++roots == 1 ? 1 : 0)],
          );
        },
      );
      final c = await _open(public);
      await c.read(_forum.notifier).reload();
      gate.complete(
        pb.ListForumEntriesResponse(entries: [fakeForumEntry('late')]),
      );
      await _settle();

      expect(c.read(_forum).tree.replyIds, isEmpty);
    });

    test('第一次讀失敗：failed，重試成功後 ready', () async {
      var fail = true;
      final public = FakeSocialPublic(
        forumResponder: (_) {
          if (fail) throw ConnectException(Code.unavailable, 'down');
          return pb.ListForumEntriesResponse(entries: [fakeForumEntry('e1')]);
        },
      );
      final c = await _open(public);
      expect(c.read(_forum).phase, CommentsPhase.failed);

      fail = false;
      await c.read(_forum.notifier).reload();

      expect(c.read(_forum).phase, CommentsPhase.ready);
      expect(_rootIds(c), ['e1']);
    });

    test('R3-5 重讀失敗：保留已載入的項目與留言權限', () async {
      var fail = false;
      final public = FakeSocialPublic(
        forumResponder: (_) {
          if (fail) throw ConnectException(Code.unavailable, 'down');
          return pb.ListForumEntriesResponse(
            entries: [fakeForumEntry('e1')],
            viewerCanComment: true,
          );
        },
      );
      final c = await _open(public);

      fail = true;
      final ok = await c.read(_forum.notifier).reload();

      expect(ok, isFalse);
      expect(c.read(_forum).phase, CommentsPhase.ready);
      expect(_rootIds(c), ['e1']);
      expect(c.read(_forum).viewerCanComment, isTrue);
    });
  });

  group('寫入', () {
    test('R3-4 換觀看者前發出的發言晚回來：丟棄，不進新觀看者的討論區', () async {
      final app = FakeSocialApp()..pending = Completer<void>();
      final public = FakeSocialPublic(forumEntries: [fakeForumEntry('e1')]);
      final c = await _open(public, app: app);

      final write = c.read(_forum.notifier).create('舊觀看者');
      await _settle();
      _session(c).signIn();
      await _settle();
      app.pending!.complete();

      expect((await write).status, WriteStatus.discarded);
      expect(_rootIds(c), ['e1']);
      expect(c.read(_forum).submitting, isFalse);
    });

    test('R3-4 回覆在途中換觀看者：丟棄，不掛到新列表', () async {
      final app = FakeSocialApp()..pending = Completer<void>();
      final public = FakeSocialPublic(
        forumEntries: [fakeForumEntry('e1', replyCount: 1)],
      );
      final c = await _open(public, app: app);

      final write = c
          .read(_forum.notifier)
          .create('回覆', parentId: 'e1', replyToId: 'e1');
      await _settle();
      _session(c).signIn();
      await _settle();
      app.pending!.complete();

      expect((await write).status, WriteStatus.discarded);
      expect(c.read(_forum).tree.replyIds, isEmpty);
    });

    test('發言：回覆帶 parent_id 與 reply_to_id；失敗不改列表', () async {
      final app = FakeSocialApp();
      final public = FakeSocialPublic(
        forumEntries: [fakeForumEntry('e1', replyCount: 1)],
        forumReplies: {
          'e1': [fakeForumEntry('r1')],
        },
      );
      final c = await _open(public, app: app);

      final ok = await c
          .read(_forum.notifier)
          .create('回覆', parentId: 'e1', replyToId: 'r1');
      expect(ok.isApplied, isTrue);
      expect(app.forumCreated.single.parentId, 'e1');
      expect(app.forumCreated.single.replyToId, 'r1');
      expect(c.read(_forum).tree.replyIds['e1'], ['r1', 'new']);

      app.createError = ConnectException(Code.unavailable, 'down');
      final failed = await c.read(_forum.notifier).create('x');
      expect(failed.status, WriteStatus.failed);
      expect(_rootIds(c), ['e1']);
    });

    test('編輯：本地先改；失敗只還原這一則；換觀看者後的結果丟棄', () async {
      final app = FakeSocialApp()..pending = Completer<void>();
      final public = FakeSocialPublic(
        forumEntries: [fakeForumEntry('e1', body: '原文', canEdit: true)],
      );
      final c = await _open(public, app: app);

      final write = c
          .read(_forum.notifier)
          .edit(c.read(_forum).tree.byId['e1']!, '新文');
      await _settle();
      expect(c.read(_forum).tree.byId['e1']!.body, '新文');
      app.pending!.completeError(ConnectException(Code.unavailable, 'down'));

      expect((await write).status, WriteStatus.failed);
      expect(c.read(_forum).tree.byId['e1']!.body, '原文');

      final app2 = FakeSocialApp()..pending = Completer<void>();
      final c2 = await _open(public, app: app2);
      final stale = c2
          .read(_forum.notifier)
          .edit(c2.read(_forum).tree.byId['e1']!, '新文');
      await _settle();
      _session(c2).signIn();
      await _settle();
      app2.pending!.completeError(ConnectException(Code.unavailable, 'down'));

      expect((await stale).status, WriteStatus.discarded);
      expect(c2.read(_forum).tree.byId['e1']!.body, '原文');
    });

    test('刪除：本地先移除；失敗還原；換觀看者後丟棄', () async {
      final app = FakeSocialApp()..pending = Completer<void>();
      final public = FakeSocialPublic(
        forumEntries: [
          fakeForumEntry('e1', canDelete: true),
          fakeForumEntry('e2'),
        ],
      );
      final c = await _open(public, app: app);

      final write = c
          .read(_forum.notifier)
          .delete(c.read(_forum).tree.byId['e1']!);
      await _settle();
      expect(_rootIds(c), ['e2']);
      app.pending!.completeError(ConnectException(Code.unavailable, 'down'));

      expect((await write).status, WriteStatus.failed);
      expect(_rootIds(c), ['e1', 'e2']);

      final app2 = FakeSocialApp()..pending = Completer<void>();
      final c2 = await _open(public, app: app2);
      final stale = c2
          .read(_forum.notifier)
          .delete(c2.read(_forum).tree.byId['e1']!);
      await _settle();
      _session(c2).signIn();
      await _settle();
      app2.pending!.completeError(ConnectException(Code.unavailable, 'down'));

      expect((await stale).status, WriteStatus.discarded);
      expect(c2.read(_forum).tree.rootIds, ['e1', 'e2']);
    });
  });
}
