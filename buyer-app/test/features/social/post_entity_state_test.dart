import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:hawkr_buyer/features/social/presentation/post_entity.dart';
import 'package:hawkr_buyer/features/social/presentation/post_entity_store.dart';
import 'package:hawkr_buyer/features/social/presentation/post_feed_controller.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'social_fakes.dart';

pb.Post _post(int id, {bool liked = false, int likes = 0}) => pb.Post(
  id: '$id',
  body: '貼文 $id',
  viewerLiked: liked,
  likeCount: Int64(likes),
);

const _gc = Duration(milliseconds: 80);

class _Clock {
  const _Clock(this._tester);
  final WidgetTester _tester;
  Future<void> elapse(Duration d) => _tester.pump(d);
  Future<void> flush() => _tester.pump(Duration.zero);
}

ProviderContainer _container(FakeSocialPublic public, FakeSocialApp app) {
  final container = ProviderContainer(
    overrides: [
      postEntityGcTimeProvider.overrideWithValue(_gc),
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

final _feed = postFeedControllerProvider(merchantId: 7);

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  group('世代檢查（base）：觀看者在途中換了，舊結果丟棄', () {
    test('讀取：換觀看者前發出的第一頁晚回來，不蓋掉新觀看者的列表與實體', () async {
      final oldPage = Completer<pb.ListPostsResponse>();
      var calls = 0;
      final public = FakeSocialPublic(
        postsResponder: (_) {
          if (++calls == 1) return oldPage.future;
          return Future.value(pb.ListPostsResponse(posts: [_post(202)]));
        },
      );
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await Future<void>.delayed(Duration.zero);

      _session(container).signIn();
      expect((await container.read(_feed.future)).ids, [202]);
      oldPage.complete(pb.ListPostsResponse(posts: [_post(101)]));
      await Future<void>.delayed(Duration.zero);

      expect(container.read(_feed).value!.ids, [202]);
      expect(container.read(postEntityStoreProvider).posts.keys, [202]);
    });

    test('分頁：換觀看者前發出的下一頁晚回來，不接到新列表、不進實體', () async {
      final gate = Completer<pb.ListPostsResponse>();
      var firstPages = 0;
      final public = FakeSocialPublic(
        postsResponder: (req) {
          if (req.pageToken == 'next') return gate.future;
          firstPages++;
          return Future.value(
            pb.ListPostsResponse(
              posts: [_post(firstPages == 1 ? 1 : 10)],
              nextPageToken: firstPages == 1 ? 'next' : '',
            ),
          );
        },
      );
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      final stale = container.read(_feed.notifier).loadMore();
      await Future<void>.delayed(Duration.zero);
      _session(container).signIn();
      expect((await container.read(_feed.future)).ids, [10]);
      gate.complete(pb.ListPostsResponse(posts: [_post(99)]));
      await stale;

      expect(container.read(_feed).value!.ids, [10]);
      expect(
        container.read(postEntityStoreProvider).posts.containsKey(99),
        isFalse,
      );
    });

    test('mutation 成功：換觀看者前發出的讚晚回來，結果丟棄，實體是新觀看者的資料', () async {
      final app = FakeSocialApp()..likeGate = Completer<void>();
      var loads = 0;
      final public = FakeSocialPublic(
        postsResponder: (_) async => pb.ListPostsResponse(
          posts: [_post(101, liked: ++loads > 1, likes: loads > 1 ? 5 : 0)],
        ),
      );
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      final like = container
          .read(postEntityStoreProvider.notifier)
          .toggleLike(101);
      await Future<void>.delayed(Duration.zero);
      expect(app.postLikes, hasLength(1));
      _session(container).signIn();
      await container.read(_feed.future);
      app.likeGate!.complete();

      expect(await like, isFalse);
      final entity = container.read(postEntityStoreProvider).posts[101]!;
      expect(entity.liked, isTrue);
      expect(entity.likeCount, 5);
      expect(app.postUnlikes, isEmpty);
    });

    test('mutation 失敗：換觀看者後才失敗，不把舊狀態還原到新觀看者的實體', () async {
      final app = FakeSocialApp()..likeGate = Completer<void>();
      app.likeError = ConnectException(Code.unavailable, 'down');
      var loads = 0;
      final public = FakeSocialPublic(
        postsResponder: (_) async => pb.ListPostsResponse(
          posts: [_post(101, liked: ++loads > 1, likes: loads > 1 ? 5 : 0)],
        ),
      );
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      final like = container
          .read(postEntityStoreProvider.notifier)
          .toggleLike(101);
      await Future<void>.delayed(Duration.zero);
      _session(container).switchTo('account-3002');
      await container.read(_feed.future);
      app.likeGate!.complete();

      expect(await like, isFalse);
      final entity = container.read(postEntityStoreProvider).posts[101]!;
      expect(entity.liked, isTrue);
      expect(entity.likeCount, 5);
    });
  });

  group('讚是集中的 mutation', () {
    test('R1-5 失敗時以 id 還原讚與讚數，並回 false', () async {
      final app = FakeSocialApp()
        ..likeError = ConnectException(Code.unavailable, 'down');
      final public = FakeSocialPublic(posts: [_post(101, likes: 3)]);
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      final ok = await container
          .read(postEntityStoreProvider.notifier)
          .toggleLike(101);

      expect(ok, isFalse);
      expect(app.postLikes, hasLength(1));
      final entity = container.read(postEntityStoreProvider).posts[101]!;
      expect(entity.liked, isFalse);
      expect(entity.likeCount, 3);
    });

    test('樂觀更新在伺服器回來前就生效', () async {
      final app = FakeSocialApp()..likeGate = Completer<void>();
      final public = FakeSocialPublic(posts: [_post(101, likes: 3)]);
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      final like = container
          .read(postEntityStoreProvider.notifier)
          .toggleLike(101);
      final mid = container.read(postEntityStoreProvider).posts[101]!;
      expect(mid.liked, isTrue);
      expect(mid.likeCount, 4);
      app.likeGate!.complete();
      expect(await like, isTrue);
    });

    test('R2-1 方向取自目前的實體：已讚的貼文送出取消讚', () async {
      final app = FakeSocialApp();
      final public = FakeSocialPublic(
        posts: [_post(101, liked: true, likes: 3)],
      );
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      await container.read(postEntityStoreProvider.notifier).toggleLike(101);

      expect(app.postUnlikes, hasLength(1));
      expect(app.postLikes, isEmpty);
      expect(container.read(postEntityStoreProvider).posts[101]!.likeCount, 2);
    });

    test('R2-1 列表縮短後對已不在的貼文按讚：不丟例外、不送請求', () async {
      final app = FakeSocialApp();
      final public = FakeSocialPublic(posts: [_post(101), _post(102)]);
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      container.read(postEntityStoreProvider.notifier).remove(101);

      expect(container.read(_feed).value!.ids, [102]);
      final ok = await container
          .read(postEntityStoreProvider.notifier)
          .toggleLike(101);
      expect(ok, isFalse);
      expect(app.postLikes, isEmpty);
    });
  });

  group('重新整理與不可見', () {
    test('重讀失敗保留舊值；只有找不到才從列表移除', () async {
      final public = FakeSocialPublic(
        posts: [_post(101, likes: 3), _post(102)],
      );
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);
      final store = container.read(postEntityStoreProvider.notifier);

      public.getPostError = ConnectException(Code.unavailable, 'down');
      expect(await store.refresh(101), isFalse);
      expect(container.read(postEntityStoreProvider).posts[101]!.likeCount, 3);
      expect(container.read(_feed).value!.ids, [101, 102]);

      public.getPostError = ConnectException(Code.notFound, 'gone');
      expect(await store.refresh(101), isFalse);
      expect(container.read(_feed).value!.ids, [102]);
    });

    test('實體 provider 不在存放處時讀 GetPost；找不到是 NotFoundException', () async {
      final public = FakeSocialPublic(post: _post(101, likes: 2));
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final provider = postEntityProvider(101);
      final sub = container.listen(provider, (_, _) {});
      addTearDown(sub.close);

      expect((await container.read(provider.future)).likeCount, 2);
      container.read(postEntityStoreProvider.notifier).remove(101);
      expect(container.read(provider).error, isA<NotFoundException>());
    });
  });

  group('同一則貼文只有一份', () {
    test('詳情（實體 provider）按讚：列表的實體與預覽卡讀到的同一份跟著變', () async {
      final public = FakeSocialPublic(posts: [_post(101, likes: 3)]);
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final feedSub = container.listen(_feed, (_, _) {});
      addTearDown(feedSub.close);
      await container.read(_feed.future);
      final detail = container.listen(postEntityProvider(101), (_, _) {});
      final preview = container.listen(postEntityProvider(101), (_, _) {});
      addTearDown(detail.close);
      addTearDown(preview.close);
      expect(public.getPostCalls, 0);

      await container.read(postEntityStoreProvider.notifier).toggleLike(101);

      expect(detail.read().value!.liked, isTrue);
      expect(preview.read().value!.likeCount, 4);
      expect(container.read(postEntityStoreProvider).posts[101]!.liked, isTrue);
      expect(container.read(_feed).value!.ids, [101]);
    });
  });

  group('實體回收', () {
    final post = Post(id: 101);

    /// 時間由 `tester.pump` 推進（fake clock），不等真實時間。
    Future<void> inClock(
      WidgetTester tester,
      FakeSocialPublic public,
      FakeSocialApp app,
      Future<void> Function(_Clock clock, ProviderContainer container) body,
    ) async {
      final container = _container(public, app);
      await container.read(sessionProvider.future);
      await body(_Clock(tester), container);
      for (var i = 0; i < 3; i++) {
        await tester.pump(Duration.zero);
      }
    }

    List<int> keys(ProviderContainer c) =>
        c.read(postEntityStoreProvider).posts.keys.toList();

    testWidgets('列表持有的實體不回收', (tester) async {
      await inClock(
        tester,
        FakeSocialPublic(posts: [_post(101)]),
        FakeSocialApp(),
        (clock, container) async {
          container.listen(_feed, (_, _) {});
          await clock.flush();

          await clock.elapse(_gc * 3);

          expect(keys(container), [101]);
        },
      );
    });

    testWidgets('有畫面監看的實體不回收，不再被監看之後過了時間才回收', (tester) async {
      await inClock(tester, FakeSocialPublic(), FakeSocialApp(), (
        clock,
        container,
      ) async {
        container.read(postEntityStoreProvider.notifier).seed([post]);
        final sub = container.listen(postSnapshotProvider(101), (_, _) {});

        await clock.elapse(_gc * 3);
        expect(keys(container), [101]);

        sub.close();
        await clock.flush();
        await clock.elapse(_gc ~/ 2);
        expect(keys(container), [101]);
        await clock.elapse(_gc);
        expect(keys(container), isEmpty);
      });
    });

    testWidgets('沒人引用的實體過了時間才回收', (tester) async {
      await inClock(tester, FakeSocialPublic(), FakeSocialApp(), (
        clock,
        container,
      ) async {
        container.read(postEntityStoreProvider.notifier).seed([post]);

        await clock.elapse(_gc ~/ 4);
        expect(keys(container), [101]);
        await clock.elapse(_gc);
        expect(keys(container), isEmpty);
      });
    });

    testWidgets('回收前又被列表持有：保留；列表放掉之後才回收', (tester) async {
      await inClock(tester, FakeSocialPublic(), FakeSocialApp(), (
        clock,
        container,
      ) async {
        final store = container.read(postEntityStoreProvider.notifier);
        final owner = Object();
        store.seed([post]);
        await clock.elapse(_gc ~/ 4);
        store.setListed(owner, [101]);

        await clock.elapse(_gc * 3);
        expect(keys(container), [101]);

        store.releaseList(owner);
        await clock.elapse(_gc * 3);
        expect(keys(container), isEmpty);
      });
    });

    testWidgets('先登記列表再 seed：不為即將被持有的實體排回收計時', (tester) async {
      await inClock(tester, FakeSocialPublic(), FakeSocialApp(), (
        clock,
        container,
      ) async {
        final store = container.read(postEntityStoreProvider.notifier);
        store.setListed(Object(), [101]);
        store.seed([post]);

        await clock.elapse(_gc * 3);

        expect(keys(container), [101]);
      });
    });

    testWidgets('換觀看者仍清掉全部實體，含被監看與被列表持有的', (tester) async {
      final public = FakeSocialPublic(posts: [_post(101)]);
      await inClock(tester, public, FakeSocialApp(), (clock, container) async {
        container.listen(_feed, (_, _) {});
        await clock.flush();
        container.listen(postSnapshotProvider(555), (_, _) {});
        container.read(postEntityStoreProvider.notifier).seed([Post(id: 555)]);
        expect(keys(container), [101, 555]);

        public.posts = [_post(202)];
        _session(container).signIn();
        await container.read(_feed.future);

        expect(keys(container), [202]);
      });
    });

    testWidgets('回收後才回來的重讀不把舊實體救回來', (tester) async {
      final gate = Completer<pb.Post>();
      final public = FakeSocialPublic(getPostResponder: () => gate.future);
      await inClock(tester, public, FakeSocialApp(), (clock, container) async {
        final store = container.read(postEntityStoreProvider.notifier);
        store.seed([post]);

        store.refresh(101);
        await clock.elapse(_gc * 3);
        expect(keys(container), isEmpty);
        gate.complete(_post(101, likes: 9));
        await clock.flush();

        expect(keys(container), isEmpty);
      });
    });

    testWidgets('進行中的讚讓實體保留；讚結束（失敗）之後才開始計回收', (tester) async {
      final app = FakeSocialApp()
        ..likeGate = Completer<void>()
        ..likeError = ConnectException(Code.unavailable, 'down');
      await inClock(tester, FakeSocialPublic(), app, (clock, container) async {
        final store = container.read(postEntityStoreProvider.notifier);
        store.seed([post]);

        bool? result;
        store.toggleLike(101).then((v) => result = v);
        await clock.elapse(_gc * 3);
        expect(keys(container), [101]);
        app.likeGate!.complete();
        await clock.flush();

        expect(result, isFalse);
        expect(keys(container), [101]);
        await clock.elapse(_gc * 3);
        expect(keys(container), isEmpty);
      });
    });
  });

  group('讚：樂觀疊加層', () {
    test('讀取中途寫入較新的基準：失敗只丟疊加層，不把舊快照寫回', () async {
      final app = FakeSocialApp()
        ..likeGate = Completer<void>()
        ..likeError = ConnectException(Code.unavailable, 'down');
      final container = _container(FakeSocialPublic(), app);
      await container.read(sessionProvider.future);
      final store = container.read(postEntityStoreProvider.notifier);
      store.seed([Post(id: 101, likeCount: 3)]);

      final like = store.toggleLike(101);
      await Future<void>.delayed(Duration.zero);
      expect(container.read(postEntityStoreProvider).posts[101]!.liked, isTrue);
      store.seed([Post(id: 101, likeCount: 7)]);
      app.likeGate!.complete();

      expect(await like, isFalse);
      final entity = container.read(postEntityStoreProvider).posts[101]!;
      expect(entity.liked, isFalse);
      expect(entity.likeCount, 7);
    });

    test('讀取中途寫入較舊的資料：樂觀的讚留著，成功後以基準加意圖為準', () async {
      final app = FakeSocialApp()..likeGate = Completer<void>();
      final container = _container(FakeSocialPublic(), app);
      await container.read(sessionProvider.future);
      final store = container.read(postEntityStoreProvider.notifier);
      store.seed([Post(id: 101, likeCount: 3)]);

      final like = store.toggleLike(101);
      await Future<void>.delayed(Duration.zero);
      store.seed([Post(id: 101, likeCount: 5)]);
      final mid = container.read(postEntityStoreProvider).posts[101]!;
      expect(mid.liked, isTrue);
      expect(mid.likeCount, 6);
      app.likeGate!.complete();

      expect(await like, isTrue);
      final entity = container.read(postEntityStoreProvider).posts[101]!;
      expect(entity.liked, isTrue);
      expect(entity.likeCount, 6);
    });

    test('快速按讚再取消：不丟點擊，最後送出取消，畫面是沒按讚', () async {
      final app = FakeSocialApp()..likeGate = Completer<void>();
      final container = _container(FakeSocialPublic(), app);
      await container.read(sessionProvider.future);
      final store = container.read(postEntityStoreProvider.notifier);
      store.seed([Post(id: 101, likeCount: 3)]);

      final first = store.toggleLike(101);
      await Future<void>.delayed(Duration.zero);
      final second = store.toggleLike(101);
      final entity = container.read(postEntityStoreProvider).posts[101]!;
      expect(entity.liked, isFalse);
      expect(entity.likeCount, 3);
      app.likeGate!.complete();

      expect(await first, isTrue);
      expect(await second, isTrue);
      expect(app.postLikes, hasLength(1));
      expect(app.postUnlikes, hasLength(1));
      final end = container.read(postEntityStoreProvider).posts[101]!;
      expect(end.liked, isFalse);
      expect(end.likeCount, 3);
    });
  });

  group('找不到', () {
    test('第一次讀就找不到：標為不可見', () async {
      final public = FakeSocialPublic(
        getPostError: ConnectException(Code.notFound, 'gone'),
      );
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final sub = container.listen(postEntityProvider(999), (_, _) {});
      addTearDown(sub.close);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      expect(container.read(postEntityStoreProvider).gone, contains(999));
    });

    test('重讀找不到：列表移除該 id', () async {
      final public = FakeSocialPublic(
        posts: [_post(101), _post(102)],
        getPostError: ConnectException(Code.notFound, 'gone'),
      );
      final container = _container(public, FakeSocialApp());
      await container.read(sessionProvider.future);
      final sub = container.listen(_feed, (_, _) {});
      addTearDown(sub.close);
      await container.read(_feed.future);

      await container.read(postEntityStoreProvider.notifier).refresh(101);

      expect(container.read(_feed).value!.ids, [102]);
    });
  });
}
