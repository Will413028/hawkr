import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:hawkr_buyer/features/social/presentation/comments_notifier.dart';
import 'package:hawkr_buyer/features/social/presentation/post_entity_store.dart';
import 'package:hawkr_buyer/features/social/presentation/write_result.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'social_fakes.dart';

final _comments = commentsProvider(101);

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
  container.listen(_comments, (_, _) {});
  container.listen(postEntityStoreProvider, (_, _) {});
  await _settle();
  return container;
}

void _seedPost(ProviderContainer c, {int comments = 5}) {
  c.read(postEntityStoreProvider.notifier).seed([
    Post(id: 101, commentCount: comments),
  ]);
  c.read(postEntityStoreProvider.notifier).retain(101);
}

List<String> _rootIds(ProviderContainer c) => c.read(_comments).tree.rootIds;

int _count(ProviderContainer c) =>
    c.read(postEntityStoreProvider).posts[101]!.commentCount;

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  group('讀取', () {
    test('R1-1 換觀看者：重讀並清掉前一位的留言與已展開的回覆', () async {
      var calls = 0;
      final public = FakeSocialPublic(
        commentsResponder: (req) {
          if (req.hasParentId()) {
            return pb.ListCommentsResponse(comments: [fakeComment('r1')]);
          }
          return pb.ListCommentsResponse(
            comments: [
              if (++calls == 1)
                fakeComment('old', replyCount: 1)
              else
                fakeComment('new'),
            ],
          );
        },
      );
      final c = await _open(public);
      await c.read(_comments.notifier).loadReplies('old');
      expect(c.read(_comments).tree.replyIds['old'], ['r1']);

      _session(c).signIn();
      await _settle();

      expect(_rootIds(c), ['new']);
      expect(c.read(_comments).tree.replyIds, isEmpty);
      expect(c.read(_comments).phase, CommentsPhase.ready);
    });

    test('R1-2 換觀看者前發出的第一頁晚回來：丟棄', () async {
      final oldPage = Completer<pb.ListCommentsResponse>();
      var calls = 0;
      final public = FakeSocialPublic(
        commentsResponder: (_) {
          if (++calls == 1) return oldPage.future;
          return pb.ListCommentsResponse(comments: [fakeComment('new')]);
        },
      );
      final c = await _open(public);
      _session(c).signIn();
      await _settle();
      oldPage.complete(pb.ListCommentsResponse(comments: [fakeComment('old')]));
      await _settle();

      expect(_rootIds(c), ['new']);
    });

    test('第一次讀失敗：進入 failed，重試成功後 ready', () async {
      var fail = true;
      final public = FakeSocialPublic(
        commentsResponder: (_) {
          if (fail) throw ConnectException(Code.unavailable, 'down');
          return pb.ListCommentsResponse(comments: [fakeComment('c1')]);
        },
      );
      final c = await _open(public);
      expect(c.read(_comments).phase, CommentsPhase.failed);

      fail = false;
      expect(await c.read(_comments.notifier).reload(), isTrue);

      expect(c.read(_comments).phase, CommentsPhase.ready);
      expect(_rootIds(c), ['c1']);
    });

    test('R3-5 重讀失敗：保留已載入的留言，回報失敗', () async {
      var fail = false;
      final public = FakeSocialPublic(
        commentsResponder: (_) {
          if (fail) throw ConnectException(Code.unavailable, 'down');
          return pb.ListCommentsResponse(comments: [fakeComment('c1')]);
        },
      );
      final c = await _open(public);

      fail = true;
      final ok = await c.read(_comments.notifier).reload();

      expect(ok, isFalse);
      expect(c.read(_comments).phase, CommentsPhase.ready);
      expect(_rootIds(c), ['c1']);
    });

    test('重新整理之前發出的分頁晚回來：丟棄，不接到新的第一頁', () async {
      final gate = Completer<pb.ListCommentsResponse>();
      var firsts = 0;
      final public = FakeSocialPublic(
        commentsResponder: (req) {
          if (req.pageToken == 'next') return gate.future;
          firsts++;
          return pb.ListCommentsResponse(
            comments: [fakeComment(firsts == 1 ? 'a' : 'b')],
            nextPageToken: firsts == 1 ? 'next' : '',
          );
        },
      );
      final c = await _open(public);

      final stale = c.read(_comments.notifier).loadMore();
      await _settle();
      await c.read(_comments.notifier).reload();
      gate.complete(pb.ListCommentsResponse(comments: [fakeComment('late')]));
      await stale;

      expect(_rootIds(c), ['b']);
      expect(c.read(_comments).nextPageToken, isEmpty);
    });

    test('分頁：下一頁接在後面，已有的 id 不重複', () async {
      final public = FakeSocialPublic(
        commentsResponder: (req) {
          if (req.pageToken == 'next') {
            return pb.ListCommentsResponse(
              comments: [fakeComment('a'), fakeComment('b')],
            );
          }
          return pb.ListCommentsResponse(
            comments: [fakeComment('a')],
            nextPageToken: 'next',
          );
        },
      );
      final c = await _open(public);

      await c.read(_comments.notifier).loadMore();

      expect(_rootIds(c), ['a', 'b']);
      expect(c.read(_comments).nextPageToken, isEmpty);
    });
  });

  group('寫入', () {
    test('R3-2 換觀看者前發出的留言晚回來：丟棄，不進新觀看者的列表也不改留言數', () async {
      final app = FakeSocialApp()..pending = Completer<void>();
      final public = FakeSocialPublic(comments: [fakeComment('c1')]);
      final c = await _open(public, app: app);
      _seedPost(c);

      final write = c.read(_comments.notifier).create('舊觀看者的留言');
      await _settle();
      _session(c).signIn();
      await _settle();
      app.pending!.complete();
      final result = await write;

      expect(result.status, WriteStatus.discarded);
      expect(_rootIds(c), ['c1']);
      expect(c.read(_comments).sending, isFalse);
    });

    test('R3-3 換觀看者前發出的回覆晚回來：丟棄，不掛到新列表的父留言', () async {
      final app = FakeSocialApp()..pending = Completer<void>();
      final public = FakeSocialPublic(
        comments: [fakeComment('c1', replyCount: 1)],
      );
      final c = await _open(public, app: app);

      final write = c
          .read(_comments.notifier)
          .create('回覆', parentId: 'c1', replyToId: 'c1');
      await _settle();
      _session(c).signIn();
      await _settle();
      app.pending!.complete();
      final result = await write;

      expect(result.status, WriteStatus.discarded);
      expect(c.read(_comments).tree.replyIds, isEmpty);
      expect(c.read(_comments).tree.byId['c1']!.replyCount.toInt(), 1);
    });

    test('R3-2 寫入失敗但觀看者已換：不回報、不動新資料', () async {
      final app = FakeSocialApp()
        ..pending = Completer<void>()
        ..createError = ConnectException(Code.unavailable, 'down');
      final public = FakeSocialPublic(comments: [fakeComment('c1')]);
      final c = await _open(public, app: app);

      final write = c.read(_comments.notifier).create('x');
      await _settle();
      _session(c).signIn();
      await _settle();
      app.pending!.complete();

      expect((await write).status, WriteStatus.discarded);
      expect(_rootIds(c), ['c1']);
    });

    test('新增留言：帶 parent_id 與 reply_to_id，加進列表，貼文留言數加一', () async {
      final app = FakeSocialApp();
      final public = FakeSocialPublic(
        comments: [fakeComment('c1', replyCount: 1)],
      );
      final c = await _open(public, app: app);
      _seedPost(c, comments: 5);

      final top = await c.read(_comments.notifier).create('頂層');
      final reply = await c
          .read(_comments.notifier)
          .create('回覆', parentId: 'c1', replyToId: 'c9');

      expect(top.isApplied, isTrue);
      expect(reply.isApplied, isTrue);
      expect(app.created[0].hasParentId(), isFalse);
      expect(app.created[1].parentId, 'c1');
      expect(app.created[1].replyToId, 'c9');
      expect(c.read(_comments).tree.replyIds['c1'], ['new']);
      expect(c.read(_comments).tree.byId['c1']!.replyCount.toInt(), 2);
      expect(_count(c), 7);
    });

    test('新增留言失敗：回報失敗，列表與留言數不變，可再送', () async {
      final app = FakeSocialApp(
        createError: ConnectException(Code.unavailable, 'down'),
      );
      final public = FakeSocialPublic(comments: [fakeComment('c1')]);
      final c = await _open(public, app: app);
      _seedPost(c);

      final result = await c.read(_comments.notifier).create('x');

      expect(result.status, WriteStatus.failed);
      expect(_rootIds(c), ['c1']);
      expect(c.read(_comments).sending, isFalse);
      expect(_count(c), 5);
    });

    test('刪除：只移除那一則、不重讀，貼文留言數減一', () async {
      final app = FakeSocialApp();
      final public = FakeSocialPublic(
        comments: [fakeComment('c1'), fakeComment('c2')],
      );
      final c = await _open(public, app: app);
      _seedPost(c);

      final result = await c
          .read(_comments.notifier)
          .delete(c.read(_comments).tree.byId['c1']!);

      expect(result.isApplied, isTrue);
      expect(_rootIds(c), ['c2']);
      expect(public.listCommentsCalls, 1);
      expect(_count(c), 4);
    });

    test('刪除有回覆的頂層留言：變成已刪除佔位；刪最後一則回覆時佔位一併移除', () async {
      final app = FakeSocialApp();
      final public = FakeSocialPublic(
        comments: [fakeComment('c1', replyCount: 1)],
        replies: {
          'c1': [fakeComment('r1')],
        },
      );
      final c = await _open(public, app: app);
      await c.read(_comments.notifier).loadReplies('c1');

      await c
          .read(_comments.notifier)
          .delete(c.read(_comments).tree.byId['c1']!);
      expect(
        c.read(_comments).tree.byId['c1']!.state,
        pb.CommentState.COMMENT_STATE_DELETED,
      );

      await c
          .read(_comments.notifier)
          .delete(c.read(_comments).tree.byId['r1']!);
      expect(_rootIds(c), isEmpty);
    });

    test('刪除失敗：還原這一則，留言數不變', () async {
      final app = FakeSocialApp()
        ..deleteError = ConnectException(Code.unavailable, 'down');
      final public = FakeSocialPublic(
        comments: [fakeComment('c1'), fakeComment('c2')],
      );
      final c = await _open(public, app: app);
      _seedPost(c);

      final result = await c
          .read(_comments.notifier)
          .delete(c.read(_comments).tree.byId['c1']!);

      expect(result.status, WriteStatus.failed);
      expect(_rootIds(c), ['c1', 'c2']);
      expect(_count(c), 5);
    });

    test('R3-4 刪除在途中換觀看者：丟棄，失敗也不把舊的還原進新列表', () async {
      final app = FakeSocialApp()
        ..pending = Completer<void>()
        ..deleteError = ConnectException(Code.unavailable, 'down');
      var calls = 0;
      final public = FakeSocialPublic(
        commentsResponder: (_) => pb.ListCommentsResponse(
          comments: [
            if (++calls == 1) fakeComment('c1') else fakeComment('n1'),
          ],
        ),
      );
      final c = await _open(public, app: app);

      final write = c
          .read(_comments.notifier)
          .delete(c.read(_comments).tree.byId['c1']!);
      await _settle();
      _session(c).signIn();
      await _settle();
      _seedPost(c);
      app.pending!.complete();

      expect((await write).status, WriteStatus.discarded);
      expect(_rootIds(c), ['n1']);
      expect(_count(c), 5);
    });

    test('讚：本地先改；失敗還原；換觀看者後的結果丟棄', () async {
      final app = FakeSocialApp(
        likeError: ConnectException(Code.unavailable, 'down'),
      );
      final public = FakeSocialPublic(comments: [fakeComment('c1')]);
      final c = await _open(public, app: app);

      final failed = await c.read(_comments.notifier).toggleLike('c1');
      expect(failed.status, WriteStatus.failed);
      expect(c.read(_comments).tree.byId['c1']!.viewerLiked, isFalse);
      expect(c.read(_comments).tree.byId['c1']!.likeCount.toInt(), 0);

      app.likeError = null;
      app.pending = Completer<void>();
      final write = c.read(_comments.notifier).toggleLike('c1');
      await _settle();
      expect(c.read(_comments).tree.byId['c1']!.viewerLiked, isTrue);
      expect(c.read(_comments).tree.byId['c1']!.likeCount.toInt(), 1);
      _session(c).signIn();
      await _settle();
      app.pending!.complete();

      expect((await write).status, WriteStatus.discarded);
      expect(c.read(_comments).tree.byId['c1']!.viewerLiked, isFalse);
    });
  });
}
