import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart'
    show BlockAuthorRequest_Source;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/moderation/data/block_target.dart';
import 'package:hawkr_buyer/features/moderation/data/report_target.dart';
import 'package:hawkr_buyer/features/moderation/presentation/report_block_action_sheet.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_chat_tab.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/social/data/social_model.dart';
import 'package:hawkr_buyer/features/social/presentation/merchant_posts_feed_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/post_detail_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/post_entity_store.dart';
import 'package:hawkr_buyer/shared/widgets/comments_bottom_sheet.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hooks_riverpod/misc.dart' show Override;

import '../../support/fake_seller_repository.dart';
import '../social/social_fakes.dart';
import 'moderation_fakes.dart';

class _Session extends Session {
  _Session(this.loggedIn);
  final bool loggedIn;
  @override
  Future<SessionModel?> build() async => loggedIn
      ? const SessionModel(userId: 'account-3001', nickname: 'Iris')
      : null;
}

/// 舊 REST 的呼叫（聊天與評價入口）；記下每個請求，回成功。
class _LegacyAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return ResponseBody.fromString(
      jsonEncode({'code': 0, 'data': <String, Object>{}}),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _Harness {
  _Harness({
    FakeBuyerApp? buyerApp,
    FakeSocialPublic? public,
    this.loggedIn = true,
  }) : app = buyerApp ?? FakeBuyerApp(),
       public = public ?? FakeSocialPublic(),
       legacy = _LegacyAdapter();

  final FakeBuyerApp app;
  final FakeSocialPublic public;
  final _LegacyAdapter legacy;
  final bool loggedIn;
  late ProviderContainer container;

  Future<void> pump(
    WidgetTester tester,
    Widget home, {
    List<Override> overrides = const [],
  }) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = legacy;
    addTearDown(dio.close);
    container = ProviderContainer(
      overrides: [
        dioProvider.overrideWithValue(dio),
        buyerAppClientProvider.overrideWithValue(app.client()),
        socialPublicClientProvider.overrideWithValue(public.client()),
        socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
        sessionProvider.overrideWith(() => _Session(loggedIn)),
        ...overrides,
      ],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, _) => MaterialApp.router(
            theme: AppTheme.light,
            routerConfig: GoRouter(
              routes: [
                GoRoute(
                  path: '/',
                  builder: (_, _) => Scaffold(body: home),
                ),
                GoRoute(
                  path: '/login',
                  builder: (_, _) => const Scaffold(body: Text('登入頁')),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pumpAndSettle();
  }
}

/// 只開 [showReportBlockActionSheet] 的按鈕，用來單獨驗證「對象型別決定走哪條 API」。
Widget _opener(BlockTarget? target, {bool Function()? onBlocked}) => Builder(
  builder: (context) => TextButton(
    onPressed: () async {
      final blocked = await showReportBlockActionSheet(
        context,
        reportTarget: const ReportTarget.post('101'),
        blockTarget: target,
        blockUserLabel: '對方',
      );
      if (blocked) onBlocked?.call();
    },
    child: const Text('開啟選單'),
  ),
);

Future<void> _confirmBlock(WidgetTester tester) async {
  await tester.tap(find.text('開啟選單'));
  await tester.pumpAndSettle();
  await tester.tap(find.text('封鎖此使用者'));
  await tester.pumpAndSettle();
}

pb.Comment _comment(String id, {pb.Actor? author, bool mine = false}) =>
    fakeComment(id)
      ..isMine = mine
      ..author = author ?? buyerActor('小明');

pb.ForumEntry _entry(String id, {pb.Actor? author, bool mine = false}) =>
    fakeForumEntry(id)
      ..isMine = mine
      ..author = author ?? buyerActor('小華');

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  group('對象型別決定走哪條 API', () {
    testWidgets('貼文的店家：BlockStore，確認說明是 C7a', (tester) async {
      final h = _Harness();
      await h.pump(tester, _opener(const StoreBlockTarget('7')));

      await _confirmBlock(tester);
      expect(find.text(blockStoreConfirmText), findsOneWidget);
      expect(
        find.text('封鎖後，您將看不到這家店的貼文與討論區，雙方也無法互傳訊息。可隨時在「封鎖名單」解除。'),
        findsOneWidget,
      );
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      expect(h.app.storeBlocks.single.storeId, '7');
      expect(h.app.authorBlocks, isEmpty);
      expect(h.legacy.requests, isEmpty);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('聊天訊息的對象送 BlockAuthor.message_id，不走舊 REST', (tester) async {
      final h = _Harness();
      await h.pump(
        tester,
        _opener(const MessageBlockTarget('m-42', storeId: '7')),
      );

      await _confirmBlock(tester);
      expect(find.text(blockStoreConfirmText), findsOneWidget);
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      expect(h.app.authorBlocks.single.messageId, 'm-42');
      expect(h.app.authorBlocks.single.whichSource(), isNotNull);
      expect(h.app.storeBlocks, isEmpty);
      expect(h.legacy.requests, isEmpty);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('封鎖成功回傳 true，取消與失敗回傳 false', (tester) async {
      var blocked = 0;
      final h = _Harness(
        buyerApp: FakeBuyerApp(
          blockError: ConnectException(Code.unavailable, 'down'),
        ),
      );
      await h.pump(
        tester,
        _opener(
          const StoreBlockTarget('7'),
          onBlocked: () {
            blocked++;
            return true;
          },
        ),
      );

      await _confirmBlock(tester);
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();
      expect(find.text('服務暫時不可用'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(blocked, 0);
    });

    testWidgets('沒有封鎖對象時選單沒有封鎖項', (tester) async {
      final h = _Harness();
      await h.pump(tester, _opener(null));

      await tester.tap(find.text('開啟選單'));
      await tester.pumpAndSettle();

      expect(find.text('檢舉'), findsOneWidget);
      expect(find.text('封鎖此使用者'), findsNothing);
    });
  });

  group('留言', () {
    Future<_Harness> pumpComments(
      WidgetTester tester,
      List<pb.Comment> comments, {
      bool loggedIn = true,
    }) async {
      final h = _Harness(
        public: FakeSocialPublic(comments: comments),
        loggedIn: loggedIn,
      );
      await h.pump(tester, CommentsBottomSheet(postId: 101));
      h.container.read(postEntityStoreProvider.notifier).seed([
        const Post(id: 101),
      ]);
      await tester.pumpAndSettle();
      return h;
    }

    testWidgets(
      '買家作者：選單有「檢舉」「封鎖此人」，封鎖送 BlockAuthor(comment_id)、說明是 C7b，封鎖後重讀',
      (tester) async {
        final h = await pumpComments(tester, [_comment('c1')]);
        final readsBefore = h.public.listCommentsCalls;

        await tester.tap(find.byKey(const ValueKey('comment-more-c1')));
        await tester.pumpAndSettle();
        expect(find.text('檢舉'), findsOneWidget);
        await tester.tap(find.text('封鎖此人'));
        await tester.pumpAndSettle();
        expect(find.text(blockBuyerConfirmText), findsOneWidget);
        expect(find.text('封鎖後，您與對方將看不到彼此的留言與評價。可隨時在「封鎖名單」解除。'), findsOneWidget);
        await tester.tap(find.text('封鎖'));
        await tester.pumpAndSettle();

        final request = h.app.authorBlocks.single;
        expect(request.whichSource(), BlockAuthorRequest_Source.commentId);
        expect(request.commentId, 'c1');
        expect(h.app.storeBlocks, isEmpty);
        expect(h.legacy.requests, isEmpty);
        expect(h.public.listCommentsCalls, greaterThan(readsBefore));
        await tester.pump(const Duration(seconds: 3));
      },
    );

    testWidgets('店家作者：確認說明是 C7a', (tester) async {
      final h = await pumpComments(tester, [
        _comment('c1', author: storeActor('好店')),
      ]);

      await tester.tap(find.byKey(const ValueKey('comment-more-c1')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();

      expect(find.text(blockStoreConfirmText), findsOneWidget);
      expect(find.text(blockBuyerConfirmText), findsNothing);
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();
      expect(h.app.authorBlocks.single.commentId, 'c1');
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('封鎖店家作者：該店已載入的貼文一併移除，其他店不動', (tester) async {
      final h = await pumpComments(tester, [
        _comment('c1', author: storeActor('好店')),
      ]);
      final posts = h.container.read(postEntityStoreProvider.notifier)
        ..setListed('test', [201, 202]);
      posts.seed(const [
        Post(id: 201, storeId: '7'),
        Post(id: 202, storeId: '8'),
      ]);

      await tester.tap(find.byKey(const ValueKey('comment-more-c1')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      final state = h.container.read(postEntityStoreProvider);
      expect(state.gone, contains(201));
      expect(state.gone, isNot(contains(202)));
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('封鎖買家作者：不動任何貼文', (tester) async {
      final h = await pumpComments(tester, [_comment('c1')]);
      final posts = h.container.read(postEntityStoreProvider.notifier)
        ..setListed('test', [201, 202]);
      posts.seed(const [Post(id: 201, storeId: '7')]);

      await tester.tap(find.byKey(const ValueKey('comment-more-c1')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      expect(h.container.read(postEntityStoreProvider).gone, isEmpty);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('自己的留言沒有檢舉與封鎖入口，別人的有', (tester) async {
      await pumpComments(tester, [
        _comment('mine', mine: true),
        _comment('other'),
      ]);

      expect(find.byKey(const ValueKey('comment-more-mine')), findsNothing);
      expect(find.byKey(const ValueKey('comment-more-other')), findsOneWidget);
    });

    testWidgets('作者身分不明時只能檢舉，不能封鎖', (tester) async {
      await pumpComments(tester, [
        _comment(
          'c1',
          author: pb.Actor(id: 'x', name: '不明'),
        ),
      ]);

      await tester.tap(find.byKey(const ValueKey('comment-more-c1')));
      await tester.pumpAndSettle();

      expect(find.text('檢舉'), findsOneWidget);
      expect(find.text('封鎖此人'), findsNothing);
    });

    testWidgets('訪客點入口先看到登入提示，沒有任何封鎖請求', (tester) async {
      final h = await pumpComments(tester, [_comment('c1')], loggedIn: false);

      await tester.tap(find.byKey(const ValueKey('comment-more-c1')));
      await tester.pumpAndSettle();

      expect(find.text('需要登入'), findsOneWidget);
      expect(find.text('封鎖此人'), findsNothing);
      expect(h.app.authorBlocks, isEmpty);
    });
  });

  group('討論區', () {
    Future<_Harness> pumpForum(
      WidgetTester tester,
      List<pb.ForumEntry> entries,
    ) async {
      final h = _Harness(public: FakeSocialPublic(forumEntries: entries));
      await h.pump(
        tester,
        MerchantChatTab(merchantId: '7', merchantName: '好店'),
      );
      return h;
    }

    testWidgets('買家作者：封鎖送 BlockAuthor(forum_entry_id)、說明是 C7b，封鎖後重讀', (
      tester,
    ) async {
      final h = await pumpForum(tester, [_entry('e1')]);
      final readsBefore = h.public.listForumCalls;

      await tester.tap(find.byTooltip('檢舉或封鎖'));
      await tester.pumpAndSettle();
      expect(find.text('檢舉'), findsOneWidget);
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();
      expect(find.text(blockBuyerConfirmText), findsOneWidget);
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      final request = h.app.authorBlocks.single;
      expect(request.whichSource(), BlockAuthorRequest_Source.forumEntryId);
      expect(request.forumEntryId, 'e1');
      expect(h.public.listForumCalls, greaterThan(readsBefore));
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('封鎖店家作者：該店已載入的貼文一併移除，其他店不動', (tester) async {
      final h = await pumpForum(tester, [
        _entry('e1', author: storeActor('好店')),
      ]);
      final posts = h.container.read(postEntityStoreProvider.notifier)
        ..setListed('test', [201, 202]);
      posts.seed(const [
        Post(id: 201, storeId: '7'),
        Post(id: 202, storeId: '8'),
      ]);

      await tester.tap(find.byTooltip('檢舉或封鎖'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      final state = h.container.read(postEntityStoreProvider);
      expect(state.gone, contains(201));
      expect(state.gone, isNot(contains(202)));
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('店家作者：確認說明是 C7a', (tester) async {
      await pumpForum(tester, [_entry('e1', author: storeActor('好店'))]);

      await tester.tap(find.byTooltip('檢舉或封鎖'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();

      expect(find.text(blockStoreConfirmText), findsOneWidget);
    });

    testWidgets('自己的留言沒有檢舉與封鎖入口', (tester) async {
      await pumpForum(tester, [_entry('mine', mine: true)]);

      expect(find.byTooltip('檢舉或封鎖'), findsNothing);
    });
  });

  group('封鎖等待中畫面已被移除', () {
    Future<void> hideWhilePending(
      WidgetTester tester, {
      required Widget Function() screen,
      required pb.Actor author,
      required Finder opener,
      required bool comments,
    }) async {
      final show = ValueNotifier(true);
      addTearDown(show.dispose);
      final gate = Completer<void>();
      final app = FakeBuyerApp()..blockGate = gate;
      final h = _Harness(
        buyerApp: app,
        public: comments
            ? FakeSocialPublic(comments: [_comment('c1', author: author)])
            : FakeSocialPublic(forumEntries: [_entry('e1', author: author)]),
      );
      await h.pump(
        tester,
        ValueListenableBuilder<bool>(
          valueListenable: show,
          builder: (_, visible, _) =>
              visible ? screen() : const SizedBox.shrink(),
        ),
      );
      final posts = h.container.read(postEntityStoreProvider.notifier);
      posts.setListed('test', [201, 202]);
      posts.seed(const [
        Post(id: 201, storeId: '7'),
        Post(id: 202, storeId: '8'),
      ]);

      await tester.tap(opener);
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此人'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖'));
      await tester.pump();
      show.value = false;
      await tester.pump();

      gate.complete();
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      final state = h.container.read(postEntityStoreProvider);
      expect(state.gone, contains(201));
      expect(state.gone, isNot(contains(202)));
      await tester.pump(const Duration(seconds: 3));
    }

    testWidgets('討論區：封鎖等待中畫面被移除，不丟例外且該店貼文已移除', (tester) async {
      await hideWhilePending(
        tester,
        screen: () => MerchantChatTab(merchantId: '7', merchantName: '好店'),
        author: storeActor('好店'),
        opener: find.byTooltip('檢舉或封鎖'),
        comments: false,
      );
    });

    testWidgets('留言：封鎖等待中畫面被移除，不丟例外且該店貼文已移除', (tester) async {
      await hideWhilePending(
        tester,
        screen: () => CommentsBottomSheet(postId: 101),
        author: storeActor('好店'),
        opener: find.byKey(const ValueKey('comment-more-c1')),
        comments: true,
      );
    });
  });

  group('貼文入口', () {
    testWidgets('貼文詳情封鎖店家：BlockStore，成功後該店的貼文標為不可見', (tester) async {
      final public = FakeSocialPublic(post: fakePost());
      final h = _Harness(public: public);
      await h.pump(
        tester,
        const PostDetailScreen(postId: 101),
        overrides: [
          sellerRepositoryProvider.overrideWithValue(fakeSellerRepository()),
        ],
      );

      await tester.tap(find.byTooltip('更多貼文操作'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此使用者'));
      await tester.pumpAndSettle();
      expect(find.text(blockStoreConfirmText), findsOneWidget);
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      expect(h.app.storeBlocks.single.storeId, '7');
      expect(h.legacy.requests, isEmpty);
      expect(h.container.read(postEntityStoreProvider).gone, contains(101));
      await tester.pump(const Duration(seconds: 3));
    });
  });

  group('店家貼文列表', () {
    testWidgets('封鎖店家：BlockStore，成功後該店的貼文標為不可見', (tester) async {
      final post = fakePost();
      final h = _Harness(
        public: FakeSocialPublic(post: post, posts: [post]),
      );
      await h.pump(
        tester,
        const MerchantPostsFeedScreen(merchantId: '7'),
        overrides: [
          sellerRepositoryProvider.overrideWithValue(fakeSellerRepository()),
        ],
      );

      await tester.tap(find.byTooltip('更多貼文操作'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('封鎖此使用者'));
      await tester.pumpAndSettle();
      expect(find.text(blockStoreConfirmText), findsOneWidget);
      await tester.tap(find.text('封鎖'));
      await tester.pumpAndSettle();

      expect(h.app.storeBlocks.single.storeId, '7');
      expect(h.container.read(postEntityStoreProvider).gone, contains(101));
      await tester.pump(const Duration(seconds: 3));
    });
  });

  group('貼文實體', () {
    ProviderContainer makeContainer() {
      final container = ProviderContainer(
        overrides: [
          sessionProvider.overrideWith(() => _Session(true)),
          socialPublicClientProvider.overrideWithValue(
            FakeSocialPublic().client(),
          ),
          socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
        ],
      );
      addTearDown(container.dispose);
      return container;
    }

    test('封鎖店家後，該店已載入的貼文標為不可見，其他店的不動', () async {
      final container = makeContainer();
      await container.read(sessionProvider.future);
      final store = container.read(postEntityStoreProvider.notifier);
      store.seed(const [
        Post(id: 1, storeId: '7'),
        Post(id: 2, storeId: '7'),
        Post(id: 3, storeId: '8'),
      ]);

      store.removeStorePosts('7');

      final state = container.read(postEntityStoreProvider);
      expect(state.gone, {1, 2});
      expect(state.posts.keys, [3]);
    });

    test('店家 id 不是數字也照字串比對；沒有店家資料的貼文由呼叫端指名一併移除', () async {
      final container = makeContainer();
      await container.read(sessionProvider.future);
      final store = container.read(postEntityStoreProvider.notifier);
      store.seed(const [
        Post(id: 1, storeId: 'store-abc'),
        Post(id: 2, storeId: 'store-abc'),
        Post(id: 3, storeId: 'other'),
        Post(id: 4),
        Post(id: 5),
      ]);

      store.removeStorePosts('store-abc', alsoPostId: 4);

      final state = container.read(postEntityStoreProvider);
      expect(state.gone, {1, 2, 4});
      expect(state.posts.keys, unorderedEquals([3, 5]));
    });

    test('一次封鎖只發佈一次狀態', () async {
      final container = makeContainer();
      await container.read(sessionProvider.future);
      final store = container.read(postEntityStoreProvider.notifier);
      store.seed([for (var i = 1; i <= 20; i++) Post(id: i, storeId: '7')]);
      var publishes = 0;
      container.listen(postEntityStoreProvider, (_, _) => publishes++);

      store.removeStorePosts('7');

      expect(publishes, 1);
      expect(container.read(postEntityStoreProvider).gone, hasLength(20));
    });
  });
}
